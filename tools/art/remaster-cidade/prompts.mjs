// Prompts do conceito 2D (text-to-image, nano-banana-pro) do kit da cidade (tunnel#100).
// Gera assets-src/meshy/prompts/<peça>.json para:
//   node tools/art/meshy.mjs create text-to-image --est 9 --name <peça> --json-file assets-src/meshy/prompts/<peça>.json
import { mkdirSync, writeFileSync } from 'node:fs';

const bld = 'hand-painted stylized heroic fantasy MMORPG game building concept, warm clean painterly look, soft painted shading with ambient occlusion painted into the crevices, chunky readable silhouette with slightly exaggerated proportions, light sandstone stone (#c9b79c) and dark brown timber (#7a5232) with deep red (#9e2b25) and gold (#d6b46e) accents, the whole object fully visible and centered, three-quarter front view from slightly above, a single isolated object with a flat bottom and no ground, no terrain, no grass, plain neutral light gray background, no text, no people, no characters';
const prp = 'hand-painted stylized heroic fantasy MMORPG game prop concept, warm clean painterly look, soft painted shading with ambient occlusion painted into the crevices, chunky readable silhouette, the whole object fully visible and centered, three-quarter front view from slightly above, a single isolated object with a flat bottom and no ground, plain neutral light gray background, no text, no people, no characters';

export const PIECES = {
  tavern: [bld, 'Two-storey fantasy tavern: ground floor of warm sandstone blocks, upper floor of cream plaster with dark brown timber framing, steep terracotta orange-red tiled roof with a stone chimney and one dormer window, a hanging wooden sign shaped like a mug (no text), warm glowing windows, a wooden double door under a small porch roof with a lantern. '],
  market: [bld, 'Wide one-storey fantasy market hall: open arched front with a long wooden counter, a big red and cream striped fabric awning with gold trim over the front, sandstone walls, a sloped warm brown wooden shingle roof with a small gold weathervane, crates of fruit and rolls of cloth on the counter. '],
  blacksmith: [bld, 'Fantasy blacksmith forge house: sturdy sandstone base with dark timber beams, a large stone chimney with a faint orange glow, a slate-blue shingle roof, an open lean-to forge on one side with an anvil, a hanging iron horseshoe sign, a wooden door. '],
  church: [bld, 'Fantasy stone chapel with a tall square bell tower on one side, steep slate-blue roofs, a round rose window, a big wooden arched double door, stone buttresses, light sandstone walls, a golden sun ornament on the roof peak, a red banner with gold trim above the door. '],
  home_a: [bld, 'Cozy small fantasy cottage: one storey plus attic, cream plaster walls with a dark timber frame, an orange-red tiled gable roof, a stone chimney, a flower box under the window, a wooden door with a small step. '],
  home_b: [bld, 'Tall narrow fantasy town house, two and a half storeys, the upper floor jutting out over a sandstone ground floor, cream plaster and dark timber, teal-green slate gable roof, a small wooden balcony, a stone chimney. '],
  tower: [bld, 'Round fantasy corner watch tower of light sandstone with a crenellated stone top ring, a tall conical deep-red tiled roof with a gold finial and a small red pennant, arrow-slit windows, a small wooden balcony, a wooden door at the base. '],
  wall: [bld, 'A straight section of a fantasy medieval city wall, light sandstone blocks with painted mortar lines, a walkway with a crenellated parapet on top, a slightly wider battered base, both short ends cut flat and vertical so that it continues into the next section, plain wall with no gate and no tower, seen from the front slightly from above. '],
  gate: [bld, 'A fantasy city gate wall section: one big arched gateway opening in the middle (the opening is about 45 percent of the total width and almost as tall as the wall) with a raised wooden portcullis, light sandstone blocks, crenellated parapet, a small wooden gatehouse hut on top, two red banners with gold trim hanging at the sides, both short ends cut flat and vertical so that it continues into the next wall section. '],
  stall: [prp, 'Fantasy market stall: four wooden posts, a red and cream striped fabric canopy, a wooden counter with colorful goods (apples, bread, folded cloth), a small hanging lantern. '],
  cart: [prp, 'Rustic wooden merchant hand cart with two large spoked wheels and two shafts, loaded with sacks and small crates, partly covered by a red cloth. '],
  bench: [prp, 'Sturdy wooden town bench with carved scroll ends and dark brown planks, resting on two stone slabs. '],
  lamp: [prp, 'Fantasy iron street lamp post on a small stone base: ornate wrought iron curl, a glowing amber glass lantern at the top, a tiny red pennant. '],
  tomb: [prp, 'Fantasy graveyard tombstone: a weathered pale bone-grey stone headstone with a rounded top and a carved skull relief, leaning slightly, a few small rust-orange (#b04a2e) iron nails and dry grass tufts at the base, a small mound of dry yellowish soil. '],
  crystal: [prp, 'A cluster of three glowing amber (#f0a640) crystals growing out of a small mossy grey rock base (#7d7f86, moss #5f7a44), one tall central crystal and two smaller ones, carved glowing runes on the rock, magical ancient look. '],
  pillar: [prp, 'A broken ancient cracked-earth stone pillar in warm brown (#8c5a3c) with orange (#d8873a) glowing cracks, jagged broken top, a few fallen stone chunks at the base, hostile desert ruins look. '],
  sign: [prp, 'Wooden signpost with two arrow-shaped direction boards without any text, on a stone base with a small lantern hook. '],
};

if (process.argv[1]?.endsWith('prompts.mjs')) {
  mkdirSync('assets-src/meshy/prompts', { recursive: true });
  for (const [k, [style, subject]] of Object.entries(PIECES)) {
    writeFileSync(`assets-src/meshy/prompts/${k}.json`, JSON.stringify({ ai_model: 'nano-banana-pro', prompt: subject + style, aspect_ratio: '4:3' }));
  }
  console.log(Object.keys(PIECES).join(' '));
}
