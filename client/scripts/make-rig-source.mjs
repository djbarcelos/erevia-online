// Corpo de um herói do remaster sem esqueleto, LOD nem textura, na pose de descanso, para pedir um rig
// novo ao Meshy (tunnel#181): a conta de hoje não enxerga os rigs da rodada anterior, e as ações da
// biblioteca só podem ser compradas para um rig da própria conta. O rig novo usa o mesmo molde
// (mixamo) e a mesma malha, então os clipes comprados servem nos GLB do jogo (mesmos nomes de osso).
//   cd client && node scripts/make-rig-source.mjs <heroi_b.glb> <saida.glb>
import { NodeIO } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';
import { prune } from '@gltf-transform/functions';

const [src, out] = process.argv.slice(2);
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const doc = await io.read(src);
const root = doc.getRoot();
for (const a of root.listAnimations()) a.dispose();
const keep = new Set(['Mesh_0']); // sem o elmo nem o chapéu (peças rígidas à parte, como no rig original)
for (const n of root.listNodes()) {
  n.setSkin(null);
  if (n.getMesh() && !keep.has(n.getName())) n.setMesh(null);
}
for (const s of root.listSkins()) s.dispose();
for (const m of root.listMeshes()) for (const p of m.listPrimitives()) {
  for (const k of ['JOINTS_0', 'WEIGHTS_0']) p.setAttribute(k, null);
  p.setMaterial(null);
}
for (const t of root.listTextures()) t.dispose();
for (const e of root.listExtensionsUsed()) e.dispose();
await doc.transform(prune());
await io.write(out, doc);
console.log('gravado', out);
