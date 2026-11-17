import type { ParticleSystem } from "@babylonjs/core";
import type { Quality } from "../../ui/kit";
import { activeQuality } from "../render";

/**
 * Orçamento global de partículas (tunnel#101). Todo efeito pede partículas aqui antes de
 * nascer; o que é do próprio jogador (ou acontece com ele) sempre passa, e o que é dos
 * outros encolhe ou é descartado quando a tela já está cheia. Assim o pico de partículas
 * na tela tem teto, mesmo no raid boss com o grupo cheio.
 *
 * Conta só os efeitos do Fx (skills, golpes, buffs, auras). As tochas têm partículas
 * fixas e baratas, criadas uma vez (torches.ts), e ficam de fora.
 *
 * URL (para medir): ?fxbudget=off desliga o teto e os multiplicadores (o comportamento de
 * antes, para comparar o pico no painel F3).
 */

/** De quem é o efeito: do jogador local (ou nele) ou de outra pessoa/monstro. */
export type FxOwner = "mine" | "other";

export interface FxPreset {
  /** Partículas vivas dos efeitos acima das quais os efeitos dos outros encolhem. */
  budget: number;
  /** Multiplicador da quantidade de partículas de cada efeito: o seu e o dos outros. */
  mine: number;
  other: number;
  /** Poeira nos passos do próprio jogador. */
  dust: boolean;
  /** Rastro da arma: o seu e o dos outros. */
  trailMine: boolean;
  trailOthers: boolean;
  /** Marcas no chão das skills de área dos outros (as suas sempre aparecem). */
  marksOthers: boolean;
}

/**
 * Por preset (Configurações → Gráficos, ou ?quality=). O baixo fica mais barato que antes:
 * os efeitos vêm com menos partículas, sem poeira e sem rastro de arma, e com teto baixo.
 */
export const FX_PRESETS: Record<Quality, FxPreset> = {
  baixa: { budget: 400, mine: 0.75, other: 0.5, dust: false, trailMine: false, trailOthers: false, marksOthers: false },
  media: { budget: 900, mine: 1, other: 0.8, dust: true, trailMine: true, trailOthers: false, marksOthers: true },
  alta: { budget: 1400, mine: 1, other: 1, dust: true, trailMine: true, trailOthers: true, marksOthers: true },
};

/** Sem teto (?fxbudget=off): o de antes, para comparar. */
const UNLIMITED: FxPreset = { budget: Infinity, mine: 1, other: 1, dust: true, trailMine: true, trailOthers: true, marksOthers: true };

/** Mínimo que um efeito encolhido ainda precisa ter para valer a pena (senão é descartado). */
const MIN_SHARE = 0.3;
/** Encolhido: as partículas também vivem menos. */
export const SHORT_LIFE = 0.7;

interface Stream {
  ps: ParticleSystem;
  base: number;
  owner: FxOwner;
}

export interface Grant {
  count: number;
  /** Encolhido pelo orçamento: vida mais curta também. */
  short: boolean;
}

export class ParticleBudget {
  private systems = new Set<ParticleSystem>();
  private streams = new Map<ParticleSystem, Stream>();
  private readonly off = new URLSearchParams(location.search).get("fxbudget") === "off";
  private quality: Quality = "alta";
  private p: FxPreset = FX_PRESETS.alta;
  /** Partículas vivas no último quadro e as pedidas neste quadro (ainda não nasceram). */
  private live = 0;
  private pending = 0;
  /**
   * Quanto os fluxos contínuos dos outros (buffs, auras, rastros de projétil) emitem
   * (0–1): cai rápido quando a tela passa do teto e volta devagar quando sobra espaço.
   */
  private othersRate = 1;
  // Números do painel F3 (janela de 1 s).
  private peak = 0;
  private cuts = 0;
  private shownPeak = 0;
  private shownCuts = 0;
  private windowEnd = 0;
  private row: HTMLElement | null = null;
  readonly stats = { live: 0, budget: 0, peak: 0, cuts: 0, quality: "alta" as string, othersRate: 1, maxPeak: 0 };

  constructor() {
    this.refreshPreset();
    (window as unknown as { __ereviaFx?: unknown }).__ereviaFx = this.stats;
  }

  get preset(): FxPreset {
    return this.p;
  }

