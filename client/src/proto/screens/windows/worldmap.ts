/**
 * Mapa do mundo (tecla M): terreno pintado, zonas de caça com faixa de nível, locais,
 * missões, grupo, monstros e você. Arrastar move, roda/pinça dá zoom, clique marca o
 * destino e o personagem anda até lá. Painel lateral com local atual, camadas e zonas.
 */

import { drawBossMarker, drawDot, drawPlayerArrow, MARKER_COLORS } from "../../../ui/minimap";
import { LANDMARKS, renderMapCanvas } from "../../../world/map-painter";
import { ui } from "../../../ui/art";
import { MAP_POIS, PARTY_POS, QUESTS, ZONES, zoneAt, type Zone } from "../../data/mock";
import type { Sim } from "../../hud/sim";
import { button, clamp, el, isCompact, toggle } from "../../../ui/kit";

type Layer = "zones" | "places" | "quests" | "party" | "monsters";

const LAYERS: { id: Layer; label: string; dot: string }[] = [
  { id: "zones", label: "Zonas de caça", dot: "linear-gradient(90deg,#78c86e,#e6b450,#e65a46)" },
  { id: "places", label: "Locais da cidade", dot: "#f3d995" },
  { id: "quests", label: "Missões", dot: "#ffd36b" },
  { id: "party", label: "Grupo", dot: MARKER_COLORS.player },
  { id: "monsters", label: "Monstros", dot: MARKER_COLORS.monster },
];

let cachedCanvas: HTMLCanvasElement | null = null;

function levelText(z: Zone): string {
  return z.safe ? "Zona segura" : `Nv ${z.levelMin}–${z.levelMax}`;
}

