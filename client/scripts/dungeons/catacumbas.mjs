// Catacumbas de Vel'Aran (masmorra 1, tunnel#140): dois andares de 60 × 60 m, gerados num mapa só.
//
// É um bioma no mesmo contrato do gerador do mundo (scripts/world-gen.mjs, README "Como criar um bioma"), com as peças do kit
// das masmorras (build-dungeon-kit.mjs, KayKit Dungeon Remastered, CC0). Quem roda é scripts/build-dungeons.mjs, que grava o
// mapa para o cliente (public/worlds/catacumbas.json) e para o servidor (server/internal/world/maps/catacumbas.json). O
// servidor lê do mesmo arquivo a colisão, o ponto de entrada, as escadas e o lugar de cada monstro (campo `dungeon`).
//
// Layout (determinístico, semente fixa): cada andar é uma grade de 15 × 15 células de 4 m (60 × 60 m), repartida em 3 × 3
// blocos de 5 células. Cada bloco leva uma sala (3 ou 4 células de lado, sempre com uma célula de folga dos blocos vizinhos,
// para duas salas nunca se emendarem sozinhas); as salas se ligam por corredores de uma célula (4 m), numa árvore geradora
// com mais dois corredores que fecham laços. As paredes ficam nas bordas entre célula livre e rocha, baixas (2,2 m) para a
// câmera enxergar por cima (a masmorra não tem teto).
//
//   Andar 1 (z de −6 a 54):  a entrada é a origem (0, 0), na sala do meio-sul; a escada sobe na sala mais distante.
//   Andar 2 (z de 74 a 134): chega-se pela escada, na sala do meio-sul; o Abade Ulric fica numa sala de 20 × 20 m ao norte.
//   Entre os dois (z de 54 a 74) é rocha sólida, sem acesso: o servidor trata os dois andares como um mapa só.
//
// O chão é pintado (paint, mais abaixo), e os monstros são ~30 esqueletos de papel, com o chefe no fim.
export const name = "Catacumbas de Vel'Aran";
export const seed = 20261140;
export const order = 10;

const CELL = 4;
const N = 15;
const X0 = -30;
const FLOOR_Z0 = [-6, 74]; // z do começo de cada andar (o andar 1 tem a origem na célula (7, 1))
export const bounds = { minX: X0, maxX: X0 + N * CELL, minZ: FLOOR_Z0[0], maxZ: FLOOR_Z0[1] + N * CELL };

const WALL_Y = 0.55; // escala em altura: 4 m viram 2,2 m
const WALL_PILLAR = 0.7; // pilar de canto: escala no chão
const ENTRY = { x: 0, z: 0 }; // onde o grupo chega (a origem do servidor: a zona segura e o renascimento)
const SAFE = 5;

// Os monstros (ids do servidor, game/catacumbas.go) e o nível de cada andar.
const MINION = 'cata_minion';
const GUARD = 'cata_guard';
const MAGE = 'cata_acolyte';
const LURKER = 'cata_lurker';
const BOSS = 'abade_ulric';
const LEVELS = [[12, 14], [14, 16]];

const cellX = (i) => X0 + i * CELL;
const cellZ = (f, j) => FLOOR_Z0[f] + j * CELL;
const centerX = (i) => cellX(i) + CELL / 2;
const centerZ = (f, j) => cellZ(f, j) + CELL / 2;

