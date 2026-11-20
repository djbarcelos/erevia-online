// Heróis v4: conferência do GLB do jogo (public/models/heroi_v4.glb), depois da importação.
// O que o Blender não vê: a mão saía aberta porque os dedos exportavam sem rotação. Aqui cada clipe precisa
// ter os dedos fechados no arquivo, e todos os clipes que o jogo pede precisam existir.
//   node scripts/qa-heroi-v4.mjs [arquivo.glb]
import { NodeIO } from "@gltf-transform/core";
import { ALL_EXTENSIONS } from "@gltf-transform/extensions";

const file = process.argv[2] ?? "public/models/heroi_v4.glb";
const REQUIRED = ["rc.idle", "base.walk", "base.run", "base.hit", "base.hit2", "base.death", "base.death2",
  "kn.attack.sword.1", "kn.attack.sword.2", "kn.attack.sword.3", "kn.skill.heavy", "kn.stance", "kn.block"];
const FINGERS = ["fingers_02_l", "fingers_02_r", "index_02_l", "index_02_r"];
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const doc = await io.read(file);
const fails = [];
const anims = new Map(doc.getRoot().listAnimations().map((a) => [a.getName(), a]));
for (const n of REQUIRED) if (!anims.has(n)) fails.push(`clipe faltando no GLB: ${n}`);
for (const [name, a] of anims) {
  if (!REQUIRED.includes(name)) continue;
  for (const c of a.listChannels()) {
    const node = c.getTargetNode()?.getName();
    if (!FINGERS.includes(node) || c.getTargetPath() !== "rotation") continue;
    const out = c.getSampler().getOutput().getArray();
    let least = 180;
    for (let i = 0; i < out.length; i += 4) least = Math.min(least, (2 * Math.acos(Math.min(1, Math.abs(out[i + 3]))) * 180) / Math.PI);
    if (least < 50) fails.push(`${name}: ${node} sem dobra no GLB (${least.toFixed(0)}°): mão aberta`);
  }
}
if (fails.length) {
  for (const f of fails) console.log("QA GLB FALHOU", f);
  process.exit(1);
}
console.log(`QA GLB OK (${REQUIRED.length} clipes, dedos fechados)`);
