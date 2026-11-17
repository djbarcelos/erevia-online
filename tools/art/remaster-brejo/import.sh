#!/bin/sh
# Peças normalizadas (node client/scripts/build-brejo-kit.mjs) -> client/public/models/nature_<peça>_b.glb
# (LOD1 automático, textura 512 px, KTX2, manifesto). Os juncos são cartões com recorte: textura de 256 px.
set -e
cd "$(dirname "$0")/../../../client"
for f in ../assets-src/meshy/norm-brejo/*.glb; do
  n=$(basename "$f" .glb)
  case "$n" in
    reeds) node scripts/import-asset.mjs "$f" cenario "nature_${n}_b" --resize=256 --force ;;
    *) node scripts/import-asset.mjs "$f" cenario "nature_${n}_b" --resize=512 --force ;;
  esac
done
