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
import type { WorldData } from "./world-data";

/** Cor do horizonte ao entardecer (neblina e fundo). */
const HORIZON = new Color3(0.78, 0.6, 0.52);
/** Direção da luz do sol (baixo, fim de tarde). */
const SUN_DIR = new Vector3(-0.55, -0.75, 0.35).normalize();
/** Lado da área coberta pelas sombras, centrada no jogador (m). */
const SHADOW_AREA = 70;

export interface WorldScene {
  engine: AbstractEngine;
  scene: Scene;
  camera: ArcRotateCamera;
  ground: Mesh;
  gui: AdvancedDynamicTexture;
  shadows: ShadowGenerator;
  /** Move a área de sombra para acompanhar o jogador. */
  followShadows(x: number, z: number): void;
}

/**
 * Cria engine, cena, céu, luz com sombras, chão pintado e câmera.
 * WebGL2 por padrão; WebGPU pode ser testado com ?webgpu=1 na URL.
 */
export async function createWorldScene(canvas: HTMLCanvasElement, world: WorldData): Promise<WorldScene> {
  const engine = await createEngine(canvas);
  const scene = new Scene(engine);
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
  const ground = MeshBuilder.CreateGround("ground", { width: world.size, height: world.size, subdivisions: 2 }, scene);
  const groundMat = new StandardMaterial("groundMat", scene);
  groundMat.diffuseTexture = paintGround(scene, world);
  groundMat.specularColor = new Color3(0.03, 0.03, 0.03);
  ground.material = groundMat;
  ground.receiveShadows = true;
  // Fora do mapa: um chão grande e simples até as montanhas.
  const outer = MeshBuilder.CreateGround("outer", { width: 900, height: 900 }, scene);
  outer.position.y = -0.05;
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

  window.addEventListener("resize", () => engine.resize());
  engine.runRenderLoop(() => scene.render());

  return {
    engine,
    scene,
    camera,
    ground,
    gui,
    shadows,
    followShadows(x, z) {
      sun.position.set(x - SUN_DIR.x * 100, -SUN_DIR.y * 100, z - SUN_DIR.z * 100);
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
 * Pinta a textura do chão uma vez: grama com variação, tom mais seco/sombrio longe da
 * cidade, estradas de terra e a praça de pedra dentro da muralha.
 */
function paintGround(scene: Scene, world: WorldData): DynamicTexture {
  const PX = 2048;
  const tex = new DynamicTexture("groundTex", { width: PX, height: PX }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  const k = PX / world.size; // pixels por metro
  // Mundo (x, z) → canvas. O chão do Babylon põe o u ao longo de +X e o v ao longo de +Z.
  const cx = (x: number) => (x + world.size / 2) * k;
  const cy = (z: number) => (world.size / 2 - z) * k;
  const rand = mulberry32(7);

  // Base: grama, escurecendo e secando com a distância.
  const grad = ctx.createRadialGradient(cx(0), cy(0), 20 * k, cx(0), cy(0), 120 * k);
  grad.addColorStop(0, "#5f7d3a");
  grad.addColorStop(0.45, "#4f6d33");
  grad.addColorStop(0.7, "#4a5a35");
  grad.addColorStop(1, "#4a4a3a");
  ctx.fillStyle = grad;
  ctx.fillRect(0, 0, PX, PX);

  // Manchas de variação (rápido: círculos translúcidos).
  const tones = ["#6d8a42", "#557233", "#47622c", "#7a8c48", "#5a5a3c", "#3f5528"];
  for (let i = 0; i < 9000; i++) {
    ctx.globalAlpha = 0.05 + rand() * 0.1;
    ctx.fillStyle = tones[Math.floor(rand() * tones.length)];
    const r = (0.6 + rand() * 4) * k;
    ctx.beginPath();
    ctx.arc(rand() * PX, rand() * PX, r, 0, Math.PI * 2);
    ctx.fill();
  }
  ctx.globalAlpha = 1;

  // Estradas de terra (borda mais escura + miolo + pedrinhas).
  ctx.lineCap = "round";
  for (const [color, extra] of [["#5a4630", 1.2], ["#8a6e4a", 0]] as const) {
    ctx.strokeStyle = color;
    for (const r of world.roads) {
      ctx.lineWidth = (r.w + extra) * k;
      ctx.beginPath();
      ctx.moveTo(cx(r.x1), cy(r.z1));
      ctx.lineTo(cx(r.x2), cy(r.z2));
      ctx.stroke();
    }
  }
  for (const r of world.roads) {
    const len = Math.hypot(r.x2 - r.x1, r.z2 - r.z1);
    for (let i = 0; i < len * 3; i++) {
      const t = rand();
      const off = (rand() - 0.5) * r.w;
      const nx = -(r.z2 - r.z1) / len;
      const nz = (r.x2 - r.x1) / len;
      const x = r.x1 + (r.x2 - r.x1) * t + nx * off;
      const z = r.z1 + (r.z2 - r.z1) * t + nz * off;
      ctx.fillStyle = rand() < 0.5 ? "#6f5a3e" : "#a08560";
      ctx.globalAlpha = 0.5;
      ctx.fillRect(cx(x), cy(z), 0.25 * k, 0.25 * k);
    }
  }
  ctx.globalAlpha = 1;

  // Praça de pedra dentro da muralha (lajotas em anéis).
  const plazaR = world.wall.apothem + 0.5;
  ctx.fillStyle = "#8b8478";
  ctx.beginPath();
  ctx.arc(cx(0), cy(0), plazaR * k, 0, Math.PI * 2);
  ctx.fill();
  ctx.strokeStyle = "rgba(60,55,50,0.35)";
  ctx.lineWidth = 0.08 * k;
  for (let r = 1.5; r < plazaR; r += 1.5) {
    ctx.beginPath();
    ctx.arc(cx(0), cy(0), r * k, 0, Math.PI * 2);
    ctx.stroke();
    const n = Math.floor((2 * Math.PI * r) / 1.6);
    for (let i = 0; i < n; i++) {
      const a = (i / n) * Math.PI * 2 + r;
      ctx.beginPath();
      ctx.moveTo(cx(Math.cos(a) * (r - 1.5)), cy(Math.sin(a) * (r - 1.5)));
      ctx.lineTo(cx(Math.cos(a) * r), cy(Math.sin(a) * r));
      ctx.stroke();
    }
  }
  for (let i = 0; i < 2500; i++) {
    const a = rand() * Math.PI * 2;
    const r = Math.sqrt(rand()) * plazaR;
    ctx.fillStyle = rand() < 0.5 ? "rgba(70,65,58,0.18)" : "rgba(170,160,145,0.18)";
    ctx.fillRect(cx(Math.cos(a) * r), cy(Math.sin(a) * r), 0.5 * k, 0.5 * k);
  }

  tex.update(true);
  tex.anisotropicFilteringLevel = 8;
  tex.wrapU = tex.wrapV = Texture.CLAMP_ADDRESSMODE;
  return tex;
}

function mulberry32(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
