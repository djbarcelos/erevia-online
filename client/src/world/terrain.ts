/**
 * Relevo do mundo (tunnel#99): colinas suaves, só visuais. O servidor continua 2D (x, z) e
 * não sabe que o chão sobe e desce; o cliente põe cada coisa na altura certa do chão com
 * `heightAt(x, z)`, uma função pura e única (personagens, monstros, NPCs, saque, avisos no
 * chão, marcadores, cenário).
 *
 * Regras do relevo (para o relevo nunca atrapalhar o que se anda):
 * - plano na cidade, nas estradas, na Cripta e no miolo do Vale (as áreas caminháveis
 *   importantes: praça, pedras de teleporte, chefes, covil do raid, spawns da estrada);
 * - no resto, colinas de até ~1,8 m de altura e inclinação máxima de ~6° (a altura sobe a
 *   partir do plano, nunca fica abaixo dele, então estradas e praça ficam nos vales);
 * - a lagoa de Vau Velho (Brejo de Lamagris, tunnel#139) fica no meio do pântano: rasa e andável (o servidor não
 *   sabe dela), com duas partes fundas (a aldeia afogada e o fosso da Ilha dos Vaga-lumes), que o servidor bloqueia
 *   com colisores; o pântano é quase plano.
 *
 * Este arquivo não importa nada do Babylon (roda no `npm run test:unit`).
 */

import type { Bounds, WorldData } from "./world-data";

// ---------------------------------------------------------------------------------------
// Parâmetros
// ---------------------------------------------------------------------------------------

/** Altura máxima das colinas (m). */
export const HILL_AMPLITUDE = 1.8;
/** Quanto o relevo se estende além do retângulo jogável (m): é o que se vê por trás da borda. */
export const TERRAIN_MARGIN = 100;
/** Folga da cidade: plano até safeRadius + isto. */
const CITY_FLAT_EXTRA = 12;
const CITY_BLEND = 18;
/** Estrada: plano até w/2 + isto, e depois sobe em BLEND metros. */
const ROAD_FLAT_EXTRA = 6;
const ROAD_BLEND = 14;
/** Vale (golens): plano até o raio da área + isto. Fora do disco, as colinas são mais baixas. */
const VALE_FLAT_EXTRA = 8;
const VALE_BLEND = 24;
const VALE_HILL_SCALE = 0.55;
/** Cripta: plano dentro do retângulo do bioma. */
const CRYPT_BLEND = 14;

/** Lagoa de Vau Velho: uma elipse rasa com partes fundas e uma ilha (a entrada `brejo` do world.json). */
export interface Lake {
  cx: number;
  cz: number;
  rx: number;
  rz: number;
  /** Altura da superfície da água. */
  level: number;
  /** Partes fundas (a água passa do joelho; o servidor bota colisores em volta). */
  deeps: { x: number; z: number; r: number }[];
  /** Ilha dentro de uma parte funda (sobe acima da água). */
  island: { x: number; z: number; r: number } | null;
}

/** Altura do chão de fora do mapa (além da malha do terreno). */
export const EDGE_Y = -2.2;

/** Fundo da parte rasa da lagoa (m): a água bate no joelho (a superfície fica em −0,35). */
const LAKE_SHALLOW = -0.8;
/** Fundo das partes fundas (m). */
const LAKE_DEEP = -1.9;
/** Quanto a ilha sobe acima da água (m). */
const ISLAND_UP = 0.3;
/** O pântano é quase plano: as colinas dele valem esta fração. */
const BREJO_HILL_SCALE = 0.25;
const BREJO_BLEND = 16;
/** Posto do Barqueiro e cais: planos num disco (raio, e a faixa em que a colina volta). */
const POST_FLAT = 14;
const DOCK_FLAT = 9;
const SPOT_BLEND = 10;
/** A parte funda desce em tanto (m) além do raio dela. */
const DEEP_BANK = 4;
/** Distância normalizada (u) em que a bacia começa e em que chega ao fundo. */
const LAKE_BANK_OUT = 1.55;
const LAKE_BANK_IN = 0.85;

/** A lagoa do mapa (a entrada `brejo` do world.json); null se o mapa não tem o Brejo. */
export function lakeOf(world: WorldData): Lake | null {
  const b = biomeById(world, "brejo");
  const l = b?.lagoon;
  if (!l) return null;
  return { cx: l.x, cz: l.z, rx: l.rx, rz: l.rz, level: l.level, deeps: b.deeps ?? [], island: b.island ?? null };
}

