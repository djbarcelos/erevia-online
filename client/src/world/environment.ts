import "@babylonjs/loaders/glTF";
import {
  AbstractMesh,
  LoadAssetContainerAsync,
  Matrix,
  Mesh,
  Quaternion,
  Scene,
  ShadowGenerator,
  Vector3,
} from "@babylonjs/core";
import type { WorldData } from "./world-data";

/** Tipos que projetam sombra (grama, arbustos e adereços pequenos não, para economizar). */
const CASTS_SHADOW = /^(tree_|rock_3|tavern|church|blacksmith|market|home_|well|windmill|tower_|wall_)/;
/** Tipos que recebem sombra (construções e muralhas ficam mais "assentadas" no chão). */
const RECEIVES_SHADOW = /^(tavern|church|blacksmith|market|home_|windmill|tower_|wall_|well)/;

/**
 * Cenário: carrega os kits (um .glb por pacote, um item = uma malha) e desenha cada
 * tipo de item com thin instances — milhares de objetos em poucas dezenas de draw calls.
 */
export async function loadEnvironment(scene: Scene, world: WorldData, shadows: ShadowGenerator): Promise<number> {
  const containers = await Promise.all(
    ["/models/env-forest.glb", "/models/env-town.glb"].map((f) => LoadAssetContainerAsync(f, scene)),
  );

  // nome do item → malha (já sem a hierarquia do glTF, com o transform aplicado nos vértices)
  const protos = new Map<string, Mesh>();
  for (const c of containers) {
    c.addAllToScene();
    for (const node of c.transformNodes) {
      if (!node.name.startsWith("item:")) continue;
      const mesh = node.getChildMeshes(false).find((m): m is Mesh => m instanceof Mesh && m.getTotalVertices() > 0);
      if (!mesh) continue;
      mesh.setParent(null); // mantém a posição no mundo (inclui a conversão de eixos do glTF)
      mesh.bakeCurrentTransformIntoVertices();
      protos.set(node.name.slice(5), mesh);
    }
    for (const root of c.rootNodes) if (root.name === "__root__") root.dispose();
  }

  // Agrupa as posições por tipo e cria um buffer de matrizes para cada um.
  const byKind = new Map<string, WorldData["objects"]>();
  for (const o of world.objects) {
    let list = byKind.get(o.k);
    if (!list) byKind.set(o.k, (list = []));
    list.push(o);
  }

  const scale = new Vector3();
  const pos = new Vector3();
  const rot = new Quaternion();
  const m = new Matrix();
  let drawn = 0;
  for (const [kind, list] of byKind) {
    const mesh = protos.get(kind);
    if (!mesh) {
      console.warn(`cenário: item sem modelo: ${kind}`);
      continue;
    }
    const buf = new Float32Array(list.length * 16);
    list.forEach((o, i) => {
      scale.set(o.s[0], o.s[1], o.s[2]);
      Quaternion.RotationYawPitchRollToRef(o.r, 0, 0, rot);
      pos.set(o.x, 0, o.z);
      Matrix.ComposeToRef(scale, rot, pos, m);
      m.copyToArray(buf, i * 16);
    });
    mesh.thinInstanceSetBuffer("matrix", buf, 16, true);
    mesh.thinInstanceRefreshBoundingInfo(false);
    prepareStatic(mesh);
    if (CASTS_SHADOW.test(kind)) shadows.addShadowCaster(mesh, false);
    if (RECEIVES_SHADOW.test(kind)) mesh.receiveShadows = true;
    drawn += list.length;
  }
  // Protótipos não usados no layout ficam invisíveis.
  for (const [kind, mesh] of protos) if (!byKind.has(kind)) mesh.setEnabled(false);
  return drawn;
}

function prepareStatic(mesh: AbstractMesh): void {
  mesh.isPickable = false; // o clique atravessa o cenário e chega ao chão
  mesh.freezeWorldMatrix();
  mesh.doNotSyncBoundingInfo = true;
}
