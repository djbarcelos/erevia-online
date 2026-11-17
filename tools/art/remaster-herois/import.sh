#!/bin/sh
# GLB cru -> client/public/models (LOD, textura 1024, KTX2, manifesto) + máscaras de repintura dos 4 heróis.
set -e
cd "$(dirname "$0")/../../../client"
M=../assets-src/meshy
node scripts/import-asset.mjs $M/knight_hat_raw.glb personagem knight_b --resize=1024 --no-lod2='^Hat$'
node scripts/import-asset.mjs $M/barbarian_raw.glb personagem barbarian_b --resize=1024
node scripts/import-asset.mjs $M/mage_raw.glb personagem mage_b --resize=1024
node scripts/import-asset.mjs $M/rogue_raw.glb personagem rogue_b --resize=1024
node scripts/make-remaster-mask.mjs $M/knight_hat_raw.glb public/models/knight_b.mask.png --cloth=red
node scripts/make-remaster-mask.mjs $M/barbarian_raw.glb public/models/barbarian_b.mask.png --cloth=brown --skin=all
node scripts/make-remaster-mask.mjs $M/mage_raw.glb public/models/mage_b.mask.png --cloth=blue
node scripts/make-remaster-mask.mjs $M/rogue_raw.glb public/models/rogue_b.mask.png --cloth=green
