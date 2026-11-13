import type { TransformNode } from "@babylonjs/core";
import { TextBlock, type AdvancedDynamicTexture } from "@babylonjs/gui";

/**
 * Números de dano com estilo próprio por tipo, num pool de TextBlocks: nada é criado nem
 * descartado por número (os blocos ficam na GUI, escondidos, até o próximo golpe).
 *
 * Cores do guia de arte (docs/guia-de-arte.md, 3.2 e 3.3):
 * - golpe normal: Comum `#e8e1d4`; o seu um pouco maior que o dos outros;
 * - crítico: Lendário `#ff9a3c`, bem maior e com "!";
 * - cura: Bom `#7bd88f`;
 * - bloqueio (escudo segurou): Raro `#6fa8ff`;
 * - golpes do raid boss: Épico `#c08cff`;
 * - errou/esquivou: pedra quente `#c9b79c`, em itálico;
 * - dano em você: o vermelho dos estandartes (`#9e2b25`) clareado para ler sobre o cenário.
 */
export type NumberKind =
  /** Golpe dos outros (menor e mais apagado). */
  | "damage"
  /** Seu golpe normal. */
  | "mine"
  | "crit"
  /** Golpe de skill (a cor vem do tema da skill). */
  | "skill"
  | "heal"
  | "miss"
  | "dodge"
  | "block"
  /** Dano em você. */
  | "taken"
  /** Golpes de área do raid boss. */
  | "raid"
  /** Dano por segundo (veneno, queimadura, praga): a cor vem do efeito. */
  | "dot"
  /** Textos do jogo (item pego, "+XP"): cor e tamanho de quem chama. */
  | "info";

interface Style {
  size: number;
  weight: string;
  color: string;
  italic?: boolean;
  /** Salto no começo (1 = nenhum). */
  pop: number;
  /** Quanto sobe (px) e quanto dura (s). */
  rise: number;
  dur: number;
  alpha: number;
}

export const NUMBER_COLORS = {
  common: "#e8e1d4",
  legendary: "#ff9a3c",
  good: "#7bd88f",
  rare: "#6fa8ff",
  epic: "#c08cff",
  stone: "#c9b79c",
  taken: "#ff6b5a",
} as const;

const STYLES: Record<NumberKind, Style> = {
  damage: { size: 13, weight: "600", color: NUMBER_COLORS.common, pop: 1, rise: 40, dur: 0.8, alpha: 0.8 },
  mine: { size: 17, weight: "bold", color: NUMBER_COLORS.common, pop: 1.25, rise: 45, dur: 0.9, alpha: 1 },
  crit: { size: 26, weight: "bold", color: NUMBER_COLORS.legendary, pop: 1.7, rise: 55, dur: 1.1, alpha: 1 },
  skill: { size: 19, weight: "bold", color: NUMBER_COLORS.common, pop: 1.35, rise: 48, dur: 1, alpha: 1 },
  heal: { size: 16, weight: "bold", color: NUMBER_COLORS.good, pop: 1.15, rise: 40, dur: 1, alpha: 1 },
  miss: { size: 14, weight: "600", color: NUMBER_COLORS.stone, italic: true, pop: 1, rise: 30, dur: 0.8, alpha: 0.9 },
  dodge: { size: 15, weight: "600", color: NUMBER_COLORS.stone, italic: true, pop: 1.1, rise: 30, dur: 0.9, alpha: 1 },
  block: { size: 14, weight: "600", color: NUMBER_COLORS.rare, pop: 1.1, rise: 34, dur: 0.9, alpha: 1 },
  taken: { size: 17, weight: "bold", color: NUMBER_COLORS.taken, pop: 1.2, rise: 45, dur: 0.9, alpha: 1 },
  raid: { size: 18, weight: "bold", color: NUMBER_COLORS.epic, pop: 1.3, rise: 48, dur: 1, alpha: 1 },
  dot: { size: 14, weight: "600", color: NUMBER_COLORS.good, pop: 1, rise: 35, dur: 0.8, alpha: 0.95 },
  info: { size: 16, weight: "600", color: NUMBER_COLORS.common, pop: 1, rise: 45, dur: 0.9, alpha: 1 },
};

/** Números na tela ao mesmo tempo; passou disso, o mais antigo sai para o novo entrar. */
const MAX_ACTIVE = 40;

interface Live {
  tb: TextBlock;
  anchor: TransformNode;
  style: Style;
  t: number;
  drift: number;
  /** Degrau na pilha do mesmo personagem (números que saem juntos não se sobrepõem). */
  lane: number;
}

export class DamageNumbers {
  private free: TextBlock[] = [];
  private live: Live[] = [];

  constructor(private gui: AdvancedDynamicTexture) {}

  /** Quantos estão na tela (para o painel e testes). */
  get count(): number {
    return this.live.length;
  }

  show(anchor: TransformNode, kind: NumberKind, text: string, color?: string, size?: number): void {
    if (anchor.isDisposed()) return;
    if (this.live.length >= MAX_ACTIVE) this.release(0);
    const base = STYLES[kind];
    const style = color || size ? { ...base, color: color ?? base.color, size: size ?? base.size } : base;
    const tb = this.free.pop() ?? this.create();
    tb.text = text;
    tb.color = style.color;
    tb.fontSize = style.size;
    tb.fontWeight = style.weight;
    tb.fontStyle = style.italic ? "italic" : "";
    tb.alpha = style.alpha;
    tb.scaleX = tb.scaleY = style.pop;
    tb.isVisible = true;
    tb.linkWithMesh(anchor);
    let lane = 0;
    for (const l of this.live) if (l.anchor === anchor && l.t < 0.25) lane++;
    tb.linkOffsetY = -18 - Math.min(3, lane) * 16;
    tb.linkOffsetX = 0;
    this.live.push({ tb, anchor, style, t: 0, drift: (Math.random() - 0.5) * (kind === "crit" ? 18 : 30), lane: Math.min(3, lane) });
  }

  private create(): TextBlock {
    const tb = new TextBlock();
    tb.outlineWidth = 3;
    tb.outlineColor = "black";
    tb.resizeToFit = true;
    tb.isHitTestVisible = false;
    this.gui.addControl(tb);
    return tb;
  }

  private release(i: number): void {
    const l = this.live[i];
    this.live.splice(i, 1);
    l.tb.isVisible = false;
    l.tb.linkWithMesh(null);
    this.free.push(l.tb);
  }

  update(dt: number): void {
    for (let i = this.live.length - 1; i >= 0; i--) {
      const l = this.live[i];
      l.t += dt;
      const s = l.style;
      const k = l.t / s.dur;
      if (k >= 1 || l.anchor.isDisposed()) {
        this.release(i);
        continue;
      }
      // Salta no começo (mais nos golpes fortes), sobe e some no fim.
      const pop = 1 + (s.pop - 1) * Math.max(0, 1 - k * 6);
      l.tb.scaleX = l.tb.scaleY = pop;
      l.tb.linkOffsetY = -18 - l.lane * 16 - k * s.rise;
      l.tb.linkOffsetX = l.drift * k;
      l.tb.alpha = s.alpha * (k < 0.6 ? 1 : 1 - (k - 0.6) / 0.4);
    }
  }
}
