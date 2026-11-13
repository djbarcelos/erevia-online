// Gera os "kits" de cenário a partir dos pacotes KayKit (CC0) descompactados:
//   public/models/env-forest.glb  (Forest Nature Pack)
//   public/models/env-town.glb    (Medieval Hexagon Pack)
// Cada item vira UM nó "item:<nome>" com UMA malha (para desenhar milhares de cópias
// com thin instances). Os pacotes usam uma textura só cada, que fica uma vez no arquivo.
// Também grava src/world/kit-sizes.json com o tamanho (x, y, z) de cada item.
//
// Uso: node scripts/build-scenery.mjs ../assets-src/kaykit/x
import { Document, NodeIO, getBounds } from '@gltf-transform/core';
import { dedup, flatten, join, mergeDocuments, prune, unpartition } from '@gltf-transform/functions';
import { statSync, writeFileSync } from 'node:fs';
import { join as pjoin, resolve } from 'node:path';

const src = resolve(process.argv[2] ?? '../assets-src/kaykit/x');
const outDir = resolve(import.meta.dirname, '../public/models');
const forest = pjoin(src, 'KayKit_Forest_Nature_Pack_1.0_FREE/Assets/gltf');
const hex = pjoin(src, 'KayKit_Medieval_Hexagon_Pack_1.0_FREE/Assets/gltf');

const FOREST = [
  ...['Tree_1_A', 'Tree_1_B', 'Tree_2_A', 'Tree_2_B', 'Tree_3_A', 'Tree_4_A', 'Tree_4_B'],
  ...['Tree_Bare_1_A', 'Tree_Bare_1_B', 'Tree_Bare_2_A', 'Tree_Bare_2_B'],
  ...['Bush_1_A', 'Bush_2_A', 'Bush_3_A', 'Bush_4_A'],
  ...['Rock_1_A', 'Rock_1_E', 'Rock_2_A', 'Rock_3_A', 'Rock_3_E'],
  // Vale das Pedras Vivas (sul): mais variedade de pedra e árvore seca.
  ...['Rock_1_C', 'Rock_1_H', 'Rock_1_K', 'Rock_2_C', 'Rock_2_F', 'Rock_3_B', 'Rock_3_C', 'Rock_3_G', 'Rock_3_J'],
  ...['Tree_Bare_1_C', 'Tree_Bare_2_C'],
  ...['Grass_1_A', 'Grass_2_A'],
].map((n) => ({ name: n.toLowerCase(), path: pjoin(forest, `${n}_Color1.gltf`) }));

const b = (n) => ({ name: n.replace('building_', '').replace('_blue', ''), path: pjoin(hex, 'buildings/blue', `${n}.gltf`) });
const neutral = (n) => ({ name: n, path: pjoin(hex, 'buildings/neutral', `${n}.gltf`) });
const prop = (n) => ({ name: n, path: pjoin(hex, 'decoration/props', `${n}.gltf`) });
const nature = (n) => ({ name: n, path: pjoin(hex, 'decoration/nature', `${n}.gltf`) });
const TOWN = [
  ...['building_tavern_blue', 'building_blacksmith_blue', 'building_market_blue', 'building_church_blue',
    'building_home_A_blue', 'building_home_B_blue', 'building_well_blue', 'building_windmill_blue', 'building_tower_A_blue'].map(b),
  ...['wall_straight', 'wall_straight_gate', 'wall_corner_A_outside', 'fence_wood_straight'].map(neutral),
  ...['barrel', 'crate_A_big', 'crate_long_A', 'tent', 'weaponrack', 'flag_blue', 'sack', 'wheelbarrow'].map(prop),
  ...['mountain_A_grass_trees', 'mountain_B_grass', 'mountain_C_grass_trees', 'hills_A_trees', 'hills_B_trees'].map(nature),
  // Montanhas de rocha nua para a borda do vale do sul.
  ...['mountain_A', 'mountain_B', 'mountain_C', 'mountain_A_grass', 'hills_A', 'hills_C'].map(nature),
];

const io = new NodeIO();
const sizes = {};

async function buildKit(items, file) {
  const target = new Document();
  target.createBuffer();
  const scene = target.createScene('kit');
  target.getRoot().setDefaultScene(scene);

  for (const item of items) {
    item.name = item.name.toLowerCase();
    const doc = await io.read(item.path);
    // Junta todas as partes do item numa malha só (mesmo material = um draw call).
    await doc.transform(flatten(), join({ keepNamed: false }));
    const srcScene = doc.getRoot().getDefaultScene() ?? doc.getRoot().listScenes()[0];
    const bbox = getBounds(srcScene);
    sizes[item.name] = bbox.max.map((v, i) => +(v - bbox.min[i]).toFixed(3));
    sizes[item.name].push(+bbox.min[1].toFixed(3)); // [x, y, z, minY]

    const map = mergeDocuments(target, doc);
    const merged = map.get(srcScene);
    const holder = target.createNode(`item:${item.name}`);
    for (const child of merged.listChildren()) {
      merged.removeChild(child);
      holder.addChild(child);
    }
    scene.addChild(holder);
    merged.dispose();
  }
  await target.transform(unpartition(), dedup(), prune());
  const meshes = target.getRoot().listMeshes().length;
  const textures = target.getRoot().listTextures().length;
  const dst = pjoin(outDir, file);
  await io.write(dst, target);
  console.log(`${file}: ${items.length} itens, ${meshes} malhas, ${textures} textura(s), ${(statSync(dst).size / 1024).toFixed(0)} KB`);
}

await buildKit(FOREST, 'env-forest.glb');
await buildKit(TOWN, 'env-town.glb');
writeFileSync(resolve(import.meta.dirname, '../src/world/kit-sizes.json'), JSON.stringify(sizes, null, 1) + '\n');
console.log(`kit-sizes.json: ${Object.keys(sizes).length} itens`);
