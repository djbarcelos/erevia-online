// Prende um elmo gerado à parte (Meshy image-to-3d de um conceito só do elmo) na cabeça de um herói do
// remaster, como a peça `Hat` (rígida no osso da cabeça; "esconder chapéu" liga e desliga o nó).
// Substitui o elmo extraído da cabeça do cavaleiro da v3 (make-remaster-hat.mjs), que ficou torto.
//
// Uso (dentro de client/): node scripts/make-remaster-helm.mjs <elmo.glb> <heroi_raw.glb> <saida.glb>
//        [--scale=0.36] [--dy=0] [--dz=0] [--ry=0]
//   --scale: escala uniforme do elmo (o Meshy entrega ~0,57 m de largura); --dy/--dz: deslocamento (m) a partir
//   do centro da cabeça; --ry: giro em torno do eixo vertical (graus). O elmo nasce com a abertura para +Z.
import { NodeIO } from '@gltf-transform/core';
import { ALL_EXTENSIONS } from '@gltf-transform/extensions';
import { MeshoptSimplifier } from 'meshoptimizer';
import { PNG } from 'pngjs';
import jpeg from 'jpeg-js';

const args = process.argv.slice(2).filter((a) => !a.startsWith('--'));
const opt = Object.fromEntries(process.argv.slice(2).filter((a) => a.startsWith('--')).map((a) => a.slice(2).split('=')));
const [helmPath, heroPath, outPath] = args;
const io = new NodeIO().registerExtensions(ALL_EXTENSIONS);
const helm = await io.read(helmPath);
const hero = await io.read(heroPath);

const meshNode = (d) => d.getRoot().listNodes().find((n) => n.getMesh());
const hp = meshNode(helm).getMesh().listPrimitives()[0];
const P = hp.getAttribute('POSITION');
const N = hp.getAttribute('NORMAL');
const UV = hp.getAttribute('TEXCOORD_0');
const idx = hp.getIndices().getArray();

// cabeça do herói: vértices ligados ao osso da cabeça
const bn = hero.getRoot().listNodes().find((n) => n.getMesh() && n.getSkin());
const skin = bn.getSkin();
const joints = skin.listJoints().map((j) => j.getName());
const headJoint = joints.indexOf('mixamorig:Head');
const bp = bn.getMesh().listPrimitives()[0];
const BP = bp.getAttribute('POSITION');
const BJ = bp.getAttribute('JOINTS_0');
const BW = bp.getAttribute('WEIGHTS_0');
let minY = 9, maxY = -9, minX = 9, maxX = -9, minZ = 9, maxZ = -9;
for (let i = 0; i < BP.getCount(); i++) {
  const j = BJ.getElement(i, []);
  const w = BW.getElement(i, []);
  let hw = 0;
  for (let k = 0; k < 4; k++) if (j[k] === headJoint) hw += w[k];
  if (hw < 0.9) continue;
  const p = BP.getElement(i, [0, 0, 0]);
  minX = Math.min(minX, p[0]); maxX = Math.max(maxX, p[0]);
  minY = Math.min(minY, p[1]); maxY = Math.max(maxY, p[1]);
  minZ = Math.min(minZ, p[2]); maxZ = Math.max(maxZ, p[2]);
}
console.log(`cabeça: x ${minX.toFixed(3)}..${maxX.toFixed(3)} y ${minY.toFixed(3)}..${maxY.toFixed(3)} z ${minZ.toFixed(3)}..${maxZ.toFixed(3)}`);
const cx = (minX + maxX) / 2, cz = (minZ + maxZ) / 2;
// o elmo (sem a crista) cobre do queixo ao topo da cabeça
const sc = opt.scale ? +opt.scale : ((maxX - minX) * 1.2) / 0.57;
const cy = minY + (maxY - minY) * 0.5 + +(opt.dy ?? 0);
const ry = ((+(opt.ry ?? 0)) * Math.PI) / 180;
const cos = Math.cos(ry), sin = Math.sin(ry);
console.log(`escala ${sc.toFixed(3)}, centro y ${cy.toFixed(3)}`);

