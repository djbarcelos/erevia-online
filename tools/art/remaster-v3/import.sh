#!/bin/sh
# GLB cru -> client/public/models (LOD com poda das ilhas, textura 1024, KTX2, manifesto) + máscara de repintura.
set -e
cd "$(dirname "$0")/../../../client"
M=../assets-src/meshy
node scripts/import-asset.mjs $M/knight_v3_raw.glb personagem knight_b --lod-prune --resize=1024
node scripts/make-remaster-mask.mjs $M/knight_v3_raw.glb public/models/knight_b.mask.png
node scripts/import-asset.mjs $M/skeleton_v3_raw.glb monstro skeleton_warrior_b --lod-prune --resize=1024
