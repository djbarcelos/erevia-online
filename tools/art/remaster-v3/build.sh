#!/bin/sh
# Remonta os GLB crus (assets-src/meshy/{knight,skeleton}_v3_raw.glb) a partir das tasks do Meshy.
# Pastas curtas = ultimos 8 caracteres do id da task (veja assets-src/meshy/tasks.jsonl).
set -e
cd "$(dirname "$0")/../../../client"
M=../assets-src/meshy
KEEP=--keep=Idle,Right_Hand_Sword_Slash,Dead,Hit_Reaction_1,Sword_Shout
node scripts/build-remaster-v3.mjs $M/rigging-b2fcb175/rigged_character_glb_url-Character_output.glb $M/animations-2abfd80f/animation_glb_url-merged_animations.glb,$M/animations-f1b0633e/animation_glb_url-merged_animations.glb $M/rigging-b2fcb175/walking_glb_url-Animation_Walking_withSkin.glb $M/rigging-b2fcb175/running_glb_url-Animation_Running_withSkin.glb KB_ $M/knight_v3_raw.glb $KEEP
node scripts/build-remaster-v3.mjs $M/rigging-d026e681/rigged_character_glb_url-Character_output.glb $M/animations-6d0d243f/animation_glb_url-merged_animations.glb,$M/animations-5b4041a6/animation_glb_url-merged_animations.glb $M/rigging-d026e681/walking_glb_url-Animation_Walking_withSkin.glb $M/rigging-d026e681/running_glb_url-Animation_Running_withSkin.glb SW_ $M/skeleton_v3_raw.glb $KEEP --slotscale=0.36
