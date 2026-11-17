#!/bin/sh
# Todos os prints da cidade: sem e com ?remaster=1, de dia e de noite (JPG 1280x720 em docs/arte/remaster-cidade/).
cd "$(dirname "$0")/../../.."
export BASE=http://localhost:5198 OUT=docs/arte/remaster-cidade/ PW_E2E=C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/
P=praca,portao,mercado,taverna,igreja,ferreiro,muralha,casas
node tools/art/remaster-cidade/shots.mjs jogo-sem-dia "time=0.4" $P
node tools/art/remaster-cidade/shots.mjs jogo-rm-dia "time=0.4&remaster=1" $P
node tools/art/remaster-cidade/shots.mjs jogo-sem-noite "time=0.85" $P
node tools/art/remaster-cidade/shots.mjs jogo-rm-noite "time=0.85&remaster=1" $P
