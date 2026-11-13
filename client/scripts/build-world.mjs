// Gera o layout do mundo (determinístico) e grava em:
//   client/public/world.json          → o cliente posiciona o cenário
//   server/internal/world/world.json  → o servidor monta a colisão
//
// Uso: node scripts/build-world.mjs   (rode build-scenery.mjs antes, para os tamanhos)
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';

const here = import.meta.dirname;
const SIZES = JSON.parse(readFileSync(resolve(here, '../src/world/kit-sizes.json'), 'utf8'));

const SIZE = 300; // lado do quadrado original (±150); a floresta continua sorteada nele
const HALF = SIZE / 2;
// O mapa cresceu só para o sul: o Vale das Pedras Vivas vai de z = -150 até -330.
const BOUNDS = { minX: -HALF, maxX: HALF, minZ: -330, maxZ: HALF };
const SAFE_RADIUS = 40; // cidade = zona segura
const WALL_APOTHEM = 38; // distância do centro ao meio de cada trecho de muralha
const WALL_SIDES = 20; // múltiplo de 4 (portões nos eixos) com vértices em 45°
// O kit medieval foi feito para hexágonos pequenos. Os heróis KayKit são "cabeçudos" e têm
// 1,8 m, então as construções precisam ser grandes para parecerem casas de verdade.
const TOWN_SCALE = 9; // taverna ~12 m, torre ~18 m
const HOME_SCALE = 7.5; // casas ~7 m
const PROP_SCALE = 4.5; // barris ~1 m
const ROAD_WIDTH = 6;
const ROAD_CLEAR = 5; // nada de árvore a menos disso do eixo da estrada
const WALL_SCALE_YZ = [6.5, 4]; // ~7 m de altura, ~3 m de espessura

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
// O vão do portão ocupa ~43% da largura do modelo (metade do vão, em m, sem colisão).
const GATE_HALF_OPENING = 0.426 * wallScaleX;
const GATE_EVERY = WALL_SIDES / 4;
for (let i = 0; i < WALL_SIDES; i++) {
  const th = (i / WALL_SIDES) * Math.PI * 2; // 0° = +X (leste)
  const cx = Math.cos(th) * WALL_APOTHEM;
  const cz = Math.sin(th) * WALL_APOTHEM;
  const tx = -Math.sin(th);
  const tz = Math.cos(th); // tangente
  const isGate = i % GATE_EVERY === 0; // 0°, 90°, 180°, 270°
  place(isGate ? 'wall_straight_gate' : 'wall_straight', cx, cz, alongX(tx, tz), [wallScaleX, ...WALL_SCALE_YZ]);
  // Colisão: círculos ao longo do trecho (sem o vão do portão).
  for (let t = -edgeLen / 2; t <= edgeLen / 2 + 0.01; t += 1.4) {
    if (isGate && Math.abs(t) < GATE_HALF_OPENING) continue;
    collide(cx + tx * t, cz + tz * t, 1.7);
  }
  if (isGate) {
    // Bandeiras dos dois lados do portão, do lado de fora.
    for (const side of [-1, 1]) {
      const fx = cx + tx * side * (GATE_HALF_OPENING + 1.6) + Math.cos(th) * 3;
      const fz = cz + tz * side * (GATE_HALF_OPENING + 1.6) + Math.sin(th) * 3;
      place('flag_blue', fx, fz, alongX(tx, tz), 10);
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

// Poço no centro (a praça precisa de espaço para nascer e andar).
place('well', 0, 0, 0, 5);
collideItem('well', 0, 0, 5, 0.45);

// Construções principais nas diagonais, de frente para o centro.
const MAIN = [
  { k: 'tavern', deg: 45, r: 25 },
  { k: 'church', deg: 135, r: 25 },
  { k: 'blacksmith', deg: 225, r: 25 },
  { k: 'market', deg: 315, r: 25 },
];
for (const b of MAIN) {
  const a = (b.deg * Math.PI) / 180;
  const x = Math.cos(a) * b.r;
  const z = Math.sin(a) * b.r;
  place(b.k, x, z, facing(x, z, 0, 0), TOWN_SCALE);
  collideItem(b.k, x, z, TOWN_SCALE);
}
// Casas: um anel externo (entre as estradas e as diagonais) e um interno, em volta da praça.
const homes = [
  ...[22, 68, 112, 158, 202, 248, 292, 338].map((deg) => ({ deg, r: 29.5 })),
  ...[25, 65, 115, 155, 205, 245, 295, 335].map((deg) => ({ deg, r: 17.5 })),
];
for (const [i, h] of homes.entries()) {
  const a = (h.deg * Math.PI) / 180;
  const x = Math.cos(a) * h.r;
  const z = Math.sin(a) * h.r;
  const k = i % 2 === 0 ? 'home_a' : 'home_b';
  const s = HOME_SCALE * between(0.92, 1.08);
  place(k, x, z, facing(x, z, 0, 0), s);
  collideItem(k, x, z, s);
}
// Adereços (sem colisão) espalhados pela praça, perto das construções principais.
const props = [
  ['barrel', -10.4, -13.6], ['barrel', -11.6, -12.4], ['crate_a_big', -13.4, -9.4], ['weaponrack', -14.2, -7.6],
  ['crate_long_a', 12.6, -10.6], ['sack', 13.8, -9.0], ['sack', 13.2, -8.0], ['tent', 9.2, -12.6],
  ['tent', 12.4, -6.2], ['wheelbarrow', 9.6, 13.4], ['barrel', 13.4, 9.6], ['barrel', 12.6, 10.6],
  ['crate_a_big', -13.2, 9.8], ['sack', -9.8, 13.2],
];
// (posições encolhidas para caberem na praça, dentro do anel interno de casas)
for (const [k, x, z] of props) place(k, x * 0.68, z * 0.68, rand() * Math.PI * 2, k === 'tent' ? 7 : PROP_SCALE);

// Moinho fora da cidade, ao lado da estrada leste.
place('windmill', 56, 16, facing(56, 16, 56, 0), TOWN_SCALE);
collideItem('windmill', 56, 16, TOWN_SCALE);

// ---------- Estradas ----------
const roads = [
  { x1: WALL_APOTHEM, z1: 0, x2: HALF, z2: 0 },
  { x1: -WALL_APOTHEM, z1: 0, x2: -HALF, z2: 0 },
  { x1: 0, z1: WALL_APOTHEM, x2: 0, z2: HALF },
  { x1: 0, z1: -WALL_APOTHEM, x2: 0, z2: -206 }, // até a entrada do Vale das Pedras Vivas
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

const RING1 = [SAFE_RADIUS + 4, 76];
const RING2 = [76, 112];
const RING3 = [112, 212]; // inclui os cantos do quadrado

// Árvores: altas (~9–14 m, bem acima do personagem de 1,8 m) e mais espaçadas.
// O tronco engrossa menos que a copa, então o colisor cresce mais devagar que a escala.
scatter(85, ...RING1, (x, z) => {
  const s = between(2.1, 2.7);
  place(pick(LUSH), x, z, rand() * 6.28, s);
  collide(x, z, 0.3 * s);
}, { spacing: 7 });
scatter(80, ...RING2, (x, z) => {
  const bare = rand() < 0.35;
  const s = between(2.1, 2.8) * (bare ? 1.25 : 1);
  place(bare ? pick(BARE) : pick(LUSH), x, z, rand() * 6.28, s);
  collide(x, z, 0.28 * s);
}, { spacing: 7.5 });
scatter(100, ...RING3, (x, z) => {
  const s = between(2.6, 3.4);
  place(pick(BARE), x, z, rand() * 6.28, s);
  collide(x, z, 0.22 * s);
}, { spacing: 7.5 });

// Pedras grandes (bloqueiam) e pequenas (decoração)
scatter(20, ...RING1, (x, z) => {
  const s = between(1.6, 2.4);
  const k = pick(ROCK_BIG);
  place(k, x, z, rand() * 6.28, s);
  collideItem(k, x, z, s, 0.4);
}, { spacing: 3 });
scatter(38, ...RING2, (x, z) => {
  const s = between(1.8, 2.8);
  const k = pick(ROCK_BIG);
  place(k, x, z, rand() * 6.28, s);
  collideItem(k, x, z, s, 0.4);
}, { spacing: 3.5 });
scatter(85, ...RING3, (x, z) => {
  const s = between(2, 3.4);
  const k = pick(ROCK_BIG);
  place(k, x, z, rand() * 6.28, s);
  collideItem(k, x, z, s, 0.4);
}, { spacing: 4 });
scatter(110, SAFE_RADIUS + 2, 212, (x, z) => place(pick(ROCK_SMALL), x, z, rand() * 6.28, between(0.6, 1.3)), { avoidRoads: false });

// Arbustos e grama (sem colisão; muitos, desenhados com thin instances)
scatter(75, ...RING1, (x, z) => place(pick(BUSH), x, z, rand() * 6.28, between(2.2, 3.4)));
scatter(45, ...RING2, (x, z) => place(pick(BUSH), x, z, rand() * 6.28, between(2.2, 3.4)));
scatter(600, ...RING1, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.9, 1.6)), { avoidRoads: false });
scatter(350, ...RING2, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.8, 1.4)), { avoidRoads: false });
scatter(180, ...RING3, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.6, 1.1)), { avoidRoads: false });

