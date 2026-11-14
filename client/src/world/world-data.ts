/** Retângulo jogável (o mapa cresceu só para o sul). */
export interface Bounds {
  minX: number;
  maxX: number;
  minZ: number;
  maxZ: number;
}

/** Bioma do sul: borda ondulada com a floresta e a área dos golens. */
export interface BiomeInfo {
  name: string;
  zFrom: number;
  zTo: number;
  amp: number;
  ph: number[];
  vale: { x: number; z: number; r: number };
}

/** Layout do mundo gerado por scripts/build-world.mjs (o servidor usa o mesmo arquivo). */
export interface WorldData {
  version: number;
  /** Lado do quadrado original (compatibilidade); os limites reais estão em `bounds`. */
  size: number;
  bounds?: Bounds;
  biome?: BiomeInfo;
  safeRadius: number;
  wall: { apothem: number; sides: number };
  roads: { x1: number; z1: number; x2: number; z2: number; w: number }[];
  colliders: { x: number; z: number; r: number }[];
  objects: { k: string; x: number; z: number; r: number; s: [number, number, number] }[];
  /** Tochas que acendem à noite. */
  torches?: { x: number; z: number }[];
  /** Mapas além do principal: id e os kits de cenário (.glb) que ele usa (sem o campo: os dois de sempre). */
  id?: string;
  kits?: string[];
}

/** Limites do mapa (sem `bounds` no arquivo: o quadrado `size` centrado na cidade). */
export function worldBounds(world: WorldData): Bounds {
  if (world.bounds) return world.bounds;
  const h = world.size / 2;
  return { minX: -h, maxX: h, minZ: -h, maxZ: h };
}

/**
 * Quanto o ponto já é "vale de pedra" (0 = floresta, 1 = pedra). Mesma conta do gerador
 * (scripts/build-world.mjs), para o chão pintado bater com as pedras e árvores.
 */
export function rockyAt(world: WorldData, x: number, z: number): number {
  const b = world.biome;
  if (!b) return 0;
  const n =
    (Math.sin(x * 0.045 + b.ph[0]) * Math.sin(z * 0.05 + b.ph[1]) +
      0.5 * Math.sin(x * 0.11 + z * 0.07 + b.ph[2]) +
      0.3 * Math.sin(x * 0.23 - z * 0.19 + b.ph[3])) /
    1.8;
  const t = Math.max(0, Math.min(1, (z + n * b.amp - b.zFrom) / (b.zTo - b.zFrom)));
  return t * t * (3 - 2 * t);
}

/** O layout do mapa: o principal em /world.json, os outros em /worlds/<id>.json (scripts/build-maps.mjs). */
export function worldUrl(map = ""): string {
  return !map || map === "starter" ? "/world.json" : `/worlds/${encodeURIComponent(map)}.json`;
}

export async function loadWorldData(map = ""): Promise<WorldData> {
  const url = worldUrl(map);
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: ${res.status}`);
  return res.json();
}

/** Raio do personagem usado na colisão (igual ao servidor). */
export const AGENT_RADIUS = 0.4;

/** Colisão no cliente (para decidir se dá para prever o movimento em linha reta). */
export class Collision {
  private cells = new Map<string, { x: number; z: number; r: number }[]>();
  private readonly cell = 4;

  constructor(world: WorldData) {
    for (const c of world.colliders) {
      const r = c.r + AGENT_RADIUS;
      for (let gx = Math.floor((c.x - r) / this.cell); gx <= Math.floor((c.x + r) / this.cell); gx++) {
        for (let gz = Math.floor((c.z - r) / this.cell); gz <= Math.floor((c.z + r) / this.cell); gz++) {
          const key = `${gx},${gz}`;
          let list = this.cells.get(key);
          if (!list) this.cells.set(key, (list = []));
          list.push(c);
        }
      }
    }
  }

  blocked(x: number, z: number): boolean {
    const list = this.cells.get(`${Math.floor(x / this.cell)},${Math.floor(z / this.cell)}`);
    if (!list) return false;
    return list.some((c) => Math.hypot(c.x - x, c.z - z) < c.r + AGENT_RADIUS);
  }

  /** Linha reta livre (amostra a cada 0,5 m). */
  segmentClear(x1: number, z1: number, x2: number, z2: number): boolean {
    const len = Math.hypot(x2 - x1, z2 - z1);
    const n = Math.max(1, Math.ceil(len / 0.5));
    for (let i = 1; i <= n; i++) {
      const t = i / n;
      if (this.blocked(x1 + (x2 - x1) * t, z1 + (z2 - z1) * t)) return false;
    }
    return true;
  }
}
