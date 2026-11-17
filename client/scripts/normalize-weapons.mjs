// Põe cada GLB do Meshy na convenção das armas KayKit (docs/arte/remaster-estaticos.md): eixo Y para
// a ponta, cabo na origem (X e Z no cabo, o fundo do cabo no mesmo Y do KayKit), machados com a
// lâmina para -X, escudos centrados em XY com a face para +Z. Grava a transformação direto nos vértices.
// Uso (dentro de client/): node scripts/normalize-weapons.mjs <saida-dir> [regex dos nomes]
import { NodeIO } from '@gltf-transform/core';
import { simplify } from '@gltf-transform/functions';
import { MeshoptSimplifier } from 'meshoptimizer';
import { readFileSync, mkdirSync } from 'node:fs';
import { resolve } from 'node:path';

const OUT = resolve(process.argv[2]);
mkdirSync(OUT, { recursive: true });
const root = resolve(import.meta.dirname, '../../assets-src/meshy');
const ids = {};
for (const l of readFileSync(`${root}/tasks.jsonl`, 'utf8').split(/\r?\n/).filter(Boolean)) {
  const r = JSON.parse(l);
  if (r.endpoint === 'image-to-3d' && r.name) ids[r.name] = r.id.slice(-8);
}
// alvo = bbox do KayKit equivalente: ymin (fundo do cabo) e comprimento; escudos: diâmetro e zback.
const CFG = {
  sword: { ymin: -0.366, len: 1.775, kind: 'weapon' },           // sword_1handed
  sword_high: { ymin: -0.401, len: 2.365, kind: 'weapon' },      // sword_2handed
  axe: { ymin: -0.273, len: 1.244, kind: 'axe' },                // axe_1handed
  axe_high: { ymin: -0.433, len: 1.725, kind: 'axe' },           // axe_2handed
  staff: { ymin: -0.9, len: 2.154, kind: 'weapon' },             // staff
  staff_high: { ymin: -0.846, len: 2.101, kind: 'weapon' },      // skeleton_staff
  wand: { ymin: -0.266, len: 0.966, kind: 'weapon' },            // wand
  dagger: { ymin: -0.225, len: 1.206, kind: 'weapon' },          // dagger
  shield: { diam: 0.882, zback: -0.145, kind: 'shield' },        // shield_round_barbarian
  shield_high: { diam: 1.19, zback: -0.145, kind: 'shield' },    // shield_square (altura 1,19)
  // Parte 2 (grau 3 = forma rara, e a forma alta da varinha e da adaga): comprimentos entre a comum e a alta.
  sword_rare: { ymin: -0.366, len: 2.0, kind: 'weapon' },
  axe_rare: { ymin: -0.273, len: 1.45, kind: 'axe' },
  staff_rare: { ymin: -0.9, len: 2.13, kind: 'weapon' },
  wand_rare: { ymin: -0.266, len: 0.966, kind: 'weapon' },
  wand_high: { ymin: -0.266, len: 1.1, kind: 'weapon' },
  dagger_rare: { ymin: -0.225, len: 1.206, kind: 'weapon' },
  dagger_high: { ymin: -0.225, len: 1.35, kind: 'weapon' },
  shield_rare: { diam: 1.0, zback: -0.145, kind: 'shield' },
};
const io = new NodeIO();
const only = new RegExp(process.argv[3] ?? '.');
for (const [name, c] of Object.entries(CFG)) {
  if (!only.test(name)) continue;
  const dir = `${root}/image-to-3d-${ids[name]}`;
  const doc = await io.read(`${dir}/glb-model.glb`);
  const prims = [];
  doc.getRoot().listScenes()[0].traverse((n) => { if (n.getMesh()) for (const p of n.getMesh().listPrimitives()) prims.push([n, p]); });
  // posições no espaço do mundo
  const pts = [];
  for (const [n, p] of prims) {
    const w = n.getWorldMatrix(); const a = p.getAttribute('POSITION');
    for (let i = 0; i < a.getCount(); i++) { const v = a.getElement(i, [0, 0, 0]);
      pts.push([w[0]*v[0]+w[4]*v[1]+w[8]*v[2]+w[12], w[1]*v[0]+w[5]*v[1]+w[9]*v[2]+w[13], w[2]*v[0]+w[6]*v[1]+w[10]*v[2]+w[14]]); }
  }
  const bb = (k) => [Math.min(...pts.map((p) => p[k])), Math.max(...pts.map((p) => p[k]))];
  const [x0, x1] = bb(0), [y0, y1] = bb(1), [z0, z1] = bb(2);
  let s, tx, ty, tz, flip = false;
  if (c.kind === 'shield') {
    s = c.diam / Math.max(x1 - x0, y1 - y0);
    tx = -(x0 + x1) / 2 * s; ty = -(y0 + y1) / 2 * s; tz = c.zback - z0 * s;
  } else {
    s = c.len / (y1 - y0);
    const low = pts.filter((p) => p[1] < y0 + 0.12 * (y1 - y0));
    const hx = low.reduce((a, p) => a + p[0], 0) / low.length, hz = low.reduce((a, p) => a + p[2], 0) / low.length;
    if (c.kind === 'axe') { const top = pts.filter((p) => p[1] > y0 + 0.6 * (y1 - y0)); flip = top.reduce((a, p) => a + p[0], 0) / top.length > hx; }
    tx = -hx * s; tz = -hz * s; ty = c.ymin - y0 * s;
    if (flip) { tx = hx * s; tz = hz * s; }
  }
  const world = new Map(prims.map(([n]) => [n, n.getWorldMatrix()]));
  for (const [n, p] of prims) {
    const w = world.get(n);
    const a = p.getAttribute('POSITION');
    for (let i = 0; i < a.getCount(); i++) {
      const v = a.getElement(i, [0, 0, 0]);
      let X = w[0]*v[0]+w[4]*v[1]+w[8]*v[2]+w[12], Y = w[1]*v[0]+w[5]*v[1]+w[9]*v[2]+w[13], Z = w[2]*v[0]+w[6]*v[1]+w[10]*v[2]+w[14];
      if (flip) { X = -X; Z = -Z; }
      a.setElement(i, [X * s + tx, Y * s + ty, Z * s + tz]);
    }
    const nm = p.getAttribute('NORMAL');
    if (nm) for (let i = 0; i < nm.getCount(); i++) { const v = nm.getElement(i, [0, 0, 0]); const X = w[0]*v[0]+w[4]*v[1]+w[8]*v[2], Y = w[1]*v[0]+w[5]*v[1]+w[9]*v[2], Z = w[2]*v[0]+w[6]*v[1]+w[10]*v[2]; nm.setElement(i, flip ? [-X, Y, -Z] : [X, Y, Z]); }
  }
  for (const [n] of prims) { n.setTranslation([0, 0, 0]); n.setRotation([0, 0, 0, 1]); n.setScale([1, 1, 1]); }
  // uma raiz limpa: filhos de nós com transformação já tiveram o mundo gravado nos vértices
  for (const n of doc.getRoot().listNodes()) { n.setTranslation([0, 0, 0]); n.setRotation([0, 0, 0, 1]); n.setScale([1, 1, 1]); }
  // Meshy erra o alvo de triângulos em ~4%: o que passa de 780 é simplificado para caber nos 800 do guia.
  let tris = 0;
  for (const [, p] of prims) tris += (p.getIndices()?.getCount() ?? 0) / 3;
  if (tris > 780) { await MeshoptSimplifier.ready; await doc.transform(simplify({ simplifier: MeshoptSimplifier, ratio: 760 / tris, error: 0.01 })); console.log('  simplificado de', tris); }
  await io.write(`${OUT}/${name}_b.glb`, doc);
  console.log(name, 'escala', s.toFixed(3), flip ? 'espelhado' : '');
}
