// GLBs mínimos gerados no próprio teste com o gltf-transform (nada de binário versionado).
import { Document, NodeIO } from '@gltf-transform/core';
import { deflateSync } from 'node:zlib';

/** PNG RGBA sólido `size`×`size`, sem dependência (para testar textura e KTX2). */
export function png(size, rgba = [200, 80, 40, 255]) {
  const crcTable = Array.from({ length: 256 }, (_, n) => {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    return c >>> 0;
  });
  const crc = (buf) => {
    let c = 0xffffffff;
    for (const b of buf) c = crcTable[(c ^ b) & 255] ^ (c >>> 8);
    return (c ^ 0xffffffff) >>> 0;
  };
  const chunk = (type, data) => {
    const body = Buffer.concat([Buffer.from(type), data]);
    const out = Buffer.alloc(12 + data.length);
    out.writeUInt32BE(data.length, 0);
    body.copy(out, 4);
    out.writeUInt32BE(crc(body), 8 + data.length);
    return out;
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(size, 0);
  ihdr.writeUInt32BE(size, 4);
  ihdr[8] = 8;
  ihdr[9] = 6;
  const raw = Buffer.alloc((size * 4 + 1) * size);
  for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) for (let c = 0; c < 4; c++) raw[y * (size * 4 + 1) + 1 + x * 4 + c] = rgba[c];
  return new Uint8Array(
    Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', ihdr), chunk('IDAT', deflateSync(raw)), chunk('IEND', Buffer.alloc(0))]),
  );
}

/** Uma grade de `quads`×1 quadrados (2 triângulos cada): `quads * 2` triângulos. */
export function addGrid(doc, buffer, quads, name, { material, parent } = {}) {
  const pos = new Float32Array((quads + 1) * 2 * 3);
  for (let i = 0; i <= quads; i++) pos.set([i, 0, 0, i, 1, 0], i * 6);
  const idx = new Uint16Array(quads * 6);
  for (let i = 0; i < quads; i++) idx.set([i * 2, i * 2 + 1, i * 2 + 2, i * 2 + 1, i * 2 + 3, i * 2 + 2], i * 6);
  const prim = doc
    .createPrimitive()
    .setAttribute('POSITION', doc.createAccessor().setType('VEC3').setArray(pos).setBuffer(buffer))
    .setIndices(doc.createAccessor().setType('SCALAR').setArray(idx).setBuffer(buffer));
  if (material) prim.setMaterial(material);
  const node = doc.createNode(name).setMesh(doc.createMesh(name).addPrimitive(prim));
  (parent ?? doc.getRoot().listScenes()[0]).addChild(node);
  return node;
}

/**
 * Monta um documento. spec: { nodes: [[nome, triângulos]...], materials, texture (px), bones }.
 * Os triângulos são pares: a grade tem `tris / 2` quadrados.
 */
export function makeDoc({ nodes = [], materials = 1, texture = 0, bones = 0 } = {}) {
  const doc = new Document();
  const buffer = doc.createBuffer();
  const scene = doc.createScene();
  const mats = Array.from({ length: materials }, (_, i) => doc.createMaterial(`m${i}`));
  if (texture) {
    const tex = doc.createTexture('atlas').setImage(png(texture)).setMimeType('image/png');
    mats[0]?.setBaseColorTexture(tex);
  }
  nodes.forEach(([name, tris], i) => addGrid(doc, buffer, tris / 2, name, { material: mats[i % Math.max(1, mats.length)] }));
  if (bones) {
    const joints = Array.from({ length: bones }, (_, i) => {
      const j = doc.createNode(`bone${i}`);
      scene.addChild(j);
      return j;
    });
    const skin = doc.createSkin('skin');
    joints.forEach((j) => skin.addJoint(j));
  }
  return doc;
}

export async function writeGlb(path, doc) {
  await new NodeIO().write(path, doc);
}
