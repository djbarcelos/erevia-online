#!/bin/sh
# Remonta os GLB crus dos 4 heróis a partir das tasks do Meshy (pastas = últimos 8 caracteres do id; ver assets-src/meshy/tasks.jsonl).
# rig + clipes pagos + andar/correr grátis + encaixes handslot.* -> assets-src/meshy/<nome>_raw.glb
set -e
cd "$(dirname "$0")/../../../client"
M=../assets-src/meshy
b() { # rig anims(vírgula) prefixo saida rename [extra]
  node scripts/build-remaster-v3.mjs $M/rigging-$1/rigged_character_glb_url-Character_output.glb $2 $M/rigging-$1/walking_glb_url-Animation_Walking_withSkin.glb $M/rigging-$1/running_glb_url-Animation_Running_withSkin.glb $3 $M/$4_raw.glb --rename=$5 $6
}
A() { echo $M/animations-$1/animation_glb_url-merged_animations.glb; }
COMMON=Hit_Reaction_1:Hit,Dead:Death,Wave_One_Hand:Wave,Sit_Cross_Legged_on_Floor:Sit,FunnyDancing_01:Dance
# Parado próprio de cada classe (Idle) e 2 pausas (break1, break2) tocadas de vez em quando; o Idle genérico (0) saiu.
# cavaleiro: corte em arco (Charged Slash) no golpe; a estocada (Right Hand Sword Slash) vira a habilidade
b 685843ce $(A 1aacc99c),$(A 7aa3b3ce),$(A 95fd7cf7) KB_ knight_bare Charged_Slash:Attack,Right_Hand_Sword_Slash:Skill,Sword_Shout:Raise,Short_Breathe_and_Look_Around:Idle,Combat_Stance:Break1,Lower_Weapon_Look_Raise:Break2,$COMMON "--keep=Short_Breathe_and_Look_Around,Combat_Stance,Lower_Weapon_Look_Raise,Charged_Slash,Right_Hand_Sword_Slash,Sword_Shout,Hit_Reaction_1,Dead,Wave_One_Hand,Sit_Cross_Legged_on_Floor,FunnyDancing_01 --trim=Sword_Shout:0:3.2"
# bárbaro: Charged Slash no golpe (o Charged Axe Chop de 7,7 s foi descartado), martelo na habilidade; respira pesado
b 4cc67c13 $(A 35267a77),$(A 26a1cecd),$(A 8ad8e8c0) BB_ barbarian Charged_Slash:Attack,Heavy_Hammer_Swing:Skill,Chest_Pound_Taunt:Raise,Long_Breathe_and_Look_Around:Idle,Axe_Breathe_and_Look_Around:Break1,Catching_Breath:Break2,$COMMON "--keep=Long_Breathe_and_Look_Around,Axe_Breathe_and_Look_Around,Catching_Breath,Charged_Slash,Heavy_Hammer_Swing,Chest_Pound_Taunt,Hit_Reaction_1,Dead,Wave_One_Hand,Sit_Cross_Legged_on_Floor,FunnyDancing_01"
# mago (sem chapéu; o chapéu é a peça Hat) e ladino
b 08cdef45 $(A 221c4049),$(A c81d372d) MB_ mage_bare mage_soell_cast:Attack,Charged_Spell_Cast:Skill,Idle_02:Idle,Talk_with_Hands_Open:Break1,Look_Around_Dumbfounded:Break2,$COMMON "--keep=Idle_02,Talk_with_Hands_Open,Look_Around_Dumbfounded,mage_soell_cast,Charged_Spell_Cast,Hit_Reaction_1,Dead,Wave_One_Hand,Sit_Cross_Legged_on_Floor,FunnyDancing_01"
b dbef244a $(A e7d606a8),$(A 428461ed) RB_ rogue Double_Combo_Attack:Attack,Thrust_Slash:Skill,Motivational_Cheer:Raise,Idle_4:Idle,Idle_03:Break1,Short_Breathe_and_Look_Around:Break2,$COMMON "--keep=Idle_4,Idle_03,Short_Breathe_and_Look_Around,Double_Combo_Attack,Thrust_Slash,Motivational_Cheer,Hit_Reaction_1,Dead,Wave_One_Hand,Sit_Cross_Legged_on_Floor,FunnyDancing_01 --trim=Motivational_Cheer:0:3.2"
# os chapéus: o elmo do cavaleiro (Meshy, peça à parte) e o chapéu do mago (tudo acima da aba, do mago da primeira rodada)
node scripts/make-remaster-helm.mjs $M/image-to-3d-7ccfd48d/glb-model.glb $M/knight_bare_raw.glb $M/knight_hat_raw.glb --maxtris=600 --scale=0.49 --dy=-0.01 --dz=-0.012
node scripts/make-remaster-hat.mjs $M/mage_first_raw.glb $M/mage_bare_raw.glb $M/mage_hat_raw.glb --cuty=1.41 --scale=1.12 --dy=-0.005 --dz=-0.03 --maxtris=480
