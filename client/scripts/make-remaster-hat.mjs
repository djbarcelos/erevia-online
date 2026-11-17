// Monta o elmo "Hat" do cavaleiro do remaster (tunnel#94) e o junta ao cavaleiro SEM elmo.
//
// O Meshy não separa o elmo da cabeça. O primeiro cavaleiro (v3, com elmo aberto) tem a cabeça numa
// ilha de triângulos só; aqui: (1) pega essa ilha; (2) fica com os triângulos de metal (aço e dourado
// do elmo), tirando os de pele e cabelo, olhando a cor da textura no centro de cada triângulo;
// (3) troca a textura por cor de vértice (o elmo é aço liso com borda dourada, não precisa do atlas);
// (4) prende o elmo rigidamente no osso da cabeça do cavaleiro novo, deslocado pela diferença entre os
// ossos da cabeça dos dois rigs. A cabeça sem elmo continua no corpo; "esconder chapéu" liga e desliga o nó Hat.
//
// Uso (dentro de client/): node scripts/make-remaster-hat.mjs <v3_raw.glb> <bare_raw.glb> <saida.glb> [--scale=1] [--dy=0] [--dz=0]
import { NodeIO } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';
import { PNG } from 'pngjs';
import jpeg from 'jpeg-js';

const args = process.argv.slice(2).filter((a) => !a.startsWith('--'));
const opt = Object.fromEntries(process.argv.slice(2).filter((a) => a.startsWith('--')).map((a) => a.slice(2).split('=')));
const [oldPath, barePath, outPath] = args;
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const old = await io.read(oldPath);
const bare = await io.read(barePath);

const decode = (buf) => (buf[0] === 0x89 ? PNG.sync.read(Buffer.from(buf)) : { ...jpeg.decode(Buffer.from(buf), { useTArray: true, formatAsRGBA: true }) });
const meshNode = (d) => d.getRoot().listNodes().find((n) => n.getMesh() && n.getSkin());

function invert4(m) {
  // inversa de uma matriz 4x4 (coluna-maior) por Gauss-Jordan
  const a = Float64Array.from(m);
  const b = new Float64Array(16);
  for (let i = 0; i < 4; i++) b[i * 5] = 1;
  for (let c = 0; c < 4; c++) {
    let p = c;
    for (let r = c + 1; r < 4; r++) if (Math.abs(a[c * 4 + r]) > Math.abs(a[c * 4 + p])) p = r;
    if (p !== c) for (let k = 0; k < 4; k++) { [a[k * 4 + c], a[k * 4 + p]] = [a[k * 4 + p], a[k * 4 + c]]; [b[k * 4 + c], b[k * 4 + p]] = [b[k * 4 + p], b[k * 4 + c]]; }
    const d = a[c * 4 + c];
    for (let k = 0; k < 4; k++) { a[k * 4 + c] /= d; b[k * 4 + c] /= d; }
    for (let r = 0; r < 4; r++) {
      if (r === c) continue;
      const f = a[c * 4 + r];
      for (let k = 0; k < 4; k++) { a[k * 4 + r] -= f * a[k * 4 + c]; b[k * 4 + r] -= f * b[k * 4 + c]; }
    }
  }
  return b;
}
const jointPos = (d, name) => {
  const skin = meshNode(d).getSkin();
  const j = skin.listJoints().findIndex((n) => n.getName() === name);
  const inv = invert4(skin.getInverseBindMatrices().getElement(j, new Array(16)));
  return [inv[12], inv[13], inv[14]]; // posição do osso em repouso
};

const on = meshNode(old);
const prim = on.getMesh().listPrimitives()[0];
const P = prim.getAttribute('POSITION'), N = prim.getAttribute('NORMAL'), UV = prim.getAttribute('TEXCOORD_0');
const idx = prim.getIndices().getArray();
const img = decode(prim.getMaterial().getBaseColorTexture().getImage());
const px = (u, v) => {
  const x = Math.min(img.width - 1, Math.max(0, Math.floor(((u % 1) + 1) % 1 * img.width)));
  const y = Math.min(img.height - 1, Math.max(0, Math.floor(((v % 1) + 1) % 1 * img.height)));
  const o = (y * img.width + x) * 4;
  return [img.data[o], img.data[o + 1], img.data[o + 2]];
};

// ilhas (vértices na mesma posição valem um só)
const key = (i) => P.getElement(i, [0, 0, 0]).map((x) => Math.round(x * 1e4)).join(',');
const rep = new Map();
const parent = [];
const find = (a) => { while (parent[a] !== a) { parent[a] = parent[parent[a]]; a = parent[a]; } return a; };
for (let i = 0; i < P.getCount(); i++) { const k = key(i); if (!rep.has(k)) rep.set(k, i); parent[i] = i; }
for (let t = 0; t < idx.length; t += 3) {
  for (let k = 1; k < 3; k++) {
    const a = find(rep.get(key(idx[t])));
    const b = find(rep.get(key(idx[t + k])));
    if (a !== b) parent[a] = b;
  }
}
const isl = new Map();
for (let t = 0; t < idx.length; t += 3) {
  const r = find(rep.get(key(idx[t])));
  if (!isl.has(r)) isl.set(r, { tris: [], maxY: -1e9 });
  const e = isl.get(r);
  e.tris.push(t);
  for (let k = 0; k < 3; k++) e.maxY = Math.max(e.maxY, P.getElement(idx[t + k], [0, 0, 0])[1]);
}
const head = [...isl.values()].sort((a, b) => b.maxY - a.maxY)[0];
console.log(`ilha da cabeça: ${head.tris.length} triângulos, topo y=${head.maxY.toFixed(3)}`);

