import { EntityKind } from "../gen/erevia/v1/game_pb";
import type { MapMarker } from "../world/entities";
import type { WorldData } from "../world/world-data";

/** Fonte de dados compartilhada pelo mini mapa e pelo mapa grande. */
export interface MapSource {
  world: WorldData;
  /** Mapa inteiro pintado uma vez (ver renderMapCanvas). */
  mapCanvas: HTMLCanvasElement;
  forEachMarker(cb: (m: MapMarker) => void): void;
  /** Para onde a câmera olha: 0 = norte, crescendo no sentido horário. */
  cameraHeading(): number;
}

/** Raios de visão disponíveis no zoom (metros do centro até a borda). */
const ZOOM_STEPS = [30, 45, 60, 90, 120];
const ZOOM_KEY = "erevia.minimap.radius";
const REDRAW_MS = 66; // ~15 Hz

export const MARKER_COLORS = {
  monster: "#e0483c",
  player: "#5fa8ff",
  boss: "#c03be0",
  local: "#f3d488",
};

function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls?: string, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

function loadZoom(): number {
  try {
    const v = Number(localStorage.getItem(ZOOM_KEY));
    if (ZOOM_STEPS.includes(v)) return v;
  } catch {
    // armazenamento bloqueado: usa o padrão
  }
  return 60;
}

function saveZoom(v: number): void {
  try {
    localStorage.setItem(ZOOM_KEY, String(v));
  } catch {
    // ignora
  }
}

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
export class Minimap {
  readonly root = el("div", "minimap");
  private canvas = el("canvas", "minimap-canvas");
  private ctx = this.canvas.getContext("2d")!;
  private coords = el("span", "minimap-coords", "–");
  private clockIcon = el("span", "minimap-clock-icon");
  private clockText = el("span", "minimap-clock-text", "");
  private clock = el("div", "minimap-clock");
  private help = el("div", "minimap-help");
  private radius = loadZoom();
  private night = 0;
  private timer: number;

  constructor(
    private src: MapSource,
    onOpenMap: () => void,
  ) {
    const frame = el("div", "minimap-frame");
    frame.append(this.canvas, el("span", "minimap-n", "N"));
    this.clock.append(this.clockIcon, this.clockText);
    this.clock.hidden = true;
    frame.append(this.clock);

    const zoomOut = el("button", "minimap-btn", "−");
    zoomOut.type = "button";
    zoomOut.title = "Afastar";
    zoomOut.setAttribute("aria-label", "Afastar o mini mapa");
    zoomOut.onclick = () => this.zoom(1);
    const zoomIn = el("button", "minimap-btn", "+");
    zoomIn.type = "button";
    zoomIn.title = "Aproximar";
    zoomIn.setAttribute("aria-label", "Aproximar o mini mapa");
    zoomIn.onclick = () => this.zoom(-1);
    const mapBtn = el("button", "minimap-btn wide", "Mapa (M)");
    mapBtn.type = "button";
    mapBtn.onclick = onOpenMap;
    const helpBtn = el("button", "minimap-btn", "?");
    helpBtn.type = "button";
    helpBtn.title = "Controles";
    helpBtn.setAttribute("aria-label", "Mostrar controles");
    helpBtn.setAttribute("aria-expanded", "false");
    helpBtn.onclick = () => {
      this.help.hidden = !this.help.hidden;
      helpBtn.setAttribute("aria-expanded", String(!this.help.hidden));
    };

    const bar = el("div", "minimap-bar");
    bar.append(zoomOut, this.coords, zoomIn);
    const actions = el("div", "minimap-bar");
    actions.append(mapBtn, helpBtn);

    const list = el("ul");
    for (const [k, v] of [
      ["Clique", "andar / atacar"],
      ["Botão direito", "girar a câmera"],
      ["Roda", "zoom"],
      ["1, 2", "skills"],
      ["M", "mapa"],
      ["Enter", "chat"],
      ["Esc", "limpar alvo"],
    ]) {
      const li = el("li");
      li.append(el("kbd", "", k), el("span", "", v));
      list.append(li);
    }
    this.help.append(list);
    this.help.hidden = true;

    this.root.append(frame, bar, actions, this.help);
    document.body.append(this.root);

    this.canvas.addEventListener(
      "wheel",
      (ev) => {
        ev.preventDefault();
        this.zoom(ev.deltaY > 0 ? 1 : -1);
      },
      { passive: false },
    );

    this.timer = window.setInterval(() => this.draw(), REDRAW_MS);
  }

  /** Relógio do jogo (Entrega do ciclo dia/noite) e escurecimento do mini mapa à noite (0–1). */
  setClock(label: string, isNight: boolean, darkness: number): void {
    this.clock.hidden = false;
    this.clockText.textContent = label;
    this.clockIcon.textContent = isNight ? "☾" : "☀";
    this.clock.classList.toggle("night", isNight);
    this.night = darkness;
  }

  dispose(): void {
    window.clearInterval(this.timer);
    this.root.remove();
  }

