/** Retângulo jogável (o mapa cresceu só para o sul). */
export interface Bounds {
  minX: number;
  maxX: number;
  minZ: number;
  maxZ: number;
}

/** Formato antigo do world.json (campo `biome`): o Vale, com a borda ondulada e a área dos golens. */
export interface BiomeInfo {
  name: string;
  zFrom: number;
  zTo: number;
  amp: number;
  ph: number[];
  vale: { x: number; z: number; r: number };
}

/**
 * Um bioma do mapa (scripts/biomes/<id>.mjs): o id é o nome do arquivo, e os outros campos
 * são o que o gerador do bioma grava (o `paint` do bioma sabe lê-los).
 */
export interface BiomeEntry {
  id: string;
  name: string;
  bounds?: Bounds;
  [field: string]: unknown;
}

/** Layout do mundo gerado por scripts/build-world.mjs (o servidor usa o mesmo arquivo). */
export interface WorldData {
  version: number;
  /** Lado do quadrado original (compatibilidade); os limites reais estão em `bounds`. */
  size: number;
  bounds?: Bounds;
  /** Biomas, na ordem do gerador (formato novo). */
  biomes?: BiomeEntry[];
  /** Formato antigo: só o Vale (o cliente aceita os dois durante a transição). */
  biome?: BiomeInfo;
  safeRadius: number;
  wall: { apothem: number; sides: number };
  roads: { x1: number; z1: number; x2: number; z2: number; w: number }[];
  colliders: { x: number; z: number; r: number }[];
  objects: { k: string; x: number; z: number; r: number; s: [number, number, number] }[];
  /** Tochas que acendem à noite. */
  torches?: { x: number; z: number }[];
}

/** Limites do mapa (sem `bounds` no arquivo: o quadrado `size` centrado na cidade). */
export function worldBounds(world: WorldData): Bounds {
  if (world.bounds) return world.bounds;
  const h = world.size / 2;
  return { minX: -h, maxX: h, minZ: -h, maxZ: h };
}

/** Biomas do mapa: `biomes[]` ou, no formato antigo, o Vale do campo `biome`. */
export function worldBiomes(world: WorldData): BiomeEntry[] {
  if (world.biomes) return world.biomes;
  return world.biome ? [{ id: "vale", ...world.biome }] : [];
}

export async function loadWorldData(): Promise<WorldData> {
  const res = await fetch("/world.json");
  if (!res.ok) throw new Error(`world.json: ${res.status}`);
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
