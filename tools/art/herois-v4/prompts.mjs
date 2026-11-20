// Heróis v4, Fase 1: conceitos 2D (docs/arte/herois-v4). Gera os JSON das tasks do Meshy em
// assets-src/meshy/prompts/hv4-*.json. Ordem de uso:
//   node tools/art/herois-v4/prompts.mjs                      (gera os JSON; os que pedem referência
//                                                             usam @caminhos de assets-src/herois-v4/ref/)
//   1) corpo-a, corpo-b                text-to-image (9 cada); escolher um -> ref/corpo.png
//   2) rostos, recruta, classe-<id>    image-to-image com ref/corpo.png (9 cada) -> ref/classe-<id>.png
//   3) duplas-<n>                      image-to-image com ref/corpo.png + as folhas das classes envolvidas
//   MESHY_BUDGET=<teto> node tools/art/meshy.mjs create <endpoint> --est 9 --name hv4-<x> --json-file assets-src/meshy/prompts/hv4-<x>.json
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

const MODEL = 'nano-banana-pro';
const REF = 'assets-src/herois-v4/ref';

export const STYLE =
  'Hand-painted dark fantasy action-RPG character concept art, gritty and weathered: worn steel with scratches, cracked leather, ' +
  'heavy wool stained with mud, desaturated earthy base colors with one saturated accent color per character, visible painterly ' +
  'brush strokes, strong readable silhouettes, adult heroic proportions about 7 heads tall, slightly oversized hands and feet, broad ' +
  'shoulders, full body from head to feet, even soft studio lighting, all figures at the same scale with feet on one ground line, ' +
  'evenly spaced and not overlapping. No text, no letters, no numbers, no labels, no frames, no UI.';

const SAME_BODY = 'Use exactly the same base human body, proportions, height and painting style as the reference image.';

// --- 1. corpo
const BODY =
  'Character turnaround model sheet for a game base mesh: one row of six figures. Left three: a man in front view, side view facing ' +
  'right, and back view. Right three: a woman of the same build family in front view, side view and back view. A-pose: arms straight ' +
  'down at 45 degrees from the body, palms facing the thighs, legs slightly apart, neutral face looking forward. Plain base look: simple ' +
  'dark linen shorts (and a plain cloth chest wrap on the woman), barefoot, short cropped hair, no armor, no jewelry, no weapons.';

// --- 2. rostos e cabelos
const FACES =
  `${SAME_BODY} Head and hair concept sheet, front three-quarter view, grid of 4 rows by 4 columns, each head on its own neck stump. ` +
  'Row 1: three male faces (a rugged veteran with a broken nose and a scar over one eyebrow; a young man with stubble and a sharp jaw; ' +
  'an older man with a short grey beard and deep wrinkles) and one empty cell. Row 2: three female faces (a stern warrior with a scar ' +
  'on the cheek; a young woman with freckles and a strong brow; an older woman with sharp cheekbones and grey streaks) and one empty cell. ' +
  'Row 3: four male hairstyles on a neutral male head (shaved sides with a short top knot; shoulder-length messy hair; long braided hair; ' +
  'close buzz cut). Row 4: four female hairstyles on a neutral female head (tight crown braid; short undercut; long single braid over the ' +
  'shoulder; shoulder-length hair tied back). Empty cells are plain background.';

