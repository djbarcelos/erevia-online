/**
 * Auto-caça no cliente: escolhe o monstro vivo mais perto dentro do raio, manda atacar,
 * usa as skills da barra quando prontas e toma poção abaixo do limite. Manda só os mesmos
 * comandos que um jogador mandaria (o servidor valida tudo) e num ritmo baixo (4×/s).
 *
 * Com filtro (o "Caçar aqui" das missões, ver questgo.ts) só mira nos monstros com aqueles
 * nomes; ligada pelo F/AUTO não tem filtro e caça como sempre.
 *
 * As regras de alvo ficam em huntrules.ts (tunnel#129): não mira o que não vê, pula os
 * neutros (a não ser com "Atacar monstros neutros" ligado), prefere quem já briga com você
 * e larga o alvo que travou a caça (o atirador que recuou para onde não dá para ir).
 */

import { SkillTarget, type SelfStats, type SkillInfo } from "../gen/erevia/v1/game_pb";
import { settings } from "../ui/kit";
import { pickTarget, StallWatch, type HuntCandidate, type HuntRules } from "./huntrules";

export interface AutoHuntDeps {
  dead(): boolean;
  stats(): SelfStats | null;
  /** Monstros vivos na tela, com o que as regras de alvo precisam (huntrules.ts). */
  monsters(): HuntCandidate[];
  targetId(): number;
  distanceTo(id: number): number | null;
  /** Seleciona e ataca. */
  attack(id: number): void;
  /** Para de perseguir o alvo (fica onde está). */
  halt(): void;
  /** Skills da barra de atalhos, na ordem. */
  hotbarSkills(): SkillInfo[];
  skillReady(s: SkillInfo): boolean;
  useSkill(s: SkillInfo): void;
  /** Toma uma poção de vida/mana se houver e estiver pronta. false = não tinha. */
  drink(kind: "hp" | "mp"): "ok" | "cooldown" | "none";
  /** Pega o item do chão mais perto (se houver um seu, a poucos metros). */
  lootNearby(radius: number): boolean;
  goToTown(): void;
  onStop(reason: string): void;
}

export class AutoHunt {
  private timer = 0;
  private lastAttack = 0;
  private lastSkill = 0;
  private current = 0;
  on = false;
  kills = 0;
  since = 0;
  xpStart = 0n;
  /** Nomes dos monstros que valem como alvo (null = qualquer um). */
  filter: readonly string[] | null = null;
  private label = "";
  private watch = new StallWatch();

  constructor(private d: AutoHuntDeps) {}

  /** Liga; com `only`, só caça os monstros desses nomes (label vai na pílula). */
  start(only: { names: readonly string[]; label: string } | null = null): void {
    if (this.on) return;
    this.on = true;
    this.filter = only ? only.names : null;
    this.label = only?.label ?? "";
    this.kills = 0;
    this.since = performance.now();
    this.xpStart = this.d.stats()?.xp ?? 0n;
    this.current = 0;
    this.watch.reset();
    this.timer = window.setInterval(() => this.tick(), 250);
  }

  /** Ligada pelo "Caçar aqui" (com filtro), não pelo F/AUTO. */
  get questMode(): boolean {
    return this.on && this.filter !== null;
  }

  stop(reason = ""): void {
    if (!this.on) return;
    this.on = false;
    window.clearInterval(this.timer);
    if (reason) this.d.onStop(reason);
  }

  /** Um jogador atacou você: a auto-caça para (nunca luta contra jogadores). */
  onPlayerAttack(): void {
    if (this.on) this.stop("Auto-caça parou: um jogador atacou você!");
  }

  /** Um monstro morreu (para contar abates do alvo da auto-caça). */
  onDeath(id: number): void {
    if (this.on && id === this.current) {
      this.kills++;
      this.current = 0;
    }
  }

  status(): string {
    const secs = Math.floor((performance.now() - this.since) / 1000);
    const xp = Number((this.d.stats()?.xp ?? this.xpStart) - this.xpStart);
    return `${this.label ? `Caçar aqui: ${this.label}` : "Auto-caça"} · ${this.kills} ${this.kills === 1 ? "abate" : "abates"} · ${Math.floor(secs / 60)}:${String(secs % 60).padStart(2, "0")} · +${Math.max(0, xp).toLocaleString("pt-BR")} XP`;
  }

  private tick(): void {
    const d = this.d;
    const s = settings.get();
    if (d.dead()) return this.stop("Auto-caça parou: você morreu.");
    const st = d.stats();
    const now = performance.now();

    // Poções
    if (st && st.maxHp > 0 && st.hp / st.maxHp < s.autoPotion / 100) {
      const r = d.drink("hp");
      if (r === "none" && s.autoReturn) {
        this.stop("Auto-caça parou: acabaram as poções de vida. Voltando à cidade.");
        d.goToTown();
        return;
      }
    } else if (st && st.maxMp > 0 && st.mp / st.maxMp < 0.2) d.drink("mp");

    // Alvo: mantém o atual se ainda valer (vivo, visível, não neutro e sem travar); senão
    // quem briga com você ou o mais perto dentro do raio.
    const list = d.monsters();
    const rules: HuntRules = { names: this.filter, neutrals: s.autoNeutral, skip: this.watch.skips(now) };
    let target = d.targetId();
    const cur = target ? list.find((c) => c.id === target) : undefined;
    if (!cur || !pickTarget([cur], rules, Infinity)) {
      if (s.autoLoot && d.lootNearby(10)) return; // pega o saque antes do próximo
      const n = pickTarget(list, rules, s.autoRadius);
      if (!n) {
        this.current = 0;
        return;
      }
      target = n.id;
      this.current = target;
      d.attack(target);
      this.lastAttack = now;
      return;
    }
    // Travou (não perde vida e você não chega mais perto): larga e vai para outro, ou para.
    if (this.watch.check(target, cur.hp, cur.dist, now)) {
      this.current = 0;
      const next = pickTarget(list, { ...rules, skip: this.watch.skips(now) }, s.autoRadius);
      if (next) {
        this.current = next.id;
        d.attack(next.id);
        this.lastAttack = now;
      } else d.halt(); // ninguém mais: não fica correndo atrás do que travou
      return;
    }
    this.current = target;
    // Reforça o ataque de vez em quando (depois de uma skill o servidor pode trocar a ação).
    if (now - this.lastAttack > 2500) {
      d.attack(target);
      this.lastAttack = now;
    }

    // Skills: uma por segundo, na ordem da barra.
    if (!s.autoSkills || now - this.lastSkill < 1000) return;
    const dist = d.distanceTo(target) ?? 99;
    for (const sk of d.hotbarSkills()) {
      if (!d.skillReady(sk)) continue;
      if (st && st.mp < sk.mpCost) continue;
      if (sk.target === SkillTarget.AREA && dist > sk.range) continue;
      d.useSkill(sk);
      this.lastSkill = now;
      return;
    }
  }
}
