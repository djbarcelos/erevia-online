import { Effect, EntityKind, SkillTarget, type SelfStats, type SkillInfo } from "../gen/erevia/v1/game_pb";
import type { ActiveEffect, EntityInfo } from "../world/entities";
import { effectIcon, effectName, effectText, formatSeconds, isDebuff, skillIcon } from "./icons";

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls?: string, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

interface Bar {
  root: HTMLDivElement;
  fill: HTMLDivElement;
  text: HTMLSpanElement;
}

function bar(cls: string): Bar {
  const root = el("div", `bar ${cls}`);
  const fill = el("div", "fill");
  const text = el("span", "bar-text");
  root.append(fill, text);
  return { root, fill, text };
}

function setBar(b: Bar, value: number, max: number, label: string): void {
  const frac = max > 0 ? Math.max(0, Math.min(1, value / max)) : 0;
  b.fill.style.width = `${(frac * 100).toFixed(1)}%`;
  b.text.textContent = label;
}

interface Slot {
  info: SkillInfo;
  root: HTMLButtonElement;
  sweep: HTMLDivElement;
  timer: HTMLSpanElement;
  readyAt: number; // performance.now() em ms
  /** Duração da recarga em andamento (pode ser menor que a total, ao voltar ao jogo). */
  span: number;
}

/** Quanto tempo uma skill de alvo pode ficar "na fila" sem resposta do servidor. */
const QUEUE_TIMEOUT_MS = 8000;

export interface HudCallbacks {
  onSkill(s: SkillInfo): void;
  onRespawn(): void;
  /** Distância do jogador até o alvo atual (null = sem alvo inimigo). */
  targetDistance(): number | null;
}

/** Linha de ícones de buffs/debuffs com a contagem regressiva. */
class EffectRow {
  readonly root: HTMLDivElement;
  private items = new Map<Effect, { root: HTMLDivElement; sweep: HTMLDivElement; time: HTMLSpanElement }>();

  constructor(cls: string) {
    this.root = el("div", `effects ${cls}`);
  }

  set(list: ActiveEffect[]): void {
    const now = performance.now();
    const seen = new Set<Effect>();
    for (const ef of list) {
      seen.add(ef.effect);
      let item = this.items.get(ef.effect);
      if (!item) {
        const root = el("div", `effect${isDebuff(ef.effect) ? " debuff" : ""}`);
        root.innerHTML = effectIcon(ef.effect);
        const sweep = el("div", "effect-sweep");
        const time = el("span", "effect-time");
        root.append(sweep, time);
        item = { root, sweep, time };
        this.items.set(ef.effect, item);
        this.root.append(root);
      }
      const left = Math.max(0, ef.until - now);
      const frac = ef.durationMs > 0 ? left / ef.durationMs : 0;
      item.sweep.style.setProperty("--left", `${((1 - frac) * 360).toFixed(1)}deg`);
      item.time.textContent = left >= 10000 ? String(Math.ceil(left / 1000)) : formatSeconds(Math.ceil(left / 100) * 100);
      item.root.title = `${effectName(ef.effect)} — ${effectText(ef.effect, ef.value, ef.durationMs)}`;
      item.root.classList.toggle("ending", left < 2000);
    }
    for (const [k, item] of this.items) {
      if (!seen.has(k)) {
        item.root.remove();
        this.items.delete(k);
      }
    }
  }
}

/** HUD do jogo (HTML sobre o canvas): barras, alvo, skills, efeitos e tela de morte. */
export class Hud {
  private root = el("div", "game-hud");
  private hp = bar("hp");
  private mp = bar("mp");
  private xp = bar("xp");
  private levelEl = el("div", "level-badge", "1");
  private targetFrame = el("div", "target-frame");
  private targetName = el("div", "target-name");
  private targetHp = bar("hp");
  private slots: Slot[] = [];
  private death = el("div", "death-screen");
  private tip = el("div", "skill-tip");
  private myEffects = new EffectRow("mine");
  private targetEffects = new EffectRow("target");
  private mpNow = 0;
  private queued = 0;
  private queuedAt = 0;
  private raf = 0;