// --- 3. kits (descrições das folhas de classe e de dupla)
export const KITS = {
  recruit: 'a recruit with no class yet: plain linen shirt with a worn leather vest, simple leather shoulder pads, linen trousers, soft leather boots, rope belt, holding a wooden practice sword loosely; no emblem, muted tan and brown',
  knight: 'a knight: chain mail with a short blue-grey surcoat, rounded steel shoulder plates, open steel helmet with a nose guard, short wool cape, plate leg guards, a large round shield on the left forearm and an arming sword; steel grey with a muted blue accent, plain shield emblem',
  bastion: 'a Bastion, the fortress knight: heavy plate armor with straps, tall stepped shoulder plates shaped like castle battlements, flat-topped bucket helmet with a cross-shaped visor, long tabard with a tower emblem, greaves, a huge tower shield and a sword; cold steel with a pale blue #8fb4ff accent',
  herald: 'a Paladin: bright plate armor with a long white tabard bearing a rising sun, shoulder plates shaped like folded wings, winged helmet with a sun crest, a small reliquary hanging on the chest, sword and kite shield; pale steel and cream with a gold #ffd86a accent',
  barbarian: 'a barbarian: bare muscular chest crossed by leather straps, bear pelt over the shoulders, wolf fur cape on the back, leather trousers, fur boots, bare head, a heavy one-handed axe; weathered leather and skin with a dark red accent, claw-mark emblem painted on the skin',
  bloodrager: 'a Berserker: bare scarred chest with crossed leather straps, a single huge horned skull shoulder piece on one shoulder and a bear pelt on the other, cloth wraps on the arms, fur cape, a massive two-handed axe; dark leather, bone and dried blood with a blood red #e85050 accent, dripping claw emblem',
  warlord: 'a Warlord: bronze cuirass over a padded jerkin, bronze shoulder plates with leather fringe, bronze helmet with short curved horns, fur cape, a one-handed axe and a round shield on the back; bronze and leather with an old gold #e0a848 accent, emblem of crossed axes under a crown',
  mage: 'a mage: long wool robe with a high belt, stiff high wool collar, tall pointed wide-brimmed hat, short cape, a grimoire chained to the belt, a tall wooden staff; dark purple wool and brown leather with an amber accent, four-pointed star emblem',
  elementalist: 'an Elementalist: long wool robe whose hem is scorched on one side and frosted on the other, a high collar with a big cluster of jagged ice crystals and burning fire shards floating above both shoulders, rising higher than the head, tall pointed hood, grimoire on the belt, a staff topped with a fire stone and an ice stone; dark wool with orange #ff9a3c and ice blue #6cc4f0 accents, half-fire half-ice circle emblem',
  occultist: 'an Arcanist: dark tattered robe hung with amulets, a deep heavy hood that droops forward over the face like a beak, thick chains with padlocks hanging from both shoulders all the way down to the floor, grimoire on the belt, a gnarled staff; near-black wool with violet #b46ae0 and sickly green #6ad06a accents, abyss eye emblem',
  rogue: 'a rogue: fitted leather jerkin, simple leather shoulder pads, leather hood with a cloth mask over the mouth, short cape, two daggers; dark leather and grey-green cloth with a crimson accent, dagger emblem',
  cutthroat: 'an Assassin: tight dark leather jerkin, a single asymmetric leather shoulder guard on the dagger arm only, hood with a cloth mask, a cape cut into loose ragged strips, poison flasks on the belt, two curved daggers held reversed; near-black leather with a crimson #e0607a accent, crescent moon and dagger emblem',
  hunter: 'a Ranger: hunting leathers with light plates and a sash, a short mantle of dark feathers over the shoulders, a cape made of leaves and netting, a quiver on the back, a longbow; brown leather and moss green cloth with a green #8cc850 accent, wolf head emblem',
};

