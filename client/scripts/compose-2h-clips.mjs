// Monta os clipes de arma de duas mãos de um herói do remaster (tunnel#94): parado, andar e correr com as
// DUAS mãos juntas na frente do peito, no lugar de uma pendurada em cada lado.
//
// A biblioteca do Meshy não tem parado nem correr de duas mãos, só "Walk_Fight_Forward" (as mãos juntas diante
// do peito). Então cada clipe novo `<prefixo>2_<Papel>` é o clipe normal (<prefixo><Papel>) com os ossos dos braços
// (ombro, braço, antebraço, mão) trocados pela pose dos braços do clipe `<prefixo>WalkFight` no tempo 0, parada.
//
// Uso (dentro de client/): node scripts/compose-2h-clips.mjs <entrada.glb> <saida.glb> <prefixo> [Idle,Walk,Run]
import { NodeIO } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';

const [inPath, outPath, prefix, rolesArg = 'Idle,Walk,Run'] = process.argv.slice(2);
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const doc = await io.read(inPath);
const root = doc.getRoot();
const buffer = root.listBuffers()[0];
const anim = (n) => root.listAnimations().find((a) => a.getName() === n);
const fight = anim(`${prefix}WalkFight`);
if (!fight) throw new Error(`falta o clipe ${prefix}WalkFight`);
const isArm = (name) => /(Shoulder|Arm|ForeArm|Hand)$/.test(name) && !/handslot/.test(name);
// pose dos braços: valor da chave em t = 0 de cada canal do clipe de referência
const armPose = new Map();
for (const ch of fight.listChannels()) {
  const name = ch.getTargetNode().getName();
  if (!isArm(name)) continue;
  const out = ch.getSampler().getOutput();
  const w = out.getElementSize();
  armPose.set(`${name}|${ch.getTargetPath()}`, Array.from(out.getArray().slice(0, w)));
}
for (const role of rolesArg.split(',')) {
  const base = anim(`${prefix}${role}`);
  if (!base) throw new Error(`falta o clipe ${prefix}${role}`);
  const clip = doc.createAnimation(`${prefix}2_${role}`);
  for (const ch of base.listChannels()) {
    const node = ch.getTargetNode();
    const name = node.getName();
    const s = ch.getSampler();
    const pose = isArm(name) ? armPose.get(`${name}|${ch.getTargetPath()}`) : null;
    let times = s.getInput().getArray().slice();
    let data = s.getOutput().getArray().slice();
    if (pose) {
      // duas chaves iguais, no início e no fim do clipe
      const end = times[times.length - 1];
      times = Float32Array.from([0, end]);
      data = Float32Array.from([...pose, ...pose]);
    }
    const input = doc.createAccessor().setType('SCALAR').setArray(times).setBuffer(buffer);
    const output = doc.createAccessor().setType(s.getOutput().getType()).setArray(data).setBuffer(buffer);
    const sampler = doc.createAnimationSampler().setInput(input).setOutput(output).setInterpolation(pose ? 'LINEAR' : s.getInterpolation());
    clip.addSampler(sampler);
    clip.addChannel(doc.createAnimationChannel().setTargetNode(node).setTargetPath(ch.getTargetPath()).setSampler(sampler));
  }
}
await io.write(outPath, doc);
console.log(`${outPath}: ${root.listAnimations().map((a) => a.getName()).join(', ')}`);
