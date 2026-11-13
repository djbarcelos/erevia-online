import {
  Color3,
  Color4,
  Constants,
  CylinderDirectedParticleEmitter,
  DynamicTexture,
  Effect,
  Material,
  Matrix,
  Mesh,
  MeshBuilder,
  ParticleSystem,
  PointLight,
  Quaternion,
  ShaderMaterial,
  StandardMaterial,
  Texture,
  Vector3,
  VertexData,
  type GlowLayer,
  type Scene,
} from "@babylonjs/core";
import { mulberry32 } from "./map-painter";
import type { WorldScene } from "./scene";
import { heightToNormal, outward, rubbleMesh } from "./teleport-stone";

/**
 * Tochas da cidade: poste de madeira torto e texturizado, braseiro de ferro com garras,
 * lenha em brasa, chama desenhada por shader (ruído que sobe, cada tocha no seu ritmo),
 * poça de luz quente no chão, faíscas e fumaça. As 35 tochas são thin instances; as 3
 * luzes de verdade (e as partículas) vão para as tochas mais próximas do jogador.
 */

/** Altura do pé da chama (dentro do braseiro). */
const FLAME_BASE = 2.5;
/** Altura do poste. */
const POST_H = 2.42;
const TORCH_LIGHTS = 3;
const TORCH_RANGE = 14;

// ---------------------------------------------------------------------------------------
// Texturas pintadas
// ---------------------------------------------------------------------------------------

function canvasTex(scene: Scene, name: string, w: number, h: number, paint: (ctx: CanvasRenderingContext2D) => void): DynamicTexture {
  const tex = new DynamicTexture(name, { width: w, height: h }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  paint(ctx);
  tex.update(true);
  tex.wrapU = Texture.WRAP_ADDRESSMODE;
  tex.wrapV = Texture.CLAMP_ADDRESSMODE;
  tex.anisotropicFilteringLevel = 4;
  return tex;
}

/** Desenha também do outro lado da emenda horizontal. */
function wrapX(w: number, x: number, r: number, draw: (dx: number) => void): void {
  draw(0);
  if (x - r < 0) draw(w);
  if (x + r > w) draw(-w);
}

function spot(ctx: CanvasRenderingContext2D, w: number, x: number, y: number, r: number, color: string): void {
  wrapX(w, x, r, (dx) => {
    const g = ctx.createRadialGradient(x + dx, y, 0, x + dx, y, r);
    g.addColorStop(0, color);
    g.addColorStop(1, color.replace(/[\d.]+\)$/, "0)"));
    ctx.fillStyle = g;
    ctx.fillRect(x + dx - r, y - r, r * 2, r * 2);
  });
}

interface Knot {
  x: number;
  y: number;
  r: number;
}

/** Veios de madeira: linhas verticais onduladas que desviam dos nós. */
function grain(ctx: CanvasRenderingContext2D, W: number, H: number, knots: Knot[], rand: () => number, style: (i: number) => [string, number]): void {
  for (let i = 0; i < 90; i++) {
    const x0 = rand() * W;
    const ph = rand() * 6.28;
    const [color, width] = style(i);
    ctx.strokeStyle = color;
    ctx.lineWidth = width;
    wrapX(W, x0, 30, (dx) => {
      ctx.beginPath();
      for (let y = 0; y <= H; y += 6) {
        let x = x0 + Math.sin(y * 0.018 + ph) * 3 + Math.sin(y * 0.061 + ph * 2) * 1.2;
        for (const k of knots) {
          const d = y - k.y;
          const near = Math.exp(-(d * d) / (k.r * k.r * 4));
          x += Math.sign(x - k.x || 1) * near * k.r * 1.3 * Math.exp(-Math.abs(x - k.x) / (k.r * 2));
        }
        if (y === 0) ctx.moveTo(x + dx, y);
        else ctx.lineTo(x + dx, y);
      }
      ctx.stroke();
    });
  }
}

function knotMarks(ctx: CanvasRenderingContext2D, W: number, knots: Knot[], dark: string, ring: string): void {
  for (const k of knots) {
    wrapX(W, k.x, k.r * 2, (dx) => {
      for (let r = k.r * 1.8; r > 1; r -= 2.5) {
        ctx.strokeStyle = ring;
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.ellipse(k.x + dx, k.y, r * 0.7, r, 0, 0, Math.PI * 2);
        ctx.stroke();
      }
      ctx.fillStyle = dark;
      ctx.beginPath();
      ctx.ellipse(k.x + dx, k.y, k.r * 0.45, k.r * 0.65, 0, 0, Math.PI * 2);
      ctx.fill();
    });
  }
}

