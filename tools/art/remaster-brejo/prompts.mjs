// Prompts do conceito 2D do Brejo de Lamagris (tunnel#139): o Brejeiro (monstro humanoide) e quatro peças
// estáticas do cenário. Gera assets-src/meshy/prompts/<id>.json; depois (da raiz do repositório):
//   MESHY_BUDGET=200 node tools/art/meshy.mjs create text-to-image --est 3 --name <id> --json-file <abs>/assets-src/meshy/prompts/<id>.json
// (o Brejeiro usa nano-banana-pro, 9 créditos; as peças, nano-banana, 3). O passo seguinte é o image-to-3d (ver README do bioma).
import { mkdirSync, writeFileSync } from 'node:fs';
const prop = (s) =>
  `hand-painted stylized fantasy MMORPG environment prop concept, vibrant saturated colors, bright clean painterly look, soft painted shading, chunky readable silhouette, a single object shown whole, three-quarter view from slightly above, centered, plain neutral light gray background, no ground, no text, ${s}`;
const monster =
  'hand-painted stylized heroic fantasy game monster concept, vibrant saturated colors, bright clean painterly look like a high quality fantasy MMORPG illustration, soft painted shading, clear readable silhouette from far away, full body front view character in a neutral A-pose standing with arms slightly away from the body, plain neutral light gray background, no text, ';
const defs = {
  brejeiro: {
    ai_model: 'nano-banana-pro',
    aspect_ratio: '3:4',
    prompt:
      'Brejeiro, a swamp lizardman tribesman, humanoid frog-lizard with olive green scaly skin, a pale yellow belly, wide flat head with big round amber eyes and a wide mouth, a short thick tail, wearing only a ragged brown reed-woven loincloth and a necklace of small bones, holding a long wooden fishing spear with a bone tip in one hand, lean wiry body about 1.8 meters tall, ' +
      monster,
  },
  tronco: {
    ai_model: 'nano-banana',
    aspect_ratio: '16:9',
    prompt: prop('a large fallen rotten tree trunk lying on its side, mossy dark bark, glowing green mushrooms along the top, a few broken branches, hollow cut end, swamp style'),
  },
  juncos: {
    ai_model: 'nano-banana',
    aspect_ratio: '1:1',
    prompt: prop('a clump of tall swamp reeds and cattails, slender green stalks with brown cattail heads, a few silvery blades, fanned out like a bouquet, filling the picture'),
  },
  cabana: {
    ai_model: 'nano-banana',
    aspect_ratio: '4:3',
    prompt: prop("a small ferryman's hut on tall wooden stilts over water, weathered grey planks, a mossy thatched roof, a short wooden pier with a ladder, a glowing green lantern hanging at the door and a small brass boat bell, swamp style"),
  },
  barco: {
    ai_model: 'nano-banana',
    aspect_ratio: '16:9',
    prompt: prop('a small old wooden rowing boat seen from the side, weathered planks, a pair of oars resting inside, a coiled rope at the bow, a little green lantern at the stern, swamp style'),
  },
};
mkdirSync('assets-src/meshy/prompts', { recursive: true });
for (const [k, v] of Object.entries(defs)) writeFileSync(`assets-src/meshy/prompts/${k}.json`, JSON.stringify(v));
console.log(Object.keys(defs).join(' '));