// ---------------------------------------------------------------------------------------
// Ruído (determinístico, sem texturas)
// ---------------------------------------------------------------------------------------

function hash(ix: number, iz: number, seed: number): number {
  let h = Math.imul(ix, 0x27d4eb2d) ^ Math.imul(iz, 0x165667b1) ^ Math.imul(seed, 0x9e3779b1);
  h = Math.imul(h ^ (h >>> 15), 0x85ebca6b);
  h = Math.imul(h ^ (h >>> 13), 0xc2b2ae35);
  return ((h ^ (h >>> 16)) >>> 0) / 4294967296;
}

const quintic = (t: number) => t * t * t * (t * (t * 6 - 15) + 10);

/** Ruído de valor suave, de -1 a 1. */
export function noise2(x: number, z: number, seed: number): number {
  const ix = Math.floor(x);
  const iz = Math.floor(z);
  const fx = quintic(x - ix);
  const fz = quintic(z - iz);
  const a = hash(ix, iz, seed);
  const b = hash(ix + 1, iz, seed);
  const c = hash(ix, iz + 1, seed);
  const d = hash(ix + 1, iz + 1, seed);
  return (a + (b - a) * fx + (c - a) * fz + (a - b - c + d) * fx * fz) * 2 - 1;
}

export function smoothstep(e0: number, e1: number, x: number): number {
  const t = Math.min(1, Math.max(0, (x - e0) / (e1 - e0)));
  return t * t * (3 - 2 * t);
}

// ---------------------------------------------------------------------------------------
// Campo de alturas
// ---------------------------------------------------------------------------------------

export interface HeightField {
  /** Retângulo coberto pela malha do terreno (o jogável mais a margem). */
  readonly extent: Bounds;
  /** Retângulo jogável do mapa. */
  readonly bounds: Bounds;
  /** Lago (só no mapa principal e só com relevo). */
  readonly lake: Lake | null;
  /** false = chão plano (render antigo). */
  readonly hilly: boolean;
  /** Altura do chão em (x, z), em metros. */
  heightAt(x: number, z: number): number;
  /** Distância normalizada ao centro do lago (1 = elipse; menor que ~1,2 é água). Infinity sem lago. */
  lakeU(x: number, z: number): number;
  /** Mundo de origem (estradas, biomas, cidade), para a mistura de texturas. */
  readonly world: WorldData | null;
}

const NO_BOUNDS: Bounds = { minX: -1, maxX: 1, minZ: -1, maxZ: 1 };

/** Chão plano em y = 0: o padrão até a cena montar o relevo, e o render antigo. */
export const FLAT_FIELD: HeightField = {
  extent: NO_BOUNDS,
  bounds: NO_BOUNDS,
  lake: null,
  hilly: false,
  heightAt: () => 0,
  lakeU: () => Infinity,
  world: null,
};

function distToSegment(px: number, pz: number, x1: number, z1: number, x2: number, z2: number): number {
  const dx = x2 - x1;
  const dz = z2 - z1;
  const l2 = dx * dx + dz * dz;
  const t = l2 > 0 ? Math.min(1, Math.max(0, ((px - x1) * dx + (pz - z1) * dz) / l2)) : 0;
  return Math.hypot(px - (x1 + t * dx), pz - (z1 + t * dz));
}

/** Distância de (x, z) ao retângulo (0 dentro). */
function distToRect(x: number, z: number, r: Bounds): number {
  const dx = Math.max(r.minX - x, 0, x - r.maxX);
  const dz = Math.max(r.minZ - z, 0, z - r.maxZ);
  return Math.hypot(dx, dz);
}

interface BiomeInfo {
  bounds?: Bounds;
  vale?: { x: number; z: number; r: number };
  lagoon?: { x: number; z: number; rx: number; rz: number; level: number };
  deeps?: { x: number; z: number; r: number }[];
  island?: { x: number; z: number; r: number };
  post?: { x: number; z: number };
  dock?: { x: number; z: number };
}

