// Brejo de Lamagris (leste, níveis 14–20; roadmap 5.3, tunnel#139): o pântano que a Partição represou. Entra pela
// estrada leste, no Posto do Barqueiro (165, 0), e se abre numa lagoa rasa e andável, a lagoa de Vau Velho, com
// duas partes fundas (de colisor): a aldeia afogada (só a torre e os telhados saem da água) e o fosso em volta da
// Ilha dos Vaga-lumes. O resto é lodo, árvores tortas, juncos e troncos caídos.
//
// Também fecha o que o mapa novo deixou aberto: o sudeste (x 150–330, z < −150) e o nordeste (x > 270, z > 60) são
// montanhas sem acesso, com uma muralha de pedras de colisor (o cliente não tem como saber que ali não se anda).
//
// O servidor lê as mesmas coordenadas em game/brejo.go (limites, posto, cais, ilha e a Mãe-Lodo), e o cliente lê a
// lagoa e as partes fundas da entrada deste bioma no world.json (terrain.ts monta o relevo com elas).
export const name = 'Brejo de Lamagris';
export const seed = 20261140;
export const order = 40;
export const bounds = { minX: 150, maxX: 330, minZ: -150, maxZ: 60 };

// ---------- Lugares (iguais ao servidor, game/brejo.go) ----------
const POST = { x: 165, z: 0 }; // Posto do Barqueiro: o fim da estrada leste
const DOCK = { x: 188, z: -10 }; // o cais: o barco de Tomé até a ilha
const MOTHER = { x: 250, z: -40 }; // a Mãe-Lodo, no meio da lagoa
const ISLAND = { x: 268, z: -20, r: 4.5 }; // Ilha dos Vaga-lumes (dentro do fosso)
const LAGOON = { x: 245, z: -38, rx: 52, rz: 36 };
const VAU = { x: 232, z: -52, r: 10 }; // a aldeia afogada: parte funda
const MOAT = { x: ISLAND.x, z: ISLAND.z, r: 10 }; // o fosso da ilha: parte funda
const DEEPS = [VAU, MOAT];
const BRIDGE = { x: 200, z: 58 }; // a Ponte de Troncos para as Terras Mortas
const MOTHER_TREE = { x: 205, z: 36 }; // a Árvore-Mãe

// ---------- Malhas (kits que já existem; as peças novas são do Meshy, ver world-gen.mjs) ----------
const BARE = ['tree_bare_1_a', 'tree_bare_1_b', 'tree_bare_2_a', 'tree_bare_2_b', 'tree_bare_1_c', 'tree_bare_2_c'];
const LUSH = ['tree_1_a', 'tree_1_b', 'tree_2_a'];
const BUSH = ['bush_1_a', 'bush_2_a', 'bush_3_a', 'bush_4_a'];
const GRASS = ['grass_1_a', 'grass_2_a'];
const ROCK_MID = ['rock_1_a', 'rock_1_e', 'rock_3_b', 'rock_3_c'];
const WALL_ROCKS = ['rock_3_a', 'rock_3_e', 'rock_3_g', 'rock_3_j', 'rock_1_h'];
const MOUNTAINS = ['mountain_a', 'mountain_b', 'mountain_c', 'mountain_a_grass', 'hills_a', 'hills_c'];

/** Distância normalizada ao centro da lagoa (1 = a borda da elipse; sem a ondulação do relevo). */
const lagoonU = (x, z) => Math.hypot((x - LAGOON.x) / LAGOON.rx, (z - LAGOON.z) / LAGOON.rz);
const nearDeep = (x, z, m) => DEEPS.some((d) => Math.hypot(x - d.x, z - d.z) < d.r + m);
const onRoad = (x, z) => Math.abs(z) < 5 && x < POST.x + 8;
const nearPost = (x, z, m) => Math.hypot(x - POST.x, z - POST.z) < m;
const nearDock = (x, z, m) => Math.hypot(x - DOCK.x, z - DOCK.z) < m;

