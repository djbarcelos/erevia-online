/**
 * Praça do remaster (tunnel#93, ?remaster=1): a fonte no lugar do poço (mesma pegada, r = 1,7 m),
 * o piso em rosa dos ventos em volta dela e quatro postes com estandarte nas diagonais, de frente
 * para o centro. Peças do kit `plaza_*_b.glb` (Blender, estilo B). Os prédios continuam os atuais.
 */
import { AbstractMesh, LoadAssetContainerAsync, Mesh, type Scene, type ShadowGenerator } from "@babylonjs/core";
import { LOD_DISTANCES } from "./remaster";

const BASE = "/models/";

/** Carrega uma peça: devolve as malhas por nome, já no espaço do mundo (sem o __root__ do glTF). */
async function loadPiece(scene: Scene, file: string): Promise<Map<string, Mesh>> {
  const c = await LoadAssetContainerAsync(BASE + file, scene);
  c.addAllToScene();
  const out = new Map<string, Mesh>();
  for (const m of c.meshes) {
    if (!(m instanceof Mesh) || !m.getTotalVertices()) continue;
    m.setParent(null);
    m.bakeCurrentTransformIntoVertices();
    out.set(m.name.replace(/\.\d+$/, ""), m);
  }
  for (const r of c.rootNodes) if (r.name === "__root__") r.dispose();
  return out;
}

/** Liga o LOD1 e o LOD2 (nós `<nome>_LOD1`, `<nome>_LOD2`) ao nó principal. */
function wireLods(parts: Map<string, Mesh>, name: string): Mesh | null {
  const main = parts.get(name);
  if (!main) return null;
  [1, 2].forEach((n) => {
    const lod = parts.get(`${name}_LOD${n}`);
    if (lod) main.addLODLevel(LOD_DISTANCES[n - 1], lod);
  });
  // Sem LOD2, o LOD1 vale até longe; sem nenhum LOD, a peça fica como está.
  return main;
}

function fix(m: AbstractMesh, shadows: ShadowGenerator | null, caster: boolean): void {
  m.isPickable = false; // o clique atravessa e chega ao chão
  m.receiveShadows = true;
  if (caster) shadows?.addShadowCaster(m, false);
  m.freezeWorldMatrix();
  m.doNotSyncBoundingInfo = true;
}

export async function addRemasterPlaza(scene: Scene, shadows: ShadowGenerator | null): Promise<void> {
  const [fountain, paving, banner] = await Promise.all([
    loadPiece(scene, "plaza_Fountain_b.glb"),
    loadPiece(scene, "plaza_Paving_b.glb"),
    loadPiece(scene, "plaza_BannerPost_b.glb"),
  ]);

  const f = wireLods(fountain, "Fountain");
  if (f) fix(f, shadows, true);
  const p = wireLods(paving, "Paving");
  if (p) {
    p.position.y = 0.045; // acima do piso da praça (0,02) e das estradas (0,03)
    p.material!.zOffset = -3;
    fix(p, shadows, false);
  }
  const b = wireLods(banner, "BannerPost");
  if (b) {
    for (const [i, [x, z]] of ([[9, 9], [-9, 9], [-9, -9], [9, -9]] as const).entries()) {
      const inst = i === 0 ? b : b.createInstance(`BannerPost${i}`);
      inst.position.set(x, 0, z);
      // A frente (o emblema) é -Z no modelo: vira para o centro.
      inst.rotation.y = Math.atan2(x, z);
      fix(inst, shadows, true);
    }
    // O poste 0 é o próprio molde: já está no lugar.
  }
  for (const m of [...fountain.values(), ...paving.values(), ...banner.values()]) {
    if (m.name.includes("_LOD")) m.isPickable = false;
  }
}
