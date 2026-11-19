// Fecha as mãos dos heróis do remaster (tunnel#181). O rig "mixamo" do Meshy não tem ossos de dedo:
// a mão é uma placa aberta presa ao osso da mão, e a arma ficava encostada na palma, sem pegada.
// Aqui os vértices dos dedos são curvados na malha (pose de descanso) em volta de um cabo imaginário,
// do nó dos dedos até a ponta, como um punho que segura algo. Continuam presos ao mesmo osso, então o
// punho acompanha qualquer animação. O LOD1 e o LOD2 usam o mesmo buffer de vértices (curvam juntos).
//
//   cd client && node scripts/make-remaster-fists.mjs public/models/{knight,barbarian,mage,rogue}_b.glb --emit src/world/hero-fists.ts
//   opções: [--curl 200] [--knuckle 0.42] [--dry] [--flip-r] [--flip-l]
//
// Idempotente: o GLB ganha asset.extras.fists e não é curvado de novo. --dry só imprime as medidas.
import { NodeIO } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';
import { writeFileSync } from 'node:fs';
import { basename } from 'node:path';

const args = process.argv.slice(2);
const opt = (k, d) => {
  const i = args.indexOf(`--${k}`);
  return i < 0 ? d : Number(args[i + 1]);
};
const DRY = args.includes('--dry');
const CURL = (opt('curl', 200) * Math.PI) / 180; // ângulo total da ponta do dedo em relação à palma
const KNUCKLE = opt('knuckle', 0.42); // onde os dedos começam, em fração do pulso até a ponta do dedo médio
const files = args.filter((a) => a.endsWith('.glb'));

const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const v3 = {
  sub: (a, b) => [a[0] - b[0], a[1] - b[1], a[2] - b[2]],
  add: (a, b) => [a[0] + b[0], a[1] + b[1], a[2] + b[2]],
  mul: (a, s) => [a[0] * s, a[1] * s, a[2] * s],
  dot: (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2],
  cross: (a, b) => [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]],
  len: (a) => Math.hypot(a[0], a[1], a[2]),
  norm: (a) => v3.mul(a, 1 / v3.len(a)),
};

// ---- matrizes 4x4 coluna-maior e quatérnios (x, y, z, w) ----
const mat = {
  mulPt: (m, p) => [m[0] * p[0] + m[4] * p[1] + m[8] * p[2] + m[12], m[1] * p[0] + m[5] * p[1] + m[9] * p[2] + m[13], m[2] * p[0] + m[6] * p[1] + m[10] * p[2] + m[14]],
  mulDir: (m, p) => [m[0] * p[0] + m[4] * p[1] + m[8] * p[2], m[1] * p[0] + m[5] * p[1] + m[9] * p[2], m[2] * p[0] + m[6] * p[1] + m[10] * p[2]],
};
const quat = {
  conj: (q) => [-q[0], -q[1], -q[2], q[3]],
  rot: (q, v) => {
    const [x, y, z, w] = q;
    const t = v3.mul(v3.cross([x, y, z], v), 2);
    return v3.add(v3.add(v, v3.mul(t, w)), v3.cross([x, y, z], t));
  },
};
// Posição do osso na pose de descanso: a translação da inversa da inverse bind matrix (coluna-maior).
function boneOrigin(ibm) {
  // inversa de uma matriz afim 4x4: R^T e -R^T t
  const r = [ibm[0], ibm[1], ibm[2], ibm[4], ibm[5], ibm[6], ibm[8], ibm[9], ibm[10]];
  const t = [ibm[12], ibm[13], ibm[14]];
  // colunas c0=(r0,r1,r2), c1=(r3,r4,r5), c2=(r6,r7,r8); R^T t = (c0·t, c1·t, c2·t)
  const c0 = [r[0], r[1], r[2]], c1 = [r[3], r[4], r[5]], c2 = [r[6], r[7], r[8]];
  // as colunas podem ter escala: divide pelo quadrado do comprimento
  return [-v3.dot(c0, t) / v3.dot(c0, c0), -v3.dot(c1, t) / v3.dot(c1, c1), -v3.dot(c2, t) / v3.dot(c2, c2)];
}