const rgb2hsv = ([r, g, b]) => {
  r /= 255; g /= 255; b /= 255;
  const mx = Math.max(r, g, b), mn = Math.min(r, g, b), d = mx - mn;
  let h = 0;
  if (d) h = mx === r ? ((g - b) / d + 6) % 6 : mx === g ? (b - r) / d + 2 : (r - g) / d + 4;
  return [h * 60, mx ? d / mx : 0, mx];
};
const metal = (c) => { const [h, s, v] = rgb2hsv(c); return (s < 0.3 && v > 0.1) || (h > 38 && h < 62 && s > 0.45 && v > 0.45); };
// acima da testa (osso da cabeça + --top, em y) tudo é elmo, mesmo o que a textura pinta de cabelo
const [, headY, headZ] = jointPos(old, 'mixamorig:Head');
const top = headY + +(opt.top ?? 0.08);
const hat = [];
const forced = new Set();
for (const t of head.tris) {
  const cy = (P.getElement(idx[t], [0, 0, 0])[1] + P.getElement(idx[t + 1], [0, 0, 0])[1] + P.getElement(idx[t + 2], [0, 0, 0])[1]) / 3;
  const cz = (P.getElement(idx[t], [0, 0, 0])[2] + P.getElement(idx[t + 1], [0, 0, 0])[2] + P.getElement(idx[t + 2], [0, 0, 0])[2]) / 3;
  // a nuca também é elmo (o rosto está na frente, em +z)
  if (cy > top || (cz < headZ - 0.005 && cy > headY - 0.1)) { hat.push(t); forced.add(t); continue; }
  let votes = 0;
  const c = [0, 1, 2].map((k) => UV.getElement(idx[t + k], [0, 0]));
  for (const uv of c) if (metal(px(uv[0], uv[1]))) votes++;
  if (metal(px((c[0][0] + c[1][0] + c[2][0]) / 3, (c[0][1] + c[1][1] + c[2][1]) / 3))) votes += 2;
  if (votes >= 3) hat.push(t);
}
console.log(`elmo: ${hat.length} de ${head.tris.length} triângulos`);

// geometria do elmo, deslocada para o cavaleiro novo
const dOld = jointPos(old, 'mixamorig:Head');
const dNew = jointPos(bare, 'mixamorig:Head');
const sc = +(opt.scale ?? 1), dy = +(opt.dy ?? 0), dz = +(opt.dz ?? 0);
const off = [dNew[0] - dOld[0], dNew[1] - dOld[1] + dy, dNew[2] - dOld[2] + dz];
console.log('osso da cabeça antigo', dOld.map((x) => x.toFixed(3)).join(' '), 'novo', dNew.map((x) => x.toFixed(3)).join(' '));
const remap = new Map();
const pos = [], nor = [], col = [], ind = [];
const lin = (c) => { c /= 255; return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; };
for (const t of hat) {
  for (let k = 0; k < 3; k++) {
    const i = idx[t + k];
    if (!remap.has(i)) {
      remap.set(i, pos.length / 3);
      const p = P.getElement(i, [0, 0, 0]), n = N.getElement(i, [0, 0, 0]), uv = UV.getElement(i, [0, 0]);
      // atrás do centro da cabeça o elmo cresce mais (--zback): o cabelo do cavaleiro novo é mais volumoso na nuca
      const zs = p[2] < dOld[2] ? sc * +(opt.zback ?? 1) : sc;
      pos.push(dOld[0] + (p[0] - dOld[0]) * sc * +(opt.xs ?? 1) + off[0], dOld[1] + (p[1] - dOld[1]) * sc + off[1], dOld[2] + (p[2] - dOld[2]) * zs + off[2]);
      nor.push(...n);
      let c = px(uv[0], uv[1]);
      if (!metal(c) && (p[1] > top || p[2] < headZ - 0.005)) c = [150, 158, 172]; // cabelo sob o elmo: aço
      col.push(lin(c[0]), lin(c[1]), lin(c[2]), 1);
    }
    ind.push(remap.get(i));
  }
}
const root = bare.getRoot();
const buffer = root.listBuffers()[0];
const bn = meshNode(bare);
const skin = bn.getSkin();
const headJoint = skin.listJoints().findIndex((n) => n.getName() === 'mixamorig:Head');
const n = pos.length / 3;
const acc = (type, arr) => bare.createAccessor().setType(type).setArray(arr).setBuffer(buffer);
const joints = new Uint16Array(n * 4);
const weights = new Float32Array(n * 4);
for (let i = 0; i < n; i++) { joints[i * 4] = headJoint; weights[i * 4] = 1; }
const mat = bare.createMaterial('Hat_steel').setBaseColorFactor([1, 1, 1, 1]).setRoughnessFactor(0.55).setMetallicFactor(0);
const hp = bare.createPrimitive().setMaterial(mat)
  .setAttribute('POSITION', acc('VEC3', new Float32Array(pos)))
  .setAttribute('NORMAL', acc('VEC3', new Float32Array(nor)))
  .setAttribute('COLOR_0', acc('VEC4', new Float32Array(col)))
  .setAttribute('JOINTS_0', acc('VEC4', joints))
  .setAttribute('WEIGHTS_0', acc('VEC4', weights))
  .setIndices(acc('SCALAR', new Uint16Array(ind)));
const mesh = bare.createMesh('Hat').addPrimitive(hp);
const hn = bare.createNode('Hat').setMesh(mesh).setSkin(skin);
const par = bn.getParentNode();
if (par) par.addChild(hn);
else for (const s of root.listScenes()) s.addChild(hn);
await io.write(outPath, bare);
console.log(`${outPath}: elmo com ${ind.length / 3} triângulos`);
