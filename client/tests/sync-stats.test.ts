import assert from "node:assert/strict";
import { test } from "node:test";
import { noteCorrection, noteDespawn, notePing, noteResync, noteSpawn, takeSyncSample } from "../src/net/sync-stats.ts";

test("conta correções acima de 0,5 m e de 2 m, retomadas e piscadas, e zera a cada amostra", () => {
  takeSyncSample();
  for (const e of [0.2, 0.6, 1.5, 2.5, 6]) noteCorrection(e);
  noteResync();
  noteDespawn(7, 1000);
  noteSpawn(7, 1500); // voltou em 0,5 s: piscou
  noteDespawn(8, 1000);
  noteSpawn(8, 3000); // voltou em 2 s: não conta
  noteSpawn(9, 3000); // nunca sumiu
  const s = takeSyncSample();
  assert.equal(s.corrections, 4);
  assert.equal(s.correctionsBig, 2);
  assert.equal(s.resyncs, 1);
  assert.equal(s.flickers, 1);
  assert.deepEqual(takeSyncSample(), { corrections: 0, correctionsBig: 0, resyncs: 0, flickers: 0, pingAvg: 0, pingP95: 0 });
});

test("ping: média e p95 da janela", () => {
  takeSyncSample();
  for (let i = 1; i <= 100; i++) notePing(i);
  notePing(Number.NaN);
  const s = takeSyncSample();
  assert.equal(s.pingAvg, 50.5);
  assert.equal(s.pingP95, 96);
});
