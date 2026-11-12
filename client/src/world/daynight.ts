import {
  Color3,
  Color4,
  Constants,
  DynamicTexture,
  GlowLayer,
  Material,
  Matrix,
  Mesh,
  MeshBuilder,
  PointLight,
  Quaternion,
  StandardMaterial,
  Vector3,
  type Scene,
} from "@babylonjs/core";
import { mulberry32 } from "./map-painter";
import type { WorldScene } from "./scene";
import type { WorldData } from "./world-data";

// ---------------------------------------------------------------------------------------
// Relógio do mundo
// ---------------------------------------------------------------------------------------

/** Início da noite na fase do ciclo (igual ao servidor: game.NightStart). */
export const NIGHT_START = 0.75;

export function isNight(phase: number): boolean {
  return phase >= NIGHT_START;
}

/**
 * Hora do jogo. O dia (fase 0–0,75) vai das 06:00 às 18:00 e a noite (0,75–1) das 18:00
 * às 06:00, então o relógio anda mais rápido à noite, que dura menos tempo real.
 */
export function gameHour(phase: number): number {
  const h = phase < NIGHT_START ? 6 + (phase / NIGHT_START) * 12 : 18 + ((phase - NIGHT_START) / (1 - NIGHT_START)) * 12;
  return h % 24;
}

export function hourLabel(phase: number): string {
  const h = gameHour(phase);
  const hh = Math.floor(h);
  const mm = Math.floor((h - hh) * 6) * 10; // de 10 em 10 minutos
  return `${String(hh).padStart(2, "0")}:${String(mm).padStart(2, "0")}`;
}

/** Fase do ciclo calculada a partir do relógio do servidor (recebido no Welcome). */
export class WorldClock {
  private offset: number;
  private fixed: number | null;

  constructor(
    serverTimeMs: number,
    private cycleMs: number,
  ) {
    this.offset = serverTimeMs - Date.now();
    // Desenvolvimento: ?time=0.85 fixa a hora.
    const t = Number(new URLSearchParams(location.search).get("time"));
    this.fixed = import.meta.env.DEV && Number.isFinite(t) && location.search.includes("time=") ? t % 1 : null;
  }

  /** Fixa a fase (0–1) ou volta ao relógio do servidor (null). Só para testes. */
  setFixed(phase: number | null): void {
    this.fixed = phase === null ? null : ((phase % 1) + 1) % 1;
  }

  phase(): number {
    if (this.fixed !== null) return this.fixed;
    if (this.cycleMs <= 0) return 0.62; // servidor antigo: fim de tarde fixo
    const ms = (Date.now() + this.offset) % this.cycleMs;
    return ms / this.cycleMs;
  }
}

// ---------------------------------------------------------------------------------------
// Aparência por hora (quadros-chave)
// ---------------------------------------------------------------------------------------

interface Key {
  p: number;
  zenith: string;
  mid: string;
  horizon: string;
  fog: string;
  fogDensity: number;
  hemi: number;
  hemiSky: string;
  hemiGround: string;
  light: number; // intensidade máxima do sol/lua (antes do fator de altura)
  lightColor: string;
  ambient: string;
  shadow: number; // ShadowGenerator.darkness (0 = sombra preta, 1 = sem sombra)
  night: number; // 0–1: estrelas, lua, tochas
}