interface Assets {
  wood: StandardMaterial;
  iron: StandardMaterial;
  stone: StandardMaterial;
  logs: StandardMaterial;
  pool: StandardMaterial;
  smokeTex: Texture;
  sparkTex: Texture;
}

function makeAssets(scene: Scene): Assets {
  // Madeira do poste (u = volta, v = altura).
  const W = 256;
  const H = 512;
  const rk = mulberry32(404);
  const knots: Knot[] = Array.from({ length: 4 }, () => ({ x: rk() * W, y: H * (0.25 + rk() * 0.6), r: 6 + rk() * 6 }));
  const woodAlbedo = canvasTex(scene, "torchWoodTex", W, H, (ctx) => {
    const rand = mulberry32(17);
    const base = ctx.createLinearGradient(0, 0, 0, H);
    base.addColorStop(0, "#5c4330");
    base.addColorStop(1, "#4a3424");
    ctx.fillStyle = base;
    ctx.fillRect(0, 0, W, H);
    for (let i = 0; i < 40; i++) spot(ctx, W, rand() * W, rand() * H, 10 + rand() * 40, `rgba(${rand() < 0.5 ? "40,26,16" : "120,92,64"},${(0.1 + rand() * 0.15).toFixed(2)})`);
    grain(ctx, W, H, knots, rand, () => (rand() < 0.6 ? [`rgba(34,22,14,${(0.25 + rand() * 0.35).toFixed(2)})`, 1 + rand() * 2] : [`rgba(150,118,84,${(0.15 + rand() * 0.2).toFixed(2)})`, 1]));
    knotMarks(ctx, W, knots, "rgba(30,18,10,0.85)", "rgba(40,26,16,0.35)");
    // Topo chamuscado pelo braseiro e base suja de terra.
    const top = ctx.createLinearGradient(0, 0, 0, H * 0.18);
    top.addColorStop(0, "rgba(18,12,8,0.9)");
    top.addColorStop(1, "rgba(18,12,8,0)");
    ctx.fillStyle = top;
    ctx.fillRect(0, 0, W, H * 0.18);
    const foot = ctx.createLinearGradient(0, H * 0.88, 0, H);
    foot.addColorStop(0, "rgba(50,40,30,0)");
    foot.addColorStop(1, "rgba(50,40,30,0.7)");
    ctx.fillStyle = foot;
    ctx.fillRect(0, H * 0.88, W, H * 0.12);
    // Rachaduras secas.
    ctx.strokeStyle = "rgba(20,12,8,0.7)";
    ctx.lineWidth = 1.2;
    for (let i = 0; i < 10; i++) {
      const x = rand() * W;
      const y = rand() * H;
      const len = 30 + rand() * 60;
      wrapX(W, x, 8, (dx) => {
        ctx.beginPath();
        ctx.moveTo(x + dx, y);
        ctx.lineTo(x + dx + (rand() - 0.5) * 6, y + len * 0.5);
        ctx.lineTo(x + dx + (rand() - 0.5) * 6, y + len);
        ctx.stroke();
      });
    }
  });
  const woodBump = canvasTex(scene, "torchWoodBump", W, H, (ctx) => {
    const rand = mulberry32(17);
    ctx.fillStyle = "rgb(128,128,128)";
    ctx.fillRect(0, 0, W, H);
    grain(ctx, W, H, knots, rand, () => (rand() < 0.6 ? ["rgba(0,0,0,0.55)", 1.5 + rand() * 2] : ["rgba(255,255,255,0.35)", 1.5]));
    knotMarks(ctx, W, knots, "rgba(255,255,255,0.6)", "rgba(0,0,0,0.4)");
    heightToNormal(ctx, W, H, 2.6);
  });
  const wood = new StandardMaterial("torchWood", scene);
  wood.diffuseTexture = woodAlbedo;
  woodBump.level = 0.9;
  wood.bumpTexture = woodBump;
  wood.specularColor = new Color3(0.06, 0.05, 0.04);

  // Ferro forjado com ferrugem.
  const iron = new StandardMaterial("torchIron", scene);
  iron.diffuseTexture = canvasTex(scene, "torchIronTex", 128, 128, (ctx) => {
    const rand = mulberry32(88);
    ctx.fillStyle = "#3a3633";
    ctx.fillRect(0, 0, 128, 128);
    for (let i = 0; i < 40; i++) spot(ctx, 128, rand() * 128, rand() * 128, 4 + rand() * 16, `rgba(${rand() < 0.55 ? "128,64,30" : "20,18,18"},${(0.25 + rand() * 0.35).toFixed(2)})`);
    for (let i = 0; i < 600; i++) {
      ctx.fillStyle = rand() < 0.5 ? "rgba(10,10,10,0.35)" : "rgba(170,150,130,0.18)";
      ctx.fillRect(rand() * 128, rand() * 128, 1, 1);
    }
  });
  iron.specularColor = new Color3(0.28, 0.25, 0.22);
  iron.specularPower = 32;
  iron.backFaceCulling = false;

  // Pedras da base.
  const stone = new StandardMaterial("torchStone", scene);
  stone.diffuseTexture = canvasTex(scene, "torchStoneTex", 128, 128, (ctx) => {
    const rand = mulberry32(61);
    ctx.fillStyle = "#77736b";
    ctx.fillRect(0, 0, 128, 128);
    for (let i = 0; i < 50; i++) spot(ctx, 128, rand() * 128, rand() * 128, 4 + rand() * 18, `rgba(${rand() < 0.5 ? "50,48,44" : "160,156,146"},${(0.15 + rand() * 0.25).toFixed(2)})`);
    for (let i = 0; i < 25; i++) spot(ctx, 128, rand() * 128, 90 + rand() * 38, 3 + rand() * 8, `rgba(88,112,52,${(0.3 + rand() * 0.3).toFixed(2)})`);
  });
  stone.specularColor = new Color3(0.05, 0.05, 0.05);

  // Lenha queimando: carvão e cinza, brasa nas rachaduras (a cor da brasa está na textura).
  const cracks: [number, number][][] = [];
  const rc = mulberry32(5);
  for (let i = 0; i < 14; i++) {
    let x = rc() * 128;
    let y = rc() * 128;
    const pts: [number, number][] = [[x, y]];
    for (let k = 0; k < 5; k++) {
      x += (rc() - 0.5) * 18;
      y += 6 + rc() * 10;
      pts.push([x, y]);
    }
    cracks.push(pts);
  }
  const drawCracks = (ctx: CanvasRenderingContext2D) => {
    for (const pts of cracks) {
      ctx.beginPath();
      pts.forEach(([x, y], k) => (k ? ctx.lineTo(x, y) : ctx.moveTo(x, y)));
      ctx.stroke();
    }
  };
  const logs = new StandardMaterial("torchLogs", scene);
  logs.diffuseTexture = canvasTex(scene, "torchLogTex", 128, 128, (ctx) => {
    const rand = mulberry32(9);
    ctx.fillStyle = "#1e1814";
    ctx.fillRect(0, 0, 128, 128);
    for (let i = 0; i < 30; i++) spot(ctx, 128, rand() * 128, rand() * 128, 3 + rand() * 12, `rgba(120,112,104,${(0.2 + rand() * 0.35).toFixed(2)})`);
    ctx.strokeStyle = "rgba(8,6,4,0.9)";
    ctx.lineWidth = 2;
    drawCracks(ctx);
  });
  logs.emissiveTexture = canvasTex(scene, "torchEmberTex", 128, 128, (ctx) => {
    ctx.fillStyle = "#000";
    ctx.fillRect(0, 0, 128, 128);
    ctx.lineCap = "round";
    ctx.shadowColor = "rgba(255,110,20,1)";
    ctx.shadowBlur = 6;
    ctx.strokeStyle = "rgb(255,120,30)";
    ctx.lineWidth = 2;
    drawCracks(ctx);
    ctx.shadowBlur = 0;
    ctx.strokeStyle = "rgb(255,210,120)";
    ctx.lineWidth = 0.8;
    drawCracks(ctx);
  });
  logs.emissiveColor = Color3.Black();
  logs.useEmissiveAsIllumination = true;
  logs.specularColor = Color3.Black();

  // Poça de luz quente no chão (faz o papel da luz nas tochas sem PointLight).
  const pool = new StandardMaterial("torchPool", scene);
  pool.emissiveColor = new Color3(0.75, 0.38, 0.12);
  pool.diffuseColor = Color3.Black();
  pool.specularColor = Color3.Black();
  pool.disableLighting = true;
  pool.opacityTexture = radial(scene, "torchPoolTex", 256, [
    [0, "rgba(255,255,255,1)"],
    [0.3, "rgba(255,255,255,0.5)"],
    [0.65, "rgba(255,255,255,0.15)"],
    [1, "rgba(255,255,255,0)"],
  ]);
  pool.alphaMode = Constants.ALPHA_ADD;
  pool.transparencyMode = Material.MATERIAL_ALPHABLEND;
  pool.disableDepthWrite = true;
  // O piso da praça e das estradas usa zOffset negativo: sem isto a poça fica por baixo.
  pool.zOffset = -4;

  const smokeTex = canvasTex(scene, "torchSmokeTex", 64, 64, (ctx) => {
    const rand = mulberry32(3);
    ctx.clearRect(0, 0, 64, 64);
    for (let i = 0; i < 8; i++) {
      const x = 32 + (rand() - 0.5) * 22;
      const y = 32 + (rand() - 0.5) * 22;
      const g = ctx.createRadialGradient(x, y, 0, x, y, 14 + rand() * 10);
      g.addColorStop(0, `rgba(255,255,255,${(0.25 + rand() * 0.2).toFixed(2)})`);
      g.addColorStop(1, "rgba(255,255,255,0)");
      ctx.fillStyle = g;
      ctx.fillRect(0, 0, 64, 64);
    }
  });
  smokeTex.hasAlpha = true;
  const sparkTex = radial(scene, "torchSparkTex", 32, [
    [0, "rgba(255,255,255,1)"],
    [0.3, "rgba(255,255,255,0.6)"],
    [1, "rgba(255,255,255,0)"],
  ]);
  return { wood, iron, stone, logs, pool, smokeTex, sparkTex };
}