// ---------- Borda: montanhas do lado de fora do quadrado ----------
const MOUNTAINS = ['mountain_a_grass_trees', 'mountain_b_grass', 'mountain_c_grass_trees', 'hills_a_trees', 'hills_b_trees'];
for (let t = -HALF - 24; t <= HALF + 24; t += 26) {
  for (const [x, z] of [[t, -HALF - 18], [t, HALF + 18], [-HALF - 18, t], [HALF + 18, t]]) {
    const k = pick(MOUNTAINS);
    const px = x + between(-4, 4);
    const pz = z + between(-4, 4);
    const rot = rand() * 6.28;
    const sc = between(20, 30);
    // A fileira sul ficaria dentro do vale novo (sorteia igual, para não mudar o resto).
    if (z === -HALF - 18) continue;
    place(k, px, pz, rot, sc);
  }
}

// ---------- Vale das Pedras Vivas (sul) ----------
// Sorteio próprio: nada do que já existia muda de lugar.
const brand = mulberry32(20261114);
const bpick = (arr) => arr[Math.floor(brand() * arr.length)];
const bbetween = (a, b) => a + brand() * (b - a);
const clamp01 = (v) => Math.max(0, Math.min(1, v));
const smoothstep = (a, b, v) => {
  const t = clamp01((v - a) / (b - a));
  return t * t * (3 - 2 * t);
};
// Borda ondulada entre a floresta e o vale (o cliente pinta o chão com a mesma conta).
const BIOME = { zFrom: -150, zTo: -205, amp: 14, ph: Array.from({ length: 4 }, () => round(brand() * 6.28)) };
const biomeNoise = (x, z) =>
  (Math.sin(x * 0.045 + BIOME.ph[0]) * Math.sin(z * 0.05 + BIOME.ph[1]) +
    0.5 * Math.sin(x * 0.11 + z * 0.07 + BIOME.ph[2]) +
    0.3 * Math.sin(x * 0.23 - z * 0.19 + BIOME.ph[3])) / 1.8;
