// Bosque Gris (noroeste, níveis 7–15; roadmap 5.1, tunnel#137): o setor NO dos anéis 2 e 3 (de 100° a 170°, raio de 80 a 150 m)
// vira uma floresta fechada de carvalhos velhos com musgo cinza-prateado nos troncos, com as árvores mais juntas que nos
// Campos (5 m em vez de 7,5 m). A estrada do oeste ficou perigosa: o Bando do Corvo, desertores da guarda de Vel'Aran,
// assalta as caravanas. No canto NO do quadrado (r > 150) fica a Toca do Corvo, o acampamento com paliçada e o chefe.
//
// O servidor lê as mesmas coordenadas em game/bosque.go (o setor e a Toca). Roda em duas fases: `build` (as árvores a mais,
// que respeitam o que a floresta já pôs) e `overlay` (a Toca limpa o lugar e se constrói por cima).
export const name = 'Bosque Gris';
export const seed = 20261160;
export const order = 35;

// ---------- Lugares (iguais ao servidor, game/bosque.go) ----------
const SECTOR = { rMin: 80, rMax: 150, a0: 100, a1: 170 }; // o setor, em graus (0° = leste, para o norte é +z)
const DEN = { x: -128, z: 118 }; // a Toca do Corvo
const GALLOWS = { x: -52, z: 84 }; // a Árvore-Forca, o landmark (um carvalho morto de 19 m, visto dos Campos)
const WAGON = { x: -33, z: 90 }; // a Carroça Tombada, o começo da história da caravana
const PACK_DEN = { x: -86, z: 52 }; // a Toca da Matilha: uma clareira de ossos
export const bounds = { minX: -150, maxX: -10, minZ: 10, maxZ: 150 };

// ---------- Malhas (kits que já existem) ----------
const OAK = ['tree_1_a', 'tree_1_b', 'tree_2_a', 'tree_2_b', 'tree_3_a', 'tree_4_a', 'tree_4_b'];
const BARE = ['tree_bare_1_a', 'tree_bare_1_b', 'tree_bare_2_a', 'tree_bare_2_b'];
const BUSH = ['bush_1_a', 'bush_2_a', 'bush_3_a', 'bush_4_a'];
const GRASS = ['grass_1_a', 'grass_2_a'];
const ROCK = ['rock_1_a', 'rock_1_e', 'rock_2_a'];

const angleOf = (x, z) => ((Math.atan2(z, x) * 180) / Math.PI + 360) % 360;
/** Dentro do setor do Bosque (a folga `m` em metros alarga o raio). */
const inSector = (x, z, m = 0) => {
  const r = Math.hypot(x, z);
  const a = angleOf(x, z);
  return r >= SECTOR.rMin - m && r <= SECTOR.rMax + m && a >= SECTOR.a0 && a <= SECTOR.a1;
};
const near = (x, z, p, d) => Math.hypot(x - p.x, z - p.z) < d;
const alongZ = (dx, dz) => Math.atan2(dx, dz); // o eixo Z do modelo na direção (dx, dz) (a cerca corre ao longo de Z)

export function build(ctx) {
  const { place, collide, rand, pick, between, scatter } = ctx;
  const accept = (x, z) =>
    inSector(x, z) && Math.abs(x) > 6 && !near(x, z, GALLOWS, 7) && !near(x, z, WAGON, 8) && !near(x, z, PACK_DEN, 9) && !near(x, z, DEN, 24);
  const ground = { x: [bounds.minX, bounds.maxX], z: [bounds.minZ, bounds.maxZ] };

  // Carvalhos velhos, juntos (5 m), com uns troncos secos no meio.
  scatter(70, { ...ground, accept, spacing: 5 }, (x, z) => {
    const s = between(2.3, 3.3);
    place(pick(OAK), x, z, rand() * 6.28, s);
    collide(x, z, 0.3 * s);
  });
  scatter(14, { ...ground, accept, spacing: 5.5 }, (x, z) => {
    const s = between(2.4, 3.2);
    place(pick(BARE), x, z, rand() * 6.28, s);
    collide(x, z, 0.24 * s);
  });
  // Pouca luz no chão: mato fechado e pedras com musgo.
  scatter(60, { ...ground, accept }, (x, z) => place(pick(BUSH), x, z, rand() * 6.28, between(2.2, 3.4)));
  scatter(260, { ...ground, accept }, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.7, 1.3)));
  scatter(26, { ...ground, accept, spacing: 3 }, (x, z) => place(pick(ROCK), x, z, rand() * 6.28, between(0.7, 1.5)));
}

