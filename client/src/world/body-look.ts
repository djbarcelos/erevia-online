/**
 * Visual do corpo dos heróis: pele, cabelo, roupa, rosto e chapéu.
 *
 * A textura KayKit de cada classe é uma paleta 8×4 de faixas em degradê (128×256 px).
 * Mudar uma cor é repintar as faixas certas numa cópia da textura; as malhas não mudam.
 * O rosto é uma das 5 cabeças que o build-models.mjs põe em cada herói, e o chapéu é uma
 * malha separada que pode ser escondida.
 */

import { api } from "../api";

export interface BodyLook {
  /** 0 = original da classe; i = paleta[i-1]. */
  skin: number;
  hair: number;
  outfit: number;
  /** "" = cabeça da própria classe. */
  head: string;
  hideHat: boolean;
}

export interface Shade {
  name: string;
  top: string;
  bottom: string;
}

export interface BodyPalettes {
  skins: Shade[];
  hairs: Shade[];
  outfits: Shade[];
  heads: { id: string; name: string }[];
  hatClasses: string[];
  barberPrice: number;
  /** Cores de roupa do Bestiário (família completa), com índice próprio (100 em diante). */
  bestiaryOutfits?: (Shade & { index: number; family: string })[];
}

/** A cor de roupa do índice (as da criação e as do Bestiário). */
export function outfitShade(pal: BodyPalettes, i: number): Shade | undefined {
  return i <= pal.outfits.length ? pal.outfits[i - 1] : pal.bestiaryOutfits?.find((o) => o.index === i);
}

export const ORIGINAL_LOOK: BodyLook = { skin: 0, hair: 0, outfit: 0, head: "", hideHat: false };

/** Visual vindo do servidor (protocolo ou JSON), com os campos que faltarem no padrão. */
export function toBodyLook(l?: Partial<BodyLook> | null): BodyLook {
  return {
    skin: l?.skin ?? 0,
    hair: l?.hair ?? 0,
    outfit: l?.outfit ?? 0,
    head: l?.head ?? "",
    hideHat: !!l?.hideHat,
  };
}

/** Mesma regra do servidor: a cabeça da classe é "" e quem não tem chapéu não o esconde. */
export function normalLook(model: string, l: BodyLook, pal?: BodyPalettes | null): BodyLook {
  const out = { ...l };
  if (out.head === model) out.head = "";
  if (pal && !pal.hatClasses.includes(model)) out.hideHat = false;
  return out;
}

export function sameLook(a: BodyLook, b: BodyLook): boolean {
  return a.skin === b.skin && a.hair === b.hair && a.outfit === b.outfit && a.head === b.head && a.hideHat === b.hideHat;
}

/** Chave das cores (o que muda a textura). */
export function colorKey(model: string, l: BodyLook): string {
  return `${model}|${l.skin}|${l.hair}|${l.outfit}`;
}

export function needsRepaint(l: BodyLook): boolean {
  return l.skin > 0 || l.hair > 0 || l.outfit > 0;
}

let palettes: BodyPalettes | null = null;
let loading: Promise<BodyPalettes | null> | null = null;

/** Paletas já carregadas (ou null). */
export function cachedPalettes(): BodyPalettes | null {
  return palettes;
}

/** Paletas do servidor (GET /api/creation), carregadas uma vez. null se falhar. */
export function bodyPalettes(): Promise<BodyPalettes | null> {
  if (palettes) return Promise.resolve(palettes);
  loading ??= api
    .creation()
    .then((o) => (palettes = o as unknown as BodyPalettes))
    .catch(() => {
      loading = null;
      return null;
    });
  return loading;
}

export function setBodyPalettes(p: BodyPalettes): void {
  palettes = p;
}

/** Faixa da textura: linha (0–3, de cima), coluna (0–7) e brilho relativo da cor. */
type Cell = [row: number, col: number, k: number];