/** 0 = floresta, 1 = vale de pedra. */
const rocky = (x, z) => smoothstep(BIOME.zFrom, BIOME.zTo, z + biomeNoise(x, z) * BIOME.amp);
const VALE = { x: 0, z: -270, r: 52 }; // área dos golens (igual ao servidor)
const VALE_STONE = { x: 6, z: -198 }; // Pedra de Teleporte (igual ao servidor)
const clearing = (x, z) => (Math.abs(x) < 6.5 && z > -214) || Math.hypot(x - VALE_STONE.x, z - VALE_STONE.z) < 10;
const inVale = (x, z) => Math.hypot(x - VALE.x, z - VALE.z) < VALE.r;
/** Tamanho no chão (maior lado) → escala do item. */
const sized = (k, meters) => meters / Math.max(SIZES[k][0], SIZES[k][2]);

function scatterSouth(count, zMin, zMax, accept, fn, { spacing = 0, tries = 30, avoidClearing = true } = {}) {
  let placed = 0;
  for (let i = 0; i < count * tries && placed < count; i++) {
    const x = bbetween(BOUNDS.minX + 2, BOUNDS.maxX - 2);
    const z = bbetween(zMin, zMax);
    if (avoidClearing && clearing(x, z)) continue;
    if (!accept(x, z)) continue;
    if (spacing > 0 && (!farFrom(x, z, spacing) || !freeOfColliders(x, z, spacing))) continue;
    fn(x, z);
    if (spacing > 0) taken.push({ x, z, d: spacing });
    placed++;
  }
}