  constructor(
    skills: SkillInfo[],
    private cb: HudCallbacks,
  ) {
    // Barras do personagem (embaixo, ao centro)
    const me = el("div", "me-panel");
    const bars = el("div", "me-bars");
    bars.append(this.hp.root, this.mp.root);
    const top = el("div", "me-top");
    top.append(this.levelEl, bars);
    me.append(this.myEffects.root, top, this.xp.root);

    // Skills
    const skillbar = el("div", "skillbar");
    for (const s of [...skills].sort((a, b) => a.key - b.key)) {
      const btn = el("button", "skill");
      btn.type = "button";
      btn.setAttribute("aria-label", `${s.name} (tecla ${s.key})`);
      const icon = el("div", "skill-icon");
      icon.innerHTML = skillIcon(s.fx);
      const sweep = el("div", "skill-sweep");
      const key = el("span", "skill-key", String(s.key));
      const cost = el("span", "skill-cost", String(s.mpCost));
      const timer = el("span", "skill-timer");
      btn.append(icon, sweep, key, cost, timer);
      btn.onclick = () => this.cb.onSkill(s);
      btn.onmouseenter = btn.onfocus = () => this.showTip(s, btn);
      btn.onmouseleave = btn.onblur = () => (this.tip.hidden = true);
      skillbar.append(btn);
      this.slots.push({ info: s, root: btn, sweep, timer, readyAt: 0, span: 1 });
    }
    me.append(skillbar);
    this.tip.hidden = true;

    // Alvo (topo)
    this.targetFrame.append(this.targetName, this.targetHp.root, this.targetEffects.root);
    this.targetFrame.hidden = true;

    // Morte
    const box = el("div", "death-box");
    box.append(el("h2", "", "Você morreu"), el("p", "", "Você perdeu um pouco de experiência."));
    const btn = el("button", "btn btn-gold", "Voltar à cidade");
    btn.type = "button";
    btn.onclick = () => this.cb.onRespawn();
    box.append(btn);
    this.death.append(box);
    this.death.hidden = true;

    this.root.append(me, this.targetFrame, this.death, this.tip);
    document.body.append(this.root);

    const loop = () => {
      this.tick();
      this.raf = requestAnimationFrame(loop);
    };
    this.raf = requestAnimationFrame(loop);
  }

  dispose(): void {
    cancelAnimationFrame(this.raf);
    this.root.remove();
  }

  setStats(s: SelfStats): void {
    setBar(this.hp, s.hp, s.maxHp, `${s.hp} / ${s.maxHp}`);
    setBar(this.mp, s.mp, s.maxMp, `${s.mp} / ${s.maxMp}`);
    this.mpNow = s.mp;
    this.levelEl.textContent = String(s.level);
    if (s.xpNextLevel > 0n) {
      const into = Number(s.xp - s.xpLevelStart);
      const need = Number(s.xpNextLevel - s.xpLevelStart);
      setBar(this.xp, into, need, `Nível ${s.level} · XP ${((into / need) * 100).toFixed(1)}%`);
    } else {
      setBar(this.xp, 1, 1, `Nível ${s.level} · máximo`);
    }
    this.death.hidden = !s.dead;
    for (const slot of this.slots) slot.root.classList.toggle("no-mana", s.mp < slot.info.mpCost);
  }

  /** Recarga vinda do servidor. 0 ms com a skill na fila = a skill foi cancelada. */
  cooldown(skillId: number, readyInMs: number): void {
    const slot = this.slots.find((s) => s.info.id === skillId);
    if (!slot) return;
    slot.readyAt = performance.now() + readyInMs;
    slot.span = Math.max(readyInMs, slot.info.cooldownMs);
    if (this.queued === skillId) this.setQueued(0);
  }

  /** Começa a recarga na hora (skills em si mesmo e de área); o servidor corrige depois. */
  startLocalCooldown(skillId: number): void {
    const slot = this.slots.find((s) => s.info.id === skillId);
    if (!slot) return;
    slot.readyAt = performance.now() + slot.info.cooldownMs;
    slot.span = slot.info.cooldownMs;
  }

  /** Skill de alvo esperando o jogador chegar ao alcance (0 = nenhuma). */
  setQueued(skillId: number): void {
    this.queued = skillId;
    this.queuedAt = performance.now();
    for (const s of this.slots) s.root.classList.toggle("queued", s.info.id === skillId);
  }

  isReady(skillId: number): boolean {
    const slot = this.slots.find((s) => s.info.id === skillId);
    return !!slot && performance.now() >= slot.readyAt;
  }

  hasMana(skill: SkillInfo): boolean {
    return this.mpNow >= skill.mpCost;
  }