function radial(scene: Scene, name: string, size: number, stops: [number, string][]): DynamicTexture {
  const tex = new DynamicTexture(name, { width: size, height: size }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  ctx.clearRect(0, 0, size, size);
  const g = ctx.createRadialGradient(size / 2, size / 2, 0, size / 2, size / 2, size / 2);
  for (const [o, c] of stops) g.addColorStop(o, c);
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, size, size);
  tex.update(true);
  tex.hasAlpha = true;
  tex.wrapU = tex.wrapV = Texture.CLAMP_ADDRESSMODE;
  return tex;
}

// ---------------------------------------------------------------------------------------
// Geometria
// ---------------------------------------------------------------------------------------

/** Poste: torno com veios, nós saltados e leve afinamento. */
function postGeometry(): VertexData {
  const rand = mulberry32(2024);
  const ph = Array.from({ length: 4 }, () => rand() * 6.28);
  const knots = Array.from({ length: 3 }, () => ({ th: rand() * 6.28, y: 0.5 + rand() * 1.6 }));
  const SEG = 14;
  const RINGS = 22;
  const positions: number[] = [];
  const uvs: number[] = [];
  const indices: number[] = [];
  for (let j = 0; j <= RINGS; j++) {
    const v = j / RINGS;
    const y = v * POST_H;
    const r0 = 0.13 - 0.035 * v + 0.035 * Math.max(0, 1 - v / 0.06) ** 2;
    for (let i = 0; i <= SEG; i++) {
      const u = i / SEG;
      const th = u * Math.PI * 2;
      let n = 0.07 * Math.sin(3 * th + ph[0] + y * 1.7) + 0.045 * Math.sin(5 * th + ph[1] - y * 2.9) + 0.03 * Math.sin(y * 5.3 + ph[2]);
      for (const k of knots) {
        const dth = Math.atan2(Math.sin(th - k.th), Math.cos(th - k.th));
        n += 0.22 * Math.exp(-(dth * dth) / 0.18 - ((y - k.y) * (y - k.y)) / 0.004);
      }
      const r = r0 * (1 + n);
      // Leve curva (madeira de verdade não é reta).
      const bend = 0.04 * Math.sin((y / POST_H) * Math.PI + ph[3]);
      positions.push(Math.sin(th) * r + bend, y - 0.03, Math.cos(th) * r);
      uvs.push(u, v);
    }
  }
  const row = SEG + 1;
  for (let j = 0; j < RINGS; j++) {
    for (let i = 0; i < SEG; i++) {
      const a = j * row + i;
      indices.push(a, a + row, a + 1, a + 1, a + row, a + row + 1);
    }
  }
  const normals = outward(positions, indices, Math.floor(RINGS / 2) * row + 3);
  for (let j = 0; j <= RINGS; j++) {
    const a = j * row * 3;
    const b = (j * row + SEG) * 3;
    for (let c = 0; c < 3; c++) normals[a + c] = normals[b + c] = (normals[a + c] + normals[b + c]) / 2;
  }
  const vd = new VertexData();
  vd.positions = positions;
  vd.indices = indices;
  vd.normals = normals;
  vd.uvs = uvs;
  return vd;
}

