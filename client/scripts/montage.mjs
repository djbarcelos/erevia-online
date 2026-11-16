// Junta imagens (JPG/PNG) numa fileira ou grade e grava um JPG (prints de prova, até 1280 px de largura).
// Uso (dentro de client/): node scripts/montage.mjs <saida.jpg> <colunas> <largura-final> <entrada...>
import jpeg from 'jpeg-js';
import { PNG } from 'pngjs';
import { readFileSync, writeFileSync } from 'node:fs';

const [out, colsArg, widthArg, ...inputs] = process.argv.slice(2);
const cols = Number(colsArg);
const decode = (f) => {
  const b = readFileSync(f);
  if (b[0] === 0x89) { const p = PNG.sync.read(b); return { width: p.width, height: p.height, data: p.data }; }
  const j = jpeg.decode(b, { useTArray: true, formatAsRGBA: true });
  return { width: j.width, height: j.height, data: j.data };
};
const imgs = inputs.map(decode);
const cw = Math.max(...imgs.map((i) => i.width));
const ch = Math.max(...imgs.map((i) => i.height));
const rows = Math.ceil(imgs.length / cols);
const W = cw * cols;
const H = ch * rows;
const sheet = { width: W, height: H, data: new Uint8Array(W * H * 4).fill(255) };
imgs.forEach((im, n) => {
  const ox = (n % cols) * cw;
  const oy = Math.floor(n / cols) * ch;
  for (let y = 0; y < im.height; y++) for (let x = 0; x < im.width; x++) {
    const s = (y * im.width + x) * 4;
    const d = ((oy + y) * W + ox + x) * 4;
    sheet.data[d] = im.data[s]; sheet.data[d + 1] = im.data[s + 1]; sheet.data[d + 2] = im.data[s + 2]; sheet.data[d + 3] = 255;
  }
});
// reduz por vizinho mais próximo suavizado (média 2x2 repetida) até a largura pedida
let cur = sheet;
const target = Number(widthArg);
while (cur.width / 2 >= target) {
  const w = Math.floor(cur.width / 2); const h = Math.floor(cur.height / 2);
  const nd = new Uint8Array(w * h * 4);
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) for (let c = 0; c < 4; c++) {
    const i = (2 * y * cur.width + 2 * x) * 4 + c;
    nd[(y * w + x) * 4 + c] = (cur.data[i] + cur.data[i + 4] + cur.data[i + cur.width * 4] + cur.data[i + cur.width * 4 + 4]) >> 2;
  }
  cur = { width: w, height: h, data: nd };
}
// ajuste final para a largura pedida (bilinear)
if (cur.width > target) {
  const w = target;
  const h = Math.round((cur.height * target) / cur.width);
  const nd = new Uint8Array(w * h * 4);
  for (let y = 0; y < h; y++) for (let x = 0; x < w; x++) {
    const fx = ((x + 0.5) * cur.width) / w - 0.5;
    const fy = ((y + 0.5) * cur.height) / h - 0.5;
    const x0 = Math.max(0, Math.floor(fx)); const y0 = Math.max(0, Math.floor(fy));
    const x1 = Math.min(cur.width - 1, x0 + 1); const y1 = Math.min(cur.height - 1, y0 + 1);
    const tx = Math.max(0, fx - x0); const ty = Math.max(0, fy - y0);
    for (let c = 0; c < 4; c++) {
      const p = (xx, yy) => cur.data[(yy * cur.width + xx) * 4 + c];
      nd[(y * w + x) * 4 + c] = (p(x0, y0) * (1 - tx) + p(x1, y0) * tx) * (1 - ty) + (p(x0, y1) * (1 - tx) + p(x1, y1) * tx) * ty;
    }
  }
  cur = { width: w, height: h, data: nd };
}
writeFileSync(out, jpeg.encode(cur, 85).data);
console.log(`${out}: ${cur.width}x${cur.height}`);
