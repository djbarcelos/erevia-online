// Monta o GLB cru de um monstro do remaster (tunnel#98) a partir dos arquivos do Meshy, como o
// build-remaster-v3.mjs, mas guiado por uma configuração (tools/art/remaster-monstros/models.mjs):
//   - o corpo rigado (Character_output.glb) e os clipes de andar/correr do próprio rig (grátis);
//   - clipes escolhidos de QUALQUER arquivo de animação, mesmo de outro personagem: todos usam o
//     template mixamo (mesmos nomes de osso), então os canais se ligam pelo nome e a altura do
//     quadril é ajustada por `hipscale` (altura do rig / altura do rig de origem);
//   - recorte opcional de cada clipe (`from`/`to`, em segundos) e velocidade (`speed`);
//   - encaixes handslot.r/.l como ossos filhos das mãos (calibrados no v3).
// O merged_animations.glb vem com o Armature girado 90 graus em X; compomos a rotação no quadril.
//
// Uso (dentro de client/): node scripts/build-remaster-monstros.mjs <id do modelo>
import { NodeIO } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';
import { resolve } from 'node:path';
import { MODELS, META } from '../../tools/art/remaster-monstros/models.mjs';

const id = process.argv[2];
const cfg = MODELS[id];
if (!cfg) throw new Error(`modelo desconhecido: ${id} (${Object.keys(MODELS).join(', ')})`);
const M = resolve(import.meta.dirname, '../../assets-src/meshy');
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const doc = await io.read(`${M}/${cfg.rig}`);
const root = doc.getRoot();
const buffer = root.listBuffers()[0];
const nodeByName = new Map(root.listNodes().map((n) => [n.getName(), n]));

const qmul = (a, b) => [
  a[3] * b[0] + a[0] * b[3] + a[1] * b[2] - a[2] * b[1],
  a[3] * b[1] - a[0] * b[2] + a[1] * b[3] + a[2] * b[0],
  a[3] * b[2] + a[0] * b[1] - a[1] * b[0] + a[2] * b[3],
  a[3] * b[3] - a[0] * b[0] - a[1] * b[1] - a[2] * b[2],
];
const qrot = (q, v) => {
  const p = qmul(qmul(q, [v[0], v[1], v[2], 0]), [-q[0], -q[1], -q[2], q[3]]);
  return [p[0], p[1], p[2]];
};

const cache = new Map();
const read = async (p) => cache.get(p) ?? (cache.set(p, await io.read(`${M}/${p}`)), cache.get(p));

/** Copia um clipe de `path` para este documento com o nome final. */
/** Altura média do quadril (y) no clipe de andar de um rig: serve de régua entre dois personagens. */
async function walkHips(p) {
  const s = await read(p);
  const ch = s.getRoot().listAnimations()[0].listChannels().find((c) => c.getTargetNode().getName().endsWith('Hips') && c.getTargetPath() === 'translation');
  const a = ch.getSampler().getOutput().getArray();
  let sum = 0;
  for (let i = 1; i < a.length; i += 3) sum += a[i];
  return sum / (a.length / 3);
}
const autoScale = (await walkHips(cfg.walkOwn)) / (await walkHips(cfg.walkRef));
console.log(`  hipscale automático (quadril do andar, ${cfg.walkOwn.split('/')[0]} / referência): ${autoScale.toFixed(3)}`);

async function copyClip({ src, pick, as, from = 0, to = Infinity, speed = 1, hipscale = 1 }) {
  if (hipscale === 'auto') hipscale = autoScale;
  const s = await read(src);
  const armature = s.getRoot().listNodes().find((n) => n.getName() === 'Armature');
  const qa = armature ? armature.getRotation() : [0, 0, 0, 1];
  const fix = Math.abs(qa[3]) < 0.999;
  const srcClip = s.getRoot().listAnimations().find((a) => (pick ? a.getName() === pick || a.getName().includes(`|${pick}|`) : true));
  if (!srcClip) throw new Error(`clipe ${pick} não existe em ${src}`);
  const clip = doc.createAnimation(`${cfg.prefix}${as}`);
  for (const ch of srcClip.listChannels()) {
    const name = ch.getTargetNode().getName();
    const target = nodeByName.get(name);
    if (!target) continue;
    const sm = ch.getSampler();
    const t = sm.getInput().getArray();
    const tp = ch.getTargetPath();
    const w = tp === 'rotation' ? 4 : 3;
    const keep = [];
    for (let i = 0; i < t.length; i++) if (t[i] >= from - 1e-6 && t[i] <= to + 1e-6) keep.push(i);
    const times = new Float32Array(keep.length);
    const data = new Float32Array(keep.length * w);
    const od = sm.getOutput().getArray();
    keep.forEach((k, j) => {
      times[j] = (t[k] - from) / speed;
      const v = Array.from(od.slice(k * w, k * w + w));
      let out = v;
      if (fix && name.endsWith('Hips')) out = tp === 'rotation' ? qmul(qa, v) : tp === 'translation' ? qrot(qa, v) : v;
      if (tp === 'translation' && name.endsWith('Hips')) out = out.map((x) => x * hipscale);
      data.set(out, j * w);
    });
    const input = doc.createAccessor().setType('SCALAR').setArray(times).setBuffer(buffer);
    const output = doc.createAccessor().setType(sm.getOutput().getType()).setArray(data).setBuffer(buffer);
    const sampler = doc.createAnimationSampler().setInput(input).setOutput(output).setInterpolation(sm.getInterpolation());
    clip.addSampler(sampler);
    clip.addChannel(doc.createAnimationChannel().setTargetNode(target).setTargetPath(tp).setSampler(sampler));
  }
}
for (const c of cfg.clips) await copyClip(c);

// Encaixes das armas (calibrados no v3: lâmina para a frente da mão, escudo com a face para fora).
const skin = root.listSkins()[0];
for (const [slot, hand] of [['handslot.r', 'mixamorig:RightHand'], ['handslot.l', 'mixamorig:LeftHand']]) {
  const parent = nodeByName.get(hand);
  if (!parent) throw new Error(`osso ${hand} não existe`);
  const k = cfg.slotscale ?? 0.4;
  const n = doc.createNode(slot).setRotation([0.5, 0.5, 0.5, 0.5]).setTranslation([0, 0.07, 0]).setScale([k, k, k]);
  parent.addChild(n);
  const ibm = skin.getInverseBindMatrices();
  const arr = ibm.getArray();
  const hi = skin.listJoints().indexOf(parent);
  const next = new Float32Array(arr.length + 16);
  next.set(arr);
  next.set(arr.slice(hi * 16, hi * 16 + 16), arr.length);
  ibm.setArray(next);
  skin.addJoint(n);
}
const out = `${M}/${cfg.out}`;
await io.write(out, doc);
console.log(`${cfg.out}: ${root.listAnimations().map((a) => a.getName()).join(', ')}`);
void META;
