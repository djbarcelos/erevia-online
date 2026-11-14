// Cripta Esquecida (covil do raid boss, nordeste): uma arena limpa no meio da floresta. Roda
// na fase `overlay`: limpa o que a floresta pôs ali e se constrói por cima. O chefe e quem
// luta com ele andam livres num raio de 24 m; a entrada fica virada para a cidade.
export const name = 'Cripta Esquecida';
export const seed = 20261115;
export const order = 30;

const LAIR = { x: 108, z: 108, r: 24 }; // igual ao servidor (game.RaidLairX/Z/Radius)
const OUTER = LAIR.r + 14; // até onde vão as árvores mortas de fora
export const bounds = { minX: LAIR.x - OUTER, maxX: LAIR.x + OUTER, minZ: LAIR.z - OUTER, maxZ: LAIR.z + OUTER };

// ---------- Malhas ----------
const PILLARS = ['rock_3_g', 'rock_3_j'];
const WALL_ROCKS = ['rock_3_a', 'rock_3_e', 'rock_3_b', 'rock_3_g'];
const DEAD_TREES = ['tree_bare_1_a', 'tree_bare_1_b', 'tree_bare_2_a', 'tree_bare_2_b'];
const CHIPS = ['rock_2_a', 'rock_1_a', 'rock_1_e'];

const ENTRANCE = Math.atan2(-LAIR.z, -LAIR.x); // ângulo que aponta para a cidade
const angleGap = (a, b) => Math.abs(Math.atan2(Math.sin(a - b), Math.cos(a - b)));
const lairAt = (a, r) => [LAIR.x + Math.cos(a) * r, LAIR.z + Math.sin(a) * r];

export function overlay(ctx) {
  const { place, collide, collideItem, freeOfColliders, facing, torch, rand, pick, between } = ctx;
  ctx.clear(LAIR.x, LAIR.z, LAIR.r + 4);

  // Círculo de monólitos, uns de pé, outros partidos ou tombados (com a entrada aberta).
  for (let i = 0; i < 16; i++) {
    const a = ENTRANCE + (i / 16) * Math.PI * 2 + between(-0.06, 0.06);
    if (angleGap(a, ENTRANCE) < 0.42) continue;
    const [x, z] = lairAt(a, LAIR.r - 1.5);
    const roll = rand();
    const k = roll < 0.5 ? 'monolith' : roll < 0.8 ? 'monolith_broken' : 'monolith_fallen';
    const sc = between(1.3, 1.7);
    const rot = k === 'monolith_fallen' ? -a + between(-0.3, 0.3) : facing(x, z, LAIR.x, LAIR.z) + between(-0.15, 0.15);
    place(k, x, z, rot, [sc, sc * between(1, 1.25), sc]);
    collide(x, z, k === 'monolith_fallen' ? 1.1 * sc : 0.75 * sc);
  }
  // Pilares da entrada: dois rochedos grandes.
  for (const side of [-1, 1]) {
    const [x, z] = lairAt(ENTRANCE + side * 0.3, LAIR.r + 0.5);
    const k = pick(PILLARS);
    const sc = between(2.6, 3);
    place(k, x, z, rand() * 6.28, sc);
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
    const a = ENTRANCE + (i / 14) * Math.PI * 2 + between(-0.1, 0.1);
    if (angleGap(a, ENTRANCE) < 0.5) continue;
    const [x, z] = lairAt(a, between(LAIR.r + 4, LAIR.r + 8));
    const k = pick(WALL_ROCKS);
    const sc = between(2.2, 3.2);
    place(k, x, z, rand() * 6.28, sc);
    collideItem(k, x, z, sc, 0.4);
  }
  for (let i = 0; i < 10; i++) {
    const a = ENTRANCE + between(0.6, Math.PI * 2 - 0.6);
    const [x, z] = lairAt(a, between(LAIR.r + 9, OUTER));
    if (!freeOfColliders(x, z, 2)) continue;
    const sc = between(2.8, 3.6);
    place(pick(DEAD_TREES), x, z, rand() * 6.28, sc);
    collide(x, z, 0.22 * sc);
  }
  // Chão: cascalho e lascas de pedra (sem colisão).
  for (let i = 0; i < 26; i++) {
    const [x, z] = lairAt(rand() * 6.28, Math.sqrt(rand()) * (LAIR.r - 3));
    place(pick(CHIPS), x, z, rand() * 6.28, between(0.5, 1));
  }
  // Tochas em volta da arena (e duas na entrada).
  for (let i = 0; i < 8; i++) {
    const a = ENTRANCE + ((i + 0.5) / 8) * Math.PI * 2;
    if (angleGap(a, ENTRANCE) < 0.4) continue;
    const [x, z] = lairAt(a, LAIR.r - 5);
    torch(x, z);
  }
  for (const side of [-1, 1]) {
    const [x, z] = lairAt(ENTRANCE + side * 0.19, LAIR.r + 1.5);
    torch(x, z);
  }
}
