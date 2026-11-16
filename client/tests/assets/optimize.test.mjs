// Testes do que o import-asset faz: LOD automático e KTX2 (scripts/lib/asset-optimize.mjs) e o
// import de ponta a ponta numa pasta temporária. GLBs mínimos gerados aqui.
import { NodeIO } from '@gltf-transform/core';
import { KHRTextureBasisu } from '@gltf-transform/extensions';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { fileURLToPath } from 'node:url';
import { checkAssets } from '../../scripts/check-assets.mjs';
import { measure } from '../../scripts/lib/asset-budget.mjs';
import { compressTextures, generateLods, resizeTextures } from '../../scripts/lib/asset-optimize.mjs';
import { denseDoc, makeDoc, writeGlb } from './helpers.mjs';

// Registrar a extensão ensina o gltf-transform a medir imagens KTX2.
new NodeIO().registerExtensions([KHRTextureBasisu]);

const IMPORT = fileURLToPath(new URL('../../scripts/import-asset.mjs', import.meta.url));

test('generateLods cria LOD1 e LOD2 nas proporções do guia, sem recontar o LOD0', async () => {
  const doc = denseDoc(60); // 7200 triângulos
  const before = measure(doc).tris[0];
  const res = await generateLods(doc, 'personagem');
  assert.equal(res.generated, 2);
  const s = measure(doc);
  assert.equal(s.tris[0], before, 'o LOD0 não muda');
  // Proporções do guia (3000/8000 e 1000/8000), com folga de uns 15% para a simplificação.
  assert.ok(s.tris[1] > 0 && s.tris[1] <= before * 0.375 * 1.15, `LOD1 ${s.tris[1]}`);
  assert.ok(s.tris[2] > 0 && s.tris[2] <= before * 0.125 * 1.15, `LOD2 ${s.tris[2]}`);
  assert.ok(s.tris[2] < s.tris[1]);
});

test('generateLods: arma só ganha LOD1; LOD do Blender é respeitado', async () => {
  const doc = denseDoc(30);
  await generateLods(doc, 'arma');
  const names = doc.getRoot().listNodes().map((n) => n.getName());
  assert.deepEqual(names.sort(), ['Corpo', 'Corpo_LOD1']);
  const again = await generateLods(doc, 'arma');
  assert.equal(again.generated, 0);
  assert.match(again.skipped, /Blender/);
});

test('generateLods: o LOD usa o mesmo esqueleto e os mesmos atributos do LOD0', async () => {
  const doc = denseDoc(20);
  const skin = doc.createSkin('s');
  const joint = doc.createNode('osso');
  doc.getRoot().listScenes()[0].addChild(joint);
  skin.addJoint(joint);
  doc.getRoot().listNodes().find((n) => n.getName() === 'Corpo').setSkin(skin);
  await generateLods(doc, 'monstro');
  const [lod0, lod1] = ['Corpo', 'Corpo_LOD1'].map((n) => doc.getRoot().listNodes().find((x) => x.getName() === n));
  assert.equal(lod1.getSkin(), skin);
  assert.equal(lod1.getMesh().listPrimitives()[0].getAttribute('POSITION'), lod0.getMesh().listPrimitives()[0].getAttribute('POSITION'));
  assert.equal(lod1.getMesh().listPrimitives()[0].getMaterial(), lod0.getMesh().listPrimitives()[0].getMaterial());
});

test('resizeTextures reduz por potências de 2 até o limite e deixa texturas pequenas quietas', async () => {
  const big = makeDoc({ nodes: [['Corpo', 4]], texture: 256 });
  assert.equal(await resizeTextures(big, 64), 1);
  assert.equal(measure(big).texture, 64);
  assert.equal(big.getRoot().listTextures()[0].getMimeType(), 'image/png');
  const small = makeDoc({ nodes: [['Corpo', 4]], texture: 32 });
  assert.equal(await resizeTextures(small, 64), 0);
  assert.equal(measure(small).texture, 32);
});

test('generateLods com prune também gera os dois LODs', async () => {
  const doc = denseDoc(60);
  const res = await generateLods(doc, 'personagem', { prune: true });
  assert.equal(res.generated, 2);
  assert.ok(measure(doc).tris[2] > 0);
});

test('compressTextures troca o PNG por KTX2 (KHR_texture_basisu) e o tamanho continua legível', async () => {
  const doc = denseDoc(4);
  const n = await compressTextures(doc);
  assert.equal(n, 1);
  const tex = doc.getRoot().listTextures()[0];
  assert.equal(tex.getMimeType(), 'image/ktx2');
  assert.equal(measure(doc).texture, 64);
  const io = new NodeIO().registerExtensions([KHRTextureBasisu]);
  const back = await io.readBinary(await io.writeBinary(doc));
  assert.equal(back.getRoot().listTextures()[0].getMimeType(), 'image/ktx2');
  assert.ok(back.getRoot().listExtensionsRequired().some((e) => e.extensionName === 'KHR_texture_basisu'));
});

test('import-asset: gera LOD, comprime a textura, registra no manifesto e passa no check', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'erevia-import-'));
  try {
    const src = join(dir, 'espada_src.glb');
    await writeGlb(src, denseDoc(14)); // 392 triângulos, cabe em "arma"
    const models = join(dir, 'models');
    const manifestPath = join(dir, 'manifest.json');
    writeFileSync(manifestPath, JSON.stringify({ modelos: {} }));
    const env = { ...process.env, EREVIA_MODELS_DIR: models, EREVIA_ASSETS_MANIFEST: manifestPath };
    const run = spawnSync(process.execPath, [IMPORT, src, 'arma', 'espada_nova'], { env, encoding: 'utf8' });
    assert.equal(run.status, 0, run.stdout + run.stderr);
    assert.match(run.stdout, /LOD gerado/);
    assert.match(run.stdout, /KTX2: 1/);
    assert.ok(existsSync(join(models, 'espada_nova.glb')));
    const manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
    assert.equal(manifest.modelos['espada_nova.glb'].categoria, 'arma');
    const { failures } = await checkAssets({ dir: models, manifest });
    assert.deepEqual(failures, []);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test('import-asset: fora do orçamento não copia nada', async () => {
  const dir = mkdtempSync(join(tmpdir(), 'erevia-import-'));
  try {
    const src = join(dir, 'gorda.glb');
    await writeGlb(src, denseDoc(60)); // 7200 triângulos numa arma
    const models = join(dir, 'models');
    const env = { ...process.env, EREVIA_MODELS_DIR: models, EREVIA_ASSETS_MANIFEST: join(dir, 'm.json') };
    const run = spawnSync(process.execPath, [IMPORT, src, 'arma', 'gorda', '--no-ktx2'], { env, encoding: 'utf8' });
    assert.equal(run.status, 1, run.stdout + run.stderr);
    assert.match(run.stdout, /FORA DO ORÇAMENTO/);
    assert.equal(existsSync(join(models, 'gorda.glb')), false);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
