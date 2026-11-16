/**
 * Lago (tunnel#99): uma malha de água só, com um shader barato.
 *
 * Onde fica: no leste, depois da borda do mapa (x > 150, perto de z = 52), fora do retângulo
 * jogável. O servidor não sabe da água e não precisa saber: ninguém chega nela (terrain.ts,
 * MAIN_LAKE). Quem anda até a borda leste vê o lago logo à frente.
 *
 * O shader (plugin do StandardMaterial, então a névoa, a gradação e o ACES do render valem):
 * - ondas por normal: soma de senos que só mexe na normal (a malha é plana), e o brilho do
 *   sol vem da luz especular normal do material;
 * - reflexo falso: a cor do céu (a mesma do céu pintado, da hora do dia) pela normal e
 *   por um fresnel, sem renderizar nada a mais;
 * - fundo: do verde-azulado raso ao azul fundo pela profundidade, que vem pronta no vértice;
 * - espuma na beira, animada.
 * A beira some pelo alfa do vértice (a água "entra" na margem sem corte duro).
 *
 * Presets: 0 = água simples (cor, fresnel e reflexo, sem ondas nem espuma animadas), 1 = ondas
 * por normal com 3 senos, 2 = 4 senos e espuma na beira.
 */

import {
  Color3,
  Material,
  Mesh,
  MaterialPluginBase,
  RegisterMaterialPlugin,
  ShaderLanguage,
  StandardMaterial,
  VertexData,
  type MaterialDefines,
  type Scene,
  type UniformBuffer,
} from "@babylonjs/core";
import type { HeightField, Lake } from "./terrain";

const NAME = "LakeWater";
const CELL = 2.5;

export type WaterDetail = 0 | 1 | 2;

/** Estado global: tempo e as cores do céu da hora (daynight.ts escreve todo quadro). */
const state = {
  time: 0,
  horizon: [0.7, 0.75, 0.85],
  zenith: [0.25, 0.4, 0.7],
  deep: [0.05, 0.16, 0.24],
  shallow: [0.2, 0.42, 0.42],
};

export function setWaterTime(t: number): void {
  state.time = t;
}

/** Cores do céu (horizonte e zênite) e da água da hora; valores 0–1 antes do ACES. */
export function setWaterSky(horizon: Color3, zenith: Color3, night: number): void {
  state.horizon[0] = horizon.r;
  state.horizon[1] = horizon.g;
  state.horizon[2] = horizon.b;
  state.zenith[0] = zenith.r;
  state.zenith[1] = zenith.g;
  state.zenith[2] = zenith.b;
  // A água escurece à noite (a cor do céu refletido já escurece sozinha).
  const day = 1 - 0.55 * night;
  state.deep[0] = 0.05 * day;
  state.deep[1] = 0.16 * day;
  state.deep[2] = 0.25 * day;
  state.shallow[0] = 0.2 * day;
  state.shallow[1] = 0.42 * day;
  state.shallow[2] = 0.4 * day;
}

const FRAGMENT_DEFS = `
#ifdef LAKEWATER
float lwWaves(vec2 q, float t, out vec2 grad) {
  // Gradiente de uma soma de ondas (só mexe na normal).
  vec2 k1 = vec2(0.9, 0.35);  float w1 = 1.3;
  vec2 k2 = vec2(-0.45, 1.1); float w2 = 1.7;
  vec2 k3 = vec2(1.7, -1.2);  float w3 = 2.3;
  float a1 = dot(k1, q) + t * w1;
  float a2 = dot(k2, q) + t * w2;
  float a3 = dot(k3, q) + t * w3;
  grad = k1 * cos(a1) * 0.05 + k2 * cos(a2) * 0.04 + k3 * cos(a3) * 0.018;
  #ifdef LAKEWATER_HI
  vec2 k4 = vec2(-2.6, -2.1); float w4 = 3.1;
  float a4 = dot(k4, q) + t * w4;
  grad += k4 * cos(a4) * 0.012;
  return sin(a1) + sin(a2) * 0.7 + sin(a4) * 0.2;
  #else
  return sin(a1) + sin(a2) * 0.7;
  #endif
}
float lwHash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }
float lwNoise(vec2 p) {
  vec2 i = floor(p); vec2 f = fract(p); f = f * f * (3.0 - 2.0 * f);
  return mix(mix(lwHash(i), lwHash(i + vec2(1.0, 0.0)), f.x), mix(lwHash(i + vec2(0.0, 1.0)), lwHash(i + vec2(1.0, 1.0)), f.x), f.y);
}
#endif
`;

