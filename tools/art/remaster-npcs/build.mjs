// Monta o GLB cru de cada NPC do remaster (assets-src/meshy/npc_<nome>_raw.glb): rig do Meshy + clipes.
// Os clipes são reaproveitados entre os rigs (todos são o esqueleto `mixamo`, os canais ligam pelo nome do osso):
// os já comprados para os heróis (assets-src/meshy/hero-clips/*.glb, cópia das animações da worktree remaster-herois)
// e os gestos do ofício comprados uma vez (task npc-gestos). Cada NPC tem: parado, duas pausas, o cumprimento
// (toca quando o jogador abre a conversa) e andar/correr (grátis, vêm no rig).
// Uso: node tools/art/remaster-npcs/build.mjs [nome ...]   (sem argumentos: todos)
import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../../..');
const M = resolve(root, 'assets-src/meshy');
const rigs = JSON.parse(readFileSync(`${M}/npc-rig-full.json`, 'utf8'));
// gestos do ofício: a task npc-gestos do tasks.jsonl (animações do rig da Lívia); a pasta é a que o meshy.mjs baixou
const gestos = `${M}/${process.env.GESTOS_DIR ?? 'animations-8adb57d7'}/animation_glb_url-merged_animations.glb`;
const H = (id) => `${M}/hero-clips/${id}.glb`;
// clipe do Meshy -> GLB que o contém
const SRC = {
  Idle: H('2abfd80f'),
  Idle_02: H('c81d372d'), Talk_with_Hands_Open: H('c81d372d'), Look_Around_Dumbfounded: H('c81d372d'),
  Idle_03: H('428461ed'), Idle_4: H('428461ed'), Short_Breathe_and_Look_Around: H('428461ed'),
  Long_Breathe_and_Look_Around: H('8ad8e8c0'), Catching_Breath: H('8ad8e8c0'),
  Lower_Weapon_Look_Raise: H('95fd7cf7'), Combat_Stance: H('95fd7cf7'),
  Charged_Spell_Cast: H('221c4049'), Wave_One_Hand: H('221c4049'),
  Heavy_Hammer_Swing: H('35267a77'),
  Motivational_Cheer: H('e7d606a8'),
};
for (const c of ['Checkout_Gesture', 'Scheming_Hand_Rub', 'Talk_Passionately', 'Collect_Object', 'Big_Wave_Hello', 'Torch_Look_Around', 'Formal_Bow', 'Hand_on_Hip_Gesture', 'Headache_Relief', 'Shrug']) SRC[c] = gestos;

// nome: [prefixo, parado, pausa 1 (olhar ou gesto do ofício), pausa 2, cumprimento]
const NPCS = {
  livia: ['LV_', 'Idle_02', 'Checkout_Gesture', 'Look_Around_Dumbfounded', 'Wave_One_Hand'],
  bruno: ['BR_', 'Long_Breathe_and_Look_Around', 'Heavy_Hammer_Swing', 'Catching_Breath', 'Wave_One_Hand'],
  teodoro: ['TD_', 'Idle', 'Scheming_Hand_Rub', 'Look_Around_Dumbfounded', 'Talk_with_Hands_Open'],
  otto: ['OT_', 'Short_Breathe_and_Look_Around', 'Lower_Weapon_Look_Raise', 'Catching_Breath', 'Formal_Bow'],
  ysolde: ['YS_', 'Idle_4', 'Talk_with_Hands_Open', 'Charged_Spell_Cast', 'Wave_One_Hand'],
  edrian: ['ED_', 'Short_Breathe_and_Look_Around', 'Charged_Spell_Cast', 'Formal_Bow', 'Talk_with_Hands_Open'],
  severino: ['SV_', 'Idle', 'Checkout_Gesture', 'Talk_Passionately', 'Big_Wave_Hello'],
  iselda: ['IS_', 'Idle_02', 'Collect_Object', 'Look_Around_Dumbfounded', 'Talk_with_Hands_Open'],
  cassio: ['CS_', 'Short_Breathe_and_Look_Around', 'Motivational_Cheer', 'Big_Wave_Hello', 'Wave_One_Hand'],
  guarda: ['GD_', 'Short_Breathe_and_Look_Around', 'Lower_Weapon_Look_Raise', 'Combat_Stance', 'Formal_Bow'],
  anselmo: ['AN_', 'Idle', 'Shrug', 'Catching_Breath', 'Wave_One_Hand'],
  marga: ['MG_', 'Idle_4', 'Headache_Relief', 'Look_Around_Dumbfounded', 'Talk_with_Hands_Open'],
  darcio: ['DC_', 'Short_Breathe_and_Look_Around', 'Torch_Look_Around', 'Look_Around_Dumbfounded', 'Wave_One_Hand'],
  maelis: ['ML_', 'Idle_02', 'Charged_Spell_Cast', 'Talk_with_Hands_Open', 'Wave_One_Hand'],
  gaspar: ['GS_', 'Long_Breathe_and_Look_Around', 'Formal_Bow', 'Catching_Breath', 'Wave_One_Hand'],
  rovena: ['RV_', 'Short_Breathe_and_Look_Around', 'Hand_on_Hip_Gesture', 'Lower_Weapon_Look_Raise', 'Talk_with_Hands_Open'],
};
const only = process.argv.slice(2);
for (const [n, [prefix, idle, b1, b2, greet]] of Object.entries(NPCS)) {
  if (only.length && !only.includes(n)) continue;
  const rig = `${M}/rigging-${rigs[n].slice(-8)}`;
  const clips = [idle, b1, b2, greet];
  const files = [...new Set(clips.map((c) => SRC[c]))];
  const trim = clips.includes('Motivational_Cheer') ? '--trim=Motivational_Cheer:0:3.2' : '';
  const args = [
    'scripts/build-remaster-v3.mjs', `${rig}/rigged_character_glb_url-Character_output.glb`, files.join(','),
    `${rig}/walking_glb_url-Animation_Walking_withSkin.glb`, `${rig}/running_glb_url-Animation_Running_withSkin.glb`,
    prefix, `${M}/npc_${n}_raw.glb`,
    `--rename=${idle}:Idle,${b1}:Break1,${b2}:Break2,${greet}:Greet`, `--keep=${clips.join(',')}`,
    ...(trim ? [trim] : []),
  ];
  console.log(execFileSync('node', args, { cwd: `${root}/client`, encoding: 'utf8' }).trim());
}
