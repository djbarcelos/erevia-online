#!/bin/sh
cd "$(dirname "$0")/../../.."
export BASE=http://localhost:5198 OUT=assets-src/shots/ PW_E2E=C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/
node tools/art/remaster-cidade/bench.mjs depois "bench=contadores&remaster=1" > assets-src/shots/bench-depois.txt 2>&1
sh tools/art/remaster-cidade/shots-all.sh > assets-src/shots/shots-all.log 2>&1
