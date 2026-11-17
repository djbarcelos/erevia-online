// Gera os mapas além do principal (determinístico), no mesmo formato do world.json:
//   client/public/worlds/<id>.json          → o cliente posiciona o cenário
//   server/internal/world/maps/<id>.json    → o servidor monta a colisão
//
// Hoje só a "teste" (Clareira de Testes): um mapa pequeno para a troca de zona pelo
// portão, ligado só com DEV_LOGIN=true ou EREVIA_ZONES=teste. Usa só o kit da floresta
// (o campo "kits" diz ao cliente quais .glb carregar).
//
// Uso: node scripts/build-maps.mjs   (rode build-scenery.mjs antes, para os tamanhos)
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';

const here = import.meta.dirname;
const SIZES = JSON.parse(readFileSync(resolve(here, '../src/world/kit-sizes.json'), 'utf8'));

function mulberry32(seed) {
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

/** Clareira: praça redonda no centro (zona segura com o portão), floresta em volta e montanhas na borda. */
function clearing({ id, seed, half, safe }) {
  const rand = mulberry32(seed);
  const pick = (arr) => arr[Math.floor(rand() * arr.length)];
  const between = (a, b) => a + rand() * (b - a);
  const objects = [];
  const colliders = [];
  const place = (k, x, z, rotY, s) => {
    if (!SIZES[k]) throw new Error(`item desconhecido: ${k}`);
    objects.push({ k, x: round(x), z: round(z), r: round(rotY), s: [s, s, s].map(round) });
  };
  const collide = (x, z, r) => colliders.push({ x: round(x), z: round(z), r: round(r) });
  const taken = [];
  const farFrom = (x, z, d) => taken.every((t) => Math.hypot(t.x - x, t.z - z) >= d + t.d);
  // A trilha norte–sul fica livre (dá para atravessar a clareira sem contornar nada).
  const onTrail = (x) => Math.abs(x) < 3;
  function scatter(count, rMin, rMax, fn, { spacing = 0, trail = true } = {}) {
    let placed = 0;
    for (let i = 0; i < count * 30 && placed < count; i++) {
      const x = between(-half + 2, half - 2);
      const z = between(-half + 2, half - 2);
      const r = Math.hypot(x, z);
      if (r < rMin || r > rMax || (trail && onTrail(x))) continue;
      if (spacing > 0 && !farFrom(x, z, spacing)) continue;
      fn(x, z);
      if (spacing > 0) taken.push({ x, z, d: spacing });
      placed++;
    }
  }
  const LUSH = ['tree_1_a', 'tree_1_b', 'tree_2_a', 'tree_2_b', 'tree_3_a', 'tree_4_a', 'tree_4_b'];
  const BUSH = ['bush_1_a', 'bush_2_a', 'bush_3_a', 'bush_4_a'];
  const ROCK_SMALL = ['rock_1_a', 'rock_1_e', 'rock_2_a'];
  const ROCK_BIG = ['rock_3_a', 'rock_3_e'];
  const GRASS = ['grass_1_a', 'grass_2_a'];
  const far = half * 1.5;
  scatter(46, safe + 6, far, (x, z) => {
    const s = between(2.1, 2.7);
    place(pick(LUSH), x, z, rand() * 6.28, s);
    collide(x, z, 0.3 * s);
  }, { spacing: 7 });
  scatter(10, safe + 6, far, (x, z) => {
    const s = between(1.6, 2.4);
    const k = pick(ROCK_BIG);
    place(k, x, z, rand() * 6.28, s);
    collide(x, z, Math.max(SIZES[k][0], SIZES[k][2]) * s * 0.4);
  }, { spacing: 3 });
  scatter(40, safe + 2, far, (x, z) => place(pick(ROCK_SMALL), x, z, rand() * 6.28, between(0.6, 1.3)), { trail: false });
  scatter(50, safe + 3, far, (x, z) => place(pick(BUSH), x, z, rand() * 6.28, between(2.2, 3.4)));
  scatter(360, safe + 1, far, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.9, 1.6)), { trail: false });
  // Montanhas do lado de fora do quadrado (fecham o horizonte; sem colisão: a borda do mapa já prende).
  const MOUNTAINS = ['mountain_a_grass_trees', 'mountain_b_grass', 'mountain_c_grass_trees', 'hills_a_trees', 'hills_b_trees'];
  for (let t = -half - 20; t <= half + 20; t += 24) {
    for (const [x, z] of [[t, -half - 16], [t, half + 16], [-half - 16, t], [half + 16, t]]) {
      place(pick(MOUNTAINS), x + between(-4, 4), z + between(-4, 4), rand() * 6.28, between(18, 26));
    }
  }
  return {
    version: 1,
    id,
    size: half * 2,
    bounds: { minX: -half, maxX: half, minZ: -half, maxZ: half },
    safeRadius: safe,
    // Só a praça pintada (sem muralha): o mapa e o chão usam o apótema como raio da praça.
    wall: { apothem: safe - 1.5, sides: 12 },
    roads: [{ x1: 0, z1: safe - 1.5, x2: 0, z2: half, w: 4 }, { x1: 0, z1: -(safe - 1.5), x2: 0, z2: -half, w: 4 }],
    kits: ['/models/env-forest.glb'],
    colliders,
    objects,
    torches: [],
  };
}

const maps = [clearing({ id: 'teste', seed: 20261136, half: 60, safe: 9 })];
for (const m of maps) {
  const json = JSON.stringify(m);
  for (const p of [resolve(here, `../public/worlds/${m.id}.json`), resolve(here, `../../server/internal/world/maps/${m.id}.json`)]) {
    mkdirSync(dirname(p), { recursive: true });
    writeFileSync(p, json);
  }
  console.log(`${m.id}.json: ${m.objects.length} objetos, ${m.colliders.length} colisores, ${(json.length / 1024).toFixed(0)} KB`);
}
