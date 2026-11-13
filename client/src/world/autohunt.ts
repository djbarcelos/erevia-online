/**
 * Auto-caça no cliente: escolhe o monstro vivo mais perto dentro do raio, manda atacar,
 * usa as skills da barra quando prontas e toma poção abaixo do limite. Manda só os mesmos
 * comandos que um jogador mandaria (o servidor valida tudo) e num ritmo baixo (4×/s).
 */

import { SkillTarget, type SelfStats, type SkillInfo } from "../gen/erevia/v1/game_pb";
import { settings } from "../ui/kit";

export interface AutoHuntDeps {
  dead(): boolean;
  stats(): SelfStats | null;
  /** Monstro vivo mais perto (distância em metros). */
  nearestMonster(): { id: number; dist: number } | null;
  targetId(): number;
  isEnemy(id: number): boolean;
  distanceTo(id: number): number | null;
  /** Seleciona e ataca. */
  attack(id: number): void;
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

  constructor(private d: AutoHuntDeps) {}

  start(): void {
    if (this.on) return;
    this.on = true;
    this.kills = 0;
    this.since = performance.now();
    this.xpStart = this.d.stats()?.xp ?? 0n;
    this.current = 0;
    this.timer = window.setInterval(() => this.tick(), 250);
  }

  stop(reason = ""): void {
    if (!this.on) return;
    this.on = false;
    window.clearInterval(this.timer);
    if (reason) this.d.onStop(reason);
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
    return `Auto-caça · ${this.kills} ${this.kills === 1 ? "abate" : "abates"} · ${Math.floor(secs / 60)}:${String(secs % 60).padStart(2, "0")} · +${Math.max(0, xp).toLocaleString("pt-BR")} XP`;
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

    // Alvo: mantém o atual se ainda for inimigo; senão o mais perto dentro do raio.
    let target = d.targetId();
    if (!d.isEnemy(target)) {
      if (s.autoLoot && d.lootNearby(10)) return; // pega o saque antes do próximo
      const n = d.nearestMonster();
      if (!n || n.dist > s.autoRadius) {
        this.current = 0;
        return;
      }
      target = n.id;
      this.current = target;
      d.attack(target);
      this.lastAttack = now;
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
