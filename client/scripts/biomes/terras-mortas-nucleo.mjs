// Terras Mortas, núcleo (nordeste do quadrado original, r > 142 de 15° a 75°; roadmap 5.2, tunnel#137): a terra cinza em
// volta da Cripta Esquecida, de onde os Devotos do Ossário cavam as covas e levantam os mortos. É a metade oeste da região:
// a metade leste (a extensão, com o Cemitério, a Torre do Sino Rachado e o Bastião Cinzento) é terras-mortas.mjs, e as duas
// compartilham a cor, a névoa e a música (a região "Terras Mortas").
//
// Roda na fase `overlay`, depois da Cripta (o covil fica como está): árvores mortas, covas rasas com lápides tortas e velas
// negras (as tochas) a partir de 38 m do centro do covil. Os monstros (nível 24 a 28) estão em game/bosque.go.
export const name = 'Terras Mortas (núcleo)';
export const seed = 20261161;
export const order = 55;
export const bounds = { minX: 96, maxX: 150, minZ: 40, maxZ: 150 };

const LAIR = { x: 108, z: 108 }; // igual ao servidor (game.RaidLairX/Z) e à cripta.mjs
const BARE = ['tree_bare_1_a', 'tree_bare_1_b', 'tree_bare_2_a', 'tree_bare_2_b', 'tree_bare_1_c', 'tree_bare_2_c'];
const ROCK = ['rock_1_a', 'rock_1_e', 'rock_3_b', 'rock_2_a'];

export function overlay(ctx) {
  const { place, collide, torch, rand, pick, between, scatter } = ctx;
  // Só o que está no setor das Terras Mortas (r > 142, de 15° a 75°) e longe do covil (a cripta cuida do dela).
  const inCore = (x, z) => {
    const r = Math.hypot(x, z);
    const a = (Math.atan2(z, x) * 180) / Math.PI;
    return r > 134 && a > 12 && a < 78 && Math.hypot(x - LAIR.x, z - LAIR.z) > 40;
  };
  const ground = { x: [bounds.minX + 2, bounds.maxX - 2], z: [bounds.minZ + 2, bounds.maxZ - 2] };

  // Árvores mortas, secas e tortas (mais abertas que na floresta: o chão é cinza e nu).
  scatter(26, { ...ground, accept: inCore, spacing: 8 }, (x, z) => {
    const s = between(2.4, 3.6);
    place(pick(BARE), x, z, rand() * 6.28, s);
    collide(x, z, 0.22 * s);
  });
  // Covas: pequenos grupos de lápides tortas com uma vela negra no meio (as covas dos Devotos).
  for (const [cx, cz] of [[138, 118], [128, 140], [144, 62], [140, 92]]) {
    for (let i = 0; i < 6; i++) {
      const a = (i / 6) * Math.PI * 2 + between(-0.3, 0.3);
      const r = between(2, 4.6);
      place('grave', cx + Math.cos(a) * r, cz + Math.sin(a) * r, rand() * 6.28 + Math.PI / 2, between(0.9, 1.3));
    }
    torch(cx, cz);
  }
  // Pedras soltas e lápides caídas pelo chão.
  scatter(14, { ...ground, accept: inCore, spacing: 4 }, (x, z) => {
    const k = pick(ROCK);
    place(k, x, z, rand() * 6.28, between(0.8, 1.8));
  });
  scatter(14, { ...ground, accept: inCore, spacing: 2.5 }, (x, z) => place('grave', x, z, rand() * 6.28, between(0.8, 1.2)));
}

// ---------- Pintura do chão (cliente: client/src/world/map-painter.ts) ----------
/** Terra cinza-osso (#8f8a80), a mesma da extensão, só dentro do setor das Terras Mortas, com manchas escuras e uns pontos roxos. */
export function paint(g, bio, h) {
  const { p } = h;
  const b = bio.bounds;
  const rand = h.rng(52);
  const inCore = (x, z) => {
    const r = Math.hypot(x, z);
    const a = (Math.atan2(z, x) * 180) / Math.PI;
    return r > 134 && a > 12 && a < 78 && Math.hypot(x - LAIR.x, z - LAIR.z) > 26;
  };
  const blotch = (n, colors, rMin, rMax, alpha) => {
    let done = 0;
    for (let i = 0; i < n * 40 && done < n; i++) {
      const wx = b.minX + rand() * (b.maxX - b.minX);
      const wz = b.minZ + rand() * (b.maxZ - b.minZ);
      if (!inCore(wx, wz)) continue;
      done++;
      g.fillStyle = colors[Math.floor(rand() * colors.length)];
      g.globalAlpha = alpha * (0.5 + rand() * 0.5);
      g.beginPath();
      g.ellipse(p.cx(wx), p.cy(wz), (rMin + rand() * (rMax - rMin)) * p.k, (rMin + rand() * (rMax - rMin)) * 0.7 * p.k, rand() * Math.PI, 0, Math.PI * 2);
      g.fill();
    }
    g.globalAlpha = 1;
  };
  blotch(160, ['#8f8a80', '#a29d92', '#7d796f'], 3, 8, 0.45);
  blotch(90, ['#3b3a40', '#2b2a33', '#46454c'], 0.8, 3, 0.5);
  blotch(12, ['#8a5cff'], 0.3, 0.9, 0.2);
}
