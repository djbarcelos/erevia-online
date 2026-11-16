// Confere o orçamento de todos os GLB de public/models contra o guia de arte
// (docs/guia-de-arte.md, seção 5) e falha se algum passar. Roda no CI (`npm run check:assets`).
//
// A categoria de cada arquivo vem de scripts/assets-manifest.json (versionado). Arquivo sem
// entrada no manifesto falha: asset novo precisa dizer o que é. Os modelos que já estouram
// entram em "excecoes", com o teto de hoje e o motivo; crescer além do teto falha, e
// exceção que já não precisa falha também (a lista só encolhe).
//
// Variantes (cabeças e chapéus alternativos que o jogo esconde) ficam em "variantes": regex
// de nomes de nó, e cada grupo conta só a maior variante.
//
// Uso (dentro de client/): node scripts/check-assets.mjs [--manifest arq.json] [--dir pasta]
import { NodeIO } from '@gltf-transform/core';
import { EXTMeshoptCompression, KHRTextureBasisu } from '@gltf-transform/extensions';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join, relative, resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { CATEGORIES, SEM_MALHA, evaluate, measure } from './lib/asset-budget.mjs';

function listGlb(dir) {
  const out = [];
  for (const f of readdirSync(dir)) {
    const p = join(dir, f);
    if (statSync(p).isDirectory()) out.push(...listGlb(p));
    else if (f.toLowerCase().endsWith('.glb')) out.push(p);
  }
  return out.sort();
}

/** Confere uma pasta contra um manifesto. Retorna { report, failures } (listas de texto). */
export async function checkAssets({ dir, manifest }) {
  const io = new NodeIO().registerExtensions([EXTMeshoptCompression, KHRTextureBasisu]);
  const models = manifest.modelos ?? {};
  const files = listGlb(dir).map((p) => relative(dir, p).split('\\').join('/'));
  const report = [];
  const failures = [];
  for (const f of files) if (!models[f]) failures.push(`${f}: sem entrada em assets-manifest.json (diga a categoria)`);
  for (const f of Object.keys(models)) if (!files.includes(f)) failures.push(`${f}: está no manifesto, mas o arquivo não existe`);
  for (const f of files) {
    const entry = models[f];
    if (!entry) continue;
    if (SEM_MALHA.includes(entry.categoria)) {
      report.push(`${f} (${entry.categoria})`);
      continue;
    }
    if (!CATEGORIES.includes(entry.categoria)) {
      failures.push(`${f}: categoria "${entry.categoria}" desconhecida (${[...CATEGORIES, ...SEM_MALHA].join(', ')})`);
      continue;
    }
    const doc = await io.read(join(dir, f));
    const stats = measure(doc, { variants: entry.variantes ?? [], perPiece: entry.porPeca === true });
    const res = evaluate(stats, entry.categoria, entry.excecoes ?? {});
    report.push(
      `${f} (${entry.categoria}): LOD0 ${stats.tris[0]} · LOD1 ${stats.tris[1]} · LOD2 ${stats.tris[2]} tri · ${stats.materials} mat · ${stats.texture || '—'} px · ${stats.bones || '—'} ossos`,
    );
    for (const w of res.waived) report.push(`    exceção: ${w}`);
    for (const p of [...res.problems, ...res.stale]) failures.push(`${f}: ${p}`);
    for (const [k, ex] of Object.entries(entry.excecoes ?? {})) if (!ex.motivo) failures.push(`${f}: exceção de ${k} sem motivo`);
  }
  return { report, failures };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const arg = (name, def) => {
    const i = process.argv.indexOf(name);
    return i > 0 ? resolve(process.argv[i + 1]) : def;
  };
  const dir = arg('--dir', resolve(import.meta.dirname, '../public/models'));
  const manifest = JSON.parse(readFileSync(arg('--manifest', resolve(import.meta.dirname, 'assets-manifest.json')), 'utf8'));
  const { report, failures } = await checkAssets({ dir, manifest });
  console.log(report.join('\n'));
  if (failures.length) {
    console.error(`\n${failures.length} problema(s) de orçamento de assets:\n  - ${failures.join('\n  - ')}`);
    console.error('\nAjuste o asset (docs/guia-de-arte.md, seção 5) ou, se for legado a substituir, registre em excecoes com o motivo.');
    process.exit(1);
  }
  console.log(`\nOrçamento de assets: ${report.filter((l) => !l.startsWith('    ')).length} arquivo(s) conferidos, tudo dentro (ou com exceção registrada).`);
}