const FRAGMENT_DIFFUSE = `
#ifdef LAKEWATER
float lwDepth = clamp(vColor.g, 0.0, 1.0);
baseColor.rgb = mix(lwShallow.rgb, lwDeep.rgb, smoothstep(0.0, 0.8, lwDepth));
#endif
`;

const FRAGMENT_LIGHTS = `
#ifdef LAKEWATER
vec2 lwGrad = vec2(0.0);
float lwH = 0.0;
#ifndef LAKEWATER_LITE
lwH = lwWaves(vPositionW.xz, lwTime.x, lwGrad);
normalW = normalize(vec3(-lwGrad.x, 1.0, -lwGrad.y));
#endif
#endif
`;

const FRAGMENT_FOG = `
#ifdef LAKEWATER
{
  vec3 lwV = viewDirectionW;
  float lwF = pow(1.0 - clamp(dot(normalW, lwV), 0.0, 1.0), 3.0);
  vec3 lwR = reflect(-lwV, normalW);
  // O céu refletido: do horizonte ao zênite, pela altura do raio refletido.
  vec3 lwSky = mix(lwHorizon.rgb, lwZenith.rgb, smoothstep(0.0, 0.8, lwR.y));
  color.rgb = mix(color.rgb, lwSky, 0.18 + 0.72 * lwF);
  #ifdef LAKEWATER_HI
  float lwFoam = smoothstep(0.32, 0.0, lwDepth) * (0.55 + 0.45 * lwNoise(vPositionW.xz * 1.7 + lwTime.x * vec2(0.3, -0.2)));
  lwFoam *= smoothstep(0.35, 0.65, lwNoise(vPositionW.xz * 3.1 - lwTime.x * 0.25));
  color.rgb += vec3(0.5, 0.55, 0.55) * lwFoam * 0.5;
  color.a = max(color.a, lwFoam * 0.5);
  #endif
  color.a *= mix(0.55, 0.93, smoothstep(0.0, 0.7, lwDepth) * 0.6 + lwF * 0.4);
}
#endif
`;

class LakeWaterPlugin extends MaterialPluginBase {
  private _detail: WaterDetail = 1;
  private _on = false;

  constructor(material: Material) {
    super(material, NAME, 280, { LAKEWATER: false, LAKEWATER_LITE: false, LAKEWATER_HI: false });
    this.doNotSerialize = true;
    this._enable(true);
  }

  get enabled(): boolean {
    return this._on;
  }
  set enabled(v: boolean) {
    if (v === this._on) return;
    this._on = v;
    this.markAllDefinesAsDirty();
  }

  get detail(): WaterDetail {
    return this._detail;
  }
  set detail(v: WaterDetail) {
    if (v === this._detail) return;
    this._detail = v;
    this.markAllDefinesAsDirty();
  }

  override getClassName(): string {
    return "LakeWaterPlugin";
  }

  override isCompatible(shaderLanguage: ShaderLanguage): boolean {
    return shaderLanguage === ShaderLanguage.GLSL;
  }

  override prepareDefines(defines: MaterialDefines): void {
    defines["LAKEWATER"] = this._on;
    defines["LAKEWATER_LITE"] = this._on && this._detail === 0;
    defines["LAKEWATER_HI"] = this._on && this._detail === 2;
  }

  override getUniforms(): { ubo: { name: string; size: number; type: string }[]; fragment: string } {
    return {
      ubo: [
        { name: "lwTime", size: 4, type: "vec4" },
        { name: "lwHorizon", size: 4, type: "vec4" },
        { name: "lwZenith", size: 4, type: "vec4" },
        { name: "lwDeep", size: 4, type: "vec4" },
        { name: "lwShallow", size: 4, type: "vec4" },
      ],
      fragment: `#ifdef LAKEWATER
uniform vec4 lwTime;
uniform vec4 lwHorizon;
uniform vec4 lwZenith;
uniform vec4 lwDeep;
uniform vec4 lwShallow;
#endif`,
    };
  }

  override bindForSubMesh(ubo: UniformBuffer): void {
    if (!this._on) return;
    // Em detalhe 0 o tempo para (água parada).
    ubo.updateFloat4("lwTime", this._detail === 0 ? 0 : state.time, 0, 0, 0);
    ubo.updateFloat4("lwHorizon", state.horizon[0], state.horizon[1], state.horizon[2], 1);
    ubo.updateFloat4("lwZenith", state.zenith[0], state.zenith[1], state.zenith[2], 1);
    ubo.updateFloat4("lwDeep", state.deep[0], state.deep[1], state.deep[2], 1);
    ubo.updateFloat4("lwShallow", state.shallow[0], state.shallow[1], state.shallow[2], 1);
  }

