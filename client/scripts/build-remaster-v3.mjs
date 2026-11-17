// Monta o GLB cru de um personagem do remaster v3 a partir dos arquivos do Meshy:
//   - o corpo rigado (Character_output.glb: Y para cima, quadril em y=1,05);
//   - as ações pagas (merged_animations.glb: Idle, golpe, dano, morte);
//   - andar e correr, que o rig do Meshy já devolve de graça (walking/running ...withSkin.glb);
//   - encaixes handslot.r/.l como ossos filhos das mãos (o import-asset não os poda).
// Os clipes ganham o prefixo do personagem (KB_ cavaleiro, SW_ esqueleto) porque o adaptador
// guarda os clipes de todos os modelos num mapa só.
//
// O merged_animations.glb vem com o Armature girado 90 graus em X (Z para cima) e o quadril
// compensando; os clipes dele são trazidos para o Y para cima compondo a rotação do Armature no
// quadril (rotação e translação).
//
// Uso (dentro de client/):
//   node scripts/build-remaster-v3.mjs <rig.glb> <merged.glb[,outro.glb]> <walk.glb> <run.glb> <prefixo> <saida.glb>
//        [--rename=Antigo:Novo,...] [--hand=<osso da mão direita>,<esquerda>]
//        [--trim=ClipeDoMeshy:inicio_s:fim_s,...]  fica só com o trecho do clipe e o desloca para t=0 (ex.: a investida de 7 s do machado)
import { NodeIO } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';

const args = process.argv.slice(2).filter((a) => !a.startsWith('--'));
const opt = Object.fromEntries(process.argv.slice(2).filter((a) => a.startsWith('--')).map((a) => a.slice(2).split('=')));
const [rigPath, mergedPath, walkPath, runPath, prefix, outPath] = args;
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const doc = await io.read(rigPath);
const root = doc.getRoot();
const buffer = root.listBuffers()[0];
const nodeByName = new Map(root.listNodes().map((n) => [n.getName(), n]));

// Nomes que o Meshy dá aos clipes → nomes do jogo.
const trims = Object.fromEntries((opt.trim ?? '').split(',').filter(Boolean).map((t) => { const [n, a, b] = t.split(':'); return [n, [+a, +b]]; }));
const rename = { Idle: 'Idle', Right_Hand_Sword_Slash: 'Attack', Hit_Reaction_1: 'Hit', Sword_Shout: 'Raise', Dead: 'Death', ...Object.fromEntries((opt.rename ?? '').split(',').filter(Boolean).map((p) => p.split(':'))) };

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

/** Copia os clipes de outro documento para este, ligando os canais aos ossos de mesmo nome. */
async function copyClips(path, namer, keep) {
  const src = await io.read(path);
  const armature = src.getRoot().listNodes().find((n) => n.getName() === 'Armature');
  const qa = armature ? armature.getRotation() : [0, 0, 0, 1];
  const fix = Math.abs(qa[3]) < 0.999; // Armature girado: compensar no quadril
  for (const srcClip of src.getRoot().listAnimations()) {
    if (keep && !keep.includes(srcClip.getName())) continue;
    const clip = doc.createAnimation(`${prefix}${namer(srcClip.getName())}`);
    for (const ch of srcClip.listChannels()) {
      const name = ch.getTargetNode().getName();
      const target = nodeByName.get(name);
      if (!target) continue;
      const s = ch.getSampler();
      let times = s.getInput().getArray().slice();
      let data = s.getOutput().getArray().slice();
      const tr = trims[srcClip.getName()];
      if (tr) {
        // só as chaves do trecho, com o tempo deslocado; a interpolação cúbica (3 valores por chave) não é usada pelo Meshy
        const w = data.length / times.length;
        const idx = [...times].map((t, i) => [t, i]).filter(([t]) => t >= tr[0] - 1e-4 && t <= tr[1] + 1e-4).map(([, i]) => i);
        if (idx.length < 2) {
          // canal parado dentro do trecho (poucas chaves): vira duas chaves iguais, no início e no fim
          const all = [...times];
          let c = 0;
          all.forEach((t, i) => { if (t <= tr[0] + 1e-4) c = i; });
          const v = data.slice(c * w, c * w + w);
          times = Float32Array.from([0, tr[1] - tr[0]]);
          data = new Float32Array(2 * w);
          data.set(v, 0); data.set(v, w);
        }
        const nt = idx.length < 2 ? times : new Float32Array(idx.length);
        if (idx.length >= 2) {
          const nd = new Float32Array(idx.length * w);
          idx.forEach((i, k) => { nt[k] = times[i] - tr[0]; nd.set(data.subarray(i * w, i * w + w), k * w); });
          times = nt; data = nd;
        }
      }
      const input = doc.createAccessor().setType('SCALAR').setArray(times).setBuffer(buffer);
      if (fix && name.endsWith('Hips')) {
        const tp = ch.getTargetPath();
        for (let i = 0; i < data.length; i += tp === 'rotation' ? 4 : 3) {
          if (tp === 'rotation') data.set(qmul(qa, [data[i], data[i + 1], data[i + 2], data[i + 3]]), i);
          else if (tp === 'translation') data.set(qrot(qa, [data[i], data[i + 1], data[i + 2]]), i);
        }
      }
      const output = doc.createAccessor().setType(s.getOutput().getType()).setArray(data).setBuffer(buffer);
      const sampler = doc.createAnimationSampler().setInput(input).setOutput(output).setInterpolation(s.getInterpolation());
      clip.addSampler(sampler);
      clip.addChannel(doc.createAnimationChannel().setTargetNode(target).setTargetPath(ch.getTargetPath()).setSampler(sampler));
    }
  }
}
// --keep=<nomes do Meshy>: só esses clipes entram (os outros foram testes que não serviram).
const keep = opt.keep ? opt.keep.split(",") : null;
for (const p of mergedPath.split(",")) await copyClips(p, (n) => rename[n] ?? n, keep);
await copyClips(walkPath, () => 'Walk');
await copyClips(runPath, () => 'Run');

// Encaixes das armas: ossos extras na mão, com a matriz de bind da própria mão (filho com
// transformação identidade). A orientação se calibra depois, no próprio nó.
const skin = root.listSkins()[0];
const [handR, handL] = (opt.hand ?? 'mixamorig:RightHand,mixamorig:LeftHand').split(',');
for (const [slot, hand] of [['handslot.r', handR], ['handslot.l', handL]]) {
  const parent = nodeByName.get(hand);
  if (!parent) throw new Error(`osso ${hand} não existe`);
  // Calibrado no Blender: a lâmina (Y da arma) aponta para a frente da mão (Z do osso) e o escudo mostra a
  // face para fora. Escala 0,4: as armas KayKit são para um boneco chibi e ficam gigantes num herói de 1,8 m.
  const n = doc.createNode(slot).setRotation([0.5, 0.5, 0.5, 0.5]).setTranslation([0, 0.07, 0]).setScale([+(opt.slotscale ?? 0.4), +(opt.slotscale ?? 0.4), +(opt.slotscale ?? 0.4)]);
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
await io.write(outPath, doc);
console.log(`${outPath}: ${root.listAnimations().map((a) => a.getName()).join(', ')}`);
