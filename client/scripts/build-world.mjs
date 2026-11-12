// Gera o layout do mundo (determinístico) e grava em:
//   client/public/world.json          → o cliente posiciona o cenário
//   server/internal/world/world.json  → o servidor monta a colisão
//
// Uso: node scripts/build-world.mjs   (rode build-scenery.mjs antes, para os tamanhos)
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';

const here = import.meta.dirname;
const SIZES = JSON.parse(readFileSync(resolve(here, '../src/world/kit-sizes.json'), 'utf8'));

const SIZE = 200; // lado do mapa (±100)
const HALF = SIZE / 2;
const SAFE_RADIUS = 20; // cidade = zona segura
const WALL_APOTHEM = 19; // distância do centro ao meio de cada trecho de muralha
const WALL_SIDES = 12;
const TOWN_SCALE = 4.5; // kit medieval foi feito para hexágonos pequenos
const ROAD_WIDTH = 5;
const ROAD_CLEAR = 4.5; // nada de árvore a menos disso do eixo da estrada
const GATE_HALF_OPENING = 2.3; // metade do vão do portão (m), sem colisão

// ---------- PRNG determinístico ----------
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
const rand = mulberry32(20261113);
const pick = (arr) => arr[Math.floor(rand() * arr.length)];
const between = (a, b) => a + rand() * (b - a);
const round = (v) => Math.round(v * 100) / 100;

const objects = []; // { k, x, z, r, s: [sx, sy, sz] }
const colliders = []; // { x, z, r }

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

// Babylon: rotation.y = a leva o eixo X local para (cos a, -sin a) no plano XZ.
// Para alinhar o X do modelo à direção (dx, dz): a = atan2(-dz, dx).
const alongX = (dx, dz) => Math.atan2(-dz, dx);
// Faz a frente (eixo -Z local, convenção KayKit) olhar para o ponto (tx, tz).
const facing = (x, z, tx, tz) => Math.atan2(tx - x, tz - z) + Math.PI;

// ---------- Cidade ----------
const edgeLen = 2 * WALL_APOTHEM * Math.tan(Math.PI / WALL_SIDES);
const wallScaleX = (edgeLen / SIZES.wall_straight[0]) * 1.06; // leve sobreposição nos cantos
for (let i = 0; i < WALL_SIDES; i++) {
  const th = (i / WALL_SIDES) * Math.PI * 2; // 0° = +X (leste)
  const cx = Math.cos(th) * WALL_APOTHEM;
  const cz = Math.sin(th) * WALL_APOTHEM;
  const tx = -Math.sin(th);
  const tz = Math.cos(th); // tangente
  const isGate = i % 3 === 0; // 0°, 90°, 180°, 270°
  place(isGate ? 'wall_straight_gate' : 'wall_straight', cx, cz, alongX(tx, tz), [wallScaleX, 4.2, 3.2]);
  // Colisão: círculos ao longo do trecho (sem o vão do portão).
  for (let t = -edgeLen / 2; t <= edgeLen / 2 + 0.01; t += 1.4) {
    if (isGate && Math.abs(t) < GATE_HALF_OPENING) continue;
    collide(cx + tx * t, cz + tz * t, 1.4);
  }
  if (isGate) {
    // Bandeiras dos dois lados do portão, do lado de fora.
    for (const side of [-1, 1]) {
      const fx = cx + tx * side * 3.6 + Math.cos(th) * 2.2;
      const fz = cz + tz * side * 3.6 + Math.sin(th) * 2.2;
      place('flag_blue', fx, fz, alongX(tx, tz), 6);
    }
  }
}
// Torres nos vértices diagonais (45°, 135°, 225°, 315°).
const vertexR = WALL_APOTHEM / Math.cos(Math.PI / WALL_SIDES);
for (const deg of [45, 135, 225, 315]) {
  const a = (deg * Math.PI) / 180;
  const x = Math.cos(a) * vertexR;
  const z = Math.sin(a) * vertexR;
  place('tower_a', x, z, facing(x, z, 0, 0), TOWN_SCALE);
  collideItem('tower_a', x, z, TOWN_SCALE, 0.45);
}

// Poço no centro (pequeno: a praça precisa de espaço para nascer e andar).
place('well', 0, 0, 0, 3.2);
collideItem('well', 0, 0, 3.2, 0.45);

