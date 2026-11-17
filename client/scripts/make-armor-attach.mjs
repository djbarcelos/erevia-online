// Calcula onde cada peça anexada (ombreiras, capa, elmo) fica no osso de cada classe (tunnel#96) e grava
// client/src/world/armor-attach.ts. A peça é posta no espaço do modelo (glTF, pose de descanso) a partir das
// medidas do corpo (largura dos ombros, altura da cabeça, caixa do `Hat`) e convertida para o espaço local do
// osso, então a peça acompanha o osso em qualquer animação sem eixo nenhum calibrado à mão.
//   cd client && node ../client/scripts/make-armor-attach.mjs
import { NodeIO } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';
import { writeFileSync } from 'node:fs';

const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
// ---- matrizes 4x4 (coluna-maior, como o glTF) ----
const mul = (a, b) => {
  const o = new Array(16).fill(0);
  for (let c = 0; c < 4; c++) for (let r = 0; r < 4; r++) for (let k = 0; k < 4; k++) o[c * 4 + r] += a[k * 4 + r] * b[c * 4 + k];
  return o;
};
const trs = (t, q, s) => {
  const [x, y, z, w] = q;
  const r = [
    1 - 2 * (y * y + z * z), 2 * (x * y + z * w), 2 * (x * z - y * w), 0,
    2 * (x * y - z * w), 1 - 2 * (x * x + z * z), 2 * (y * z + x * w), 0,
    2 * (x * z + y * w), 2 * (y * z - x * w), 1 - 2 * (x * x + y * y), 0,
    0, 0, 0, 1,
  ];
  for (let c = 0; c < 3; c++) for (let i = 0; i < 3; i++) r[c * 4 + i] *= s[c];
  r[12] = t[0];
  r[13] = t[1];
  r[14] = t[2];
  return r;
};
function inv(a) {
  const o = new Array(16);
  const [a00, a01, a02, a03, a10, a11, a12, a13, a20, a21, a22, a23, a30, a31, a32, a33] = a;
  const b00 = a00 * a11 - a01 * a10, b01 = a00 * a12 - a02 * a10, b02 = a00 * a13 - a03 * a10, b03 = a01 * a12 - a02 * a11, b04 = a01 * a13 - a03 * a11, b05 = a02 * a13 - a03 * a12;
  const b06 = a20 * a31 - a21 * a30, b07 = a20 * a32 - a22 * a30, b08 = a20 * a33 - a23 * a30, b09 = a21 * a32 - a22 * a31, b10 = a21 * a33 - a23 * a31, b11 = a22 * a33 - a23 * a32;
  const d = 1 / (b00 * b11 - b01 * b10 + b02 * b09 + b03 * b08 - b04 * b07 + b05 * b06);
  o[0] = (a11 * b11 - a12 * b10 + a13 * b09) * d; o[1] = (a02 * b10 - a01 * b11 - a03 * b09) * d; o[2] = (a31 * b05 - a32 * b04 + a33 * b03) * d; o[3] = (a22 * b04 - a21 * b05 - a23 * b03) * d;
  o[4] = (a12 * b08 - a10 * b11 - a13 * b07) * d; o[5] = (a00 * b11 - a02 * b08 + a03 * b07) * d; o[6] = (a32 * b02 - a30 * b05 - a33 * b01) * d; o[7] = (a20 * b05 - a22 * b02 + a23 * b01) * d;
  o[8] = (a10 * b10 - a11 * b08 + a13 * b06) * d; o[9] = (a01 * b08 - a00 * b10 - a03 * b06) * d; o[10] = (a30 * b04 - a31 * b02 + a33 * b00) * d; o[11] = (a21 * b02 - a20 * b04 - a23 * b00) * d;
  o[12] = (a11 * b07 - a10 * b09 - a12 * b06) * d; o[13] = (a00 * b09 - a01 * b07 + a02 * b06) * d; o[14] = (a31 * b01 - a30 * b03 - a32 * b00) * d; o[15] = (a20 * b03 - a21 * b01 + a22 * b00) * d;
  return o;
}
function decompose(m) {
  const sx = Math.hypot(m[0], m[1], m[2]), sy = Math.hypot(m[4], m[5], m[6]), sz = Math.hypot(m[8], m[9], m[10]);
  // r[col*3+row]
  const r = [m[0] / sx, m[1] / sx, m[2] / sx, m[4] / sy, m[5] / sy, m[6] / sy, m[8] / sz, m[9] / sz, m[10] / sz];
  const m00 = r[0], m10 = r[1], m20 = r[2], m01 = r[3], m11 = r[4], m21 = r[5], m02 = r[6], m12 = r[7], m22 = r[8];
  const tr = m00 + m11 + m22;
  let q;
  if (tr > 0) {
    const s = Math.sqrt(tr + 1) * 2;
    q = [(m21 - m12) / s, (m02 - m20) / s, (m10 - m01) / s, s / 4];
  } else if (m00 > m11 && m00 > m22) {
    const s = Math.sqrt(1 + m00 - m11 - m22) * 2;
    q = [s / 4, (m01 + m10) / s, (m02 + m20) / s, (m21 - m12) / s];
  } else if (m11 > m22) {
    const s = Math.sqrt(1 + m11 - m00 - m22) * 2;
    q = [(m01 + m10) / s, s / 4, (m12 + m21) / s, (m02 - m20) / s];
  } else {
    const s = Math.sqrt(1 + m22 - m00 - m11) * 2;
    q = [(m02 + m20) / s, (m12 + m21) / s, s / 4, (m10 - m01) / s];
  }
  return { pos: [m[12], m[13], m[14]], rot: q, scale: [sx, sy, sz] };
}
const apply = (m, p) => [
  m[0] * p[0] + m[4] * p[1] + m[8] * p[2] + m[12],
  m[1] * p[0] + m[5] * p[1] + m[9] * p[2] + m[13],
  m[2] * p[0] + m[6] * p[1] + m[10] * p[2] + m[14],
];

