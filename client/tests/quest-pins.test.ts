// Testes das missões fixadas e da rastreada (client/src/ui/quest-pins.ts), sem navegador:
// `npm run test:unit`.
import assert from "node:assert/strict";
import { test } from "node:test";
import { MAX_PINS, NEW_QUEST_MS, parsePins, QuestPins, type PinStorage } from "../src/ui/quest-pins.ts";

/** Armazenamento de mentira (o localStorage de um personagem) e um relógio na mão. */
function rig(raw: string | null = null, legacy: string | null = null) {
  const s = { t: 1000, raw, saves: 0 };
  const store: PinStorage = {
    load: () => s.raw,
    save: (r) => {
      s.raw = r;
      s.saves++;
    },
  };
  const pins = new QuestPins(store, () => s.t, legacy);
  return { s, store, pins };
}

test("primeira vez: fixa as 3 primeiras e rastreia a primeira, sem recém-aceitas", () => {
  const { pins } = rig();
  pins.update(["a", "b", "c", "d"]);
  assert.deepEqual(pins.pinned(), ["a", "b", "c"]);
  assert.equal(pins.tracked(), "a");
  assert.equal(pins.isTemp("d"), false);
  assert.equal(pins.visible("d"), false);
});

test("limite de 3: fixar com o painel cheio tira a mais antiga que não é a rastreada", () => {
  const { pins } = rig();
  pins.update(["a", "b", "c", "d", "e"]);
  pins.track("b");
  assert.equal(pins.wouldEvict(), "a");
  assert.equal(pins.pin("d"), "a");
  assert.deepEqual(pins.pinned(), ["b", "c", "d"]);
  assert.equal(pins.pin("e"), "c");
  assert.deepEqual(pins.pinned(), ["b", "d", "e"]);
  assert.equal(pins.pinned().length, MAX_PINS);
  assert.equal(pins.tracked(), "b");
  // Fixar de novo a mesma ou uma que não existe não muda nada.
  assert.equal(pins.pin("e"), null);
  assert.equal(pins.pin("zz"), null);
  assert.deepEqual(pins.pinned(), ["b", "d", "e"]);
});

test("a rastreada fica sempre entre as fixadas", () => {
  const { pins } = rig();
  pins.update(["a", "b", "c", "d"]);
  // Rastrear uma que não está fixada fixa ela (e tira outra, nunca a rastreada nova).
  assert.equal(pins.track("d"), "b");
  assert.equal(pins.tracked(), "d");
  assert.ok(pins.isPinned("d"));
  assert.deepEqual(pins.pinned(), ["a", "c", "d"]);
  // Desafixar a rastreada passa o rastreio para outra fixada.
  pins.unpin("d");
  assert.equal(pins.tracked(), "a");
  assert.ok(pins.isPinned(pins.tracked()!));
  // Sem nenhuma fixada, nenhuma rastreada.
  pins.unpin("a");
  pins.unpin("c");
  assert.deepEqual(pins.pinned(), []);
  assert.equal(pins.tracked(), null);
  // Fixar de novo volta a ter rastreada.
  pins.pin("c");
  assert.equal(pins.tracked(), "c");
  // Só uma rastreada: rastrear outra troca.
  pins.track("a");
  assert.equal(pins.tracked(), "a");
  assert.ok(pins.isPinned("c"));
});

test("missão nova entra sozinha com menos de 3 fixadas", () => {
  const { pins } = rig();
  pins.update(["a"]);
  pins.update(["a", "b"]);
  assert.deepEqual(pins.pinned(), ["a", "b"]);
  assert.equal(pins.isTemp("b"), false);
  assert.equal(pins.tracked(), "a");
});

test("missão nova com o painel cheio aparece por alguns segundos e some se não for fixada", () => {
  const { s, pins } = rig();
  pins.update(["a", "b", "c"]);
  pins.update(["a", "b", "c", "d"]);
  assert.deepEqual(pins.pinned(), ["a", "b", "c"]);
  assert.ok(pins.isTemp("d"));
  assert.ok(pins.visible("d"));
  assert.equal(pins.nextExpiry(), NEW_QUEST_MS);
  // Outro QuestLog no meio (um monstro a mais) não renova nem tira.
  s.t += 3000;
  pins.update(["a", "b", "c", "d"]);
  assert.ok(pins.isTemp("d"));
  assert.equal(pins.nextExpiry(), NEW_QUEST_MS - 3000);
  s.t += NEW_QUEST_MS;
  assert.equal(pins.visible("d"), false);
  assert.equal(pins.expire(), true);
  assert.equal(pins.nextExpiry(), null);
  assert.equal(pins.expire(), false);
  // Depois de sumir, um QuestLog novo não traz ela de volta.
  pins.update(["a", "b", "c", "d"]);
  assert.equal(pins.visible("d"), false);
});