/** Ferragens: cintas no poste, colar e braseiro em cesto com garras curvas. */
function ironMesh(scene: Scene): Mesh {
  const parts: Mesh[] = [];
  for (const [y, d] of [
    [0.55, 0.25],
    [1.95, 0.205],
  ] as const) {
    const band = MeshBuilder.CreateTorus("torchBand", { diameter: d, thickness: 0.028, tessellation: 18 }, scene);
    band.position.y = y;
    band.scaling.y = 1.6;
    parts.push(band);
  }
  const collar = MeshBuilder.CreateCylinder("torchCollar", { height: 0.12, diameterTop: 0.22, diameterBottom: 0.19, tessellation: 14 }, scene);
  collar.position.y = 2.36;
  parts.push(collar);
  // Tigela rasa (perfil curvo).
  const profile: Vector3[] = [];
  for (let k = 0; k <= 8; k++) {
    const t = k / 8;
    profile.push(new Vector3(0.07 + 0.15 * Math.sin((t * Math.PI) / 2), 2.4 + 0.15 * t * t, 0));
  }
  const bowl = MeshBuilder.CreateLathe("torchBowl", { shape: profile, tessellation: 18, sideOrientation: Mesh.DOUBLESIDE }, scene);
  parts.push(bowl);
  const rim = MeshBuilder.CreateTorus("torchRim", { diameter: 0.44, thickness: 0.032, tessellation: 22 }, scene);
  rim.position.y = 2.55;
  parts.push(rim);
  // Garras: sobem abrindo e curvam para dentro na ponta.
  const CLAWS = 6;
  for (let c = 0; c < CLAWS; c++) {
    const a = (c / CLAWS) * Math.PI * 2;
    const path: Vector3[] = [];
    for (let k = 0; k <= 10; k++) {
      const t = k / 10;
      const r = 0.21 + 0.07 * Math.sin(t * Math.PI * 0.85) - 0.09 * t * t * t;
      const y = 2.55 + 0.3 * t - 0.04 * t * t * t;
      path.push(new Vector3(Math.sin(a) * r, y, Math.cos(a) * r));
    }
    const claw = MeshBuilder.CreateTube("torchClaw", { path, radiusFunction: (i) => 0.017 * (1 - (i / 10) * 0.7), tessellation: 6, cap: Mesh.CAP_END }, scene);
    parts.push(claw);
  }
  return Mesh.MergeMeshes(parts, true, true)!;
}

