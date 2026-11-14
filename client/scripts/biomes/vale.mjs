// Vale das Pedras Vivas (sul, níveis 20–30): a floresta seca e rareia numa borda ondulada,
// e o chão vira cascalho e rocha, com paredões, círculos de monólitos e a área dos golens.
export const name = 'Vale das Pedras Vivas';
export const seed = 20261114;
export const order = 20;
export const bounds = { minX: -150, maxX: 150, minZ: -330, maxZ: -150 };

const VALE = { x: 0, z: -270, r: 52 }; // área dos golens (igual ao servidor)
const VALE_STONE = { x: 6, z: -198 }; // Pedra de Teleporte (igual ao servidor)

// ---------- Regra de transição (o gerador e a pintura do chão usam esta mesma conta) ----------
const clamp01 = (v) => Math.max(0, Math.min(1, v));
const smoothstep = (a, b, v) => {
  const t = clamp01((v - a) / (b - a));
  return t * t * (3 - 2 * t);
};
/**
 * Quanto o ponto já é vale de pedra: 0 = floresta, 1 = pedra. `b` traz a borda ondulada
 * ({ zFrom, zTo, amp, ph }), como sai no world.json.
 */
export function transition(b, x, z) {
  const n =
    (Math.sin(x * 0.045 + b.ph[0]) * Math.sin(z * 0.05 + b.ph[1]) +
      0.5 * Math.sin(x * 0.11 + z * 0.07 + b.ph[2]) +
      0.3 * Math.sin(x * 0.23 - z * 0.19 + b.ph[3])) /
    1.8;
  return smoothstep(b.zFrom, b.zTo, z + n * b.amp);
}

// ---------- Malhas ----------
const LUSH = ['tree_1_a', 'tree_1_b', 'tree_2_a', 'tree_2_b', 'tree_3_a', 'tree_4_a', 'tree_4_b'];
const BARE_ALL = ['tree_bare_1_a', 'tree_bare_1_b', 'tree_bare_2_a', 'tree_bare_2_b', 'tree_bare_1_c', 'tree_bare_2_c'];
const BUSH = ['bush_1_a', 'bush_2_a', 'bush_3_a', 'bush_4_a'];
const GRASS = ['grass_1_a', 'grass_2_a'];
const CLIFF = ['rock_1_k', 'rock_2_f', 'rock_3_g', 'rock_3_j', 'rock_1_h'];
const BOULDER = ['rock_3_a', 'rock_3_e', 'rock_3_b', 'rock_3_c', 'rock_3_g', 'rock_3_j', 'rock_1_h'];
const STONE_MID = ['rock_1_a', 'rock_1_c', 'rock_1_e', 'rock_3_b', 'rock_3_c'];
const GRAVEL = ['rock_2_a', 'rock_2_c', 'rock_1_c'];
const ROCK_MOUNTAINS = ['mountain_a', 'mountain_b', 'mountain_c', 'mountain_a_grass', 'hills_a', 'hills_c'];

// ---------- Cristas: paredões de pedras grandes nas laterais e no fundo, com falhas (passagens) ----------
const RIDGES = [
  [[-62, -214], [-74, -245], [-88, -280], [-96, -318]],
  [[60, -218], [76, -250], [92, -286], [102, -320]],
  [[-58, -323], [-20, -326], [20, -325], [56, -322]],
  [[-112, -232], [-128, -248], [-141, -266]],
  [[116, -226], [131, -244], [143, -262]],
  [[-138, -296], [-120, -312]],
  [[134, -300], [118, -316]],
];

// ---------- Landmarks: círculos de monólitos rúnicos, alguns inteiros, outros partidos ou tombados ----------
const CIRCLES = [
  { x: -36, z: -244, r: 6.5, n: 7 },
  { x: 40, z: -258, r: 7, n: 7 },
  { x: -10, z: -302, r: 7.5, n: 8 },
  { x: -104, z: -284, r: 5.5, n: 5 },
  { x: 108, z: -272, r: 5.5, n: 5 },
];

const clearing = (x, z) => (Math.abs(x) < 6.5 && z > -214) || Math.hypot(x - VALE_STONE.x, z - VALE_STONE.z) < 10;
const inVale = (x, z) => Math.hypot(x - VALE.x, z - VALE.z) < VALE.r;

