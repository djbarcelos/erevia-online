import kitSizes from "./kit-sizes.json";
import { worldBiomes, worldBounds, type BiomeEntry, type Bounds, type WorldData } from "./world-data";

type Ctx = CanvasRenderingContext2D;

/** O que o cliente entrega à pintura do chão de um bioma (`paint` em scripts/biomes/<id>.mjs). */
export interface BiomePaintHelpers {
  p: MapProjection;
  /** Limites do mapa inteiro. */
  bounds: Bounds;
  rng: (seed: number) => () => number;
}
export type BiomePaint = (ctx: Ctx, biome: BiomeEntry, h: BiomePaintHelpers) => void;

/**
 * A pintura do chão de cada bioma mora no arquivo do bioma: uma fonte só da regra de
 * transição, a mesma que o gerador usa para espalhar árvores e pedras. Só a função `paint`
 * (e o que ela usa) entra no pacote do cliente; o resto do arquivo é do gerador.
 */
const BIOME_PAINT: Record<string, BiomePaint> = Object.fromEntries(
  Object.entries(import.meta.glob<{ paint?: BiomePaint }>("../../scripts/biomes/*.mjs", { eager: true }))
    .filter(([, mod]) => typeof mod.paint === "function")
    .map(([path, mod]) => [path.slice(path.lastIndexOf("/") + 1, -".mjs".length), mod.paint!]),
);

/** Converte mundo (x, z) → canvas, com +Z (norte) para cima e +X (leste) à direita. */
export interface MapProjection {
  /** Pixels por metro. */
  k: number;
  /** Tamanho do canvas que cobre o mapa inteiro. */
  width: number;
  height: number;
  cx(x: number): number;
  cy(z: number): number;
}

/** Projeção do mapa (retângulo `bounds`) num canvas de `px` pixels de largura. */
export function projection(world: WorldData, px: number): MapProjection {
  const b = worldBounds(world);
  const k = px / (b.maxX - b.minX);
  return {
    k,
    width: px,
    height: Math.round((b.maxZ - b.minZ) * k),
    cx: (x) => (x - b.minX) * k,
    cy: (z) => (b.maxZ - z) * k,
  };
}

/**
 * Pinta o terreno (grama, a camada de cada bioma, estradas de terra e praça de pedra) num
 * canvas de `px` pixels de largura que cobre o mapa inteiro (a altura segue a proporção
 * do mapa). Usado pela textura do chão e pelo mapa.
 */
