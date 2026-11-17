import { DynamicTexture, Texture, type Scene } from "@babylonjs/core";

/**
 * Atlas dos efeitos novos (faíscas de impacto, poeira dos passos e rastro da arma): uma
 * textura só, pintada em canvas na carga, com as células lado a lado numa linha. As
 * partículas escolhem a célula pela folha de sprites do Babylon e o rastro pelas UVs.
 *
 * Tudo é simétrico na vertical: assim não importa se a textura sobe invertida (invertY).
 */
export const ATLAS_CELL = 128;
export const ATLAS_CELLS = 4;

/** Células do atlas (índice da esquerda para a direita). */
export const CELL = {
  /** Risco de faísca (esticado na direção do voo). */
  spark: 0,
  /** Nuvem de poeira macia. */
  puff: 1,
  /** Brilho de quatro pontas (lasca de luz do impacto). */
  glint: 2,
  /** Degradê do rastro: forte à esquerda (o mais novo) e some à direita; faixa clara no meio. */
  trail: 3,
} as const;

/** Faixa de U (0–1) de uma célula, para quem mapeia UVs à mão (o rastro). */
export function cellU(cell: number): [number, number] {
  // Meio texel de margem: o filtro bilinear não puxa a célula vizinha.
  const pad = 0.5 / (ATLAS_CELL * ATLAS_CELLS);
  return [cell / ATLAS_CELLS + pad, (cell + 1) / ATLAS_CELLS - pad];
}

export function createFxAtlas(scene: Scene): Texture {
  const w = ATLAS_CELL * ATLAS_CELLS;
  const h = ATLAS_CELL;
  const tex = new DynamicTexture("fx-atlas", { width: w, height: h }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  ctx.clearRect(0, 0, w, h);
  const s = ATLAS_CELL;
  const cell = (i: number, paint: (c: CanvasRenderingContext2D) => void) => {
    ctx.save();
    ctx.beginPath();
    ctx.rect(i * s, 0, s, s);
    ctx.clip();
    ctx.translate(i * s, 0);
    paint(ctx);
    ctx.restore();
  };

  cell(CELL.spark, (c) => {
    const g = c.createLinearGradient(0, 0, 0, s);
    g.addColorStop(0, "rgba(255,255,255,0)");
    g.addColorStop(0.5, "rgba(255,255,255,1)");
    g.addColorStop(1, "rgba(255,255,255,0)");
    c.fillStyle = g;
    c.beginPath();
    c.ellipse(s / 2, s / 2, s * 0.08, s * 0.46, 0, 0, Math.PI * 2);
    c.fill();
  });

  cell(CELL.puff, (c) => {
    // Bolhas macias sobrepostas (posições fixas: o atlas sai igual em toda carga).
    const blobs = [
      [0.5, 0.5, 0.36, 0.5],
      [0.38, 0.44, 0.22, 0.35],
      [0.62, 0.46, 0.24, 0.35],
      [0.46, 0.6, 0.22, 0.3],
      [0.58, 0.58, 0.2, 0.3],
    ];
    for (const [x, y, r, a] of blobs) {
      const g = c.createRadialGradient(x * s, y * s, 0, x * s, y * s, r * s);
      g.addColorStop(0, `rgba(255,255,255,${a})`);
      g.addColorStop(1, "rgba(255,255,255,0)");
      c.fillStyle = g;
      c.fillRect(0, 0, s, s);
    }
  });

  cell(CELL.glint, (c) => {
    const h2 = s / 2;
    const g = c.createRadialGradient(h2, h2, 0, h2, h2, h2);
    g.addColorStop(0, "rgba(255,255,255,0.9)");
    g.addColorStop(0.3, "rgba(255,255,255,0.2)");
    g.addColorStop(0.6, "rgba(255,255,255,0)");
    c.fillStyle = g;
    c.fillRect(0, 0, s, s);
    c.fillStyle = "white";
    c.beginPath();
    c.moveTo(h2, 6);
    c.quadraticCurveTo(h2, h2, s - 6, h2);
    c.quadraticCurveTo(h2, h2, h2, s - 6);
    c.quadraticCurveTo(h2, h2, 6, h2);
    c.quadraticCurveTo(h2, h2, h2, 6);
    c.fill();
  });

  {
    // Some da esquerda (golpe agora) para a direita (o que já passou); mais claro no meio
    // da lâmina e macio nas bordas (simétrico na vertical). putImageData ignora o
    // translate/clip: vai direto na posição da célula.
    const img = ctx.createImageData(s, s);
    for (let y = 0; y < s; y++) {
      const v = Math.abs(y / (s - 1) - 0.5) * 2; // 0 no meio, 1 nas bordas
      const across = Math.max(0, 1 - v * v) * (0.55 + 0.45 * (1 - v));
      for (let x = 0; x < s; x++) {
        const age = x / (s - 1);
        const along = (1 - age) ** 1.6;
        const a = Math.round(255 * Math.min(1, along * across * 1.15));
        const i = (y * s + x) * 4;
        img.data[i] = img.data[i + 1] = img.data[i + 2] = 255;
        img.data[i + 3] = a;
      }
    }
    ctx.putImageData(img, CELL.trail * s, 0);
  }

  tex.update(false);
  tex.hasAlpha = true;
  tex.wrapU = tex.wrapV = Texture.CLAMP_ADDRESSMODE;
  return tex;
}
