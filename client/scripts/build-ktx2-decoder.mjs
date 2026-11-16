// Prepara o decodificador de KTX2/Basis do Babylon para servir do próprio site (public/ktx2/),
// em vez de buscar em cdn.babylonjs.com a cada jogador: o worker do Babylon faz importScripts
// de um script global (KTX2DECODER) e carrega os .wasm por URL. Empacotamos o
// @babylonjs/ktx2decoder (mesma versão do @babylonjs/core) nesse formato e copiamos os wasm.
// Saída ignorada pelo git; roda no `npm run build` e no `npm run dev`.
import { copyFileSync, mkdirSync, rmSync } from 'node:fs';
import { resolve } from 'node:path';
import { build } from 'vite';

const root = resolve(import.meta.dirname, '..');
const pkg = resolve(root, 'node_modules/@babylonjs/ktx2decoder');
const out = resolve(root, 'public/ktx2');

rmSync(out, { recursive: true, force: true });
mkdirSync(out, { recursive: true });
await build({
  root,
  configFile: false,
  logLevel: 'warn',
  publicDir: false,
  build: {
    outDir: out,
    emptyOutDir: false,
    minify: true,
    lib: { entry: resolve(pkg, 'index.js'), name: 'KTX2DECODER', formats: ['iife'], fileName: () => 'babylon.ktx2Decoder.js' },
  },
});
for (const f of [
  'msc_basis_transcoder.js',
  'msc_basis_transcoder.wasm',
  'zstddec.wasm',
  'uastc_astc.wasm',
  'uastc_bc7.wasm',
  'uastc_r8_unorm.wasm',
  'uastc_rg8_unorm.wasm',
  'uastc_rgba8_srgb_v2.wasm',
  'uastc_rgba8_unorm_v2.wasm',
]) {
  copyFileSync(resolve(pkg, 'wasm', f), resolve(out, f));
}
console.log(`ktx2: decodificador em ${out}`);
