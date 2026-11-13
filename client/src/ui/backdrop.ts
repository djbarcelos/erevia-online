/**
 * Fundo animado das telas de entrada: céu de crepúsculo, lua, montanhas em camadas,
 * silhueta de fortaleza, névoa correndo e brasas subindo. Tudo em canvas 2D.
 */

import { mulberry32 } from "../world/map-painter";

interface Ember {
  x: number;
  y: number;
  vx: number;
  vy: number;
  life: number;
  max: number;
  r: number;
}

export function animatedBackdrop(host: HTMLElement, opts: { castle?: boolean; tint?: string } = {}): () => void {
  const canvas = document.createElement("canvas");
  canvas.className = "backdrop";
  host.prepend(canvas);
  const ctx = canvas.getContext("2d")!;
  const reduce = matchMedia("(prefers-reduced-motion: reduce)").matches;
  let w = 0;
  let h = 0;
  let dpr = 1;
  let ridges: { pts: number[]; color: string; y: number; amp: number }[] = [];
  const embers: Ember[] = [];
  let raf = 0;
  let last = performance.now();
  let t = 0;

  const resize = () => {
    dpr = Math.min(2, devicePixelRatio || 1);
    w = host.clientWidth;
    h = host.clientHeight;
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    const rand = mulberry32(7);
    const layers = [
      { color: "#2a2340", y: 0.58, amp: 0.16 },
      { color: "#1d1830", y: 0.68, amp: 0.12 },
      { color: "#130f20", y: 0.8, amp: 0.08 },
    ];
    ridges = layers.map((l) => {
      const pts: number[] = [];
      const n = 48;
      let v = rand();
      for (let i = 0; i <= n; i++) {
        v += (rand() - 0.5) * 0.5;
        v = Math.max(0, Math.min(1, v));
        pts.push(v);
      }
      return { ...l, pts };
    });
  };

  const spawnEmber = () => {
    embers.push({
      x: Math.random() * w,
      y: h + 10,
      vx: (Math.random() - 0.5) * 12,
      vy: -(18 + Math.random() * 40),
      life: 0,
      max: 5 + Math.random() * 6,
      r: 0.8 + Math.random() * 1.8,
    });
  };

  const draw = (dt: number) => {
    t += dt;
    // Céu
    const sky = ctx.createLinearGradient(0, 0, 0, h);
    sky.addColorStop(0, "#0b0a18");
    sky.addColorStop(0.45, "#251a3a");
    sky.addColorStop(0.62, opts.tint ?? "#5a3248");
    sky.addColorStop(0.75, "#2a1a2a");
    sky.addColorStop(1, "#0a0810");
    ctx.fillStyle = sky;
    ctx.fillRect(0, 0, w, h);

    // Estrelas
    const sr = mulberry32(3);
    for (let i = 0; i < 120; i++) {
      const x = sr() * w;
      const y = sr() * h * 0.5;
      const a = 0.25 + 0.5 * Math.abs(Math.sin(t * (0.3 + sr()) + i));
      ctx.fillStyle = `rgba(255,240,220,${a * (1 - y / (h * 0.5))})`;
      ctx.fillRect(x, y, 1.2, 1.2);
    }

    // Lua com halo
    const mx = w * 0.72;
    const my = h * 0.22;
    const mr = Math.min(w, h) * 0.06;
    const halo = ctx.createRadialGradient(mx, my, mr * 0.5, mx, my, mr * 6);
    halo.addColorStop(0, "rgba(255,226,180,0.35)");
    halo.addColorStop(1, "rgba(255,226,180,0)");
    ctx.fillStyle = halo;
    ctx.fillRect(0, 0, w, h);
    ctx.fillStyle = "#f6e6c4";
    ctx.beginPath();
    ctx.arc(mx, my, mr, 0, Math.PI * 2);
    ctx.fill();
    ctx.fillStyle = "rgba(200,170,130,0.35)";
    ctx.beginPath();
    ctx.arc(mx - mr * 0.3, my - mr * 0.2, mr * 0.22, 0, Math.PI * 2);
    ctx.arc(mx + mr * 0.35, my + mr * 0.3, mr * 0.15, 0, Math.PI * 2);
    ctx.fill();

    // Montanhas
    ridges.forEach((r, li) => {
      const drift = reduce ? 0 : Math.sin(t * 0.02 + li) * 6 * (li + 1);
      ctx.fillStyle = r.color;
      ctx.beginPath();
      ctx.moveTo(-20, h);
      const n = r.pts.length - 1;
      for (let i = 0; i <= n; i++) {
        const x = (i / n) * (w + 40) - 20 + drift;
        const y = h * r.y - r.pts[i] * h * r.amp;
        ctx.lineTo(x, y);
      }
      ctx.lineTo(w + 20, h);
      ctx.fill();

      // Fortaleza na segunda camada
      if (li === 1 && opts.castle !== false) {
        const cx = w * 0.28 + drift;
        const base = h * r.y - h * r.amp * 0.55;
        const s = Math.min(w, h) / 700;
        ctx.fillStyle = r.color;
        const tower = (x: number, tw: number, th: number) => {
          ctx.fillRect(x - tw / 2, base - th, tw, th + 10);
          for (let k = -tw / 2; k < tw / 2; k += 8 * s) ctx.fillRect(x + k, base - th - 6 * s, 4 * s, 6 * s);
        };
        tower(cx, 60 * s, 70 * s);
        tower(cx - 50 * s, 22 * s, 110 * s);
        tower(cx + 50 * s, 22 * s, 95 * s);
        tower(cx, 18 * s, 140 * s);
        ctx.beginPath();
        ctx.moveTo(cx - 13 * s, base - 140 * s);
        ctx.lineTo(cx, base - 170 * s);
        ctx.lineTo(cx + 13 * s, base - 140 * s);
        ctx.fill();
        // Janelas acesas
        ctx.fillStyle = "rgba(255,190,90,0.85)";
        for (const [dx, dy] of [
          [-2, 120],
          [-52, 90],
          [48, 76],
          [-12, 40],
          [12, 40],
        ]) {
          ctx.fillRect(cx + dx * s, base - dy * s, 4 * s, 7 * s);
        }
      }
    });

    // Névoa
    for (let i = 0; i < 3; i++) {
      const y = h * (0.66 + i * 0.09);
      const off = reduce ? 0 : ((t * (8 + i * 5)) % (w + 400)) - 200;
      const g = ctx.createRadialGradient(off, y, 10, off, y, w * 0.5);
      g.addColorStop(0, "rgba(160,140,190,0.10)");
      g.addColorStop(1, "rgba(160,140,190,0)");
      ctx.fillStyle = g;
      ctx.fillRect(0, y - h * 0.2, w, h * 0.4);
    }

    // Brasas
    if (!reduce) {
      if (embers.length < Math.min(70, w / 14) && Math.random() < 0.5) spawnEmber();
      for (let i = embers.length - 1; i >= 0; i--) {
        const e = embers[i];
        e.life += dt;
        e.x += (e.vx + Math.sin(t * 2 + i) * 8) * dt;
        e.y += e.vy * dt;
        const k = e.life / e.max;
        if (k >= 1 || e.y < -10) {
          embers.splice(i, 1);
          continue;
        }
        const a = Math.sin(k * Math.PI) * 0.9;
        const g = ctx.createRadialGradient(e.x, e.y, 0, e.x, e.y, e.r * 4);
        g.addColorStop(0, `rgba(255,200,120,${a})`);
        g.addColorStop(0.3, `rgba(255,120,40,${a * 0.5})`);
        g.addColorStop(1, "rgba(255,80,20,0)");
        ctx.fillStyle = g;
        ctx.fillRect(e.x - e.r * 4, e.y - e.r * 4, e.r * 8, e.r * 8);
      }
    }

    // Vinheta
    const v = ctx.createRadialGradient(w / 2, h / 2, Math.min(w, h) * 0.3, w / 2, h / 2, Math.max(w, h) * 0.75);
    v.addColorStop(0, "rgba(0,0,0,0)");
    v.addColorStop(1, "rgba(0,0,0,0.65)");
    ctx.fillStyle = v;
    ctx.fillRect(0, 0, w, h);
  };

  const loop = (now: number) => {
    const dt = Math.min(0.1, (now - last) / 1000);
    last = now;
    draw(dt);
    raf = requestAnimationFrame(loop);
  };

  const ro = new ResizeObserver(resize);
  ro.observe(host);
  resize();
  raf = requestAnimationFrame(loop);

  return () => {
    cancelAnimationFrame(raf);
    ro.disconnect();
    canvas.remove();
  };
}
