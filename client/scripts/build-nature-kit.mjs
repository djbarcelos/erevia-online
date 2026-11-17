// Kit de natureza do remaster (?remaster=1): pega os GLB do Meshy (image-to-3d, Smart Topology) listados em
// assets-src/meshy/tasks.jsonl, põe cada peça de pé no chão (base em y=0, centrada), com altura 1, e grava
// em assets-src/meshy/norm/<nome>.glb (o import-asset.mjs entrega o arquivo final a public/models).
// Também grava src/world/nature-map.json: para cada item do kit KayKit (tree_1_a, rock_2_c...), qual peça nova
// o substitui e a escala que a leva ao tamanho do item antigo (kit-sizes.json), para o cenário ficar nas
// mesmas posições, com a mesma altura (o vento depende dela) e os mesmos colisores.
// Uso (dentro de client/): node scripts/build-nature-kit.mjs
import { Document, NodeIO, getBounds } from '@gltf-transform/core';
import sharp from 'sharp';
import { flatten, join, prune, transformMesh, dedup, simplify } from '@gltf-transform/functions';
import { MeshoptSimplifier } from 'meshoptimizer';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

const ROOT = resolve(import.meta.dirname, '../..');
const SRC = resolve(ROOT, 'assets-src/meshy');
const sizes = JSON.parse(readFileSync(resolve(import.meta.dirname, '../src/world/kit-sizes.json'), 'utf8'));

// Peça do kit -> nome da task image-to-3d no tasks.jsonl.
const PIECES = ['oak', 'pine', 'dead', 'vale', 'bush', 'tuft', 'rock_s', 'rock_m', 'rock_l', 'cliff', 'grave', 'crystal'];
export const FILES = { oak: 'tree_oak', pine: 'tree_pine', dead: 'tree_dead', vale: 'tree_vale', bush: 'bush_leaf', tuft: 'grass_tuft', rock_s: 'rock_s', rock_m: 'rock_m', rock_l: 'rock_l', cliff: 'cliff', grave: 'grave', crystal: 'crystal' };

// Item KayKit -> peça nova.
const MAP = {
  tree_1_a: 'oak', tree_1_b: 'oak', tree_2_a: 'oak', tree_2_b: 'oak', tree_3_a: 'oak', tree_4_a: 'pine', tree_4_b: 'pine',
  tree_bare_1_a: 'dead', tree_bare_1_b: 'dead', tree_bare_2_a: 'dead', tree_bare_2_b: 'dead', tree_bare_1_c: 'dead', tree_bare_2_c: 'dead',
  bush_1_a: 'bush', bush_2_a: 'bush', bush_3_a: 'bush', bush_4_a: 'bush',
  grass_1_a: 'tuft', grass_2_a: 'tuft',
  rock_2_a: 'rock_s', rock_2_c: 'rock_s', rock_1_c: 'rock_s',
  rock_1_a: 'rock_m', rock_1_e: 'rock_m', rock_3_b: 'rock_m', rock_3_c: 'rock_m',
  rock_3_a: 'rock_l', rock_3_e: 'rock_l', rock_3_g: 'rock_l', rock_3_j: 'rock_l', rock_1_h: 'rock_l', rock_1_k: 'rock_l', rock_2_f: 'rock_l',
  mountain_a_grass_trees: 'cliff', mountain_b_grass: 'cliff', mountain_c_grass_trees: 'cliff', hills_a_trees: 'cliff', hills_b_trees: 'cliff',
  mountain_a: 'cliff', mountain_b: 'cliff', mountain_c: 'cliff', mountain_a_grass: 'cliff', hills_a: 'cliff', hills_c: 'cliff',
};
// Quanto a forma pode se deformar para caber no tamanho do item antigo (por eixo, contra a escala uniforme).
const STRETCH = { cliff: [0.35, 3], rock_l: [0.6, 1.7], rock_m: [0.6, 1.7], rock_s: [0.6, 1.7], bush: [0.7, 1.4], tuft: [0.5, 2], oak: [0.75, 1.3], pine: [0.75, 1.3], dead: [0.7, 1.4] };

// Fração de triângulos que fica depois do Meshy (o alvo do Meshy passa um pouco do orçamento nas árvores).
const SIMPLIFY = { pine: 0.9, vale: 0.9, rock_s: 0.4, rock_m: 0.55, rock_l: 0.7, bush: 0.8 };

const tasks = readFileSync(resolve(SRC, 'tasks.jsonl'), 'utf8').split(/\r?\n/).filter(Boolean).map((l) => JSON.parse(l));
const idOf = (name) => tasks.filter((t) => t.endpoint === 'image-to-3d' && t.status === 'CREATED' && t.name === name).pop()?.id;

const io = new NodeIO();

/**
 * O mato: o Meshy com 200 triângulos devolve estilhaços cinzentos. No lugar, o conceito pintado vira três
 * cartões cruzados com recorte (alpha): 6 triângulos, uma textura de 256 px, balança com o vento como o resto.
 */