  /** Há folga para enfeites dos outros (marcas, rastros)? */
  get relaxed(): boolean {
    return this.othersRate > 0.5;
  }

  private refreshPreset(): void {
    this.quality = activeQuality();
    this.p = this.off ? UNLIMITED : FX_PRESETS[this.quality];
  }

  track(ps: ParticleSystem): void {
    this.systems.add(ps);
  }

  untrack(ps: ParticleSystem): void {
    this.systems.delete(ps);
    this.streams.delete(ps);
  }

  /**
   * Quantas partículas uma rajada pode ter. As suas sempre saem (com o multiplicador do
   * preset); as dos outros cabem na folga ou são encolhidas, e somem se não couber nem um
   * terço.
   */
  grant(count: number, owner: FxOwner): Grant {
    const p = this.p;
    if (owner === "mine") {
      const n = Math.max(1, Math.round(count * p.mine));
      this.pending += n;
      return { count: n, short: false };
    }
    const want = Math.max(1, Math.round(count * p.other));
    const room = p.budget - this.live - this.pending;
    if (want <= room) {
      this.pending += want;
      return { count: want, short: false };
    }
    this.cuts++;
    if (room >= Math.max(3, want * MIN_SHARE)) {
      const n = Math.floor(room);
      this.pending += n;
      return { count: n, short: true };
    }
    return { count: 0, short: true };
  }

  /** Fluxo contínuo: a taxa é ajustada a cada quadro (os dos outros seguem a pressão). */
  stream(ps: ParticleSystem, rate: number, owner: FxOwner): void {
    this.streams.set(ps, { ps, base: rate, owner });
    ps.emitRate = this.rateOf(rate, owner);
  }

  private rateOf(base: number, owner: FxOwner): number {
    return owner === "mine" ? base * this.p.mine : base * this.p.other * this.othersRate;
  }

  /** Começo de cada quadro: mede o que está vivo e ajusta os fluxos dos outros. */
  tick(dt: number, now: number): void {
    this.refreshPreset();
    let live = 0;
    for (const ps of this.systems) live += ps.getActiveCount();
    this.live = live;
    this.pending = 0;
    const limit = this.p.budget;
    const before = this.othersRate;
    if (live > limit) this.othersRate = Math.max(0, this.othersRate - dt * 4);
    else if (live < limit * 0.75) this.othersRate = Math.min(1, this.othersRate + dt * 0.5);
    if (this.othersRate !== before || this.streams.size) {
      for (const s of this.streams.values()) {
        // Um fluxo parado (ps.stop) fica com a taxa que tinha: o Babylon já não emite.
        if (s.ps.isStarted()) s.ps.emitRate = this.rateOf(s.base, s.owner);
      }
    }
    this.peak = Math.max(this.peak, live);
    if (now >= this.windowEnd) {
      this.shownPeak = this.peak;
      this.shownCuts = this.cuts;
      this.peak = 0;
      this.cuts = 0;
      this.windowEnd = now + 1000;
      this.updatePanel();
    }
    const st = this.stats;
    st.live = live;
    st.budget = limit;
    st.peak = this.shownPeak;
    st.cuts = this.shownCuts;
    st.quality = this.off ? `${this.quality}, sem teto` : this.quality;
    st.othersRate = this.othersRate;
    st.maxPeak = Math.max(st.maxPeak, live);
  }

  /**
   * Linha "Partículas" no painel F3 (ui/perf.ts): vivas / teto (preset) · pico do último
   * segundo · efeitos dos outros cortados ou encolhidos no último segundo.
   */
  private updatePanel(): void {
    const box = document.querySelector<HTMLElement>("#hud .perf");
    if (!box || box.hidden) return;
    if (!this.row || !box.contains(this.row)) {
      const r = document.createElement("div");
      const k = document.createElement("span");
      k.className = "k";
      k.textContent = "Partículas";
      const v = document.createElement("span");
      r.append(k, " ", v);
      box.appendChild(r);
      this.row = v;
    }
    const budget = Number.isFinite(this.p.budget) ? String(this.p.budget) : "∞";
    const st = this.stats;
    this.row.textContent = `${this.live} / ${budget} (${st.quality}) · pico ${this.shownPeak} · cortes ${this.shownCuts}`;
  }
}