/** Lenha em pé, cruzada como uma fogueirinha dentro do braseiro. */
function logsMesh(scene: Scene): Mesh {
  const parts: Mesh[] = [];
  for (let i = 0; i < 4; i++) {
    const a = (i / 4) * Math.PI * 2 + 0.4;
    const log = MeshBuilder.CreateCylinder("torchLog", { height: 0.3, diameterTop: 0.05, diameterBottom: 0.07, tessellation: 8 }, scene);
    log.position.set(Math.sin(a) * 0.06, 2.6, Math.cos(a) * 0.06);
    log.rotation.set(Math.cos(a) * 0.45, 0, -Math.sin(a) * 0.45);
    parts.push(log);
  }
  return Mesh.MergeMeshes(parts, true, true)!;
}

// ---------------------------------------------------------------------------------------
// Chama (shader)
// ---------------------------------------------------------------------------------------

Effect.ShadersStore["torchFlameVertexShader"] = `
precision highp float;
attribute vec3 position;
attribute vec2 uv;
#include<instancesDeclaration>
uniform mat4 viewProjection;
uniform vec3 cameraPosition;
uniform float intensity;
varying vec2 vUV;
varying float vSeed;
void main(void) {
  #include<instancesVertex>
  vec3 center = finalWorld[3].xyz;
  vec3 toCam = cameraPosition - center;
  toCam.y = 0.0;
  vec3 fwd = normalize(toCam + vec3(0.0001, 0.0, 0.0));
  vec3 right = vec3(fwd.z, 0.0, -fwd.x);
  float s = 0.35 + 0.65 * intensity;
  vec3 p = center + right * position.x * s + vec3(0.0, position.y * s, 0.0);
  gl_Position = viewProjection * vec4(p, 1.0);
  vUV = uv;
  vSeed = fract(sin(dot(center.xz, vec2(12.9898, 78.233))) * 43758.5453);
}`;

