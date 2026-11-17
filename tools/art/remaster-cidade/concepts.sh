#!/bin/sh
# Gera os conceitos 2D em paralelo (9 créditos cada). Uso: sh concepts.sh peça1 peça2 ...
cd "$(dirname "$0")/../../.."
for p in "$@"; do
  MESHY_BUDGET=600 node tools/art/meshy.mjs create text-to-image --est 9 --name "$p" --json-file assets-src/meshy/prompts/$p.json > assets-src/meshy/log-concept-$p.txt 2>&1 &
done
wait
