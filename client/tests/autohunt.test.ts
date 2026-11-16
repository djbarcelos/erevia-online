// Testes das regras de alvo da auto-caça (client/src/world/huntrules.ts) e do papel dos
// monstros (client/src/world/roles.ts), sem navegador: `npm run test:unit`.
import assert from "node:assert/strict";
import { test } from "node:test";
import { FIGHT_MS, SKIP_MS, STALL_MS, StallWatch, huntable, pickTarget, type HuntCandidate, type HuntRules } from "../src/world/huntrules.ts";
import { ARCH, neutralNow, neutralText, roleInfo } from "../src/world/roles.ts";

const mon = (id: number, dist: number, more: Partial<HuntCandidate> = {}): HuntCandidate => ({
  id, name: `M${id}`, dist, hp: 100, neutral: false, hidden: false, fightingMe: false, ...more,
});
const rules = (more: Partial<HuntRules> = {}): HuntRules => ({ names: null, neutrals: false, skip: new Set(), ...more });

test("não mira o que não vê nem o que travou", () => {
  assert.equal(huntable(mon(1, 5, { hidden: true }), rules()), false);
  assert.equal(huntable(mon(1, 5), rules({ skip: new Set([1]) })), false);
  // O emboscador escondido não está na lista; o visível mais longe é o alvo.
  const got = pickTarget([mon(1, 3, { hidden: true }), mon(2, 12)], rules(), 25);
  assert.equal(got?.id, 2);
});

test("pula os neutros, a não ser com a opção, brigando com você ou pedido pelo nome", () => {
  const n = mon(1, 4, { neutral: true });
  assert.equal(huntable(n, rules()), false);
  assert.equal(huntable(n, rules({ neutrals: true })), true);
  assert.equal(huntable({ ...n, fightingMe: true }, rules()), true);
  assert.equal(huntable(n, rules({ names: ["M1"] })), true);
  assert.equal(huntable(mon(2, 4), rules({ names: ["M1"] })), false, "o Caçar aqui só vale os nomes da missão");
  assert.equal(pickTarget([n, mon(2, 20)], rules(), 25)?.id, 2);
  assert.equal(pickTarget([n], rules(), 25), null, "só neutros por perto: nenhum alvo");
});

test("prefere quem já briga com você, dentro do raio", () => {
  const list = [mon(1, 3), mon(2, 9, { fightingMe: true }), mon(3, 30, { fightingMe: true })];
  assert.equal(pickTarget(list, rules(), 25)?.id, 2);
  assert.equal(pickTarget([mon(1, 26)], rules(), 25), null);
});

test("larga o alvo que não perde vida e de quem você não se aproxima (atirador fugindo)", () => {
  const w = new StallWatch();
  let t = 0;
  assert.equal(w.check(7, 100, 9, t), false);
  // Perseguindo: a distância cai, não trava.
  for (let d = 9; d > 3; d -= 1) assert.equal(w.check(7, 100, d, (t += 1000)), false);
  // Batendo: a vida cai, não trava (mesmo com o alvo recuando).
  for (let i = 0; i < 12; i++) assert.equal(w.check(7, 90 - i, 3 + (i % 5), (t += 1000)), false);
  // O atirador recuou para onde você não chega: nada muda por STALL_MS.
  const stuck = t;
  let stalled = false;
  while (!stalled && t - stuck < STALL_MS + 2000) stalled = w.check(7, 78, 12, (t += 250));
  assert.ok(stalled, "travou");
  assert.ok(t - stuck >= STALL_MS);
  assert.ok(w.skips(t).has(7), "pula o alvo travado por um tempo");
  assert.equal(pickTarget([mon(7, 12), mon(8, 15)], rules({ skip: w.skips(t) }), 25)?.id, 8);
  assert.equal(w.skips(t + SKIP_MS + 1).has(7), false, "depois volta a valer");
});

test("um alvo novo começa a contagem do zero", () => {
  const w = new StallWatch();
  assert.equal(w.check(1, 100, 5, 0), false);
  assert.equal(w.check(1, 100, 5, STALL_MS - 1), false);
  assert.equal(w.check(2, 100, 5, STALL_MS + 1), false);
  assert.equal(w.check(2, 100, 5, STALL_MS + 2), false);
  assert.ok(FIGHT_MS > 0);
});

test("papel: ícone e dica, e o neutro de dia", () => {
  assert.equal(roleInfo({}), null, "o bruto não tem selo");
  assert.equal(roleInfo({ role: ARCH.brute }), null);
  for (const [k, v] of Object.entries(ARCH)) {
    if (v === ARCH.brute) continue;
    const r = roleInfo({ role: v });
    assert.ok(r && r.icon.startsWith("<svg") && r.label && r.hint, k);
  }
  assert.equal(roleInfo({ role: ARCH.shield })?.label, "Escudo frontal");
  assert.equal(roleInfo({ neutral: true })?.key, "neutral");
  const byDay = { role: ARCH.pack, neutralByDay: true };
  assert.equal(neutralNow(byDay, false), true);
  assert.equal(neutralNow(byDay, true), false);
  assert.equal(neutralNow({ neutral: true }, true), true);
  assert.match(neutralText(byDay, true), /agressivo\)/);
});
