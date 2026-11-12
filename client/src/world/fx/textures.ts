import { DynamicTexture, Texture, type Scene } from "@babylonjs/core";
import { mulberry32 } from "../map-painter";

/**
 * Texturas dos efeitos, pintadas uma vez em canvas (brancas/cinza: a cor vem das
 * partículas e dos materiais). Nada de arquivos de arte.
 */
export interface FxTextures {
  /** Brilho redondo e macio. */
  flare: Texture;
  /** Risco fino (faíscas esticadas). */
  spark: Texture;
  /** Nuvem de fumaça. */
  smoke: Texture;
  /** Floco de gelo de seis pontas. */
  flake: Texture;
  /** Estrela de quatro pontas. */
  star: Texture;
  /** Anel fino de borda macia (ondas de choque). */
  ring: Texture;
  /** Anel de gelo com cristais. */
  frostRing: Texture;
  /** Círculo de runas. */
  runes: Texture;
  /** Marca de queimado. */
  scorch: Texture;
  /** Rachaduras no chão. */
  crack: Texture;
  /** Meia-lua de corte. */
  crescent: Texture;
  /** Chama (lágrima) para partículas de fogo. */
  flame: Texture;
}

type Paint = (ctx: CanvasRenderingContext2D, s: number) => void;

function make(scene: Scene, name: string, size: number, paint: Paint): Texture {
  const tex = new DynamicTexture(`fx-${name}`, { width: size, height: size }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  ctx.clearRect(0, 0, size, size);
  paint(ctx, size);
  tex.update(false);
  tex.hasAlpha = true;
  tex.wrapU = tex.wrapV = Texture.CLAMP_ADDRESSMODE;
  return tex;
}

function radial(ctx: CanvasRenderingContext2D, s: number, stops: [number, string][]): void {
  const g = ctx.createRadialGradient(s / 2, s / 2, 0, s / 2, s / 2, s / 2);
  for (const [o, c] of stops) g.addColorStop(o, c);
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, s, s);
}

