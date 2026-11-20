// Corta uma folha de personagens (fundo liso ou transparente) em uma imagem por figura, pelas
// colunas que têm figura. Cada recorte sai quadrado, centrado, com margem e fundo cinza.
//   node tools/art/herois-v4/split-sheet.mjs <folha.png> <pasta-saida> [prefixo]
import { createRequire } from 'node:module';
import { mkdirSync } from 'node:fs';
const require = createRequire(new URL('../../../client/package.json', import.meta.url));
const sharp = require('sharp');

const [sheet, out, prefix = 'fig'] = process.argv.slice(2);
mkdirSync(out, { recursive: true });
const img = sharp(sheet).ensureAlpha();
const { data, info } = await img.raw().toBuffer({ resolveWithObject: true });
const { width: W, height: H, channels: C } = info;
const bg = [data[0], data[1], data[2]];
const isFig = (i) => data[i + 3] > 128 && Math.abs(data[i] - bg[0]) + Math.abs(data[i + 1] - bg[1]) + Math.abs(data[i + 2] - bg[2]) > 40;

// colunas e linhas ocupadas
const col = new Array(W).fill(0);
for (let y = 0; y < H; y++) for (let x = 0; x < W; x++) if (isFig((y * W + x) * C)) col[x]++;
const runs = [];
let start = -1;
for (let x = 0; x <= W; x++) {
  const on = x < W && col[x] > Math.max(2, H * 0.03);
  if (on && start < 0) start = x;
  if (!on && start >= 0) { if (x - start > 20) runs.push([start, x]); start = -1; }
}
let n = 0;
for (const [x0, x1] of runs) {
  let y0 = H, y1 = 0;
  for (let y = 0; y < H; y++) for (let x = x0; x < x1; x++) if (isFig((y * W + x) * C)) { y0 = Math.min(y0, y); y1 = Math.max(y1, y); }
  const w = x1 - x0, h = y1 - y0 + 1, side = Math.round(Math.max(w, h) * 1.12);
  const crop = await sharp(sheet).extract({ left: x0, top: y0, width: w, height: h }).toBuffer();
  await sharp({ create: { width: side, height: side, channels: 3, background: '#808080' } })
    .composite([{ input: crop, left: Math.round((side - w) / 2), top: Math.round((side - h) / 2) }])
    .png().toFile(`${out}/${prefix}-${++n}.png`);
  console.log(`${prefix}-${n}: x ${x0}-${x1}, y ${y0}-${y1}`);
}
