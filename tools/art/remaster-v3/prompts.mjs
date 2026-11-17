// Prompts do conceito 2D (text-to-image, nano-banana-pro) do cavaleiro e do esqueleto do remaster v3.
// Gera assets-src/meshy/prompts/{k1,k2,s1,s2}.json para: node tools/art/meshy.mjs create text-to-image --est 9 --json-file <arquivo>
import { writeFileSync } from 'node:fs';
const style = 'hand-painted stylized heroic fantasy game character concept, vibrant saturated colors, bright clean painterly look like a high quality fantasy MMORPG illustration, soft painted shading, clear readable silhouette, heroic stylized proportions about 6.5 heads tall (not realistic, not chibi), slightly enlarged hands, feet and shoulders, full body front view character sheet, strict A-pose with arms slightly away from the body and legs slightly apart, standing straight upright, relaxed straight knees, plain neutral light gray background, no text, no weapon in hands, no shield';
const knights = [
  'Heroic knight in polished steel plate armor with large rounded pauldrons, OPEN-FACE helmet showing a young confident human face with brown hair, flat-topped helmet with small crest, saturated crimson red tabard with gold trim, short red cape, brown leather belt and boots, gold accents. ',
  'Heroic knight with a friendly determined face fully visible, open visor helmet with a red plume, bright silver plate armor with gold edges, vivid red surcoat with a simple cross emblem, short crimson cape, large rounded shoulder guards, leather gauntlets and boots. ',
];
const skel = [
  'Skeleton warrior undead, bright bone-white skull and bones with warm ivory tones, wearing rusty orange-brown half plate armor on the chest, one shoulder and the hips, tattered loincloth, glowing faint amber eyes, bony fingers and feet slightly enlarged, menacing but stylized and slightly charming. ',
  'Skeleton warrior, clean pale ivory bones with visible ribs and spine, rusted iron breastplate and pauldron with orange rust, small tattered dark red cloth belt, empty eye sockets with a faint warm glow, stylized exaggerated shoulders, no helmet. ',
];
const all = { k1: knights[0], k2: knights[1], s1: skel[0], s2: skel[1] };
for (const [k, v] of Object.entries(all)) writeFileSync(`assets-src/meshy/prompts/${k}.json`, JSON.stringify({ ai_model: 'nano-banana-pro', prompt: v + style, aspect_ratio: '3:4' }));
