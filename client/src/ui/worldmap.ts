import { EntityKind, type MapZone } from "../gen/erevia/v1/game_pb";
import { LANDMARKS } from "../world/map-painter";
import type { MapMarker } from "../world/entities";
import { drawBossMarker, drawDot, drawPlayerArrow, MARKER_COLORS, type MapSource } from "./minimap";

const REDRAW_MS = 100;

/** Cor da zona de caça pela dificuldade (nível máximo). */
function tierColor(levelMax: number): string {
  if (levelMax <= 5) return "120,200,110";
  if (levelMax <= 12) return "230,180,80";
  return "230,90,70";
}

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls?: string, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

/** Texto com contorno escuro (legível sobre qualquer fundo). */
function label(ctx: CanvasRenderingContext2D, text: string, x: number, y: number, font: string, color: string): void {
  ctx.font = font;
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.lineJoin = "round";
  ctx.lineWidth = 4;
  ctx.strokeStyle = "rgba(12,8,4,0.85)";
  ctx.strokeText(text, x, y);
  ctx.fillStyle = color;
  ctx.fillText(text, x, y);
}

/** Mapa do mundo inteiro em tela cheia (tecla M). Norte em cima. */
export class WorldMap {
  readonly root = el("div", "worldmap");
  private canvas = el("canvas", "worldmap-canvas");
  private ctx = this.canvas.getContext("2d")!;
  private timer = 0;

