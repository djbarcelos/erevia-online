import { EntityKind, SkillTarget, type SelfStats, type SkillInfo } from "../gen/erevia/v1/game_pb";
import type { EntityInfo } from "../world/entities";

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
  shade: HTMLDivElement;
  timer: HTMLSpanElement;
  readyAt: number; // performance.now() em ms
}

/** HUD do jogo (HTML sobre o canvas): barras, alvo, skills e tela de morte. */
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
  private mpNow = 0;

  constructor(
    skills: SkillInfo[],
    onSkill: (s: SkillInfo) => void,
    onRespawn: () => void,
  ) {
    // Barras do personagem (embaixo, ao centro)
    const me = el("div", "me-panel");
    const bars = el("div", "me-bars");
    bars.append(this.hp.root, this.mp.root);
    const top = el("div", "me-top");
    top.append(this.levelEl, bars);
    me.append(top, this.xp.root);

    // Skills
    const bar = el("div", "skillbar");
    for (const s of [...skills].sort((a, b) => a.key - b.key)) {
      const btn = el("button", "skill");
      btn.type = "button";
      btn.title = `${s.name} — ${s.mpCost} de mana, recarga ${(s.cooldownMs / 1000).toFixed(0)} s` +
        (s.target === SkillTarget.ENEMY ? `, alcance ${s.range} m` : "");
      const key = el("span", "skill-key", String(s.key));
      const name = el("span", "skill-name", s.name);
      const cost = el("span", "skill-cost", `${s.mpCost} MP`);
      const shade = el("div", "skill-shade");
      const timer = el("span", "skill-timer");
      btn.append(key, name, cost, shade, timer);
      btn.onclick = () => onSkill(s);
      bar.append(btn);
      this.slots.push({ info: s, root: btn, shade, timer, readyAt: 0 });
    }
    me.append(bar);

    // Alvo (topo)
    this.targetFrame.append(this.targetName, this.targetHp.root);
    this.targetFrame.hidden = true;

    // Morte
    const box = el("div", "death-box");
    box.append(el("h2", "", "Você morreu"), el("p", "", "Você perdeu um pouco de experiência."));
    const btn = el("button", "btn btn-gold", "Voltar à cidade");
    btn.type = "button";
    btn.onclick = onRespawn;
    box.append(btn);
    this.death.append(box);
    this.death.hidden = true;

    this.root.append(me, this.targetFrame, this.death);
    document.body.append(this.root);

    const loop = () => {
      this.tickCooldowns();
      requestAnimationFrame(loop);
    };
    requestAnimationFrame(loop);
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

  cooldown(skillId: number, readyInMs: number): void {
    const slot = this.slots.find((s) => s.info.id === skillId);
    if (slot) slot.readyAt = performance.now() + readyInMs;
  }

  isReady(skillId: number): boolean {
    const slot = this.slots.find((s) => s.info.id === skillId);
    return !slot || performance.now() >= slot.readyAt;
  }

  hasMana(skill: SkillInfo): boolean {
    return this.mpNow >= skill.mpCost;
  }

  setTarget(info: EntityInfo | undefined): void {
    if (!info) {
      this.targetFrame.hidden = true;
      return;
    }
    this.targetFrame.hidden = false;
    this.targetFrame.classList.toggle("enemy", info.kind === EntityKind.MONSTER);
    this.targetName.textContent = `${info.name}  ·  Nível ${info.level}${info.dead ? "  ·  morto" : ""}`;
    setBar(this.targetHp, info.hp, info.maxHp, `${info.hp} / ${info.maxHp}`);
  }

  private tickCooldowns(): void {
    const now = performance.now();
    for (const s of this.slots) {
      const left = s.readyAt - now;
      if (left > 0) {
        const frac = Math.min(1, left / Math.max(1, s.info.cooldownMs));
        s.shade.style.height = `${(frac * 100).toFixed(1)}%`;
        s.timer.textContent = left > 1000 ? String(Math.ceil(left / 1000)) : (left / 1000).toFixed(1);
        s.root.classList.add("cooling");
      } else if (s.root.classList.contains("cooling")) {
        s.shade.style.height = "0";
        s.timer.textContent = "";
        s.root.classList.remove("cooling");
      }
    }
  }
}