function biomeById(world: WorldData, id: string): BiomeInfo | null {
  const b = world.biomes?.find((e) => e.id === id) as BiomeInfo | undefined;
  if (b) return b;
  if (id === "vale" && world.biome) return { vale: world.biome.vale, bounds: { minX: -1e9, maxX: 1e9, minZ: world.biome.zTo, maxZ: world.biome.zFrom } };
  return null;
}

/**
 * Monta o campo de alturas do mundo. `hilly = false` devolve um campo plano com a mesma
 * interface (render antigo).
 */
export function buildHeightField(world: WorldData, hilly = true, withLake = true): HeightField {
  const bounds = world.bounds ?? { minX: -world.size / 2, maxX: world.size / 2, minZ: -world.size / 2, maxZ: world.size / 2 };
  const extent: Bounds = {
    minX: bounds.minX - TERRAIN_MARGIN,
    maxX: bounds.maxX + TERRAIN_MARGIN,
    minZ: bounds.minZ - TERRAIN_MARGIN,
    maxZ: bounds.maxZ + TERRAIN_MARGIN,
  };
  if (!hilly) return { ...FLAT_FIELD, extent, bounds, world };

  const isMain = !world.id || world.id === "starter";
  const lake = isMain && withLake ? lakeOf(world) : null;
  const brejo = biomeById(world, "brejo");
  const brejoZone = brejo?.bounds ?? null;
  const crypt = biomeById(world, "cripta")?.bounds ?? null;
  const vale = biomeById(world, "vale");
  const valeDisc = vale?.vale ?? null;
  const valeZone = vale?.bounds ?? null;
  const cityFlat = world.safeRadius + CITY_FLAT_EXTRA;
  const roads = world.roads;
  // Escala das colinas num mapa pequeno (arena de teste): proporcional, para não engolir o mapa.
  const span = Math.min(bounds.maxX - bounds.minX, bounds.maxZ - bounds.minZ);
  const amp = HILL_AMPLITUDE * Math.min(1, span / 200);

  const lakeU = (x: number, z: number): number => {
    if (!lake) return Infinity;
    const wob = 1 + 0.1 * noise2(x / 28, z / 28, 5);
    return Math.hypot((x - lake.cx) / lake.rx, (z - lake.cz) / lake.rz) / wob;
  };

  const heightAt = (x: number, z: number): number => {
    // Máscara: 0 = plano, 1 = colina inteira.
    let m = smoothstep(cityFlat, cityFlat + CITY_BLEND, Math.hypot(x, z));
    if (m > 0) {
      for (const r of roads) {
        const d = distToSegment(x, z, r.x1, r.z1, r.x2, r.z2);
        m = Math.min(m, smoothstep(r.w / 2 + ROAD_FLAT_EXTRA, r.w / 2 + ROAD_FLAT_EXTRA + ROAD_BLEND, d));
        if (m === 0) break;
      }
    }
    if (m > 0 && crypt) m = Math.min(m, smoothstep(0, CRYPT_BLEND, distToRect(x, z, crypt)));
    if (m > 0 && brejoZone) {
      // O pântano é quase plano; o posto e o cais ficam planos de vez.
      m *= 1 - (1 - BREJO_HILL_SCALE) * (1 - smoothstep(0, BREJO_BLEND, distToRect(x, z, brejoZone)));
      if (brejo?.post) m = Math.min(m, smoothstep(POST_FLAT, POST_FLAT + SPOT_BLEND, Math.hypot(x - brejo.post.x, z - brejo.post.z)));
      if (brejo?.dock) m = Math.min(m, smoothstep(DOCK_FLAT, DOCK_FLAT + SPOT_BLEND, Math.hypot(x - brejo.dock.x, z - brejo.dock.z)));
    }
    if (m > 0 && valeZone) {
      // Colinas mais baixas no Vale (a transição acompanha a borda do bioma) e plano no miolo.
      const inVale = 1 - smoothstep(0, 20, distToRect(x, z, valeZone));
      m *= 1 - (1 - VALE_HILL_SCALE) * inVale;
      if (valeDisc) m = Math.min(m, smoothstep(valeDisc.r + VALE_FLAT_EXTRA, valeDisc.r + VALE_FLAT_EXTRA + VALE_BLEND, Math.hypot(x - valeDisc.x, z - valeDisc.z)));
    }
    let h = 0;
    if (m > 0) {
      const n = 0.8 * noise2(x / 110, z / 110, 1) + 0.2 * noise2(x / 46 + 13, z / 46, 2);
      h = amp * (0.5 + 0.5 * n) * m;
    }
    // Na borda da malha o chão desce até EDGE_Y, onde fica o chão de fora (scene.ts, "outer"): assim
    // ele não cobre a bacia do lago.
    const edge = Math.min(x - extent.minX, extent.maxX - x, z - extent.minZ, extent.maxZ - z);
    const inside = smoothstep(0, 30, edge);
    h = h * inside + EDGE_Y * (1 - inside);
    // Bacia da lagoa: rasa até o meio, funda nas duas partes fundas, com a ilha subindo dentro de uma delas.
    if (lake) {
      const u = lakeU(x, z);
      if (u < LAKE_BANK_OUT) {
        const b = smoothstep(LAKE_BANK_OUT, LAKE_BANK_IN, u);
        h += (LAKE_SHALLOW - h) * b;
      }
      for (const d of lake.deeps) {
        const t = smoothstep(d.r + DEEP_BANK, d.r, Math.hypot(x - d.x, z - d.z));
        if (t > 0) h += (LAKE_DEEP - h) * t;
      }
      const i = lake.island;
      if (i) {
        const t = smoothstep(i.r + 1.5, i.r - 1.5, Math.hypot(x - i.x, z - i.z));
        if (t > 0) h += (Math.max(h, lake.level + ISLAND_UP) - h) * t;
      }
    }
    return h;
  };

  return { extent, bounds, lake, hilly: true, heightAt, lakeU, world };
}