  constructor(
    private src: MapSource,
    private zones: MapZone[],
    private safeRadius: number,
  ) {
    const panel = el("div", "worldmap-panel");
    panel.setAttribute("role", "dialog");
    panel.setAttribute("aria-label", "Mapa do mundo");
    const head = el("div", "worldmap-head");
    const close = el("button", "minimap-btn", "✕");
    close.type = "button";
    close.title = "Fechar (M ou Esc)";
    close.setAttribute("aria-label", "Fechar o mapa");
    close.onclick = () => this.close();
    head.append(el("h2", "", "Erevia"), close);

    const legend = el("ul", "worldmap-legend");
    const item = (cls: string, text: string) => {
      const li = el("li");
      li.append(el("i", cls), el("span", "", text));
      legend.append(li);
    };
    item("lg-you", "Você");
    item("lg-player", "Jogadores");
    item("lg-monster", "Monstros");
    item("lg-boss", "Chefe");
    item("lg-zone", "Zona de caça");

    panel.append(head, this.canvas, legend);
    this.root.append(panel);
    this.root.hidden = true;
    // Clicar fora do painel fecha.
    this.root.addEventListener("pointerdown", (ev) => {
      if (ev.target === this.root) this.close();
    });
    document.body.append(this.root);
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  toggle(): void {
    if (this.isOpen) this.close();
    else this.open();
  }

  open(): void {
    if (this.isOpen) return;
    this.root.hidden = false;
    this.draw();
    this.timer = window.setInterval(() => this.draw(), REDRAW_MS);
  }

  close(): void {
    if (!this.isOpen) return;
    this.root.hidden = true;
    window.clearInterval(this.timer);
  }

  dispose(): void {
    this.close();
    this.root.remove();
  }

  private draw(): void {
    const css = this.canvas.clientWidth || 600;
    const dpr = Math.min(2, window.devicePixelRatio || 1);
    const size = Math.round(css * dpr);
    if (this.canvas.width !== size) this.canvas.width = this.canvas.height = size;
    const ctx = this.ctx;
    const { world, mapCanvas } = this.src;
    const k = size / world.size; // pixels por metro
    const cx = (x: number) => (x + world.size / 2) * k;
    const cy = (z: number) => (world.size / 2 - z) * k;
    // Escala dos ícones e textos: acompanha a densidade da tela e encolhe em mapas pequenos.
    const u = dpr * Math.max(0.55, Math.min(1, css / 640));

    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.imageSmoothingEnabled = true;
    ctx.drawImage(mapCanvas, 0, 0, size, size);

    // Zonas de caça: anéis coloridos com a faixa de níveis.
    for (const z of this.zones) {
      if (z.fixed) continue;
      const rgb = tierColor(z.levelMax);
      ctx.beginPath();
      ctx.arc(cx(0), cy(0), z.rMax * k, 0, Math.PI * 2);
      ctx.arc(cx(0), cy(0), z.rMin * k, 0, Math.PI * 2, true);
      ctx.fillStyle = `rgba(${rgb},0.13)`;
      ctx.fill();
      ctx.setLineDash([6 * u, 5 * u]);
      ctx.lineWidth = 1.2 * u;
      ctx.strokeStyle = `rgba(${rgb},0.7)`;
      for (const r of [z.rMin, z.rMax]) {
        ctx.beginPath();
        ctx.arc(cx(0), cy(0), r * k, 0, Math.PI * 2);
        ctx.stroke();
      }
      ctx.setLineDash([]);
      // Rótulo entre a estrada leste e a diagonal nordeste (longe das estradas).
      const a = (28 * Math.PI) / 180;
      const r = (z.rMin + z.rMax) / 2;
      const x = cx(Math.cos(a) * r);
      const y = cy(Math.sin(a) * r);
      label(ctx, `Nv ${z.levelMin}–${z.levelMax}`, x, y - 7 * u, `700 ${13 * u}px system-ui, sans-serif`, `rgb(${rgb})`);
      label(ctx, z.name, x, y + 8 * u, `500 ${10 * u}px system-ui, sans-serif`, "#efe6d2");
    }

    // Zona segura (cidade).
    ctx.setLineDash([3 * u, 4 * u]);
    ctx.lineWidth = 1.2 * u;
    ctx.strokeStyle = "rgba(240,215,150,0.75)";
    ctx.beginPath();
    ctx.arc(cx(0), cy(0), this.safeRadius * k, 0, Math.PI * 2);
    ctx.stroke();
    ctx.setLineDash([]);
    label(ctx, "Cidade · zona segura", cx(0), cy(this.safeRadius + 5), `600 ${11 * u}px Georgia, serif`, "#f3d488");

    // Construções com nome.
    for (const o of world.objects) {
      const name = LANDMARKS[o.k];
      if (!name || o.k === "well") continue;
      label(ctx, name, cx(o.x), cy(o.z) + 12 * u, `600 ${10 * u}px system-ui, sans-serif`, "#fff1d6");
    }

    // Chefes (ponto fixo).
    for (const z of this.zones) {
      if (!z.fixed) continue;
      drawBossMarker(ctx, cx(z.x), cy(z.z), 7 * u);
      label(ctx, `${z.name} · Nv ${z.levelMin}`, cx(z.x), cy(z.z) - 15 * u, `700 ${11 * u}px system-ui, sans-serif`, "#e7b3ff");
    }

    // Entidades.
    let me: MapMarker | null = null;
    this.src.forEachMarker((m) => {
      if (m.local) {
        me = m;
        return;
      }
      if (m.boss) return; // já marcado pela zona fixa
      if (m.kind === EntityKind.MONSTER) drawDot(ctx, cx(m.x), cy(m.z), 2 * u, MARKER_COLORS.monster);
      else drawDot(ctx, cx(m.x), cy(m.z), 3 * u, MARKER_COLORS.player);
      if (m.target) {
        ctx.beginPath();
        ctx.arc(cx(m.x), cy(m.z), 5 * u, 0, Math.PI * 2);
        ctx.strokeStyle = "#fff6dc";
        ctx.lineWidth = 1.5 * u;
        ctx.stroke();
      }
    });
    const mine = me as MapMarker | null;
    if (mine) {
      ctx.save();
      ctx.translate(cx(mine.x), cy(mine.z));
      ctx.beginPath();
      ctx.arc(0, 0, 11 * u, 0, Math.PI * 2);
      ctx.fillStyle = "rgba(243,212,136,0.22)";
      ctx.fill();
      ctx.rotate(mine.heading);
      drawPlayerArrow(ctx, 8 * u);
      ctx.restore();
    }

    // Pontos cardeais.
    const pad = 14 * u;
    const font = `700 ${13 * u}px Georgia, serif`;
    label(ctx, "N", size / 2, pad, font, "#f3d488");
    label(ctx, "S", size / 2, size - pad, font, "#e6dcc8");
    label(ctx, "L", size - pad, size / 2, font, "#e6dcc8");
    label(ctx, "O", pad, size / 2, font, "#e6dcc8");
  }
}
