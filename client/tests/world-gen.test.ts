// Testes do gerador do mundo (scripts/world-gen.mjs e scripts/biomes/), sem navegador:
// `npm run test:unit`.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { COVER, KEEP_CLEAR } from "../scripts/biomes/brejo.mjs";
import { paint as paintVale, transition as valeTransition } from "../scripts/biomes/vale.mjs";
import { BIOMES_DIR, buildWorld, loadBiomes, mulberry32, OUTPUTS } from "../scripts/world-gen.mjs";
import { Collision, worldBiomes, worldBounds, type BiomeEntry, type WorldData } from "../src/world/world-data.ts";

const sha256 = (s: string) => createHash("sha256").update(s).digest("hex");

/** O world.json do gerador antigo (um arquivo só, antes dos biomas), commit c2d3dde. */
const LEGACY_SHA256 = "5fd3be88bf27962fdb4117081cc8ba4e31db2f6296687e31ec9915d4fcb7a0e3";

/** Os três biomas de antes do Brejo: o mapa deles é o que o hash e a comparação dos formatos conferem. */
const ORIGINAL = ["floresta", "vale", "cripta"];
const original = async () => (await loadBiomes()).filter((b: { id: string }) => ORIGINAL.includes(b.id));

test("no formato antigo, o mapa sai byte a byte igual ao do gerador de antes dos biomas", async () => {
  const { json } = await buildWorld({ legacy: true, biomes: await original() });
  assert.equal(sha256(json), LEGACY_SHA256);
});

test("o mundo de hoje tem os cinco biomas, e o Brejo e as Terras Mortas só acrescentam (o resto não muda)", async () => {
  const base = (await buildWorld({ biomes: await original() })).world as WorldData;
  const now = (await buildWorld()).world as WorldData;
  assert.deepEqual(now.biomes!.map((b) => b.id), [...ORIGINAL, "brejo", "terras-mortas"]);
  assert.deepEqual(now.bounds, { minX: -150, maxX: 330, minZ: -330, maxZ: 150 });
  assert.deepEqual(base.bounds, { minX: -150, maxX: 150, minZ: -330, maxZ: 150 });
  assert.deepEqual(now.biomes!.slice(0, 3), base.biomes);
  // As montanhas da borda leste da floresta (x ≈ 168) que caíram dentro do mapa novo saem; o resto fica igual.
  const gone = base.objects.filter((o) => !now.objects.some((p) => p.k === o.k && p.x === o.x && p.z === o.z));
  assert.ok(gone.length > 0 && gone.length <= 12, `${gone.length} objetos sumiram`);
  for (const o of gone) assert.ok(/^(mountain|hills)/.test(o.k) && o.x > 150 && o.x < 190 && Math.abs(o.z) < 160, `${o.k} em (${o.x}, ${o.z})`);
  // O Brejo diz onde fica a lagoa, as partes fundas e a ilha (o relevo e a água do cliente leem daqui).
  const brejo = now.biomes!.find((b) => b.id === "brejo") as BiomeEntry & { lagoon: { x: number; z: number }; deeps: { x: number; z: number; r: number }[]; island: { r: number } };
  assert.equal(brejo.deeps.length, 2);
  assert.ok(brejo.island.r > 0 && brejo.lagoon.x === 245);
  // O servidor passa a ver a água funda: um anel de colisores em volta de cada parte funda.
  for (const d of brejo.deeps) {
    const ring = now.colliders.filter((c) => Math.abs(Math.hypot(c.x - d.x, c.z - d.z) - (d.r + 1.5)) < 0.05);
    assert.ok(ring.length >= 20, `${ring.length} colisores no anel de (${d.x}, ${d.z})`);
  }
});

