#!/bin/sh
# Peças normalizadas (node client/scripts/build-nature-kit.mjs) -> client/public/models/nature_<nome>_b.glb
# (LOD1 automático, textura 512 px, KTX2, manifesto). As texturas do chão: client/scripts/make-seamless.mjs.
# A montanha (malha em ilhas) só emagrece no LOD com a poda; o mato (cartões com recorte) usa textura de 256 px.
set -e
cd "$(dirname "$0")/../../../client"
for f in ../assets-src/meshy/norm/*.glb; do
  n=$(basename "$f" .glb)
  case "$n" in
    cliff) node scripts/import-asset.mjs "$f" cenario "nature_${n}_b" --resize=512 --force --lod-prune ;;
    grass_tuft) node scripts/import-asset.mjs "$f" cenario "nature_${n}_b" --resize=256 --force ;;
    *) node scripts/import-asset.mjs "$f" cenario "nature_${n}_b" --resize=512 --force ;;
  esac
done
