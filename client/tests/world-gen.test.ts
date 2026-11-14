// Testes do gerador do mundo (scripts/world-gen.mjs e scripts/biomes/), sem navegador:
// `npm run test:unit`.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { BIOMES_DIR, buildWorld, loadBiomes, OUTPUTS } from "../scripts/world-gen.mjs";

const sha256 = (s: string) => createHash("sha256").update(s).digest("hex");

/** O world.json do gerador antigo (um arquivo só, antes dos biomas), commit c2d3dde. */
const LEGACY_SHA256 = "5fd3be88bf27962fdb4117081cc8ba4e31db2f6296687e31ec9915d4fcb7a0e3";

test("o mapa sai byte a byte igual ao do gerador de antes dos biomas", async () => {
  const { json } = await buildWorld();
  assert.equal(sha256(json), LEGACY_SHA256);
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
       export const bounds = { minX: 150, maxX: 210, minZ: -150, maxZ: 60 };
       export function build(ctx) {
         ctx.scatter(12, { x: [155, 205], z: [-140, 50], spacing: 3 }, (x, z) => {
           ctx.place('barrel', x, z, ctx.rand() * 6.28, 2);
           ctx.collide(x, z, 0.8);
         });
       }`,
    );
    const base = (await buildWorld()).world;
    const biomes = await loadBiomes(BIOMES_DIR, dir);
    assert.deepEqual(biomes.map((b: { id: string }) => b.id), ["floresta", "vale", "cripta", "teste"]);
    const grown = (await buildWorld({ biomes })).world;

    assert.deepEqual(grown.bounds, { ...base.bounds, maxX: 210 });
    const isTest = (o: { k?: string; x: number }) => o.x > 152 && (o.k === undefined || o.k === "barrel");
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
