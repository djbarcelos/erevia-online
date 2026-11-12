import {
  AbstractEngine,
  ArcRotateCamera,
  Color3,
  Color4,
  DirectionalLight,
  Engine,
  HemisphericLight,
  Mesh,
  MeshBuilder,
  Scene,
  StandardMaterial,
  Vector3,
  WebGPUEngine,
} from "@babylonjs/core";
import { AdvancedDynamicTexture } from "@babylonjs/gui";
import { GridMaterial } from "@babylonjs/materials";

const SKY = new Color3(0.07, 0.06, 0.1);

export interface WorldScene {
  engine: AbstractEngine;
  scene: Scene;
  camera: ArcRotateCamera;
  ground: Mesh;
  gui: AdvancedDynamicTexture;
}

/**
 * Cria engine, cena, luz, chão e câmera.
 * WebGL2 por padrão; WebGPU pode ser testado com ?webgpu=1 na URL.
 */
export async function createWorldScene(canvas: HTMLCanvasElement, zoneSize: number, safeRadius = 15): Promise<WorldScene> {
  const engine = await createEngine(canvas);
  const scene = new Scene(engine);
  scene.clearColor = new Color4(SKY.r, SKY.g, SKY.b, 1);
  scene.ambientColor = new Color3(0.25, 0.22, 0.3);
  scene.fogMode = Scene.FOGMODE_EXP2;
  scene.fogDensity = 0.012;
  scene.fogColor = SKY;

  // Luz: céu frio + "sol" de fim de tarde.
  const hemi = new HemisphericLight("hemi", new Vector3(0, 1, 0), scene);
  hemi.intensity = 0.55;
  hemi.groundColor = new Color3(0.15, 0.12, 0.2);
  const sun = new DirectionalLight("sun", new Vector3(-0.5, -1, 0.3), scene);
  sun.intensity = 0.9;
  sun.diffuse = new Color3(1, 0.88, 0.7);

  // Chão em grade: 1 linha por metro, linha forte a cada 10 m.
  const ground = MeshBuilder.CreateGround("ground", { width: zoneSize, height: zoneSize }, scene);
  const grid = new GridMaterial("grid", scene);
  grid.mainColor = new Color3(0.16, 0.2, 0.14);
  grid.lineColor = new Color3(0.42, 0.5, 0.36);
  grid.gridRatio = 1;
  grid.majorUnitFrequency = 10;
  grid.minorUnitVisibility = 0.25;
  grid.opacity = 0.99;
  ground.material = grid;

  addTown(scene, safeRadius);
  addScenery(scene, zoneSize, safeRadius);

  // Câmera estilo L2: orbita o personagem, botão direito gira, roda dá zoom.
  const camera = new ArcRotateCamera("cam", -Math.PI / 2, 1.0, 18, Vector3.Zero(), scene);
  camera.attachControl(canvas, true);
  camera.lowerRadiusLimit = 5;
  camera.upperRadiusLimit = 45;
  camera.lowerBetaLimit = 0.35;
  camera.upperBetaLimit = 1.4;
  camera.wheelDeltaPercentage = 0.02;
  camera.panningSensibility = 0; // sem pan: a câmera segue o jogador
  camera.inputs.attached.pointers && ((camera.inputs.attached.pointers as unknown as { buttons: number[] }).buttons = [2]);
  camera.inputs.remove(camera.inputs.attached.keyboard);
  canvas.addEventListener("contextmenu", (e) => e.preventDefault());

  const gui = AdvancedDynamicTexture.CreateFullscreenUI("ui", true, scene);

  window.addEventListener("resize", () => engine.resize());
  engine.runRenderLoop(() => scene.render());

  return { engine, scene, camera, ground, gui };
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

/** Praça de pedra marcando a cidade (zona segura, sem combate). */
function addTown(scene: Scene, radius: number): void {
  const plaza = MeshBuilder.CreateDisc("town", { radius, tessellation: 64 }, scene);
  plaza.rotation.x = Math.PI / 2;
  plaza.position.y = 0.01;
  const mat = new StandardMaterial("townMat", scene);
  mat.diffuseColor = new Color3(0.42, 0.39, 0.36);
  mat.specularColor = new Color3(0.05, 0.05, 0.05);
  plaza.material = mat;
  plaza.isPickable = false; // o clique passa para o chão

  const border = MeshBuilder.CreateTorus("townBorder", { diameter: radius * 2, thickness: 0.35, tessellation: 96 }, scene);
  border.position.y = 0.05;
  const bmat = new StandardMaterial("townBorderMat", scene);
  bmat.diffuseColor = new Color3(0.3, 0.27, 0.24);
  border.material = bmat;
  border.isPickable = false;

  // Postes com luz nas quatro direções.
  const postMat = new StandardMaterial("postMat", scene);
  postMat.diffuseColor = new Color3(0.25, 0.2, 0.16);
  const lampMat = new StandardMaterial("lampMat", scene);
  lampMat.emissiveColor = new Color3(1, 0.78, 0.4);
  lampMat.disableLighting = true;
  for (let i = 0; i < 8; i++) {
    const a = (i / 8) * Math.PI * 2;
    const post = MeshBuilder.CreateCylinder(`post${i}`, { height: 2.6, diameter: 0.18, tessellation: 8 }, scene);
    post.position.set(Math.cos(a) * radius, 1.3, Math.sin(a) * radius);
    post.material = postMat;
    post.isPickable = false;
    const lamp = MeshBuilder.CreateSphere(`lamp${i}`, { diameter: 0.35, segments: 8 }, scene);
    lamp.position.set(post.position.x, 2.7, post.position.z);
    lamp.material = lampMat;
    lamp.isPickable = false;
  }
}

/** Árvores e pedras em posições fixas (mesma semente = mesmo mapa para todos). */
function addScenery(scene: Scene, zoneSize: number, safeRadius: number): void {
  const rand = mulberry32(20261112);
  const half = zoneSize / 2 - 4;

  const trunkMat = new StandardMaterial("trunk", scene);
  trunkMat.diffuseColor = new Color3(0.32, 0.22, 0.14);
  const leafMat = new StandardMaterial("leaf", scene);
  leafMat.diffuseColor = new Color3(0.14, 0.3, 0.16);
  const rockMat = new StandardMaterial("rock", scene);
  rockMat.diffuseColor = new Color3(0.35, 0.34, 0.38);

  const trunk = MeshBuilder.CreateCylinder("trunk", { height: 2, diameter: 0.4, tessellation: 8 }, scene);
  trunk.material = trunkMat;
  const leaves = MeshBuilder.CreateCylinder("leaves", { height: 4, diameterTop: 0, diameterBottom: 2.6, tessellation: 8 }, scene);
  leaves.material = leafMat;
  const rock = MeshBuilder.CreateIcoSphere("rock", { radius: 0.9, subdivisions: 1, flat: true }, scene);
  rock.material = rockMat;

  for (const m of [trunk, leaves, rock]) {
    m.isPickable = false;
    m.setEnabled(false); // os originais ficam escondidos; usamos instâncias
  }

  for (let i = 0; i < 160; i++) {
    const x = (rand() * 2 - 1) * half;
    const z = (rand() * 2 - 1) * half;
    if (Math.hypot(x, z) < safeRadius + 3) continue; // deixa a cidade livre
    const s = 0.8 + rand() * 0.8;

    if (rand() < 0.75) {
      const t = trunk.createInstance(`t${i}`);
      t.position.set(x, 1 * s, z);
      t.scaling.setAll(s);
      const l = leaves.createInstance(`l${i}`);
      l.position.set(x, 3.6 * s, z);
      l.scaling.setAll(s);
      t.isPickable = l.isPickable = false;
    } else {
      const r = rock.createInstance(`r${i}`);
      r.position.set(x, 0.3 * s, z);
      r.scaling.set(s * 1.3, s * 0.8, s);
      r.rotation.y = rand() * Math.PI;
      r.isPickable = false;
    }
  }
}

/** PRNG pequeno e determinístico. */
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
