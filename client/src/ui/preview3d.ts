/**
 * Palco 3D do personagem (criação, seleção e atributos): pedestal de pedra com anel de luz
 * na cor do emblema, luz de recorte colorida, girar arrastando e zoom (roda ou pinça).
 */

import {
  ArcRotateCamera,
  Color3,
  Color4,
  DirectionalLight,
  DynamicTexture,
  Engine,
  GlowLayer,
  HemisphericLight,
  MeshBuilder,
  PointLight,
  Scene,
  ShadowGenerator,
  StandardMaterial,
  TransformNode,
  Vector3,
} from "@babylonjs/core";
import type { ClassId } from "../api";
import { AnimController } from "../world/anim";
import { CharacterModels, type AnimRole, type CharacterInstance } from "../world/models";

/** Aparência mostrada no palco: o que vai em cada mão e a cor do emblema. */
export interface Appearance {
  /** Arquivo da arma da mão direita/esquerda (em /models/weapons); null = mão vazia. */
  right: string | null;
  left: string | null;
  emblem: string;
}

export interface Preview {
  /** Troca a classe e a aparência (recria o modelo). */
  show(cls: ClassId, look: Appearance): Promise<void>;
  setEmblem(color: string): void;
  play(role: AnimRole): void;
  /** Gira o personagem um pouco (botões ◀ ▶). */
  spin(delta: number): void;
  ready: Promise<void>;
  dispose(): void;
}