const pos = [], nor = [], uv = [];
for (let i = 0; i < P.getCount(); i++) {
  const p = P.getElement(i, [0, 0, 0]);
  const n = N.getElement(i, [0, 0, 0]);
  const x = p[0] * cos + p[2] * sin, z = -p[0] * sin + p[2] * cos;
  pos.push(cx + x * sc, cy + p[1] * sc, cz + z * sc + +(opt.dz ?? 0));
  nor.push(n[0] * cos + n[2] * sin, n[1], -n[0] * sin + n[2] * cos);
  uv.push(...UV.getElement(i, [0, 0]));
}
// --maxtris: o corpo do Meshy já usa ~7.300 dos 8.000 triângulos do orçamento
let indices = Uint32Array.from(idx);
if (opt.maxtris && indices.length / 3 > +opt.maxtris) {
  await MeshoptSimplifier.ready;
  [indices] = MeshoptSimplifier.simplify(indices, new Float32Array(pos), 3, +opt.maxtris * 3, 1, ['LockBorder']);
}
const root = hero.getRoot();
const buffer = root.listBuffers()[0];
const acc = (type, arr) => hero.createAccessor().setType(type).setArray(arr).setBuffer(buffer);
const nv = pos.length / 3;
const jts = new Uint16Array(nv * 4);
const wts = new Float32Array(nv * 4);
for (let i = 0; i < nv; i++) { jts[i * 4] = headJoint; wts[i * 4] = 1; }
const srcTex = hp.getMaterial().getBaseColorTexture();
// textura do elmo em 512 px (média de blocos): é aço liso, não precisa mais
const raw = Buffer.from(srcTex.getImage());
const src = raw[0] === 0x89 ? PNG.sync.read(raw) : jpeg.decode(raw, { useTArray: true, formatAsRGBA: true });
const f = Math.max(1, Math.round(src.width / 512));
const small = new PNG({ width: Math.floor(src.width / f), height: Math.floor(src.height / f) });
for (let y = 0; y < small.height; y++) {
  for (let x = 0; x < small.width; x++) {
    const sum = [0, 0, 0, 0];
    for (let dy = 0; dy < f; dy++) for (let dx = 0; dx < f; dx++) for (let c = 0; c < 4; c++) sum[c] += src.data[((y * f + dy) * src.width + x * f + dx) * 4 + c];
    for (let c = 0; c < 4; c++) small.data[(y * small.width + x) * 4 + c] = Math.round(sum[c] / (f * f));
  }
}
const tex = hero.createTexture('Hat_tex').setImage(PNG.sync.write(small)).setMimeType('image/png');
const mat = hero.createMaterial('Hat_steel').setBaseColorTexture(tex).setRoughnessFactor(0.6).setMetallicFactor(0);
const prim = hero.createPrimitive().setMaterial(mat)
  .setAttribute('POSITION', acc('VEC3', new Float32Array(pos)))
  .setAttribute('NORMAL', acc('VEC3', new Float32Array(nor)))
  .setAttribute('TEXCOORD_0', acc('VEC2', new Float32Array(uv)))
  .setAttribute('JOINTS_0', acc('VEC4', jts))
  .setAttribute('WEIGHTS_0', acc('VEC4', wts))
  .setIndices(acc('SCALAR', new Uint16Array(indices)));
const node = hero.createNode('Hat').setMesh(hero.createMesh('Hat').addPrimitive(prim)).setSkin(skin);
const par = bn.getParentNode();
if (par) par.addChild(node);
else for (const s of root.listScenes()) s.addChild(node);
await io.write(outPath, hero);
console.log(`${outPath}: elmo com ${indices.length / 3} triângulos`);