const BARE_ALL = [...BARE, 'tree_bare_1_c', 'tree_bare_2_c'];
const CLIFF = ['rock_1_k', 'rock_2_f', 'rock_3_g', 'rock_3_j', 'rock_1_h'];
const BOULDER = ['rock_3_a', 'rock_3_e', 'rock_3_b', 'rock_3_c', 'rock_3_g', 'rock_3_j', 'rock_1_h'];
const STONE_MID = ['rock_1_a', 'rock_1_c', 'rock_1_e', 'rock_3_b', 'rock_3_c'];
const GRAVEL = ['rock_2_a', 'rock_2_c', 'rock_1_c'];
const MIN_Z = BOUNDS.minZ + 3;

// Paredões: cristas de pedras grandes nas laterais e no fundo, com falhas (passagens).
const RIDGES = [
  [[-62, -214], [-74, -245], [-88, -280], [-96, -318]],
  [[60, -218], [76, -250], [92, -286], [102, -320]],
  [[-58, -323], [-20, -326], [20, -325], [56, -322]],
  [[-112, -232], [-128, -248], [-141, -266]],
  [[116, -226], [131, -244], [143, -262]],
  [[-138, -296], [-120, -312]],
  [[134, -300], [118, -316]],
];
for (const ridge of RIDGES) {
  for (let i = 0; i < ridge.length - 1; i++) {
    const [ax, az] = ridge[i];
    const [bx, bz] = ridge[i + 1];
    const len = Math.hypot(bx - ax, bz - az);
    for (let d = 0; d < len; d += bbetween(2.8, 4.2)) {
      const t = d / len;
      const k = bpick(CLIFF);
      const m = bbetween(4, 7.5);
      const s = sized(k, m);
      const x = ax + (bx - ax) * t + bbetween(-2, 2);
      const z = az + (bz - az) * t + bbetween(-2, 2);
      if (brand() < 0.12 || clearing(x, z)) continue; // falha na crista
      place(k, x, z, brand() * 6.28, [s * bbetween(1, 1.35), s * bbetween(0.9, 1.5), s]);
      collide(x, z, m * 0.42);
      taken.push({ x, z, d: m * 0.4 });
    }
  }
}

// Monólitos rúnicos: círculos antigos, alguns inteiros, outros partidos ou tombados.
// (desenhados pelo cliente, sem modelo do kit; os tamanhos abaixo são a escala 1)
SIZES.monolith = [1.1, 2.6, 1.1, 0];
SIZES.monolith_broken = [1.1, 1.4, 1.1, 0];
SIZES.monolith_fallen = [2.6, 0.9, 1.1, 0];
const CIRCLES = [
  { x: -36, z: -244, r: 6.5, n: 7 },
  { x: 40, z: -258, r: 7, n: 7 },
  { x: -10, z: -302, r: 7.5, n: 8 },
  { x: -104, z: -284, r: 5.5, n: 5 },
  { x: 108, z: -272, r: 5.5, n: 5 },
];
for (const c of CIRCLES) {
  const a0 = brand() * 6.28;
  for (let i = 0; i < c.n; i++) {
    const a = a0 + (i / c.n) * Math.PI * 2 + bbetween(-0.12, 0.12);
    const x = c.x + Math.cos(a) * c.r;
    const z = c.z + Math.sin(a) * c.r;
    if (!freeOfColliders(x, z, 1)) continue;
    const roll = brand();
    const k = roll < 0.45 ? 'monolith' : roll < 0.75 ? 'monolith_broken' : 'monolith_fallen';
    const s = bbetween(0.9, 1.25);
    // De pé, olha para o centro; tombado, cai para fora.
    const rot = k === 'monolith_fallen' ? -a + bbetween(-0.4, 0.4) : facing(x, z, c.x, c.z) + bbetween(-0.2, 0.2);
    place(k, x, z, rot, [s, s * bbetween(0.9, 1.15), s]);
    collide(x, z, k === 'monolith_fallen' ? 1.1 * s : 0.75 * s);
    taken.push({ x, z, d: 1.5 });
  }
  // Altar no meio: o toco largo de um monólito antigo, partido rente ao chão.
  const s = bbetween(1.5, 1.8);
  place('monolith_broken', c.x, c.z, brand() * 6.28, [s, s * 0.6, s]);
  collide(c.x, c.z, 0.8 * s);
  taken.push({ x: c.x, z: c.z, d: 2 });
}

