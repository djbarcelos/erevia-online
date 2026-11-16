// Empacota tests/assets/load-page.ts (Babylon + src/world/assets.ts) num script único e copia o
// decodificador de KTX2, para o teste de navegador (e2e/tests/assets.spec.ts) servir de uma pasta.
// Uso: node tests/assets/build-page.mjs <pasta de saída>
import { cpSync, mkdirSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { build } from 'vite';

const client = resolve(import.meta.dirname, '../..');
const out = resolve(process.argv[2] ?? resolve(client, 'test-results/asset-page'));
mkdirSync(out, { recursive: true });
await build({
  root: client,
  configFile: false,
  logLevel: 'warn',
  publicDir: false,
  build: {
    outDir: out,
    emptyOutDir: false,
    minify: false,
    target: 'es2022',
    lib: { entry: resolve(client, 'tests/assets/load-page.ts'), name: 'AssetLoad', formats: ['iife'], fileName: () => 'load-page.js' },
  },
});
cpSync(resolve(client, 'public/ktx2'), resolve(out, 'ktx2'), { recursive: true });
writeFileSync(resolve(out, 'index.html'), '<!doctype html><meta charset="utf-8"><canvas id="c" width="256" height="256"></canvas><script src="/load-page.js"></script>');
console.log(out);