// As 28 duplas: [1ª, 2ª, nome, ordem A (1ª -> 2ª), ordem B (2ª -> 1ª)]
export const DUOS = [
  ['bastion', 'herald', 'Crusader', 'Bastion plate and battlement shoulders with a long white mantle bearing a sun and a winged helmet', 'Paladin sun plate and wing shoulders with the flat bucket helmet and a tower shield'],
  ['bastion', 'bloodrager', 'Colossus', 'Bastion plate with a horned skull shoulder piece and blood splashed on the tabard', 'bare-chested berserker with a bear pelt, a single battlement steel shoulder plate and a tower shield strapped to the back'],
  ['bastion', 'warlord', 'Marshal', 'Bastion plate and battlement shoulders with a war banner mounted on the back', 'bronze warlord cuirass with the flat bucket helmet and a blue tower tabard'],
  ['bastion', 'elementalist', 'Rune Guardian', 'Bastion plate with a big collar of ice crystals and embers rising above the shoulders and a shield carved with glowing fire and frost runes', 'elementalist robe with huge stepped battlement shoulder plates of glowing runic stone rising above the head'],
  ['bastion', 'occultist', 'Veil Warden', 'Bastion plate with massive chains wrapped around the body and hanging from the shoulders to the floor, a giant padlock on the chest and a padlocked tower shield', 'tattered arcanist robe with a tall flat bucket helmet with long chains hanging from it to the waist, and a small shield'],
  ['bastion', 'cutthroat', 'Shadow Sentinel', 'dark chain mail and plate with a ragged cape whose strips turn into smoke', 'assassin leathers with a single battlement steel shoulder plate and a small shield'],
  ['bastion', 'hunter', 'Border Warden', 'Bastion plate with a huge mantle of black feathers rising like folded wings above the shoulders, and a quiver', 'ranger leathers with a tall bucket helmet crowned by a long feather plume, and a tower shield on the back'],
  ['herald', 'bloodrager', 'Zealot', 'Paladin sun plate with a huge horned skull on one shoulder whose horns rise above the head, and a blood-stained tabard', 'bare-chested berserker with large gilded wings spreading wide from the shoulders and a reliquary on the chest'],
  ['herald', 'warlord', 'Champion', 'Paladin sun plate with a sun war banner on the back', 'bronze warlord cuirass with a long white mantle bearing a sun'],
  ['herald', 'elementalist', 'Theurge', 'Paladin sun plate with a halo of floating ice crystals and flames above the shoulders and head', 'elementalist robe with large golden wing shoulder plates spreading wide and a golden sun disc behind the head'],
  ['herald', 'occultist', 'Exorcist', 'Paladin sun plate with a one-eyed leather mask and long chains hanging from the mask and shoulders to the floor', 'tattered arcanist robe with large wing shoulder plates and a big reliquary lantern hanging from a tall staff'],
  ['herald', 'cutthroat', 'Penitent', 'light paladin plate with a tall pointed white hood and a long cape of white ragged strips trailing behind', 'assassin leather jerkin dyed white and stained, a deep white hood, one large wing-shaped steel plate on one shoulder only, two daggers'],
  ['herald', 'hunter', 'Witch Hunter', 'Paladin sun plate with a wide-brimmed leather hat and a crossbow', 'ranger leathers with a white sun tabard and a reliquary'],
  ['bloodrager', 'warlord', 'Clan Chief', 'bare-chested berserker with a tall war banner made of bones on the back', 'bronze warlord cuirass and a bronze helmet with huge curved horns, plus a horned skull shoulder piece'],
  ['bloodrager', 'elementalist', 'Firebrand', 'bare-chested berserker with glowing embers in cracked skin and an axe wrapped in flames', 'scorched elementalist robe with a horned skull shoulder piece'],
  ['bloodrager', 'occultist', 'Possessed', 'berserker leathers and axe with chains and padlocks and a one-eyed leather mask', 'arcanist robe torn by claws and stained with blood, with a horned skull shoulder piece'],
  ['bloodrager', 'cutthroat', 'Butcher', 'bare-chested berserker with an executioner hood and two cleavers', 'assassin leathers with a horned skull shoulder piece and chains at the waist'],
  ['bloodrager', 'hunter', 'Predator', 'bare-chested berserker wearing a wolf head hood', 'ranger leathers with a bear pelt and a skull shoulder piece'],
  ['warlord', 'elementalist', 'Storm Lord', 'bronze warlord cuirass with a floating fire stone and a floating ice stone behind the shoulders', 'elementalist robe with the horned bronze helmet and a storm banner'],
  ['warlord', 'occultist', 'Tyrant', 'bronze warlord cuirass with an iron crown over a hood and chains', 'tattered arcanist robe with an iron crown and a scepter'],
  ['warlord', 'cutthroat', 'Mercenary', 'light bronze cuirass with a wide-brimmed feathered hat and big pouches of coins on the belt', 'assassin leathers with large bronze fringed shoulder plates and a tall crested bronze helmet'],
  ['warlord', 'hunter', 'Pathfinder', 'bronze warlord cuirass with a wolf head hood and a short banner', 'ranger leathers with the horned bronze helmet'],
  ['elementalist', 'occultist', 'Archmage', 'elementalist robe with large fire and ice stones orbiting above the head and chains of glowing runes', 'tattered arcanist robe with a huge collar of ice crystals and embers rising above the head'],
  ['elementalist', 'cutthroat', 'Elemental Blade', 'short elementalist robe with an oversized fire dagger and ice dagger crossed on the back like wings, and a ragged strip cape', 'assassin leathers with a tall spiky collar of ice crystals and embers framing the head'],
  ['elementalist', 'hunter', 'Rune Archer', 'elementalist robe with a tall quiver of glowing runic arrows rising above the head and a feather mantle', 'ranger leathers with an ice crystal collar and a large glowing runic longbow on the back'],
  ['occultist', 'cutthroat', 'Warlock', 'tattered arcanist robe with a cape whose strips turn into smoke and a glowing violet dagger', 'assassin leathers with chains across the shoulders and a very tall pointed hood bent forward'],
  ['occultist', 'hunter', 'Plaguebearer', 'arcanist robe with a long beaked plague mask and poison flasks', 'ranger leathers with chains and a large clay jar on the back leaking green mist'],
  ['cutthroat', 'hunter', 'Stalker', 'assassin leathers with a wolf head hood and a short bow', 'ranger leathers with a cape whose strips turn into smoke'],
];