for (const file of files) {
  const doc = await io.read(file);
  const root = doc.getRoot();
  const extras = root.getAsset().extras ?? {};
  if (extras.fists && !DRY) {
    console.log(file, 'já tem as mãos fechadas; nada a fazer');
    continue;
  }
  const done = new Set();
  const frames = {};
  for (const node of root.listNodes()) {
    const skin = node.getSkin();
    const mesh = node.getMesh();
    if (!skin || !mesh) continue;
    const joints = skin.listJoints().map((j) => j.getName());
    const ibmAcc = skin.getInverseBindMatrices();
    const ibm = (name) => {
      const i = joints.indexOf(name);
      return i < 0 ? null : ibmAcc.getElement(i, new Array(16));
    };
    for (const side of ['Right', 'Left']) {
      const hand = joints.indexOf(`mixamorig:${side}Hand`);
      const tipM = ibm(`mixamorig:${side}HandMiddle4`);
      if (hand < 0) continue;
      const wrist = boneOrigin(ibm(`mixamorig:${side}Hand`));
      for (const prim of mesh.listPrimitives()) {
        const P = prim.getAttribute('POSITION'), N = prim.getAttribute('NORMAL');
        const J = prim.getAttribute('JOINTS_0'), W = prim.getAttribute('WEIGHTS_0');
        if (!P || !J || done.has(`${side}:${P.getName()}:${root.listAccessors().indexOf(P)}`)) continue;
        done.add(`${side}:${P.getName()}:${root.listAccessors().indexOf(P)}`);
        const j = [], w = [], p = [], n = [];
        const idx = [];
        for (let i = 0; i < P.getCount(); i++) {
          J.getElement(i, j);
          W.getElement(i, w);
          let on = 0;
          for (let k = 0; k < 4; k++) if (j[k] === hand || joints[j[k]] === `mixamorig:${side}HandMiddle4`) on += w[k];
          if (on >= 0.5) idx.push(i);
        }
        if (!idx.length) continue;
        const pts = idx.map((i) => P.getElement(i, []));
        // Eixo do comprimento: do pulso até a ponta do dedo médio (ou o vértice mais longe do pulso).
        let tip = tipM ? boneOrigin(tipM) : null;
        if (!tip || v3.len(v3.sub(tip, wrist)) < 1e-4) tip = pts.reduce((a, b) => (v3.len(v3.sub(b, wrist)) > v3.len(v3.sub(a, wrist)) ? b : a));
        const far = pts.reduce((a, b) => Math.max(a, v3.dot(v3.sub(b, wrist), v3.norm(v3.sub(tip, wrist)))), 0);
        const len = v3.norm(v3.sub(tip, wrist));
        const L = Math.max(far, v3.len(v3.sub(tip, wrist)));
        // Largura da palma: maior variância perpendicular ao comprimento (iteração de potência 3x3).
        const perp = pts.map((q) => {
          const r = v3.sub(q, wrist);
          return v3.sub(r, v3.mul(len, v3.dot(r, len)));
        });
        const mean = v3.mul(perp.reduce(v3.add, [0, 0, 0]), 1 / perp.length);
        const C = [0, 0, 0, 0, 0, 0, 0, 0, 0];
        for (const q of perp) {
          const d = v3.sub(q, mean);
          for (let a = 0; a < 3; a++) for (let b = 0; b < 3; b++) C[a * 3 + b] += d[a] * d[b];
        }
        let wdir = v3.cross(len, [0, 1, 0]);
        if (v3.len(wdir) < 1e-3) wdir = v3.cross(len, [1, 0, 0]);
        for (let it = 0; it < 50; it++) {
          const m = [0, 1, 2].map((a) => C[a * 3] * wdir[0] + C[a * 3 + 1] * wdir[1] + C[a * 3 + 2] * wdir[2]);
          wdir = v3.norm(v3.sub(m, v3.mul(len, v3.dot(m, len))));
        }
        // O cabo sai do punho para a frente do corpo (+Z na pose de descanso do glTF): a lâmina aponta
        // para a frente com o braço solto, como quem segura uma espada.
        if (wdir[2] < 0) wdir = v3.mul(wdir, -1);
        // Normal da palma: perpendicular ao comprimento e à largura. Na pose de descanso (A-pose) a
        // palma olha para o corpo: o lado que aponta para o eixo do corpo (x = 0) é o da palma.
        // --flip-r / --flip-l invertem a regra numa das mãos, se o modelo vier diferente.
        let nrm = v3.norm(v3.cross(len, wdir));
        const inward = v3.norm([-wrist[0], 0, -wrist[2] * 0]);
        if (v3.dot(nrm, inward) < 0) nrm = v3.mul(nrm, -1);
        if (args.includes(`--flip-${side === 'Right' ? 'r' : 'l'}`)) nrm = v3.mul(nrm, -1);
        const tk = KNUCKLE * L;
        const knuckle = v3.add(wrist, v3.mul(len, tk));
        const Lf = L - tk;
        const R = Lf / CURL;
        // Pega: o centro do punho e o eixo do cabo (largura da palma) no espaço do encaixe handslot,
        // já desfeita a rotação SLOT_INV do jogo (models.ts). É o "pos" da WEAPON_GRIPS.
        {
          const slot = root.listNodes().find((x) => x.getName() === `handslot.${side === 'Right' ? 'r' : 'l'}`);
          const handIbm = ibm(`mixamorig:${side}Hand`);
          if (slot && handIbm) {
            const widthMid = v3.dot(mean, wdir);
            const fist = v3.add(v3.add(knuckle, v3.mul(nrm, R * 0.55)), v3.mul(wdir, widthMid));
            const toSlot = (pt, dir) => {
              let b = dir ? mat.mulDir(handIbm, pt) : mat.mulPt(handIbm, pt);
              const t = slot.getTranslation(), r = slot.getRotation(), sc = slot.getScale();
              if (!dir) b = v3.sub(b, t);
              b = quat.rot(quat.conj(r), b);
              b = [b[0] / sc[0], b[1] / sc[1], b[2] / sc[2]];
              const slotInv = [-0.5, -0.5, -0.5, 0.5];
              return quat.rot(quat.conj(slotInv), b);
            };
            const f = (a) => a.map((x) => +x.toFixed(3));
            frames[side === 'Right' ? 'r' : 'l'] = { pos: f(toSlot(fist)), axis: f(v3.norm(toSlot(wdir, true))), fingers: f(v3.norm(toSlot(len, true))) };
            console.log(`  pega ${side}: pos ${JSON.stringify(f(toSlot(fist)))}, cabo (largura) ${JSON.stringify(f(v3.norm(toSlot(wdir, true))))}, dedos ${JSON.stringify(f(v3.norm(toSlot(len, true))))}, palma ${JSON.stringify(f(v3.norm(toSlot(nrm, true))))}`);
          }
        }
        let moved = 0;
        for (const i of idx) {
          P.getElement(i, p);
          const rel = v3.sub(p, wrist);
          const t = v3.dot(rel, len);
          if (t <= tk) continue;
          const u = t - tk;
          const v = v3.dot(rel, nrm);
          const wc = v3.dot(rel, wdir);
          const th = u / R;
          const r = R - v;
          const center = v3.add(v3.add(knuckle, v3.mul(wdir, wc)), v3.mul(nrm, R));
          const np = v3.add(center, v3.add(v3.mul(len, r * Math.sin(th)), v3.mul(nrm, -r * Math.cos(th))));
          if (!DRY) P.setElement(i, np);
          if (N && !DRY) {
            N.getElement(i, n);
            const a = v3.dot(n, len), b = v3.dot(n, nrm), c = v3.dot(n, wdir);
            const a2 = a * Math.cos(th) - b * Math.sin(th);
            const b2 = a * Math.sin(th) + b * Math.cos(th);
            N.setElement(i, v3.norm(v3.add(v3.add(v3.mul(len, a2), v3.mul(nrm, b2)), v3.mul(wdir, c))));
          }
          moved++;
        }
        console.log(`${file} ${node.getName()} ${side}: ${idx.length} vértices da mão, ${moved} curvados; mão ${(L * 100).toFixed(1)} cm, dedos ${(Lf * 100).toFixed(1)} cm, raio do punho ${(R * 100).toFixed(1)} cm`);
      }
    }
  }
  if (DRY) continue;
  root.getAsset().extras = { ...extras, fists: { curl: (CURL * 180) / Math.PI, knuckle: KNUCKLE, frames } };
  await io.write(file, doc);
}