// Transição: a floresta seca e rareia, as pedras aumentam.
scatterSouth(70, -215, -150, (x, z) => brand() < (1 - rocky(x, z)) * 0.95, (x, z) => {
  const lush = brand() < 0.22 * (1 - rocky(x, z));
  const s = bbetween(2.4, 3.2) * (lush ? 1 : 1.15);
  place(lush ? bpick(LUSH) : bpick(BARE_ALL), x, z, brand() * 6.28, s);
  collide(x, z, 0.22 * s);
}, { spacing: 7.5 });
// Árvores mortas solitárias no vale.
scatterSouth(16, MIN_Z, -210, () => true, (x, z) => {
  const s = bbetween(2.8, 3.8);
  place(bpick(BARE_ALL), x, z, brand() * 6.28, s);
  collide(x, z, 0.22 * s);
}, { spacing: 14 });
// Pedras grandes (menos dentro da área de caça, para dar espaço à luta).
scatterSouth(120, MIN_Z, -158, (x, z) => brand() < rocky(x, z) * (inVale(x, z) ? 0.3 : 1), (x, z) => {
  const k = bpick(BOULDER);
  const m = bbetween(1.6, 4.2);
  const s = sized(k, m);
  place(k, x, z, brand() * 6.28, [s, s * bbetween(0.8, 1.25), s]);
  collide(x, z, m * 0.4);
}, { spacing: 4 });
// Pedras médias (as maiores bloqueiam).
scatterSouth(170, MIN_Z, -155, (x, z) => brand() < 0.15 + rocky(x, z) * 0.85, (x, z) => {
  const k = bpick(STONE_MID);
  const m = bbetween(0.6, 1.8);
  place(k, x, z, brand() * 6.28, sized(k, m));
  if (m > 1.2) collide(x, z, m * 0.38);
}, { spacing: 2 });
// Cascalho (só visual).
scatterSouth(700, MIN_Z, -152, (x, z) => brand() < rocky(x, z), (x, z) => {
  const k = bpick(GRAVEL);
  place(k, x, z, brand() * 6.28, sized(k, bbetween(0.18, 0.55)));
}, { avoidClearing: false });
// Arbustos e grama rareando; capim baixo e esparso no vale.
scatterSouth(40, -210, -150, (x, z) => brand() < (1 - rocky(x, z)) * 0.9, (x, z) => place(bpick(BUSH), x, z, brand() * 6.28, bbetween(2, 3)));
scatterSouth(340, -232, -150, (x, z) => brand() < (1 - rocky(x, z)) * 0.9 + 0.05, (x, z) => place(bpick(GRASS), x, z, brand() * 6.28, bbetween(0.6, 1.2)), { avoidClearing: false });
scatterSouth(130, MIN_Z, -215, () => true, (x, z) => place(bpick(GRASS), x, z, brand() * 6.28, bbetween(0.45, 0.85)), { avoidClearing: false });

