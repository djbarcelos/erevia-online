// Floresta de Vel'Aran: a cidade murada no centro, as estradas, a floresta em anéis e as
// montanhas da borda. É o quadrado original de 300 m (±150).
export const name = "Floresta de Vel'Aran";
export const seed = 20261113;
export const order = 10;

const HALF = 150;
export const bounds = { minX: -HALF, maxX: HALF, minZ: -HALF, maxZ: HALF };

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

// ---------- Malhas ----------
const LUSH = ['tree_1_a', 'tree_1_b', 'tree_2_a', 'tree_2_b', 'tree_3_a', 'tree_4_a', 'tree_4_b'];
const BARE = ['tree_bare_1_a', 'tree_bare_1_b', 'tree_bare_2_a', 'tree_bare_2_b'];
const BUSH = ['bush_1_a', 'bush_2_a', 'bush_3_a', 'bush_4_a'];
const GRASS = ['grass_1_a', 'grass_2_a'];
const ROCK_SMALL = ['rock_1_a', 'rock_1_e', 'rock_2_a'];
const ROCK_BIG = ['rock_3_a', 'rock_3_e'];
const MOUNTAINS = ['mountain_a_grass_trees', 'mountain_b_grass', 'mountain_c_grass_trees', 'hills_a_trees', 'hills_b_trees'];

// ---------- Landmarks: as construções principais nas diagonais, de frente para o centro ----------
const MAIN = [
  { k: 'tavern', deg: 45, r: 25 },
  { k: 'church', deg: 135, r: 25 },
  { k: 'blacksmith', deg: 225, r: 25 },
  { k: 'market', deg: 315, r: 25 },
];
// Adereços (sem colisão) espalhados pela praça, perto das construções principais.
const PROPS = [
  ['barrel', -10.4, -13.6], ['barrel', -11.6, -12.4], ['crate_a_big', -13.4, -9.4], ['weaponrack', -14.2, -7.6],
  ['crate_long_a', 12.6, -10.6], ['sack', 13.8, -9.0], ['sack', 13.2, -8.0], ['tent', 9.2, -12.6],
  ['tent', 12.4, -6.2], ['wheelbarrow', 9.6, 13.4], ['barrel', 13.4, 9.6], ['barrel', 12.6, 10.6],
  ['crate_a_big', -13.2, 9.8], ['sack', -9.8, 13.2],
];

const RING1 = [SAFE_RADIUS + 4, 76];
const RING2 = [76, 112];
const RING3 = [112, 212]; // inclui os cantos do quadrado

const onRoad = (x, z) => (Math.abs(x) < ROAD_CLEAR || Math.abs(z) < ROAD_CLEAR);