const SKIN_CELLS: Record<string, Cell[]> = {
  mage: [
    [0, 0, 1],
    [2, 7, 1], // mãos
  ],
};
const HAIR_CELLS: Cell[] = [[0, 1, 1]];
/** Cor de destaque da roupa de cada classe. */
const OUTFIT_CELLS: Record<string, Cell[]> = {
  knight: [[1, 0, 1]], // capa e roseta
  barbarian: [
    [0, 6, 1], // cinto e braçadeiras
    [2, 3, 0.6], // calça
  ],
  mage: [
    [1, 0, 1], // manto
    [1, 1, 1], // chapéu
    [1, 7, 0.45], // barra escura
  ],
  rogue: [
    [1, 1, 1], // capa
    [1, 0, 0.6], // túnica
  ],
};

function rgb(hex: string): [number, number, number] {
  const n = parseInt(hex.slice(1), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

/** Repinta as faixas de pele, cabelo e roupa em `data` (RGBA, linha 0 = topo da imagem). */
export function paintTexture(data: Uint8Array, w: number, h: number, model: string, look: BodyLook, pal: BodyPalettes): void {
  const cw = w / 8;
  const ch = h / 4;
  const fill = (cells: Cell[], shade: Shade | undefined) => {
    if (!shade) return;
    const top = rgb(shade.top);
    const bottom = rgb(shade.bottom);
    for (const [row, col, k] of cells) {
      const x0 = Math.round(col * cw);
      const x1 = Math.round((col + 1) * cw);
      const y0 = Math.round(row * ch);
      const y1 = Math.round((row + 1) * ch);
      for (let y = y0; y < y1; y++) {
        const t = (y - y0) / Math.max(1, y1 - y0 - 1);
        const r = Math.min(255, (top[0] + (bottom[0] - top[0]) * t) * k);
        const g = Math.min(255, (top[1] + (bottom[1] - top[1]) * t) * k);
        const b = Math.min(255, (top[2] + (bottom[2] - top[2]) * t) * k);
        let i = (y * w + x0) * 4;
        for (let x = x0; x < x1; x++, i += 4) {
          data[i] = r;
          data[i + 1] = g;
          data[i + 2] = b;
        }
      }
    }
  };
  if (look.skin > 0) fill(SKIN_CELLS[model] ?? [[0, 0, 1]], pal.skins[look.skin - 1]);
  if (look.hair > 0) fill(HAIR_CELLS, pal.hairs[look.hair - 1]);
  if (look.outfit > 0) fill(OUTFIT_CELLS[model] ?? [], outfitShade(pal, look.outfit));
}

/**
 * Remaster (texturas geradas, sem o atlas 8x4): a roupa é tingida por máscara de cor (o pano vermelho
 * do cavaleiro, mantendo a luz pintada) e a pele é um quadradinho no canto superior direito, onde
 * a cabeça por baixo do elmo está mapeada. Cabelo não existe nesses modelos.
 */
export function paintRemasterTexture(data: Uint8Array, w: number, h: number, look: BodyLook, pal: BodyPalettes): void {
  const outfit = look.outfit > 0 ? outfitShade(pal, look.outfit) : undefined;
  if (outfit) {
    const a = rgb(outfit.top);
    const b = rgb(outfit.bottom);
    const mid = [(a[0] + b[0]) / 2, (a[1] + b[1]) / 2, (a[2] + b[2]) / 2];
    for (let i = 0; i < w * h * 4; i += 4) {
      const r = data[i];
      const g = data[i + 1];
      const bl = data[i + 2];
      if (r < 70 || r < 1.4 * g || r < 1.4 * bl) continue; // só o pano vermelho
      const k = Math.min(1.6, (0.299 * r + 0.587 * g + 0.114 * bl) / 85);
      data[i] = Math.min(255, mid[0] * k);
      data[i + 1] = Math.min(255, mid[1] * k);
      data[i + 2] = Math.min(255, mid[2] * k);
    }
  }
  const skin = look.skin > 0 ? pal.skins[look.skin - 1] : undefined;
  if (skin) {
    const c = rgb(skin.top);
    for (let y = 0; y < Math.floor(h * 0.045); y++) {
      for (let x = Math.ceil(w * 0.955); x < w; x++) {
        const i = (y * w + x) * 4;
        data[i] = c[0];
        data[i + 1] = c[1];
        data[i + 2] = c[2];
      }
    }
  }
}
