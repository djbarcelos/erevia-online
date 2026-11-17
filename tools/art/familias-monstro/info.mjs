// Resumo de um GLB: animações (duração), pele (ossos), materiais, texturas. Uso (em client/): node ../tools/art/familias-monstro/info.mjs <glb...>
import { createRequire } from 'node:module';
const { NodeIO } = createRequire(import.meta.dirname + '/../../../client/')('@gltf-transform/core');
const io = new NodeIO();
for (const f of process.argv.slice(2)) {
  const d = await io.read(f);
  const r = d.getRoot();
  console.log('==', f);
  console.log('skins', r.listSkins().map((s) => s.listJoints().length), 'meshes', r.listMeshes().map((m) => m.getName()),
    'mats', r.listMaterials().map((m) => `${m.getName()}:${m.getBaseColorFactor().map((v) => +v.toFixed(2))}${m.getBaseColorTexture() ? '+tex' : ''}`),
    'tex', r.listTextures().map((t) => t.getSize()?.join('x')));
  for (const a of r.listAnimations()) {
    let end = 0;
    for (const s of a.listSamplers()) { const t = s.getInput(); end = Math.max(end, t.getElement(t.getCount() - 1, [0])[0]); }
    console.log('  anim', a.getName(), end.toFixed(2) + 's', a.listChannels().length + 'ch');
  }
}
