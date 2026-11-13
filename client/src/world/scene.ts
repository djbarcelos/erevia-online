import {
  AbstractEngine,
  ArcRotateCamera,
  Color3,
  Color4,
  DirectionalLight,
  DynamicTexture,
  Engine,
  HemisphericLight,
  Mesh,
  MeshBuilder,
  Scene,
  ShadowGenerator,
  StandardMaterial,
  Texture,
  Vector3,
  WebGPUEngine,
} from "@babylonjs/core";
import { AdvancedDynamicTexture } from "@babylonjs/gui";
import { addGroundDetail } from "./ground";
import { paintTerrain, projection } from "./map-painter";
import { registerPaletteMaterial } from "./palette-material";
import { worldBounds, type WorldData } from "./world-data";

/** Cor do horizonte ao entardecer (valor inicial; o ciclo dia/noite muda depois). */
const HORIZON = new Color3(0.78, 0.6, 0.52);
/** Direção inicial da luz do sol (baixo, fim de tarde). */
const SUN_DIR = new Vector3(-0.55, -0.75, 0.35).normalize();
/** Distância do sol até o centro da área de sombra (m). */
const SUN_DISTANCE = 100;
/** Lado da área coberta pelas sombras, centrada no jogador (m). */
const SHADOW_AREA = 70;

export interface WorldScene {
  engine: AbstractEngine;
  scene: Scene;
  camera: ArcRotateCamera;
  ground: Mesh;
  gui: AdvancedDynamicTexture;
  shadows: ShadowGenerator;
  sun: DirectionalLight;
  hemi: HemisphericLight;
  /** Céu: esfera com um degradê vertical (canvas 4×512) pintado pelo ciclo dia/noite. */
  sky: { mesh: Mesh; texture: DynamicTexture };
  /** Material do chão de fora do mapa. */
  outerMat: StandardMaterial;
  /** Move a área de sombra para acompanhar o jogador. */
  followShadows(x: number, z: number): void;
}

/**
 * Cria engine, cena, céu, luz com sombras, chão pintado e câmera.
 * WebGL2 por padrão; WebGPU pode ser testado com ?webgpu=1 na URL.
 */
