// Monta o GLB cru de cada família: o rig e os clipes de prep.py (<id>_uv.glb) + a textura do retexture do Meshy.
// uso: node tools/art/familias-monstro/build.mjs   (lê RETEX abaixo; grava assets-src/familias/<id>_raw.glb)
// Depois: sh tools/art/familias-monstro/import.sh
import { createRequire } from 'node:module';
import { resolve } from 'node:path';
import { readFileSync } from 'node:fs';
const req = createRequire(resolve(import.meta.dirname, '../../../client/') + '/');
const { NodeIO } = req('@gltf-transform/core');
const sharp = req('sharp');
const DIR = resolve(import.meta.dirname, '../../../assets-src/familias');
const MESHY = resolve(import.meta.dirname, '../../../assets-src/meshy');

/** id -> task do retexture (pasta em assets-src/meshy) que vale; ajustes de cor opcionais (sharp.modulate). */
const RETEX = {
  wolf: { task: 'retexture-248a3010' },
  spider: { task: 'retexture-d3be6ba2' },
  slime: { task: 'retexture-6f66a2bc', mod: { saturation: 0.82, brightness: 0.95 } },
  frog: { task: 'retexture-bf536705' },
};

const io = new NodeIO();

/**
 * Custo de CPU da animação (o AnimController avalia cada canal a cada quadro): tira o que não aparece.
 *  1. canais de ossos que não deformam nada (alvos de IK, polos): nenhum vértice depende deles nem de seus filhos;
 *  2. canais constantes em TODOS os clipes e iguais à pose de repouso (a escala, quase sempre).
 * Devolve quantos canais ficaram e quantos saíram.
 */
function trim(doc) {
  const root = doc.getRoot();
  const used = new Set();
  const skin = root.listSkins()[0];
  const joints = skin.listJoints();
  for (const mesh of root.listMeshes()) {
    for (const prim of mesh.listPrimitives()) {
      const j = prim.getAttribute('JOINTS_0');
      const w = prim.getAttribute('WEIGHTS_0');
      if (!j || !w) continue;
      const ja = j.getArray();
      const wa = w.getArray();
      for (let i = 0; i < ja.length; i++) if (wa[i] > 0.001) used.add(joints[ja[i]]);
    }
  }
  // quem precisa ficar: ossos com peso e todos os seus ancestrais (a cadeia de transformações)
  const keep = new Set();
  for (const n of used) {
    let p = n;
    for (let guard = 0; p && guard < 1000; guard++) {
      keep.add(p);
      p = p.getParentNode();
    }
  }
  const same = (a, b, eps = 1e-4) => a.length === b.length && a.every((v, i) => Math.abs(v - b[i]) <= eps);
  const rest = { translation: (n) => n.getTranslation(), rotation: (n) => n.getRotation(), scale: (n) => n.getScale() };
  // o canal varia em algum clipe?
  const varies = new Map();
  for (const a of root.listAnimations()) {
    for (const ch of a.listChannels()) {
      const n = ch.getTargetNode();
      const path = ch.getTargetPath();
      const k = `${n.getName()}|${path}`;
      const out = ch.getSampler().getOutput();
      const r = Array.from(rest[path](n));
      let v = varies.get(k) ?? false;
      for (let i = 0; i < out.getCount() && !v; i++) if (!same(out.getElement(i, Array(out.getElementSize()).fill(0)), r)) v = true;
      varies.set(k, v);
    }
  }
  let kept = 0;
  let dropped = 0;
  for (const a of root.listAnimations()) {
    for (const ch of a.listChannels()) {
      const n = ch.getTargetNode();
      if (keep.has(n) && varies.get(`${n.getName()}|${ch.getTargetPath()}`)) { kept++; continue; }
      dropped++;
      const s = ch.getSampler();
      a.removeChannel(ch);
      ch.dispose();
      if (!a.listChannels().some((c) => c.getSampler() === s)) { a.removeSampler(s); s.dispose(); }
    }
  }
  return { kept, dropped };
}

for (const [id, r] of Object.entries(RETEX)) {
  const doc = await io.read(resolve(DIR, `${id}_uv.glb`));
  let img = sharp(resolve(MESHY, r.task, 'base_color-texture_0.png')).resize(1024, 1024);
  if (r.mod) img = img.modulate(r.mod);
  const png = await img.png().toBuffer();
  const root = doc.getRoot();
  const mat = root.listMaterials()[0];
  const tex = doc.createTexture(id).setImage(png).setMimeType('image/png');
  mat.setName(id).setBaseColorFactor([1, 1, 1, 1]).setBaseColorTexture(tex).setMetallicFactor(0).setRoughnessFactor(0.85);
  const t = trim(doc);
  console.log(id, `canais: ${t.kept} ficaram, ${t.dropped} saíram`);
  await io.write(resolve(DIR, `${id}_raw.glb`), doc);
  console.log(id, 'ok', root.listAnimations().map((a) => a.getName()).join(','));
}