export function build(ctx) {
  const { SIZES, place, collide, collideItem, alongX, facing, rand, pick, between } = ctx;
  ctx.meta.safeRadius = SAFE_RADIUS;
  ctx.meta.wall = { apothem: WALL_APOTHEM, sides: WALL_SIDES };

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
  // (posições encolhidas para caberem na praça, dentro do anel interno de casas)
  for (const [k, x, z] of PROPS) place(k, x * 0.68, z * 0.68, rand() * Math.PI * 2, k === 'tent' ? 7 : PROP_SCALE);

  // Moinho fora da cidade, ao lado da estrada leste.
  place('windmill', 56, 16, facing(56, 16, 56, 0), TOWN_SCALE);
  collideItem('windmill', 56, 16, TOWN_SCALE);

  // ---------- Estradas ----------
  ctx.roads.push(
    ...[
      { x1: WALL_APOTHEM, z1: 0, x2: HALF, z2: 0 },
      { x1: -WALL_APOTHEM, z1: 0, x2: -HALF, z2: 0 },
      { x1: 0, z1: WALL_APOTHEM, x2: 0, z2: HALF },
      { x1: 0, z1: -WALL_APOTHEM, x2: 0, z2: -206 }, // até a entrada do Vale das Pedras Vivas
    ].map((r) => ({ ...r, w: ROAD_WIDTH })),
  );

  // ---------- Floresta (por anel) ----------
  const ring = ([rMin, rMax], avoidRoads = true) => {
    const accept = (x, z) => {
      const r = Math.hypot(x, z);
      if (r < rMin || r > rMax) return false;
      return !(avoidRoads && onRoad(x, z));
    };
    return { x: [-HALF + 2, HALF - 2], z: [-HALF + 2, HALF - 2], accept };
  };
  const scatter = (count, rr, fn, { spacing = 0, avoidRoads = true } = {}) =>
    ctx.scatter(count, { ...ring(rr, avoidRoads), spacing }, fn);

  // Árvores: altas (~9–14 m, bem acima do personagem de 1,8 m) e mais espaçadas.
  // O tronco engrossa menos que a copa, então o colisor cresce mais devagar que a escala.
  scatter(85, RING1, (x, z) => {
    const s = between(2.1, 2.7);
    place(pick(LUSH), x, z, rand() * 6.28, s);
    collide(x, z, 0.3 * s);
  }, { spacing: 7 });
  scatter(80, RING2, (x, z) => {
    const bare = rand() < 0.35;
    const s = between(2.1, 2.8) * (bare ? 1.25 : 1);
    place(bare ? pick(BARE) : pick(LUSH), x, z, rand() * 6.28, s);
    collide(x, z, 0.28 * s);
  }, { spacing: 7.5 });
  scatter(100, RING3, (x, z) => {
    const s = between(2.6, 3.4);
    place(pick(BARE), x, z, rand() * 6.28, s);
    collide(x, z, 0.22 * s);
  }, { spacing: 7.5 });

  // Pedras grandes (bloqueiam) e pequenas (decoração)
  scatter(20, RING1, (x, z) => {
    const s = between(1.6, 2.4);
    const k = pick(ROCK_BIG);
    place(k, x, z, rand() * 6.28, s);
    collideItem(k, x, z, s, 0.4);
  }, { spacing: 3 });
  scatter(38, RING2, (x, z) => {
    const s = between(1.8, 2.8);
    const k = pick(ROCK_BIG);
    place(k, x, z, rand() * 6.28, s);
    collideItem(k, x, z, s, 0.4);
  }, { spacing: 3.5 });
  scatter(85, RING3, (x, z) => {
    const s = between(2, 3.4);
    const k = pick(ROCK_BIG);
    place(k, x, z, rand() * 6.28, s);
    collideItem(k, x, z, s, 0.4);
  }, { spacing: 4 });
  scatter(110, [SAFE_RADIUS + 2, 212], (x, z) => place(pick(ROCK_SMALL), x, z, rand() * 6.28, between(0.6, 1.3)), { avoidRoads: false });

  // Arbustos e grama (sem colisão; muitos, desenhados com thin instances)
  scatter(75, RING1, (x, z) => place(pick(BUSH), x, z, rand() * 6.28, between(2.2, 3.4)));
  scatter(45, RING2, (x, z) => place(pick(BUSH), x, z, rand() * 6.28, between(2.2, 3.4)));
  scatter(600, RING1, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.9, 1.6)), { avoidRoads: false });
  scatter(350, RING2, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.8, 1.4)), { avoidRoads: false });
  scatter(180, RING3, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.6, 1.1)), { avoidRoads: false });

  // ---------- Borda: montanhas do lado de fora do quadrado ----------
  for (let t = -HALF - 24; t <= HALF + 24; t += 26) {
    for (const [x, z] of [[t, -HALF - 18], [t, HALF + 18], [-HALF - 18, t], [HALF + 18, t]]) {
      const k = pick(MOUNTAINS);
      const px = x + between(-4, 4);
      const pz = z + between(-4, 4);
      const rot = rand() * 6.28;
      const sc = between(20, 30);
      // A fileira sul ficaria dentro do vale (sorteia igual, para não mudar o resto).
      if (z === -HALF - 18) continue;
      place(k, px, pz, rot, sc);
    }
  }
}

// ---------- Tochas (acendem à noite) ----------
// Posições fixas (sem sortear). Ladeiam as avenidas dentro da muralha, os portões por fora e
// o poço; descarta o que bate em algo (de qualquer bioma: por isso é a fase `lights`).
export function lights(ctx) {
  const { objects, torches, freeOfColliders } = ctx;
  const torchFree = (x, z) =>
    freeOfColliders(x, z, 1.2) &&
    objects.every((o) => o.k.startsWith('grass_') || Math.hypot(o.x - x, o.z - z) > 2.2) &&
    torches.every((t) => Math.hypot(t.x - x, t.z - z) > 4);
  const candidates = [];
  for (const a of [0, 90, 180, 270]) {
    const th = (a * Math.PI) / 180;
    const [ux, uz, nx, nz] = [Math.cos(th), Math.sin(th), -Math.sin(th), Math.cos(th)];
    for (const r of [13, 21.5, 31]) for (const side of [-1, 1]) candidates.push([ux * r + nx * side * 3.6, uz * r + nz * side * 3.6]);
    for (const side of [-1, 1]) candidates.push([ux * 46 + nx * side * 3.8, uz * 46 + nz * side * 3.8]);
  }
  for (const a of [45, 135, 225, 315]) {
    const th = (a * Math.PI) / 180;
    candidates.push([Math.cos(th) * 6.5, Math.sin(th) * 6.5]);
  }
  for (const [x, z] of candidates) {
    if (!torchFree(x, z)) continue;
    ctx.torch(x, z);
  }
}
