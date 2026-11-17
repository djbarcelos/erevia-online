#!/bin/sh
# GLB cru (build.mjs) -> client/public/models (LOD, textura 1024, KTX2, manifesto).
set -e
cd "$(dirname "$0")/../../../client"
M=../assets-src/familias
for id in wolf spider slime frog; do
  max=3990; [ $id = frog ] && max=2800
  node scripts/import-asset.mjs $M/${id}_raw.glb monstro ${id}_b --lod-prune --resize=1024 --max-tris=$max "$@"
done
