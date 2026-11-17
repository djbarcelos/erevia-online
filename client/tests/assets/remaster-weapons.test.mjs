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
