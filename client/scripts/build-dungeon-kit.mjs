// Kit de cenário das masmorras (tunnel#140): public/models/env-catacumbas.glb, a partir do KayKit Dungeon Remastered 1.0
// (Kay Lousberg, CC0 1.0). Mesmo formato dos kits de cenário (build-scenery.mjs): cada peça vira UM nó "item:<nome>" com UMA
// malha, para desenhar centenas de cópias com thin instances.
//
// Por que o KayKit Dungeon e não outro kit CC0 (por exemplo, o Quaternius Modular Dungeon):
//   - é o mesmo autor, a mesma escala e o mesmo atlas de cores dos esqueletos e do cenário que o jogo já usa (os monstros das
//     Catacumbas são os esqueletos do remaster; peças de outro autor teriam proporção e traço diferentes ao lado deles);
//   - as peças são modulares em 4 m (parede de 4 × 4 × 1 m, ladrilho de 4 × 4 m), então o gerador monta o mapa numa grade de
//     4 m sem emendas; e cada peça tem poucos triângulos (a maior que entra tem 1.068), com uma textura só (1024 px, 1 material);
//   - a licença é CC0 (domínio público), registrada em public/models/CREDITS.md.
// O atlas original é colorido e quente; aqui ele é regradado para o tom frio e dessaturado da cripta (as pedras acinzentadas
// do remaster), sem mexer na malha: ver GRADE.
//
// Baixe o pacote para uma pasta de assets-src (nada de dentro dela é executado):
//   git clone --depth 1 https://github.com/KayKit-Game-Assets/KayKit-Dungeon-Remastered-1.0.git assets-src/kaykit-dungeon/src
// Uso (dentro de client/): node scripts/build-dungeon-kit.mjs [pasta com os .gltf.glb]
// Grava public/models/env-catacumbas.glb e scripts/dungeons/kit-sizes.json (tamanhos [x, y, z, minY] de cada peça).
import { Document, NodeIO, getBounds } from '@gltf-transform/core';
import { dedup, flatten, join, mergeDocuments, prune, unpartition } from '@gltf-transform/functions';
import { mkdirSync, statSync, writeFileSync } from 'node:fs';
import { dirname, join as pjoin, resolve } from 'node:path';
import { PNG } from 'pngjs';

const src = resolve(process.argv[2] ?? pjoin(import.meta.dirname, '../../assets-src/kaykit-dungeon/src/addons/kaykit_dungeon_remastered/Assets/gltf'));
const out = resolve(import.meta.dirname, '../public/models/env-catacumbas.glb');
const sizesOut = resolve(import.meta.dirname, 'dungeons/kit-sizes.json');

/** nome no jogo → arquivo do KayKit. Poucas peças: cada tipo é um draw call. */
const PIECES = {
  dg_wall: 'wall',
  dg_wall_broken: 'wall_broken',
  dg_pillar: 'pillar',
  dg_pillar_deco: 'pillar_decorated',
  dg_column: 'column',
  dg_barrel: 'barrel_large',
  dg_box: 'box_large',
  dg_rubble: 'rubble_large',
  dg_candles: 'candle_triple',
  dg_stairs: 'stairs',
  dg_banner: 'banner_red',
};

/** Gradação do atlas: dessatura e esfria (o remaster da cripta é cinza-azulado). */
const GRADE = { sat: 0.5, tint: [0.88, 0.91, 1.07], gain: 0.94 };
function grade(png) {
  const img = PNG.sync.read(Buffer.from(png));
  for (let i = 0; i < img.data.length; i += 4) {
    const r = img.data[i], g = img.data[i + 1], b = img.data[i + 2];
    const lum = 0.299 * r + 0.587 * g + 0.114 * b;
    const px = [r, g, b].map((c, k) => Math.max(0, Math.min(255, (lum + (c - lum) * GRADE.sat) * GRADE.tint[k] * GRADE.gain)));
    img.data[i] = px[0];
    img.data[i + 1] = px[1];
    img.data[i + 2] = px[2];
  }
  return new Uint8Array(PNG.sync.write(img));
}

const io = new NodeIO();
const target = new Document();
target.createBuffer();
const scene = target.createScene('kit');
target.getRoot().setDefaultScene(scene);
const sizes = {};
let graded = null;

for (const [name, file] of Object.entries(PIECES)) {
  const doc = await io.read(pjoin(src, `${file}.gltf.glb`));
  for (const tex of doc.getRoot().listTextures()) {
    graded ??= grade(tex.getImage());
    tex.setImage(graded);
  }
  await doc.transform(flatten(), join({ keepNamed: false }));
  const srcScene = doc.getRoot().getDefaultScene() ?? doc.getRoot().listScenes()[0];
  const bbox = getBounds(srcScene);
  sizes[name] = bbox.max.map((v, i) => +(v - bbox.min[i]).toFixed(3));
  sizes[name].push(+bbox.min[1].toFixed(3));
  const merged = mergeDocuments(target, doc).get(srcScene);
  const holder = target.createNode(`item:${name}`);
  for (const child of merged.listChildren()) {
    merged.removeChild(child);
    holder.addChild(child);
  }
  scene.addChild(holder);
  merged.dispose();
}
await target.transform(unpartition(), dedup(), prune());
mkdirSync(dirname(out), { recursive: true });
await io.write(out, target);
mkdirSync(dirname(sizesOut), { recursive: true });
writeFileSync(sizesOut, JSON.stringify(sizes, null, 1) + '\n');
console.log(
  `env-catacumbas.glb: ${Object.keys(PIECES).length} itens, ${target.getRoot().listMeshes().length} malhas, ` +
    `${target.getRoot().listTextures().length} textura(s), ${(statSync(out).size / 1024).toFixed(0)} KB`,
);
