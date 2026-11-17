// Os NPCs do remaster (docs/arte/remaster-npcs.md): cada NPC com identidade tem arquivo, altura, clipes e nome que o
// servidor realmente usa. Sem navegador: `npm run test:unit`.
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
const serverSrc = ["game/talk.go", "zone/npc.go", "zone/specs.go", "zone/auction.go", "game/subclass.go"].map((f) => readFileSync(join(root, "../server/internal", f), "utf8")).join("\n");
const manifest = JSON.parse(readFileSync(join(root, "scripts/assets-manifest.json"), "utf8"));
const io = new NodeIO().registerExtensions([EXTMeshoptCompression, KHRTextureBasisu]);

test("cada NPC do mapa tem nome no servidor, arquivo no manifesto, altura e prefixo de clipes", () => {
  const models = Object.values(R.REMASTER_NPC_BY_NAME);
  assert.equal(new Set(models).size, models.length, "cada NPC tem o seu modelo (Bertrand usa o do guarda da cidade, repintado)");
  for (const [name, model] of Object.entries(R.REMASTER_NPC_BY_NAME)) {
    assert.ok(serverSrc.includes(`"${name}"`), `o servidor não tem o NPC ${name}`);
    assert.equal(R.REMASTER_FILES[model], `${model}_b`);
    assert.ok(existsSync(join(MODELS, `${model}_b.glb`)), `falta ${model}_b.glb`);
    assert.ok(manifest.modelos[`${model}_b.glb`], `${model}_b.glb fora do manifesto`);
    assert.ok(R.REMASTER_HEIGHTS[model] > 1.3 && R.REMASTER_HEIGHTS[model] < 2.2, `altura de ${model}`);
    assert.match(modelsSrc, new RegExp(`${model}: "[A-Z]{2}_"`), `prefixo de clipes de ${model}`);
  }
  for (const m of models) assert.ok(R.REMASTER_LAZY.includes(m), `${m} não está na lista sob demanda`);
  assert.equal(R.REMASTER_MASKS.npc_guarda, "npc_guarda_b.mask.png");
  assert.ok(existsSync(join(MODELS, "npc_guarda_b.mask.png")));
});

test("cada arquivo de NPC traz parado, duas pausas, cumprimento, andar e correr com o prefixo próprio", async () => {
  for (const model of new Set(Object.values(R.REMASTER_NPC_BY_NAME))) {
    const prefix = new RegExp(`${model}: "([A-Z]{2}_)"`).exec(modelsSrc)![1];
    const doc = await io.read(join(MODELS, `${model}_b.glb`));
    const names = doc.getRoot().listAnimations().map((a) => a.getName()).sort();
    assert.deepEqual(names, ["Break1", "Break2", "Greet", "Idle", "Run", "Walk"].map((n) => prefix + n).sort(), model);
    const tris = doc.getRoot().listNodes().some((n) => n.getName() === "handslot.r");
    assert.ok(tris, `${model}: sem encaixe de mão`);
  }
});

test("sem ?remaster=1 nada muda: remasterNpcModel só responde no remaster", () => {
  assert.equal(R.remasterNpcModel("Lívia"), "npc_livia");
  assert.equal(R.remasterNpcModel("Bertrand"), "npc_guarda");
  assert.equal(R.remasterNpcModel("Ninguém"), null);
});
