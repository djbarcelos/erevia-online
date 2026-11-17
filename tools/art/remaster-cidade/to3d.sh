#!/bin/sh
# Imagem para 3D (smart-topology, meshy-t2.5, 15 créditos) em paralelo. Uso: sh to3d.sh peça:polycount ...
cd "$(dirname "$0")/../../.."
for pp in "$@"; do
  p=${pp%%:*}; n=${pp##*:}
  MESHY_BUDGET=600 node tools/art/meshy.mjs create image-to-3d --est 15 --name "$p" --json "{\"image_url\":\"@assets-src/meshy/concepts/$p.png\",\"ai_model\":\"meshy-t2.5\",\"model_type\":\"smart-topology\",\"target_polycount\":$n,\"should_texture\":true,\"texture_resolution\":\"2k\",\"target_formats\":[\"glb\"]}" > assets-src/meshy/log-3d-$p.txt 2>&1 &
done
wait
