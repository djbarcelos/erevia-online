// Juncos e taboas do Brejo (tunnel#139, polimento): três cartões cruzados com recorte (alphaTest, nunca blend), 6 triângulos.
// A textura é pintada aqui (SVG → PNG), sem fundo a recortar: a versão antiga chaveava o fundo do conceito por cor e deixava um
// disco claro na base e um contorno esbranquiçado nas folhas. Aqui o alfa nasce da própria forma e a cor dos pixels vazios é o
// verde-escuro das folhas (os mipmaps e o filtro não puxam branco para a borda).
//
// Uso (dentro de client/): node scripts/build-reeds.mjs && node scripts/import-asset.mjs <saída>/reeds.glb cenario nature_reeds_b --resize=256 --force
// A saída padrão é a pasta temporária do sistema (o `--out=<pasta>` troca).
import { Document, NodeIO } from '@gltf-transform/core';
import sharp from 'sharp';
import { mkdirSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';

const N = 256;
const W = 1.6; // largura real (m)
const H = 1.9; // altura real (m)
const DARK = [0x33, 0x4a, 0x24]; // cor dos pixels vazios (o filtro nunca mistura branco)

function mulberry32(a) {
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** O desenho: folhas compridas e quatro taboas (haste e espiga marrom), tudo saindo do mesmo ponto da base. */
function reedsSvg() {
  const rand = mulberry32(139);
  const base = 252;
  const parts = [];
  parts.push(`<defs>
    <linearGradient id="leaf" gradientUnits="userSpaceOnUse" x1="0" y1="${base}" x2="0" y2="30">
      <stop offset="0" stop-color="#2f4620"/><stop offset="0.55" stop-color="#4d6a2c"/><stop offset="1" stop-color="#8aa14b"/>
    </linearGradient>
    <linearGradient id="head" x1="0" y1="0" x2="1" y2="0">
      <stop offset="0" stop-color="#4a2e1b"/><stop offset="0.5" stop-color="#7a5230"/><stop offset="1" stop-color="#4a2e1b"/>
    </linearGradient>
  </defs>`);
  // Folhas: lentes finas da base até a ponta, abrindo para os lados.
  const leaves = 13;
  for (let i = 0; i < leaves; i++) {
    const t = i / (leaves - 1);
    const side = (t - 0.5) * 2; // -1 a 1
    const bx = 128 + side * 16 + (rand() - 0.5) * 6;
    const tipX = 128 + side * (30 + rand() * 62) + (rand() - 0.5) * 14;
    const tipY = 34 + rand() * 78 + Math.abs(side) * 38;
    const w = 5 + rand() * 4;
    const bend = (rand() - 0.5) * 28;
    const mx = (bx + tipX) / 2 + bend;
    const my = (base + tipY) / 2;
    parts.push(`<path d="M${(bx - w).toFixed(1)} ${base} Q${(mx - w * 0.9).toFixed(1)} ${my.toFixed(1)} ${tipX.toFixed(1)} ${tipY.toFixed(1)} Q${(mx + w * 0.9).toFixed(1)} ${my.toFixed(1)} ${(bx + w).toFixed(1)} ${base} Z" fill="url(#leaf)" stroke="#2a3d1c" stroke-width="0.8" stroke-opacity="0.55"/>`);
  }
  // Taboas: haste fina e a espiga marrom no alto.
  const stems = [[104, 96, -6], [122, 70, 3], [141, 88, 8], [158, 116, 16]];
  for (const [x, top, lean] of stems) {
    const tx = x + lean;
    parts.push(`<path d="M${x - 1.6} ${base} Q${x + lean * 0.2} ${(base + top) / 2} ${tx - 1.3} ${top + 20} L${tx + 1.3} ${top + 20} Q${x + lean * 0.2 + 3} ${(base + top) / 2} ${x + 1.6} ${base} Z" fill="#62743a" stroke="#2a3d1c" stroke-width="0.6" stroke-opacity="0.5"/>`);
    parts.push(`<ellipse cx="${tx}" cy="${top + 2}" rx="7.2" ry="22" fill="url(#head)" stroke="#2c1a0e" stroke-width="1" stroke-opacity="0.7"/>`);
    parts.push(`<path d="M${tx} ${top - 24} L${tx} ${top - 38}" stroke="#6b7a3e" stroke-width="2.2" stroke-linecap="round"/>`);
  }
  return `<svg xmlns="http://www.w3.org/2000/svg" width="${N}" height="${N}" viewBox="0 0 ${N} ${N}">${parts.join('')}</svg>`;
}

/** PNG RGBA: o alfa é o da forma; onde é vazio a cor é o verde-escuro (sem halo claro). */
export async function reedsPng() {
  const { data } = await sharp(Buffer.from(reedsSvg())).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
  const out = Buffer.from(data);
  for (let i = 0; i < N * N; i++) {
    const a = data[i * 4 + 3];
    if (a < 12) {
      out[i * 4] = DARK[0];
      out[i * 4 + 1] = DARK[1];
      out[i * 4 + 2] = DARK[2];
      out[i * 4 + 3] = 0;
    }
  }
  return sharp(out, { raw: { width: N, height: N, channels: 4 } }).png().toBuffer();
}

export async function buildReeds(dir) {
  const png = await reedsPng();
  const doc = new Document();
  const buf = doc.createBuffer();
  const tex = doc.createTexture('reeds').setImage(png).setMimeType('image/png');
  // MASK = alphaTest (cartão com recorte): nada de blend, o servidor de sombras e a névoa tratam como opaco.
  const mat = doc.createMaterial('reeds').setBaseColorTexture(tex).setAlphaMode('MASK').setAlphaCutoff(0.5).setDoubleSided(true).setMetallicFactor(0).setRoughnessFactor(1);
  const pos = [], nor = [], uv = [], idx = [];
  for (let q = 0; q < 3; q++) {
    const a = (q * Math.PI) / 3;
    const dx = Math.cos(a) * (W / 2);
    const dz = Math.sin(a) * (W / 2);
    const o = q * 4;
    pos.push(-dx, 0, -dz, dx, 0, dz, dx, H, dz, -dx, H, -dz);
    for (let i = 0; i < 4; i++) nor.push(0, 1, 0);
    uv.push(0, 1, 1, 1, 1, 0, 0, 0);
    idx.push(o, o + 1, o + 2, o, o + 2, o + 3);
  }
  const prim = doc.createPrimitive().setMaterial(mat)
    .setAttribute('POSITION', doc.createAccessor().setType('VEC3').setArray(new Float32Array(pos)).setBuffer(buf))
    .setAttribute('NORMAL', doc.createAccessor().setType('VEC3').setArray(new Float32Array(nor)).setBuffer(buf))
    .setAttribute('TEXCOORD_0', doc.createAccessor().setType('VEC2').setArray(new Float32Array(uv)).setBuffer(buf))
    .setIndices(doc.createAccessor().setType('SCALAR').setArray(new Uint16Array(idx)).setBuffer(buf));
  doc.createScene('s').addChild(doc.createNode('reeds').setMesh(doc.createMesh('reeds').addPrimitive(prim)));
  mkdirSync(dir, { recursive: true });
  const file = resolve(dir, 'reeds.glb');
  await new NodeIO().write(file, doc);
  await sharp(png).toFile(resolve(dir, 'reeds.png'));
  return file;
}

if (process.argv[1] && resolve(process.argv[1]) === resolve(import.meta.filename)) {
  const outArg = process.argv.find((a) => a.startsWith('--out='));
  const dir = outArg ? resolve(outArg.slice(6)) : mkdtempSync(resolve(tmpdir(), 'reeds-'));
  console.log(await buildReeds(dir));
}
