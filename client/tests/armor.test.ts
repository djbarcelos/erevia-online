import assert from "node:assert/strict";
import { test } from "node:test";
import { armorKey, bodyTier, capeFile, helmFile, NO_ARMOR, pauldronFile, toArmorGrades } from "../src/world/armor.ts";

const a = (o: Partial<typeof NO_ARMOR>) => toArmorGrades(o);

test("o peito escolhe a faixa de textura e as ombreiras", () => {
  assert.equal(bodyTier(a({})), 0);
  assert.equal(bodyTier(a({ chest: 1 })), 0);
  assert.equal(bodyTier(a({ chest: 2 })), 1);
  assert.equal(bodyTier(a({ chest: 3 })), 2);
  assert.equal(bodyTier(a({ chest: 4 })), 3);
  assert.equal(bodyTier(a({ chest: 5 })), 4);
  assert.equal(pauldronFile(a({ chest: 3 })), null);
  assert.equal(pauldronFile(a({ chest: 4 })), "pauldron_epic");
  assert.equal(pauldronFile(a({ chest: 5 })), "pauldron_legend");
});

test("sem peito, a melhor das outras peças vale até raro", () => {
  assert.equal(bodyTier(a({ legs: 4 })), 2);
  assert.equal(bodyTier(a({ feet: 2 })), 1);
  assert.equal(bodyTier(a({ chest: 1, legs: 4 })), 0);
});

test("capa e elmo seguem o item do próprio encaixe", () => {
  assert.equal(capeFile(a({ cape: 2 })), null);
  assert.equal(capeFile(a({ cape: 3 })), "cape_rare");
  assert.equal(capeFile(a({ cape: 4 })), "cape_epic");
  assert.equal(capeFile(a({ chest: 5 })), "cape_legend");
  assert.equal(capeFile(a({ chest: 5, cape: 4 })), "cape_legend");
  assert.equal(helmFile("knight", a({ helm: 3 })), "helm_knight_rare");
  assert.equal(helmFile("knight", a({ helm: 5 })), "helm_knight_epic");
  assert.equal(helmFile("mage", a({ helm: 3 })), null);
  assert.equal(helmFile("mage", a({ helm: 4 })), "hat_mage_epic");
  assert.equal(helmFile("rogue", a({ helm: 4 })), "hood_rogue_epic");
  assert.equal(helmFile("barbarian", a({ helm: 4 })), "helm_barbarian_epic");
});

test("a chave só muda quando o visual muda", () => {
  assert.equal(armorKey("knight", a({ chest: 1, legs: 2 })), armorKey("knight", a({ chest: 1, feet: 3 })));
  assert.notEqual(armorKey("knight", a({ chest: 3 })), armorKey("knight", a({ chest: 4 })));
  assert.equal(toArmorGrades({ chest: 99, helm: -3 }).chest, 5);
  assert.equal(toArmorGrades(null).helm, 0);
});
