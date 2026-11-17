// Textos da XP do grupo (janela do grupo), sem navegador: `npm run test:unit`.
import assert from "node:assert/strict";
import { test } from "node:test";
import { partyXPText } from "../src/ui/party-xp.ts";

const st = (near: number, bonus: number, gap = 0, cut = 0) => ({
  members: [1, 2, 3],
  xpNear: near,
  xpBonusPct: bonus,
  xpLevelGap: gap,
  xpGapCutPct: cut,
});

test("sem grupo, nada", () => {
  assert.deepEqual(partyXPText({ ...st(0, 0), members: [] }), { bonus: "", warn: "" });
});

test("bônus com quem está por perto", () => {
  assert.equal(partyXPText(st(3, 20)).bonus, "Bônus de grupo +20% (3 por perto)");
  assert.equal(partyXPText(st(3, 20)).warn, "");
  assert.match(partyXPText(st(1, 0)).bonus, /^Sem bônus de grupo: ninguém do grupo a até 40 m\.$/);
  assert.match(partyXPText(st(0, 0)).bonus, /^Morto/);
});

test("aviso do corte pela diferença de nível", () => {
  const t = partyXPText(st(2, 10, 9, 50));
  assert.equal(t.bonus, "Bônus de grupo +10% (2 por perto)");
  assert.equal(t.warn, "Diferença de 9 níveis no grupo: a XP de todos cai 50% (o corte começa com mais de 8).");
  assert.equal(partyXPText(st(2, 10, 8, 0)).warn, "");
});
