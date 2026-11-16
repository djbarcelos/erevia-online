import "@babylonjs/loaders/glTF";
import { AbstractMesh, KhronosTextureContainer2, Mesh, type Node } from "@babylonjs/core";

/**
 * Carregamento dos assets do pipeline de arte (scripts/import-asset.mjs): texturas KTX2/Basis
 * e LOD por nós `<nome>_LOD1` / `<nome>_LOD2`. Os modelos KayKit atuais não usam nenhum dos dois.
 */

/**
 * Aponta o decodificador de KTX2 do Babylon para os arquivos do próprio site (public/ktx2/,
 * gerados por scripts/build-ktx2-decoder.mjs), em vez de cdn.babylonjs.com. O worker do Babylon
 * nasce de um blob e só entende URL absoluta. Chame uma vez, antes do primeiro GLB com KTX2.
 */
export function configureKtx2(origin: string = location.origin, base = "/ktx2/"): void {
  const url = (file: string) => new URL(base + file, origin).href;
  Object.assign(KhronosTextureContainer2.URLConfig, {
    jsDecoderModule: url("babylon.ktx2Decoder.js"),
    wasmUASTCToASTC: url("uastc_astc.wasm"),
    wasmUASTCToBC7: url("uastc_bc7.wasm"),
    wasmUASTCToRGBA_UNORM: url("uastc_rgba8_unorm_v2.wasm"),
    wasmUASTCToRGBA_SRGB: url("uastc_rgba8_srgb_v2.wasm"),
    wasmUASTCToR8_UNORM: url("uastc_r8_unorm.wasm"),
    wasmUASTCToRG8_UNORM: url("uastc_rg8_unorm.wasm"),
    jsMSCTranscoder: url("msc_basis_transcoder.js"),
    wasmMSCTranscoder: url("msc_basis_transcoder.wasm"),
    wasmZSTDDecoder: url("zstddec.wasm"),
  });
}

/** Distâncias de troca de LOD do guia de arte (seção 5), em metros: LOD1 a ~20 m e LOD2 a ~35 m. */
export const LOD_DISTANCES = [20, 35] as const;

const LOD_NAME = /^(.*)_LOD([12])(?:#\d+)?$/i;

/**
 * Liga os nós `<nome>_LOD1` e `<nome>_LOD2` ao `<nome>` (LOD0) com o LOD do Babylon: a malha
 * troca sozinha pela distância da câmera, e os LOD deixam de ser desenhados à parte.
 * Chame na cópia final de cada entidade (os LOD têm de ser da mesma instância do LOD0, para
 * dividir o mesmo esqueleto). Retorna quantas malhas ganharam LOD.
 */
export function applyLods(meshes: readonly AbstractMesh[], distances: readonly number[] = LOD_DISTANCES): number {
  const byName = new Map<string, AbstractMesh>();
  for (const m of meshes) byName.set(baseName(m), m);
  let n = 0;
  for (const m of meshes) {
    const hit = LOD_NAME.exec(baseName(m));
    if (!hit || !(m instanceof Mesh)) continue;
    const base = byName.get(hit[1]!);
    if (!(base instanceof Mesh)) continue;
    // Do mais perto para o mais longe: addLODLevel guarda em ordem de distância.
    base.addLODLevel(distances[Number(hit[2]) - 1] ?? distances[distances.length - 1]!, m);
    n++;
  }
  return n;
}

const baseName = (node: Node): string => node.name.split("#")[0]!;