async function makeTuft(conceitoPng, out) {
  const N = 256;
  const { data } = await sharp(conceitoPng).resize(N, N).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
  // Fundo: cinza claro quase liso; o que se afasta dele em cor ou brilho é planta.
  const bg = [0, 1, 2].map((c) => [data[c], data[(N - 1) * 4 + c], data[((N - 1) * N) * 4 + c], data[(N * N - 1) * 4 + c]].sort((a, b) => a - b)[1]);
  const rgba = Buffer.from(data);
  for (let i = 0; i < N * N; i++) {
    const d = Math.hypot(data[i * 4] - bg[0], data[i * 4 + 1] - bg[1], data[i * 4 + 2] - bg[2]);
    rgba[i * 4 + 3] = d > 26 ? 255 : 0;
  }
  const png = await sharp(rgba, { raw: { width: N, height: N, channels: 4 } }).png().toBuffer();
  const doc = new Document();
  const buf = doc.createBuffer();
  const tex = doc.createTexture("tuft").setImage(png).setMimeType("image/png");
  const mat = doc.createMaterial("tuft").setBaseColorTexture(tex).setAlphaMode("MASK").setAlphaCutoff(0.5).setDoubleSided(true).setMetallicFactor(0).setRoughnessFactor(1);
  const pos = [], nor = [], uv = [], idx = [];
  for (let q = 0; q < 3; q++) {
    const a = (q * Math.PI) / 3;
    const dx = Math.cos(a) * 0.5;
    const dz = Math.sin(a) * 0.5;
    const o = q * 4;
    pos.push(-dx, 0, -dz, dx, 0, dz, dx, 1, dz, -dx, 1, -dz);
    for (let k = 0; k < 4; k++) nor.push(0, 1, 0);
    uv.push(0, 1, 1, 1, 1, 0, 0, 0);
    idx.push(o, o + 1, o + 2, o, o + 2, o + 3);
  }
  const prim = doc.createPrimitive().setMaterial(mat)
    .setAttribute("POSITION", doc.createAccessor().setType("VEC3").setArray(new Float32Array(pos)).setBuffer(buf))
    .setAttribute("NORMAL", doc.createAccessor().setType("VEC3").setArray(new Float32Array(nor)).setBuffer(buf))
    .setAttribute("TEXCOORD_0", doc.createAccessor().setType("VEC2").setArray(new Float32Array(uv)).setBuffer(buf))
    .setIndices(doc.createAccessor().setType("SCALAR").setArray(new Uint16Array(idx)).setBuffer(buf));
  const mesh = doc.createMesh("tuft").addPrimitive(prim);
  doc.createScene("s").addChild(doc.createNode("tuft").setMesh(mesh));
  await io.write(out, doc);
}
mkdirSync(resolve(SRC, 'norm'), { recursive: true });
const dims = {};
for (const p of PIECES) {
  const id = idOf(p);
  if (!id) { console.warn(`sem task image-to-3d para ${p}`); continue; }
  if (p === 'tuft') {
    await makeTuft(resolve(ROOT, JSON.parse(readFileSync(resolve(SRC, 'concepts.json'), 'utf8')).tuft), resolve(SRC, 'norm/grass_tuft.glb'));
    dims.tuft = [1, 1, 1];
    console.log('grass_tuft: 3 cartões com recorte (conceito pintado)');
    continue;
  }
  const doc = await io.read(resolve(SRC, `image-to-3d-${id.slice(-8)}/glb-model.glb`));
  await doc.transform(flatten(), join({ keepNamed: false }), dedup(), prune());
  // Orçamento do guia: árvore até 1.500 triângulos no LOD0 (o Meshy passa um pouco do alvo).
  // As pedras e a montanha se repetem às centenas (thin instances desenham todas): mais leves ainda.
  if (SIMPLIFY[p]) await doc.transform(simplify({ simplifier: MeshoptSimplifier, ratio: SIMPLIFY[p], error: p === 'pine' || p === 'vale' ? 0.01 : p === 'rock_s' ? 0.5 : 0.08 }));
  const scene = doc.getRoot().listScenes()[0];
  const b = getBounds(scene);
  const h = b.max[1] - b.min[1];
  const k = 1 / h;
  const cx = (b.max[0] + b.min[0]) / 2;
  const cz = (b.max[2] + b.min[2]) / 2;
  // x' = (x - cx) * k, y' = (y - minY) * k, z' = (z - cz) * k
  const m = [k, 0, 0, 0, 0, k, 0, 0, 0, 0, k, 0, -cx * k, -b.min[1] * k, -cz * k, 1];
  for (const mesh of doc.getRoot().listMeshes()) transformMesh(mesh, m);
  for (const n of doc.getRoot().listNodes()) { n.setTranslation([0, 0, 0]); n.setRotation([0, 0, 0, 1]); n.setScale([1, 1, 1]); }
  dims[p] = [(b.max[0] - b.min[0]) * k, 1, (b.max[2] - b.min[2]) * k];
  const out = resolve(SRC, `norm/${FILES[p]}.glb`);
  await io.write(out, doc);
  console.log(`${FILES[p]}: ${dims[p].map((v) => v.toFixed(2)).join(' x ')} (altura original ${h.toFixed(2)})`);
}

const kinds = {};
for (const [kind, piece] of Object.entries(MAP)) {
  const d = dims[piece];
  const t = sizes[kind];
  if (!d || !t) continue;
  // Escala uniforme pela altura; cada eixo pode se afastar dela dentro do limite da peça.
  const u = t[1] / d[1];
  const [lo, hi] = STRETCH[piece];
  const fit = (target, native) => Math.min(u * hi, Math.max(u * lo, target / native));
  // Mato: touceira redonda (cartões 1:1), não fina como o capim KayKit.
  const s = (piece === "tuft" ? [u * 1.15, u, u * 1.15] : [fit(t[0], d[0]), u, fit(t[2], d[2])]).map((v) => +v.toFixed(3));
  kinds[kind] = { model: FILES[piece], s };
}
writeFileSync(resolve(import.meta.dirname, '../src/world/nature-map.json'), JSON.stringify({ models: Object.values(FILES), kinds }, null, 1) + '\n');
console.log(`nature-map.json: ${Object.keys(kinds).length} itens`);
