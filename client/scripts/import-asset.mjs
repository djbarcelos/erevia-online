// Importa um asset do remaster (GLB exportado do Blender pela skill blender-image-to-3d)
// para client/public/models, conferindo o orçamento do guia de arte (docs/guia-de-arte.md,
// seção 5). Estoura o orçamento → sai com erro e não copia (a não ser com --force).
//
// Uso (dentro de client/):
//   node scripts/import-asset.mjs <arquivo.glb> <categoria> [nome] [--check] [--force]
//        [--no-lod] [--lod-prune] [--no-ktx2] [--etc1s] [--resize=<px>] [--max-tris=<n>] [--variantes=<regex>]
//   categorias: personagem, arma, monstro, chefe, cenario
//   ex.: node scripts/import-asset.mjs ~/arte/CH_Knight/exports/CH_Knight.glb personagem knight_b
//
// Faz: tira duplicatas e nós sem uso, reamostra animações e junta primitivas de malhas sem
// esqueleto (menos draw calls); gera LOD1 e LOD2 (simplify do meshoptimizer, nas proporções do
// guia) se o Blender não trouxe os nós _LOD1/_LOD2; comprime as texturas para KTX2/Basis
// (UASTC; --etc1s troca por ETC1S, menor e pior). --no-lod e --no-ktx2 desligam cada um.
// Ao copiar, registra o arquivo em scripts/assets-manifest.json (o check:assets do CI exige).
import { NodeIO } from '@gltf-transform/core';
import { EXTMeshoptCompression, KHRTextureBasisu } from '@gltf-transform/extensions';
import { dedup, join as joinMeshes, prune, resample } from '@gltf-transform/functions';
import { existsSync, mkdirSync, readFileSync, statSync, writeFileSync } from 'node:fs';
import { basename, join, resolve } from 'node:path';
import { BUDGET, evaluate, measure } from './lib/asset-budget.mjs';
import { capTriangles, compressTextures, generateLods, resizeTextures } from './lib/asset-optimize.mjs';

const args = process.argv.slice(2).filter((a) => !a.startsWith('--'));
const flags = new Set(process.argv.slice(2).filter((a) => a.startsWith('--')));
// --variantes=<regex>: nós que o jogo esconde (cabeças e chapéus alternativos); contam só a maior.
const variants = process.argv.slice(2).filter((a) => a.startsWith('--variantes=')).map((a) => a.slice(12));
const [input, category, name = basename(args[0] ?? '', '.glb')] = args;
if (!input || !BUDGET[category]) {
  console.error('Uso: node scripts/import-asset.mjs <arquivo.glb> <personagem|arma|monstro|chefe|cenario> [nome] [--check] [--force] [--no-lod] [--no-ktx2] [--etc1s] [--variantes=regex]');
  process.exit(2);
}
// Destino e manifesto podem ser trocados por variável de ambiente (os testes usam uma pasta temporária).
const out = resolve(process.env.EREVIA_MODELS_DIR ?? resolve(import.meta.dirname, '../public/models'));
const manifestPath = resolve(process.env.EREVIA_ASSETS_MANIFEST ?? resolve(import.meta.dirname, 'assets-manifest.json'));

const io = new NodeIO().registerExtensions([EXTMeshoptCompression, KHRTextureBasisu]);
const doc = await io.read(resolve(input));
// keepNamed: os nós _LOD1/_LOD2 de peças estáticas não podem ser juntos ao LOD0.
await doc.transform(dedup(), prune(), resample(), joinMeshes({ keepNamed: true }));

const resize = process.argv.slice(2).find((a) => a.startsWith('--resize='));
if (resize) {
  const n = await resizeTextures(doc, Number(resize.slice(9)));
  if (n) console.log(`  textura reduzida para no máximo ${resize.slice(9)} px (${n})`);
}

const maxTris = process.argv.slice(2).find((a) => a.startsWith('--max-tris='));
if (maxTris) {
  const r = await capTriangles(doc, Number(maxTris.slice(11)));
  if (r.after !== r.before) console.log(`  LOD0 simplificado: ${r.before} → ${r.after} triângulos`);
}

if (!flags.has('--no-lod')) {
  const lod = await generateLods(doc, category, { prune: flags.has('--lod-prune') });
  console.log(lod.generated ? `  LOD gerado: ${lod.generated} nó(s) _LOD1/_LOD2` : `  LOD: ${lod.skipped}`);
}

const stats = measure(doc, { variants });
const anims = doc.getRoot().listAnimations().map((a) => a.getName());
const { problems, over } = evaluate(stats, category);

console.log(`${name} (${category})`);
console.log(`  triângulos: LOD0 ${stats.tris[0]} · LOD1 ${stats.tris[1]} · LOD2 ${stats.tris[2]}`);
console.log(`  materiais: ${stats.materials} · maior textura: ${stats.texture || '—'} px · ossos: ${stats.bones || '—'}`);
console.log(`  animações: ${anims.length ? anims.join(', ') : '—'}`);
if (problems.length) {
  console.log(`  FORA DO ORÇAMENTO: ${problems.join('; ')}`);
  if (!flags.has('--force')) process.exit(1);
}
if (flags.has('--check')) process.exit(0);

if (!flags.has('--no-ktx2') && doc.getRoot().listTextures().length) {
  const n = await compressTextures(doc, { etc1s: flags.has('--etc1s') });
  console.log(`  KTX2: ${n} textura(s) comprimida(s) (${flags.has('--etc1s') ? 'ETC1S' : 'UASTC'})`);
}

mkdirSync(out, { recursive: true });
const dest = join(out, `${name}.glb`);
await io.write(dest, doc);
console.log(`  → ${dest} (${(statSync(dest).size / 1024).toFixed(1)} KB)`);

// O manifesto é o que o check:assets confere no CI. Um import forçado para fora do orçamento
// entra com a exceção e um motivo a preencher (o CI falha enquanto o motivo estiver vazio).
const manifest = existsSync(manifestPath) ? JSON.parse(readFileSync(manifestPath, 'utf8')) : { modelos: {} };
const entry = { categoria: category };
if (variants.length) entry.variantes = variants;
if (problems.length) entry.excecoes = Object.fromEntries(Object.entries(over).map(([k, v]) => [k, { max: v, motivo: '' }]));
manifest.modelos[`${name}.glb`] = entry;
manifest.modelos = Object.fromEntries(Object.entries(manifest.modelos).sort(([a], [b]) => a.localeCompare(b)));
writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + '\n');
console.log(`  manifesto: ${name}.glb registrado em scripts/assets-manifest.json${entry.excecoes ? ' (preencha o motivo das exceções)' : ''}`);
