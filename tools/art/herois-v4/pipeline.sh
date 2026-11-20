#!/bin/zsh
# Heróis v4: refaz tudo a partir do corpo rigado (assets-src/herois-v4/corpo/corpo.blend, saída do build_rig.py)
# até o GLB do jogo. Ordem: LODs -> mapa de normais -> animações base (fórmula, só de reserva) -> locomoção e
# parado do Meshy -> reações do Meshy -> camada do Cavaleiro -> exportação -> encaixes -> importação no cliente.
#
#     tools/art/herois-v4/pipeline.sh            (na raiz do repositório)
#
# O corpo espelhado vem antes: mirror_left.py (corpo-m2k-bracos.glb -> corpo-m2k-espelho.glb) e build_rig.py.
set -e
cd "$(dirname "$0")/../../.."
B=${BLENDER:-/Applications/Blender.app/Contents/MacOS/Blender}
T=tools/art/herois-v4
C=assets-src/herois-v4/corpo
A=assets-src/herois-v4/anim
M=assets-src/meshy
LOCO=$M/animations-665c9148/animation_glb_url-merged_animations.glb   # Run_02, walking_2
IDLE=$M/animations-1e7c7120/animation_glb_url-merged_animations.glb   # Idle_3
REAC=$M/animations-cd7e1774/animation_glb_url-merged_animations.glb   # dano e morte
run() { echo "== $1"; shift; "$@" 2>&1 | grep -E "triângulos|CLIPE|IMPACTO|referência|assado|EXPORTADO|SALVO|Error|Traceback" | cut -c1-140 || true; }

run "LODs"        $B --background --factory-startup $C/corpo.blend --python $T/make_lods.py -- $C/corpo-lod.blend $C/work/corpo-lod.glb
run "normais"     $B --background --factory-startup $C/corpo-lod.blend --python $T/bake_normal.py -- $C/corpo-normal.png 2048
run "base"        $B --background --factory-startup $C/corpo-lod.blend --python $T/anim_base.py -- $A/base.blend $A/clips-base.json
run "locomoção"   $B --background --factory-startup $A/base.blend --python $T/retarget_meshy.py -- $LOCO $A/clips-base.json base.run=Run_02 base.walk=walking_2
run "parado"      $B --background --factory-startup $A/base.blend --python $T/retarget_meshy.py -- $IDLE $A/clips-base.json rc.idle=Idle_3
run "reações"     $B --background --factory-startup $A/base.blend --python $T/retarget_meshy.py -- $REAC $A/clips-base.json base.hit=Hit_Reaction:once base.hit2=Face_Punch_Reaction:once base.death=dying_backwards:once base.death2=Knock_Down:once
run "Cavaleiro"   $B --background --factory-startup $A/base.blend --python $T/anim_knight.py -- $A/clips-kn.json
KNC=$M/animations-96e54f6c/animation_glb_url-merged_animations.glb   # combate do Cavaleiro (Meshy)
run "Cavaleiro (Meshy)" $B --background --factory-startup $A/base.blend --python $T/retarget_meshy.py -- $KNC $A/clips-kn.json kn.attack.sword.1=Right_Hand_Sword_Slash:once kn.attack.sword.2=Left_Slash:once kn.attack.sword.3=Shield_Push_Left:once kn.skill.heavy=Charged_Upward_Slash:once kn.stance=Combat_Stance kn.block=Sword_Parry:once
run "pulso do escudo" $B --background --factory-startup $A/base.blend --python $T/lock_wrist.py -- rc.idle 0
run "impactos"    $B --background --factory-startup $A/base.blend --python $T/impacts.py -- $A/clips-kn.json kn.attack.sword.1=r kn.attack.sword.2=r kn.attack.sword.3=l kn.skill.heavy=r
run "exportar"    $B --background --factory-startup $A/base.blend --python $T/export_v4.py -- assets-src/herois-v4/heroi_v4_raw.glb client/src/world/hero-fists-v4.ts
node $T/slots-glb.mjs assets-src/herois-v4/heroi_v4_raw.glb | tail -1
cd client && node scripts/import-asset.mjs ../assets-src/herois-v4/heroi_v4_raw.glb personagem heroi_v4 --resize=1024 2>&1 | grep -E "→|FORA|triângulos"