export function build(sim: Sim): HTMLElement {
  const world = sim.data;
  const src = cachedCanvas ?? (cachedCanvas = renderMapCanvas(world, 6));
  const half = world.size / 2;
  const show: Record<Layer, boolean> = { zones: true, places: true, quests: true, party: true, monsters: true };

  const root = el("div", "wmap");
  const view = el("div", "wmap-view");
  const canvas = el("canvas", "wmap-canvas");
  canvas.setAttribute("aria-label", "Mapa do mundo — arraste para mover, role para aproximar, clique para ir");
  const ctx = canvas.getContext("2d")!;

  // Rosa dos ventos, escala, botões e info do cursor
  const compass = el("div", "wmap-compass");
  compass.innerHTML = `<svg viewBox="0 0 64 64" aria-hidden="true">
    <circle cx="32" cy="32" r="27" fill="rgba(10,8,14,.7)" stroke="#d6b46e" stroke-width="1.5"/>
    <circle cx="32" cy="32" r="21" fill="none" stroke="rgba(214,180,110,.35)" stroke-dasharray="2 3"/>
    <path d="M32 8 L37 32 L32 28 L27 32 Z" fill="#f3d995"/><path d="M32 56 L27 32 L32 36 L37 32 Z" fill="#8a6a32"/>
    <path d="M8 32 L32 27 L28 32 L32 37 Z" fill="#8a6a32"/><path d="M56 32 L32 37 L36 32 L32 27 Z" fill="#8a6a32"/>
    <text x="32" y="6.5" text-anchor="middle" font-size="7" font-weight="800" fill="#f3d995" font-family="serif">N</text></svg>`;
  const scaleBar = el("div", "wmap-scale", el("i"), el("span"));
  const hoverInfo = el("div", "wmap-hover");
  hoverInfo.hidden = true;
  const zoomIn = button("", "wmap-btn", () => zoomBy(1.4));
  zoomIn.innerHTML = ui("plus", 18);
  zoomIn.setAttribute("aria-label", "Aproximar");
  const zoomOut = button("−", "wmap-btn", () => zoomBy(1 / 1.4));
  zoomOut.setAttribute("aria-label", "Afastar");
  const centerBtn = button("", "wmap-btn", () => {
    follow = true;
    centerBtn.classList.add("on");
  });
  centerBtn.innerHTML = ui("target", 18);
  centerBtn.title = "Centralizar em você";
  centerBtn.setAttribute("aria-label", "Centralizar em você");
  centerBtn.classList.add("on");
  const tools = el("div", "wmap-tools", zoomIn, zoomOut, centerBtn);
  const hint = el("div", "wmap-hint", "Clique no mapa para ir até lá");
  view.append(canvas, compass, scaleBar, tools, hoverInfo, hint);

  // ---------- Painel lateral ----------
  const side = el("aside", "wmap-side scroll");
  const here = el("div", "wmap-here");
  const layers = el("div", "wmap-layers");
  for (const l of LAYERS) {
    const sw = el("i", "lg-dot");
    sw.style.background = l.dot;
    layers.append(el("label", "wmap-layer", sw, el("span", null, l.label), toggle(show[l.id], (v) => (show[l.id] = v), l.label)));
  }
  const zoneList = el("div", "wmap-zones");
  for (const z of ZONES) {
    const b = el("button", "wmap-zone");
    b.type = "button";
    b.style.setProperty("--zc", `rgb(${z.rgb})`);
    b.append(el("span", "zc"), el("span", "zn", el("b", null, z.name), el("small", null, z.desc)), el("span", "zl", levelText(z)));
    b.onclick = () => {
      const r = (z.rMin + z.rMax) / 2;
      flyTo(z.safe ? 0 : 0, z.safe ? 0 : r, z.safe ? 4 : Math.max(1.4, Math.min(4, 220 / (z.rMax * 2))));
    };
    zoneList.append(b);
  }
  const questList = el("div", "wmap-quests");
  for (const p of MAP_POIS.filter((p) => p.kind === "quest" || p.kind === "quest-area")) {
    const q = QUESTS.find((x) => x.title === p.sub || x.title === p.name);
    const b = el("button", "wmap-quest");
    b.type = "button";
    b.append(el("span", "qm", p.kind === "quest" ? "!" : "?"), el("span", null, el("b", null, q?.title ?? p.name), el("small", null, p.kind === "quest" ? p.name : (p.sub ?? ""))));
    b.onclick = () => flyTo(p.x, p.z, 4);
    questList.append(b);
  }
  side.append(
    el("h3", "wmap-title", "Vale de Vel'Aran"),
    here,
    el("div", "wmap-sec", "Camadas"),
    layers,
    el("div", "wmap-sec", "Regiões"),
    zoneList,
    el("div", "wmap-sec", "Missões"),
    questList,
  );
  root.append(view, side);

  // ---------- Câmera do mapa ----------
  let scale = 3.2; // px CSS por metro
  let cx = sim.me.x;
  let cz = sim.me.z;
  let follow = true;
  let fly: { x: number; z: number; s: number } | null = null;
  let waypoint: { x: number; z: number; t: number } | null = null;
  let hover: { x: number; z: number } | null = null;

  const minScale = () => Math.min(view.clientWidth, view.clientHeight) / (world.size * 1.05);
  const toWorld = (px: number, py: number) => ({ x: cx + (px - view.clientWidth / 2) / scale, z: cz - (py - view.clientHeight / 2) / scale });

  function zoomBy(f: number, px = view.clientWidth / 2, py = view.clientHeight / 2) {
    const before = toWorld(px, py);
    scale = clamp(scale * f, minScale(), 12);
    const after = toWorld(px, py);
    cx += before.x - after.x;
    cz += before.z - after.z;
    fly = null;
  }

  function flyTo(x: number, z: number, s: number) {
    follow = false;
    centerBtn.classList.remove("on");
    fly = { x, z, s };
  }

  // ---------- Entrada ----------
  const pointers = new Map<number, { x: number; y: number }>();
  let downAt: { x: number; y: number; t: number } | null = null;
  let pinch = 0;
  canvas.addEventListener("pointerdown", (e) => {
    canvas.setPointerCapture(e.pointerId);
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (pointers.size === 1) downAt = { x: e.clientX, y: e.clientY, t: performance.now() };
    else {
      downAt = null;
      const [a, b] = [...pointers.values()];
      pinch = Math.hypot(a.x - b.x, a.y - b.y);
    }
  });
  canvas.addEventListener("pointermove", (e) => {
    const r = canvas.getBoundingClientRect();
    hover = toWorld(e.clientX - r.left, e.clientY - r.top);
    const p = pointers.get(e.pointerId);
    if (!p) return;
    if (pointers.size === 1) {
      const dx = e.clientX - p.x;
      const dy = e.clientY - p.y;
      if (downAt && Math.hypot(e.clientX - downAt.x, e.clientY - downAt.y) > 6) {
        follow = false;
        centerBtn.classList.remove("on");
        fly = null;
      }
      if (!follow) {
        cx -= dx / scale;
        cz += dy / scale;
      }
      canvas.style.cursor = "grabbing";
    } else if (pointers.size === 2) {
      pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
      const [a, b] = [...pointers.values()];
      const d = Math.hypot(a.x - b.x, a.y - b.y);
      zoomBy(d / pinch, (a.x + b.x) / 2 - r.left, (a.y + b.y) / 2 - r.top);
      pinch = d;
      return;
    }
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
  });
  const up = (e: PointerEvent) => {
    pointers.delete(e.pointerId);
    canvas.style.cursor = "";
    if (downAt && performance.now() - downAt.t < 400 && Math.hypot(e.clientX - downAt.x, e.clientY - downAt.y) < 6) {
      const r = canvas.getBoundingClientRect();
      const w = toWorld(e.clientX - r.left, e.clientY - r.top);
      if (Math.abs(w.x) < half && Math.abs(w.z) < half) {
        waypoint = { ...w, t: performance.now() };
        sim.moveTo(w.x, w.z);
      }
    }
    downAt = null;
  };
  canvas.addEventListener("pointerup", up);
  canvas.addEventListener("pointercancel", up);
  canvas.addEventListener("pointerleave", () => (hover = null));
  canvas.addEventListener(
    "wheel",
    (e) => {
      e.preventDefault();
      const r = canvas.getBoundingClientRect();
      zoomBy(Math.exp(-e.deltaY * 0.0015), e.clientX - r.left, e.clientY - r.top);
    },
    { passive: false },
  );

  // ---------- Desenho ----------
  const label = (text: string, x: number, y: number, size: number, color: string, weight = 600, font = "system-ui, sans-serif") => {
    ctx.font = `${weight} ${size}px ${font}`;
    ctx.textAlign = "center";
    ctx.textBaseline = "middle";
    ctx.lineJoin = "round";
    ctx.lineWidth = Math.max(3, size / 3.5);
    ctx.strokeStyle = "rgba(10,7,4,0.85)";
    ctx.strokeText(text, x, y);
    ctx.fillStyle = color;
    ctx.fillText(text, x, y);
  };

  let raf = 0;
  let alive = true;
  const draw = () => {
    if (!alive) return;
    raf = requestAnimationFrame(draw);
    const W = view.clientWidth;
    const H = view.clientHeight;
    if (!W || !H) return;
    const dpr = Math.min(2, devicePixelRatio || 1);
    if (canvas.width !== Math.round(W * dpr) || canvas.height !== Math.round(H * dpr)) {
      canvas.width = Math.round(W * dpr);
      canvas.height = Math.round(H * dpr);
    }
    scale = Math.max(scale, minScale());
    if (follow) {
      cx += (sim.me.x - cx) * 0.2;
      cz += (sim.me.z - cz) * 0.2;
    } else if (fly) {
      cx += (fly.x - cx) * 0.14;
      cz += (fly.z - cz) * 0.14;
      scale += (fly.s - scale) * 0.14;
      if (Math.abs(fly.x - cx) < 0.3 && Math.abs(fly.z - cz) < 0.3 && Math.abs(fly.s - scale) < 0.01) fly = null;
    }
    // Não deixa sair muito do mapa.
    const lim = half - Math.min(W, H) / scale / 2 + 20;
    cx = clamp(cx, -Math.max(0, lim), Math.max(0, lim));
    cz = clamp(cz, -Math.max(0, lim), Math.max(0, lim));

    const sx = (x: number) => W / 2 + (x - cx) * scale;
    const sy = (z: number) => H / 2 - (z - cz) * scale;
    const now = performance.now();

    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = "#0d120b";
    ctx.fillRect(0, 0, W, H);

    // Terreno
    ctx.imageSmoothingEnabled = true;
    ctx.imageSmoothingQuality = "high";
    ctx.drawImage(src, sx(-half), sy(half), world.size * scale, world.size * scale);
    // Leve tom de pergaminho
    ctx.fillStyle = "rgba(60,40,10,0.12)";
    ctx.fillRect(sx(-half), sy(half), world.size * scale, world.size * scale);

    // Zonas
    if (show.zones) {
      for (const z of ZONES) {
        if (z.safe) {
          ctx.beginPath();
          ctx.arc(sx(0), sy(0), z.rMax * scale, 0, Math.PI * 2);
          ctx.strokeStyle = `rgba(${z.rgb},0.6)`;
          ctx.lineWidth = 2;
          ctx.setLineDash([]);
          ctx.stroke();
          continue;
        }
        ctx.beginPath();
        ctx.arc(sx(0), sy(0), z.rMax * scale, 0, Math.PI * 2);
        ctx.arc(sx(0), sy(0), z.rMin * scale, 0, Math.PI * 2, true);
        ctx.fillStyle = `rgba(${z.rgb},0.10)`;
        ctx.fill();
        ctx.setLineDash([7, 6]);
        ctx.lineWidth = 1.4;
        ctx.strokeStyle = `rgba(${z.rgb},0.65)`;
        ctx.beginPath();
        ctx.arc(sx(0), sy(0), z.rMax * scale, 0, Math.PI * 2);
        ctx.stroke();
        ctx.setLineDash([]);
        // Rótulo em quatro pontos (diagonais, longe das estradas)
        const r = (z.rMin + z.rMax) / 2;
        const fs = clamp(scale * 4.2, 10, 18);
        // Afastado, só dois rótulos por zona (senão o mapa fica poluído).
        for (const a of scale < 2.6 ? [45, 225] : [45, 135, 225, 315]) {
          const rad = (a * Math.PI) / 180;
          const x = sx(Math.cos(rad) * r);
          const y = sy(Math.sin(rad) * r);
          if (x < -80 || x > W + 80 || y < -40 || y > H + 40) continue;
          label(z.name.toUpperCase(), x, y - fs * 0.6, fs * 0.8, "#f4ead2", 700, "Cinzel, Georgia, serif");
          label(levelText(z), x, y + fs * 0.55, fs * 0.72, `rgb(${z.rgb})`, 700);
        }
      }
    }

    // Locais da cidade
    if (show.places) {
      const fs = clamp(scale * 2.6, 9, 14);
      if (scale > 2.2) {
        for (const o of world.objects) {
          const name = LANDMARKS[o.k];
          if (!name) continue;
          const x = sx(o.x);
          const y = sy(o.z);
          ctx.fillStyle = "#f3d995";
          ctx.beginPath();
          ctx.moveTo(x, y - 5);
          ctx.lineTo(x + 5, y);
          ctx.lineTo(x, y + 5);
          ctx.lineTo(x - 5, y);
          ctx.fill();
          label(name, x, y - 13, fs, "#fff3d0");
        }
        for (const p of MAP_POIS.filter((p) => p.kind === "gate" || p.kind === "portal")) {
          const x = sx(p.x);
          const y = sy(p.z);
          if (p.kind === "portal") {
            const pulse = 1 + Math.sin(now / 400) * 0.15;
            const g = ctx.createRadialGradient(x, y, 0, x, y, 12 * pulse);
            g.addColorStop(0, "rgba(160,200,255,0.95)");
            g.addColorStop(1, "rgba(80,120,255,0)");
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.arc(x, y, 12 * pulse, 0, Math.PI * 2);
            ctx.fill();
            label(p.name, x, y + 16, fs * 0.9, "#cfe0ff");
          } else if (scale > 3) {
            label(p.name, x, y, fs * 0.8, "#d8cbb0", 500);
          }
        }
      }
      label("VEL'ARAN", sx(0), sy(0) + (scale > 2.2 ? 24 : 0), clamp(scale * 6, 13, 26), "#f3d995", 700, "Cinzel, Georgia, serif");
    }

    // Missões
    if (show.quests) {
      for (const p of MAP_POIS) {
        const x = sx(p.x);
        const y = sy(p.z);
        if (p.kind === "quest-area") {
          ctx.beginPath();
          ctx.arc(x, y, (p.r ?? 10) * scale, 0, Math.PI * 2);
          ctx.fillStyle = "rgba(255,211,107,0.12)";
          ctx.fill();
          ctx.strokeStyle = "rgba(255,211,107,0.8)";
          ctx.lineWidth = 1.5;
          ctx.setLineDash([3, 4]);
          ctx.stroke();
          ctx.setLineDash([]);
          questIcon(x, y, "?");
        } else if (p.kind === "quest") {
          questIcon(x, y - 4 + Math.sin(now / 300) * 2, "!");
        }
      }
    }

    // Monstros e chefe
    if (show.monsters) {
      for (const u of sim.units) {
        if (u.dead) continue;
        if (u.boss) {
          drawBossMarker(ctx, sx(u.x), sy(u.z), 9);
          if (scale > 2) label(u.name, sx(u.x), sy(u.z) - 17, 11, "#e6a6ff", 700);
        } else drawDot(ctx, sx(u.x), sy(u.z), u.id === sim.targetId ? 5 : 3.5, u.id === sim.targetId ? "#ffd36b" : MARKER_COLORS.monster);
      }
    }

    // Grupo
    if (show.party) {
      for (const p of PARTY_POS) {
        drawDot(ctx, sx(p.x), sy(p.z), 5, MARKER_COLORS.player);
        if (scale > 1.6) label(p.name, sx(p.x), sy(p.z) - 13, 11, "#bfe0ff", 700);
      }
    }

    // Destino marcado + caminho
    if (waypoint) {
      const arrived = Math.hypot(sim.me.x - waypoint.x, sim.me.z - waypoint.z) < 1.2 || !sim.dest;
      if (arrived && now - waypoint.t > 600) waypoint = null;
      else {
        ctx.setLineDash([4, 6]);
        ctx.lineWidth = 2;
        ctx.strokeStyle = "rgba(243,217,149,0.85)";
        ctx.beginPath();
        ctx.moveTo(sx(sim.me.x), sy(sim.me.z));
        ctx.lineTo(sx(waypoint.x), sy(waypoint.z));
        ctx.stroke();
        ctx.setLineDash([]);
        const k = ((now - waypoint.t) % 1200) / 1200;
        ctx.strokeStyle = `rgba(243,217,149,${1 - k})`;
        ctx.beginPath();
        ctx.arc(sx(waypoint.x), sy(waypoint.z), 4 + k * 14, 0, Math.PI * 2);
        ctx.stroke();
        pin(sx(waypoint.x), sy(waypoint.z));
      }
    }

    // Você: cone da câmera + seta
    const px = sx(sim.me.x);
    const py = sy(sim.me.z);
    ctx.save();
    ctx.translate(px, py);
    ctx.rotate(sim.cameraHeading());
    const g = ctx.createRadialGradient(0, 0, 0, 0, 0, 60);
    g.addColorStop(0, "rgba(255,240,200,0.28)");
    g.addColorStop(1, "rgba(255,240,200,0)");
    ctx.fillStyle = g;
    ctx.beginPath();
    ctx.moveTo(0, 0);
    ctx.arc(0, 0, 60, -Math.PI / 2 - 0.55, -Math.PI / 2 + 0.55);
    ctx.fill();
    ctx.restore();
    ctx.save();
    ctx.translate(px, py);
    ctx.rotate(sim.me.heading);
    ctx.scale(1.15, 1.15);
    drawPlayerArrow(ctx, 14);
    ctx.restore();

    // Moldura escura nas bordas (vinheta)
    const v = ctx.createRadialGradient(W / 2, H / 2, Math.min(W, H) * 0.35, W / 2, H / 2, Math.max(W, H) * 0.75);
    v.addColorStop(0, "rgba(0,0,0,0)");
    v.addColorStop(1, "rgba(0,0,0,0.55)");
    ctx.fillStyle = v;
    ctx.fillRect(0, 0, W, H);

    // Escala: um valor redondo de metros
    const target = 110 / scale;
    const nice = [5, 10, 20, 25, 50, 100, 200].find((n) => n >= target * 0.6) ?? 200;
    scaleBar.querySelector("i")!.style.width = `${nice * scale}px`;
    scaleBar.querySelector("span")!.textContent = `${nice} m`;

    // Painel: onde você está
    const zone = zoneAt(sim.me.x, sim.me.z);
    const hereKey = zone.id;
    if (here.dataset.z !== hereKey) {
      here.dataset.z = hereKey;
      here.style.setProperty("--zc", `rgb(${zone.rgb})`);
      here.replaceChildren(el("span", "label", "Você está em"), el("b", null, zone.name), el("span", "zl", levelText(zone)), el("small", "muted", zone.desc));
    }

    // Cursor
    if (hover && !isCompact()) {
      const hz = zoneAt(hover.x, hover.z);
      hoverInfo.hidden = false;
      hoverInfo.textContent = `${hz.name} · ${levelText(hz)} · ${Math.round(hover.x)}, ${Math.round(hover.z)}`;
    } else hoverInfo.hidden = true;
  };

  function questIcon(x: number, y: number, ch: string) {
    ctx.beginPath();
    ctx.arc(x, y, 9, 0, Math.PI * 2);
    ctx.fillStyle = "#2a1d07";
    ctx.fill();
    ctx.lineWidth = 2;
    ctx.strokeStyle = "#ffd36b";
    ctx.stroke();
    label(ch, x, y + 1, 13, "#ffd36b", 900);
  }

  function pin(x: number, y: number) {
    ctx.fillStyle = "#f3d995";
    ctx.strokeStyle = "#2a1d07";
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.moveTo(x, y);
    ctx.bezierCurveTo(x - 8, y - 10, x - 8, y - 20, x, y - 22);
    ctx.bezierCurveTo(x + 8, y - 20, x + 8, y - 10, x, y);
    ctx.fill();
    ctx.stroke();
    ctx.fillStyle = "#2a1d07";
    ctx.beginPath();
    ctx.arc(x, y - 14, 3, 0, Math.PI * 2);
    ctx.fill();
  }

  raf = requestAnimationFrame(draw);
  root.addEventListener("win-close", () => {
    alive = false;
    cancelAnimationFrame(raf);
  });
  return root;
}
