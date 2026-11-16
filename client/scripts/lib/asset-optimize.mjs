// Otimizações que o import-asset aplica a um asset novo: LOD1/LOD2 automáticos (simplify do
// meshoptimizer) e textura em KTX2/Basis (encoder em wasm do pacote ktx2-encoder; nada
// instalado no sistema).
import { MeshoptSimplifier } from 'meshoptimizer';
import { ktx2 } from 'ktx2-encoder/gltf-transform';
import jpeg from 'jpeg-js';
import { PNG } from 'pngjs';
import { BUDGET } from './asset-budget.mjs';

const LOD_SUFFIX = /_LOD\d$/i;

function positionsOf(accessor) {
  const out = new Float32Array(accessor.getCount() * 3);
  const v = [0, 0, 0];
  for (let i = 0; i < accessor.getCount(); i++) {
    accessor.getElement(i, v);
    out.set(v, i * 3);
  }
  return out;
}

function indicesOf(prim) {
  const idx = prim.getIndices();
  if (idx) return Uint32Array.from(idx.getArray());
  return Uint32Array.from({ length: prim.getAttribute('POSITION').getCount() }, (_, i) => i);
}

/**
 * Cria a malha de um LOD: as mesmas posições, UVs, pesos de osso e material (os atributos são
 * compartilhados com o LOD0, então o arquivo só ganha os índices novos), com `ratio` dos triângulos.
 */
function simplifiedMesh(doc, mesh, ratio, name, flags) {
  const lod = doc.createMesh(name);
  const buffer = doc.getRoot().listBuffers()[0];
  for (const prim of mesh.listPrimitives()) {
    const positions = positionsOf(prim.getAttribute('POSITION'));
    const indices = indicesOf(prim);
    const target = Math.max(3, Math.floor((indices.length * ratio) / 3) * 3);
    // Erro máximo 1 (relativo ao tamanho da malha): o alvo de triângulos manda, não o erro.
    const [simplified] = MeshoptSimplifier.simplify(indices, positions, 3, target, 1, flags);
    const next = doc.createPrimitive().setMode(prim.getMode()).setMaterial(prim.getMaterial());
    for (const semantic of prim.listSemantics()) next.setAttribute(semantic, prim.getAttribute(semantic));
    let max = 0;
    for (const i of simplified) if (i > max) max = i;
    next.setIndices(
      doc
        .createAccessor()
        .setType('SCALAR')
        .setArray(max > 65535 ? new Uint32Array(simplified) : new Uint16Array(simplified))
        .setBuffer(buffer),
    );
    lod.addPrimitive(next);
  }
  return lod;
}

/**
 * Gera LOD1 e LOD2 (nós `<nome>_LOD1` e `<nome>_LOD2`, irmãos do LOD0 e com o mesmo esqueleto)
 * nas proporções do guia para a categoria. Não mexe em quem já tem LOD vindo do Blender.
 * Retorna { generated: n de nós criados, skipped: motivo | null }.
 */
export async function generateLods(doc, category, { skip = () => false, prune = false } = {}) {
  // prune: também descarta as ilhas pequenas (malhas de IA com peças soltas não descem do piso de ~1.100 triângulos sem isso).
  const flags = prune ? ['LockBorder', 'Prune'] : ['LockBorder'];
  const ratios = BUDGET[category].ratios;
  const nodes = doc.getRoot().listNodes();
  if (nodes.some((n) => LOD_SUFFIX.test(n.getName()))) return { generated: 0, skipped: 'já tem LOD do Blender' };
  await MeshoptSimplifier.ready;
  let generated = 0;
  for (const node of nodes) {
    const mesh = node.getMesh();
    if (!mesh || skip(node)) continue;
    ratios.forEach((ratio, i) => {
      if (!ratio) return;
      const lodNode = doc
        .createNode(`${node.getName()}_LOD${i + 1}`)
        .setMesh(simplifiedMesh(doc, mesh, ratio, `${mesh.getName() || node.getName()}_LOD${i + 1}`, flags))
        .setTranslation(node.getTranslation())
        .setRotation(node.getRotation())
        .setScale(node.getScale());
      if (node.getSkin()) lodNode.setSkin(node.getSkin());
      const parent = node.getParentNode();
      if (parent) parent.addChild(lodNode);
      else for (const scene of doc.getRoot().listScenes()) if (scene.listChildren().includes(node)) scene.addChild(lodNode);
      generated++;
    });
  }
  return { generated, skipped: generated ? null : 'sem malhas' };
}

// O encoder do Basis, em Node, pede um decodificador de imagem: PNG e JPEG em JS puro.
async function imageDecoder(buffer) {
  const isPng = buffer[0] === 0x89 && buffer[1] === 0x50;
  if (isPng) {
    const png = PNG.sync.read(Buffer.from(buffer));
    return { width: png.width, height: png.height, data: new Uint8Array(png.data) };
  }
  const img = jpeg.decode(Buffer.from(buffer), { useTArray: true, formatAsRGBA: true });
  return { width: img.width, height: img.height, data: new Uint8Array(img.data) };
}

/**
 * Comprime as texturas para KTX2/Basis. UASTC (qualidade alta, boa para as cores chapadas e as
 * bordas duras dos atlas) com Zstd por cima e mipmaps; o aparelho transcodifica para BC7/ASTC/ETC
 * e a VRAM fica em 1 byte por pixel em vez dos 4 do PNG decodificado. Mapas que não são cor
 * (normal, rugosidade) não são perceptuais.
 */
export async function compressTextures(doc, { etc1s = false } = {}) {
  const common = { isUASTC: !etc1s, needSupercompression: !etc1s, generateMipmap: true, isKTX2File: true, imageDecoder };
  await doc.transform(
    ktx2({ ...common, slots: /^(baseColorTexture|emissiveTexture)$/, isPerceptual: true, isSetKTX2SRGBTransferFunc: true }),
    ktx2({ ...common, slots: /^(?!baseColorTexture|emissiveTexture)/, isPerceptual: false, isSetKTX2SRGBTransferFunc: false }),
  );
  return doc.getRoot().listTextures().filter((t) => t.getMimeType() === 'image/ktx2').length;
}

/**
 * Reduz as texturas que passam de `maxPx` (maior lado) por média de blocos, direto no GLB, antes
 * da medição do orçamento e do KTX2. Saída em PNG. Pensado para a textura 2K que o Meshy entrega
 * (o guia pede até 1024 px para personagem e monstro). Só divide por potências de 2.
 */
export async function resizeTextures(doc, maxPx) {
  let n = 0;
  for (const tex of doc.getRoot().listTextures()) {
    const [w0, h0] = tex.getSize() ?? [0, 0];
    let f = 1;
    while (Math.max(w0, h0) / f > maxPx) f *= 2;
    if (f === 1) continue;
    const src = await imageDecoder(tex.getImage());
    const w = Math.floor(src.width / f);
    const h = Math.floor(src.height / f);
    const out = new PNG({ width: w, height: h });
    for (let y = 0; y < h; y++) {
      for (let x = 0; x < w; x++) {
        const acc = [0, 0, 0, 0];
        for (let dy = 0; dy < f; dy++) {
          for (let dx = 0; dx < f; dx++) {
            const i = ((y * f + dy) * src.width + (x * f + dx)) * 4;
            for (let c = 0; c < 4; c++) acc[c] += src.data[i + c];
          }
        }
        for (let c = 0; c < 4; c++) out.data[(y * w + x) * 4 + c] = Math.round(acc[c] / (f * f));
      }
    }
    tex.setImage(PNG.sync.write(out)).setMimeType('image/png');
    n++;
  }
  return n;
}