// ---------------------------------------------------------------------------------------
// Campo ativo (um por cena)
// ---------------------------------------------------------------------------------------

let active: HeightField = FLAT_FIELD;

/** Define o relevo da cena atual (createWorldScene). Sem isso, o chão é plano em y = 0. */
export function setTerrain(f: HeightField): void {
  active = f;
}

export function terrain(): HeightField {
  return active;
}

/** Altura do chão em (x, z). Todo objeto que fica no chão usa esta função. */
export function heightAt(x: number, z: number): number {
  return active.heightAt(x, z);
}

/** Inclinação (m por m) do chão em (x, z). */
export function slopeAt(f: HeightField, x: number, z: number): number {
  const e = 1.2;
  const dx = (f.heightAt(x + e, z) - f.heightAt(x - e, z)) / (2 * e);
  const dz = (f.heightAt(x, z + e) - f.heightAt(x, z - e)) / (2 * e);
  return Math.hypot(dx, dz);
}

/**
 * Quanto uma peça plana e redonda (anel, marca no chão) de raio `radius` em (x, z) precisa
 * subir para não afundar nas colinas em volta: a maior altura na borda menos a do centro.
 * É 0 no plano (cidade, estradas), então não muda nada onde o chão é reto.
 */
export function groundClearance(f: HeightField, x: number, z: number, radius: number): number {
  if (!f.hilly) return 0;
  const h0 = f.heightAt(x, z);
  let up = 0;
  for (let i = 0; i < 8; i++) {
    const a = (i / 8) * Math.PI * 2;
    up = Math.max(up, f.heightAt(x + Math.cos(a) * radius, z + Math.sin(a) * radius) - h0);
  }
  return up;
}

/** Normal do chão (unitária) em (x, z), por diferenças finitas. */
export function normalAt(f: HeightField, x: number, z: number, out: [number, number, number] = [0, 1, 0]): [number, number, number] {
  const e = 1.2;
  const dx = (f.heightAt(x + e, z) - f.heightAt(x - e, z)) / (2 * e);
  const dz = (f.heightAt(x, z + e) - f.heightAt(x, z - e)) / (2 * e);
  const l = Math.hypot(dx, 1, dz);
  out[0] = -dx / l;
  out[1] = 1 / l;
  out[2] = -dz / l;
  return out;
}

// ---------------------------------------------------------------------------------------
// Clique no chão e câmera
// ---------------------------------------------------------------------------------------

/**
 * Primeiro ponto em que o raio (origem o, direção d) encontra o chão, ou null. Marcha de
 * 0,75 m e refina por bisseção; sem malha nenhuma (o chão é uma função).
 */
