// Peças anexadas que mudam a silhueta (tunnel#96). Conceito 2D (text-to-image, nano-banana, 3 créditos) e
// imagem para 3D (smart topology, 15). Gera assets-src/meshy/req/piece-<id>.json:
//   MESHY_BUDGET=400 node tools/art/meshy.mjs create text-to-image --est 3 --json-file assets-src/meshy/req/piece-<id>.json
import { mkdirSync, writeFileSync } from 'node:fs';
const style = 'hand-painted stylized fantasy MMORPG armor piece concept, vibrant saturated colors, bright clean painterly look, soft painted shading with dark outline feel, chunky readable silhouette with slightly exaggerated proportions, a single object shown whole, centered, no person, no mannequin, no stand, plain neutral light gray background, no text';
export const PIECES = {
  pauldron_epic: ['One single large shoulder pauldron armor piece seen from the front, blackened steel with a tall curved gold-edged flange and three gold spikes along the top, red gem in the center, epic quality, wide dramatic silhouette. ', '1:1'],
  pauldron_legend: ['One single huge shoulder pauldron armor piece seen from the front, radiant polished golden steel with a tall winged flare and sweeping gold feathers, glowing cyan gem in the center, legendary quality, very wide dramatic silhouette. ', '1:1'],
  cape_rare: ['A knee-length cloak cape seen from the back hanging straight and flat, deep cobalt blue cloth with silver trim along the hem and a silver heraldic crest in the middle, rare quality. ', '9:16'],
  cape_epic: ['A long floor-length cloak cape seen from the back hanging straight and flat, deep crimson cloth with ornate gold trim along the hem and a large gold emblem of a winged crest in the middle, with a high pointed collar, epic quality. ', '9:16'],
  cape_legend: ['A very long flowing royal cloak cape seen from the back hanging straight and flat, brilliant white cloth with wide golden embroidered borders, a huge golden sunburst emblem in the middle and glowing cyan runes along the hem, legendary quality. ', '9:16'],
  helm_knight_rare: ['A knight great helm seen from the front, polished silver steel with cobalt blue enamel stripes, an open face wide opening, a tall silver crest fin on top and small side wings, rare quality. ', '1:1'],
  helm_knight_epic: ['A knight great helm seen from the front, blackened steel with ornate gold filigree, open face wide opening, a tall gold crest topped with a crimson plume and winged cheek guards, epic quality, striking silhouette. ', '1:1'],
  hat_mage_epic: ['A very tall pointed wizard hat with a wide curved brim seen from the front, deep purple and black cloth with gold embroidered runes, a glowing violet gem on the band and a gold star at the tip, epic quality. ', '1:1'],
  hood_rogue_epic: ['A pointed rogue hood with a long hanging tail seen from the front, dark blackened leather and dark crimson cloth with gold buckles and stitched gold trim, a shadowed hollow face opening, epic quality. ', '1:1'],
  helm_barbarian_epic: ['A barbarian horned helmet seen from the front, blackened iron with riveted bands, two huge curved ivory horns sweeping outward and up, gold rune engravings and a fur trim, open face opening, epic quality, wide striking silhouette. ', '1:1'],
};
mkdirSync('assets-src/meshy/req', { recursive: true });
for (const [k, [v, ar]] of Object.entries(PIECES)) writeFileSync(`assets-src/meshy/req/piece-${k}.json`, JSON.stringify({ ai_model: 'nano-banana', prompt: v + style, aspect_ratio: ar }));