export function build(ctx) {
  const { place, collide, taken, freeOfColliders, sized, facing, round, rand, pick, between } = ctx;
  // Borda ondulada entre a floresta e o vale (o cliente pinta o chão com a mesma conta).
  const BIOME = { zFrom: -150, zTo: -205, amp: 14, ph: Array.from({ length: 4 }, () => round(rand() * 6.28)) };
  const rocky = (x, z) => transition(BIOME, x, z);
  const MIN_Z = bounds.minZ + 3;

  const scatter = (count, zMin, zMax, accept, fn, { spacing = 0, tries = 30, avoidClearing = true } = {}) =>
    ctx.scatter(count, {
      x: [bounds.minX + 2, bounds.maxX - 2],
      z: [zMin, zMax],
      accept: (x, z) => !(avoidClearing && clearing(x, z)) && accept(x, z),
      spacing,
      tries,
    }, fn);

  for (const ridge of RIDGES) {
    for (let i = 0; i < ridge.length - 1; i++) {
      const [ax, az] = ridge[i];
      const [bx, bz] = ridge[i + 1];
      const len = Math.hypot(bx - ax, bz - az);
      for (let d = 0; d < len; d += between(2.8, 4.2)) {
        const t = d / len;
        const k = pick(CLIFF);
        const m = between(4, 7.5);
        const s = sized(k, m);
        const x = ax + (bx - ax) * t + between(-2, 2);
        const z = az + (bz - az) * t + between(-2, 2);
        if (rand() < 0.12 || clearing(x, z)) continue; // falha na crista
        place(k, x, z, rand() * 6.28, [s * between(1, 1.35), s * between(0.9, 1.5), s]);
        collide(x, z, m * 0.42);
        taken.push({ x, z, d: m * 0.4 });
      }
    }
  }

  // Monólitos (desenhados pelo cliente, sem modelo do kit).
  for (const c of CIRCLES) {
    const a0 = rand() * 6.28;
    for (let i = 0; i < c.n; i++) {
      const a = a0 + (i / c.n) * Math.PI * 2 + between(-0.12, 0.12);
      const x = c.x + Math.cos(a) * c.r;
      const z = c.z + Math.sin(a) * c.r;
      if (!freeOfColliders(x, z, 1)) continue;
      const roll = rand();
      const k = roll < 0.45 ? 'monolith' : roll < 0.75 ? 'monolith_broken' : 'monolith_fallen';
      const s = between(0.9, 1.25);
      // De pé, olha para o centro; tombado, cai para fora.
      const rot = k === 'monolith_fallen' ? -a + between(-0.4, 0.4) : facing(x, z, c.x, c.z) + between(-0.2, 0.2);
      place(k, x, z, rot, [s, s * between(0.9, 1.15), s]);
      collide(x, z, k === 'monolith_fallen' ? 1.1 * s : 0.75 * s);
      taken.push({ x, z, d: 1.5 });
    }
    // Altar no meio: o toco largo de um monólito antigo, partido rente ao chão.
    const s = between(1.5, 1.8);
    place('monolith_broken', c.x, c.z, rand() * 6.28, [s, s * 0.6, s]);
    collide(c.x, c.z, 0.8 * s);
    taken.push({ x: c.x, z: c.z, d: 2 });
  }

  // Transição: a floresta seca e rareia, as pedras aumentam.
  scatter(70, -215, -150, (x, z) => rand() < (1 - rocky(x, z)) * 0.95, (x, z) => {
    const lush = rand() < 0.22 * (1 - rocky(x, z));
    const s = between(2.4, 3.2) * (lush ? 1 : 1.15);
    place(lush ? pick(LUSH) : pick(BARE_ALL), x, z, rand() * 6.28, s);
    collide(x, z, 0.22 * s);
  }, { spacing: 7.5 });
  // Árvores mortas solitárias no vale.
  scatter(16, MIN_Z, -210, () => true, (x, z) => {
    const s = between(2.8, 3.8);
    place(pick(BARE_ALL), x, z, rand() * 6.28, s);
    collide(x, z, 0.22 * s);
  }, { spacing: 14 });
  // Pedras grandes (menos dentro da área de caça, para dar espaço à luta).
  scatter(120, MIN_Z, -158, (x, z) => rand() < rocky(x, z) * (inVale(x, z) ? 0.3 : 1), (x, z) => {
    const k = pick(BOULDER);
    const m = between(1.6, 4.2);
    const s = sized(k, m);
    place(k, x, z, rand() * 6.28, [s, s * between(0.8, 1.25), s]);
    collide(x, z, m * 0.4);
  }, { spacing: 4 });
  // Pedras médias (as maiores bloqueiam).
  scatter(170, MIN_Z, -155, (x, z) => rand() < 0.15 + rocky(x, z) * 0.85, (x, z) => {
    const k = pick(STONE_MID);
    const m = between(0.6, 1.8);
    place(k, x, z, rand() * 6.28, sized(k, m));
    if (m > 1.2) collide(x, z, m * 0.38);
  }, { spacing: 2 });
  // Cascalho (só visual).
  scatter(700, MIN_Z, -152, (x, z) => rand() < rocky(x, z), (x, z) => {
    const k = pick(GRAVEL);
    place(k, x, z, rand() * 6.28, sized(k, between(0.18, 0.55)));
  }, { avoidClearing: false });
  // Arbustos e grama rareando; capim baixo e esparso no vale.
  scatter(40, -210, -150, (x, z) => rand() < (1 - rocky(x, z)) * 0.9, (x, z) => place(pick(BUSH), x, z, rand() * 6.28, between(2, 3)));
  scatter(340, -232, -150, (x, z) => rand() < (1 - rocky(x, z)) * 0.9 + 0.05, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.6, 1.2)), { avoidClearing: false });
  scatter(130, MIN_Z, -215, () => true, (x, z) => place(pick(GRASS), x, z, rand() * 6.28, between(0.45, 0.85)), { avoidClearing: false });

  // Montanhas de rocha nua em volta do vale.
  for (let z = -176; z >= bounds.minZ - 26; z -= 26) {
    for (const x of [bounds.minX - 18, bounds.maxX + 18]) {
      place(pick(ROCK_MOUNTAINS), x + between(-4, 4), z + between(-4, 4), rand() * 6.28, between(20, 30));
    }
  }
  for (let x = bounds.minX - 24; x <= bounds.maxX + 24; x += 26) {
    place(pick(ROCK_MOUNTAINS), x + between(-4, 4), bounds.minZ - 18 + between(-4, 4), rand() * 6.28, between(22, 32));
  }

  return { ...BIOME, vale: VALE };
}

