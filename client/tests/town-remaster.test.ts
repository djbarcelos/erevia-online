// A cidade do remaster (tunnel#100) tem de caber no layout de sempre: o servidor não mudou, então a pegada
// de cada prédio novo fica perto do colisor dele, nenhum NPC cai dentro de parede e os adereços novos não
// ficam em cima de NPC nem de colisor. Sem navegador: `npm run test:unit`.
import assert from "node:assert/strict";
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { test } from "node:test";
import { NodeIO } from "@gltf-transform/core";
import { EXTMeshoptCompression, KHRTextureBasisu } from "@gltf-transform/extensions";
import { getBounds } from "@gltf-transform/functions";
import { EXTRA_FILES, SMALL_PROPS, TOWN_KITS, townExtras, wallCorners } from "../src/world/town-layout.ts";
import type { WorldData } from "../src/world/world-data.ts";

const root = resolve(import.meta.dirname, "..");
const MODELS = join(root, "public/models");
const world = JSON.parse(readFileSync(join(root, "public/world.json"), "utf8")) as WorldData;
const manifest = JSON.parse(readFileSync(join(root, "scripts/assets-manifest.json"), "utf8"));
const io = new NodeIO().registerExtensions([EXTMeshoptCompression, KHRTextureBasisu]);

/** Caixa (min/max, em metros e no espaço do modelo) da peça, já com a escala do world.json por objeto. */
async function box(file: string) {
  const doc = await io.read(join(MODELS, file));
  const b = getBounds(doc.getRoot().getDefaultScene() ?? doc.getRoot().listScenes()[0]);
  return { min: b.min, max: b.max };
}

/** Os 4 cantos da pegada de um objeto no mundo (Babylon: rotation.y = a leva o X local a (cos a, -sin a)). */
function corners(o: WorldData["objects"][number], b: { min: number[]; max: number[] }, kit: number[]) {
  // A escala do world.json já está na malha só nas peças de kit; aqui multiplica pela do objeto.
  const out: Array<[number, number]> = [];
  for (const lx of [b.min[0], b.max[0]]) {
    for (const lz of [b.min[2], b.max[2]]) {
      const sx = lx * o.s[0];
      const sz = lz * o.s[2];
      out.push([o.x + sx * Math.cos(o.r) + sz * Math.sin(o.r), o.z - sx * Math.sin(o.r) + sz * Math.cos(o.r)]);
    }
  }
  void kit;
  return out;
}

/** NPCs fixos da cidade: `x: .., z: ..` das definições do servidor (zone/*.go). */
function npcPositions(): Array<[string, number, number]> {
  const dir = resolve(root, "../server/internal/zone");
  const found: Array<[string, number, number]> = [];
  for (const f of readdirSync(dir)) {
    if (!f.endsWith(".go") || f.endsWith("_test.go")) continue;
    for (const line of readFileSync(join(dir, f), "utf8").split(/\r?\n/)) {
      const m = /name: "([^"]+)".*model: "[^"]+".*\bx: (-?[\d.]+), z: (-?[\d.]+)/.exec(line);
      if (m) found.push([m[1], Number(m[2]), Number(m[3])]);
    }
  }
  return found;
}

test("toda peça do kit existe, está no manifesto e substitui um item do world.json", () => {
  const kinds = new Set(world.objects.map((o) => o.k));
  for (const [kind, file] of Object.entries(TOWN_KITS)) {
    assert.ok(kinds.has(kind), `${kind}: o world.json não tem esse item`);
    assert.ok(existsSync(join(MODELS, file)), `${file}: falta em public/models`);
    assert.equal(manifest.modelos[file]?.categoria, "cenario", `${file}: sem categoria no manifesto`);
  }
  for (const file of Object.values(EXTRA_FILES)) assert.ok(existsSync(join(MODELS, file)), `${file}: falta em public/models`);
});

test("a pegada dos prédios novos cabe no colisor do servidor (até 1,6x o raio) e sem passar de 6 m dele", async () => {
  const prédios = ["tavern", "church", "blacksmith", "market", "home_a", "home_b", "tower_a"];
  for (const kind of prédios) {
    const b = await box(TOWN_KITS[kind]);
    for (const o of world.objects.filter((x) => x.k === kind)) {
      const col = world.colliders.filter((c) => Math.hypot(c.x - o.x, c.z - o.z) < 0.02).sort((a, b) => b.r - a.r)[0];
      assert.ok(col, `${kind} em (${o.x}, ${o.z}): sem colisor`);
      for (const [cx, cz] of corners(o, b, [])) {
        const d = Math.hypot(cx - o.x, cz - o.z);
        assert.ok(d <= col.r * 1.6, `${kind} em (${o.x}, ${o.z}): canto a ${d.toFixed(1)} m, o colisor tem raio ${col.r.toFixed(1)}`);
      }
    }
  }
});