// Construções principais nas diagonais, de frente para o centro.
const MAIN = [
  { k: 'tavern', deg: 45, r: 12.5 },
  { k: 'church', deg: 135, r: 12.5 },
  { k: 'blacksmith', deg: 225, r: 12.5 },
  { k: 'market', deg: 315, r: 12.5 },
];
for (const b of MAIN) {
  const a = (b.deg * Math.PI) / 180;
  const x = Math.cos(a) * b.r;
  const z = Math.sin(a) * b.r;
  place(b.k, x, z, facing(x, z, 0, 0), TOWN_SCALE);
  collideItem(b.k, x, z, TOWN_SCALE);
}
// Casas menores entre as estradas e as diagonais.
for (const [i, deg] of [22, 68, 112, 158, 202, 248, 292, 338].entries()) {
  const a = (deg * Math.PI) / 180;
  const r = 14.2;
  const x = Math.cos(a) * r;
  const z = Math.sin(a) * r;
  const k = i % 2 === 0 ? 'home_a' : 'home_b';
  place(k, x, z, facing(x, z, 0, 0), 3.6);
  collideItem(k, x, z, 3.6);
}
// Adereços (sem colisão): perto do ferreiro e do mercado.
const props = [
  ['barrel', -5.2, -9.0], ['barrel', -6.0, -8.2], ['crate_a_big', -8.6, -5.6], ['weaponrack', -9.3, -4.4],
  ['crate_long_a', 7.0, -7.4], ['sack', 8.6, -6.2], ['sack', 8.2, -5.4], ['tent', 4.6, -9.6],
  ['wheelbarrow', 5.6, 8.8], ['barrel', 8.8, 5.6], ['crate_a_big', -8.8, 5.8],
];
for (const [k, x, z] of props) place(k, x, z, rand() * Math.PI * 2, TOWN_SCALE);

// Moinho fora da cidade, ao lado da estrada leste.
place('windmill', 32, 10, facing(32, 10, 32, 0), TOWN_SCALE);
collideItem('windmill', 32, 10, TOWN_SCALE);

// ---------- Estradas ----------
const roads = [
  { x1: WALL_APOTHEM, z1: 0, x2: HALF, z2: 0 },
  { x1: -WALL_APOTHEM, z1: 0, x2: -HALF, z2: 0 },
  { x1: 0, z1: WALL_APOTHEM, x2: 0, z2: HALF },
  { x1: 0, z1: -WALL_APOTHEM, x2: 0, z2: -HALF },
].map((r) => ({ ...r, w: ROAD_WIDTH }));
const onRoad = (x, z) => (Math.abs(x) < ROAD_CLEAR || Math.abs(z) < ROAD_CLEAR);

// ---------- Floresta (por anel) ----------
const LUSH = ['tree_1_a', 'tree_1_b', 'tree_2_a', 'tree_2_b', 'tree_3_a', 'tree_4_a', 'tree_4_b'];
const BARE = ['tree_bare_1_a', 'tree_bare_1_b', 'tree_bare_2_a', 'tree_bare_2_b'];
const BUSH = ['bush_1_a', 'bush_2_a', 'bush_3_a', 'bush_4_a'];
const ROCK_SMALL = ['rock_1_a', 'rock_1_e', 'rock_2_a'];
const ROCK_BIG = ['rock_3_a', 'rock_3_e'];
const GRASS = ['grass_1_a', 'grass_2_a'];

// Grade de ocupação para manter distância entre objetos grandes.
const taken = [];
const farFrom = (x, z, d) => taken.every((t) => Math.hypot(t.x - x, t.z - z) >= d + t.d);
const freeOfColliders = (x, z, margin) => colliders.every((c) => Math.hypot(c.x - x, c.z - z) >= c.r + margin);

function scatter(count, rMin, rMax, fn, { spacing = 0, avoidRoads = true, tries = 30 } = {}) {
  let placed = 0;
  for (let i = 0; i < count * tries && placed < count; i++) {
    const x = between(-HALF + 2, HALF - 2);
    const z = between(-HALF + 2, HALF - 2);
    const r = Math.hypot(x, z);
    if (r < rMin || r > rMax) continue;
    if (avoidRoads && onRoad(x, z)) continue;
    if (spacing > 0 && (!farFrom(x, z, spacing) || !freeOfColliders(x, z, spacing))) continue;
    fn(x, z, r);
    if (spacing > 0) taken.push({ x, z, d: spacing });
    placed++;
  }
}

