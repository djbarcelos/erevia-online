// Heróis v4: põe os encaixes de arma (handslot.r/l) na escala 0,4 do padrão das pegas do jogo
// (WEAPON_GRIPS em client/src/world/models.ts: "0,4 m reais por unidade"; o handslot dos heróis do
// remaster tem escala 0,4). O Blender não guarda escala no repouso de um osso, então é feito no GLB.
//   node tools/art/herois-v4/slots-glb.mjs <entrada.glb> [saida.glb]   (rodar de client/ ou da raiz)
import { createRequire } from 'node:module';
const require = createRequire(new URL('../../../client/package.json', import.meta.url));
const { NodeIO } = require('@gltf-transform/core');
const { ALL_EXTENSIONS } = require('@gltf-transform/extensions');
const [src, dst = src] = process.argv.slice(2);
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const doc = await io.read(src);
let n = 0;
for (const node of doc.getRoot().listNodes()) {
  if (node.getName() === 'handslot.r' || node.getName() === 'handslot.l') {
    node.setScale([0.4, 0.4, 0.4]);
    n++;
  }
}
// o Blender grava trilhas para todos os ossos (escala 1 inclusive): sem tirar, o clipe desfaz a escala do encaixe
let removed = 0;
for (const anim of doc.getRoot().listAnimations()) {
  for (const ch of anim.listChannels()) {
    const t = ch.getTargetNode();
    if (t && (t.getName() === 'handslot.r' || t.getName() === 'handslot.l')) {
      const smp = ch.getSampler();
      ch.dispose();
      if (smp && smp.listParents().every((p) => p === anim)) smp.dispose();
      removed++;
    }
  }
}
console.log(`trilhas de animação dos encaixes removidas: ${removed}`);
await io.write(dst, doc);
console.log(`encaixes ajustados: ${n} (${dst})`);