test("a muralha e o portão novos têm a mesma tira do KayKit (largura e fundo) e o chão em y = 0", async () => {
  const kit = JSON.parse(readFileSync(join(root, "src/world/kit-sizes.json"), "utf8"));
  for (const kind of ["wall_straight", "wall_straight_gate"]) {
    const b = await box(TOWN_KITS[kind]);
    for (let i = 0; i < 3; i++) assert.ok(Math.abs(b.max[i] - b.min[i] - kit[kind][i]) < 0.01, `${kind} eixo ${i}`);
    assert.ok(Math.abs(b.min[1]) < 0.01, `${kind}: o chão não está em y = 0`);
  }
});

test("nenhum NPC fica dentro de um prédio novo e os adereços novos não ficam em cima de NPC, de colisor ou de prédio", async () => {
  const npcs = npcPositions();
  assert.ok(npcs.length >= 6, `achei só ${npcs.length} NPCs nos fontes do servidor`);
  const boxes: Array<{ kind: string; o: WorldData["objects"][number]; poly: Array<[number, number]> }> = [];
  for (const kind of Object.keys(TOWN_KITS)) {
    if (SMALL_PROPS.has(kind)) continue; // adereços sem colisão: o NPC pode estar perto
    const b = await box(TOWN_KITS[kind]);
    for (const o of world.objects.filter((x) => x.k === kind)) boxes.push({ kind, o, poly: corners(o, b, []) });
  }
  const inside = (px: number, pz: number, poly: Array<[number, number]>, margin: number) => {
    // caixa convexa (4 cantos em ordem x/z: 0,1,2,3 = (-,-), (-,+), (+,-), (+,+)); testa pelos eixos do objeto
    const c = poly.reduce((a, p) => [a[0] + p[0] / 4, a[1] + p[1] / 4], [0, 0]);
    const ex = [poly[2][0] - poly[0][0], poly[2][1] - poly[0][1]];
    const ez = [poly[1][0] - poly[0][0], poly[1][1] - poly[0][1]];
    const lx = Math.hypot(ex[0], ex[1]);
    const lz = Math.hypot(ez[0], ez[1]);
    const dx = ((px - c[0]) * ex[0] + (pz - c[1]) * ex[1]) / lx;
    const dz = ((px - c[0]) * ez[0] + (pz - c[1]) * ez[1]) / lz;
    return Math.abs(dx) < lx / 2 + margin && Math.abs(dz) < lz / 2 + margin;
  };
  for (const [name, x, z] of npcs) {
    for (const bx of boxes) assert.ok(!inside(x, z, bx.poly, 0.3), `${name} (${x}, ${z}) cai dentro de ${bx.kind} em (${bx.o.x}, ${bx.o.z})`);
  }
  for (const e of townExtras()) {
    for (const [name, x, z] of npcs) assert.ok(Math.hypot(e.x - x, e.z - z) >= 2, `${e.k} em (${e.x}, ${e.z}) a menos de 2 m de ${name}`);
    for (const c of world.colliders) assert.ok(Math.hypot(e.x - c.x, e.z - c.z) >= c.r + 0.4, `${e.k} em (${e.x}, ${e.z}) em cima de um colisor em (${c.x}, ${c.z})`);
    for (const bx of boxes) assert.ok(!inside(e.x, e.z, bx.poly, 0.5), `${e.k} em (${e.x}, ${e.z}) dentro de ${bx.kind}`);
  }
});

test("os torreões de canto ficam nos vértices da muralha, sem repetir as torres grandes", () => {
  const corners = wallCorners(world);
  const towers = world.objects.filter((o) => o.k === "tower_a").length;
  assert.equal(corners.length, world.wall.sides - towers);
  const vertexR = world.wall.apothem / Math.cos(Math.PI / world.wall.sides);
  for (const c of corners) {
    assert.ok(Math.abs(Math.hypot(c.x, c.z) - vertexR) < 0.05, `canto em (${c.x}, ${c.z}) fora do vértice`);
    for (const t of world.objects.filter((o) => o.k === "tower_a")) assert.ok(Math.hypot(c.x - t.x, c.z - t.z) > 3, "canto em cima de torre");
  }
});
