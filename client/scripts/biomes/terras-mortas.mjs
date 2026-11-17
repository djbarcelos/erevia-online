// Terras Mortas, extensão (nordeste, x 150–270 e z 60–150; roadmap 5.2): o bloco de terra cinza que nasce junto com o
// Brejo, ao lado do núcleo (a Cripta, em volta de (108, 108)). Árvores mortas, cemitérios sem nome com lápides tortas,
// velas negras (as tochas) e a Torre do Sino Rachado, o landmark da região, visível da cidade.
//
// Os monstros (nível 24 a 28, game/bosque.go) moram aqui e no núcleo (terras-mortas-nucleo.mjs, em volta da Cripta): as
// duas partes são a mesma região, com a mesma cor, névoa e música. O Bastião Cinzento (o posto avançado, com a pedra de
// teleporte) e a Lápide do Último Regicida ficam nesta parte. A muralha do leste (x = 270) e a do norte do Brejo (z = 62)
// estão em brejo.mjs, junto com as outras montanhas sem acesso.
export const name = 'Terras Mortas (extensão)';
export const seed = 20261150;
export const order = 50;
export const bounds = { minX: 150, maxX: 270, minZ: 60, maxZ: 150 };

const BELL = { x: 238, z: 122 }; // a Torre do Sino Rachado
const GRAVEYARD = { x: 172, z: 122, r: 20 }; // o Cemitério dos Reis Sem Nome
const BASTION = { x: 196, z: 94 }; // o Bastião Cinzento: o posto avançado da Ordem (igual ao servidor, game/bosque.go)
const TOMBSTONE = { x: 172, z: 132 }; // a Lápide do Último Regicida (o NPC fica ao lado do monólito do cemitério)
const BARE = ['tree_bare_1_a', 'tree_bare_1_b', 'tree_bare_2_a', 'tree_bare_2_b', 'tree_bare_1_c', 'tree_bare_2_c'];
const ROCK = ['rock_1_a', 'rock_1_e', 'rock_3_b', 'rock_2_a'];

export function build(ctx) {
  const { place, collide, collideItem, torch, facing, alongX, sized, rand, pick, between, scatter } = ctx;

  // A Torre do Sino Rachado: a torre da cidade, grande, no meio de um círculo de lápides caídas.
  place('tower_a', BELL.x, BELL.z, rand() * 6.28, 10);
  collideItem('tower_a', BELL.x, BELL.z, 10, 0.45);
  for (let i = 0; i < 7; i++) {
    const a = (i / 7) * Math.PI * 2 + between(-0.2, 0.2);
    const x = BELL.x + Math.cos(a) * 11;
    const z = BELL.z + Math.sin(a) * 11;
    place('monolith_broken', x, z, rand() * 6.28, between(1.1, 1.5));
    collide(x, z, 0.9);
  }

  // O cemitério: lápides em fileiras tortas, uma cruz de pedra (monólito caído) e velas negras.
  for (let row = -2; row <= 2; row++) {
    for (let i = -4; i <= 4; i++) {
      const x = GRAVEYARD.x + i * 3.6 + between(-0.7, 0.7);
      const z = GRAVEYARD.z + row * 4.4 + between(-0.8, 0.8);
      if (rand() < 0.18) continue;
      place('grave', x, z, rand() * 6.28 + Math.PI / 2, between(0.9, 1.3));
    }
  }
  place('monolith', GRAVEYARD.x, GRAVEYARD.z + 12, 0, [1.6, 2, 1.6]);
  collide(GRAVEYARD.x, GRAVEYARD.z + 12, 1.2);
  for (const [dx, dz] of [[-16, 0], [16, 0], [-8, 9], [8, 9], [-8, -9], [8, -9]]) torch(GRAVEYARD.x + dx, GRAVEYARD.z + dz);

  // O Bastião Cinzento: uma torre de pedra cinza com o estandarte da Ordem, um muro baixo de caixas e barris, tendas
  // e fogueiras. A pedra de teleporte e o Irmão Caedmon ficam no pátio (o servidor cria os dois).
  place('tower_a', BASTION.x, BASTION.z + 9, facing(BASTION.x, BASTION.z + 9, BASTION.x, BASTION.z), 6.5);
  collideItem('tower_a', BASTION.x, BASTION.z + 9, 6.5, 0.45);
  for (const side of [-1, 1]) {
    place('flag_blue', BASTION.x + side * 4.2, BASTION.z + 5, alongX(1, 0), 10);
    place('tent', BASTION.x + side * 9, BASTION.z + 3, facing(BASTION.x + side * 9, BASTION.z + 3, BASTION.x, BASTION.z), 7);
    collide(BASTION.x + side * 9, BASTION.z + 3, 1.6);
    place('tent', BASTION.x + side * 8, BASTION.z - 7, facing(BASTION.x + side * 8, BASTION.z - 7, BASTION.x, BASTION.z), 7);
    collide(BASTION.x + side * 8, BASTION.z - 7, 1.6);
  }
  for (let i = -5; i <= 5; i++) {
    if (Math.abs(i) < 2) continue; // a entrada, para o sul
    const x = BASTION.x + i * 2.1;
    place('fence_wood_straight', x, BASTION.z - 11, alongX(1, 0) + Math.PI / 2, 3.6);
    collide(x, BASTION.z - 11, 1.1);
  }
  for (const [k, dx, dz, s] of [['barrel', -5, -4, 4.5], ['barrel', -5.9, -3.3, 4.5], ['crate_a_big', 5, -3.6, 4.5], ['sack', 6.2, -3, 4.5], ['weaponrack', 2.8, 4.6, 4.5]]) {
    place(k, BASTION.x + dx, BASTION.z + dz, rand() * 6.28, s);
  }
  for (const [dx, dz] of [[-3.2, 1], [3.2, 1], [-3, -6], [3, -6], [0, 3]]) torch(BASTION.x + dx, BASTION.z + dz);

  // A Lápide do Último Regicida: uma pedra alta, ao lado da cruz de pedra do cemitério, com velas.
  place('monolith_broken', TOMBSTONE.x + 2.4, TOMBSTONE.z + 1.2, 0.4, [1.5, 1.6, 1.5]);
  collide(TOMBSTONE.x + 2.4, TOMBSTONE.z + 1.2, 1.1);
  torch(TOMBSTONE.x - 2, TOMBSTONE.z - 1.5);
  torch(TOMBSTONE.x + 4.6, TOMBSTONE.z - 1.5);

  const free = (x, z) =>
    Math.hypot(x - BELL.x, z - BELL.z) > 14 && Math.hypot(x - GRAVEYARD.x, z - GRAVEYARD.z) > GRAVEYARD.r + 3 &&
    Math.hypot(x - BASTION.x, z - BASTION.z) > 17;
  const ground = { x: [bounds.minX + 2, bounds.maxX - 2], z: [bounds.minZ + 3, bounds.maxZ - 2] };
  // Árvores mortas, secas e tortas.
  scatter(62, { ...ground, accept: free, spacing: 8 }, (x, z) => {
    const s = between(2.4, 3.6);
    place(pick(BARE), x, z, rand() * 6.28, s);
    collide(x, z, 0.22 * s);
  });
  // Pedras e lápides soltas pelo chão (as soltas não bloqueiam).
  scatter(24, { ...ground, accept: free, spacing: 4 }, (x, z) => {
    const k = pick(ROCK);
    const m = between(0.8, 2);
    place(k, x, z, rand() * 6.28, sized(k, m));
    if (m > 1.4) collide(x, z, m * 0.38);
  });
  scatter(22, { ...ground, accept: free, spacing: 2.5 }, (x, z) => place('grave', x, z, rand() * 6.28, between(0.8, 1.2)));
}