test("o formato novo (biomes[]) descreve o mesmo mapa que o antigo (biome)", async () => {
  const old = (await buildWorld({ legacy: true, biomes: await original() })).world as WorldData;
  const now = (await buildWorld({ biomes: await original() })).world as WorldData;
  assert.equal(now.biome, undefined);
  assert.deepEqual(
    now.biomes!.map((b) => b.id),
    ORIGINAL,
  );
  // Tudo o que não é bioma é igual: colisores, instâncias (posição, giro, escala), tochas…
  const rest = ({ biome: _a, biomes: _b, ...r }: WorldData) => r;
  assert.deepEqual(rest(now), rest(old));
  // …e o cliente lê o mesmo Vale dos dois formatos (a entrada nova só ganha os limites).
  const vOld = worldBiomes(old).find((b) => b.id === "vale")!;
  const vNow = worldBiomes(now).find((b) => b.id === "vale")!;
  const { bounds, ...vNowRest } = vNow;
  assert.deepEqual(bounds, { minX: -150, maxX: 150, minZ: -330, maxZ: -150 });
  assert.deepEqual(vNowRest, vOld);
  assert.deepEqual(worldBiomes(old).map((b) => b.id), ["vale"]);
  // Colisão do cliente igual (amostra na cidade, na Cripta e no Vale).
  const cOld = new Collision(old);
  const cNow = new Collision(now);
  for (let x = -150; x <= 150; x += 1.5) for (let z = -330; z <= 150; z += 1.5) assert.equal(cNow.blocked(x, z), cOld.blocked(x, z));
});

/** Contexto 2D de mentira: grava cada chamada e cada propriedade, na ordem. */
function recorder() {
  const log: string[] = [];
  const g = new Proxy({} as Record<string, unknown>, {
    get(_t, k) {
      if (k === "getImageData")
        return (...a: number[]) => {
          log.push(`getImageData(${a})`);
          const data = new Uint8ClampedArray(a[2] * a[3] * 4);
          for (let i = 0; i < data.length; i++) data[i] = (i * 37) & 255; // grama de mentira
          return { data };
        };
      if (k === "putImageData")
        return (img: { data: Uint8ClampedArray }, x: number, y: number) =>
          log.push(`putImageData(${createHash("sha256").update(img.data).digest("hex")},${x},${y})`);
      return (...a: unknown[]) => log.push(`${String(k)}(${a})`);
    },
    set(_t, k, v) {
      log.push(`${String(k)}=${v}`);
      return true;
    },
  });
  return { g, log };
}

test("a pintura do chão do Vale é a mesma nos dois formatos", async () => {
  const draw = (world: WorldData) => {
    const b = worldBounds(world);
    const px = 150; // 0,5 px/m (a mesma conta do map-painter.ts: projection)
    const k = px / (b.maxX - b.minX);
    const p = { k, width: px, height: Math.round((b.maxZ - b.minZ) * k), cx: (x: number) => (x - b.minX) * k, cy: (z: number) => (b.maxZ - z) * k };
    const r = recorder();
    for (const bio of worldBiomes(world)) if (bio.id === "vale") paintVale(r.g, bio, { p, bounds: b, rng: mulberry32 });
    return r.log;
  };
  const old = draw((await buildWorld({ legacy: true, biomes: await original() })).world);
  const now = draw((await buildWorld({ biomes: await original() })).world);
  assert.ok(old.length > 1000, `pintou pouco: ${old.length} chamadas`);
  assert.deepEqual(now, old);
});

test("a borda do Vale é a conta de antes (rockyAt do world-data.ts até c2d3dde)", async () => {
  const vale = (await buildWorld()).world.biomes.find((b: BiomeEntry) => b.id === "vale");
  const rockyAt = (b: typeof vale, x: number, z: number) => {
    const n =
      (Math.sin(x * 0.045 + b.ph[0]) * Math.sin(z * 0.05 + b.ph[1]) +
        0.5 * Math.sin(x * 0.11 + z * 0.07 + b.ph[2]) +
        0.3 * Math.sin(x * 0.23 - z * 0.19 + b.ph[3])) /
      1.8;
    const t = Math.max(0, Math.min(1, (z + n * b.amp - b.zFrom) / (b.zTo - b.zFrom)));
    return t * t * (3 - 2 * t);
  };
  for (let x = -150; x <= 150; x += 0.7) for (let z = -330; z <= -120; z += 0.7) assert.equal(valeTransition(vale, x, z), rockyAt(vale, x, z));
  assert.equal(valeTransition(vale, 0, -100), 0); // floresta no norte
  assert.equal(valeTransition(vale, 0, -300), 1); // pedra no fundo
});

test("o world.json commitado (cliente e servidor) é o que o gerador produz", async () => {
  const { json } = await buildWorld();
  for (const p of OUTPUTS) assert.equal(sha256(readFileSync(p, "utf8")), sha256(json), `${p}: rode node scripts/build-world.mjs`);
});

