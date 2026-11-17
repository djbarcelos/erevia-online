// Gera assets-src/familias/<id>_retex.json (corpo do retexture do Meshy) a partir da malha estática de prep.py.
// uso: node tools/art/familias-monstro/prompts.mjs <id>   depois:
//   node tools/art/meshy.mjs create retexture --est 10 --name <id> --json-file assets-src/familias/<id>_retex.json
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

export const STYLE = 'hand-painted stylized fantasy MMORPG look, vibrant saturated colors, clean painterly shading, no baked lighting';

export const DEFS = {
  wolf: 'gray timber wolf with thick two-tone fur: slate gray back and sides, cream white belly and muzzle, darker ears and tail tip, glowing amber eyes, black nose, visible painted fur strokes',
  spider: 'giant moss spider: dark olive-green and brown banded carapace with patches of green moss and tiny mushrooms on the back, a cluster of glowing pale yellow eyes, striped legs with lighter joints, pale fangs',
  slime: 'cute glossy jelly slime monster, one smooth uniform lime green body with a soft gradient from light yellow-green on top to deeper green at the bottom, no patterns or stripes, two round black eyes with white shine on the front face, wet translucent shine',
  frog: 'swamp toad: warty olive green and brown mottled skin with dark spots, mustard yellow belly and throat, big orange eyes with black slit pupils, wide mouth line',
};

if (process.argv[1] && import.meta.filename === resolve(process.argv[1])) {
  const id = process.argv[2];
  const dir = resolve(import.meta.dirname, '../../../assets-src/familias');
  const glb = readFileSync(resolve(dir, `${id}_static.glb`));
  const body = {
    model_url: `data:application/octet-stream;base64,${glb.toString('base64')}`,
    text_style_prompt: `${DEFS[id]}, ${STYLE}`,
    enable_original_uv: true,
    enable_pbr: false,
    ai_model: 'meshy-6',
  };
  writeFileSync(resolve(dir, `${id}_retex.json`), JSON.stringify(body));
  console.log(`${id}: ${(glb.length / 1024).toFixed(0)} KB, ${body.text_style_prompt}`);
}