  setTarget(info: EntityInfo | undefined): void {
    if (!info) {
      this.targetFrame.hidden = true;
      this.targetEffects.set([]);
      return;
    }
    this.targetFrame.hidden = false;
    this.targetFrame.classList.toggle("enemy", info.kind === EntityKind.MONSTER);
    this.targetName.textContent = `${info.name}  ·  Nível ${info.level}${info.dead ? "  ·  morto" : ""}`;
    setBar(this.targetHp, info.hp, info.maxHp, `${info.hp} / ${info.maxHp}`);
  }

  setEffects(mine: ActiveEffect[], target: ActiveEffect[]): void {
    this.myEffects.set(mine);
    this.targetEffects.set(target);
  }

  private tick(): void {
    const now = performance.now();
    if (this.queued && now - this.queuedAt > QUEUE_TIMEOUT_MS) this.setQueued(0);
    const dist = this.cb.targetDistance();
    for (const s of this.slots) {
      const left = s.readyAt - now;
      if (left > 0) {
        const frac = Math.min(1, left / Math.max(1, s.span));
        s.sweep.style.setProperty("--done", `${((1 - frac) * 360).toFixed(1)}deg`);
        s.timer.textContent = left > 1000 ? String(Math.ceil(left / 1000)) : (left / 1000).toFixed(1);
        s.root.classList.add("cooling");
      } else if (s.root.classList.contains("cooling")) {
        s.timer.textContent = "";
        s.root.classList.remove("cooling");
        // Brilho rápido avisando que a skill voltou.
        s.root.classList.remove("ready-flash");
        void s.root.offsetWidth;
        s.root.classList.add("ready-flash");
      }
      const far = s.info.target === SkillTarget.ENEMY && dist !== null && dist > s.info.range;
      s.root.classList.toggle("far", far);
    }
  }

  private showTip(s: SkillInfo, anchor: HTMLElement): void {
    const t = this.tip;
    t.replaceChildren();
    const head = el("div", "tip-head");
    const icon = el("div", "tip-icon");
    icon.innerHTML = skillIcon(s.fx);
    const title = el("div", "tip-title");
    title.append(el("strong", "", s.name), el("span", "tip-kind", kindLabel(s)));
    head.append(icon, title);
    t.append(head);

    const stats = el("ul", "tip-stats");
    const stat = (k: string, v: string, cls = "") => {
      const li = el("li", cls);
      li.append(el("span", "", k), el("b", "", v));
      stats.append(li);
    };
    stat("Mana", `${s.mpCost}`, "mana");
    stat("Recarga", `${formatSeconds(s.cooldownMs)} s`);
    if (s.target === SkillTarget.ENEMY) stat("Alcance", `${formatSeconds(s.range * 1000)} m`);
    if (s.target === SkillTarget.AREA) stat("Raio", `${formatSeconds(s.range * 1000)} m`);
    if (s.power > 0) stat("Dano", `${Math.round(s.power * 100)}% do ataque ${s.magic ? "mágico" : "físico"}`);
    if (s.extraCrit > 0) stat("Crítico", `+${Math.round(s.extraCrit * 100)}% de chance`);
    t.append(stats);

    if (s.description) t.append(el("p", "tip-desc", s.description));
    if (s.effect !== Effect.UNSPECIFIED) {
      const fx = el("p", `tip-effect${isDebuff(s.effect) ? " debuff" : ""}`);
      const ic = el("span", "tip-effect-icon");
      ic.innerHTML = effectIcon(s.effect);
      fx.append(ic, el("span", "", (s.effectOnSelf ? "Em você: " : "No alvo: ") + effectText(s.effect, s.effectValue, s.effectMs)));
      t.append(fx);
    }
    t.append(el("p", "tip-key", `Tecla ${s.key}`));

    t.hidden = false;
    const r = anchor.getBoundingClientRect();
    const w = t.offsetWidth;
    const left = Math.max(8, Math.min(window.innerWidth - w - 8, r.left + r.width / 2 - w / 2));
    t.style.left = `${left}px`;
    t.style.bottom = `${window.innerHeight - r.top + 8}px`;
  }
}

function kindLabel(s: SkillInfo): string {
  switch (s.target) {
    case SkillTarget.SELF:
      return "Em si mesmo";
    case SkillTarget.AREA:
      return s.magic ? "Magia em área" : "Golpe em área";
    default:
      return s.magic ? "Magia à distância" : "Golpe corpo a corpo";
  }
}
