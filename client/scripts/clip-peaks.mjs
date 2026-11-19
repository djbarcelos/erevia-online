// Picos de velocidade das mãos em cada clipe de um GLB (tunnel#181): mostra onde cada golpe encosta e onde
// dá para cortar um combo em golpes separados (add-remaster-clips.mjs Nome:Papel@ini:fim) e qual é o
// instante de impacto (CLIP_IMPACT em models.ts). Cinemática direta só com as curvas do próprio arquivo.
//   cd client && node scripts/clip-peaks.mjs <arquivo.glb> [Clipe ...]
import { NodeIO, MathUtils } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';

const [file, ...only] = process.argv.slice(2);
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const doc = await io.read(file);
const nodes = doc.getRoot().listNodes();
const parent = new Map();
for (const p of nodes) for (const c of p.listChildren()) parent.set(c, p);
const FPS = 30;
// a · b (4x4 coluna-maior, como o glTF)
const mul = (a, b) => {
  const o = new Array(16).fill(0);
  for (let c = 0; c < 4; c++) for (let r = 0; r < 4; r++) for (let k = 0; k < 4; k++) o[c * 4 + r] += a[k * 4 + r] * b[c * 4 + k];
  return o;
};

function sample(ch, t) {
  const s = ch.getSampler();
  const T = s.getInput().getArray(), V = s.getOutput().getArray();
  const n = V.length / T.length;
  let i = 0;
  while (i < T.length - 1 && T[i + 1] <= t) i++;
  const j = Math.min(i + 1, T.length - 1);
  const a = T[j] > T[i] ? Math.min(1, Math.max(0, (t - T[i]) / (T[j] - T[i]))) : 0;
  const va = Array.from(V.slice(i * n, i * n + n)), vb = Array.from(V.slice(j * n, j * n + n));
  if (n === 4) {
    const d = va[0] * vb[0] + va[1] * vb[1] + va[2] * vb[2] + va[3] * vb[3];
    const sb = d < 0 ? -1 : 1;
    const q = va.map((x, k) => x * (1 - a) + vb[k] * sb * a);
    const l = Math.hypot(...q);
    return q.map((x) => x / l);
  }
  return va.map((x, k) => x * (1 - a) + vb[k] * a);
}

for (const anim of doc.getRoot().listAnimations()) {
  if (only.length && !only.includes(anim.getName())) continue;
  const byNode = new Map();
  let end = 0;
  for (const ch of anim.listChannels()) {
    const n = ch.getTargetNode();
    if (!byNode.has(n)) byNode.set(n, {});
    byNode.get(n)[ch.getTargetPath()] = ch;
    end = Math.max(end, ch.getSampler().getInput().getMax([])[0]);
  }
  const world = (node, t, cache) => {
    if (cache.has(node)) return cache.get(node);
    const c = byNode.get(node) ?? {};
    const tr = c.translation ? sample(c.translation, t) : node.getTranslation();
    const ro = c.rotation ? sample(c.rotation, t) : node.getRotation();
    const sc = c.scale ? sample(c.scale, t) : node.getScale();
    const local = MathUtils.compose(tr, ro, sc, new Array(16));
    const p = parent.get(node);
    const m = p ? mul(world(p, t, cache), local) : local;
    cache.set(node, m);
    return m;
  };
  const hands = nodes.filter((n) => /mixamorig:(Right|Left)Hand$/.test(n.getName()));
  const speeds = hands.map(() => []);
  let prev = null;
  for (let f = 0; f <= Math.round(end * FPS); f++) {
    const cache = new Map();
    const pos = hands.map((h) => { const m = world(h, f / FPS, cache); return [m[12], m[13], m[14]]; });
    if (prev) pos.forEach((p, k) => speeds[k].push(Math.hypot(p[0] - prev[k][0], p[1] - prev[k][1], p[2] - prev[k][2]) * FPS));
    else speeds.forEach((s) => s.push(0));
    prev = pos;
  }
  // Picos: máximo local acima de 60% do maior, separados por 0,25 s; o golpe "encosta" no fim do pico.
  const out = [];
  hands.forEach((h, k) => {
    const s = speeds[k], top = Math.max(...s);
    const peaks = [];
    for (let f = 1; f < s.length - 1; f++) {
      if (s[f] >= top * 0.6 && s[f] >= s[f - 1] && s[f] >= s[f + 1] && (!peaks.length || f - peaks[peaks.length - 1] > 0.25 * FPS)) peaks.push(f);
    }
    out.push(`${h.getName().replace('mixamorig:', '')}: pico ${top.toFixed(1)} m/s em ${peaks.map((f) => (f / FPS).toFixed(2) + ' s').join(', ')}`);
  });
  console.log(`${anim.getName()} (${end.toFixed(2)} s): ${out.join(' | ')}`);
}