export function createFxTextures(scene: Scene): FxTextures {
  const rand = mulberry32(4242);
  return {
    flare: make(scene, "flare", 128, (c, s) =>
      radial(c, s, [
        [0, "rgba(255,255,255,1)"],
        [0.18, "rgba(255,255,255,0.85)"],
        [0.45, "rgba(255,255,255,0.25)"],
        [1, "rgba(255,255,255,0)"],
      ]),
    ),

    spark: make(scene, "spark", 64, (c, s) => {
      const g = c.createLinearGradient(0, 0, 0, s);
      g.addColorStop(0, "rgba(255,255,255,0)");
      g.addColorStop(0.5, "rgba(255,255,255,1)");
      g.addColorStop(1, "rgba(255,255,255,0)");
      c.fillStyle = g;
      c.beginPath();
      c.ellipse(s / 2, s / 2, s * 0.09, s * 0.48, 0, 0, Math.PI * 2);
      c.fill();
    }),

    smoke: make(scene, "smoke", 128, (c, s) => {
      for (let i = 0; i < 14; i++) {
        const x = s / 2 + (rand() - 0.5) * s * 0.4;
        const y = s / 2 + (rand() - 0.5) * s * 0.4;
        const r = s * (0.18 + rand() * 0.2);
        const g = c.createRadialGradient(x, y, 0, x, y, r);
        g.addColorStop(0, `rgba(255,255,255,${0.22 + rand() * 0.2})`);
        g.addColorStop(1, "rgba(255,255,255,0)");
        c.fillStyle = g;
        c.fillRect(0, 0, s, s);
      }
    }),

    flake: make(scene, "flake", 64, (c, s) => {
      c.translate(s / 2, s / 2);
      c.strokeStyle = "rgba(255,255,255,1)";
      c.lineCap = "round";
      c.shadowColor = "rgba(255,255,255,0.9)";
      c.shadowBlur = 4;
      for (let i = 0; i < 6; i++) {
        c.rotate(Math.PI / 3);
        c.lineWidth = 3;
        c.beginPath();
        c.moveTo(0, 0);
        c.lineTo(0, -s * 0.42);
        c.stroke();
        c.lineWidth = 2;
        c.beginPath();
        c.moveTo(0, -s * 0.24);
        c.lineTo(-s * 0.09, -s * 0.33);
        c.moveTo(0, -s * 0.24);
        c.lineTo(s * 0.09, -s * 0.33);
        c.stroke();
      }
    }),

    star: make(scene, "star", 64, (c, s) => {
      radial(c, s, [
        [0, "rgba(255,255,255,0.9)"],
        [0.25, "rgba(255,255,255,0.15)"],
        [0.5, "rgba(255,255,255,0)"],
      ]);
      c.fillStyle = "white";
      c.beginPath();
      const h = s / 2;
      c.moveTo(h, 2);
      c.quadraticCurveTo(h, h, s - 2, h);
      c.quadraticCurveTo(h, h, h, s - 2);
      c.quadraticCurveTo(h, h, 2, h);
      c.quadraticCurveTo(h, h, h, 2);
      c.fill();
    }),

    ring: make(scene, "ring", 256, (c, s) =>
      radial(c, s, [
        [0, "rgba(255,255,255,0)"],
        [0.72, "rgba(255,255,255,0)"],
        [0.86, "rgba(255,255,255,0.55)"],
        [0.93, "rgba(255,255,255,1)"],
        [1, "rgba(255,255,255,0)"],
      ]),
    ),

    frostRing: make(scene, "frostRing", 512, (c, s) => {
      radial(c, s, [
        [0, "rgba(255,255,255,0.08)"],
        [0.6, "rgba(255,255,255,0.18)"],
        [0.88, "rgba(255,255,255,0.75)"],
        [0.95, "rgba(255,255,255,0.4)"],
        [1, "rgba(255,255,255,0)"],
      ]);
      // Cristais apontando para fora ao longo da borda.
      c.translate(s / 2, s / 2);
      c.fillStyle = "rgba(255,255,255,0.95)";
      for (let i = 0; i < 46; i++) {
        const a = (i / 46) * Math.PI * 2 + rand() * 0.08;
        const r0 = s * (0.36 + rand() * 0.06);
        const r1 = s * (0.46 + rand() * 0.035);
        const w = 0.025 + rand() * 0.02;
        c.beginPath();
        c.moveTo(Math.cos(a - w) * r0, Math.sin(a - w) * r0);
        c.lineTo(Math.cos(a) * r1, Math.sin(a) * r1);
        c.lineTo(Math.cos(a + w) * r0, Math.sin(a + w) * r0);
        c.closePath();
        c.fill();
      }
      // Rachaduras de gelo para dentro.
      c.strokeStyle = "rgba(255,255,255,0.45)";
      c.lineWidth = 2;
      for (let i = 0; i < 18; i++) {
        let a = rand() * Math.PI * 2;
        let r = s * 0.4;
        c.beginPath();
        c.moveTo(Math.cos(a) * r, Math.sin(a) * r);
        for (let k = 0; k < 4; k++) {
          a += (rand() - 0.5) * 0.25;
          r -= s * (0.03 + rand() * 0.04);
          c.lineTo(Math.cos(a) * r, Math.sin(a) * r);
        }
        c.stroke();
      }
    }),

    runes: make(scene, "runes", 256, (c, s) => {
      c.translate(s / 2, s / 2);
      c.strokeStyle = "white";
      c.lineWidth = 3;
      for (const r of [0.47, 0.37]) {
        c.beginPath();
        c.arc(0, 0, s * r, 0, Math.PI * 2);
        c.stroke();
      }
      c.lineWidth = 2;
      // Glifos simples (traços) entre os dois círculos.
      for (let i = 0; i < 12; i++) {
        c.save();
        c.rotate((i / 12) * Math.PI * 2);
        c.translate(0, -s * 0.42);
        c.beginPath();
        const kind = i % 4;
        if (kind === 0) {
          c.moveTo(-6, -6);
          c.lineTo(0, 6);
          c.lineTo(6, -6);
        } else if (kind === 1) {
          c.moveTo(0, -7);
          c.lineTo(0, 7);
          c.moveTo(-5, 0);
          c.lineTo(5, 0);
        } else if (kind === 2) {
          c.arc(0, 0, 5, 0, Math.PI * 2);
        } else {
          c.moveTo(-6, 6);
          c.lineTo(0, -6);
          c.lineTo(6, 6);
          c.moveTo(-3, 1);
          c.lineTo(3, 1);
        }
        c.stroke();
        c.restore();
      }
      // Estrela de seis pontas no meio.
      c.lineWidth = 2.5;
      for (const off of [0, Math.PI / 3]) {
        c.beginPath();
        for (let i = 0; i <= 3; i++) {
          const a = off + (i / 3) * Math.PI * 2 - Math.PI / 2;
          const x = Math.cos(a) * s * 0.33;
          const y = Math.sin(a) * s * 0.33;
          if (i === 0) c.moveTo(x, y);
          else c.lineTo(x, y);
        }
        c.stroke();
      }
    }),

    scorch: make(scene, "scorch", 256, (c, s) => {
      for (let i = 0; i < 26; i++) {
        const a = rand() * Math.PI * 2;
        const d = rand() * s * 0.22;
        const x = s / 2 + Math.cos(a) * d;
        const y = s / 2 + Math.sin(a) * d;
        const r = s * (0.08 + rand() * 0.16);
        const g = c.createRadialGradient(x, y, 0, x, y, r);
        g.addColorStop(0, "rgba(20,12,8,0.55)");
        g.addColorStop(1, "rgba(20,12,8,0)");
        c.fillStyle = g;
        c.fillRect(0, 0, s, s);
      }
    }),

    crack: make(scene, "crack", 256, (c, s) => {
      c.translate(s / 2, s / 2);
      c.strokeStyle = "rgba(25,15,10,0.85)";
      c.lineCap = "round";
      for (let i = 0; i < 9; i++) {
        let a = (i / 9) * Math.PI * 2 + rand() * 0.4;
        let x = 0;
        let y = 0;
        c.lineWidth = 5;
        c.beginPath();
        c.moveTo(0, 0);
        const len = s * (0.25 + rand() * 0.2);
        const steps = 5;
        for (let k = 0; k < steps; k++) {
          a += (rand() - 0.5) * 0.7;
          x += (Math.cos(a) * len) / steps;
          y += (Math.sin(a) * len) / steps;
          c.lineTo(x, y);
          c.lineWidth = Math.max(1, 5 - k);
        }
        c.stroke();
      }
    }),

    crescent: make(scene, "crescent", 256, (c, s) => {
      // Rastro de corte: arco de ~150° que começa fino e transparente (rastro) e termina
      // grosso e claro (a lâmina).
      c.translate(s / 2, s / 2);
      c.lineCap = "round";
      const r = s * 0.38;
      const a0 = Math.PI * 0.95;
      const a1 = Math.PI * 1.85;
      const steps = 70;
      for (let i = 0; i < steps; i++) {
        const t = i / (steps - 1);
        const a = a0 + (a1 - a0) * t;
        const b = a + (a1 - a0) / steps + 0.01;
        c.strokeStyle = `rgba(255,255,255,${(0.08 + 0.92 * t ** 1.6).toFixed(3)})`;
        c.lineWidth = 2 + 26 * t ** 1.4;
        c.beginPath();
        c.arc(0, 0, r, a, b);
        c.stroke();
      }
      // Fio claro na borda externa.
      c.strokeStyle = "rgba(255,255,255,0.9)";
      c.lineWidth = 2;
      c.beginPath();
      c.arc(0, 0, r + 12, a0 + 0.5, a1);
      c.stroke();
    }),

    flame: make(scene, "flame", 64, (c, s) => {
      const g = c.createRadialGradient(s / 2, s * 0.62, 0, s / 2, s * 0.62, s * 0.42);
      g.addColorStop(0, "rgba(255,255,255,1)");
      g.addColorStop(0.5, "rgba(255,255,255,0.55)");
      g.addColorStop(1, "rgba(255,255,255,0)");
      c.fillStyle = g;
      c.beginPath();
      c.moveTo(s / 2, s * 0.05);
      c.bezierCurveTo(s * 0.8, s * 0.4, s * 0.9, s * 0.95, s / 2, s * 0.95);
      c.bezierCurveTo(s * 0.1, s * 0.95, s * 0.2, s * 0.4, s / 2, s * 0.05);
      c.fill();
    }),
  };
}
