// Copia cada resultado baixado para nomes de peça (a partir do tasks.jsonl):
//   assets-src/meshy/concepts/<peça>.png  e  assets-src/meshy/raw/<peça>.glb
import { copyFileSync, existsSync, mkdirSync, readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
const M = resolve(import.meta.dirname, '../../../assets-src/meshy');
mkdirSync(`${M}/concepts`, { recursive: true });
mkdirSync(`${M}/raw`, { recursive: true });
for (const l of readFileSync(`${M}/tasks.jsonl`, 'utf8').split(/\r?\n/).filter(Boolean)) {
  const r = JSON.parse(l);
  if (!r.name || !['text-to-image', 'image-to-3d'].includes(r.endpoint)) continue;
  const dir = `${M}/${r.endpoint}-${r.id.slice(-8)}`;
  if (!existsSync(dir)) continue;
  if (r.endpoint === 'text-to-image') {
    const f = readdirSync(dir).find((x) => x.endsWith('.png'));
    if (f) copyFileSync(`${dir}/${f}`, `${M}/concepts/${r.name}.png`);
  } else if (existsSync(`${dir}/glb-model.glb`)) copyFileSync(`${dir}/glb-model.glb`, `${M}/raw/${r.name}.glb`);
}
console.log(readdirSync(`${M}/raw`).join(' '));
