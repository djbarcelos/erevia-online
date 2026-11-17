// Nenhum NPC ou objeto desenhado em código (pedra de teleporte, túmulo, barco de Tomé) vira cápsula amarela.
// Sem navegador: `npm run test:unit`.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { test } from "node:test";

const root = resolve(import.meta.dirname, "..");
const entities = readFileSync(join(root, "src/world/entities.ts"), "utf8");
const server = readFileSync(join(root, "../server/internal/zone/teleport.go"), "utf8");

test("os modelos que o servidor manda para o barco, a pedra e o túmulo são desenhos próprios, nunca cápsula", () => {
  const model = (name: string) => new RegExp(name + ' = "([a-z_]+)"').exec(entities)?.[1];
  assert.equal(model("FERRY_MODEL"), /const ferryModel = "([a-z_]+)"/.exec(server)?.[1], "o cliente e o servidor falam do mesmo modelo do barco");
  const fn = /export function drawnInCode\(kind: EntityKind, model: string\): boolean \{([\s\S]*?)\n\}/.exec(entities)?.[1] ?? "";
  for (const m of ["STONE_MODEL", "TOMB_MODEL", "FERRY_MODEL"]) assert.ok(fn.includes(m), `${m} fora da lista de desenhos próprios`);
  // As duas portas da cápsula (a entrada e a soltura depois do carregamento) passam pela mesma regra.
  assert.match(entities, /private wantsCapsule\(v: View\): boolean \{\s*return [^;]*drawnInCode\(v\.kind, v\.model\)/);
  assert.match(entities, /if \(drawnInCode\(v\.kind, v\.model\) \|\| v\.model3d \|\| v\.capsule\) continue;/);
});
