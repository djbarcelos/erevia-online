// Normaliza os GLB crus do Meshy (assets-src/meshy/raw/<peça>.glb) para o kit da cidade do remaster
// (tunnel#100): vira a frente para -Z (a convenção do kit KayKit: `facing()` do gerador do mundo
// aponta -Z para a praça), centra a pegada em x/z, põe o chão em y = 0 e escala para a pegada do
// prédio KayKit equivalente. Grava assets-src/meshy/town/town_<peça>_b.glb (o import-asset.mjs
// faz o resto: textura 1024 px, KTX2, orçamento e manifesto, ver tools/art/remaster-cidade/import.sh).
//
// As peças do kit entram no jogo no lugar do item de mesmo "kind" do world.json, com a MESMA escala
// do world.json. Por isso a escala vai embutida na malha: ela é calculada em "unidades do kit"
// (kit-sizes.json, o tamanho do item KayKit com escala 1). Casas e prédios: escala uniforme, a maior que
// cabe na pegada KayKit (largura e fundo), na altura (<= 1,15x a KayKit) e dentro de 1,5x o raio do
// colisor do servidor (a meia diagonal da pegada), para a parede nunca passar do que o servidor bloqueia.
// Muralha e portão: cada eixo vai exatamente ao tamanho KayKit (a muralha é uma tira de 12,8 m que se
// emenda com a vizinha). Adereços sem item no kit (banco, poste de luz, placa): tamanho em metros.
//
// Uso (dentro de client/): node scripts/build-remaster-town.mjs [peça ...]   (sem argumento: todas)
import { NodeIO } from '@gltf-transform/core';
import { flatten, getBounds, join } from '@gltf-transform/functions';
import { mkdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

const root = resolve(import.meta.dirname, '../..');
const RAW = resolve(root, 'assets-src/meshy/raw');
const OUT = resolve(root, 'assets-src/meshy/town');
const KIT = JSON.parse(readFileSync(resolve(import.meta.dirname, '../src/world/kit-sizes.json'), 'utf8'));

/** kind: item KayKit que a peça substitui; mode: 'fit' (uniforme) | 'axes' (cada eixo) | 'meters' (altura/largura em m). */
export const PIECES = {
  tavern: { kind: 'tavern', mode: 'fit', scale: 9 },
  church: { kind: 'church', mode: 'fit', scale: 9 },
  blacksmith: { kind: 'blacksmith', mode: 'fit', scale: 9 },
  market: { kind: 'market', mode: 'fit', scale: 9 },
  home_a: { kind: 'home_a', mode: 'fit', scale: 7.5 },
  home_b: { kind: 'home_b', mode: 'fit', scale: 7.5 },
  tower: { kind: 'tower_a', mode: 'fit', scale: 9 },
  wall: { kind: 'wall_straight', mode: 'axes' },
  gate: { kind: 'wall_straight_gate', mode: 'axes' },
  stall: { kind: 'tent', mode: 'fit', scale: 7, hmax: 1.0 },
  cart: { kind: 'wheelbarrow', mode: 'box', box: [0.55, 0.45, 0.55] }, // unidades do kit (x4,5 no mundo)
  bench: { mode: 'meters', width: 1.9 },
  lamp: { mode: 'meters', height: 3.6 },
  sign: { mode: 'meters', height: 2.4 },
  // Identidade das regiões (adereços avulsos, só no remaster)
  tomb: { mode: 'meters', height: 1.5 },
  crystal: { mode: 'meters', height: 2.4 },
  pillar: { mode: 'meters', height: 2.6 },
};

const io = new NodeIO();

function fixBadNormals(prim) {
  const pos = prim.getAttribute('POSITION');
  const nor = prim.getAttribute('NORMAL');
  const idx = prim.getIndices();
  if (!nor || !idx) return 0;
  const n = nor.getCount();
  const bad = new Set();
  const v = [0, 0, 0];
  for (let i = 0; i < n; i++) {
    nor.getElement(i, v);
    if (Math.hypot(v[0], v[1], v[2]) < 0.5) bad.add(i);
  }
  if (!bad.size) return 0;
  const acc = new Map([...bad].map((i) => [i, [0, 0, 0]]));
  const P = (i) => pos.getElement(i, [0, 0, 0]);
  for (let t = 0; t + 2 < idx.getCount(); t += 3) {
    const ids = [idx.getScalar(t), idx.getScalar(t + 1), idx.getScalar(t + 2)];
    if (!ids.some((i) => bad.has(i))) continue;
    const [a, b, c] = ids.map(P);
    const u = [b[0] - a[0], b[1] - a[1], b[2] - a[2]];
    const w = [c[0] - a[0], c[1] - a[1], c[2] - a[2]];
    const fn = [u[1] * w[2] - u[2] * w[1], u[2] * w[0] - u[0] * w[2], u[0] * w[1] - u[1] * w[0]]; // área x normal
    for (const i of ids) if (bad.has(i)) for (let k = 0; k < 3; k++) acc.get(i)[k] += fn[k];
  }
  for (const [i, a] of acc) {
    const l = Math.hypot(a[0], a[1], a[2]);
    nor.setElement(i, l > 1e-12 ? [a[0] / l, a[1] / l, a[2] / l] : [0, 1, 0]);
  }
  return bad.size;
}

export async function normalize(name) {
  const spec = PIECES[name];
  const doc = await io.read(resolve(RAW, `${name}.glb`));
  await doc.transform(flatten(), join({ keepNamed: false }));
  // O Meshy entrega alguns vértices com normal zerada (aparecem pretos no jogo): refaz a normal deles
  // com a das faces que os usam.
  let fixed = 0;
  for (const mesh of doc.getRoot().listMeshes()) {
    for (const prim of mesh.listPrimitives()) fixed += fixBadNormals(prim);
  }
  if (fixed) console.log(`  ${name}: ${fixed} normais zeradas refeitas`);
  const scene = doc.getRoot().getDefaultScene() ?? doc.getRoot().listScenes()[0];
  const b = getBounds(scene);
  const dim = b.max.map((v, i) => v - b.min[i]);
  const [w, h, d] = dim;
  const [kw, kh, kd] = spec.kind ? KIT[spec.kind] : [0, 0, 0];
  let s; // fator por eixo
  if (spec.mode === 'axes') s = [kw / w, kh / h, kd / d];
  else if (spec.mode === 'fit') {
    const R = 0.42 * Math.max(kw, kd); // raio do colisor em unidades do kit
    const u = Math.min(kw / w, kd / d, (kh * (spec.hmax ?? 1.15)) / h, (1.5 * R) / (0.5 * Math.hypot(w, d)));
    s = [u, u, u];
  } else if (spec.mode === 'box') {
    const u = Math.min(spec.box[0] / w, spec.box[1] / h, spec.box[2] / d);
    s = [u, u, u];
  } else {
    const u = spec.height ? spec.height / h : spec.width / w;
    s = [u, u, u];
  }
  const cx = (b.max[0] + b.min[0]) / 2;
  const cz = (b.max[2] + b.min[2]) / 2;
  const flip = -1; // a frente do Meshy é +Z; a do kit é -Z (gira 180 graus em Y)
  for (const mesh of doc.getRoot().listMeshes()) {
    for (const prim of mesh.listPrimitives()) {
      const pos = prim.getAttribute('POSITION');
      const nor = prim.getAttribute('NORMAL');
      const p = [0, 0, 0];
      for (let i = 0; i < pos.getCount(); i++) {
        pos.getElement(i, p);
        pos.setElement(i, [(p[0] - cx) * flip * s[0], (p[1] - b.min[1]) * s[1], (p[2] - cz) * flip * s[2]]);
        if (nor) {
          nor.getElement(i, p);
          const n = [(p[0] * flip) / s[0], p[1] / s[1], (p[2] * flip) / s[2]];
          const l = Math.hypot(...n) || 1;
          nor.setElement(i, [n[0] / l, n[1] / l, n[2] / l]);
        }
      }
      // A inversão de dois eixos (rotação) mantém o sentido dos triângulos; escala positiva também.
    }
  }
  for (const node of doc.getRoot().listNodes()) node.setTranslation([0, 0, 0]).setRotation([0, 0, 0, 1]).setScale([1, 1, 1]);
  mkdirSync(OUT, { recursive: true });
  const file = resolve(OUT, `town_${name}_b.glb`);
  await io.write(file, doc);
  const out = dim.map((v, i) => v * s[i]);
  return { name, file, size: out, unit: spec.kind ? KIT[spec.kind] : null, scale: spec.scale };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const names = process.argv.slice(2).length ? process.argv.slice(2) : Object.keys(PIECES);
  for (const n of names) {
    const r = await normalize(n);
    const m = r.scale ?? (r.unit ? (n === 'cart' ? 4.5 : n === 'wall' || n === 'gate' ? 1 : 1) : 1);
    const world = r.size.map((v) => (v * (r.scale ?? 1)).toFixed(1));
    console.log(`${n}: ${r.size.map((v) => v.toFixed(3)).join(' x ')} (kit)${r.scale ? `, no mundo ${world.join(' x ')} m` : ''}${r.unit ? `  [KayKit ${r.unit.slice(0, 3).join(' x ')}]` : ''}`);
  }
}
