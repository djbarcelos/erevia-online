// Ícones de skill gerados no Meshy (tunnel#181): uma folha 5x5 por classe (text-to-image, nano-banana-pro, 9
// créditos cada), na ordem de leitura abaixo; as células que sobram ficam vazias (escuras). Depois:
//   node tools/art/icones-skill/prompts.mjs
//   node tools/art/meshy.mjs create text-to-image --est 9 --name icons-<classe> --json-file assets-src/meshy/prompts/icons-<classe>.json
//   cd client && node scripts/slice-skill-icons.mjs <folha.png> <classe>
import { mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

// [fx do servidor, o que o ícone mostra]
export const SHEETS = {
  knight: [
    ['shield_bash', 'a heater shield slamming forward with a white impact burst and stun stars'],
    ['fortress', 'a glowing golden stone castle tower with a protective dome'],
    ['k_iron', 'a steel breastplate with riveted iron plates shining'],
    ['k_taunt', 'an angry knight helmet shouting with red shock waves'],
    ['k_wall', 'a cracked stone wall glowing gold, holding firm'],
    ['k_fencing', 'two crossed elegant longswords with a sparkle on the edge'],
    ['k_spin', 'a sword spinning in a circle with a silver whirlwind trail'],
    ['k_charge', 'a knight rushing forward with a lance of light and speed lines'],
    ['k_judg', 'a giant radiant sword descending from the sky'],
    ['k_cry', 'a war horn blasting golden sound waves'],
    ['k_aura', 'a warm golden heart inside a ring of light'],
    ['k_banner', 'a blue and gold war banner planted in the ground'],
    ['k_bulwark', 'a huge tower shield hitting the ground with a shockwave ring'],
    ['k_horizon', 'a wide horizontal sword slash leaving a long white arc'],
    ['k_aegis', 'a translucent golden hexagon barrier dome with a crest'],
    ['kb_defy', 'a fortress shield with a roaring lion emblem and red challenge aura'],
    ['kb_vigor', 'a muscular arm in armor with a green life glow'],
    ['kb_last', 'a lone knight planting a sword amid a ring of golden light'],
    ['kh_dawn', 'a holy warhammer blazing with sunrise light'],
    ['kh_bless', 'golden wings around a raised sword, blessing rays'],
    ['kh_sun', 'a blinding rising sun exploding with white rays'],
  ],
  barbarian: [
    ['brutal_strike', 'a heavy axe smashing down with a red impact burst'],
    ['fury', 'a roaring barbarian face surrounded by red flames of rage'],
    ['b_heavy', 'a massive double-bit battle axe with a heavy glint'],
    ['b_quake', 'a fist punching the ground cracking it with rocks flying'],
    ['b_exec', 'a blood red axe blade over a skull'],
    ['b_thirst', 'a blood drop flowing into an open hand, crimson glow'],
    ['b_frenzy', 'two axes blurred by speed with red motion trails'],
    ['b_undying', 'a cracked skull wreathed in defiant orange fire'],
    ['b_hide', 'a thick brown fur pelt with tough leather patches'],
    ['b_roar', 'a bear head roaring with sound waves'],
    ['b_stomp', 'a giant fur boot stomping, dust shockwave'],
    ['b_bear', 'a glowing spectral bear spirit silhouette in green'],
    ['b_seismic', 'an axe splitting the earth with a glowing orange fissure'],
    ['b_whirl', 'a crimson whirlwind of blood and axes'],
    ['b_stone', 'skin turning into gray granite stone with cracks'],
    ['bb_red', 'a red crescent axe slash dripping crimson'],
    ['bb_blind', 'a furious red eye with blood veins'],
    ['bb_carnage', 'three bloody axe slashes crossing in a storm'],
    ['bw_cry', 'a war chief shouting with a horned helmet and golden waves'],
    ['bw_charge', 'a barbarian leaping forward with an axe, dust trail'],
    ['bw_banner', 'a tattered red war banner with a skull sigil'],
  ],
  mage: [
    ['fireball', 'a blazing fireball with a fiery tail'],
    ['ice_nova', 'an exploding ring of ice shards, frost burst'],
    ['m_ember', 'glowing embers and a small living flame'],
    ['m_rain', 'burning embers raining from dark clouds'],
    ['m_meteor', 'a huge flaming meteor falling from the sky'],
    ['m_lance', 'a sharp crystal ice spear flying'],
    ['m_armor', 'an icy crystal armor with frost mist'],
    ['m_prison', 'a frozen block of ice trapping a shadow'],
    ['m_mind', 'a glowing blue crystal brain with calm aura'],
    ['m_shield', 'a purple arcane rune shield bubble'],
    ['m_blink', 'a purple teleport flash with swirling particles'],
    ['m_over', 'a staff orb overflowing with crackling violet energy'],
    ['m_inferno', 'a raging column of fire engulfing the ground'],
    ['m_zero', 'a snowflake star freezing everything, white blue glow'],
    ['m_arcane', 'a violet arcane missile beam with runes'],
    ['me_lance', 'a spear made of pure fire'],
    ['me_focus', 'a floating orb split half fire half ice'],
    ['me_cata', 'a cataclysm of fire and ice clashing'],
    ['mo_rot', 'a green plague skull emitting dark miasma'],
    ['mo_drain', 'a dark hand pulling a blue wisp of will from a head'],
    ['mo_abyss', 'a black abyss eye sigil with purple glow'],
  ],
  rogue: [
    ['backstab', 'a dagger stabbing from the shadows with a red crit flash'],
    ['shadow_step', 'a hooded figure dashing with dark shadow trails'],
    ['r_weak', 'a target crosshair over a cracked armor weak point'],
    ['r_twin', 'two curved daggers crossing with silver slashes'],
    ['r_mark', 'a red skull mark target sigil'],
    ['r_dodge', 'a cloaked figure sidestepping a blade, afterimage'],
    ['r_smoke', 'a gray smoke bomb exploding in a cloud'],
    ['r_vanish', 'a hood fading into purple mist, invisible'],
    ['r_poison', 'a dagger dripping green poison'],
    ['r_flask', 'a glass flask of bubbling green toxin'],
    ['r_para', 'a paralyzed hand with yellow venom sparks'],
    ['r_plague', 'a rat skull in a sickly green cloud'],
    ['r_fatal', 'a dagger piercing a heart with crimson burst'],
    ['r_shadow', 'a shadow blade lunging out of darkness'],
    ['r_miasma', 'a swirling toxic green fog cloud'],
    ['rc_blades', 'three hidden blades flashing from a sleeve'],
    ['rc_instinct', 'a narrowed predator eye glowing yellow in darkness'],
    ['rc_sentence', 'a hooded executioner dagger raised, red moon'],
    ['rh_knife', 'a throwing knife spinning through the air'],
    ['rh_net', 'a hunting net with weights flying open'],
    ['rh_rain', 'many blades raining down from above'],
  ],
};

const STYLE =
  'Hand-painted stylized fantasy MMORPG skill icons, vibrant saturated colors, bold readable silhouette centered in each tile, ' +
  'soft painterly shading, each tile has its own dark rich gradient background matching its element, ' +
  'no text, no letters, no numbers, no frames, no borders inside the tiles.';

if (process.argv[1] && import.meta.filename === resolve(process.argv[1])) {
  const dir = resolve(import.meta.dirname, '../../../assets-src/meshy/prompts');
  mkdirSync(dir, { recursive: true });
  for (const [cls, list] of Object.entries(SHEETS)) {
    const cells = list.map(([, d], i) => `${i + 1}. ${d}`).join('; ');
    const prompt =
      `A sprite sheet: a perfect 5 by 5 grid of 25 equal square tiles separated by thin pure black lines, filling the whole square image. ` +
      `Tiles in reading order (left to right, top to bottom): ${cells}; tiles ${list.length + 1} to 25 are empty plain black. ${STYLE}`;
    writeFileSync(resolve(dir, `icons-${cls}.json`), JSON.stringify({ ai_model: 'nano-banana-pro', prompt, aspect_ratio: '1:1' }));
    console.log(cls, list.length, 'ícones,', prompt.length, 'caracteres');
  }
}