/** Caixa (espaço do modelo) das malhas cujo nome casa; usa a posição do vértice no repouso (sem pele). */
function bbox(doc, re) {
  const mn = [1e9, 1e9, 1e9];
  const mx = [-1e9, -1e9, -1e9];
  for (const node of doc.getRoot().listNodes()) {
    const mesh = node.getMesh();
    if (!mesh || !re.test(node.getName()) || /_LOD/.test(node.getName())) continue;
    const w = node.getWorldMatrix();
    for (const prim of mesh.listPrimitives()) {
      const pos = prim.getAttribute('POSITION');
      const v = [0, 0, 0];
      for (let i = 0; i < pos.getCount(); i++) {
        pos.getElement(i, v);
        const p = apply(w, v);
        for (let k = 0; k < 3; k++) {
          mn[k] = Math.min(mn[k], p[k]);
          mx[k] = Math.max(mx[k], p[k]);
        }
      }
    }
  }
  return { mn, mx, c: mn.map((x, k) => (x + mx[k]) / 2), s: mx.map((x, k) => x - mn[k]) };
}

const CLASSES = ['knight', 'barbarian', 'mage', 'rogue'];
const PIECES = {
  pauldron_epic: 'spine', pauldron_legend: 'spine', cape_rare: 'spine', cape_epic: 'spine', cape_legend: 'spine',
  helm_knight_rare: 'head', helm_knight_epic: 'head', hat_mage_epic: 'head', hood_rogue_epic: 'head', helm_barbarian_epic: 'head',
};
const OWNER = { helm_knight: 'knight', hat_mage: 'mage', hood_rogue: 'rogue', helm_barbarian: 'barbarian' };
// Ajustes finos (medidos olhando o laboratório): por classe e peça.
const TWEAK = { 'rogue.hood_rogue_epic': { k: 0.72, dy: -0.045 }, ...JSON.parse(process.env.ARMOR_TWEAK ?? '{}') };
const YAW = { helm_knight_rare: 0, helm_knight_epic: 0, hat_mage_epic: 0, hood_rogue_epic: 0, helm_barbarian_epic: 0, pauldron_epic: 0, pauldron_legend: 0, cape_rare: 0, cape_epic: 0, cape_legend: 0 };
const out = {};
for (const cls of CLASSES) {
  const body = await io.read(`public/models/${cls}_b.glb`);
  const node = (n) => body.getRoot().listNodes().find((x) => x.getName() === n);
  const wp = (n) => {
    const m = node(n).getWorldMatrix();
    return [m[12], m[13], m[14]];
  };
  const lA = wp('mixamorig:LeftArm');
  const rA = wp('mixamorig:RightArm');
  const shoulderMid = lA.map((v, k) => (v + rA[k]) / 2);
  const shoulderW = Math.hypot(lA[0] - rA[0], lA[1] - rA[1], lA[2] - rA[2]);
  const head = wp('mixamorig:Head');
  const top = wp('mixamorig:HeadTop_End');
  const headH = top[1] - head[1];
  const hat = ['knight', 'mage'].includes(cls) ? bbox(body, /^Hat$/) : null;
  const spineM = node('mixamorig:Spine2').getWorldMatrix();
  const headM = node('mixamorig:Head').getWorldMatrix();
  console.log(cls, 'ombros', shoulderW.toFixed(3), 'y', shoulderMid[1].toFixed(3), 'cabeça', head.map((v) => v.toFixed(3)).join(','), 'altura', headH.toFixed(3), hat ? `hat ${hat.s.map((v) => v.toFixed(2))} c ${hat.c.map((v) => v.toFixed(2))}` : '');
  out[cls] = {};
  for (const [file, kind] of Object.entries(PIECES)) {
    const owner = Object.entries(OWNER).find(([p]) => file.startsWith(p));
    if (owner && owner[1] !== cls) continue;
    const pdoc = await io.read(`public/models/armor/${file}.glb`);
    const b = bbox(pdoc, /(?:)/);
    const tw = TWEAK[`${cls}.${file}`] ?? {};
    let target;
    // Meia volta (graus) por peça, conferida no laboratório: a frente de cada objeto do Meshy cai num lado diferente.
    let yaw = ((YAW[file] ?? 0) * Math.PI) / 180;
    if (file.startsWith('pauldron')) {
      const wide = cls === 'barbarian' ? 0.34 : cls === 'mage' ? 0.2 : 0.26;
      const k = (shoulderW + wide) / b.s[0];
      target = { c: [shoulderMid[0], shoulderMid[1] - b.s[1] * k * 0.3, shoulderMid[2] + 0.02], k };
    } else if (file.startsWith('cape')) {
      const hgt = file === 'cape_rare' ? 0.85 : file === 'cape_epic' ? 1.15 : 1.3;
      const k = hgt / b.s[1];
      const topY = shoulderMid[1] + 0.03;
      target = { c: [shoulderMid[0], topY - hgt / 2, shoulderMid[2] - (cls === 'barbarian' ? 0.2 : 0.17)], k };
    } else if (hat) {
      // elmo ou chapéu no lugar do Hat do corpo: mesma largura e base
      const mage = file.startsWith('hat_mage');
      const k = (mage ? hat.s[0] * 1.0 : hat.s[0] * 1.15) / b.s[0];
      target = { c: [hat.c[0], hat.mn[1] + (b.s[1] * k) / 2 - (mage ? 0.02 : 0), hat.c[2]], k };
    } else {
      const k = (cls === 'barbarian' ? headH * 2.0 : headH * 2.1) / b.s[1];
      target = { c: [head[0], head[1] + headH * 0.5 + (cls === 'barbarian' ? 0.06 : 0.02), head[2]], k };
    }
    if (tw.k) target.k *= tw.k;
    if (tw.dy) target.c[1] += tw.dy;
    if (tw.dz) target.c[2] += tw.dz;
    const cosy = Math.cos(yaw / 2);
    const siny = Math.sin(yaw / 2);
    const model = mul(trs(target.c, [0, siny, 0, cosy], [target.k, target.k, target.k]), trs(b.c.map((v) => -v), [0, 0, 0, 1], [1, 1, 1]));
    const boneM = kind === 'spine' ? spineM : headM;
    const d = decompose(mul(inv(boneM), model));
    const r = (v, n = 4) => Number(v.toFixed(n));
    out[cls][file] = {
      bone: kind === 'spine' ? 'mixamorig:Spine2' : 'mixamorig:Head',
      pos: d.pos.map((v) => r(v)),
      rot: d.rot.map((v) => r(v)),
      scale: r((d.scale[0] + d.scale[1] + d.scale[2]) / 3, 5),
    };
  }
}
const lines = [
  '// Gerado por client/scripts/make-armor-attach.mjs (não editar à mão): onde cada peça de armadura anexada',
  '// fica no osso de cada classe (posição, rotação e escala locais do osso), calculado no repouso.',
  '',
  'export interface ArmorAttach {',
  '  bone: string;',
  '  pos: [number, number, number];',
  '  rot: [number, number, number, number];',
  '  scale: number;',
  '}',
  '',
  'export const ARMOR_ATTACH: Record<string, Record<string, ArmorAttach>> = ' + JSON.stringify(out, null, 2).replace(/"(\w+)":/g, '$1:') + ';',
  '',
];
writeFileSync('src/world/armor-attach.ts', lines.join('\n'));
