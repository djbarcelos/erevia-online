// Importa um asset do remaster (GLB exportado do Blender pela skill blender-image-to-3d)
// para client/public/models, conferindo o orçamento do guia de arte (docs/guia-de-arte.md,
// seção 5). Estoura o orçamento → sai com erro e não copia (a não ser com --force).
//
// Uso (dentro de client/):
//   node scripts/import-asset.mjs <arquivo.glb> <categoria> [nome] [--check] [--force]
//   categorias: personagem, arma, monstro, chefe, cenario
//   ex.: node scripts/import-asset.mjs ~/arte/CH_Knight/exports/CH_Knight.glb personagem knight_b
//
// Faz: tira duplicatas e nós sem uso, reamostra animações e junta primitivas de malhas sem
// esqueleto (menos draw calls). Os LODs vêm do Blender (nós com sufixo _LOD1/_LOD2).
import { NodeIO } from '@gltf-transform/core';
import { dedup, join as joinMeshes, prune, resample } from '@gltf-transform/functions';
import { mkdirSync, statSync } from 'node:fs';
import { basename, join, resolve } from 'node:path';

// Orçamento do guia de arte (LOD0). Ajustar junto com o guia.
const BUDGET = {
  personagem: { tris: 8000, texture: 1024, materials: 2 },
  arma: { tris: 800, texture: 1024, materials: 1 },
  monstro: { tris: 4000, texture: 1024, materials: 1 },
  chefe: { tris: 15000, texture: 2048, materials: 3 },
  cenario: { tris: 3000, texture: 2048, materials: 1 },
};

const args = process.argv.slice(2).filter((a) => !a.startsWith('--'));
const flags = new Set(process.argv.slice(2).filter((a) => a.startsWith('--')));
const [input, category, name = basename(args[0] ?? '', '.glb')] = args;
if (!input || !BUDGET[category]) {
  console.error('Uso: node scripts/import-asset.mjs <arquivo.glb> <personagem|arma|monstro|chefe|cenario> [nome] [--check] [--force]');
  process.exit(2);
}
const budget = BUDGET[category];
const out = resolve(import.meta.dirname, '../public/models');

const io = new NodeIO();
const doc = await io.read(resolve(input));
await doc.transform(dedup(), prune(), resample(), joinMeshes());

// Conta triângulos por LOD (o LOD vem do sufixo do nó; sem sufixo = LOD0).
const tris = { 0: 0, 1: 0, 2: 0 };
for (const node of doc.getRoot().listNodes()) {
  const mesh = node.getMesh();
  if (!mesh) continue;
  const lod = Number(/_LOD(\d)$/i.exec(node.getName())?.[1] ?? 0);
  for (const prim of mesh.listPrimitives()) {
    const idx = prim.getIndices();
    const count = idx ? idx.getCount() : (prim.getAttribute('POSITION')?.getCount() ?? 0);
    tris[lod] = (tris[lod] ?? 0) + Math.floor(count / 3);
  }
}
const materials = doc.getRoot().listMaterials().length;
const textures = doc.getRoot().listTextures().map((t) => ({ name: t.getName() || t.getURI(), size: t.getSize() }));
const biggest = Math.max(0, ...textures.map((t) => Math.max(...(t.size ?? [0, 0]))));
const anims = doc.getRoot().listAnimations().map((a) => a.getName());
const skins = doc.getRoot().listSkins();
const bones = Math.max(0, ...skins.map((s) => s.listJoints().length));

const problems = [];
if (tris[0] > budget.tris) problems.push(`LOD0 com ${tris[0]} triângulos (máximo ${budget.tris})`);
if (biggest > budget.texture) problems.push(`textura de ${biggest} px (máximo ${budget.texture})`);
if (materials > budget.materials) problems.push(`${materials} materiais (máximo ${budget.materials})`);
if (category === 'personagem' && bones > 60) problems.push(`${bones} ossos no esqueleto (o guia pede ~45 que deformam)`);

console.log(`${name} (${category})`);
console.log(`  triângulos: LOD0 ${tris[0]} · LOD1 ${tris[1]} · LOD2 ${tris[2]}`);
console.log(`  materiais: ${materials} · maior textura: ${biggest || '—'} px · ossos: ${bones || '—'}`);
console.log(`  animações: ${anims.length ? anims.join(', ') : '—'}`);
if (problems.length) {
  console.log(`  FORA DO ORÇAMENTO: ${problems.join('; ')}`);
  if (!flags.has('--force')) process.exit(1);
}
if (flags.has('--check')) process.exit(0);

mkdirSync(out, { recursive: true });
const dest = join(out, `${name}.glb`);
await io.write(dest, doc);
console.log(`  → ${dest} (${(statSync(dest).size / 1024).toFixed(1)} KB)`);