export async function createWorldScene(canvas: HTMLCanvasElement, world: WorldData): Promise<WorldScene> {
  const engine = await createEngine(canvas);
  // Material de paleta em todo material Standard/PBR criado daqui em diante (só GLSL).
  if (!engine.isWebGPU) registerPaletteMaterial();
  const scene = new Scene(engine);
  // O Babylon faz picking a cada movimento do mouse contra todas as malhas clicáveis; o jogo
  // só precisa de pick no clique e no hover (feito à mão, com intervalo), então desligamos.
  scene.skipPointerMovePicking = true;
  // A esfera do céu cobre a tela inteira: não precisa limpar a cor (a profundidade continua).
  scene.autoClear = false;
  scene.clearColor = new Color4(HORIZON.r, HORIZON.g, HORIZON.b, 1);
  scene.ambientColor = new Color3(0.3, 0.27, 0.32);
  scene.fogMode = Scene.FOGMODE_EXP2;
  scene.fogDensity = 0.0085;
  scene.fogColor = HORIZON;

  // Céu de fim de tarde: esfera grande com degradê (zênite → horizonte), sem luz nem neblina.
  const skybox = MeshBuilder.CreateSphere("sky", { diameter: 1600, segments: 24, sideOrientation: Mesh.BACKSIDE }, scene);
  const skyTex = new DynamicTexture("skyTex", { width: 4, height: 512 }, scene, false);
  const sctx = skyTex.getContext() as CanvasRenderingContext2D;
  // v=0 embaixo da esfera, v=1 em cima; o canvas desenha de cima para baixo.
  const g = sctx.createLinearGradient(0, 0, 0, 512);
  g.addColorStop(0, "#2c3f6e"); // zênite
  g.addColorStop(0.32, "#6b6f9a");
  g.addColorStop(0.46, "#d79a7f"); // perto do horizonte
  g.addColorStop(0.5, "#e8b48c");
  g.addColorStop(0.56, "#9e8578"); // abaixo do horizonte (quase nunca visível)
  g.addColorStop(1, "#5b5450");
  sctx.fillStyle = g;
  sctx.fillRect(0, 0, 4, 512);
  skyTex.update(true);
  const skyMat = new StandardMaterial("skyMat", scene);
  skyMat.emissiveTexture = skyTex;
  skyMat.diffuseColor = Color3.Black();
  skyMat.specularColor = Color3.Black();
  skyMat.disableLighting = true;
  skyMat.fogEnabled = false;
  skybox.material = skyMat;
  skybox.isPickable = false;
  skybox.infiniteDistance = true;

  // Luz: céu + sol quente com sombras.
  const hemi = new HemisphericLight("hemi", new Vector3(0, 1, 0), scene);
  hemi.intensity = 0.62;
  hemi.diffuse = new Color3(0.85, 0.85, 1);
  hemi.groundColor = new Color3(0.28, 0.24, 0.2);
  const sun = new DirectionalLight("sun", SUN_DIR, scene);
  sun.intensity = 1.6;
  sun.diffuse = new Color3(1, 0.86, 0.68);
  sun.autoUpdateExtends = false;
  sun.shadowFrustumSize = SHADOW_AREA;
  sun.shadowMinZ = 1;
  sun.shadowMaxZ = 200;
  const shadows = new ShadowGenerator(2048, sun);
  shadows.usePercentageCloserFiltering = true;
  shadows.filteringQuality = ShadowGenerator.QUALITY_MEDIUM;
  shadows.bias = 0.0015;
  shadows.normalBias = 0.02;
  shadows.darkness = 0.35;

  // Chão pintado (grama, estradas, praça).
  // O mapa é um retângulo (cresceu só para o sul): o chão fica centrado nele.
  const b = worldBounds(world);
  const ground = MeshBuilder.CreateGround("ground", { width: b.maxX - b.minX, height: b.maxZ - b.minZ, subdivisions: 2 }, scene);
  ground.position.set((b.minX + b.maxX) / 2, 0, (b.minZ + b.maxZ) / 2);
  const groundMat = new StandardMaterial("groundMat", scene);
  groundMat.diffuseTexture = paintGround(scene, world);
  groundMat.specularColor = new Color3(0.03, 0.03, 0.03);
  ground.material = groundMat;
  ground.receiveShadows = true;
  addGroundDetail(scene, world, groundMat);
  // Fora do mapa: um chão grande e simples até as montanhas.
  const outer = MeshBuilder.CreateGround("outer", { width: 1200, height: 1200 }, scene);
  outer.position.set((b.minX + b.maxX) / 2, -0.05, (b.minZ + b.maxZ) / 2);
  const outerMat = new StandardMaterial("outerMat", scene);
  outerMat.diffuseColor = new Color3(0.2, 0.26, 0.14);
  outerMat.specularColor = Color3.Black();
  outer.material = outerMat;
  outer.isPickable = false;

  // Câmera estilo L2: orbita o personagem, botão direito gira, roda dá zoom.
  const camera = new ArcRotateCamera("cam", -Math.PI / 2, 1.0, 18, Vector3.Zero(), scene);
  camera.attachControl(canvas, true);
  camera.lowerRadiusLimit = 5;
  camera.upperRadiusLimit = 45;
  camera.lowerBetaLimit = 0.35;
  camera.upperBetaLimit = 1.4;
  camera.wheelDeltaPercentage = 0.02;
  camera.maxZ = 2000;
  camera.panningSensibility = 0; // sem pan: a câmera segue o jogador
  // Babylon 9 usa um mapa de entrada: por padrão o botão direito faz "pan" (que está
  // desligado acima), então nada acontecia. Aqui o direito passa a girar a câmera.
  camera.movement.input.setInteraction("pointer", { button: 2 }, "rotate");
  // Só o botão direito mexe na câmera; o esquerdo fica livre para andar/atacar.
  const pointers = camera.inputs.attached.pointers as unknown as { buttons: number[] } | undefined;
  if (pointers) pointers.buttons = [2];
  camera.inputs.remove(camera.inputs.attached.keyboard);
  canvas.addEventListener("contextmenu", (e) => e.preventDefault());

  const gui = AdvancedDynamicTexture.CreateFullscreenUI("ui", true, scene);
  // Nomes, barras e números de dano ficam fora do pós-processamento (SSAO e FXAA do preset alto).
  if (gui.layer) gui.layer.applyPostProcess = false;

  window.addEventListener("resize", () => engine.resize());
  engine.runRenderLoop(() => scene.render());

  return {
    engine,
    scene,
    camera,
    ground,
    gui,
    shadows,
    sun,
    hemi,
    sky: { mesh: skybox, texture: skyTex },
    outerMat,
    followShadows(x, z) {
      // Usa a direção atual da luz (o ciclo dia/noite move o sol e a lua).
      const d = sun.direction;
      sun.position.set(x - d.x * SUN_DISTANCE, -d.y * SUN_DISTANCE, z - d.z * SUN_DISTANCE);
    },
  };
}

async function createEngine(canvas: HTMLCanvasElement): Promise<AbstractEngine> {
  const wantWebGPU = new URLSearchParams(location.search).has("webgpu");
  if (wantWebGPU && (await WebGPUEngine.IsSupportedAsync)) {
    const gpu = new WebGPUEngine(canvas, { antialias: true });
    await gpu.initAsync();
    return gpu;
  }
  return new Engine(canvas, true, { preserveDrawingBuffer: false, stencil: true });
}

/**
 * Pinta a textura do chão uma vez (o mesmo desenho do mapa). Com o mapa retangular, a
 * textura segue a proporção e fica com ~8 px por metro (o lado maior cabe em 4096).
 */
function paintGround(scene: Scene, world: WorldData): DynamicTexture {
  const b = worldBounds(world);
  const w = b.maxX - b.minX;
  const d = b.maxZ - b.minZ;
  const k = Math.min(10.24, 4096 / Math.max(w, d));
  const proj = projection(world, Math.round(w * k));
  const tex = new DynamicTexture("groundTex", { width: proj.width, height: proj.height }, scene, true);
  // O chão do Babylon põe o u ao longo de +X e o v ao longo de +Z: o mesmo do canvas do mapa.
  paintTerrain(tex.getContext() as CanvasRenderingContext2D, world, proj.width);
  tex.update(true);
  tex.anisotropicFilteringLevel = 8;
  tex.wrapU = tex.wrapV = Texture.CLAMP_ADDRESSMODE;
  return tex;
}