// --emit <arquivo.ts>: grava o quadro do punho de cada herói (lido de asset.extras.fists dos GLB dados),
// que o jogo usa para pôr o cabo das armas dentro da mão fechada (models.ts, HERO_FISTS).
const emitAt = args.indexOf('--emit');
if (emitAt >= 0) {
  const out = {};
  for (const file of files) {
    const doc = await io.read(file);
    const fr = doc.getRoot().getAsset().extras?.fists?.frames;
    if (!fr) throw new Error(`${file} não tem as mãos fechadas (rode o script sem --emit antes)`);
    out[basename(file).replace(/_b\.glb$/, '').replace(/\.glb$/, '')] = fr;
  }
  const body = Object.entries(out).map(([m, fr]) => `  ${m}: ${JSON.stringify(fr)},`).join('\n');
  writeFileSync(args[emitAt + 1], `// Gerado por client/scripts/make-remaster-fists.mjs (não editar à mão).
// Punho fechado de cada herói do remaster, no espaço do encaixe handslot (sem a rotação SLOT_INV):
// pos = centro do punho, axis = direção do cabo (para a frente do corpo), fingers = direção dos dedos.
export interface FistFrame { pos: [number, number, number]; axis: [number, number, number]; fingers: [number, number, number] }
export const HERO_FISTS: Record<string, { r?: FistFrame; l?: FistFrame }> = {
${body}
};
`);
  console.log('gravado', args[emitAt + 1]);
}