test("criar um bioma é escrever um arquivo: o mapa cresce e o resto não muda", async () => {
  const dir = mkdtempSync(join(tmpdir(), "bioma-"));
  try {
    writeFileSync(
      join(dir, "teste.mjs"),
      `export const name = 'Bioma de Teste';
       export const seed = 1;
       export const order = 99;
       export const bounds = { minX: 330, maxX: 390, minZ: -150, maxZ: 60 };
       export function build(ctx) {
         ctx.scatter(12, { x: [335, 385], z: [-140, 50], spacing: 3 }, (x, z) => {
           ctx.place('barrel', x, z, ctx.rand() * 6.28, 2);
           ctx.collide(x, z, 0.8);
         });
       }`,
    );
    const base = (await buildWorld()).world;
    const biomes = await loadBiomes(BIOMES_DIR, dir);
    assert.deepEqual(biomes.map((b: { id: string }) => b.id), ["floresta", "vale", "cripta", "brejo", "terras-mortas", "teste"]);
    const grown = (await buildWorld({ biomes })).world;

    assert.deepEqual(grown.bounds, { ...base.bounds, maxX: 390 });
    assert.deepEqual(grown.biomes.at(-1), { id: "teste", name: "Bioma de Teste", bounds: { minX: 330, maxX: 390, minZ: -150, maxZ: 60 } });
    assert.deepEqual(grown.biomes.slice(0, -1), base.biomes);
    const isTest = (o: { k?: string; x: number }) => o.x > 332 && (o.k === undefined || o.k === "barrel");
    assert.equal(grown.objects.filter(isTest).length, 12);
    assert.equal(grown.colliders.filter(isTest).length, 12);
    assert.deepEqual(grown.objects.filter((o: { k: string; x: number }) => !isTest(o)), base.objects);
    assert.deepEqual(grown.colliders.filter((c: { x: number }) => !isTest(c)), base.colliders);
    assert.deepEqual(grown.torches, base.torches);
    assert.deepEqual(grown.roads, base.roads);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("um bioma sem semente própria (ou com a de outro) não entra", async () => {
  const dir = mkdtempSync(join(tmpdir(), "bioma-"));
  try {
    writeFileSync(
      join(dir, "copia.mjs"),
      `export const name = 'Cópia'; export const seed = 20261114; export const order = 50;
       export const bounds = { minX: 0, maxX: 1, minZ: 0, maxZ: 1 }; export function build() {}`,
    );
    await assert.rejects(loadBiomes(BIOMES_DIR, dir), /seed 20261114 repetida/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("Brejo: juncos, mato e capim não ficam em cima de NPC, pedra, bote, cais, estrada nem dos colisores do posto", async () => {
  const w = (await buildWorld()).world as WorldData;
  const cover = w.objects.filter((o) => o.x >= 150 && COVER.test(o.k));
  assert.ok(cover.length > 100, "o Brejo ainda tem mato");
  const boats = w.objects.filter((o) => o.k === "boat");
  assert.equal(boats.length, 2, "um bote no cais e outro na ilha");
  for (const o of cover) {
    for (const k of KEEP_CLEAR) assert.ok(Math.hypot(o.x - k.x, o.z - k.z) >= k.r, `${o.k} em (${o.x}, ${o.z}) em cima de um NPC ou pedra`);
    for (const b of boats) assert.ok(Math.hypot(o.x - b.x, o.z - b.z) >= 3.2, `${o.k} em (${o.x}, ${o.z}) dentro do bote`);
    for (const r of w.roads) {
      if (Math.max(r.x1, r.x2) < 150) continue;
      const dx = r.x2 - r.x1;
      const dz = r.z2 - r.z1;
      const t = Math.max(0, Math.min(1, ((o.x - r.x1) * dx + (o.z - r.z1) * dz) / (dx * dx + dz * dz || 1)));
      assert.ok(Math.hypot(o.x - (r.x1 + dx * t), o.z - (r.z1 + dz * t)) >= r.w / 2, `${o.k} em (${o.x}, ${o.z}) na estrada`);
    }
    if (Math.hypot(o.x - 165, o.z) < 16) {
      for (const c of w.colliders) assert.ok(Math.hypot(o.x - c.x, o.z - c.z) >= c.r, `${o.k} em (${o.x}, ${o.z}) dentro de um colisor do posto`);
    }
  }
});
