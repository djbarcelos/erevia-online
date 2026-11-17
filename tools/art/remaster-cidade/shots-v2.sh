#!/bin/sh
# Rodada 2: AO por vértice (ligado e desligado), de dia e de noite.
cd "$(dirname "$0")/../../.."
export BASE=http://localhost:5198 OUT=docs/arte/remaster-cidade/ PW_E2E=C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/
P=praca,portao,mercado,taverna,igreja,muralha,casas
node tools/art/remaster-cidade/shots.mjs jogo2-ao-dia "time=0.4&remaster=1" $P
node tools/art/remaster-cidade/shots.mjs jogo2-sem-ao-dia "time=0.4&remaster=1&ao=0" $P
node tools/art/remaster-cidade/shots.mjs jogo2-ao-noite "time=0.85&remaster=1" praca,muralha,igreja
node tools/art/remaster-cidade/shots.mjs jogo2-sem-ao-noite "time=0.85&remaster=1&ao=0" praca,muralha,igreja