export function paintTerrain(ctx: Ctx, world: WorldData, px: number): void {
  const { k, cx, cy, width: pw, height: ph } = projection(world, px);
  const rand = mulberry32(7);

  // Base: grama, escurecendo e secando com a distância.
  const grad = ctx.createRadialGradient(cx(0), cy(0), world.safeRadius * k, cx(0), cy(0), world.size * 0.6 * k);
  grad.addColorStop(0, "#5f7d3a");
  grad.addColorStop(0.45, "#4f6d33");
  grad.addColorStop(0.7, "#4a5a35");
  grad.addColorStop(1, "#4a4a3a");
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, pw, ph);

  // Manchas de variação (rápido: círculos translúcidos).
  const tones = ["#6d8a42", "#557233", "#47622c", "#7a8c48", "#5a5a3c", "#3f5528"];
  const area = (pw * ph) / (200 * k) ** 2; // quantidades calibradas para um mapa de 200 m
  for (let i = 0; i < 9000 * area; i++) {
    ctx.globalAlpha = 0.05 + rand() * 0.1;
    ctx.fillStyle = tones[Math.floor(rand() * tones.length)];
    const r = (0.6 + rand() * 4) * k;
    ctx.beginPath();
    ctx.arc(rand() * pw, rand() * ph, r, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.globalAlpha = 1;

  // Camada de cada bioma (por cima da grama, por baixo das estradas e da praça).
  const p = projection(world, px);
  for (const bio of worldBiomes(world)) BIOME_PAINT[bio.id]?.(ctx, bio, { p, bounds: worldBounds(world), rng: mulberry32 });

  // Estradas de terra (borda mais escura + miolo + pedrinhas).
  ctx.lineCap = "round";
  for (const [color, extra] of [["#5a4630", 1.2], ["#8a6e4a", 0]] as const) {
    ctx.strokeStyle = color;
    for (const r of world.roads) {
      ctx.lineWidth = (r.w + extra) * k;
      ctx.beginPath();
      ctx.moveTo(cx(r.x1), cy(r.z1));
      ctx.lineTo(cx(r.x2), cy(r.z2));
      ctx.stroke();
    }
  }
  for (const r of world.roads) {
    const len = Math.hypot(r.x2 - r.x1, r.z2 - r.z1);
    for (let i = 0; i < len * 3; i++) {
      const t = rand();
      const off = (rand() - 0.5) * r.w;
      const nx = -(r.z2 - r.z1) / len;
      const nz = (r.x2 - r.x1) / len;
      const x = r.x1 + (r.x2 - r.x1) * t + nx * off;
      const z = r.z1 + (r.z2 - r.z1) * t + nz * off;
      ctx.fillStyle = rand() < 0.5 ? "#6f5a3e" : "#a08560";
      ctx.globalAlpha = 0.5;
      ctx.fillRect(cx(x), cy(z), 0.25 * k, 0.25 * k);
    }
  }
  ctx.globalAlpha = 1;

  // Praça de pedra dentro da muralha (lajotas em anéis).
  const plazaR = world.wall.apothem + 0.5;
  ctx.fillStyle = "#8b8478";
  ctx.beginPath();
  ctx.arc(cx(0), cy(0), plazaR * k, 0, Math.PI * 2);
  ctx.fill();
  ctx.strokeStyle = "rgba(60,55,50,0.35)";
  ctx.lineWidth = 0.08 * k;
  for (let r = 1.5; r < plazaR; r += 1.5) {
    ctx.beginPath();
    ctx.arc(cx(0), cy(0), r * k, 0, Math.PI * 2);
    ctx.stroke();
    const n = Math.floor((2 * Math.PI * r) / 1.6);
    for (let i = 0; i < n; i++) {
      const a = (i / n) * Math.PI * 2 + r;
      ctx.beginPath();
      ctx.moveTo(cx(Math.cos(a) * (r - 1.5)), cy(Math.sin(a) * (r - 1.5)));
      ctx.lineTo(cx(Math.cos(a) * r), cy(Math.sin(a) * r));
      ctx.stroke();
    }
  }
  for (let i = 0; i < 10 * plazaR * plazaR; i++) {
    const a = rand() * Math.PI * 2;
    const r = Math.sqrt(rand()) * plazaR;
    ctx.fillStyle = rand() < 0.5 ? "rgba(70,65,58,0.18)" : "rgba(170,160,145,0.18)";
    ctx.fillRect(cx(Math.cos(a) * r), cy(Math.sin(a) * r), 0.5 * k, 0.5 * k);
  }
}

/** Nome no mapa das construções que merecem rótulo. */
export const LANDMARKS: Record<string, string> = {
  tavern: "Taverna",
  church: "Igreja",
  blacksmith: "Ferreiro",
  market: "Mercado",
  windmill: "Moinho",
  well: "Poço",
};

const ROOF: Record<string, string> = {
  tavern: "#9a5434",
  church: "#6f7488",
  blacksmith: "#5d5048",
  market: "#a8743a",
  home_a: "#8e4e36",
  home_b: "#7d5a3c",
  windmill: "#9b7a52",
  tent: "#b9a27a",
};

const sizes = kitSizes as Record<string, number[]>;

/**
 * Canvas do mapa (gerado uma vez): terreno + muralha, construções, árvores e pedras vistos
 * de cima. `pxPerMeter` define a resolução (4 px/m dá 1200 px num mapa de 300 m).
 */
export function renderMapCanvas(world: WorldData, pxPerMeter = 4): HTMLCanvasElement {
  const b = worldBounds(world);
  const px = Math.round((b.maxX - b.minX) * pxPerMeter);
  const { k, cx, cy, height } = projection(world, px);
  const canvas = document.createElement("canvas");
  canvas.width = px;
  canvas.height = height;
  const ctx = canvas.getContext("2d", { willReadFrequently: true })!;
  paintTerrain(ctx, world, px);

  // Pedras e árvores: copas escuras com um leve brilho, por cima do terreno.
  for (const o of world.objects) {
    const isTree = o.k.startsWith("tree_");
    const isRock = o.k.startsWith("rock_");
    if (o.k.startsWith("monolith")) {
      // Monólito rúnico: lajota escura com um risco âmbar (as runas).
      const fallen = o.k === "monolith_fallen";
      const w = (fallen ? 2.6 : 1.1) * o.s[0];
      ctx.save();
      ctx.translate(cx(o.x), cy(o.z));
      ctx.rotate(o.r);
      ctx.fillStyle = "rgba(70,72,80,0.95)";
      ctx.fillRect((-w / 2) * k, -0.55 * k, w * k, 1.1 * k);
      ctx.fillStyle = "rgba(255,170,80,0.8)";
      ctx.fillRect(-0.12 * k, -0.25 * k, 0.24 * k, 0.5 * k);
      ctx.restore();
      continue;
    }
    if (!isTree && !isRock) continue;
    const sz = sizes[o.k];
    const r = sz ? (Math.max(sz[0] * o.s[0], sz[2] * o.s[2]) / 2) * (isTree ? 0.8 : 0.7) : 1;
    const x = cx(o.x);
    const y = cy(o.z);
    if (isTree) {
      const bare = o.k.includes("bare");
      ctx.fillStyle = bare ? "rgba(58,52,40,0.85)" : "rgba(34,58,28,0.9)";
      ctx.beginPath();
      ctx.arc(x, y, Math.max(1.5, r * k), 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = bare ? "rgba(110,98,74,0.5)" : "rgba(84,120,58,0.55)";
      ctx.beginPath();
      ctx.arc(x - r * k * 0.25, y - r * k * 0.25, Math.max(0.8, r * k * 0.55), 0, Math.PI * 2);
      ctx.fill();
    } else {
      ctx.fillStyle = "rgba(120,116,108,0.9)";
      ctx.beginPath();
      ctx.arc(x, y, Math.max(1, r * k), 0, Math.PI * 2);
      ctx.fill();
    }
  }

  // Muralha: polígono com aberturas nos portões (0°, 90°, 180°, 270°).
  const { apothem, sides } = world.wall;
  const vertexR = apothem / Math.cos(Math.PI / sides);
  const gateEvery = sides / 4;
  ctx.lineCap = "butt";
  for (const [color, width] of [["#2e2a26", 2.6], ["#9d968a", 1.6]] as const) {
    ctx.strokeStyle = color;
    ctx.lineWidth = width * k;
    for (let i = 0; i < sides; i++) {
      const a0 = ((i - 0.5) / sides) * Math.PI * 2;
      const a1 = ((i + 0.5) / sides) * Math.PI * 2;
      const x0 = Math.cos(a0) * vertexR;
      const z0 = Math.sin(a0) * vertexR;
      const x1 = Math.cos(a1) * vertexR;
      const z1 = Math.sin(a1) * vertexR;
      ctx.beginPath();
      if (i % gateEvery === 0) {
        // Dois pedaços, deixando o vão do portão no meio.
        const gap = 0.22;
        ctx.moveTo(cx(x0), cy(z0));
        ctx.lineTo(cx(x0 + (x1 - x0) * (0.5 - gap)), cy(z0 + (z1 - z0) * (0.5 - gap)));
        ctx.moveTo(cx(x0 + (x1 - x0) * (0.5 + gap)), cy(z0 + (z1 - z0) * (0.5 + gap)));
        ctx.lineTo(cx(x1), cy(z1));
      } else {
        ctx.moveTo(cx(x0), cy(z0));
        ctx.lineTo(cx(x1), cy(z1));
      }
      ctx.stroke();
    }
  }

  // Construções: retângulos girados com cor de telhado.
  for (const o of world.objects) {
    if (o.k === "tower_a" || o.k === "well") {
      const sz = sizes[o.k];
      const r = sz ? (sz[0] * o.s[0]) / 2 : 1.5;
      ctx.fillStyle = o.k === "well" ? "#4c6b8a" : "#6d675e";
      ctx.strokeStyle = "#2a2622";
      ctx.lineWidth = 0.4 * k;
      ctx.beginPath();
      ctx.arc(cx(o.x), cy(o.z), r * k * (o.k === "well" ? 0.8 : 0.9), 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
      continue;
    }
    const roof = ROOF[o.k];
    const sz = sizes[o.k];
    if (!roof || !sz) continue;
    const w = sz[0] * o.s[0];
    const d = sz[2] * o.s[2];
    ctx.save();
    ctx.translate(cx(o.x), cy(o.z));
    // Babylon gira em Y no sentido horário visto de cima; no canvas (y para baixo) é o mesmo.
    ctx.rotate(o.r);
    ctx.fillStyle = roof;
    ctx.strokeStyle = "rgba(25,18,12,0.9)";
    ctx.lineWidth = 0.35 * k;
    ctx.fillRect((-w / 2) * k, (-d / 2) * k, w * k, d * k);
    ctx.strokeRect((-w / 2) * k, (-d / 2) * k, w * k, d * k);
    // Cumeeira do telhado.
    ctx.strokeStyle = "rgba(255,230,200,0.25)";
    ctx.beginPath();
    ctx.moveTo((-w / 2) * k, 0);
    ctx.lineTo((w / 2) * k, 0);
    ctx.stroke();
    ctx.restore();
  }

  // Vinheta suave nas bordas do mapa.
  const big = Math.max(px, height);
  const vig = ctx.createRadialGradient(px / 2, height / 2, big * 0.35, px / 2, height / 2, big * 0.72);
  vig.addColorStop(0, "rgba(0,0,0,0)");
  vig.addColorStop(1, "rgba(0,0,0,0.35)");
  ctx.fillStyle = vig;
  ctx.fillRect(0, 0, px, height);
  return canvas;
}

export function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
