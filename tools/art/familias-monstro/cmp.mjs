import { createRequire } from 'node:module';
const { NodeIO } = createRequire('C:/Users/DanielTeofilo/erevia-wt/familias-monstro/client/')('@gltf-transform/core');
const io = new NodeIO();
for (const f of process.argv.slice(2)) {
  const d = await io.read(f); const p = d.getRoot().listMeshes()[0].listPrimitives()[0];
  const uv = p.getAttribute('TEXCOORD_0'); const pos = p.getAttribute('POSITION');
  const a = uv.getArray(); let mn=[9,9],mx=[-9,-9];
  for (let i=0;i<a.length;i+=2){mn[0]=Math.min(mn[0],a[i]);mx[0]=Math.max(mx[0],a[i]);mn[1]=Math.min(mn[1],a[i+1]);mx[1]=Math.max(mx[1],a[i+1]);}
  console.log(f.split('/').pop(), 'verts', pos.getCount(), 'tris', p.getIndices().getCount()/3, 'uv', mn, mx, 'first', Array.from(a.slice(0,6)).map(v=>+v.toFixed(3)), Array.from(pos.getArray().slice(0,3)).map(v=>+v.toFixed(3)));
}
