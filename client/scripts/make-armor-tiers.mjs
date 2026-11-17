// Texturas de armadura por raridade (tunnel#96): junta o retexture do Meshy com a textura original.
//
// O retexture do Meshy mantém a malha e a UV (5.936 vértices e UV idênticas ao image-to-3d), então a
// textura nova serve no mesmo GLB com rig. Ele troca também o rosto e o cabelo (a pele saía de mármore):
// aqui as faixas de pele e cabelo da máscara de repintura (G e B) voltam ao pixel original, e só a roupa e
// a armadura vêm do retexture. Saída: public/models/armor/<classe>_<faixa>.ktx2 (1024 px, UASTC),
// compartilhada por todos os personagens da classe naquela faixa (um material por classe e faixa).
//
// Uso (dentro de client/): node scripts/make-armor-tiers.mjs <pasta dos retextures> <pasta dos image-to-3d> <classe> <faixa> <retexture-id8> <imagem-id8>
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { encodeToKTX2 } from 'ktx2-encoder';
import sharp from 'sharp';

const [retexDir, i3dDir, cls, tier, retexId, srcId] = process.argv.slice(2);
const SIZE = 1024;
const raw = async (p) => sharp(p).resize(SIZE, SIZE, { fit: 'fill' }).ensureAlpha().raw().toBuffer();
const orig = await raw(`${i3dDir}/image-to-3d-${srcId}/base_color-texture_0.png`);
const tex = await raw(`${retexDir}/retexture-${retexId}/base_color-texture_0.png`);
const mask = await sharp(`public/models/${cls}_b.mask.png`).resize(SIZE, SIZE).ensureAlpha().raw().toBuffer();
const out = Buffer.alloc(SIZE * SIZE * 4, 255);
for (let i = 0; i < SIZE * SIZE; i++) {
  // pele e cabelo voltam ao original (suave nas bordas da máscara)
  const keep = Math.min(1, (mask[i * 4 + 1] + mask[i * 4 + 2]) / 255);
  for (let c = 0; c < 3; c++) out[i * 4 + c] = Math.round(tex[i * 4 + c] * (1 - keep) + orig[i * 4 + c] * keep);
}
const png = await sharp(out, { raw: { width: SIZE, height: SIZE, channels: 4 } }).png().toBuffer();
mkdirSync('public/models/armor', { recursive: true });
writeFileSync(`../assets-src/meshy/armor-${cls}_${tier}.png`, png);
const decode = async (b) => ({ width: SIZE, height: SIZE, data: new Uint8Array(await sharp(b).ensureAlpha().raw().toBuffer()) });
const ktx = await encodeToKTX2(png, {
  isUASTC: true, needSupercompression: true, generateMipmap: true, isKTX2File: true, isPerceptual: true,
  isSetKTX2SRGBTransferFunc: true, imageDecoder: decode,
});
writeFileSync(`public/models/armor/${cls}_${tier}.ktx2`, ktx);
console.log(`${cls}_${tier}.ktx2 ${(ktx.length / 1024).toFixed(0)} KB`);
