// Configuração da montagem dos monstros do remaster (tunnel#98): de onde vem cada clipe.
// Pastas = assets-src/meshy/<endpoint>-<últimos 8 do id da task> (veja tasks.jsonl).
// Clipes de OUTRO personagem (os do esqueleto guerreiro, já pagos na v3) são religados pelo nome do osso
// (todos usam o template mixamo) com `hipscale` = altura do rig / altura do rig de origem (1,7 m).
const SW1 = 'animations-6d0d243f/animation_glb_url-merged_animations.glb'; // Idle, golpe de espada, Hit_Reaction, Dead
const SW2 = 'animations-5b4041a6/animation_glb_url-merged_animations.glb'; // Hit_Reaction_1, Sword_Shout
const rigOf = (d) => ({
  rig: `rigging-${d}/rigged_character_glb_url-Character_output.glb`,
  walk: { src: `rigging-${d}/walking_glb_url-Animation_Walking_withSkin.glb`, as: 'Walk', hipscale: 1 },
  run: { src: `rigging-${d}/running_glb_url-Animation_Running_withSkin.glb`, as: 'Run', hipscale: 1 },
});
/** Clipes comuns do esqueleto guerreiro (parado, dano, morte, grito). */
const common = [
  { src: SW1, pick: 'Idle', as: 'Idle', hipscale: 'auto' },
  { src: SW2, pick: 'Hit_Reaction_1', as: 'Hit', hipscale: 'auto' },
  { src: SW1, pick: 'Dead', as: 'Death', hipscale: 'auto' },
  { src: SW2, pick: 'Sword_Shout', as: 'Raise', hipscale: 'auto' },
];
const mk = (d, o) => {
  const r = rigOf(d);
  return { rig: r.rig, walkOwn: r.walk.src, walkRef: 'rigging-d026e681/walking_glb_url-Animation_Walking_withSkin.glb', ...o, clips: [...(o.ownClips ? [] : common), ...o.clips, o.walkClip ?? r.walk, r.run].filter((c) => !(o.skipCommon ?? []).includes(c.as)) };
};
export const MODELS = {
  minion: mk('25dc7dbe', { prefix: 'MN_', out: 'skeleton_minion_raw.glb', slotscale: 0.3,
    clips: [{ src: 'animations-1e83a97d/animation_glb_url-Animation_Charged_Axe_Chop_withSkin.glb', as: 'Attack', hipscale: 1, from: 2.9, to: 4.3 }] }),
  rogue: mk('8be2c598', { prefix: 'RG_', out: 'skeleton_rogue_raw.glb', slotscale: 0.3,
    clips: [
      { src: 'animations-a7765151/animation_glb_url-merged_animations.glb', pick: 'Double_Combo_Attack', as: 'Attack', hipscale: 1, to: 1.3 },
      { src: 'animations-a7765151/animation_glb_url-merged_animations.glb', pick: 'Thrust_Slash', as: 'Skill', hipscale: 1, to: 1.4 },
    ] }),
  mage: mk('5e58954e', { prefix: 'MG_', out: 'skeleton_mage_raw.glb', slotscale: 0.4,
    clips: [
      { src: 'animations-f4e71ccf/animation_glb_url-merged_animations.glb', pick: 'mage_soell_cast', as: 'Attack', hipscale: 1 },
      { src: 'animations-f4e71ccf/animation_glb_url-merged_animations.glb', pick: 'Charged_Spell_Cast', as: 'Cast', hipscale: 1, to: 1.7 },
    ] }),
  golem: mk('e8dba804', { prefix: 'GL_', out: 'stone_golem_raw.glb', slotscale: 0.4, skipCommon: ['Raise'],
    walkClip: { src: 'animations-46952595/animation_glb_url-merged_animations.glb', pick: 'Slow_Orc_Walk', as: 'Walk', hipscale: 1 },
    clips: [
      { src: 'animations-46952595/animation_glb_url-merged_animations.glb', pick: 'Heavy_Hammer_Swing', as: 'Attack', hipscale: 1 },
      { src: 'animations-46952595/animation_glb_url-merged_animations.glb', pick: 'Charged_Ground_Slam', as: 'Slam', hipscale: 1 },
      { src: 'animations-46952595/animation_glb_url-merged_animations.glb', pick: 'Chest_Pound_Taunt', as: 'Roar', hipscale: 1, to: 2.2 },
    ] }),
  // Chefes: todas as ações são do próprio rig (pagas), sem religar as do esqueleto guerreiro.
  bonelord: mk('ef411e57', { prefix: 'BL_', out: 'bone_lord_raw.glb', slotscale: 0.4, ownClips: true,
    clips: [
      { src: 'animations-2645a86c/animation_glb_url-merged_animations.glb', pick: 'Idle', as: 'Idle', hipscale: 1 },
      { src: 'animations-2645a86c/animation_glb_url-merged_animations.glb', pick: 'Hit_Reaction_1', as: 'Hit', hipscale: 1 },
      { src: 'animations-2645a86c/animation_glb_url-merged_animations.glb', pick: 'Dead', as: 'Death', hipscale: 1 },
      { src: 'animations-2645a86c/animation_glb_url-merged_animations.glb', pick: 'Sword_Shout', as: 'Raise', hipscale: 1, to: 3.2 },
      { src: 'animations-6be94979/animation_glb_url-merged_animations.glb', pick: 'Sword_Judgment', as: 'Attack', hipscale: 1, to: 1.9 },
      { src: 'animations-6be94979/animation_glb_url-merged_animations.glb', pick: 'Charged_Ground_Slam', as: 'Slam', hipscale: 1, to: 2.4 },
    ] }),
  ozrhal: mk('d2870503', { prefix: 'OZ_', out: 'ozrhal_raw.glb', slotscale: 0.4, ownClips: true,
    clips: [
      { src: 'animations-fdeee824/animation_glb_url-merged_animations.glb', pick: 'Idle', as: 'Idle', hipscale: 1 },
      { src: 'animations-fdeee824/animation_glb_url-merged_animations.glb', pick: 'Hit_Reaction_1', as: 'Hit', hipscale: 1 },
      { src: 'animations-fdeee824/animation_glb_url-merged_animations.glb', pick: 'Dead', as: 'Death', hipscale: 1 },
      { src: 'animations-fdeee824/animation_glb_url-merged_animations.glb', pick: 'mage_soell_cast', as: 'Attack', hipscale: 1 },
      { src: 'animations-fdeee824/animation_glb_url-merged_animations.glb', pick: 'Zombie_Scream', as: 'Roar', hipscale: 1 },
      { src: 'animations-fdeee824/animation_glb_url-merged_animations.glb', pick: 'Angry_Ground_Stomp', as: 'Stomp', hipscale: 1 },
    ] }),
};
export const META = {};