function shuffle(rand, list) {
  const a = [...list];
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(rand() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

/** Sala de um bloco: o miolo do bloco, com folga (1 célula) dos blocos vizinhos. */
function roomOf(rand, bx, bz) {
  const span = (b) => [b * 5 + (b > 0 ? 1 : 0), b * 5 + 5 - (b < 2 ? 1 : 0)]; // [lo, hi) em células
  const [xlo, xhi] = span(bx);
  const [zlo, zhi] = span(bz);
  const w = 3 + Math.floor(rand() * (xhi - xlo - 3 + 1));
  const h = 3 + Math.floor(rand() * (zhi - zlo - 3 + 1));
  return { bx, bz, x0: xlo + Math.floor(rand() * (xhi - xlo - w + 1)), z0: zlo + Math.floor(rand() * (zhi - zlo - h + 1)), w, h };
}

export function build(ctx) {
  const { place, collide, torch, rand, pick, between, alongX } = ctx;
  ctx.meta.safeRadius = SAFE;
  ctx.meta.wall = { apothem: SAFE - 1, sides: 12 }; // o círculo de pedra da entrada

  const spawns = [];
  const stairs = [];
  const floors = [];
  const runs = []; // [andar, i, j, comprimento]: faixas de células livres (a pintura do chão)
  const stats = { rooms: 0, cells: 0, walls: 0 };

  for (const f of [0, 1]) {
    // ---------- Salas ----------
    const rooms = new Map();
    for (let bz = 0; bz < 3; bz++) {
      for (let bx = 0; bx < 3; bx++) {
        let r = roomOf(rand, bx, bz);
        if (bx === 1 && bz === 0) r = { bx, bz, x0: 6, z0: 0, w: 3, h: 3 }; // a sala da entrada (a origem fica nela)
        if (f === 1 && bx === 1 && bz === 2) r = { bx, bz, x0: 5, z0: 10, w: 5, h: 5 }; // a sala do chefe: 20 × 20 m
        r.cx = r.x0 + Math.floor(r.w / 2);
        r.cz = r.z0 + Math.floor(r.h / 2);
        r.key = `${bx},${bz}`;
        rooms.set(r.key, r);
      }
    }
    // Um canto a menos em cada andar (variedade): nunca o da escada nem o do chefe, que ainda nem foram escolhidos.
    const corners = shuffle(rand, ['0,0', '2,0', '0,2', '2,2']);
    rooms.delete(corners[0]);

    // ---------- Ligações: árvore geradora (Prim aleatório) + dois laços ----------
    const adj = (r) =>
      [[1, 0], [-1, 0], [0, 1], [0, -1]].map(([dx, dz]) => rooms.get(`${r.bx + dx},${r.bz + dz}`)).filter(Boolean);
    const inTree = new Set(['1,0']);
    const edges = [];
    const dist = new Map([['1,0', 0]]);
    while (inTree.size < rooms.size) {
      const frontier = [];
      for (const k of inTree) for (const n of adj(rooms.get(k))) if (!inTree.has(n.key)) frontier.push([rooms.get(k), n]);
      const [a, b] = frontier[Math.floor(rand() * frontier.length)];
      inTree.add(b.key);
      edges.push([a, b]);
      dist.set(b.key, dist.get(a.key) + 1);
    }
    const has = (a, b) => edges.some(([x, y]) => (x === a && y === b) || (x === b && y === a));
    const spare = [];
    for (const r of rooms.values()) for (const n of adj(r)) if (r.key < n.key && !has(r, n)) spare.push([r, n]);
    for (const e of shuffle(rand, spare).slice(0, 2)) edges.push(e);

    // ---------- Células livres ----------
    const walk = Array.from({ length: N }, () => new Array(N).fill(false));
    for (const r of rooms.values()) for (let j = r.z0; j < r.z0 + r.h; j++) for (let i = r.x0; i < r.x0 + r.w; i++) walk[j][i] = true;
    const carve = (i, j) => { walk[j][i] = true; };
    for (const [a, b] of edges) {
      if (a.bz === b.bz) {
        for (let i = Math.min(a.cx, b.cx); i <= Math.max(a.cx, b.cx); i++) carve(i, a.cz);
        for (let j = Math.min(a.cz, b.cz); j <= Math.max(a.cz, b.cz); j++) carve(b.cx, j);
      } else {
        for (let j = Math.min(a.cz, b.cz); j <= Math.max(a.cz, b.cz); j++) carve(a.cx, j);
        for (let i = Math.min(a.cx, b.cx); i <= Math.max(a.cx, b.cx); i++) carve(i, b.cz);
      }
    }
    const isWalk = (i, j) => i >= 0 && j >= 0 && i < N && j < N && walk[j][i];
    for (let j = 0; j < N; j++) {
      for (let i = 0; i < N; i++) {
        if (!walk[j][i]) continue;
        stats.cells++;
        let len = 1;
        if (i > 0 && walk[j][i - 1]) continue; // só o começo de cada faixa
        while (isWalk(i + len, j)) len++;
        runs.push([f, i, j, len]);
      }
    }
    floors.push({ minX: X0, maxX: X0 + N * CELL, minZ: cellZ(f, 0), maxZ: cellZ(f, N) });

    // ---------- Paredes (nas bordas entre célula livre e rocha) e pilares de canto ----------
    const verts = new Set();
    const wallAt = (x, z, vertical) => {
      const k = rand() < 0.12 ? 'dg_wall_broken' : 'dg_wall';
      place(k, x, z, vertical ? alongX(0, 1) : 0, [1, WALL_Y, 1]);
      stats.walls++;
      for (const o of [-1.5, -0.5, 0.5, 1.5]) collide(vertical ? x : x + o, vertical ? z + o : z, 0.5);
      for (const s of [-2, 2]) verts.add(vertical ? `${x},${z + s}` : `${x + s},${z}`);
    };
    for (let j = 0; j < N; j++) {
      for (let i = 0; i < N; i++) {
        if (!walk[j][i]) continue;
        const cx = centerX(i);
        const cz = centerZ(f, j);
        if (!isWalk(i, j - 1)) wallAt(cx, cz - CELL / 2, false);
        if (!isWalk(i, j + 1)) wallAt(cx, cz + CELL / 2, false);
        if (!isWalk(i - 1, j)) wallAt(cx - CELL / 2, cz, true);
        if (!isWalk(i + 1, j)) wallAt(cx + CELL / 2, cz, true);
      }
    }
    for (const v of verts) {
      const [x, z] = v.split(',').map(Number);
      place('dg_pillar', x, z, 0, [WALL_PILLAR, WALL_Y, WALL_PILLAR]);
    }

    // ---------- Escadas ----------
    // Andar 1: sobe na sala mais distante da entrada; andar 2: desce na sala da chegada (a mesma da entrada).
    const list = [...rooms.values()].filter((r) => r.key !== '1,0');
    const farthest = f === 0 ? list.filter((r) => r.key !== '1,0').sort((a, b) => dist.get(b.key) - dist.get(a.key) || (a.key < b.key ? -1 : 1))[0] : rooms.get('1,0');
    const stairRoom = farthest;
    // Visual: a escada encostada na parede norte (sobe) ou sul (desce) da sala; o portão fica 1,2 m à frente dela.
    const up = f === 0;
    const sx = centerX(stairRoom.cx);
    const wallZ = up ? cellZ(f, stairRoom.z0 + stairRoom.h) : cellZ(f, stairRoom.z0);
    const sz = up ? wallZ - 2.4 : wallZ + 2.4;
    place('dg_stairs', sx, sz, up ? 0 : Math.PI, [0.7, 0.45, 0.7]);
    collide(sx, sz + (up ? 0.6 : -0.6), 1.4);
    place('dg_pillar_deco', sx - 2.2, up ? wallZ - 0.9 : wallZ + 0.9, 0, [0.55, 0.55, 0.55]);
    place('dg_pillar_deco', sx + 2.2, up ? wallZ - 0.9 : wallZ + 0.9, 0, [0.55, 0.55, 0.55]);
    stairRoom.stairs = { x: sx, z: up ? wallZ - 5.0 : wallZ + 5.0, up };
    stairs.push({ f, ...stairRoom.stairs });

    // ---------- Decoração: cantos das salas, tochas e o resto ----------
    const wallOut = (i, j, di, dj) => !isWalk(i + di, j) && !isWalk(i, j + dj);
    for (const r of rooms.values()) {
      stats.rooms++;
      const corners = [[r.x0, r.z0, -1, -1], [r.x0 + r.w - 1, r.z0, 1, -1], [r.x0, r.z0 + r.h - 1, -1, 1], [r.x0 + r.w - 1, r.z0 + r.h - 1, 1, 1]];
      for (const [i, j, di, dj] of corners) {
        if (!wallOut(i, j, di, dj)) continue;
        const x = centerX(i) + di * 1.0;
        const z = centerZ(f, j) + dj * 1.0;
        if (Math.hypot(x - ENTRY.x, z - ENTRY.z) < 7 && f === 0) continue;
        const roll = rand();
        if (roll < 0.25) { place('dg_barrel', x, z, rand() * 6.28, 0.75); collide(x, z, 0.6); }
        else if (roll < 0.5) { place('dg_box', x, z, rand() * 6.28, 0.8); collide(x, z, 0.6); }
        else if (roll < 0.65) { place('dg_rubble', x, z, rand() * 6.28, 0.55); collide(x, z, 1.0); }
        else if (roll < 0.8) { place('dg_column', x, z, 0, 1); collide(x, z, 0.4); }
        else if (roll < 0.9) place('dg_candles', x, z, rand() * 6.28, 1.4);
        else torch(x, z);
      }
      // Um braseiro na borda de cada sala (a de dentro e as do meio levam dois).
      torch(centerX(r.x0) + 1.2, centerZ(f, r.z0 + r.h - 1) + 1.2);
      if (r.w * r.h >= 12) torch(centerX(r.x0 + r.w - 1) - 1.2, centerZ(f, r.z0) - 1.2);
    }

    // ---------- A sala do chefe: pilares em volta e o trono do Abade ----------
    if (f === 1) {
      const boss = rooms.get('1,2');
      const bx = centerX(boss.cx);
      const bz = centerZ(1, boss.cz);
      for (const [dx, dz] of [[-6, -6], [6, -6], [-6, 6], [6, 6]]) {
        place('dg_pillar_deco', bx + dx, bz + dz, 0, [0.85, 0.7, 0.85]);
        collide(bx + dx, bz + dz, 0.9);
      }
      for (const dx of [-3, 3]) place('dg_banner', bx + dx, bz + 9.2, 0, [1, 1, 1]);
      spawns.push({ m: BOSS, x: bx, z: bz + 1, l: LEVELS[1][1], n: 1, f });
      boss.boss = true;
    }

    // ---------- Monstros (a sala da chegada e a do chefe ficam de fora; a escada do andar 1 também tem guarda) ----------
    const fighting = [...rooms.values()].filter((r) => r.key !== '1,0' && !r.boss);
    const [lvLo, lvHi] = LEVELS[f];
    const maxD = Math.max(...fighting.map((r) => dist.get(r.key)));
    const packRoom = shuffle(rand, fighting)[0];
    for (const r of fighting) {
      const level = Math.round(lvLo + ((lvHi - lvLo) * dist.get(r.key)) / Math.max(1, maxD));
      const x = centerX(r.cx);
      const z = centerZ(f, r.cz);
      if (r === packRoom) {
        // Uma matilha de lacaios (arquétipo: puxar um puxa os três).
        spawns.push({ m: MINION, x, z, l: level, n: 3, f });
        continue;
      }
      // Um par: o papel de cada um é a graça da sala. O andar 2 pesa mais para os magos e os guardas.
      const mix = f === 0
        ? [[GUARD, MAGE], [LURKER, LURKER], [GUARD, LURKER], [MAGE, LURKER]]
        : [[GUARD, MAGE], [MAGE, MAGE], [GUARD, GUARD], [LURKER, MAGE]];
      const [a, b] = pick(mix);
      // Um de cada lado do centro, longe das paredes (a sala tem no mínimo 12 m).
      const off = Math.min(r.w, r.h) >= 4 ? 2.6 : 1.8;
      const alongZ = rand() < 0.5;
      spawns.push({ m: a, x: x + (alongZ ? 0 : -off), z: z + (alongZ ? -off : 0), l: level, n: 1, f });
      spawns.push({ m: b, x: x + (alongZ ? 0 : off), z: z + (alongZ ? off : 0), l: Math.min(lvHi, level + (rand() < 0.4 ? 1 : 0)), n: 1, f });
    }
  }

  // As escadas se ligam: a de subida do andar 1 chega na frente da de descida do andar 2, e vice-versa.
  const [up, down] = stairs;
  const link = [
    { name: 'Escada para o Andar 2', x: up.x, z: up.z, toX: down.x, toZ: down.z + 2.5 },
    { name: 'Escada para o Andar 1', x: down.x, z: down.z, toX: up.x, toZ: up.z - 2.5 },
  ];

  const round = (v) => Math.round(v * 100) / 100;
  return {
    // O servidor lê `dungeon`; o cliente lê `runs` (a pintura do chão).
    dungeon: {
      entry: ENTRY,
      spawns: spawns.map((s) => ({ ...s, x: round(s.x), z: round(s.z) })),
      stairs: link.map((s) => ({ ...s, x: round(s.x), z: round(s.z), toX: round(s.toX), toZ: round(s.toZ) })),
      floors,
    },
    runs,
    cell: CELL,
    x0: X0,
    floorZ: FLOOR_Z0,
    cells: N,
    stats,
  };
}

// ---------- Pintura do chão (cliente: client/src/world/map-painter.ts) ----------
// Rocha escura em todo o mapa e, por cima, as lajes de pedra de cada faixa de células livres: cada célula de 4 m é uma laje
// com rejunte, um tom por laje e rachaduras. Mesma assinatura dos outros biomas: (ctx 2D, entrada do bioma, { p, bounds, rng }).
export function paint(g, bio, h) {
  const { p } = h;
  const rand = h.rng(7919);
  const k = p.k;
  const cell = bio.cell * k;
  g.fillStyle = '#17181f';
  g.fillRect(0, 0, p.width, p.height);
  const tones = ['#4b4d5b', '#464857', '#505261', '#42444f', '#53515c'];
  for (const [f, i, j, len] of bio.runs) {
    const x = p.cx(bio.x0 + i * bio.cell);
    const y = p.cy(bio.floorZ[f] + (j + 1) * bio.cell);
    for (let n = 0; n < len; n++) {
      const cx = x + n * cell;
      g.fillStyle = tones[Math.floor(rand() * tones.length)];
      g.fillRect(cx, y, cell, cell);
      // Lajes menores dentro da célula (2 × 2), cada uma um pouco diferente.
      for (let a = 0; a < 2; a++) {
        for (let b = 0; b < 2; b++) {
          g.fillStyle = `rgba(${rand() < 0.5 ? '20,20,28' : '150,150,170'},${0.05 + rand() * 0.1})`;
          g.fillRect(cx + (a * cell) / 2, y + (b * cell) / 2, cell / 2, cell / 2);
        }
      }
      g.strokeStyle = 'rgba(14,14,20,0.6)';
      g.lineWidth = 0.09 * k;
      g.strokeRect(cx, y, cell, cell);
      g.beginPath();
      g.moveTo(cx + cell / 2, y);
      g.lineTo(cx + cell / 2, y + cell);
      g.moveTo(cx, y + cell / 2);
      g.lineTo(cx + cell, y + cell / 2);
      g.stroke();
      if (rand() < 0.3) {
        g.strokeStyle = 'rgba(10,10,14,0.55)';
        g.lineWidth = 0.05 * k;
        g.beginPath();
        let px = cx + rand() * cell;
        let py = y + rand() * cell;
        g.moveTo(px, py);
        for (let s = 0; s < 4; s++) {
          px += (rand() - 0.5) * cell * 0.4;
          py += (rand() - 0.5) * cell * 0.4;
          g.lineTo(px, py);
        }
        g.stroke();
      }
    }
  }
}