Effect.ShadersStore["torchFlameFragmentShader"] = `
precision highp float;
varying vec2 vUV;
varying float vSeed;
uniform float time;
uniform float intensity;
float hash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }
float noise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  f = f * f * (3.0 - 2.0 * f);
  return mix(mix(hash(i), hash(i + vec2(1.0, 0.0)), f.x), mix(hash(i + vec2(0.0, 1.0)), hash(i + vec2(1.0, 1.0)), f.x), f.y);
}
float fbm(vec2 p) {
  float v = 0.0;
  float a = 0.5;
  for (int i = 0; i < 4; i++) { v += a * noise(p); p *= 2.03; a *= 0.5; }
  return v;
}
void main(void) {
  float t = time * (0.9 + vSeed * 0.35) + vSeed * 37.0;
  float y = vUV.y;
  float x = (vUV.x - 0.5) * 2.0;
  // A chama balança mais no alto.
  x += (noise(vec2(t * 1.1, 3.0 + vSeed * 5.0)) - 0.5) * 0.7 * y * y;
  float n = fbm(vec2(x * 1.7 + vSeed * 9.0, y * 2.4 - t * 2.4));
  float n2 = fbm(vec2(x * 3.1 - vSeed * 4.0, y * 4.0 - t * 3.6));
  // Lágrima: larga embaixo, fina na ponta, bordas comidas pelo ruído.
  // Línguas: o ruído fino abre "dedos" de fogo que sobem e se soltam.
  float tongues = 0.75 + 0.55 * n2 + 0.25 * sin(x * 7.0 + t * 3.0 + n * 6.0);
  float width = mix(0.9, 0.04, pow(y, 1.15)) * (0.62 + 0.5 * n) * tongues;
  float body = 1.0 - smoothstep(width * 0.3, width, abs(x));
  float base = smoothstep(0.0, 0.07, y);
  float tip = 1.0 - smoothstep(0.4 + 0.45 * n2, 0.98, y);
  float shape = body * base * tip;
  float heat = shape * (1.3 - y * 0.95) * (0.6 + 0.75 * n2) * (1.0 + 0.2 * (1.0 - abs(x)));
  vec3 col = mix(vec3(0.45, 0.05, 0.01), vec3(1.0, 0.38, 0.06), smoothstep(0.05, 0.4, heat));
  col = mix(col, vec3(1.0, 0.72, 0.28), smoothstep(0.4, 0.8, heat));
  col = mix(col, vec3(1.0, 0.93, 0.72), smoothstep(1.0, 1.35, heat));
  float a = clamp(heat * 1.5, 0.0, 0.95) * intensity;
  gl_FragColor = vec4(col, a);
}`;

function flameMaterial(scene: Scene): ShaderMaterial {
  const m = new ShaderMaterial(
    "torchFlame",
    scene,
    { vertex: "torchFlame", fragment: "torchFlame" },
    {
      attributes: ["position", "uv"],
      uniforms: ["world", "viewProjection", "cameraPosition", "time", "intensity"],
      needAlphaBlending: true,
    },
  );
  m.alphaMode = Constants.ALPHA_ADD;
  m.backFaceCulling = false;
  m.disableDepthWrite = true;
  return m;
}

// ---------------------------------------------------------------------------------------
// Tochas
// ---------------------------------------------------------------------------------------

/** Cintilação suave (-1..1) de cada chama. */
function flicker(t: number, seed: number): number {
  return Math.sin(t * 9.1 + seed) * 0.45 + Math.sin(t * 15.7 + seed * 2.3) * 0.25 + Math.sin(t * 4.3 + seed * 0.7) * 0.2 + Math.sin(t * 23.3 + seed * 3.1) * 0.1;
}

function smooth(t: number): number {
  const c = Math.max(0, Math.min(1, t));
  return c * c * (3 - 2 * c);
}

interface NearFx {
  sparks: ParticleSystem;
  smoke: ParticleSystem;
  at: Vector3;
}

export class Torches {
  private flame: Mesh;
  private flameMat: ShaderMaterial;
  private logsMat: StandardMaterial;
  private pool: Mesh;
  /** Ponto do fogo de cada tocha (no alto do poste, que é torto). */
  private tops: Vector3[];
  private seeds: number[];
  private lights: PointLight[] = [];
  private near: NearFx[] = [];
  private nextPick = 0;

