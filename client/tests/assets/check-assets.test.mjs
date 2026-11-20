// Testes do orçamento de assets (scripts/check-assets.mjs e scripts/lib/asset-budget.mjs),
// com GLBs mínimos gerados aqui: `npm run test:unit` ou `npm run test:assets`.
import assert from 'node:assert/strict';
import { mkdirSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { checkAssets } from '../../scripts/check-assets.mjs';
import { evaluate, measure } from '../../scripts/lib/asset-budget.mjs';
import { makeDoc, writeGlb } from './helpers.mjs';

const tmp = () => mkdtempSync(join(tmpdir(), 'erevia-assets-'));

test('conta triângulos por LOD pelo sufixo do nó', () => {
  const doc = makeDoc({ nodes: [['Corpo', 1000], ['Corpo_LOD1', 400], ['Corpo_LOD2', 100]] });
  const s = measure(doc);
  assert.deepEqual(s.tris, { 0: 1000, 1: 400, 2: 100 });
});

test('variantes escondidas contam só a maior do grupo', () => {
  const doc = makeDoc({ nodes: [['Corpo', 1000], ['Head_a', 300], ['Head_b', 500], ['Head_c', 200], ['Hat', 100]] });
  assert.equal(measure(doc).tris[0], 1000 + 300 + 500 + 200 + 100, 'sem manifesto soma tudo');
  assert.equal(measure(doc, { variants: ['^Head_'] }).tris[0], 1000 + 500 + 100);
});

test('kit mede a maior peça, não a soma', () => {
  const doc = makeDoc({ nodes: [['Casa', 2000], ['Arvore', 900], ['Poco', 100]] });
  assert.equal(measure(doc).tris[0], 3000);
  assert.equal(measure(doc, { perPiece: true }).tris[0], 2000);
});

test('mede materiais, maior textura e ossos', () => {
  const s = measure(makeDoc({ nodes: [['A', 100]], materials: 2, texture: 512, bones: 30 }));
  assert.equal(s.materials, 2);
  assert.equal(s.texture, 512);
  assert.equal(s.bones, 30);
});

test('evaluate: dentro do orçamento passa; fora falha com o número', () => {
  const ok = { tris: { 0: 800, 1: 300, 2: 0 }, materials: 1, texture: 1024, bones: 0 };
  assert.deepEqual(evaluate(ok, 'arma').problems, []);
  const bad = { ...ok, tris: { 0: 801, 1: 301, 2: 0 }, materials: 2, texture: 2048 };
  const { problems } = evaluate(bad, 'arma');
  assert.equal(problems.length, 4);
  assert.match(problems.join('|'), /801/);
});

test('evaluate: exceção cobre até o teto, e crescer além do teto falha', () => {
  const stats = (tris) => ({ tris: { 0: tris, 1: 0, 2: 0 }, materials: 1, texture: 512, bones: 0 });
  const ex = { tris: { max: 900, motivo: 'legado' } };
  const within = evaluate(stats(900), 'arma', ex);
  assert.deepEqual(within.problems, []);
  assert.equal(within.waived.length, 1);
  assert.equal(evaluate(stats(901), 'arma', ex).problems.length, 1);
});

test('evaluate: exceção que já não é necessária é apontada', () => {
  const stats = { tris: { 0: 500, 1: 0, 2: 0 }, materials: 1, texture: 512, bones: 0 };
  const { problems, stale } = evaluate(stats, 'arma', { tris: { max: 900, motivo: 'legado' } });
  assert.deepEqual(problems, []);
  assert.equal(stale.length, 1);
});

test('categoria sem LOD2 não aceita LOD2', () => {
  const stats = { tris: { 0: 500, 1: 100, 2: 50 }, materials: 1, texture: 512, bones: 0 };
  assert.equal(evaluate(stats, 'arma').problems.length, 1);
});

test('checkAssets: pasta dentro do orçamento passa', async () => {
  const dir = tmp();
  try {
    await writeGlb(join(dir, 'espada.glb'), makeDoc({ nodes: [['Espada', 400]], texture: 512 }));
    const { failures } = await checkAssets({ dir, manifest: { modelos: { 'espada.glb': { categoria: 'arma' } } } });
    assert.deepEqual(failures, []);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test('checkAssets: asset novo sem manifesto, fora do orçamento e arquivo sumido falham', async () => {
  const dir = tmp();
  try {
    mkdirSync(join(dir, 'sub'));
    await writeGlb(join(dir, 'novo.glb'), makeDoc({ nodes: [['Novo', 100]] }));
    await writeGlb(join(dir, 'sub', 'gorda.glb'), makeDoc({ nodes: [['Gorda', 1200]], texture: 2048 }));
    const manifest = { modelos: { 'sub/gorda.glb': { categoria: 'arma' }, 'sumiu.glb': { categoria: 'arma' } } };
    const { failures } = await checkAssets({ dir, manifest });
    const all = failures.join('\n');
    assert.match(all, /novo\.glb: sem entrada/);
    assert.match(all, /sumiu\.glb: está no manifesto/);
    assert.match(all, /sub\/gorda\.glb: triângulos \(LOD0\): 1200/);
    assert.match(all, /sub\/gorda\.glb: px de textura: 2048/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test('checkAssets: exceção documentada passa; sem motivo falha', async () => {
  const dir = tmp();
  try {
    await writeGlb(join(dir, 'gorda.glb'), makeDoc({ nodes: [['Gorda', 1200]] }));
    const withReason = { modelos: { 'gorda.glb': { categoria: 'arma', excecoes: { tris: { max: 1200, motivo: 'KayKit legado' } } } } };
    assert.deepEqual((await checkAssets({ dir, manifest: withReason })).failures, []);
    const noReason = { modelos: { 'gorda.glb': { categoria: 'arma', excecoes: { tris: { max: 1200 } } } } };
    assert.match((await checkAssets({ dir, manifest: noReason })).failures.join(), /sem motivo/);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test('checkAssets: variantes do manifesto evitam contar cabeças duas vezes', async () => {
  const dir = tmp();
  try {
    await writeGlb(join(dir, 'heroi.glb'), makeDoc({ nodes: [['Corpo', 15000], ['Head_a', 3000], ['Head_b', 3000], ['Head_c', 3000]], bones: 20 }));
    const sem = await checkAssets({ dir, manifest: { modelos: { 'heroi.glb': { categoria: 'personagem' } } } });
    assert.equal(sem.failures.length, 1, '24000 tri passa de 20000 (orçamento de personagem)');
    const com = await checkAssets({ dir, manifest: { modelos: { 'heroi.glb': { categoria: 'personagem', variantes: ['^Head_'] } } } });
    assert.deepEqual(com.failures, []);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test('o manifesto real cobre todos os GLB de public/models e passa', async () => {
  const { readFileSync } = await import('node:fs');
  const manifest = JSON.parse(readFileSync(new URL('../../scripts/assets-manifest.json', import.meta.url), 'utf8'));
  const dir = new URL('../../public/models', import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, '$1');
  const { failures } = await checkAssets({ dir, manifest });
  assert.deepEqual(failures, []);
});