// ---------- Pintura do chão (cliente: client/src/world/map-painter.ts) ----------
/**
 * A grama seca, vira terra batida e depois cascalho e rocha, com a borda ondulada de
 * `transition` (as árvores e pedras param no mesmo lugar). `bio` é a entrada do bioma no
 * world.json; `h` traz a projeção do mapa (`p`), os limites do mapa (`bounds`) e o PRNG
 * (`rng(seed)`).
 */
export function paint(g, bio, h) {
  const { p, bounds: b } = h;
  const top = Math.max(0, Math.floor(p.cy(bio.zFrom + bio.amp + 2)));
  const rows = p.height - top;
  if (rows <= 0) return;
  const img = g.getImageData(0, top, p.width, rows);
  const d = img.data;
  const DRY = [124, 118, 76];
  const DIRT = [122, 104, 80];
  const GRAVEL_RGB = [136, 131, 121];
  const mix = (a, c, t, i) => a[i] + (c[i] - a[i]) * t;
  for (let y = 0; y < rows; y++) {
    const z = b.maxZ - (top + y + 0.5) / p.k;
    for (let x = 0; x < p.width; x++) {
      const wx = b.minX + (x + 0.5) / p.k;
      const t = transition(bio, wx, z);
      if (t <= 0.002) continue;
      // Variação fina (manchas de terra e pedra) para o chão não ficar liso.
      const n = Math.sin(wx * 0.9 + z * 0.35) * Math.sin(z * 0.8 - wx * 0.3) * 0.5 + Math.sin(wx * 2.7 + z * 3.1) * 0.25;
      const u = Math.max(0, Math.min(1, t + n * 0.18));
      const a = Math.min(1, t * 1.6); // a grama começa a secar antes das pedras aparecerem
      const i = (y * p.width + x) * 4;
      for (let c = 0; c < 3; c++) {
        const target = u < 0.5 ? mix(DRY, DIRT, u * 2, c) : mix(DIRT, GRAVEL_RGB, (u - 0.5) * 2, c);
        d[i + c] = d[i + c] + (target + n * 10 - d[i + c]) * a;
      }
    }
  }
  g.putImageData(img, 0, top);

  // Lajes de rocha exposta, veios claros e pedrinhas, mais densos no fundo do vale.
  const rand = h.rng(31);
  const area = ((b.maxX - b.minX) * (b.maxZ - (bio.zFrom + bio.amp))) / 1000;
  for (let i = 0; i < 260 * area; i++) {
    const wx = b.minX + rand() * (b.maxX - b.minX);
    const wz = bio.zFrom + bio.amp + rand() * (b.minZ - bio.zFrom - bio.amp);
    const t = transition(bio, wx, wz);
    if (rand() > t) continue;
    const kind = rand();
    if (kind < 0.18) {
      g.fillStyle = rand() < 0.5 ? 'rgba(92,88,82,0.35)' : 'rgba(170,165,154,0.3)';
      g.beginPath();
      g.ellipse(p.cx(wx), p.cy(wz), (0.8 + rand() * 2.6) * p.k, (0.5 + rand() * 1.6) * p.k, rand() * Math.PI, 0, Math.PI * 2);
      g.fill();
    } else if (kind < 0.26) {
      g.strokeStyle = 'rgba(60,56,50,0.4)';
      g.lineWidth = 0.08 * p.k;
      g.beginPath();
      let x = wx;
      let z = wz;
      g.moveTo(p.cx(x), p.cy(z));
      for (let s = 0; s < 4; s++) {
        x += (rand() - 0.5) * 2.4;
        z += (rand() - 0.5) * 2.4;
        g.lineTo(p.cx(x), p.cy(z));
      }
      g.stroke();
    } else {
      g.fillStyle = rand() < 0.5 ? 'rgba(70,66,60,0.45)' : 'rgba(190,184,172,0.45)';
      const s = (0.1 + rand() * 0.25) * p.k;
      g.fillRect(p.cx(wx), p.cy(wz), s, s);
    }
  }
}
