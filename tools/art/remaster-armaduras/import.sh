#!/bin/sh
# Peças anexadas (ombreiras, capas, elmos): GLB do Meshy -> client/public/models/armor (LOD1, textura 1024, KTX2, manifesto).
# Texturas de armadura por faixa: tools/art/remaster-armaduras/tiers.sh (make-armor-tiers.mjs).
set -e
cd "$(dirname "$0")/../../../client"
M=../assets-src/meshy
imp() { node scripts/import-asset.mjs $M/image-to-3d-$2/glb-model.glb armadura armor/$1 --resize=512; }
imp pauldron_epic 349bda75
imp pauldron_legend 8e776dbd
imp cape_rare 9d07efa8
imp cape_epic 703a3603
imp cape_legend bc03ff2a
imp helm_knight_rare a853bfe5
imp helm_knight_epic 986f562e
imp hat_mage_epic 528931e4
imp hood_rogue_epic 31ec17d7
imp helm_barbarian_epic 7382de89
