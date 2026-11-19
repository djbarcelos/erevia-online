// Põe clipes comprados no Meshy num herói do remaster já pronto (tunnel#181), com retargeting.
//
// A conta do Meshy de hoje não enxerga o rig da rodada anterior; o herói foi rigado de novo com o mesmo
// molde (mixamo) e a mesma malha (scripts/make-rig-source.mjs). Os nomes dos ossos batem, mas a pose de
// descanso difere alguns graus (até ~18° nos pés). Tocar o clipe direto torceria a malha; aqui cada osso
// recebe a correção constante entre os dois descansos:
//   W_A(t) = W_B(t) · C,  C = W_B(descanso)⁻¹ · W_A(descanso)  =>  L_A(t) = C_pai⁻¹ · L_B(t) · C
// (W = rotação no mundo, L = local). O quadril leva a translação convertida pelo pai de cada esqueleto;
// nos outros ossos fica só a rotação (comprimento dos ossos do jogo). Escala é ignorada.
//
//   cd client && node scripts/add-remaster-clips.mjs <heroi_b.glb> <merged_animations.glb> \
//       Nome_No_Meshy:KB_Papel[@ini:fim] [...]
// O clipe de mesmo nome no herói é trocado; o quadril fica no lugar (--root-motion mantém o deslocamento). ini:fim (segundos) recorta o trecho que lê bem.
import { NodeIO, MathUtils } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';
import { prune } from '@gltf-transform/functions';

const argv = process.argv.slice(2);
const ROOT_MOTION = argv.includes('--root-motion'); // padrão: no lugar
const [gameFile, animFile, ...maps] = argv.filter((a) => a !== '--root-motion');
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const game = await io.read(gameFile);
const src = await io.read(animFile);

// ---- quatérnios (x, y, z, w) ----
const qmul = (a, b) => [
  a[3] * b[0] + a[0] * b[3] + a[1] * b[2] - a[2] * b[1],
  a[3] * b[1] - a[0] * b[2] + a[1] * b[3] + a[2] * b[0],
  a[3] * b[2] + a[0] * b[1] - a[1] * b[0] + a[2] * b[3],
  a[3] * b[3] - a[0] * b[0] - a[1] * b[1] - a[2] * b[2],
];
const qinv = (q) => [-q[0], -q[1], -q[2], q[3]];
const qnorm = (q) => { const l = Math.hypot(...q); return q.map((x) => x / l); };
const qrot = (q, v) => { const p = qmul(qmul(q, [v[0], v[1], v[2], 0]), qinv(q)); return [p[0], p[1], p[2]]; };
function worldOf(node) {
  const t = [0, 0, 0], r = [0, 0, 0, 1], s = [1, 1, 1];
  MathUtils.decompose(node.getWorldMatrix(), t, r, s);
  return { t, r: qnorm(r), s };
}
const parentOf = (doc, node) => doc.getRoot().listNodes().find((p) => p.listChildren().includes(node)) ?? null;
const byName = (doc) => new Map(doc.getRoot().listNodes().map((n) => [n.getName(), n]));
const A = byName(game), B = byName(src);
const ID = { t: [0, 0, 0], r: [0, 0, 0, 1], s: [1, 1, 1] };
// C de cada osso (e do pai), na pose de descanso dos dois arquivos.
const corr = new Map();
const C = (name) => {
  if (corr.has(name)) return corr.get(name);
  const a = A.get(name), b = B.get(name);
  const c = qmul(qinv(worldOf(b).r), worldOf(a).r);
  corr.set(name, c);
  return c;
};
const parentWorld = (doc, node) => { const p = parentOf(doc, node); return p ? worldOf(p) : ID; };