const KEYS: Key[] = [
  { p: 0.0, zenith: "#34406e", mid: "#9a7896", horizon: "#f2a676", fog: "#b98c82", fogDensity: 0.0095, hemi: 0.46, hemiSky: "#b8a8cc", hemiGround: "#3a3030", light: 1.0, lightColor: "#ffb27a", ambient: "#40384a", shadow: 0.5, night: 0.25 },
  { p: 0.07, zenith: "#4672b6", mid: "#8cb0da", horizon: "#ead6b8", fog: "#b8c0c4", fogDensity: 0.0062, hemi: 0.6, hemiSky: "#d4deff", hemiGround: "#4a4030", light: 1.45, lightColor: "#fff0d4", ambient: "#4a4a50", shadow: 0.38, night: 0 },
  { p: 0.375, zenith: "#3a78c8", mid: "#80b4e8", horizon: "#d2e2ee", fog: "#b6c8d4", fogDensity: 0.0052, hemi: 0.7, hemiSky: "#dde7ff", hemiGround: "#4a4434", light: 1.75, lightColor: "#fff7e8", ambient: "#50505a", shadow: 0.33, night: 0 },
  // O visual original do jogo (fim de tarde).
  { p: 0.62, zenith: "#2c3f6e", mid: "#6b6f9a", horizon: "#e8b48c", fog: "#c7998a", fogDensity: 0.0085, hemi: 0.62, hemiSky: "#d9d9ff", hemiGround: "#473d33", light: 1.6, lightColor: "#ffdbad", ambient: "#4d4552", shadow: 0.35, night: 0 },
  { p: 0.715, zenith: "#2a3060", mid: "#8a5a7c", horizon: "#ff8c52", fog: "#b0705c", fogDensity: 0.009, hemi: 0.5, hemiSky: "#c4a2c4", hemiGround: "#3a2a26", light: 1.15, lightColor: "#ff9a5a", ambient: "#40303a", shadow: 0.45, night: 0.1 },
  { p: 0.78, zenith: "#141a3a", mid: "#2a2d58", horizon: "#5c4264", fog: "#353450", fogDensity: 0.0105, hemi: 0.34, hemiSky: "#8494c8", hemiGround: "#202028", light: 0.45, lightColor: "#a6baff", ambient: "#262a3e", shadow: 0.62, night: 0.85 },
  { p: 0.875, zenith: "#070b1c", mid: "#101834", horizon: "#1f2a48", fog: "#182036", fogDensity: 0.011, hemi: 0.3, hemiSky: "#7488c4", hemiGround: "#16182a", light: 0.5, lightColor: "#aec0ff", ambient: "#20243a", shadow: 0.62, night: 1 },
  { p: 0.965, zenith: "#0e1434", mid: "#222a54", horizon: "#4e4668", fog: "#2c2c48", fogDensity: 0.0105, hemi: 0.33, hemiSky: "#7e8cc4", hemiGround: "#1c1c28", light: 0.42, lightColor: "#a8bcff", ambient: "#24283c", shadow: 0.62, night: 0.8 },
];

const hex = (h: string) => Color3.FromHexString(h);

/** Valores interpolados para a fase (o ciclo dá a volta: depois de 0,965 vem o 0). */
function sample(phase: number): Key {
  let a = KEYS[KEYS.length - 1];
  let b = KEYS[0];
  for (let i = 0; i < KEYS.length; i++) {
    if (KEYS[i].p <= phase) {
      a = KEYS[i];
      b = KEYS[(i + 1) % KEYS.length];
    }
  }
  const span = (b.p - a.p + 1) % 1 || 1;
  const t = smooth((((phase - a.p) % 1) + 1) % 1 / span);
  const col = (x: string, y: string) => Color3.Lerp(hex(x), hex(y), t).toHexString();
  const num = (x: number, y: number) => x + (y - x) * t;
  return {
    p: phase,
    zenith: col(a.zenith, b.zenith),
    mid: col(a.mid, b.mid),
    horizon: col(a.horizon, b.horizon),
    fog: col(a.fog, b.fog),
    fogDensity: num(a.fogDensity, b.fogDensity),
    hemi: num(a.hemi, b.hemi),
    hemiSky: col(a.hemiSky, b.hemiSky),
    hemiGround: col(a.hemiGround, b.hemiGround),
    light: num(a.light, b.light),
    lightColor: col(a.lightColor, b.lightColor),
    ambient: col(a.ambient, b.ambient),
    shadow: num(a.shadow, b.shadow),
    night: num(a.night, b.night),
  };
}

function smooth(t: number): number {
  const c = Math.max(0, Math.min(1, t));
  return c * c * (3 - 2 * c);
}

/**
 * Direção (unitária) do chão para o astro. O sol nasce no leste (+X) na fase 0, passa alto
 * pelo sul e se põe no oeste na fase 0,75; a lua faz o mesmo arco durante a noite.
 */
function bodyDir(phase: number, out: Vector3): { elevation: number; moon: boolean } {
  const moon = isNight(phase);
  const t = moon ? (phase - NIGHT_START) / (1 - NIGHT_START) : phase / NIGHT_START;
  const th = Math.PI * t;
  out.set(Math.cos(th), Math.sin(th) * 0.92, -0.42).normalize();
  return { elevation: out.y, moon };
}

// ---------------------------------------------------------------------------------------
// Texturas pintadas (sem arquivos de arte)
// ---------------------------------------------------------------------------------------

