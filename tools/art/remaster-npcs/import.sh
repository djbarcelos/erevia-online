#!/bin/sh
# GLB cru dos NPCs -> client/public/models (LOD, textura, KTX2, manifesto). Rode build.mjs antes.
# Praça: textura 1024 px (os NPCs mais vistos, carregam com o jogo). Contadores de história, que ficam longe
# da cidade e carregam sob demanda: 512 px (UASTC de 1024 pesa ~1,9 MB por modelo, de 512 ~0,7 MB).
set -e
cd "$(dirname "$0")/../../../client"
M=../assets-src/meshy
for n in livia bruno teodoro otto ysolde edrian severino iselda cassio; do
  node scripts/import-asset.mjs $M/npc_${n}_raw.glb personagem npc_${n}_b --resize=1024
done
for n in guarda anselmo marga darcio maelis gaspar rovena; do
  node scripts/import-asset.mjs $M/npc_${n}_raw.glb personagem npc_${n}_b --resize=512
done
# Bertrand (Cavaleiro do Bastião) é o guarda repintado: máscara do tabardo (R) para a cor do Bastião
node scripts/make-remaster-mask.mjs $M/npc_guarda_raw.glb public/models/npc_guarda_b.mask.png --cloth=blue