for (const spec of maps) {
  const m = /^([^:]+):([^@]+)(?:@([\d.]+):([\d.]+))?$/.exec(spec);
  if (!m) throw new Error(`mapa inválido: ${spec}`);
  const [, from, to, t0s, t1s] = m;
  const anim = src.getRoot().listAnimations().find((a) => a.getName() === from);
  if (!anim) throw new Error(`${animFile} não tem o clipe ${from} (tem: ${src.getRoot().listAnimations().map((a) => a.getName()).join(', ')})`);
  const t0 = t0s ? Number(t0s) : 0, t1 = t1s ? Number(t1s) : Infinity;
  // Só a animação sai: os accessors podem ser compartilhados com outros clipes (o prune no fim limpa).
  for (const old of game.getRoot().listAnimations().filter((a) => a.getName() === to)) old.dispose();
  const out = game.createAnimation(to);
  const buf = game.getRoot().listBuffers()[0];
  let chans = 0;
  for (const ch of anim.listChannels()) {
    const path = ch.getTargetPath();
    const bn = ch.getTargetNode();
    const name = bn?.getName();
    const an = name && A.get(name);
    if (!an || path === 'scale' || path === 'weights') continue;
    const isHips = /Hips$/.test(name);
    if (path === 'translation' && !isHips) continue;
    const smp = ch.getSampler();
    const inT = smp.getInput().getArray(), inV = smp.getOutput().getArray();
    const n = path === 'rotation' ? 4 : 3;
    const times = [], vals = [];
    const pB = parentOf(src, bn), pA = parentOf(game, an);
    const cSelf = C(name);
    const cPar = pB && pA && B.get(pA.getName()) ? C(pA.getName()) : qmul(qinv(parentWorld(src, bn).r), parentWorld(game, an).r);
    const pwB = parentWorld(src, bn), pwA = parentWorld(game, an);
    // Chaves do trecho, mais as bordas interpoladas (um canal constante, com uma chave só antes do
    // corte, não pode sumir: o osso ficaria sem animação no golpe).
    const at = (t) => {
      let i = 0;
      while (i < inT.length - 1 && inT[i + 1] <= t) i++;
      const j = Math.min(i + 1, inT.length - 1);
      const a = inT[j] > inT[i] ? Math.min(1, Math.max(0, (t - inT[i]) / (inT[j] - inT[i]))) : 0;
      const va = Array.from(inV.slice(i * n, i * n + n)), vb = Array.from(inV.slice(j * n, j * n + n));
      const sb = n === 4 && va[0] * vb[0] + va[1] * vb[1] + va[2] * vb[2] + va[3] * vb[3] < 0 ? -1 : 1;
      const v = va.map((x, k) => x * (1 - a) + vb[k] * sb * a);
      return n === 4 ? qnorm(v) : v;
    };
    const end = Math.min(t1, inT[inT.length - 1]);
    const keys = [];
    if (t0 > inT[0] + 1e-6 || t0s) keys.push([t0, at(t0)]);
    for (let i = 0; i < inT.length; i++) if (inT[i] > t0 + 1e-6 && inT[i] < end - 1e-6) keys.push([inT[i], Array.from(inV.slice(i * n, i * n + n))]);
    if (!t0s) keys.unshift([inT[0], Array.from(inV.slice(0, n))]);
    if (end > keys[keys.length - 1][0] + 1e-6) keys.push([end, at(end)]);
    for (const [t, raw] of keys) {
      times.push(t - (t0s ? t0 : 0));
      const v = raw;
      if (path === 'rotation') {
        vals.push(...qnorm(qmul(qmul(qinv(cPar), v), cSelf)));
      } else {
        // translação do quadril: local de B -> mundo -> local de A
        const w = qrot(pwB.r, [v[0] * pwB.s[0], v[1] * pwB.s[1], v[2] * pwB.s[2]]).map((x, k) => x + pwB.t[k]);
        // No lugar: quem anda é o servidor. O quadril fica no X/Z do descanso do herói (sobe e desce à vontade).
        if (!ROOT_MOTION) { const rest = worldOf(an).t; w[0] = rest[0]; w[2] = rest[2]; }
        const l = qrot(qinv(pwA.r), w.map((x, k) => x - pwA.t[k])).map((x, k) => x / pwA.s[k]);
        vals.push(...l);
      }
    }
    if (!times.length) continue;
    const input = game.createAccessor().setType('SCALAR').setArray(new Float32Array(times)).setBuffer(buf);
    const output = game.createAccessor().setType(path === 'rotation' ? 'VEC4' : 'VEC3').setArray(new Float32Array(vals)).setBuffer(buf);
    const sampler = game.createAnimationSampler().setInput(input).setOutput(output).setInterpolation(smp.getInterpolation());
    out.addSampler(sampler).addChannel(game.createAnimationChannel().setTargetNode(an).setTargetPath(path).setSampler(sampler));
    chans++;
  }
  console.log(`${to} <- ${from}${t0s ? ` (${t0}–${t1} s)` : ''}: ${chans} canais`);
}
await game.transform(prune({ propertyTypes: ['Accessor', 'AnimationSampler'] }));
await io.write(gameFile, game);
