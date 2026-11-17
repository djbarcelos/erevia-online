// Testes dos mapas por zona (portão entre mapas), sem navegador: `npm run test:unit`.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { buildRegions, regionAt } from "../src/ui/regions.ts";
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

test("regiões: o Bosque Gris e as Terras Mortas (núcleo e extensão) são uma região cada, com os monstros dos discos (tunnel#137)", () => {
  const zones = [
    { name: "Esqueleto Guerreiro", rMin: 80, rMax: 108, levelMin: 6, levelMax: 12 },
    { name: "Esqueleto Mago", rMin: 116, rMax: 140, levelMin: 13, levelMax: 20 },
    // Os setores com dono (sem monstros): o noroeste dos anéis e o núcleo das Terras Mortas.
    { name: "Lobo Cinzento", rMin: 80, rMax: 212, levelMin: 7, levelMax: 15, angleMin: 100, angleMax: 170, region: "Bosque Gris" },
    { name: "Acólito do Ossário", rMin: 142, rMax: 212, levelMin: 24, levelMax: 28, angleMin: 15, angleMax: 75, region: "Terras Mortas" },
    // Os discos de caça das duas regiões não viram regiões à parte.
    { name: "Bandoleiro do Corvo, Atirador de Facas", area: true, x: -98, z: 98, radius: 12, levelMin: 13, levelMax: 15, region: "Bosque Gris" },
    { name: "Lobo Cinzento", area: true, x: -106, z: 34, radius: 6, levelMin: 7, levelMax: 9, region: "Bosque Gris" },
    { name: "Cavaleiro do Ossário", area: true, x: 146, z: 128, radius: 12, levelMin: 25, levelMax: 26, region: "Terras Mortas" },
    { name: "Espectro (à noite)", area: true, x: 250, z: 94, radius: 12, levelMin: 27, levelMax: 28, region: "Terras Mortas" },
  ] as never[];
  const { regions } = buildRegions(zones, 40);
  const names = regions.map((r) => r.name);
  assert.equal(names.filter((n) => n === "Bosque Gris").length, 1, "uma região só no Bosque");
  assert.equal(names.filter((n) => n === "Terras Mortas").length, 2, "o núcleo (setor) e a extensão (retângulo)");
  const bosque = regions.find((r) => r.name === "Bosque Gris")!;
  assert.equal(bosque.monsters, "Bandoleiro do Corvo, Atirador de Facas, Lobo Cinzento");
  assert.deepEqual(bosque.sector, { a0: 100, a1: 170 });
  // Onde se está: o Bosque no noroeste, as Terras Mortas no núcleo (setor) e na extensão (retângulo), e os anéis no resto.
  assert.equal(regionAt(regions, -98, 98).name, "Bosque Gris");
  assert.equal(regionAt(regions, -128, 118).name, "Bosque Gris", "a Toca do Corvo, no canto do quadrado");
  assert.equal(regionAt(regions, 120, 120).name, "Terras Mortas", "o núcleo, em volta da Cripta");
  assert.equal(regionAt(regions, 196, 94).name, "Terras Mortas", "o Bastião Cinzento, na extensão");
  assert.equal(regionAt(regions, 238, 122).name, "Terras Mortas", "a Torre do Sino Rachado");
  assert.notEqual(regionAt(regions, 0, 94).name, "Bosque Gris");
  assert.notEqual(regionAt(regions, -94, -10).name, "Bosque Gris");
  const mortas = regions.filter((r) => r.name === "Terras Mortas");
  assert.ok(mortas.every((r) => r.monsters === "Cavaleiro do Ossário, Espectro (à noite)" || r.monsters === ""), "os monstros vêm dos discos");
  assert.ok(mortas.some((r) => r.rect) && mortas.some((r) => r.sector));
});