function glowTexture(scene: Scene, name: string, stops: [number, string][]): DynamicTexture {
  const tex = new DynamicTexture(name, { width: 256, height: 256 }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  const g = ctx.createRadialGradient(128, 128, 0, 128, 128, 128);
  for (const [o, c] of stops) g.addColorStop(o, c);
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, 256, 256);
  tex.update(true);
  tex.hasAlpha = true;
  return tex;
}

function moonTexture(scene: Scene): DynamicTexture {
  const tex = new DynamicTexture("moonTex", { width: 256, height: 256 }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  ctx.clearRect(0, 0, 256, 256);
  // Halo
  const halo = ctx.createRadialGradient(128, 128, 40, 128, 128, 128);
  halo.addColorStop(0, "rgba(170,190,255,0.45)");
  halo.addColorStop(1, "rgba(170,190,255,0)");
  ctx.fillStyle = halo;
  ctx.fillRect(0, 0, 256, 256);
  // Disco
  const disc = ctx.createRadialGradient(112, 112, 6, 128, 128, 46);
  disc.addColorStop(0, "#fbfaf2");
  disc.addColorStop(1, "#cfd4e6");
  ctx.fillStyle = disc;
  ctx.beginPath();
  ctx.arc(128, 128, 44, 0, Math.PI * 2);
  ctx.fill();
  // Crateras
  const rand = mulberry32(31);
  for (let i = 0; i < 14; i++) {
    const a = rand() * Math.PI * 2;
    const r = Math.sqrt(rand()) * 36;
    ctx.fillStyle = `rgba(140,150,175,${0.18 + rand() * 0.2})`;
    ctx.beginPath();
    ctx.arc(128 + Math.cos(a) * r, 128 + Math.sin(a) * r, 2 + rand() * 7, 0, Math.PI * 2);
    ctx.fill();
  }
  tex.update(true);
  tex.hasAlpha = true;
  return tex;
}

function starTexture(scene: Scene): DynamicTexture {
  const W = 2048;
  const H = 1024;
  const tex = new DynamicTexture("starTex", { width: W, height: H }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  ctx.fillStyle = "#000";
  ctx.fillRect(0, 0, W, H);
  const rand = mulberry32(911);
  // Só a metade de cima da esfera fica acima do horizonte: no canvas, y < H/2.
  for (let i = 0; i < 3400; i++) {
    const x = rand() * W;
    const y = Math.pow(rand(), 1.4) * H * 0.5; // mais estrelas no alto
    const big = rand() < 0.04;
    const b = 0.35 + rand() * 0.65;
    const tint = rand();
    const c = tint < 0.15 ? `rgba(255,220,190,${b})` : tint < 0.3 ? `rgba(190,210,255,${b})` : `rgba(255,255,255,${b})`;
    ctx.fillStyle = c;
    ctx.beginPath();
    ctx.arc(x, y, big ? 1.1 : 0.35 + rand() * 0.4, 0, Math.PI * 2);
    ctx.fill();
  }
  // Faixa leitosa suave
  ctx.globalAlpha = 0.07;
  for (let i = 0; i < 260; i++) {
    const x = rand() * W;
    const y = H * 0.18 + Math.sin((x / W) * Math.PI * 2) * H * 0.1 + (rand() - 0.5) * 60;
    const g = ctx.createRadialGradient(x, y, 0, x, y, 40);
    g.addColorStop(0, "#c8d4ff");
    g.addColorStop(1, "rgba(200,212,255,0)");
    ctx.fillStyle = g;
    ctx.fillRect(x - 40, y - 40, 80, 80);
  }
  ctx.globalAlpha = 1;
  tex.update(false); // y=0 do canvas no topo da esfera (ver paintSky)
  return tex;
}

function additive(scene: Scene, name: string, tex: DynamicTexture): StandardMaterial {
  const m = new StandardMaterial(name, scene);
  m.emissiveTexture = tex;
  m.opacityTexture = tex;
  m.diffuseColor = Color3.Black();
  m.specularColor = Color3.Black();
  m.disableLighting = true;
  m.fogEnabled = false;
  m.alphaMode = Constants.ALPHA_ADD;
  // Sempre no passe transparente (soma à cor do céu), mesmo com visibilidade 1.
  m.transparencyMode = Material.MATERIAL_ALPHABLEND;
  m.disableDepthWrite = true;
  m.backFaceCulling = false;
  return m;
}

// ---------------------------------------------------------------------------------------
// Tochas
// ---------------------------------------------------------------------------------------

const TORCH_LIGHTS = 3;
const TORCH_HEIGHT = 2.75;
const TORCH_RANGE = 14;

class Torches {
  private flame: Mesh;
  private core: Mesh;
  private flameBuf: Float32Array;
  private coreBuf: Float32Array;
  private lights: PointLight[] = [];
  private seeds: number[];
  private nextPick = 0;
  private lit = -1;

  constructor(
    scene: Scene,
    private spots: { x: number; z: number }[],
    world: WorldScene,
    glow: GlowLayer,
  ) {
    const wood = new StandardMaterial("torchWood", scene);
    wood.diffuseColor = new Color3(0.28, 0.18, 0.11);
    wood.specularColor = Color3.Black();
    const iron = new StandardMaterial("torchIron", scene);
    iron.diffuseColor = new Color3(0.16, 0.15, 0.15);
    iron.specularColor = new Color3(0.15, 0.15, 0.15);

    const post = MeshBuilder.CreateCylinder("torchPost", { height: 2.5, diameterTop: 0.12, diameterBottom: 0.2, tessellation: 8 }, scene);
    post.bakeTransformIntoVertices(Matrix.Translation(0, 1.25, 0));
    post.material = wood;
    const bowl = MeshBuilder.CreateCylinder("torchBowl", { height: 0.26, diameterTop: 0.46, diameterBottom: 0.2, tessellation: 10 }, scene);
    bowl.bakeTransformIntoVertices(Matrix.Translation(0, 2.55, 0));
    bowl.material = iron;

    const flameMat = new StandardMaterial("torchFlame", scene);
    flameMat.emissiveColor = new Color3(1, 0.45, 0.12);
    flameMat.diffuseColor = Color3.Black();
    flameMat.specularColor = Color3.Black();
    flameMat.disableLighting = true;
    flameMat.fogEnabled = false;
    this.flame = MeshBuilder.CreateSphere("torchFlame", { diameter: 1, segments: 8 }, scene);
    this.flame.material = flameMat;
    const coreMat = flameMat.clone("torchCore");
    coreMat.emissiveColor = new Color3(1, 0.88, 0.5);
    this.core = MeshBuilder.CreateSphere("torchCore", { diameter: 1, segments: 6 }, scene);
    this.core.material = coreMat;

    const base = new Float32Array(spots.length * 16);
    spots.forEach((s, i) => Matrix.Translation(s.x, 0, s.z).copyToArray(base, i * 16));
    for (const m of [post, bowl]) {
      m.thinInstanceSetBuffer("matrix", base.slice(), 16, true);
      m.isPickable = false;
      m.receiveShadows = true;
      m.freezeWorldMatrix();
      world.shadows.addShadowCaster(m, false);
    }
    this.flameBuf = new Float32Array(spots.length * 16);
    this.coreBuf = new Float32Array(spots.length * 16);
    for (const m of [this.flame, this.core]) {
      m.isPickable = false;
      m.thinInstanceSetBuffer("matrix", m === this.flame ? this.flameBuf : this.coreBuf, 16, false);
      glow.addIncludedOnlyMesh(m);
    }
    this.seeds = spots.map((_, i) => i * 1.731 + 0.37);

    for (let i = 0; i < TORCH_LIGHTS; i++) {
      const l = new PointLight(`torchLight${i}`, new Vector3(0, -50, 0), scene);
      l.diffuse = new Color3(1, 0.62, 0.3);
      l.specular = new Color3(0.4, 0.25, 0.1);
      l.range = TORCH_RANGE;
      l.intensity = 0;
      this.lights.push(l);
    }
  }

  /** `lit`: 0 (apagadas, de dia) a 1 (acesas). */
  update(lit: number, t: number, player: Vector3 | undefined): void {
    const on = smooth((lit - 0.15) / 0.5);
    const tmp = new Matrix();
    const q = Quaternion.Identity();
    const sc = new Vector3();
    const pos = new Vector3();
    if (on > 0.001 || this.lit !== 0) {
      this.spots.forEach((s, i) => {
        const f = flicker(t, this.seeds[i]);
        const k = on * (0.85 + 0.25 * f);
        sc.set(0.3 * k, 0.55 * k, 0.3 * k);
        pos.set(s.x, TORCH_HEIGHT + 0.1 * k, s.z);
        Matrix.ComposeToRef(sc, q, pos, tmp);
        tmp.copyToArray(this.flameBuf, i * 16);
        sc.set(0.15 * k, 0.3 * k, 0.15 * k);
        pos.set(s.x, TORCH_HEIGHT + 0.02, s.z);
        Matrix.ComposeToRef(sc, q, pos, tmp);
        tmp.copyToArray(this.coreBuf, i * 16);
      });
      this.flame.thinInstanceBufferUpdated("matrix");
      this.core.thinInstanceBufferUpdated("matrix");
      this.lit = on > 0.001 ? on : 0;
    }

    // As 3 luzes de verdade vão para as tochas mais próximas do jogador.
    if (player && performance.now() > this.nextPick) {
      this.nextPick = performance.now() + 500;
      const near = this.spots
        .map((s, i) => ({ i, d: Math.hypot(s.x - player.x, s.z - player.z) }))
        .sort((a, b) => a.d - b.d)
        .slice(0, TORCH_LIGHTS);
      near.forEach((n, j) => {
        const s = this.spots[n.i];
        this.lights[j].position.set(s.x, TORCH_HEIGHT + 0.3, s.z);
        this.lights[j].metadata = n.i;
      });
    }
    for (const l of this.lights) {
      const i = typeof l.metadata === "number" ? l.metadata : 0;
      l.intensity = on * (1.5 + 0.35 * flicker(t, this.seeds[i] ?? 0));
    }
  }
}

/** Cintilação suave (-1..1) de cada chama. */
function flicker(t: number, seed: number): number {
  return Math.sin(t * 9.1 + seed) * 0.5 + Math.sin(t * 15.7 + seed * 2.3) * 0.3 + Math.sin(t * 4.3 + seed * 0.7) * 0.2;
}

// ---------------------------------------------------------------------------------------
// Ciclo dia/noite
// ---------------------------------------------------------------------------------------

/** Liga céu, luzes, neblina, sol, lua, estrelas e tochas à hora do mundo. */
export class DayNight {
  /** Camada de brilho compartilhada (chamas e efeitos visuais). */
  readonly glow: GlowLayer;
  private torches: Torches | null = null;
  private sunDisc: Mesh;
  private moon: Mesh;
  private stars: Mesh;
  private starMat: StandardMaterial;
  private sunMat: StandardMaterial;
  private moonMat: StandardMaterial;
  private lastSky = -1;
  private lastLightSweep = 0;
  private dir = new Vector3();
  private _night = 0;

  constructor(
    private world: WorldScene,
    data: WorldData,
  ) {
    const scene = world.scene;
    this.glow = new GlowLayer("glow", scene, { mainTextureRatio: 0.5, blurKernelSize: 48 });
    this.glow.intensity = 0.9;

    this.stars = MeshBuilder.CreateSphere("stars", { diameter: 1500, segments: 16, sideOrientation: Mesh.BACKSIDE }, scene);
    this.starMat = additive(scene, "starMat", starTexture(scene));
    this.starMat.opacityTexture = null;
    this.stars.material = this.starMat;
    this.stars.infiniteDistance = true;
    this.stars.isPickable = false;

    this.sunMat = additive(
      scene,
      "sunMat",
      glowTexture(scene, "sunTex", [
        [0, "rgba(255,255,240,1)"],
        [0.12, "rgba(255,245,210,1)"],
        [0.2, "rgba(255,210,150,0.55)"],
        [0.55, "rgba(255,170,100,0.12)"],
        [1, "rgba(255,150,80,0)"],
      ]),
    );
    this.sunDisc = MeshBuilder.CreatePlane("sunDisc", { size: 180 }, scene);
    this.sunDisc.material = this.sunMat;
    this.sunDisc.billboardMode = Mesh.BILLBOARDMODE_ALL;
    this.sunDisc.infiniteDistance = true;
    this.sunDisc.isPickable = false;

    this.moonMat = additive(scene, "moonMat", moonTexture(scene));
    this.moon = MeshBuilder.CreatePlane("moon", { size: 110 }, scene);
    this.moon.material = this.moonMat;
    this.moon.billboardMode = Mesh.BILLBOARDMODE_ALL;
    this.moon.infiniteDistance = true;
    this.moon.isPickable = false;

    // Céu primeiro, depois estrelas, depois sol/lua (todos atrás do resto da cena).
    world.sky.mesh.alphaIndex = 0;
    this.stars.alphaIndex = 1;
    this.sunDisc.alphaIndex = 2;
    this.moon.alphaIndex = 2;

    if (data.torches?.length) this.torches = new Torches(scene, data.torches, world, this.glow);
  }

  /** 0 (dia claro) a 1 (noite fechada). */
  get night(): number {
    return this._night;
  }

  update(phase: number, player: Vector3 | undefined): void {
    const k = sample(phase);
    const { scene, hemi, sun, shadows } = this.world;
    this._night = k.night;

    // Céu (redesenhado só quando a fase muda o bastante).
    if (Math.abs(phase - this.lastSky) > 0.0015) {
      this.lastSky = phase;
      this.paintSky(k);
    }
    const fog = hex(k.fog);
    scene.fogColor = fog;
    scene.fogDensity = k.fogDensity;
    scene.clearColor = new Color4(fog.r, fog.g, fog.b, 1);
    scene.ambientColor = hex(k.ambient);
    hemi.intensity = k.hemi;
    hemi.diffuse = hex(k.hemiSky);
    hemi.groundColor = hex(k.hemiGround);
    this.world.outerMat.diffuseColor = Color3.Lerp(new Color3(0.2, 0.26, 0.14), new Color3(0.07, 0.09, 0.08), k.night);

    // Sol de dia, lua à noite. Perto do horizonte a luz some, então a troca não aparece.
    const { elevation, moon } = bodyDir(phase, this.dir);
    const up = smooth(elevation / 0.22);
    const lightDir = new Vector3(-this.dir.x, -Math.max(this.dir.y, 0.3), -this.dir.z).normalize();
    sun.direction.copyFrom(lightDir);
    sun.intensity = k.light * up;
    sun.diffuse = hex(k.lightColor);
    shadows.darkness = 1 - (1 - k.shadow) * up;

    const far = 650;
    if (moon) {
      this.moon.position.copyFrom(this.dir).scaleInPlace(far);
      this.sunDisc.setEnabled(false);
    } else {
      this.sunDisc.position.copyFrom(this.dir).scaleInPlace(far);
      this.sunDisc.setEnabled(elevation > -0.05);
      // Sol baixo fica laranja; alto, branco.
      this.sunMat.emissiveColor = Color3.Lerp(new Color3(1, 0.55, 0.3), new Color3(1, 1, 1), smooth(elevation / 0.45));
    }
    // Antes da noite a lua espera no ponto onde nasce (leste); de manhã, já se pôs.
    const moonVis = smooth((k.night - 0.2) / 0.5) * (moon || phase > 0.5 ? 1 : 0);
    this.moon.setEnabled(moonVis > 0.01);
    this.moon.visibility = moonVis;
    if (!moon) this.moon.position.copyFromFloats(0.92, 0.06, -0.39).normalize().scaleInPlace(far);
    const starVis = smooth((k.night - 0.25) / 0.6);
    this.stars.setEnabled(starVis > 0.01);
    this.stars.visibility = starVis;
    this.stars.rotation.y = phase * Math.PI * 0.6;

    this.torches?.update(k.night, performance.now() / 1000, player);
    this.glow.intensity = 0.55 + 0.45 * k.night;

    // Sol + céu + 3 tochas = 5 luzes; o padrão do Babylon é 4 por material.
    if (performance.now() > this.lastLightSweep) {
      this.lastLightSweep = performance.now() + 2000;
      for (const m of scene.materials) {
        const lit = m as unknown as { maxSimultaneousLights?: number };
        if (typeof lit.maxSimultaneousLights === "number" && lit.maxSimultaneousLights < 6) lit.maxSimultaneousLights = 6;
      }
    }
  }

  private paintSky(k: Key): void {
    const tex = this.world.sky.texture;
    const ctx = tex.getContext() as CanvasRenderingContext2D;
    // O canvas desenha de cima (zênite) para baixo (abaixo do horizonte).
    const g = ctx.createLinearGradient(0, 0, 0, 512);
    const hor = hex(k.horizon);
    const mid = hex(k.mid);
    const fog = hex(k.fog);
    g.addColorStop(0, k.zenith);
    g.addColorStop(0.3, k.mid);
    g.addColorStop(0.46, Color3.Lerp(mid, hor, 0.65).toHexString());
    g.addColorStop(0.5, k.horizon);
    g.addColorStop(0.56, Color3.Lerp(hor, fog, 0.6).toHexString());
    g.addColorStop(1, fog.scale(0.7).toHexString());
    ctx.fillStyle = g;
    ctx.fillRect(0, 0, 4, 512);
    // Sem inverter: na esfera do Babylon o v=0 fica no topo, igual ao y=0 do canvas.
    tex.update(false);
  }
}
