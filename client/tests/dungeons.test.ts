// Testes do gerador de masmorras (scripts/dungeons/, scripts/lib/dungeon-gen.mjs) e do que o cliente lê delas,
// sem navegador: `npm run test:unit`.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { buildDungeon, DUNGEONS_DIR, MOB_CAP, BOSS_SUMMONS } from "../scripts/lib/dungeon-gen.mjs";
import { loadBiomes } from "../scripts/world-gen.mjs";
import { worldBounds, type WorldData } from "../src/world/world-data.ts";

const here = import.meta.dirname;
type Dungeon = {
  entry: { x: number; z: number };
  spawns: { m: string; x: number; z: number; l: number; n: number; f: number }[];
  stairs: { name: string; x: number; z: number; toX: number; toZ: number }[];
  floors: { minX: number; maxX: number; minZ: number; maxZ: number }[];
};
const build = async () => {
  const [biome] = await loadBiomes(DUNGEONS_DIR);
  return buildDungeon(biome);
};

test("o mapa das Catacumbas commitado (cliente e servidor) é o que o gerador faz", async () => {
  const { json } = await build();
  assert.equal(readFileSync(`${here}/../public/worlds/catacumbas.json`, "utf8"), json);
  assert.equal(readFileSync(`${here}/../../server/internal/world/maps/catacumbas.json`, "utf8"), json);
});

test("o gerador é determinístico", async () => {
  assert.equal((await build()).json, (await build()).json);
});

test("Catacumbas: 2 andares de 60 × 60 m, o kit próprio e a luz fixa de masmorra", async () => {
  const w = (await build()).world as WorldData & { dungeon: Dungeon };
  assert.equal(w.id, "catacumbas");
  assert.equal(w.indoor, true);
  assert.deepEqual(w.kits, ["/models/env-catacumbas.glb"]);
  assert.equal(w.dungeon.floors.length, 2);
  for (const f of w.dungeon.floors) assert.deepEqual([f.maxX - f.minX, f.maxZ - f.minZ], [60, 60]);
  const b = worldBounds(w);
  assert.deepEqual(w.dungeon.floors.map((f) => [f.minZ, f.maxZ]), [[-6, 54], [74, 134]]);
  assert.ok(b.minZ <= -6 && b.maxZ >= 134);
  // A entrada é a origem (a zona segura e o renascimento do servidor), dentro da zona segura.
  assert.deepEqual(w.dungeon.entry, { x: 0, z: 0 });
  assert.ok(w.safeRadius > 0);
  // Só peças do kit: cada tipo é um draw call, então poucos tipos.
  const kinds = new Set(w.objects.map((o) => o.k));
  assert.ok([...kinds].every((k) => k.startsWith("dg_")), `tipos fora do kit: ${[...kinds].filter((k) => !k.startsWith("dg_"))}`);
  assert.ok(kinds.size <= 12, `${kinds.size} tipos de peça`);
});

test("Catacumbas: ~30 monstros (no máximo 40 com as invocações), um chefe e 2 escadas ligadas", async () => {
  const { world, monsters } = await build();
  const d = (world as unknown as { dungeon: Dungeon }).dungeon;
  assert.ok(monsters >= 25 && monsters + BOSS_SUMMONS <= MOB_CAP, `${monsters} monstros`);
  assert.equal(d.spawns.filter((s) => s.m === "abade_ulric").length, 1);
  // Nível 12 a 16, o andar 2 mais forte que o 1, e cada monstro dentro do seu andar.
  for (const s of d.spawns) {
    assert.ok(s.l >= 12 && s.l <= 16, `${s.m} no nível ${s.l}`);
    const f = d.floors[s.f];
    assert.ok(s.x > f.minX && s.x < f.maxX && s.z > f.minZ && s.z < f.maxZ, `${s.m} fora do andar ${s.f + 1}`);
  }
  assert.equal(d.spawns.find((s) => s.m === "abade_ulric")!.l, 16);
  const roles = new Set(d.spawns.map((s) => s.m));
  for (const m of ["cata_minion", "cata_guard", "cata_acolyte", "cata_lurker"]) assert.ok(roles.has(m), `sem ${m}`);
  // Matilha: os lacaios nascem em grupos de 3.
  assert.ok(d.spawns.filter((s) => s.m === "cata_minion").every((s) => s.n === 3));
  // As escadas: uma de cada andar, e a chegada de uma fica no andar da outra.
  const [up, down] = d.stairs;
  assert.ok(up.z < 54 && down.z > 74 && up.toZ > 74 && down.toZ < 54);
});

test("Catacumbas: nenhum colisor na entrada (o grupo chega e renasce ali)", async () => {
  const { world } = await build();
  assert.ok(world.colliders.every((c: { x: number; z: number; r: number }) => Math.hypot(c.x, c.z) - c.r > 1.5));
});
