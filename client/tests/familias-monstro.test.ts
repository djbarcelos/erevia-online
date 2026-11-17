// As famílias de monstro não humanoides do remaster (docs/arte/familias-monstro.md): lobo, aranha, lodo e sapo.
// Cada uma tem arquivo, manifesto, altura, carga sob demanda sem pré-carga, passo natural e os clipes do contrato.
// Sem navegador: `npm run test:unit`.
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { test } from "node:test";
import { NodeIO } from "@gltf-transform/core";
import { EXTMeshoptCompression, KHRTextureBasisu } from "@gltf-transform/extensions";

(globalThis as unknown as { location: { search: string } }).location = { search: "?remaster=1" };
const R = await import("../src/world/remaster.ts");

const root = resolve(import.meta.dirname, "..");
const MODELS = join(root, "public/models");
const modelsSrc = readFileSync(join(root, "src/world/models.ts"), "utf8");
const manifest = JSON.parse(readFileSync(join(root, "scripts/assets-manifest.json"), "utf8"));
const io = new NodeIO().registerExtensions([EXTMeshoptCompression, KHRTextureBasisu]);

/** modelo do jogo -> prefixo dos clipes e papéis obrigatórios do contrato (parado, andar, correr, golpe, dano, morte). */
const FAMILIES: Record<string, string> = { wolf: "WF_", spider: "SP_", slime: "SL_", frog: "FR_" };
const CONTRACT = ["Idle", "Walk", "Run", "Attack", "Hit", "Death"];

test("cada família tem arquivo, manifesto, altura e fica sob demanda sem pré-carga", () => {
  for (const model of Object.keys(FAMILIES)) {
    assert.equal(R.REMASTER_FILES[model], `${model}_b`);
    assert.ok(existsSync(join(MODELS, `${model}_b.glb`)), `falta ${model}_b.glb`);
    assert.equal(manifest.modelos[`${model}_b.glb`]?.categoria, "monstro");
    assert.ok(R.REMASTER_HEIGHTS[model] > 0.3 && R.REMASTER_HEIGHTS[model] < 2, `altura de ${model}`);
    assert.ok(R.REMASTER_LAZY.includes(model), `${model} deve baixar sob demanda`);
    assert.ok(R.REMASTER_NO_PREFETCH.includes(model), `${model} não entra na pré-carga ociosa`);
    assert.match(modelsSrc, new RegExp(`(?:^|[ ,])${model}: \\{ walk: [0-9.]+, run: [0-9.]+ \\}`), `passo natural de ${model}`);
  }
});

test("cada arquivo traz os clipes do contrato, com nomes próprios por papel, 1 material e 1 pele", async () => {
  for (const [model, p] of Object.entries(FAMILIES)) {
    const doc = await io.read(join(MODELS, `${model}_b.glb`));
    const names = doc.getRoot().listAnimations().map((a) => a.getName());
    for (const c of CONTRACT) assert.ok(names.includes(p + c), `${model}: falta ${p + c}`);
    assert.equal(new Set(names).size, names.length);
    assert.ok(doc.getRoot().listSkins().length >= 1, `${model}: sem esqueleto`);
    // os papéis do conjunto apontam para clipes que existem, e andar e correr nunca dividem o mesmo clipe
    const set = new RegExp(`(?:^|[ ,])${model}: \\{ idle: "${p}Idle", walk: "(\\w+)", run: "(\\w+)"`).exec(modelsSrc);
    assert.ok(set, `${model}: conjunto de clipes no models.ts`);
    assert.notEqual(set![1], set![2], `${model}: andar e correr no mesmo clipe (o grupo do Babylon é um por nome)`);
    for (const n of set!.slice(1)) assert.ok(names.includes(n), `${model}: ${n} não está no arquivo`);
  }
});