export function build(ctx) {
  const { place, collide, collideItem, torch, facing, alongX, sized, freeOfColliders, rand, pick, between, scatter } = ctx;

  // ---------- A borda leste da floresta (montanhas em x ≈ 168) caiu para dentro do mapa novo ----------
  for (let z = -148; z <= 148; z += 26) ctx.clear(168, z, 9);

  // ---------- Estrada leste, até o Posto ----------
  ctx.roads.push({ x1: 150, z1: 0, x2: POST.x + 5, z2: 0, w: 6 });

  // ---------- Posto do Barqueiro ----------
  // A cabana de palafitas do Tomé, de frente para a estrada, e um acampamento pequeno em volta.
  place('ferryman_hut', POST.x + 7, POST.z - 9, facing(POST.x + 7, POST.z - 9, POST.x, POST.z), 1);
  collide(POST.x + 7, POST.z - 9, 3.2);
  for (const [k, x, z, s] of [
    ['tent', POST.x - 6, POST.z - 9, 6], ['tent', POST.x + 10, POST.z + 8, 6], ['barrel', POST.x + 4, POST.z - 5, 4.5],
    ['barrel', POST.x + 5.2, POST.z - 4.4, 4.5], ['crate_a_big', POST.x + 2.5, POST.z - 6.5, 4.5], ['sack', POST.x + 3.8, POST.z - 6.8, 4.5],
    ['weaponrack', POST.x - 4, POST.z + 8, 4.5], ['crate_long_a', POST.x + 9, POST.z + 5, 4.5], ['sack', POST.x + 8.2, POST.z + 4.4, 4.5],
  ]) place(k, x, z, rand() * 6.28, s);
  for (const side of [-1, 1]) place('flag_blue', 152, side * 5, alongX(0, 1), 10); // as bandeiras da entrada do Brejo
  for (const [x, z] of [[POST.x - 5, 4], [POST.x - 5, -4], [POST.x + 3, 4], [POST.x + 3, -4], [POST.x + 12, -3], [POST.x + 12, 3]]) torch(x, z);

  // ---------- O cais e o barco (o NPC do barco fica em DOCK; o desenho do bote, ao lado) ----------
  place('boat', DOCK.x + 3, DOCK.z, alongX(1, 0), 1);
  collide(DOCK.x + 3, DOCK.z, 1.4);
  for (const side of [-1, 1]) torch(DOCK.x - 1.5, DOCK.z + side * 3);

  // ---------- Vau Velho: a aldeia afogada. Só os telhados e a torre da capela saem da água ----------
  place('church', VAU.x, VAU.z, rand() * 6.28, sized('church', 9));
  place('tower_a', VAU.x + 5, VAU.z + 3, rand() * 6.28, 5.5);
  place('home_b', VAU.x - 5.5, VAU.z + 2, rand() * 6.28, 5.5);
  place('home_a', VAU.x + 1, VAU.z - 6, rand() * 6.28, 5);
  // ---------- As partes fundas têm colisor: um anel de círculos que se encostam ----------
  for (const d of DEEPS) {
    const R = d.r + 1.5;
    const n = Math.ceil((2 * Math.PI * R) / 2.4);
    for (let i = 0; i < n; i++) {
      const a = (i / n) * Math.PI * 2;
      collide(d.x + Math.cos(a) * R, d.z + Math.sin(a) * R, 1.5);
    }
  }
  // ---------- A Ilha dos Vaga-lumes: uma árvore, mato e o bote de volta ----------
  place('tree_2_a', ISLAND.x + 2.6, ISLAND.z + 2, rand() * 6.28, 2.4);
  collide(ISLAND.x + 2.6, ISLAND.z + 2, 0.6);
  place('boat', ISLAND.x - 3, ISLAND.z - 1.5, alongX(0, 1), 1);
  for (let i = 0; i < 9; i++) {
    const a = rand() * 6.28;
    const r = 1.5 + rand() * 2.6;
    place(i % 3 === 0 ? pick(BUSH) : 'reeds', ISLAND.x + Math.cos(a) * r, ISLAND.z + Math.sin(a) * r, rand() * 6.28, i % 3 === 0 ? between(1.4, 2.2) : between(0.9, 1.4));
  }

  // ---------- Árvore-Mãe e Ponte de Troncos ----------
  place('tree_bare_2_b', MOTHER_TREE.x, MOTHER_TREE.z, rand() * 6.28, 7.5);
  collide(MOTHER_TREE.x, MOTHER_TREE.z, 2.2);
  for (let i = 0; i < 5; i++) place('fallen_log', BRIDGE.x - 6 + i * 3, BRIDGE.z + (i % 2) * 0.6, alongX(1, 0) + between(-0.1, 0.1), between(0.9, 1.1));

  const accept = (x, z) =>
    lagoonU(x, z) > 1.28 && !onRoad(x, z) && !nearPost(x, z, 16) && !nearDock(x, z, 7) && !nearDeep(x, z, 4) &&
    Math.hypot(x - MOTHER_TREE.x, z - MOTHER_TREE.z) > 6 && Math.hypot(x - BRIDGE.x, z - BRIDGE.z) > 9;
  const ground = { x: [bounds.minX + 2, bounds.maxX - 2], z: [bounds.minZ + 4, bounds.maxZ - 2] };

  // ---------- Árvores tortas, mortas, com poucas vivas ----------
  scatter(115, { ...ground, accept, spacing: 7.5 }, (x, z) => {
    const s = between(2.4, 3.6);
    place(pick(BARE), x, z, rand() * 6.28, s);
    collide(x, z, 0.22 * s);
  });
  scatter(30, { ...ground, accept: (x, z) => accept(x, z) && x < 215, spacing: 8 }, (x, z) => {
    const s = between(2.1, 2.7);
    place(pick(LUSH), x, z, rand() * 6.28, s);
    collide(x, z, 0.3 * s);
  });
  // Pedras (algumas bloqueiam) e troncos caídos (decoração e esconderijo).
  scatter(26, { ...ground, accept, spacing: 4 }, (x, z) => {
    const k = pick(ROCK_MID);
    const m = between(1, 2.2);
    place(k, x, z, rand() * 6.28, sized(k, m));
    if (m > 1.5) collide(x, z, m * 0.38);
  });
  scatter(16, { ...ground, accept, spacing: 6 }, (x, z) => {
    const s = between(0.9, 1.5);
    place('fallen_log', x, z, rand() * 6.28, s);
    collide(x, z, 1.1 * s);
  });
  // ---------- Juncos (nas margens e pelo resto), arbustos e capim ----------
  scatter(70, {
    ...ground, spacing: 2.2,
    accept: (x, z) => { const u = lagoonU(x, z); return u > 0.9 && u < 1.45 && !nearDeep(x, z, 2) && !nearDock(x, z, 3); },
  }, (x, z) => place('reeds', x, z, rand() * 6.28, between(0.9, 1.5)));
  scatter(70, { ...ground, accept, spacing: 2.5 }, (x, z) => place('reeds', x, z, rand() * 6.28, between(0.8, 1.3)));
  scatter(80, { ...ground, accept }, (x, z) => place(pick(BUSH), x, z, rand() * 6.28, between(2, 3)));
  scatter(600, { ...ground, accept: (x, z) => accept(x, z) || (lagoonU(x, z) > 1.1 && !nearDeep(x, z, 1)) }, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.6, 1.2)));

  // ---------- Borda leste (a foz do Lamagris, no meio, vira o portão para Maresia depois) e o resto ----------
  for (let z = bounds.minZ - 8; z <= bounds.maxZ + 30; z += 26) {
    place(pick(MOUNTAINS), bounds.maxX + 18 + between(-4, 4), z + between(-4, 4), rand() * 6.28, between(20, 30));
  }
  // ---------- Montanhas sem acesso: o sudeste e o nordeste, fechados por uma muralha de pedras ----------
  const wall = (x1, z1, x2, z2) => {
    const len = Math.hypot(x2 - x1, z2 - z1);
    for (let d = 0; d <= len; d += 3.6) {
      const t = d / len;
      const k = pick(WALL_ROCKS);
      const m = between(4.2, 6.4);
      const x = x1 + (x2 - x1) * t + between(-0.5, 0.5);
      const z = z1 + (z2 - z1) * t + between(-0.5, 0.5);
      place(k, x, z, rand() * 6.28, sized(k, m));
      collide(x, z, m * 0.45);
    }
  };
  wall(bounds.minX + 2, bounds.minZ - 3, bounds.maxX, bounds.minZ - 3); // sul do Brejo
  wall(bounds.minX + 2, bounds.minZ - 3, bounds.minX + 2, -334); // leste do Vale (até o fim do mapa)
  wall(270, 62, 270, 154); // leste da extensão das Terras Mortas
  wall(270, 62, bounds.maxX, 62); // norte do Brejo, a leste da extensão
  for (let x = bounds.minX + 14; x <= bounds.maxX + 24; x += 26) {
    for (const z of [-170, -210, -250, -290, -325]) {
      place(pick(MOUNTAINS), x + between(-5, 5), z + between(-5, 5), rand() * 6.28, between(22, 34));
    }
  }
  for (let z = 80; z <= 150; z += 26) place(pick(MOUNTAINS), 292 + between(-6, 6), z + between(-4, 4), rand() * 6.28, between(22, 30));
  for (let x = 280; x <= bounds.maxX + 24; x += 26) place(pick(MOUNTAINS), x + between(-4, 4), 90 + between(-8, 8), rand() * 6.28, between(22, 32));

  return { lagoon: { ...LAGOON, level: -0.35 }, deeps: DEEPS, island: ISLAND, post: POST, dock: DOCK, mother: MOTHER };
}

