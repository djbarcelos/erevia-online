// Peças do Brejo de Lamagris (tunnel#139): normaliza os GLB do Meshy (image-to-3d, Smart Topology) de assets-src/meshy/tasks.jsonl
// e grava em assets-src/meshy/norm-brejo/<peça>.glb (base em y=0, centrada, a maior medida horizontal no eixo X). Os juncos
// são três cartões cruzados com recorte, pintados em build-reeds.mjs.
// Também acrescenta as peças a src/world/nature-map.json (modelo + escala que leva a peça ao tamanho real, em metros), que é
// o que o cliente lê para desenhar `fallen_log`, `reeds`, `ferryman_hut` e `boat` do world.json.
//
// Depois: sh tools/art/remaster-brejo/import.sh (importa para client/public/models/nature_<peça>_b.glb).
// Uso (dentro de client/): node scripts/build-brejo-kit.mjs
import { Document, NodeIO, getBounds } from '@gltf-transform/core';
import { dedup, flatten, join, prune, simplify, transformMesh } from '@gltf-transform/functions';
import { MeshoptSimplifier } from 'meshoptimizer';
import { buildReeds } from './build-reeds.mjs';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

const ROOT = resolve(import.meta.dirname, '../..');
const SRC = resolve(ROOT, 'assets-src/meshy');
const OUT = resolve(SRC, 'norm-brejo');
const MAP = resolve(import.meta.dirname, '../src/world/nature-map.json');

// peça do kit -> nome da task image-to-3d, medida real (m) e qual medida ela fixa ('h' = altura, 'l' = a maior horizontal).
const PIECES = {
  fallen_log: { task: 'tronco', size: 4.2, by: 'l', tris: 0.6 },
  ferryman_hut: { task: 'cabana', size: 5, by: 'h', tris: 0.6 },
  boat: { task: 'barco', size: 3.6, by: 'l', tris: 0.7 },
};
const tasks = readFileSync(resolve(SRC, 'tasks.jsonl'), 'utf8').split(/\r?\n/).filter(Boolean).map((l) => JSON.parse(l));
const idOf = (name) => tasks.filter((t) => t.endpoint === 'image-to-3d' && t.status === 'CREATED' && t.name === name).pop()?.id;
const io = new NodeIO();
mkdirSync(OUT, { recursive: true });
const map = JSON.parse(readFileSync(MAP, 'utf8'));
const dims = {};

for (const [piece, p] of Object.entries(PIECES)) {
  const id = idOf(p.task);
  const doc = await io.read(resolve(SRC, `image-to-3d-${id.slice(-8)}/glb-model.glb`));
  await doc.transform(flatten(), join({ keepNamed: false }), dedup(), prune());
  // Orçamento do guia (cenário: 3.000 triângulos no LOD0, 1.000 no LOD1): o Meshy passa um pouco do alvo.
  await doc.transform(simplify({ simplifier: MeshoptSimplifier, ratio: p.tris, error: 0.05 }));
  const scene = doc.getRoot().listScenes()[0];
  let b = getBounds(scene);
  // A maior medida horizontal no eixo X (gira 90° em Y quando está em Z).
  const rot = b.max[2] - b.min[2] > b.max[0] - b.min[0] && p.by === 'l';
  if (rot) {
    // (x, z) -> (z, -x)
    for (const mesh of doc.getRoot().listMeshes()) transformMesh(mesh, [0, 0, -1, 0, 0, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 1]);
    b = getBounds(scene);
  }
  const w = b.max[0] - b.min[0];
  const h = b.max[1] - b.min[1];
  const d = b.max[2] - b.min[2];
  const k = p.by === 'h' ? p.size / h : p.size / Math.max(w, d);
  const cx = (b.max[0] + b.min[0]) / 2;
  const cz = (b.max[2] + b.min[2]) / 2;
  // O arquivo guarda a peça já no tamanho real (m): o mapa leva escala 1.
  const m = [k, 0, 0, 0, 0, k, 0, 0, 0, 0, k, 0, -cx * k, -b.min[1] * k, -cz * k, 1];
  for (const mesh of doc.getRoot().listMeshes()) transformMesh(mesh, m);
  for (const n of doc.getRoot().listNodes()) { n.setTranslation([0, 0, 0]); n.setRotation([0, 0, 0, 1]); n.setScale([1, 1, 1]); }
  dims[piece] = [w * k, h * k, d * k];
  await io.write(resolve(OUT, `${piece}.glb`), doc);
  console.log(`${piece}: ${dims[piece].map((v) => v.toFixed(2)).join(' x ')} m${rot ? ' (girada)' : ''}`);
}

// Juncos: três cartões cruzados com recorte, pintados em scripts/build-reeds.mjs (o conceito chaveado por cor deixava um disco claro na base).
await buildReeds(OUT);
dims.reeds = [1.6, 1.9, 1.6];
console.log('reeds: 3 cartões com recorte (build-reeds.mjs)');

// nature-map.json: as peças novas entram como modelos e como itens do world.json (escala 1: o arquivo já tem o tamanho real).
for (const piece of Object.keys(dims)) {
  if (!map.models.includes(piece)) map.models.push(piece);
  map.kinds[piece] = { model: piece, s: [1, 1, 1] };
}
writeFileSync(MAP, JSON.stringify(map, null, 1) + '\n');
console.log(`nature-map.json: ${Object.keys(dims).join(', ')}`);
console.log(JSON.stringify(Object.fromEntries(Object.entries(dims).map(([k, v]) => [k, v.map((x) => +x.toFixed(2))]))));