export function overlay(ctx) {
  const { place, collide, torch, facing, rand, pick, between } = ctx;

  // ---------- A Árvore-Forca: um carvalho morto de uns 19 m, cheio de corvos, o landmark do Bosque ----------
  ctx.clear(GALLOWS.x, GALLOWS.z, 6);
  place('tree_bare_1_a', GALLOWS.x, GALLOWS.z, rand() * 6.28, 6.6);
  collide(GALLOWS.x, GALLOWS.z, 1.7);
  for (let i = 0; i < 5; i++) {
    const a = (i / 5) * Math.PI * 2 + between(-0.3, 0.3);
    const x = GALLOWS.x + Math.cos(a) * between(3.5, 5);
    const z = GALLOWS.z + Math.sin(a) * between(3.5, 5);
    place(pick(ROCK), x, z, rand() * 6.28, between(0.8, 1.3));
  }

  // ---------- A Carroça Tombada: o começo da história da caravana de Caio ----------
  ctx.clear(WAGON.x, WAGON.z, 5);
  place('wheelbarrow', WAGON.x, WAGON.z, rand() * 6.28, 8);
  collide(WAGON.x, WAGON.z, 1.6);
  for (const [k, dx, dz, s] of [['crate_a_big', 2.6, 1.4, 5], ['crate_long_a', -2.4, 2, 4.5], ['barrel', 1.2, -2.6, 4.5], ['sack', -1.6, -1.4, 4.5], ['sack', 3.4, -1.2, 4.5]]) {
    place(k, WAGON.x + dx, WAGON.z + dz, rand() * 6.28, s);
  }

  // ---------- A Toca da Matilha: uma clareira de ossos (pedras pequenas e uma árvore seca) ----------
  ctx.clear(PACK_DEN.x, PACK_DEN.z, 7);
  for (let i = 0; i < 9; i++) {
    const a = rand() * 6.28;
    const r = between(1.5, 5.5);
    place(pick(ROCK), PACK_DEN.x + Math.cos(a) * r, PACK_DEN.z + Math.sin(a) * r, rand() * 6.28, between(0.5, 1.1));
  }
  place('tree_bare_2_a', PACK_DEN.x + 4, PACK_DEN.z - 3, rand() * 6.28, 3.2);
  collide(PACK_DEN.x + 4, PACK_DEN.z - 3, 0.8);

  // ---------- A Toca do Corvo: o acampamento com paliçada, seis tendas e o chefe ----------
  const R = 13.5;
  ctx.clear(DEN.x, DEN.z, R + 4);
  const gate = Math.atan2(-DEN.z, -DEN.x); // o portão da paliçada olha para a cidade
  const pieces = 22;
  for (let i = 0; i < pieces; i++) {
    const a = (i / pieces) * Math.PI * 2;
    const gap = Math.abs(Math.atan2(Math.sin(a - gate), Math.cos(a - gate)));
    if (gap < 0.3) continue; // a abertura
    const x = DEN.x + Math.cos(a) * R;
    const z = DEN.z + Math.sin(a) * R;
    place('fence_wood_straight', x, z, alongZ(-Math.sin(a), Math.cos(a)), 3.9);
    collide(x, z, 1.1);
    collide(x + -Math.sin(a) * 1.9, z + Math.cos(a) * 1.9, 1.1);
  }
  // As tendas em volta da fogueira, viradas para o meio.
  for (let i = 0; i < 6; i++) {
    const a = gate + 0.9 + (i / 6) * (Math.PI * 2 - 1.8);
    const x = DEN.x + Math.cos(a) * 8.2;
    const z = DEN.z + Math.sin(a) * 8.2;
    place('tent', x, z, facing(x, z, DEN.x, DEN.z), 9);
    collide(x, z, 1.9);
  }
  // Fogueira (tochas), mesa de armas, barris e caixas: a luz do acampamento.
  for (const [dx, dz] of [[-2, 0], [2, 0], [0, -2], [0, 2]]) torch(DEN.x + dx, DEN.z + dz);
  for (const [k, dx, dz, s] of [['weaponrack', 5, 3, 4.5], ['barrel', -5, 4, 4.5], ['barrel', -5.9, 3.4, 4.5], ['crate_a_big', 4.6, -4.6, 4.5], ['sack', 5.8, -4, 4.5]]) {
    place(k, DEN.x + dx, DEN.z + dz, rand() * 6.28, s);
  }
  // Bandeiras nas pontas da abertura.
  for (const side of [-1, 1]) {
    const a = gate + side * 0.36;
    place('flag_blue', DEN.x + Math.cos(a) * (R + 1.2), DEN.z + Math.sin(a) * (R + 1.2), alongZ(-Math.sin(a), Math.cos(a)), 10);
  }
}

// ---------- Pintura do chão (cliente: client/src/world/map-painter.ts) ----------
/** Musgo cinza-prateado (#7d8a6a) só dentro do setor do Bosque, com sombra escura (#2b3530) e manchas de terra. */
export function paint(g, bio, h) {
  const { p } = h;
  const rand = h.rng(61);
  const blotch = (n, colors, rMin, rMax, alpha, where) => {
    let done = 0;
    for (let i = 0; i < n * 40 && done < n; i++) {
      const wx = bounds.minX + rand() * (bounds.maxX - bounds.minX);
      const wz = bounds.minZ + rand() * (bounds.maxZ - bounds.minZ);
      if (!where(wx, wz)) continue;
      done++;
      g.fillStyle = colors[Math.floor(rand() * colors.length)];
      g.globalAlpha = alpha * (0.5 + rand() * 0.5);
      g.beginPath();
      g.ellipse(p.cx(wx), p.cy(wz), (rMin + rand() * (rMax - rMin)) * p.k, (rMin + rand() * (rMax - rMin)) * 0.75 * p.k, rand() * Math.PI, 0, Math.PI * 2);
      g.fill();
    }
    g.globalAlpha = 1;
  };
  const inside = (x, z) => inSector(x, z, 6) || near(x, z, DEN, 22);
  blotch(220, ['#7d8a6a', '#6f7c5f', '#8a9678'], 5, 12, 0.34, inside); // a base de musgo, em manchas largas
  blotch(150, ['#2b3530', '#34403a'], 1.5, 5, 0.4, inside); // a sombra sob as copas
  blotch(40, ['#4e4234', '#5a4c3b'], 0.8, 2.6, 0.45, inside); // terra e raízes
  blotch(14, ['#a2302a'], 0.3, 0.9, 0.3, (x, z) => near(x, z, DEN, 14)); // o pano dos bandidos no chão da Toca
}