// Montanhas de rocha nua em volta do vale.
const ROCK_MOUNTAINS = ['mountain_a', 'mountain_b', 'mountain_c', 'mountain_a_grass', 'hills_a', 'hills_c'];
for (let z = -176; z >= BOUNDS.minZ - 26; z -= 26) {
  for (const x of [BOUNDS.minX - 18, BOUNDS.maxX + 18]) {
    place(bpick(ROCK_MOUNTAINS), x + bbetween(-4, 4), z + bbetween(-4, 4), brand() * 6.28, bbetween(20, 30));
  }
}
for (let x = BOUNDS.minX - 24; x <= BOUNDS.maxX + 24; x += 26) {
  place(bpick(ROCK_MOUNTAINS), x + bbetween(-4, 4), BOUNDS.minZ - 18 + bbetween(-4, 4), brand() * 6.28, bbetween(22, 32));
}

// ---------- Tochas (acendem à noite) ----------
// Posições fixas (sem sortear: não muda a sequência do PRNG do resto do mapa). Ladeiam as
// avenidas dentro da muralha, os portões por fora e o poço; descarta o que bate em algo.
const torches = [];
const torchFree = (x, z) =>
  freeOfColliders(x, z, 1.2) &&
  objects.every((o) => o.k.startsWith('grass_') || Math.hypot(o.x - x, o.z - z) > 2.2) &&
  torches.every((t) => Math.hypot(t.x - x, t.z - z) > 4);
const torchCandidates = [];
for (const a of [0, 90, 180, 270]) {
  const th = (a * Math.PI) / 180;
  const [ux, uz, nx, nz] = [Math.cos(th), Math.sin(th), -Math.sin(th), Math.cos(th)];
  for (const r of [13, 21.5, 31]) for (const side of [-1, 1]) torchCandidates.push([ux * r + nx * side * 3.6, uz * r + nz * side * 3.6]);
  for (const side of [-1, 1]) torchCandidates.push([ux * 46 + nx * side * 3.8, uz * 46 + nz * side * 3.8]);
}
for (const a of [45, 135, 225, 315]) {
  const th = (a * Math.PI) / 180;
  torchCandidates.push([Math.cos(th) * 6.5, Math.sin(th) * 6.5]);
}
for (const [x, z] of torchCandidates) {
  if (!torchFree(x, z)) continue;
  torches.push({ x: round(x), z: round(z) });
  collide(x, z, 0.3);
}