  override getCustomCode(shaderType: string, shaderLanguage?: ShaderLanguage): { [pointName: string]: string } | null {
    if (shaderType !== "fragment" || (shaderLanguage ?? ShaderLanguage.GLSL) !== ShaderLanguage.GLSL) return null;
    return {
      CUSTOM_FRAGMENT_DEFINITIONS: FRAGMENT_DEFS,
      CUSTOM_FRAGMENT_UPDATE_DIFFUSE: FRAGMENT_DIFFUSE,
      CUSTOM_FRAGMENT_BEFORE_LIGHTS: FRAGMENT_LIGHTS,
      CUSTOM_FRAGMENT_BEFORE_FOG: FRAGMENT_FOG,
    };
  }
}

let registered = false;
/** Registra o plugin da água (uma vez; antes de criar a cena). */
export function registerWaterPlugin(): void {
  if (registered) return;
  registered = true;
  RegisterMaterialPlugin(NAME, (m) => (m instanceof StandardMaterial ? new LakeWaterPlugin(m) : null));
}

export interface Water {
  mesh: Mesh;
  setDetail(d: WaterDetail): void;
}

/**
 * Cria a malha do lago: uma grade sobre a elipse, na altura da água, com a cor do vértice
 * levando a profundidade (g) e o alfa da margem (a). Sem lago (outros mapas), devolve null.
 */
export function createWater(scene: Scene, field: HeightField): Water | null {
  const lake = field.lake;
  if (!lake) return null;
  const mesh = buildMesh(scene, field, lake);
  if (!mesh) return null;
  const mat = new StandardMaterial("waterMat", scene);
  mat.diffuseColor = Color3.White();
  mat.specularColor = new Color3(0.9, 0.9, 0.85);
  mat.specularPower = 180;
  mat.alpha = 0.999; // entra na fila transparente (o alfa final sai do shader e do vértice)
  mat.transparencyMode = Material.MATERIAL_ALPHABLEND;
  mat.backFaceCulling = false;
  mat.disableDepthWrite = true;
  mat.zOffset = -1;
  mesh.material = mat;
  mesh.isPickable = false;
  mesh.hasVertexAlpha = true;
  mesh.freezeWorldMatrix();
  mesh.doNotSyncBoundingInfo = true;
  const plugin = mat.pluginManager?.getPlugin(NAME) as LakeWaterPlugin | undefined;
  if (plugin) plugin.enabled = true;
  return {
    mesh,
    setDetail(d) {
      if (plugin) plugin.detail = d;
    },
  };
}

function buildMesh(scene: Scene, field: HeightField, lake: Lake): Mesh | null {
  // Caixa da água: u < 1,5 (a margem é em torno de 1,2).
  const x0 = lake.cx - lake.rx * 1.5;
  const x1 = lake.cx + lake.rx * 1.5;
  const z0 = lake.cz - lake.rz * 1.5;
  const z1 = lake.cz + lake.rz * 1.5;
  const nx = Math.ceil((x1 - x0) / CELL);
  const nz = Math.ceil((z1 - z0) / CELL);
  const pos: number[] = [];
  const col: number[] = [];
  const alive: boolean[] = [];
  for (let j = 0; j <= nz; j++) {
    for (let i = 0; i <= nx; i++) {
      const x = x0 + (i * (x1 - x0)) / nx;
      const z = z0 + (j * (z1 - z0)) / nz;
      const depth = lake.level - field.heightAt(x, z);
      pos.push(x, lake.level, z);
      // g: profundidade (0 a 1 = 1,5 m); a: some na margem (0 a 0,35 m de fundura).
      const a = Math.min(1, Math.max(0, depth / 0.35));
      col.push(1, Math.min(1, Math.max(0, depth / 1.5)), 0, a);
      alive.push(depth > -0.05);
    }
  }
  const idx: number[] = [];
  const w = nx + 1;
  for (let j = 0; j < nz; j++) {
    for (let i = 0; i < nx; i++) {
      const a = j * w + i;
      const b = a + 1;
      const c = a + w;
      const d = c + 1;
      if (!(alive[a] || alive[b] || alive[c] || alive[d])) continue;
      idx.push(a, c, b, b, c, d);
    }
  }
  if (!idx.length) return null;
  const normals = new Array<number>(pos.length);
  for (let i = 0; i < pos.length; i += 3) {
    normals[i] = 0;
    normals[i + 1] = 1;
    normals[i + 2] = 0;
  }
  const mesh = new Mesh("lake", scene);
  const data = new VertexData();
  data.positions = pos;
  data.normals = normals;
  data.colors = col;
  data.indices = idx;
  data.applyToMesh(mesh);
  return mesh;
}
