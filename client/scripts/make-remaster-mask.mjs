// Gera a máscara de repintura de um modelo do remaster (textura gerada, sem o atlas 8x4 do KayKit).
//
// A textura-base fica em KTX2 (o jogo não consegue ler os pixels dela); a repintura de roupa e de
// pele é feita na GPU por um shader que mistura a base com esta máscara pequena (PNG, 512x512).
//   R = pano a tingir (o vermelho do tabardo e da capa),
//   G = pele a tingir (a pele do rosto, só nos triângulos ligados ao osso da cabeça),
//   A = brilho médio da pele original (0-255, igual em todo pixel), para manter a luz pintada.
// A máscara está no mesmo espaço UV da textura-base (linha 0 = v 0, como no glTF).
//
// Uso (dentro de client/):
//   node scripts/make-remaster-mask.mjs <cru.glb> <saida.png> [--pano=<regex>]
import { NodeIO } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';
import jpeg from 'jpeg-js';
import { writeFileSync } from 'node:fs';
import { PNG } from 'pngjs';

const [input, output] = process.argv.slice(2).filter((a) => !a.startsWith('--'));
const OUT = 512;
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const doc = await io.read(input);
const root = doc.getRoot();
const tex = root.listTextures()[0];
const img = tex.getMimeType() === 'image/png'
  ? (() => { const p = PNG.sync.read(Buffer.from(tex.getImage())); return { width: p.width, height: p.height, data: p.data }; })()
  : jpeg.decode(Buffer.from(tex.getImage()), { useTArray: true, formatAsRGBA: true });
const { width: W, height: H, data } = img;

// 1. Pano vermelho: mesmo critério da repintura antiga (v2), agora só para gerar a máscara.
const cloth = new Uint8Array(W * H);
// 2. Triângulos da cabeça, rasterizados no espaço UV.
const mesh = root.listMeshes()[0];
const skin = root.listSkins()[0];
const joints = skin.listJoints().map((j) => j.getName());
const isHead = joints.map((n) => /Head|Neck/.test(n) && !/handslot/.test(n));
const head = new Uint8Array(W * H);
for (const prim of mesh.listPrimitives()) {
  const uv = prim.getAttribute('TEXCOORD_0');
  const jt = prim.getAttribute('JOINTS_0');
  const wt = prim.getAttribute('WEIGHTS_0');
  const idx = prim.getIndices();
  const headW = new Float32Array(uv.getCount());
  for (let v = 0; v < uv.getCount(); v++) {
    const j = jt.getElement(v, []);
    const w = wt.getElement(v, []);
    for (let k = 0; k < 4; k++) if (isHead[j[k]]) headW[v] += w[k];
  }
  for (let t = 0; t < idx.getCount(); t += 3) {
    const ids = [idx.getScalar(t), idx.getScalar(t + 1), idx.getScalar(t + 2)];
    if (ids.some((i) => headW[i] < 0.6)) continue;
    const p = ids.map((i) => { const e = uv.getElement(i, []); return [e[0] * W, e[1] * H]; });
    const minX = Math.max(0, Math.floor(Math.min(...p.map((q) => q[0]))));
    const maxX = Math.min(W - 1, Math.ceil(Math.max(...p.map((q) => q[0]))));
    const minY = Math.max(0, Math.floor(Math.min(...p.map((q) => q[1]))));
    const maxY = Math.min(H - 1, Math.ceil(Math.max(...p.map((q) => q[1]))));
    const d = (p[1][1] - p[2][1]) * (p[0][0] - p[2][0]) + (p[2][0] - p[1][0]) * (p[0][1] - p[2][1]);
    if (Math.abs(d) < 1e-9) continue;
    for (let y = minY; y <= maxY; y++) {
      for (let x = minX; x <= maxX; x++) {
        const a = ((p[1][1] - p[2][1]) * (x + 0.5 - p[2][0]) + (p[2][0] - p[1][0]) * (y + 0.5 - p[2][1])) / d;
        const b = ((p[2][1] - p[0][1]) * (x + 0.5 - p[2][0]) + (p[0][0] - p[2][0]) * (y + 0.5 - p[2][1])) / d;
        if (a >= 0 && b >= 0 && a + b <= 1) head[y * W + x] = 1;
      }
    }
  }
}
// 3. Pele = pixel da cabeça com cor de pele (laranja claro, nem cinza do elmo nem marrom escuro do cabelo).
const skinMask = new Uint8Array(W * H);
let skinLuma = 0;
let skinN = 0;
for (let i = 0; i < W * H; i++) {
  const r = data[i * 4];
  const g = data[i * 4 + 1];
  const b = data[i * 4 + 2];
  const luma = 0.299 * r + 0.587 * g + 0.114 * b;
  if (r >= 70 && r >= 1.4 * g && r >= 1.4 * b) cloth[i] = 1;
  if (head[i] && r > g && g > b && r - b > 28 && luma > 95) {
    skinMask[i] = 1;
    skinLuma += luma;
    skinN++;
  }
}
const avg = skinN ? skinLuma / skinN : 150;

// 4. Média de blocos para 512x512 (bordas suaves).
const f = W / OUT;
const png = new PNG({ width: OUT, height: OUT });
let clothPx = 0;
for (let y = 0; y < OUT; y++) {
  for (let x = 0; x < OUT; x++) {
    let c = 0;
    let s = 0;
    for (let dy = 0; dy < f; dy++) {
      for (let dx = 0; dx < f; dx++) {
        const i = (y * f + dy) * W + (x * f + dx);
        c += cloth[i];
        s += skinMask[i];
      }
    }
    const o = (y * OUT + x) * 4;
    png.data[o] = Math.round((255 * c) / (f * f));
    png.data[o + 1] = Math.round((255 * s) / (f * f));
    png.data[o + 2] = 0;
    png.data[o + 3] = Math.round(avg);
    if (png.data[o] > 127) clothPx++;
  }
}
writeFileSync(output, PNG.sync.write(png));
console.log(`${output}: pano ${((100 * clothPx) / (OUT * OUT)).toFixed(1)}% da textura, pele ${skinN} px (brilho médio ${avg.toFixed(0)})`);