// ---------- Cripta Esquecida (covil do raid boss, nordeste) ----------
// Sorteio próprio (nada do resto muda de lugar). A arena fica limpa: o chefe e quem luta
// com ele andam livres num raio de 24 m. Entrada virada para a cidade.
const LAIR = { x: 108, z: 108, r: 24 }; // igual ao servidor (game.RaidLairX/Z/Radius)
const lrand = mulberry32(20261115);
const lpick = (arr) => arr[Math.floor(lrand() * arr.length)];
const lbetween = (a, b) => a + lrand() * (b - a);
const lairDist = (o) => Math.hypot(o.x - LAIR.x, o.z - LAIR.z);
for (const arr of [objects, colliders, torches]) {
  for (let i = arr.length - 1; i >= 0; i--) if (lairDist(arr[i]) < LAIR.r + 4) arr.splice(i, 1);
}
const ENTRANCE = Math.atan2(-LAIR.z, -LAIR.x); // ângulo que aponta para a cidade
const angleGap = (a, b) => Math.abs(Math.atan2(Math.sin(a - b), Math.cos(a - b)));
const lairAt = (a, r) => [LAIR.x + Math.cos(a) * r, LAIR.z + Math.sin(a) * r];
// Círculo de monólitos, uns de pé, outros partidos ou tombados (com a entrada aberta).
for (let i = 0; i < 16; i++) {
  const a = ENTRANCE + (i / 16) * Math.PI * 2 + lbetween(-0.06, 0.06);
  if (angleGap(a, ENTRANCE) < 0.42) continue;
  const [x, z] = lairAt(a, LAIR.r - 1.5);
  const roll = lrand();
  const k = roll < 0.5 ? 'monolith' : roll < 0.8 ? 'monolith_broken' : 'monolith_fallen';
  const sc = lbetween(1.3, 1.7);
  const rot = k === 'monolith_fallen' ? -a + lbetween(-0.3, 0.3) : facing(x, z, LAIR.x, LAIR.z) + lbetween(-0.15, 0.15);
  place(k, x, z, rot, [sc, sc * lbetween(1, 1.25), sc]);
  collide(x, z, k === 'monolith_fallen' ? 1.1 * sc : 0.75 * sc);
}
// Pilares da entrada: dois rochedos grandes.
for (const side of [-1, 1]) {
  const [x, z] = lairAt(ENTRANCE + side * 0.3, LAIR.r + 0.5);
  const k = lpick(['rock_3_g', 'rock_3_j']);
  const sc = lbetween(2.6, 3);
  place(k, x, z, lrand() * 6.28, sc);
  collideItem(k, x, z, sc, 0.4);
}
// Trono: rochedo largo atrás do centro, ladeado por dois monólitos altos.
{
  const back = ENTRANCE + Math.PI;
  const [x, z] = lairAt(back, 9);
  place('rock_3_c', x, z, facing(x, z, LAIR.x, LAIR.z), 3.2);
  collideItem('rock_3_c', x, z, 3.2, 0.4);
  for (const side of [-1, 1]) {
    const [mx, mz] = lairAt(back + side * 0.38, 10);
    place('monolith', mx, mz, facing(mx, mz, LAIR.x, LAIR.z), [1.8, 2.3, 1.8]);
    collide(mx, mz, 0.75 * 1.8);
  }
}
// Do lado de fora: muralha de pedras e árvores mortas (a entrada fica livre).
for (let i = 0; i < 14; i++) {
  const a = ENTRANCE + (i / 14) * Math.PI * 2 + lbetween(-0.1, 0.1);
  if (angleGap(a, ENTRANCE) < 0.5) continue;
  const [x, z] = lairAt(a, lbetween(LAIR.r + 4, LAIR.r + 8));
  const k = lpick(['rock_3_a', 'rock_3_e', 'rock_3_b', 'rock_3_g']);
  const sc = lbetween(2.2, 3.2);
  place(k, x, z, lrand() * 6.28, sc);
  collideItem(k, x, z, sc, 0.4);
}
for (let i = 0; i < 10; i++) {
  const a = ENTRANCE + lbetween(0.6, Math.PI * 2 - 0.6);
  const [x, z] = lairAt(a, lbetween(LAIR.r + 9, LAIR.r + 14));
  if (!freeOfColliders(x, z, 2)) continue;
  const sc = lbetween(2.8, 3.6);
  place(lpick(['tree_bare_1_a', 'tree_bare_1_b', 'tree_bare_2_a', 'tree_bare_2_b']), x, z, lrand() * 6.28, sc);
  collide(x, z, 0.22 * sc);
}
// Chão: cascalho e lascas de pedra (sem colisão).
for (let i = 0; i < 26; i++) {
  const [x, z] = lairAt(lrand() * 6.28, Math.sqrt(lrand()) * (LAIR.r - 3));
  place(lpick(['rock_2_a', 'rock_1_a', 'rock_1_e']), x, z, lrand() * 6.28, lbetween(0.5, 1));
}
// Tochas em volta da arena (e duas na entrada).
for (let i = 0; i < 8; i++) {
  const a = ENTRANCE + ((i + 0.5) / 8) * Math.PI * 2;
  if (angleGap(a, ENTRANCE) < 0.4) continue;
  const [x, z] = lairAt(a, LAIR.r - 5);
  torches.push({ x: round(x), z: round(z) });
  collide(x, z, 0.3);
}
for (const side of [-1, 1]) {
  const [x, z] = lairAt(ENTRANCE + side * 0.19, LAIR.r + 1.5);
  torches.push({ x: round(x), z: round(z) });
  collide(x, z, 0.3);
}

// ---------- Saída ----------
const world = {
  version: 1,
  size: SIZE,
  bounds: BOUNDS,
  biome: { name: 'Vale das Pedras Vivas', ...BIOME, vale: VALE },
  safeRadius: SAFE_RADIUS,
  wall: { apothem: WALL_APOTHEM, sides: WALL_SIDES },
  roads,
  colliders,
  objects,
  torches,
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