const CLASS_OF = { bastion: 'knight', herald: 'knight', bloodrager: 'barbarian', warlord: 'barbarian', elementalist: 'mage', occultist: 'mage', cutthroat: 'rogue', hunter: 'rogue' };
const CLASSES = { knight: ['bastion', 'herald'], barbarian: ['bloodrager', 'warlord'], mage: ['elementalist', 'occultist'], rogue: ['cutthroat', 'hunter'] };

export const SHEETS = [];
const t2i = (name, prompt) => SHEETS.push({ name, endpoint: 'text-to-image', body: { ai_model: MODEL, prompt, aspect_ratio: '16:9' } });
const i2i = (name, refs, prompt, aspect = '16:9', transparent = true) =>
  SHEETS.push({ name, endpoint: 'image-to-image', refs, body: { ai_model: MODEL, prompt, aspect_ratio: aspect, remove_background: transparent, reference_image_urls: refs.map((r) => `@${REF}/${r}.png`) } });

t2i('hv4-corpo-a', `${BODY} Build: rugged, muscular warrior build, weathered skin with a few old scars. ${STYLE} Plain flat neutral grey background.`);
t2i('hv4-corpo-b', `${BODY} Build: lean, athletic and wiry, long limbs, weathered skin. ${STYLE} Plain flat neutral grey background.`);
i2i('hv4-rostos', ['corpo'], `${FACES} ${STYLE}`, '1:1', false);
i2i('hv4-recruta', ['corpo'], `${SAME_BODY} One row of four figures: the man front view, the man back view, the woman front view, the woman back view, all wearing the same outfit: ${KITS.recruit}. ${STYLE}`);
for (const [cls, [a, b]] of Object.entries(CLASSES)) {
  i2i(`hv4-classe-${cls}`, ['corpo'], `${SAME_BODY} One row of six figures, three outfits each shown in front view then back view. ` +
    `Figures 1 and 2 (a man): ${KITS[cls]}. Figures 3 and 4 (a woman): ${KITS[a]}. Figures 5 and 6 (a man): ${KITS[b]}. ` +
    `The three outfits must be clearly different silhouettes even as black shapes 85 pixels tall, each with an oversized signature piece breaking the outline, getting heavier and more ornate from left to right. ${STYLE}`);
}
for (let i = 0; i < DUOS.length; i += 3) {
  const group = DUOS.slice(i, i + 3);
  const refs = ['corpo', ...new Set(group.flatMap(([a, b]) => [`classe-${CLASS_OF[a]}`, `classe-${CLASS_OF[b]}`]))].slice(0, 5);
  const figs = group.map(([a, b, name, A, B], j) =>
    `Figures ${j * 2 + 1} and ${j * 2 + 2}, the ${name}: figure ${j * 2 + 1} is a ${j % 2 ? 'woman' : 'man'} wearing ${A}; ` +
    `figure ${j * 2 + 2} is a ${j % 2 ? 'man' : 'woman'} wearing ${B}.`).join(' ');
  i2i(`hv4-duplas-${i / 3 + 1}`, refs, `${SAME_BODY} The other reference sheets show the class outfits these hybrids are made from. ` +
    `One row of ${group.length * 2} figures in front view, in pairs: each pair is the same hybrid class seen from its two possible orders, ` +
    `the first figure dominated by the first class and the second by the second class. ${figs} Each figure must have one unique, oversized piece on the head, shoulders or cape that breaks the outline and stays readable as a black silhouette only 85 pixels tall. ${STYLE}`);
}

if (process.argv[1] && import.meta.filename === resolve(process.argv[1])) {
  const dir = resolve(import.meta.dirname, '../../../assets-src/meshy/prompts');
  mkdirSync(dir, { recursive: true });
  for (const s of SHEETS) {
    writeFileSync(resolve(dir, `${s.name}.json`), JSON.stringify(s.body));
    console.log(s.name.padEnd(18), s.endpoint.padEnd(15), (s.refs || []).join(',').padEnd(40), s.body.prompt.length, 'car.');
  }
  console.log(SHEETS.length, 'folhas,', SHEETS.length * 9, 'créditos');
}