// ---------- Pintura do chão (cliente: client/src/world/map-painter.ts) ----------
/** Terra cinza-osso (#8f8a80) com a borda oeste misturada na grama, manchas escuras e uns pontos roxos da Cripta. */
export function paint(g, bio, h) {
  const { p } = h;
  const b = bio.bounds;
  const rand = h.rng(51);
  const x0 = p.cx(b.minX - 14);
  const y0 = p.cy(b.maxZ);
  const y1 = p.cy(b.minZ);
  const grad = g.createLinearGradient(x0, 0, p.cx(b.minX + 4), 0);
  grad.addColorStop(0, 'rgba(120,116,106,0)');
  grad.addColorStop(1, 'rgba(120,116,106,0.9)');
  g.fillStyle = grad;
  g.fillRect(x0, y0, p.cx(b.minX + 4) - x0, y1 - y0);
  g.fillStyle = 'rgba(120,116,106,0.9)';
  g.fillRect(p.cx(b.minX + 4), y0, p.cx(b.maxX) - p.cx(b.minX + 4), y1 - y0);
  const area = ((b.maxX - b.minX) * (b.maxZ - b.minZ)) / 1000;
  for (const [n, colors, rMin, rMax, a] of [
    [60, ['#8f8a80', '#a29d92', '#7d796f'], 1.2, 4.5, 0.5],
    [40, ['#3b3a40', '#2b2a33', '#46454c'], 0.8, 3, 0.5],
    [5, ['#8a5cff'], 0.3, 0.9, 0.2],
  ]) {
    for (let i = 0; i < n * area; i++) {
      const wx = b.minX + rand() * (b.maxX - b.minX);
      const wz = b.minZ + rand() * (b.maxZ - b.minZ);
      g.fillStyle = colors[Math.floor(rand() * colors.length)];
      g.globalAlpha = a * (0.5 + rand() * 0.5);
      g.beginPath();
      g.ellipse(p.cx(wx), p.cy(wz), (rMin + rand() * (rMax - rMin)) * p.k, (rMin + rand() * (rMax - rMin)) * 0.7 * p.k, rand() * Math.PI, 0, Math.PI * 2);
      g.fill();
    }
  }
  g.globalAlpha = 1;
}