  constructor(
    scene: Scene,
    private spots: { x: number; z: number }[],
    world: WorldScene,
    glow: GlowLayer,
  ) {
    const a = makeAssets(scene);
    const rand = mulberry32(31337);

    // Matrizes: cada tocha gira e entorta um pouco.
    const base = new Float32Array(spots.length * 16);
    const flames = new Float32Array(spots.length * 16);
    const pools = new Float32Array(spots.length * 16);
    const m = new Matrix();
    const q = new Quaternion();
    this.tops = [];
    spots.forEach((s, i) => {
      Quaternion.FromEulerAnglesToRef((rand() - 0.5) * 0.06, rand() * Math.PI * 2, (rand() - 0.5) * 0.06, q);
      Matrix.ComposeToRef(Vector3.One(), q, new Vector3(s.x, 0, s.z), m);
      m.copyToArray(base, i * 16);
      const top = Vector3.TransformCoordinates(new Vector3(0, FLAME_BASE, 0), m);
      this.tops.push(top);
      Matrix.Translation(top.x, top.y, top.z).copyToArray(flames, i * 16);
      Matrix.ComposeToRef(new Vector3(1, 1, 1), Quaternion.RotationYawPitchRoll(rand() * 6.28, 0, 0), new Vector3(s.x, 0.065, s.z), m);
      m.copyToArray(pools, i * 16);
    });
    this.seeds = spots.map((_, i) => i * 1.731 + 0.37);

    const post = new Mesh("torchPost", scene);
    postGeometry().applyToMesh(post);
    post.material = a.wood;
    const iron = ironMesh(scene);
    iron.name = "torchIron";
    iron.material = a.iron;
    const logs = logsMesh(scene);
    logs.name = "torchLogs";
    logs.material = a.logs;
    const rubble = rubbleMesh(scene, "torchRubble", 77, { count: 4, ring: [0.16, 0.28], size: [0.07, 0.13] });
    rubble.material = a.stone;
    for (const mesh of [post, iron, logs, rubble]) {
      mesh.thinInstanceSetBuffer("matrix", base.slice(), 16, true);
      mesh.isPickable = false;
      mesh.receiveShadows = true;
      mesh.freezeWorldMatrix();
      mesh.alwaysSelectAsActiveMesh = false;
      if (mesh !== logs) world.shadows.addShadowCaster(mesh, false);
    }

    // Chama: um cartão que vira para a câmera (no shader), base no centro de baixo.
    this.flame = MeshBuilder.CreatePlane("torchFlame", { width: 0.62, height: 0.9 }, scene);
    this.flame.bakeTransformIntoVertices(Matrix.Translation(0, 0.45 - 0.07, 0));
    this.flameMat = flameMaterial(scene);
    this.flame.material = this.flameMat;
    this.flame.thinInstanceSetBuffer("matrix", flames, 16, true);
    this.flame.isPickable = false;
    // As instâncias ficam em volta da cidade inteira: sem isto o Babylon recorta pela caixa da malha base.
    this.flame.alwaysSelectAsActiveMesh = true;
    glow.addIncludedOnlyMesh(this.flame);
    glow.referenceMeshToUseItsOwnMaterial(this.flame);

    this.pool = MeshBuilder.CreateGround("torchPool", { width: 4.6, height: 4.6 }, scene);
    this.pool.material = a.pool;
    this.pool.thinInstanceSetBuffer("matrix", pools, 16, true);
    this.pool.isPickable = false;
    this.pool.alwaysSelectAsActiveMesh = true;

    this.logsMat = a.logs;

    for (let i = 0; i < TORCH_LIGHTS; i++) {
      const l = new PointLight(`torchLight${i}`, new Vector3(0, -50, 0), scene);
      l.diffuse = new Color3(1, 0.6, 0.28);
      l.specular = new Color3(0.4, 0.25, 0.1);
      l.range = TORCH_RANGE;
      l.intensity = 0;
      this.lights.push(l);
      this.near.push(this.nearFx(scene, a, i));
    }
  }

