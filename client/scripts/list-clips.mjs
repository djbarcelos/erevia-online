// Lista os clipes de cada GLB de animação (pasta animations-*) de uma worktree: node list-clips.mjs <assets-src/meshy>
import { NodeIO } from '@gltf-transform/core';
import { readdirSync, existsSync } from 'node:fs';
const dir = process.argv[2];
const io = new NodeIO();
for (const d of readdirSync(dir).filter((x) => x.startsWith('animations-'))) {
  const f = `${dir}/${d}/animation_glb_url-merged_animations.glb`;
  if (!existsSync(f)) continue;
  const doc = await io.read(f);
  console.log(d, doc.getRoot().listAnimations().map((a) => `${a.getName()}(${Math.max(...a.listSamplers().map((s) => s.getInput().getMax([])[0])).toFixed(1)}s)`).join(' '));
}
