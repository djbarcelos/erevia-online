// Página mínima do teste de navegador e2e/tests/assets.spec.ts: carrega um GLB do pipeline de
// arte (LOD + KTX2) com o mesmo código do jogo (src/world/assets.ts) e reporta o que o Babylon
// entendeu. Empacotada por tests/assets/build-page.mjs; não faz parte do build do jogo.
import "@babylonjs/loaders/glTF";
import { ArcRotateCamera, Engine, HemisphericLight, LoadAssetContainerAsync, Mesh, Scene, Texture, Vector3 } from "@babylonjs/core";
import { applyLods, configureKtx2 } from "../../src/world/assets";

declare global {
  interface Window {
    runAssetLoad: (glb: string) => Promise<unknown>;
  }
}

window.runAssetLoad = async (glb: string) => {
  const canvas = document.getElementById("c") as HTMLCanvasElement;
  const engine = new Engine(canvas, true, { preserveDrawingBuffer: true });
  const scene = new Scene(engine);
  const camera = new ArcRotateCamera("cam", 0, 1, 5, Vector3.Zero(), scene);
  new HemisphericLight("sol", new Vector3(0, 0, -1), scene);
  configureKtx2();
  const container = await LoadAssetContainerAsync(glb, scene);
  container.addAllToScene();
  const meshes = container.meshes.filter((m): m is Mesh => m instanceof Mesh && m.getTotalVertices() > 0);
  const lods = applyLods(meshes);

  // A textura KTX2 decodifica num worker: espera ficar pronta.
  const tex = meshes.map((m) => m.material).flatMap((m) => m?.getActiveTextures() ?? [])[0] as Texture | undefined;
  if (tex) await new Promise<void>((ok) => (tex.isReady() ? ok() : tex.onLoadObservable.addOnce(() => ok())));
  const size = tex?.getSize();

  const base = meshes.find((m) => m.name === "Corpo")!;
  const center = base.getBoundingInfo().boundingSphere.centerWorld.clone();
  const pick = (distance: number) => {
    camera.radius = distance;
    camera.target = center;
    camera.getViewMatrix(true);
    return base.getLOD(camera)?.name ?? null;
  };
  const lodAt = { perto: pick(5), medio: pick(25), longe: pick(40) };

  // Desenha de verdade (com o esqueleto, o material e a textura) e vê se algum pixel mudou do fundo.
  scene.clearColor.set(0, 0, 0, 1);
  camera.alpha = -Math.PI / 2;
  camera.beta = Math.PI / 2;
  camera.radius = 3;
  camera.target = center;
  for (const m of meshes) if (m.material) m.material.backFaceCulling = false;
  scene.render();
  await scene.whenReadyAsync();
  scene.render();
  const gl = engine._gl;
  const px = new Uint8Array(4);
  gl.readPixels(canvas.width / 2, canvas.height / 2, 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, px);
  return {
    meshes: meshes.map((m) => m.name),
    lods,
    textureReady: !!tex && tex.isReady(),
    textureSize: size ? [size.width, size.height] : null,
    textureFormat: tex?.getInternalTexture()?.format ?? null,
    lodAt,
    centerPixel: Array.from(px),
  };
};