export function rayTerrain(
  f: HeightField,
  ox: number, oy: number, oz: number,
  dx: number, dy: number, dz: number,
  maxDist = 260,
): { x: number; y: number; z: number } | null {
  const step = 0.75;
  let prev = 0;
  if (oy <= f.heightAt(ox, oz)) return null; // câmera dentro do chão
  for (let t = step; t <= maxDist; t += step) {
    const x = ox + dx * t;
    const z = oz + dz * t;
    const y = oy + dy * t;
    if (y <= f.heightAt(x, z)) {
      let lo = prev;
      let hi = t;
      for (let i = 0; i < 10; i++) {
        const mid = (lo + hi) / 2;
        if (oy + dy * mid <= f.heightAt(ox + dx * mid, oz + dz * mid)) hi = mid;
        else lo = mid;
      }
      const tt = (lo + hi) / 2;
      const px = ox + dx * tt;
      const pz = oz + dz * tt;
      return { x: px, y: f.heightAt(px, pz), z: pz };
    }
    prev = t;
  }
  return null;
}

/**
 * Maior raio (até `radius`) em que a câmera que orbita `target` (alpha, beta, como o
 * ArcRotateCamera) fica `clearance` acima do chão: a câmera nunca atravessa uma colina.
 */
export function clearRadius(
  f: HeightField,
  tx: number, ty: number, tz: number,
  alpha: number, beta: number, radius: number,
  minRadius: number, clearance = 0.8,
): number {
  const sb = Math.sin(beta);
  const dirX = Math.cos(alpha) * sb;
  const dirY = Math.cos(beta);
  const dirZ = Math.sin(alpha) * sb;
  for (let r = radius; r > minRadius; r -= 0.75) {
    const x = tx + dirX * r;
    const z = tz + dirZ * r;
    if (ty + dirY * r >= f.heightAt(x, z) + clearance) return r;
  }
  return minRadius;
}

// ---------------------------------------------------------------------------------------
// Mistura de texturas (splat) por região
// ---------------------------------------------------------------------------------------

/** Camadas do chão: grama, terra, pedra e cascalho. */
export const LAYERS = ["grama", "terra", "pedra", "cascalho"] as const;
export type Splat = [number, number, number, number];

/**
 * Peso de cada camada por região (docs/guia-de-arte.md, 3.3): Arredores verde vivo;
 * Campos dos Ossos grama seca com osso; Ermos terra rachada; Vale rocha e musgo; Cripta
 * pedra fria. Soma 1.
 */
export const REGION_SPLAT: Record<string, Splat> = {
  Cidade: [0.6, 0.22, 0.06, 0.12],
  Arredores: [0.74, 0.14, 0.04, 0.08],
  "Campos dos Ossos": [0.36, 0.4, 0.04, 0.2],
  "Ermos Partidos": [0.1, 0.46, 0.18, 0.26],
  "Vale das Pedras Vivas": [0.18, 0.04, 0.5, 0.28],
  "Cripta Esquecida": [0, 0.04, 0.8, 0.16],
  // Brejo de Lamagris: lodo (terra) com musgo, sem pedra; as Terras Mortas (extensão): terra cinza e cascalho.
  "Brejo de Lamagris": [0.34, 0.56, 0, 0.1],
  "Terras Mortas": [0.06, 0.34, 0.2, 0.4],
};

/** Raio (do centro da cidade) onde cada região do anel está inteira; entre dois, mistura. */
const RING_KEYS: [number, string][] = [
  [30, "Cidade"],
  [60, "Arredores"],
  [94, "Campos dos Ossos"],
  [128, "Ermos Partidos"],
];

function lerp4(a: Splat, b: Splat, t: number): Splat {
  return [a[0] + (b[0] - a[0]) * t, a[1] + (b[1] - a[1]) * t, a[2] + (b[2] - a[2]) * t, a[3] + (b[3] - a[3]) * t];
}

