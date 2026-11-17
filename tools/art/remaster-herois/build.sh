#!/bin/sh
# Remonta os GLB crus dos 4 heróis a partir das tasks do Meshy (pastas = últimos 8 caracteres do id; ver assets-src/meshy/tasks.jsonl).
# rig + clipes pagos + andar/correr grátis + encaixes handslot.* -> assets-src/meshy/<nome>_raw.glb
set -e
cd "$(dirname "$0")/../../../client"
M=../assets-src/meshy
b() { # rig anim prefixo saida rename [extra]
  node scripts/build-remaster-v3.mjs $M/rigging-$1/rigged_character_glb_url-Character_output.glb $M/animations-$2/animation_glb_url-merged_animations.glb $M/rigging-$1/walking_glb_url-Animation_Walking_withSkin.glb $M/rigging-$1/running_glb_url-Animation_Running_withSkin.glb $3 $M/$4_raw.glb --rename=$5 $6
}
COMMON=Hit_Reaction_1:Hit,Dead:Death,Wave_One_Hand:Wave,Sit_Cross_Legged_on_Floor:Sit,FunnyDancing_01:Dance
b 685843ce 1aacc99c KB_ knight_bare   Right_Hand_Sword_Slash:Attack,Sword_Shout:Raise,$COMMON --trim=Sword_Shout:0:3.2
b 4cc67c13 35267a77 BB_ barbarian     Charged_Axe_Chop:Attack,Heavy_Hammer_Swing:Skill,Chest_Pound_Taunt:Raise,$COMMON --trim=Charged_Axe_Chop:2.5:4.2
b cb52138e f11db608 MB_ mage          mage_soell_cast:Attack,Charged_Spell_Cast:Skill,$COMMON 
b dbef244a e7d606a8 RB_ rogue         Double_Combo_Attack:Attack,Thrust_Slash:Skill,Motivational_Cheer:Raise,$COMMON --trim=Motivational_Cheer:0:3.2
# o elmo do cavaleiro (peça "Hat" separada, tirada do cavaleiro com elmo da v3)
node scripts/make-remaster-hat.mjs $M/knight_v3_raw.glb $M/knight_bare_raw.glb $M/knight_hat_raw.glb
