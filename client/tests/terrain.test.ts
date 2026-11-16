// Testes do relevo e da mistura de texturas do chão (src/world/terrain.ts), sem navegador:
// `npm run test:unit`.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import {
  bakeSplat,
  buildHeightField,
  clearRadius,
  heightAt,
  HILL_AMPLITUDE,
  MAIN_LAKE,
  rayTerrain,
  regionSplat,
  setTerrain,
  slopeAt,
  splatAt,
  type HeightField,
  type Splat,
} from "../src/world/terrain.ts";
import type { WorldData } from "../src/world/world-data.ts";

const load = (f: string) => JSON.parse(readFileSync(new URL(`../public/${f}`, import.meta.url), "utf8")) as WorldData;
const world = load("world.json");
const field = buildHeightField(world);

test("heightAt é pura: o mesmo ponto dá sempre a mesma altura", () => {
  const a = field.heightAt(63.3, -41.7);
  assert.equal(field.heightAt(63.3, -41.7), a);
  assert.equal(buildHeightField(world).heightAt(63.3, -41.7), a);
});

test("o chão é plano (y = 0) na cidade, nas estradas, na Cripta e no miolo do Vale", () => {
  const flat: [string, number, number][] = [
    ["praça", 0, 0],
    ["dentro da muralha", 30, -20],
    ["portão leste", 45, 0],
    ["estrada leste", 100, 2],
    ["estrada oeste", -140, -1],
    ["estrada norte", 1, 120],
    ["estrada do Vale", -2, -190],
    ["pedra de teleporte do Vale", 6, -198],
    ["chefe dos Ossos", 0, -132],
    ["covil do raid", 108, 108],
    ["miolo do Vale", 0, -270],
    ["golens do Vale", 30, -250],
  ];
  for (const [nome, x, z] of flat) assert.equal(field.heightAt(x, z), 0, nome);
  // Todas as estradas, de ponta a ponta.
  for (const r of world.roads) {
    for (let t = 0; t <= 1; t += 0.05) assert.equal(field.heightAt(r.x1 + (r.x2 - r.x1) * t, r.z1 + (r.z2 - r.z1) * t), 0);
  }
});

test("as colinas existem e são suaves: nunca abaixo do plano, no máximo HILL_AMPLITUDE, inclinação baixa", () => {
  const b = field.bounds;
  let max = 0;
  let maxSlope = 0;
  let hilly = 0;
  let n = 0;
  for (let x = b.minX; x <= b.maxX; x += 3) {
    for (let z = b.minZ; z <= b.maxZ; z += 3) {
      // O lago e a margem dele ficam fora do retângulo (testados à parte).
      if (field.lakeU(x, z) < 1.7) continue;
      const h = field.heightAt(x, z);
      assert.ok(h >= 0 && h <= HILL_AMPLITUDE + 1e-9, `altura ${h} em ${x},${z}`);
      max = Math.max(max, h);
      maxSlope = Math.max(maxSlope, slopeAt(field, x, z));
      n++;
      if (h > 0.4) hilly++;
    }
  }
  assert.ok(max > 0.8, `a maior colina tem ${max} m: sem relevo`);
  assert.ok(hilly / n > 0.15, "colinas demais planas");
  assert.ok(maxSlope < 0.13, `inclinação máxima ${maxSlope} (limite 0,13, uns 7 graus)`);
});

test("sem relevo (render antigo), tudo é plano e sem lago", () => {
  const flat = buildHeightField(world, false);
  assert.equal(flat.hilly, false);
  assert.equal(flat.lake, null);
  assert.equal(flat.heightAt(80, 60), 0);
  assert.equal(heightAt(80, 60), 0); // o campo ativo padrão também é plano
});

test("o campo ativo é o que setTerrain define", () => {
  setTerrain(field);
  assert.equal(heightAt(80, 60), field.heightAt(80, 60));
  assert.ok(heightAt(80, 60) > 0);
  setTerrain(buildHeightField(world, false));
  assert.equal(heightAt(80, 60), 0);
});

test("o lago fica fora do retângulo jogável: a água nunca está onde se anda", () => {
  const lake = field.lake!;
  assert.deepEqual(lake, MAIN_LAKE);
  const b = field.bounds;
  let water = 0;
  for (let x = b.minX; x <= b.maxX; x += 1) {
    for (let z = b.minZ; z <= b.maxZ; z += 1) {
      // Dentro do jogável o chão nunca fica abaixo do nível da água.
      assert.ok(field.heightAt(x, z) > lake.level - 0.05, `água em ${x},${z} (jogável)`);
    }
  }
  for (let x = field.extent.minX; x <= field.extent.maxX; x += 2) {
    for (let z = field.extent.minZ; z <= field.extent.maxZ; z += 2) if (field.heightAt(x, z) < lake.level) water++;
  }
  assert.ok(water > 300, `lago pequeno demais (${water} amostras)`);
  // O centro do lago é fundo, a borda do jogável é terra firme.
  assert.ok(field.heightAt(lake.cx, lake.cz) < lake.level - 1);
  assert.ok(field.heightAt(b.maxX, lake.cz) > lake.level);
});

test("o lago não existe em outros mapas", () => {
  const arena = buildHeightField(load("worlds/teste.json"));
  assert.equal(arena.lake, null);
  assert.equal(arena.heightAt(0, 30), 0); // a estrada
});

