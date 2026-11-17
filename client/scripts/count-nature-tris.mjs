// Triângulos de natureza no mapa principal: KayKit (env-forest.glb, env-town.glb) contra o kit novo (nature_*_b.glb),
// somando as instâncias de world.json (thin instances desenham todas, sem corte por instância). Uso (em client/).
import { NodeIO } from '@gltf-transform/core';
import { EXTMeshoptCompression, KHRTextureBasisu } from '@gltf-transform/extensions';
import { readFileSync } from 'node:fs';
const io = new NodeIO().registerExtensions([EXTMeshoptCompression, KHRTextureBasisu]);
const world = JSON.parse(readFileSync('public/world.json', 'utf8'));
const map = JSON.parse(readFileSync('src/world/nature-map.json', 'utf8'));
const tris = (m) => m.listPrimitives().reduce((n, p) => n + (p.getIndices()?.getCount() ?? 0) / 3, 0);
const old = {};
for (const f of ['public/models/env-forest.glb', 'public/models/env-town.glb']) {
  const d = await io.read(f);
  for (const n of d.getRoot().listNodes()) if (n.getName().startsWith('item:')) {
    const walk = (x) => { let t = x.getMesh() ? tris(x.getMesh()) : 0; for (const c of x.listChildren()) t += walk(c); return t; };
    old[n.getName().slice(5)] = walk(n);
  }
}
const neu = {};
for (const m of map.models) neu[m] = tris((await io.read(`public/models/nature_${m}_b.glb`)).getRoot().listMeshes()[0]);
let a = 0, b = 0;
const cat = {};
for (const o of world.objects) {
  const e = map.kinds[o.k];
  if (!e) continue;
  const g = o.k.replace(/_.*/, '').replace('hills', 'mountain');
  cat[g] ??= [0, 0, 0];
  cat[g][0]++; cat[g][1] += old[o.k] ?? 0; cat[g][2] += neu[e.model];
  a += old[o.k] ?? 0; b += neu[e.model];
}
console.log('grupo: instâncias, triângulos KayKit -> novo');
for (const [g, v] of Object.entries(cat)) console.log(g.padEnd(10), v[0], v[1], '->', v[2]);
console.log('total', a, '->', b, '(sem os extras do remaster: lápides e cristais)');
