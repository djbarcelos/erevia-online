// Motor do gerador do mundo: junta os biomas de scripts/biomes/ num world.json.
//
// Cada bioma é um arquivo `scripts/biomes/<id>.mjs` (veja o README, "Como criar um bioma").
// O bioma não importa nada: o gerador entrega tudo pelo `ctx` (o sorteio do próprio bioma,
// `place`, `collide`, `scatter`…) e o cliente entrega o necessário para pintar o chão.
//
// As fases rodam nesta ordem, cada uma passando pelos biomas na ordem de `order`:
//   1. build(ctx)   → o cenário do bioma; pode devolver dados para a entrada dele em `biomes[]`
//   2. lights(ctx)  → tochas, que evitam o que todos os biomas já puseram no chão
//   3. overlay(ctx) → lugares que limpam a área (`ctx.clear`) e se constroem por cima
// Um bioma usa só as fases de que precisa. A ordem de tudo (sorteios, listas) é parte do
// mapa: mexer nela muda o world.json.
import { readdirSync, readFileSync } from 'node:fs';
import { basename, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

const here = import.meta.dirname;
export const BIOMES_DIR = resolve(here, 'biomes');
export const OUTPUTS = [resolve(here, '../public/world.json'), resolve(here, '../../server/internal/world/world.json')];

/** Lado do quadrado original (compatibilidade: o servidor e o cliente ainda leem `size`). */
const SIZE = 300;

/** Itens desenhados pelo cliente, sem modelo do kit (os tamanhos são a escala 1). */
const CLIENT_ITEMS = {
  monolith: [1.1, 2.6, 1.1, 0],
  monolith_broken: [1.1, 1.4, 1.1, 0],
  monolith_fallen: [2.6, 0.9, 1.1, 0],
};

// ---------- PRNG determinístico ----------
export function mulberry32(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const round = (v) => Math.round(v * 100) / 100;

const PHASES = ['build', 'lights', 'overlay'];

/** Lê os biomas de um ou mais diretórios (arquivos `<id>.mjs`; `_` no começo fica de fora). */
export async function loadBiomes(...dirs) {
  const list = [];
  for (const dir of dirs.length ? dirs : [BIOMES_DIR]) {
    for (const f of readdirSync(dir).sort()) {
      if (!/^[a-z0-9][a-z0-9-]*\.mjs$/.test(f)) continue;
      const mod = await import(pathToFileURL(resolve(dir, f)).href);
      list.push({ id: basename(f, '.mjs'), ...mod });
    }
  }
  const seeds = new Set();
  const ids = new Set();
  for (const b of list) {
    const where = `bioma ${b.id}`;
    if (ids.has(b.id)) throw new Error(`${where}: id repetido`);
    if (typeof b.name !== 'string' || !b.name) throw new Error(`${where}: falta o name`);
    if (!Number.isInteger(b.seed)) throw new Error(`${where}: falta a seed (inteiro)`);
    if (seeds.has(b.seed)) throw new Error(`${where}: seed ${b.seed} repetida (cada bioma sorteia com a sua)`);
    if (typeof b.order !== 'number') throw new Error(`${where}: falta o order (número)`);
    const bb = b.bounds;
    if (!bb || !(bb.maxX > bb.minX) || !(bb.maxZ > bb.minZ)) throw new Error(`${where}: bounds inválido`);
    if (!PHASES.some((p) => typeof b[p] === 'function')) throw new Error(`${where}: sem build, lights nem overlay`);
    ids.add(b.id);
    seeds.add(b.seed);
  }
  return list.sort((a, b) => a.order - b.order || (a.id < b.id ? -1 : 1));
}

/**
 * Gera o mundo. `legacy: true` grava o formato de antes (`biome` único, o do bioma que
 * devolve dados no build), byte a byte igual ao gerador de antes dos biomas: serve ao teste
 * de que o formato novo descreve o mesmo mapa, enquanto o cliente aceitar os dois.
 */
export async function buildWorld({ biomes, legacy = false } = {}) {
  biomes ??= await loadBiomes();
  const SIZES = { ...JSON.parse(readFileSync(resolve(here, '../src/world/kit-sizes.json'), 'utf8')), ...CLIENT_ITEMS };

  const objects = []; // { k, x, z, r, s: [sx, sy, sz] }
  const colliders = []; // { x, z, r }
  const torches = []; // { x, z }
  const roads = []; // { x1, z1, x2, z2, w }
  const taken = []; // { x, z, d }: ocupação, para manter distância entre objetos grandes
  const meta = {}; // safeRadius, wall (a cidade)

  // O mapa é a união dos limites dos biomas.
  const bounds = { minX: Infinity, maxX: -Infinity, minZ: Infinity, maxZ: -Infinity };
  for (const b of biomes) {
    bounds.minX = Math.min(bounds.minX, b.bounds.minX);
    bounds.maxX = Math.max(bounds.maxX, b.bounds.maxX);
    bounds.minZ = Math.min(bounds.minZ, b.bounds.minZ);
    bounds.maxZ = Math.max(bounds.maxZ, b.bounds.maxZ);
  }

  function place(k, x, z, rotY, scale) {
    if (!SIZES[k]) throw new Error(`item desconhecido: ${k}`);
    const s = Array.isArray(scale) ? scale : [scale, scale, scale];
    objects.push({ k, x: round(x), z: round(z), r: round(rotY), s: s.map(round) });
  }
  function collide(x, z, r) {
    colliders.push({ x: round(x), z: round(z), r: round(r) });
  }
  /** Colisor redondo a partir do tamanho do item (fração do maior lado no chão). */
  function collideItem(k, x, z, scale, frac = 0.42) {
    const [sx, , sz] = SIZES[k];
    collide(x, z, Math.max(sx, sz) * scale * frac);
  }
  function torch(x, z) {
    torches.push({ x: round(x), z: round(z) });
    collide(x, z, 0.3);
  }
  /** Tira o que os biomas anteriores puseram a menos de `r` de (x, z): objetos, colisores e tochas. */
  function clear(x, z, r) {
    for (const arr of [objects, colliders, torches]) {
      for (let i = arr.length - 1; i >= 0; i--) if (Math.hypot(arr[i].x - x, arr[i].z - z) < r) arr.splice(i, 1);
    }
  }
  const farFrom = (x, z, d) => taken.every((t) => Math.hypot(t.x - x, t.z - z) >= d + t.d);
  const freeOfColliders = (x, z, margin) => colliders.every((c) => Math.hypot(c.x - x, c.z - z) >= c.r + margin);
  /** Tamanho no chão (maior lado) → escala do item. */
  const sized = (k, meters) => meters / Math.max(SIZES[k][0], SIZES[k][2]);
  // Babylon: rotation.y = a leva o eixo X local para (cos a, -sin a) no plano XZ.
  // Para alinhar o X do modelo à direção (dx, dz): a = atan2(-dz, dx).
  const alongX = (dx, dz) => Math.atan2(-dz, dx);
  // Faz a frente (eixo -Z local, convenção KayKit) olhar para o ponto (tx, tz).
  const facing = (x, z, tx, tz) => Math.atan2(tx - x, tz - z) + Math.PI;

  const shared = {
    SIZES, objects, colliders, torches, roads, taken, meta, mapBounds: bounds,
    place, collide, collideItem, torch, clear, farFrom, freeOfColliders, sized, alongX, facing, round,
  };
  const ctxs = biomes.map((b) => {
    const rand = mulberry32(b.seed);
    const between = (a, c) => a + rand() * (c - a);
    /**
     * Sorteia até `count` pontos em x × z que passem em `accept` (e, com `spacing`, longe do
     * que já está ocupado e dos colisores) e chama `fn(x, z)` em cada um.
     */
    function scatter(count, { x: [x0, x1], z: [z0, z1], accept = () => true, spacing = 0, tries = 30 }, fn) {
      let placed = 0;
      for (let i = 0; i < count * tries && placed < count; i++) {
        const x = between(x0, x1);
        const z = between(z0, z1);
        if (!accept(x, z)) continue;
        if (spacing > 0 && (!farFrom(x, z, spacing) || !freeOfColliders(x, z, spacing))) continue;
        fn(x, z);
        if (spacing > 0) taken.push({ x, z, d: spacing });
        placed++;
      }
    }
    return {
      ...shared,
      bounds: b.bounds,
      rand,
      pick: (arr) => arr[Math.floor(rand() * arr.length)],
      between,
      scatter,
    };
  });

  const info = new Map(); // id → dados que o build devolveu
  for (const phase of PHASES) {
    biomes.forEach((b, i) => {
      if (typeof b[phase] !== 'function') return;
      const out = b[phase](ctxs[i]);
      if (phase === 'build' && out) info.set(b.id, out);
    });
  }
  if (meta.safeRadius === undefined || !meta.wall) throw new Error('nenhum bioma definiu a cidade (meta.safeRadius, meta.wall)');

  let biomeField;
  if (legacy) {
    const withInfo = biomes.filter((b) => info.has(b.id));
    if (withInfo.length > 1) throw new Error('o formato antigo só tem lugar para um bioma com dados');
    const b = withInfo[0];
    biomeField = b ? { biome: { name: b.name, ...info.get(b.id) } } : {};
  } else {
    biomeField = { biomes: biomes.map((b) => ({ id: b.id, name: b.name, bounds: b.bounds, ...info.get(b.id) })) };
  }
  const world = {
    version: 1,
    size: SIZE,
    bounds,
    ...biomeField,
    safeRadius: meta.safeRadius,
    wall: meta.wall,
    roads,
    colliders,
    objects,
    torches,
  };
  return { world, json: JSON.stringify(world) };
}
