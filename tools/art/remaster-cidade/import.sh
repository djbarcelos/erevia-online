#!/bin/sh
# GLB normalizados (assets-src/meshy/town) -> client/public/models: textura reduzida, KTX2 (ETC1S), orçamento, manifesto.
# Sem LOD: as peças da cidade são thin instances (uma malha por tipo), e o LOD do Babylon é por malha.
# ETC1S (e não UASTC): 14 peças em UASTC de 1024 px pesariam ~20 MB; em ETC1S ficam em ~5 MB e, vistas de
# longe (a câmera do jogo fica a ~18 m), a perda do pintado não aparece. Adereços pequenos: 512 px.
set -e
cd "$(dirname "$0")/../../../client"
node scripts/build-remaster-town.mjs
for p in tavern church blacksmith market home_a home_b tower wall gate; do
  node scripts/import-asset.mjs ../assets-src/meshy/town/town_${p}_b.glb cenario town_${p}_b --no-lod --resize=1024 --etc1s "$@"
done
for p in stall cart bench lamp sign tomb crystal pillar barrel crate banner corner; do
  node scripts/import-asset.mjs ../assets-src/meshy/town/town_${p}_b.glb cenario town_${p}_b --no-lod --resize=512 --etc1s "$@"
done
