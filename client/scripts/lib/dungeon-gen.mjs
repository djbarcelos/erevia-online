// Monta o mapa de uma masmorra (scripts/dungeons/<id>.mjs) a partir do motor do mundo (world-gen.mjs). Usado por
// build-dungeons.mjs (grava os arquivos) e pelos testes (conferem que o que está commitado é o que o gerador faz).
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { buildWorld } from '../world-gen.mjs';

export const DUNGEONS_DIR = resolve(import.meta.dirname, '../dungeons');
export const KIT = '/models/env-catacumbas.glb';
/** Roadmap, seção 8: monstros por cópia (as invocações do chefe entram na conta). */
export const MOB_CAP = 40;
export const BOSS_SUMMONS = 4;

export const dungeonSizes = () => JSON.parse(readFileSync(resolve(DUNGEONS_DIR, 'kit-sizes.json'), 'utf8'));

/** O mapa da masmorra `biome` (como o loadBiomes devolve): { world, json, stats, monsters }. */
export async function buildDungeon(biome, sizes = dungeonSizes()) {
  const { world: w } = await buildWorld({ biomes: [biome], sizes });
  const { dungeon, stats, ...rest } = w.biomes[0];
  const monsters = dungeon.spawns.reduce((n, s) => n + s.n, 0);
  if (monsters + BOSS_SUMMONS > MOB_CAP) throw new Error(`${biome.id}: ${monsters} monstros + ${BOSS_SUMMONS} invocações passam do teto de ${MOB_CAP}`);
  const b = w.bounds;
  const world = {
    ...w,
    id: biome.id,
    size: Math.max(b.maxX - b.minX, b.maxZ - b.minZ),
    biomes: [rest],
    dungeon,
    kits: [KIT],
    indoor: true, // sem céu: luz fixa de cripta, chão plano de pedra
    roads: [],
  };
  return { world, json: JSON.stringify(world), stats, monsters };
}
