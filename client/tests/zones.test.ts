// Testes dos mapas por zona (portão entre mapas), sem navegador: `npm run test:unit`.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { buildRegions } from "../src/ui/regions.ts";
import { worldBounds, worldUrl, type WorldData } from "../src/world/world-data.ts";

const here = import.meta.dirname;

test("cada mapa tem o seu world.json (o principal continua em /world.json)", () => {
  assert.equal(worldUrl(""), "/world.json");
  assert.equal(worldUrl("starter"), "/world.json");
  assert.equal(worldUrl("teste"), "/worlds/teste.json");
});

test("a Clareira de Testes: limites, praça, kits e o mesmo arquivo do servidor", () => {
  const raw = readFileSync(`${here}/../public/worlds/teste.json`, "utf8");
  const w = JSON.parse(raw) as WorldData;
  assert.deepEqual(worldBounds(w), { minX: -60, maxX: 60, minZ: -60, maxZ: 60 });
  assert.deepEqual(w.kits, ["/models/env-forest.glb"]);
  assert.ok(w.safeRadius > 0 && w.wall.apothem < w.safeRadius);
  // Nada colide na praça (onde ficam o portão e a chegada).
  assert.ok(w.colliders.every((c) => Math.hypot(c.x, c.z) - c.r > 5));
  assert.equal(readFileSync(`${here}/../../server/internal/world/maps/teste.json`, "utf8"), raw);
});

test("regiões: o mapa principal mantém os nomes; os outros usam o nome do mapa", () => {
  const zones = [{ name: "Esqueleto Lacaio", rMin: 18, rMax: 52, levelMin: 1, levelMax: 4 }] as never[];
  assert.equal(buildRegions(zones, 40).regions[1].name, "Arredores");
  const other = buildRegions(zones, 9, "Clareira de Testes").regions;
  assert.equal(other[0].name, "Praça");
  assert.equal(other[1].name, "Clareira de Testes");
});