const RING1 = [SAFE_RADIUS + 3, 52];
const RING2 = [52, 80];
const RING3 = [80, 142]; // inclui os cantos do quadrado

// Árvores: altas (~7–11 m, bem acima do personagem de 1,8 m) e mais espaçadas.
// O tronco engrossa menos que a copa, então o colisor cresce mais devagar que a escala.
scatter(50, ...RING1, (x, z) => {
  const s = between(1.7, 2.2);
  place(pick(LUSH), x, z, rand() * 6.28, s);
  collide(x, z, 0.3 * s);
}, { spacing: 7 });
scatter(45, ...RING2, (x, z) => {
  const bare = rand() < 0.35;
  const s = between(1.7, 2.3) * (bare ? 1.25 : 1);
  place(bare ? pick(BARE) : pick(LUSH), x, z, rand() * 6.28, s);
  collide(x, z, 0.28 * s);
}, { spacing: 7.5 });
scatter(40, ...RING3, (x, z) => {
  const s = between(2.1, 2.8);
  place(pick(BARE), x, z, rand() * 6.28, s);
  collide(x, z, 0.22 * s);
}, { spacing: 7.5 });

// Pedras grandes (bloqueiam) e pequenas (decoração)
scatter(12, ...RING1, (x, z) => {
  const s = between(1.6, 2.4);
  const k = pick(ROCK_BIG);
  place(k, x, z, rand() * 6.28, s);
  collideItem(k, x, z, s, 0.4);
}, { spacing: 3 });
scatter(22, ...RING2, (x, z) => {
  const s = between(1.8, 2.8);
  const k = pick(ROCK_BIG);
  place(k, x, z, rand() * 6.28, s);
  collideItem(k, x, z, s, 0.4);
}, { spacing: 3.5 });
scatter(35, ...RING3, (x, z) => {
  const s = between(2, 3.4);
  const k = pick(ROCK_BIG);
  place(k, x, z, rand() * 6.28, s);
  collideItem(k, x, z, s, 0.4);
}, { spacing: 4 });
scatter(50, SAFE_RADIUS + 2, 142, (x, z) => place(pick(ROCK_SMALL), x, z, rand() * 6.28, between(0.6, 1.3)), { avoidRoads: false });

// Arbustos e grama (sem colisão; muitos, desenhados com thin instances)
scatter(45, ...RING1, (x, z) => place(pick(BUSH), x, z, rand() * 6.28, between(2.2, 3.4)));
scatter(25, ...RING2, (x, z) => place(pick(BUSH), x, z, rand() * 6.28, between(2.2, 3.4)));
scatter(350, ...RING1, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.9, 1.6)), { avoidRoads: false });
scatter(200, ...RING2, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.8, 1.4)), { avoidRoads: false });
scatter(70, ...RING3, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.6, 1.1)), { avoidRoads: false });

// ---------- Borda: montanhas do lado de fora do quadrado ----------
const MOUNTAINS = ['mountain_a_grass_trees', 'mountain_b_grass', 'mountain_c_grass_trees', 'hills_a_trees', 'hills_b_trees'];
for (let t = -HALF - 20; t <= HALF + 20; t += 22) {
  for (const [x, z] of [[t, -HALF - 14], [t, HALF + 14], [-HALF - 14, t], [HALF + 14, t]]) {
    const k = pick(MOUNTAINS);
    place(k, x + between(-4, 4), z + between(-4, 4), rand() * 6.28, between(16, 24));
  }
}

// ---------- Saída ----------
const world = {
  version: 1,
  size: SIZE,
  safeRadius: SAFE_RADIUS,
  wall: { apothem: WALL_APOTHEM, sides: WALL_SIDES },
  roads,
  colliders,
  objects,
};
const json = JSON.stringify(world);
const outs = [resolve(here, '../public/world.json'), resolve(here, '../../server/internal/world/world.json')];
for (const p of outs) {
  mkdirSync(dirname(p), { recursive: true });
  writeFileSync(p, json);
}
const byKind = {};
for (const o of objects) byKind[o.k] = (byKind[o.k] ?? 0) + 1;
console.log(`world.json: ${objects.length} objetos, ${colliders.length} colisores, ${(json.length / 1024).toFixed(0)} KB`);
console.log(Object.entries(byKind).map(([k, n]) => `${k}:${n}`).join(' '));