/** Mistura de camadas só pela região (sem ruído, inclinação nem água), em (x, z). Soma 1. */
export function regionSplat(world: WorldData, x: number, z: number): Splat {
  const r = Math.hypot(x, z);
  let w: Splat = REGION_SPLAT[RING_KEYS[RING_KEYS.length - 1][1]];
  if (r <= RING_KEYS[0][0]) w = REGION_SPLAT[RING_KEYS[0][1]];
  else {
    for (let i = 1; i < RING_KEYS.length; i++) {
      if (r <= RING_KEYS[i][0]) {
        const t = smoothstep(RING_KEYS[i - 1][0], RING_KEYS[i][0], r);
        w = lerp4(REGION_SPLAT[RING_KEYS[i - 1][1]], REGION_SPLAT[RING_KEYS[i][1]], t);
        break;
      }
    }
  }
  const vale = biomeById(world, "vale");
  if (vale?.bounds) {
    // Entra no Vale pela fronteira do bioma (a névoa e a cor já mudam ali).
    const inside = 1 - smoothstep(0, 16, distToRect(x, z, vale.bounds));
    if (inside > 0) w = lerp4(w, REGION_SPLAT["Vale das Pedras Vivas"], inside);
  }
  for (const [id, region] of [["brejo", "Brejo de Lamagris"], ["terras-mortas", "Terras Mortas"]] as const) {
    const bounds = biomeById(world, id)?.bounds;
    if (!bounds) continue;
    const inside = 1 - smoothstep(0, 16, distToRect(x, z, bounds));
    if (inside > 0) w = lerp4(w, REGION_SPLAT[region], inside);
  }
  const crypt = biomeById(world, "cripta")?.bounds;
  if (crypt) {
    const inside = 1 - smoothstep(0, 12, distToRect(x, z, crypt));
    if (inside > 0) w = lerp4(w, REGION_SPLAT["Cripta Esquecida"], inside);
  }
  return w;
}

/**
 * Mistura final das camadas em (x, z): a região, com manchas de ruído (grama ↔ terra),
 * mais pedra onde o chão é inclinado, cascalho e terra na beira da água e terra perto das
 * estradas. Pesos de 0 a 1, somando 1.
 */
export function splatAt(f: HeightField, x: number, z: number): Splat {
  const world = f.world;
  const w: Splat = world ? [...regionSplat(world, x, z)] as Splat : [1, 0, 0, 0];
  // Manchas: onde o ruído é alto, mais grama; onde é baixo, mais terra.
  const p = noise2(x / 9, z / 9, 7) * 0.6 + noise2(x / 3.2, z / 3.2, 8) * 0.4;
  const patch = 0.3 * p * (w[0] + w[1]) * 1.6;
  w[0] += patch;
  w[1] -= patch;
  // Inclinação: pedra nas encostas.
  const s = smoothstep(0.035, 0.1, slopeAt(f, x, z));
  w[2] += 0.5 * s;
  w[0] *= 1 - 0.7 * s;
  // Beira do lago: cascalho e terra, sem grama.
  const u = f.lakeU(x, z);
  const shore = smoothstep(1.75, 1.2, u);
  if (shore > 0) {
    w[3] += 0.7 * shore;
    w[1] += 0.25 * shore;
    w[0] *= 1 - shore;
    w[2] *= 1 - 0.5 * shore;
  }
  // Perto das estradas: terra.
  if (world) {
    for (const r of world.roads) {
      const d = distToSegment(x, z, r.x1, r.z1, r.x2, r.z2);
      const near = 1 - smoothstep(r.w / 2, r.w / 2 + 6, d);
      if (near > 0) {
        w[1] += 0.45 * near;
        w[0] *= 1 - 0.6 * near;
      }
    }
  }
  let sum = 0;
  for (let i = 0; i < 4; i++) {
    if (w[i] < 0) w[i] = 0;
    sum += w[i];
  }
  if (sum <= 0) return [1, 0, 0, 0];
  for (let i = 0; i < 4; i++) w[i] /= sum;
  return w;
}

/** Textura de mistura (RGBA, um texel por `cell` metros) sobre `extent`, para o shader do chão. */
export function bakeSplat(f: HeightField, cell: number): { width: number; height: number; data: Uint8Array } {
  const e = f.extent;
  const width = Math.ceil((e.maxX - e.minX) / cell);
  const height = Math.ceil((e.maxZ - e.minZ) / cell);
  const data = new Uint8Array(width * height * 4);
  for (let j = 0; j < height; j++) {
    const z = e.minZ + (j + 0.5) * cell;
    for (let i = 0; i < width; i++) {
      const x = e.minX + (i + 0.5) * cell;
      const w = splatAt(f, x, z);
      const o = (j * width + i) * 4;
      for (let k = 0; k < 4; k++) data[o + k] = Math.round(w[k] * 255);
    }
  }
  return { width, height, data };
}
