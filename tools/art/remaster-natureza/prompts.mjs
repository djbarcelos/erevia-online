// Prompts do conceito 2D (text-to-image) do kit de natureza do remaster. Gera assets-src/meshy/prompts/<id>.json.
// Uso: node tools/art/remaster-natureza/prompts.mjs && node tools/art/meshy.mjs create text-to-image --est 3 --name <id> --json-file assets-src/meshy/prompts/<id>.json
import { mkdirSync, writeFileSync } from 'node:fs';
const style = 'hand-painted stylized fantasy game asset, vibrant saturated colors, bright clean painterly look like a high quality fantasy MMORPG illustration, soft painted shading, simple chunky readable silhouette, low-poly friendly shapes, single isolated object centered, three-quarter view, plain neutral light gray background, no ground, no shadow, no text';
export const items = {
  oak: 'Lush stout oak tree with a thick short trunk and a big rounded full canopy of bright green leaf clumps, a few visible roots. ',
  pine: 'Tall fir pine tree with a straight brown trunk and dense layered conical dark green boughs, slight snow-free. ',
  dead: 'Dead twisted leafless tree, gnarled gray-brown trunk with a few crooked bare branches, spooky but stylized. ',
  vale: 'Ancient bent tree with a pale silver-lilac bark trunk and a small canopy of teal and violet leaves, magical valley tree, slender and elegant. ',
  bush: 'Round leafy green shrub bush with clumps of leaves and a few small red berries. ',
  tuft: 'Small tuft of tall wild grass with a few little flowers (yellow and white), one clump. ',
  rock_s: 'Small smooth gray stones cluster of three pebbles, chunky faceted shapes with moss on top. ',
  rock_m: 'Medium faceted gray boulder with brown-gray planes and a bit of green moss on top, chunky rounded. ',
  rock_l: 'Large rugged rock outcrop with tall faceted gray-brown stone planes and cracks, a few tufts of grass on top. ',
  cliff: 'Big rocky mountain cliff peak with steep layered gray-brown rock faces, a few green grass patches and small pine trees on ledges, wide base. ',
  grave: 'Old crooked stone gravestone with a rounded top and a cracked cross, moss, and a small pile of bones at the base. ',
  crystal: 'Cluster of tall glowing blue-violet crystals growing from a small base of dark stone, stylized magical crystal formation. ',
};
mkdirSync('assets-src/meshy/prompts', { recursive: true });
for (const [k, v] of Object.entries(items)) writeFileSync(`assets-src/meshy/prompts/${k}.json`, JSON.stringify({ ai_model: 'nano-banana', prompt: v + style, aspect_ratio: '1:1' }));
