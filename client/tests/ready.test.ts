// Lógica de "mundo pronto" (tela de carregamento), sem navegador: `npm run test:unit`.
import assert from "node:assert/strict";
import { test } from "node:test";
import { afterFrames, waitWorldReady } from "../src/world/ready.ts";

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

test("espera todas as etapas em ordem e avisa o progresso", async () => {
  const seen: string[] = [];
  const r = await waitWorldReady(
    [
      { label: "Cenário", wait: () => sleep(5) },
      { label: "Personagens", wait: () => sleep(5) },
    ],
    { timeoutMs: 1000, onStage: (i, t, l) => seen.push(`${i}/${t}:${l}`) },
  );
  assert.deepEqual(r, { timedOut: false, failed: [] });
  assert.deepEqual(seen, ["0/2:Cenário", "1/2:Personagens", "2/2:"]);
});

test("teto de tempo: entra assim mesmo e diz onde travou", async () => {
  let stuck = "";
  const r = await waitWorldReady(
    [
      { label: "Cenário", wait: () => sleep(1) },
      { label: "Personagens", wait: () => new Promise(() => undefined) },
    ],
    { timeoutMs: 30, onTimeout: (l) => (stuck = l) },
  );
  assert.equal(r.timedOut, true);
  assert.equal(stuck, "Personagens");
});

test("etapa que falha (modelos) não trava a entrada: segue e registra", async () => {
  const errs: string[] = [];
  const r = await waitWorldReady(
    [
      { label: "Personagens", wait: () => Promise.reject(new Error("glb 404")) },
      { label: "Preparando…", wait: () => sleep(1) },
    ],
    { timeoutMs: 1000, onStageError: (l) => errs.push(l) },
  );
  assert.deepEqual(r, { timedOut: false, failed: ["Personagens"] });
  assert.deepEqual(errs, ["Personagens"]);
});

test("sem etapas, termina na hora", async () => {
  assert.deepEqual(await waitWorldReady([], { timeoutMs: 1000 }), { timedOut: false, failed: [] });
});

test("afterFrames conta quadros e solta o observador", async () => {
  const fns = new Set<() => void>();
  const obs = {
    add: (fn: () => void) => (fns.add(fn), fn),
    remove: (fn: never) => void fns.delete(fn),
  };
  let done = false;
  void afterFrames(obs, 3).then(() => (done = true));
  for (let i = 0; i < 2; i++) for (const f of [...fns]) f();
  await sleep(1);
  assert.equal(done, false);
  for (const f of [...fns]) f();
  await sleep(1);
  assert.equal(done, true);
  assert.equal(fns.size, 0);
});