// ---------- Pintura do chão (cliente: client/src/world/map-painter.ts) ----------
/**
 * Lodo e musgo: a grama da floresta vai ficando verde-oliva escura de oeste para leste (a borda mistura em 18 m),
 * com poças escuras, manchas de musgo e, em volta da lagoa, lama molhada. `bio` é a entrada do bioma no world.json.
 */
export function paint(g, bio, h) {
  const { p } = h;
  const b = bio.bounds;
  const rand = h.rng(41);
  const x0 = p.cx(b.minX - 18);
  const x1 = p.cx(b.maxX);
  const y0 = p.cy(b.maxZ);
  const y1 = p.cy(b.minZ);
  // Mistura de oeste: faixa de 18 m do verde da floresta para o lodo.
  const grad = g.createLinearGradient(x0, 0, p.cx(b.minX + 4), 0);
  grad.addColorStop(0, 'rgba(91,90,58,0)');
  grad.addColorStop(1, 'rgba(91,90,58,0.92)');
  g.fillStyle = grad;
  g.fillRect(x0, y0, p.cx(b.minX + 4) - x0, y1 - y0);
  g.fillStyle = 'rgba(91,90,58,0.92)'; // base lodo #5b5a3a
  g.fillRect(p.cx(b.minX + 4), y0, x1 - p.cx(b.minX + 4), y1 - y0);
  const area = ((b.maxX - b.minX) * (b.maxZ - b.minZ)) / 1000;
  const blotch = (n, colors, rMin, rMax, alpha) => {
    for (let i = 0; i < n * area; i++) {
      const wx = b.minX + rand() * (b.maxX - b.minX);
      const wz = b.minZ + rand() * (b.maxZ - b.minZ);
      g.fillStyle = colors[Math.floor(rand() * colors.length)];
      g.globalAlpha = alpha * (0.5 + rand() * 0.5);
      g.beginPath();
      g.ellipse(p.cx(wx), p.cy(wz), (rMin + rand() * (rMax - rMin)) * p.k, (rMin + rand() * (rMax - rMin)) * 0.7 * p.k, rand() * Math.PI, 0, Math.PI * 2);
      g.fill();
    }
    g.globalAlpha = 1;
  };
  blotch(70, ['#6b7d3e', '#5d7035', '#7a8a46'], 1.5, 5, 0.5); // musgo #6b7d3e
  blotch(55, ['#3f3d28', '#4a4730', '#34331f'], 0.8, 3.5, 0.55); // lama escura e poças
  blotch(8, ['#9fe870'], 0.4, 1.2, 0.22); // brilho de fungo
  // Lama molhada em volta da lagoa.
  const l = bio.lagoon;
  const mud = g.createRadialGradient(p.cx(l.x), p.cy(l.z), l.rx * 0.7 * p.k, p.cx(l.x), p.cy(l.z), l.rx * 1.5 * p.k);
  mud.addColorStop(0, 'rgba(46,44,28,0.85)');
  mud.addColorStop(1, 'rgba(46,44,28,0)');
  g.fillStyle = mud;
  g.save();
  g.translate(p.cx(l.x), p.cy(l.z));
  g.scale(1, l.rz / l.rx);
  g.translate(-p.cx(l.x), -p.cy(l.z));
  g.beginPath();
  g.arc(p.cx(l.x), p.cy(l.z), l.rx * 1.5 * p.k, 0, Math.PI * 2);
  g.fill();
  g.restore();
}
