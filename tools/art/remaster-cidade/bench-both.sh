#!/bin/sh
# Contadores da Cidade (e das outras regiões) sem e com ?remaster=1, um depois do outro.
cd "$(dirname "$0")/../../.."
export BASE=http://localhost:5198 OUT=assets-src/shots/ PW_E2E=C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/
node tools/art/remaster-cidade/bench.mjs antes "bench=contadores" > assets-src/shots/bench-antes.txt 2>&1
node tools/art/remaster-cidade/bench.mjs depois "bench=contadores&remaster=1" > assets-src/shots/bench-depois.txt 2>&1