export function createPreview(canvas: HTMLCanvasElement, opts: { radius?: number; interactive?: boolean } = {}): Preview {
  const engine = new Engine(canvas, true, { alpha: true, premultipliedAlpha: false, preserveDrawingBuffer: false, stencil: true }, true);
  engine.setHardwareScalingLevel(1 / Math.min(2, devicePixelRatio || 1));
  const scene = new Scene(engine);
  scene.clearColor = new Color4(0, 0, 0, 0);
  scene.ambientColor = new Color3(0.25, 0.22, 0.28);

  const camera = new ArcRotateCamera("cam", -Math.PI / 2, 1.38, opts.radius ?? 5.8, new Vector3(0, 0.95, 0), scene);
  camera.lowerRadiusLimit = 2.6;
  camera.upperRadiusLimit = 7.5;
  camera.lowerBetaLimit = 0.9;
  camera.upperBetaLimit = 1.6;
  camera.wheelDeltaPercentage = 0.015;
  camera.pinchDeltaPercentage = 0.004;
  camera.panningSensibility = 0;
  camera.minZ = 0.1;
  if (opts.interactive !== false) {
    camera.attachControl(canvas, true);
    camera.useAutoRotationBehavior = true;
    const auto = camera.autoRotationBehavior!;
    auto.idleRotationSpeed = 0.18;
    auto.idleRotationWaitTime = 4000;
    auto.idleRotationSpinupTime = 1500;
  }

  const hemi = new HemisphericLight("hemi", new Vector3(0, 1, 0), scene);
  hemi.intensity = 0.55;
  hemi.diffuse = new Color3(0.85, 0.82, 1);
  hemi.groundColor = new Color3(0.2, 0.16, 0.18);

  const key = new DirectionalLight("key", new Vector3(0.5, -1, 0.8).normalize(), scene);
  key.position = new Vector3(-4, 8, -6);
  key.intensity = 1.5;
  key.diffuse = new Color3(1, 0.9, 0.78);

  const rim = new PointLight("rim", new Vector3(0, 2.6, 2.4), scene);
  rim.intensity = 18;
  rim.range = 8;

  const shadows = new ShadowGenerator(1024, key);
  shadows.useBlurExponentialShadowMap = true;
  shadows.blurKernel = 24;
  shadows.darkness = 0.4;

  const glow = new GlowLayer("glow", scene, { mainTextureRatio: 0.5, blurKernelSize: 40 });
  glow.intensity = 0.35;

  // Pedestal: pedra escura com borda dourada e anel de luz em cima.
  const stone = MeshBuilder.CreateCylinder("pedestal", { diameterTop: 2.2, diameterBottom: 2.5, height: 0.32, tessellation: 48 }, scene);
  stone.position.y = -0.16;
  const stoneMat = new StandardMaterial("stoneMat", scene);
  stoneMat.diffuseColor = new Color3(0.16, 0.14, 0.2);
  stoneMat.specularColor = new Color3(0.08, 0.08, 0.1);
  stone.material = stoneMat;
  stone.receiveShadows = true;

  const trim = MeshBuilder.CreateTorus("trim", { diameter: 2.22, thickness: 0.05, tessellation: 64 }, scene);
  trim.position.y = 0.005;
  const trimMat = new StandardMaterial("trimMat", scene);
  trimMat.diffuseColor = new Color3(0.84, 0.7, 0.43);
  trimMat.emissiveColor = new Color3(0.3, 0.24, 0.12);
  trimMat.specularPower = 32;
  trim.material = trimMat;

  const ringTex = new DynamicTexture("ringTex", { width: 256, height: 256 }, scene, false);
  const rctx = ringTex.getContext() as CanvasRenderingContext2D;
  const g = rctx.createRadialGradient(128, 128, 30, 128, 128, 128);
  g.addColorStop(0, "rgba(255,255,255,0)");
  g.addColorStop(0.62, "rgba(255,255,255,0.05)");
  g.addColorStop(0.8, "rgba(255,255,255,0.9)");
  g.addColorStop(0.86, "rgba(255,255,255,0.3)");
  g.addColorStop(1, "rgba(255,255,255,0)");
  rctx.fillStyle = g;
  rctx.fillRect(0, 0, 256, 256);
  // Runas em volta
  rctx.strokeStyle = "rgba(255,255,255,0.55)";
  rctx.lineWidth = 2;
  for (let i = 0; i < 24; i++) {
    const a = (i / 24) * Math.PI * 2;
    const r1 = 92;
    const r2 = 100;
    rctx.beginPath();
    rctx.moveTo(128 + Math.cos(a) * r1, 128 + Math.sin(a) * r1);
    rctx.lineTo(128 + Math.cos(a + 0.08) * r2, 128 + Math.sin(a + 0.08) * r2);
    rctx.stroke();
  }
  ringTex.hasAlpha = true;
  ringTex.update();
  const ring = MeshBuilder.CreateGround("ring", { width: 2.6, height: 2.6 }, scene);
  ring.position.y = 0.012;
  const ringMat = new StandardMaterial("ringMat", scene);
  ringMat.diffuseTexture = ringTex;
  ringMat.opacityTexture = ringTex;
  ringMat.emissiveColor = new Color3(1, 0.8, 0.4);
  ringMat.disableLighting = true;
  ringMat.alpha = 0.7;
  ring.material = ringMat;
  glow.addIncludedOnlyMesh(ring);

  // Partículas leves de poeira de luz subindo do anel (esferas pequenas animadas).
  const motes: { m: ReturnType<typeof MeshBuilder.CreateSphere>; a: number; r: number; s: number; o: number }[] = [];
  const moteMat = new StandardMaterial("moteMat", scene);
  moteMat.disableLighting = true;
  moteMat.emissiveColor = new Color3(1, 0.8, 0.4);
  for (let i = 0; i < 14; i++) {
    const m = MeshBuilder.CreateSphere("mote", { diameter: 0.035, segments: 4 }, scene);
    m.material = moteMat;
    glow.addIncludedOnlyMesh(m);
    motes.push({ m, a: Math.random() * Math.PI * 2, r: 0.85 + Math.random() * 0.25, s: 0.2 + Math.random() * 0.35, o: Math.random() * 2.4 });
  }

  const holder = new TransformNode("holder", scene);
  holder.rotation.y = Math.PI; // de frente para a câmera
  let inst: CharacterInstance | null = null;
  let anim: AnimController | null = null;
  let seq = 0;
  let targetSpin = holder.rotation.y;

  // Cada palco tem a sua cena; o navegador reaproveita os .glb já baixados (cache HTTP).
  const models = CharacterModels.load(scene);

  let t = 0;
  scene.onBeforeRenderObservable.add(() => {
    const dt = engine.getDeltaTime() / 1000;
    t += dt;
    anim?.update(dt, false, 1);
    holder.rotation.y += (targetSpin - holder.rotation.y) * Math.min(1, dt * 8);
    for (const p of motes) {
      const k = ((t * p.s + p.o) % 2.4) / 2.4;
      p.m.position.set(Math.cos(p.a + t * 0.2) * p.r, k * 2.2, Math.sin(p.a + t * 0.2) * p.r);
      p.m.visibility = Math.sin(k * Math.PI);
    }
  });

  engine.runRenderLoop(() => scene.render());
  const ro = new ResizeObserver(() => engine.resize());
  ro.observe(canvas);

  const setEmblem = (hex: string) => {
    const c = Color3.FromHexString(hex);
    ringMat.emissiveColor = c.scale(0.75);
    moteMat.emissiveColor = c.scale(1.2);
    rim.diffuse = c;
    rim.specular = c;
  };

  const show = async (cls: ClassId, look: Appearance) => {
    const my = ++seq;
    const m = await models;
    if (my !== seq || scene.isDisposed) return;
    inst?.dispose();
    inst = m.instantiate(cls, `pv${my}`, { "handslot.r": look.right, "handslot.l": look.left });
    if (!inst) return;
    inst.root.parent = holder;
    for (const mesh of inst.root.getChildMeshes()) {
      shadows.addShadowCaster(mesh);
      mesh.isPickable = false;
    }
    anim = new AnimController(inst.anims, inst.resetPose);
    setEmblem(look.emblem);
  };

  return {
    show,
    setEmblem,
    play: (role) => anim?.play(role),
    spin: (d) => (targetSpin += d),
    ready: models.then(() => undefined),
    dispose() {
      seq++;
      ro.disconnect();
      engine.stopRenderLoop();
      scene.dispose();
      engine.dispose();
    },
  };
}
