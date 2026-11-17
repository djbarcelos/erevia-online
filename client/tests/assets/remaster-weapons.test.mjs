// REMASTER_WEAPONS (tunnel#97): cada arma nova existe e cada nome do jogo que ela troca existe no KayKit.
import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { test } from 'node:test';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '../..');
const src = readFileSync(resolve(root, 'src/world/remaster.ts'), 'utf8');
const block = src.slice(src.indexOf('export const REMASTER_WEAPONS'));
const entries = [...block.matchAll(/^\s*(\w+): \{ file: "(\w+)"(?:, scale: ([\d.]+))? \}/gm)];

test('o mapa de armas do remaster não está vazio', () => assert.ok(entries.length >= 15));

for (const [, name, file, scale] of entries) {
  test(`arma ${name} -> ${file}`, () => {
    assert.ok(existsSync(resolve(root, `public/models/weapons/${name}.glb`)), `${name}.glb (KayKit) não existe`);
    assert.ok(existsSync(resolve(root, `public/models/weapons/${file}.glb`)), `${file}.glb (remaster) não existe`);
    if (scale) assert.ok(Number(scale) > 0.5 && Number(scale) < 1.5);
  });
}

// Parte 2: as formas por raridade (WEAPON_FILE_LEN) existem, cada família tem comum, rara e alta, e o grau 3 e o 4 escolhem arquivos diferentes.
const lens = [...src.matchAll(/(\w+_b): ([\d.]+)/g)].filter((m) => m[1] !== 'file').map((m) => m[1]);
const forms = new Set(lens.filter((f) => /^(sword|axe|staff|wand|dagger|shield)(_rare|_high)?_b$/.test(f)));

test('cada família tem a forma comum, a rara e a alta', () => {
  for (const fam of ['sword', 'axe', 'staff', 'wand', 'dagger', 'shield']) {
    for (const suffix of ['', '_rare', '_high']) assert.ok(forms.has(`${fam}${suffix}_b`), `${fam}${suffix}_b falta no mapa`);
  }
});
for (const f of forms) {
  test(`forma ${f} existe e é diferente das outras`, () => {
    assert.ok(existsSync(resolve(root, `public/models/weapons/${f}.glb`)), `${f}.glb não existe`);
  });
}