test("o Fixar da recém-aceita fixa no lugar da mais antiga", () => {
  const { pins } = rig();
  pins.update(["a", "b", "c"]);
  pins.update(["a", "b", "c", "d"]);
  assert.equal(pins.pin("d"), "b"); // "a" é a rastreada
  assert.equal(pins.isTemp("d"), false);
  assert.deepEqual(pins.pinned(), ["a", "c", "d"]);
  assert.equal(pins.nextExpiry(), null);
});

test("missão concluída sai do painel e a rastreada segue a próxima da sequência", () => {
  const { pins } = rig();
  pins.update(["a", "b", "c"]);
  pins.track("b");
  // "b" terminou e "b2" (a próxima) chegou junto; "x" chegou também (nível novo).
  pins.update(["a", "c", "b2", "x"]);
  assert.equal(pins.isPinned("b"), false);
  assert.deepEqual(pins.pinned(), ["a", "c", "b2"]);
  assert.equal(pins.tracked(), "b2");
  assert.ok(pins.isTemp("x"));
  // Uma recém-aceita que termina antes de sumir sai também.
  pins.update(["a", "c", "b2"]);
  assert.equal(pins.visible("x"), false);
  assert.equal(pins.nextExpiry(), null);
  // A rastreada termina sozinha: outra fixada assume.
  pins.update(["a", "c"]);
  assert.equal(pins.tracked(), "a");
  pins.update([]);
  assert.deepEqual(pins.pinned(), []);
  assert.equal(pins.tracked(), null);
});

test("a escolha sobrevive a relogar e a desafixada não volta sozinha", () => {
  const first = rig();
  first.pins.update(["a", "b", "c", "d"]);
  first.pins.unpin("a");
  first.pins.track("d");
  const raw = first.s.raw;

  const again = rig(raw);
  // Ao entrar chegou "e" (nível novo enquanto estava fora): conta como nova.
  again.pins.update(["a", "b", "c", "d", "e"]);
  assert.deepEqual(again.pins.pinned(), ["b", "c", "d"]);
  assert.equal(again.pins.tracked(), "d");
  assert.equal(again.pins.isPinned("a"), false);
  assert.ok(again.pins.isTemp("e"));
});

test("relogar com uma fixada que acabou libera o lugar para a nova", () => {
  const first = rig();
  first.pins.update(["a", "b", "c"]);
  const again = rig(first.s.raw);
  again.pins.update(["b", "c", "z"]);
  assert.deepEqual(again.pins.pinned(), ["b", "c", "z"]);
  assert.equal(again.pins.tracked(), "z"); // a rastreada "a" acabou: segue a que chegou
});

test("só grava quando a escolha muda (o QuestLog chega a cada monstro)", () => {
  const { s, pins } = rig();
  pins.update(["a", "b"]);
  const n = s.saves;
  pins.update(["a", "b"]);
  pins.update(["a", "b"]);
  pins.track("a");
  assert.equal(s.saves, n);
  pins.track("b");
  assert.equal(s.saves, n + 1);
});

test("rastreada do esquema antigo vale na primeira vez", () => {
  const { pins } = rig(null, "c");
  pins.update(["a", "b", "c", "d"]);
  assert.equal(pins.tracked(), "c");
  assert.deepEqual(pins.pinned(), ["c", "a", "b"]);
});

test("persistência quebrada não derruba nada", () => {
  for (const raw of ["{oops", "null", "42", '"texto"', "[]", '{"pins":"a","track":7,"seen":{}}', '{"pins":[1,null,"a","a","b","c","d"]}']) {
    const { pins } = rig(raw);
    assert.doesNotThrow(() => pins.update(["a", "b", "c", "d"]), raw);
    assert.ok(pins.pinned().length <= MAX_PINS, raw);
    assert.ok(pins.tracked() === null || pins.isPinned(pins.tracked()!), raw);
  }
  assert.equal(parsePins("{oops"), null);
  assert.deepEqual(parsePins('{"pins":[1,"a","a","b","c","d"],"track":"b"}'), { pins: ["a", "b", "c"], track: "b", seen: [] });

  // Armazenamento que explode (modo privado, cota cheia): segue só na memória.
  const broken: PinStorage = {
    load: () => {
      throw new Error("bloqueado");
    },
    save: () => {
      throw new Error("cheio");
    },
  };
  const pins = new QuestPins(broken, () => 0);
  assert.doesNotThrow(() => pins.update(["a", "b"]));
  assert.doesNotThrow(() => pins.track("b"));
  assert.equal(pins.tracked(), "b");
  // Sem armazenamento nenhum também.
  const none = new QuestPins(null, () => 0);
  none.update(["a"]);
  assert.equal(none.tracked(), "a");
});
