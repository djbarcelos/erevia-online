/**
 * Cidade de Vel'Aran no estilo novo (tunnel#100, ?remaster=1): prédios e adereços gerados pelo
 * Meshy no lugar das peças do kit KayKit, no MESMO layout do world.json (mesma pegada, mesma
 * orientação, mesmo colisor no servidor). Cada peça é um GLB próprio (`town_<peça>_b.glb`, 1
 * material, sem LOD: são thin instances, que dividem uma malha só) e a escala do world.json já
 * vem embutida na malha (scripts/build-remaster-town.mjs), então o `environment.ts` usa a peça
 * como se fosse o item antigo.
 */
import { LoadAssetContainerAsync, Mesh, PBRMaterial, type Scene } from "@babylonjs/core";
import { EXTRA_FILES, TOWN_KITS } from "./town-layout";

const BASE = "/models/";

/** Carrega as peças: tipo → malha (já no espaço do mundo, sem o __root__ do glTF). */
export async function loadTownProtos(scene: Scene): Promise<Map<string, Mesh>> {
  const files = { ...TOWN_KITS, ...EXTRA_FILES };
  const out = new Map<string, Mesh>();
  await Promise.all(
    Object.entries(files).map(async ([kind, file]) => {
      const c = await LoadAssetContainerAsync(BASE + file, scene);
      c.addAllToScene();
      const mesh = c.meshes.find((m): m is Mesh => m instanceof Mesh && m.getTotalVertices() > 0);
      if (!mesh) return;
      mesh.setParent(null);
      mesh.bakeCurrentTransformIntoVertices();
      mesh.name = `town:${kind}`;
      // Pedra, madeira e telha pintadas: sem metal e sem brilho de plástico.
      const mat = mesh.material;
      if (mat instanceof PBRMaterial) {
        mat.metallic = 0;
        mat.roughness = 1;
      }
      for (const r of c.rootNodes) if (r.name === "__root__") r.dispose();
      out.set(kind, mesh);
    }),
  );
  return out;
}
