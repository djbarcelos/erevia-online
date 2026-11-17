// Gera as masmorras (determinístico) a partir dos biomas de scripts/dungeons/ e grava, como o world.json:
//   client/public/worlds/<id>.json          → o cliente posiciona o cenário e pinta o chão
//   server/internal/world/maps/<id>.json    → o servidor monta a colisão, as escadas e os monstros (campo `dungeon`)
//
// Usa o mesmo motor do mundo (world-gen.mjs: place, collide, torch...), com o kit das masmorras (build-dungeon-kit.mjs).
// Cada arquivo de scripts/dungeons/<id>.mjs é UMA masmorra (um mapa por arquivo, id = nome do arquivo).
//
// Uso: node scripts/build-dungeons.mjs   (rode build-dungeon-kit.mjs antes, para os tamanhos)
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { BOSS_SUMMONS, buildDungeon, DUNGEONS_DIR, dungeonSizes } from './lib/dungeon-gen.mjs';
import { loadBiomes } from './world-gen.mjs';

const here = import.meta.dirname;
const sizes = dungeonSizes();

for (const biome of await loadBiomes(DUNGEONS_DIR)) {
  const { world, json, stats, monsters } = await buildDungeon(biome, sizes);
  for (const p of [resolve(here, `../public/worlds/${biome.id}.json`), resolve(here, `../../server/internal/world/maps/${biome.id}.json`)]) {
    mkdirSync(dirname(p), { recursive: true });
    writeFileSync(p, json);
  }
  const byKind = {};
  for (const o of world.objects) byKind[o.k] = (byKind[o.k] ?? 0) + 1;
  console.log(`${biome.id}.json: ${world.objects.length} objetos, ${world.colliders.length} colisores, ${world.torches.length} tochas, ${(json.length / 1024).toFixed(0)} KB`);
  console.log(Object.entries(byKind).map(([k, n]) => `${k}:${n}`).join(' '));
  console.log(`salas ${stats.rooms}, células ${stats.cells}, paredes ${stats.walls}; ${monsters} monstros (com o chefe, +${BOSS_SUMMONS} invocados)`);
}