test("a mistura de texturas soma 1 em todo o mapa", () => {
  const e = field.extent;
  for (let x = e.minX; x <= e.maxX; x += 17) {
    for (let z = e.minZ; z <= e.maxZ; z += 17) {
      const w = splatAt(field, x, z);
      assert.ok(Math.abs(w[0] + w[1] + w[2] + w[3] - 1) < 1e-9, `soma em ${x},${z}`);
      assert.ok(w.every((v) => v >= 0 && v <= 1));
    }
  }
});

const dominant = (w: Splat) => w.indexOf(Math.max(...w));
const GRAMA = 0;
const TERRA = 1;
const PEDRA = 2;

test("cada região tem a sua mistura: grama nos Arredores, terra nos Campos e nos Ermos, pedra no Vale e na Cripta", () => {
  const arredores = regionSplat(world, 60, 0);
  const campos = regionSplat(world, 94, 0);
  const ermos = regionSplat(world, 130, 0);
  const vale = regionSplat(world, 0, -270);
  const cripta = regionSplat(world, 108, 108);
  assert.equal(dominant(arredores), GRAMA);
  assert.equal(dominant(campos), TERRA);
  assert.equal(dominant(ermos), TERRA);
  assert.equal(dominant(vale), PEDRA);
  assert.equal(dominant(cripta), PEDRA);
  // A grama diminui do centro para fora e a pedra aumenta (da cidade ao Vale).
  assert.ok(arredores[GRAMA] > campos[GRAMA] && campos[GRAMA] > ermos[GRAMA]);
  assert.ok(cripta[PEDRA] > vale[PEDRA] && vale[PEDRA] > arredores[PEDRA]);
});

test("a transição entre regiões é gradual (sem degrau)", () => {
  for (let r = 40; r < 140; r += 1) {
    const a = regionSplat(world, r, 0);
    const b = regionSplat(world, r + 1, 0);
    for (let i = 0; i < 4; i++) assert.ok(Math.abs(a[i] - b[i]) < 0.04, `salto em r=${r}`);
  }
});

test("encostas ganham pedra e a beira da água ganha cascalho", () => {
  const lake = field.lake!;
  // Beira do lago (u de 1,25) contra o mesmo tipo de chão longe da água.
  const sx = lake.cx - lake.rx * 1.25;
  const shore = splatAt(field, sx, lake.cz);
  const dry = regionSplat(world, sx, lake.cz);
  assert.ok(shore[3] > dry[3] + 0.1, "cascalho na beira");
  assert.ok(shore[0] < dry[0] + 0.05, "sem grama a mais na beira");
  // A região mais inclinada do mapa tem mais pedra que o plano em volta (mesma região).
  let best = { s: 0, x: 0, z: 0 };
  for (let x = -100; x <= 100; x += 2) {
    for (let z = 50; z <= 130; z += 2) {
      const s = slopeAt(field, x, z);
      if (s > best.s) best = { s, x, z };
    }
  }
  const steep = splatAt(field, best.x, best.z);
  const base = regionSplat(world, best.x, best.z);
  assert.ok(steep[PEDRA] > base[PEDRA], "pedra na encosta");
});

test("bakeSplat cobre a malha inteira em RGBA", () => {
  const s = bakeSplat(field, 8);
  assert.equal(s.data.length, s.width * s.height * 4);
  assert.ok(s.width >= (field.extent.maxX - field.extent.minX) / 8);
  for (let i = 0; i < s.data.length; i += 4) {
    const sum = s.data[i] + s.data[i + 1] + s.data[i + 2] + s.data[i + 3];
    assert.ok(Math.abs(sum - 255) <= 2, "soma dos canais");
  }
});

test("rayTerrain acerta o chão na altura certa", () => {
  const f: HeightField = field;
  // Raio de uma câmera a 12 m de altura olhando para baixo, para uma colina.
  const x = 70;
  const z = 40;
  const h = f.heightAt(x, z);
  const hit = rayTerrain(f, x, h + 12, z - 12, 0, -0.7071, 0.7071)!;
  assert.ok(hit);
  assert.ok(Math.abs(hit.y - f.heightAt(hit.x, hit.z)) < 1e-6);
  // Olhando para o céu, não acerta nada.
  assert.equal(rayTerrain(f, 0, 5, 0, 0, 1, 0), null);
});

test("a câmera nunca atravessa uma colina", () => {
  const f = field;
  for (let x = 20; x < 140; x += 7) {
    for (let z = 40; z < 140; z += 7) {
      for (const beta of [0.35, 0.7, 1.0, 1.4]) {
        for (const alpha of [0, 1.5, 3, 4.5]) {
          const ty = f.heightAt(x, z) + 1.4;
          const r = clearRadius(f, x, ty, z, alpha, beta, 45, 5);
          const cx = x + Math.cos(alpha) * Math.sin(beta) * r;
          const cz = z + Math.sin(alpha) * Math.sin(beta) * r;
          const cy = ty + Math.cos(beta) * r;
          assert.ok(r <= 45 && r >= 5);
          // Ou a câmera está acima do chão, ou chegou ao raio mínimo.
          assert.ok(cy >= f.heightAt(cx, cz) + 0.8 - 1e-9 || r === 5, `câmera no chão em ${x},${z}`);
        }
      }
    }
  }
});
