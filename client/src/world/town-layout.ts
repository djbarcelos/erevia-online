/**
 * Layout puro da cidade do remaster (sem Babylon, para os testes): que peça nova ocupa o lugar de cada
 * item do kit KayKit e onde ficam os adereços novos. O carregamento está em town-remaster.ts.
 */
import type { WorldData } from "./world-data";

/** Item do kit KayKit (world.json) → peça nova. */
export const TOWN_KITS: Record<string, string> = {
  tavern: "town_tavern_b.glb",
  church: "town_church_b.glb",
  blacksmith: "town_blacksmith_b.glb",
  market: "town_market_b.glb",
  home_a: "town_home_a_b.glb",
  home_b: "town_home_b_b.glb",
  tower_a: "town_tower_b.glb",
  wall_straight: "town_wall_b.glb",
  wall_straight_gate: "town_gate_b.glb",
  tent: "town_stall_b.glb",
  wheelbarrow: "town_cart_b.glb",
  barrel: "town_barrel_b.glb",
  crate_a_big: "town_crate_b.glb",
  crate_long_a: "town_crate_b.glb",
  flag_blue: "town_banner_b.glb",
};

/** Adereços novos, sem item no kit antigo: tipo → arquivo (a escala é 1: o GLB já está em metros). */
export const EXTRA_FILES: Record<string, string> = {
  town_bench: "town_bench_b.glb",
  town_lamp: "town_lamp_b.glb",
  town_sign: "town_sign_b.glb",
  town_corner: "town_corner_b.glb",
  town_tomb: "town_tomb_b.glb",
  town_crystal: "town_crystal_b.glb",
  town_pillar: "town_pillar_b.glb",
};

/** Nenhum fica em cima de NPC, de estrada ou de colisor: conferido por client/src/world/town-remaster.test.ts. */
const EXTRAS: Array<[kind: string, x: number, z: number]> = [
  // Bancos nos quatro lados da fonte, de frente para ela.
  ["town_bench", 0, 5.6],
  ["town_bench", 0, -5.6],
  ["town_bench", 5.8, 0.5],
  ["town_bench", -5.6, 0],
  // Postes de luz na beira das duas ruas, entre as tochas (que ficam em 13, 21,5 e 31 m do centro).
  ["town_lamp", 17.25, 3.6],
  ["town_lamp", -17.25, -3.6],
  ["town_lamp", -3.6, 17.25],
  ["town_lamp", 3.6, -17.25],
  // Placas de rua logo depois de cada portão.
  ["town_sign", 34.5, 4.6],
  ["town_sign", -34.5, -4.6],
  ["town_sign", -4.6, 34.5],
  ["town_sign", 4.6, -34.5],
];

/** Os adereços extras como objetos de cenário (viram thin instances no environment.ts). */
export function townExtras(): WorldData["objects"] {
  return EXTRAS.map(([k, x, z]) => ({
    k,
    x,
    z,
    // A frente do modelo é -Z: vira para o centro da praça (o mesmo `facing` do gerador do mundo).
    r: Math.atan2(-x, -z) + Math.PI,
    s: [1, 1, 1],
  }));
}

/** PRNG pequeno e determinístico (o mesmo mapa a cada carga). */
function rng(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** Identidade das regiões: lápides nos Campos dos Ossos, pilares rachados nos Ermos, cristais no Vale. */
const REGION_PROPS: Array<{ kind: string; cx: number; cz: number; count: number; rMin: number; rMax: number; seed: number }> = [
  { kind: "town_tomb", cx: 5, cz: 94, count: 16, rMin: 7, rMax: 34, seed: 101 },
  { kind: "town_pillar", cx: -128, cz: 5, count: 10, rMin: 9, rMax: 38, seed: 102 },
  { kind: "town_crystal", cx: 6, cz: -198, count: 12, rMin: 7, rMax: 30, seed: 103 },
];

/** Distância de um ponto a um segmento (estradas). */
function distSeg(px: number, pz: number, r: WorldData["roads"][number]): number {
  const dx = r.x2 - r.x1;
  const dz = r.z2 - r.z1;
  const t = Math.max(0, Math.min(1, ((px - r.x1) * dx + (pz - r.z1) * dz) / (dx * dx + dz * dz || 1)));
  return Math.hypot(px - (r.x1 + dx * t), pz - (r.z1 + dz * t));
}

/**
 * Adereços de região do remaster: sorteados em volta da pedra de teleporte de cada região, fora de
 * estrada e de colisor (só visuais, como os outros adereços; o servidor não sabe deles).
 */
export function regionProps(world: Pick<WorldData, "colliders" | "roads">): WorldData["objects"] {
  const out: WorldData["objects"] = [];
  for (const p of REGION_PROPS) {
    const rand = rng(p.seed);
    for (let i = 0, placed = 0; i < p.count * 40 && placed < p.count; i++) {
      const a = rand() * Math.PI * 2;
      const d = p.rMin + rand() * (p.rMax - p.rMin);
      const x = Math.round((p.cx + Math.cos(a) * d) * 100) / 100;
      const z = Math.round((p.cz + Math.sin(a) * d) * 100) / 100;
      if (world.colliders.some((c) => Math.hypot(c.x - x, c.z - z) < c.r + 1.2)) continue;
      if (world.roads.some((r) => distSeg(x, z, r) < r.w / 2 + 2)) continue;
      if (out.some((o) => Math.hypot(o.x - x, o.z - z) < 4)) continue;
      const s = 0.85 + rand() * 0.4;
      out.push({ k: p.kind, x, z, r: rand() * Math.PI * 2, s: [s, s, s] });
      placed++;
    }
  }
  return out;
}

/** Adereços pequenos do kit antigo, sem colisão: ficam fora da conferência de "NPC dentro de parede". */
export const SMALL_PROPS = new Set(["tent", "wheelbarrow", "barrel", "crate_a_big", "crate_long_a", "flag_blue"]);

/**
 * Trechos de canto da muralha: um torreão pequeno em cada vértice entre dois trechos (menos onde já há
 * uma torre grande), que esconde a emenda e os postes das pontas dos trechos. Só visual.
 */
export function wallCorners(world: Pick<WorldData, "wall" | "objects">): WorldData["objects"] {
  const { apothem, sides } = world.wall;
  const vertexR = apothem / Math.cos(Math.PI / sides);
  const towers = world.objects.filter((o) => o.k === "tower_a");
  const out: WorldData["objects"] = [];
  for (let i = 0; i < sides; i++) {
    const a = ((i + 0.5) / sides) * Math.PI * 2;
    const x = Math.round(Math.cos(a) * vertexR * 100) / 100;
    const z = Math.round(Math.sin(a) * vertexR * 100) / 100;
    if (towers.some((t) => Math.hypot(t.x - x, t.z - z) < 3)) continue;
    out.push({ k: "town_corner", x, z, r: Math.atan2(-x, -z) + Math.PI, s: [1, 1, 1] });
  }
  return out;
}