  private zoom(dir: number): void {
    const i = ZOOM_STEPS.indexOf(this.radius);
    const next = ZOOM_STEPS[Math.max(0, Math.min(ZOOM_STEPS.length - 1, i + dir))];
    if (next === this.radius) return;
    this.radius = next;
    saveZoom(next);
    this.draw();
  }

  private draw(): void {
    if (!this.root.isConnected) {
      window.clearInterval(this.timer);
      return;
    }
    // Resolução do canvas acompanha o tamanho na tela (muda com o CSS em telas pequenas).
    const css = this.canvas.clientWidth || 180;
    const size = Math.round(css * Math.min(2, window.devicePixelRatio || 1));
    if (this.canvas.width !== size) this.canvas.width = this.canvas.height = size;
    const ctx = this.ctx;
    const R = size / 2;
    const scale = R / this.radius; // pixels do canvas por metro
    const unit = size / 180; // tamanho dos ícones independe da resolução

    let me: MapMarker | null = null;
    const others: MapMarker[] = [];
    this.src.forEachMarker((m) => {
      if (m.local) me = m;
      else others.push(m);
    });
    const meX = (me as MapMarker | null)?.x ?? 0;
    const meZ = (me as MapMarker | null)?.z ?? 0;
    const heading = (me as MapMarker | null)?.heading ?? 0;
    this.coords.textContent = `${meX.toFixed(0)}, ${meZ.toFixed(0)}`;

    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.clearRect(0, 0, size, size);
    ctx.save();
    ctx.beginPath();
    ctx.arc(R, R, R, 0, Math.PI * 2);
    ctx.clip();

    // Fora do mapa (borda do mundo): grama escura.
    ctx.fillStyle = "#26321a";
    ctx.fillRect(0, 0, size, size);

    // Fundo: o mapa pintado, deslocado para o jogador ficar no centro.
    const { world, mapCanvas } = this.src;
    const mapK = mapCanvas.width / world.size; // pixels do mapa por metro
    ctx.save();
    ctx.translate(R, R);
    ctx.scale(scale / mapK, scale / mapK);
    ctx.translate(-(meX + world.size / 2) * mapK, -(world.size / 2 - meZ) * mapK);
    ctx.imageSmoothingEnabled = true;
    ctx.drawImage(mapCanvas, 0, 0);
    ctx.restore();

    if (this.night > 0) {
      ctx.fillStyle = `rgba(8,12,38,${(0.4 * this.night).toFixed(3)})`;
      ctx.fillRect(0, 0, size, size);
    }

    // Cone de visão da câmera.
    const cam = this.src.cameraHeading();
    const half = 0.6;
    const grad = ctx.createRadialGradient(R, R, 0, R, R, R * 0.95);
    grad.addColorStop(0, "rgba(255,244,214,0.32)");
    grad.addColorStop(1, "rgba(255,244,214,0)");
    ctx.fillStyle = grad;
    ctx.beginPath();
    ctx.moveTo(R, R);
    // Canvas: ângulo 0 = +X (direita); o norte (para cima) é -π/2.
    ctx.arc(R, R, R * 0.95, cam - Math.PI / 2 - half, cam - Math.PI / 2 + half);
    ctx.closePath();
    ctx.fill();

    // Outras entidades. Chefe e alvo fora do alcance ficam presos na borda.
    const toCanvas = (m: MapMarker) => ({ x: R + (m.x - meX) * scale, y: R - (m.z - meZ) * scale });
    const edge = (p: { x: number; y: number }, pad: number) => {
      const dx = p.x - R;
      const dy = p.y - R;
      const d = Math.hypot(dx, dy);
      const max = R - pad;
      return d > max ? { x: R + (dx / d) * max, y: R + (dy / d) * max, clamped: true } : { ...p, clamped: false };
    };
    for (const m of others) {
      if (m.kind === EntityKind.MONSTER && !m.boss) {
        const p = toCanvas(m);
        if (Math.hypot(p.x - R, p.y - R) > R + 4) continue;
        drawDot(ctx, p.x, p.y, 2.6 * unit, MARKER_COLORS.monster);
      }
    }
    for (const m of others) {
      if (m.kind === EntityKind.PLAYER) {
        const p = toCanvas(m);
        if (Math.hypot(p.x - R, p.y - R) > R + 4) continue;
        drawDot(ctx, p.x, p.y, 3 * unit, MARKER_COLORS.player);
      }
    }
    for (const m of others) {
      if (!m.boss && !m.target) continue;
      const p = edge(toCanvas(m), 7 * unit);
      if (m.boss) drawBossMarker(ctx, p.x, p.y, 5.5 * unit);
      if (m.target) {
        ctx.beginPath();
        ctx.arc(p.x, p.y, (m.boss ? 8.5 : 5.5) * unit, 0, Math.PI * 2);
        ctx.strokeStyle = "#fff6dc";
        ctx.lineWidth = 1.6 * unit;
        ctx.stroke();
      }
    }

    // Jogador no centro.
    ctx.save();
    ctx.translate(R, R);
    ctx.rotate(heading);
    drawPlayerArrow(ctx, 7 * unit);
    ctx.restore();

    ctx.restore();
  }
}