  /** Faíscas e fumaça que acompanham uma das tochas mais próximas. */
  private nearFx(scene: Scene, a: Assets, i: number): NearFx {
    const at = new Vector3(0, -50, 0);
    const sparks = new ParticleSystem(`torchSparks${i}`, 40, scene);
    sparks.particleTexture = a.sparkTex;
    sparks.emitter = at;
    sparks.particleEmitterType = new CylinderDirectedParticleEmitter(0.08, 0.05, 1, new Vector3(-0.25, 1, -0.25), new Vector3(0.25, 1.4, 0.25));
    sparks.updateSpeed = 1 / 60;
    sparks.minLifeTime = 0.6;
    sparks.maxLifeTime = 1.4;
    sparks.minSize = 0.025;
    sparks.maxSize = 0.06;
    sparks.minEmitPower = 0.6;
    sparks.maxEmitPower = 1.5;
    sparks.gravity = new Vector3(0, 0.25, 0);
    sparks.addColorGradient(0, new Color4(1.4, 0.8, 0.3, 1));
    sparks.addColorGradient(0.6, new Color4(1.2, 0.45, 0.1, 0.9));
    sparks.addColorGradient(1, new Color4(0.6, 0.1, 0, 0));
    sparks.addDragGradient(0, 0.3);
    sparks.blendMode = ParticleSystem.BLENDMODE_ADD;
    sparks.emitRate = 0;
    sparks.start();

    const smoke = new ParticleSystem(`torchSmoke${i}`, 40, scene);
    smoke.particleTexture = a.smokeTex;
    smoke.emitter = at;
    smoke.particleEmitterType = new CylinderDirectedParticleEmitter(0.06, 0.05, 1, new Vector3(-0.12, 1, -0.12), new Vector3(0.12, 1, 0.12));
    smoke.updateSpeed = 1 / 60;
    smoke.minLifeTime = 2.2;
    smoke.maxLifeTime = 3.6;
    smoke.minEmitPower = 0.35;
    smoke.maxEmitPower = 0.6;
    smoke.addSizeGradient(0, 0.18, 0.26);
    smoke.addSizeGradient(1, 0.7, 1.1);
    smoke.addColorGradient(0, new Color4(0.25, 0.22, 0.2, 0));
    smoke.addColorGradient(0.15, new Color4(0.25, 0.22, 0.2, 0.22));
    smoke.addColorGradient(1, new Color4(0.3, 0.3, 0.3, 0));
    smoke.minAngularSpeed = -0.6;
    smoke.maxAngularSpeed = 0.6;
    smoke.minInitialRotation = 0;
    smoke.maxInitialRotation = Math.PI * 2;
    smoke.gravity = new Vector3(0.05, 0.05, 0);
    smoke.blendMode = ParticleSystem.BLENDMODE_STANDARD;
    smoke.emitRate = 0;
    smoke.start();
    return { sparks, smoke, at };
  }

  /** `lit`: 0 (apagadas, de dia) a 1 (acesas). */
  update(lit: number, t: number, player: Vector3 | undefined): void {
    const on = smooth((lit - 0.15) / 0.5);

    // Chama e poça: tudo no shader/material, sem reescrever as instâncias.
    const visible = on > 0.001;
    this.flame.setEnabled(visible);
    this.pool.setEnabled(visible);
    if (visible) {
      this.flameMat.setFloat("time", t);
      this.flameMat.setFloat("intensity", on);
      this.pool.visibility = on * (0.8 + 0.08 * flicker(t, 1.3));
    }
    // Brasa: fraca de dia (lenha ainda quente), forte e cintilando à noite.
    if (this.logsMat.emissiveTexture) this.logsMat.emissiveTexture.level = 0.25 + on * (1.1 + 0.25 * flicker(t * 0.7, 4.2));

    // As luzes e as partículas vão para as tochas mais próximas do jogador.
    if (player && performance.now() > this.nextPick) {
      this.nextPick = performance.now() + 500;
      const near = this.spots
        .map((s, i) => ({ i, d: Math.hypot(s.x - player.x, s.z - player.z) }))
        .sort((p, q) => p.d - q.d)
        .slice(0, TORCH_LIGHTS);
      near.forEach((n, j) => {
        this.lights[j].metadata = n.i;
        this.near[j].at.copyFrom(this.tops[n.i]).addInPlaceFromFloats(0, 0.12, 0);
      });
    }
    for (let j = 0; j < this.lights.length; j++) {
      const l = this.lights[j];
      const i = typeof l.metadata === "number" ? l.metadata : 0;
      const f = flicker(t, this.seeds[i] ?? 0);
      l.intensity = on * (1.5 + 0.4 * f);
      // A luz "dança" um pouco (as sombras tremem junto com o fogo).
      const top = this.tops[i];
      if (top) l.position.set(top.x + 0.05 * Math.sin(t * 7.3 + i), top.y + 0.35 + 0.04 * f, top.z + 0.05 * Math.cos(t * 6.1 + i));
      const fx = this.near[j];
      fx.sparks.emitRate = on * (5 + 4 * Math.max(0, f));
      fx.smoke.emitRate = on > 0.05 ? 3 + on * 3 : 0;
    }
  }
}
