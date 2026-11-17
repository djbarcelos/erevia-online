// Triângulos, tamanho e textura de um GLB. Uso (em client/): node ../tools/art/remaster-natureza/inspect.mjs <glb...>
import { NodeIO, getBounds } from '@gltf-transform/core';
const io = new NodeIO();
for (const f of process.argv.slice(2)) {
  const d = await io.read(f);
  let tris = 0;
  for (const m of d.getRoot().listMeshes()) for (const p of m.listPrimitives()) tris += (p.getIndices()?.getCount() ?? p.getAttribute('POSITION').getCount()) / 3;
  const b = getBounds(d.getRoot().listScenes()[0]);
  const tex = d.getRoot().listTextures().map((t) => t.getSize()?.join('x'));
  console.log(f.split(/[\/]/).slice(-2).join('/'), 'tris', tris, 'size', b.max.map((v, i) => +(v - b.min[i]).toFixed(2)), 'min', b.min.map((v) => +v.toFixed(2)), 'tex', tex.join(','));
}
