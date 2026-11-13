import type { MapMarker } from "../world/entities";
import type { WorldData } from "../world/world-data";

/** Fonte de dados do minimapa (HUD) e do mapa do mundo. */
export interface MapSource {
  world: WorldData;
  /** Mapa inteiro pintado uma vez (ver renderMapCanvas). */
  mapCanvas: HTMLCanvasElement;
  forEachMarker(cb: (m: MapMarker) => void): void;
  /** Para onde a câmera olha: 0 = norte, crescendo no sentido horário. */
  cameraHeading(): number;
}

export const MARKER_COLORS = {
  npc: "#ffd36b",
  monster: "#e0483c",
  player: "#5fa8ff",
  boss: "#c03be0",
  local: "#f3d488",
};

/** Seta do jogador apontando para cima (o chamador gira o contexto). */
export function drawPlayerArrow(ctx: CanvasRenderingContext2D, size: number): void {
  ctx.beginPath();
  ctx.moveTo(0, -size);
  ctx.lineTo(size * 0.72, size * 0.8);
  ctx.lineTo(0, size * 0.38);
  ctx.lineTo(-size * 0.72, size * 0.8);
  ctx.closePath();
  ctx.fillStyle = MARKER_COLORS.local;
  ctx.strokeStyle = "#1b1408";
  ctx.lineWidth = Math.max(1.5, size * 0.22);
  ctx.lineJoin = "round";
  ctx.stroke();
  ctx.fill();
}

/** Losango com contorno claro: marca de chefe. */
export function drawBossMarker(ctx: CanvasRenderingContext2D, x: number, y: number, r: number): void {
  ctx.beginPath();
  ctx.moveTo(x, y - r);
  ctx.lineTo(x + r, y);
  ctx.lineTo(x, y + r);
  ctx.lineTo(x - r, y);
  ctx.closePath();
  ctx.fillStyle = MARKER_COLORS.boss;
  ctx.strokeStyle = "#fff2d0";
  ctx.lineWidth = Math.max(1.2, r * 0.3);
  ctx.fill();
  ctx.stroke();
  ctx.fillStyle = "#1a0e1e";
  ctx.beginPath();
  ctx.arc(x, y, r * 0.3, 0, Math.PI * 2);
  ctx.fill();
}

export function drawDot(ctx: CanvasRenderingContext2D, x: number, y: number, r: number, color: string): void {
  ctx.beginPath();
  ctx.arc(x, y, r, 0, Math.PI * 2);
  ctx.fillStyle = color;
  ctx.fill();
  ctx.lineWidth = Math.max(1, r * 0.45);
  ctx.strokeStyle = "rgba(10,6,4,0.85)";
  ctx.stroke();
}

/**
 * Mini mapa redondo no canto superior direito. O norte fica sempre em cima: o fundo só
 * se desloca com o jogador, e a seta (jogador) e o cone (câmera) é que giram.
 */
