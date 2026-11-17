/**
 * Malha e material do terreno (tunnel#99): uma malha só (colinas de terrain.ts) com um
 * shader só, que mistura quatro texturas (grama, terra, pedra e cascalho) por um mapa de
 * mistura (splat) calculado por região em terrain.ts. É um plugin do StandardMaterial (como
 * o material de paleta), então continuam valendo sombras, névoa, luzes, a gradação de cor e
 * o ACES do render.
 *
 * Custo no pixel: 1 amostra do mapa de mistura + 4 das camadas (preset baixo: 2) + a pintura
 * do chão que já existia. Sem draw call novo: o terreno substitui o chão plano antigo.
 */

import {
  Color3,
  DynamicTexture,
  Mesh,
  MaterialPluginBase,
  RawTexture,
  RegisterMaterialPlugin,
  ShaderLanguage,
  StandardMaterial,
  Texture,
  VertexData,
  type BaseTexture,
  type Material,
  type MaterialDefines,
  type Scene,
  type UniformBuffer,
} from "@babylonjs/core";
import { REMASTER } from "./remaster";
import { bakeSplat, normalAt, type HeightField } from "./terrain";

/** Lado de uma célula da malha (m): as colinas têm 46 m ou mais de comprimento de onda. */
const CELL = 3;
/** Metros por texel do mapa de mistura. */
const SPLAT_CELL = 2;
/** Metros que cada camada cobre antes de repetir: grama, terra, pedra e cascalho. */
const TILE_M = [2.6, 3.4, 4.2, 2.2];

/** Remaster (remaster, o padrão): metros por repetição das texturas pintadas à mão (maiores, para a repetição não aparecer). */
const TILE_M_PAINTED = [5.2, 5.2, 6.5, 3.6];
/** Arquivos das camadas pintadas (public/terrain, feitos por scripts/make-seamless.mjs): grama, terra, pedra, cascalho. */
const PAINTED_FILES = ["grass", "dirt", "stone", "gravel"];

const NAME = "TerrainSplat";

// ---------------------------------------------------------------------------------------
// Texturas das camadas (pintadas em canvas, sem arquivos para baixar)
// ---------------------------------------------------------------------------------------

function rng(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/** Desenha f em (x, y) e nas cópias deslocadas, para a textura emendar sem costura. */
function wrap(size: number, x: number, y: number, margin: number, f: (x: number, y: number) => void): void {
  for (const dx of [-size, 0, size]) {
    for (const dy of [-size, 0, size]) {
      const px = x + dx;
      const py = y + dy;
      if (px > -margin && px < size + margin && py > -margin && py < size + margin) f(px, py);
    }
  }
}

interface LayerStyle {
  base: string;
  /** Pintas: cor, quantidade, raio mínimo e máximo (px), alfa. */
  specks: [string, number, number, number, number][];
  /** Traços curtos (folhinhas): cor, quantidade, comprimento. */
  blades?: [string, number, number][];
  /** Rachaduras/linhas finas escuras. */
  cracks?: number;
}

/** Paletas do guia de arte (3.3): verde vivo, terra rachada, rocha e cascalho claro. */
const LAYERS: LayerStyle[] = [
  {
    base: "#5e8040",
    specks: [["#74954a", 160, 6, 22, 0.25], ["#476a30", 160, 6, 22, 0.25], ["#8da552", 40, 3, 9, 0.2]],
    blades: [["#86a64e", 700, 10], ["#3f5f2b", 700, 9], ["#a3b45c", 160, 8]],
  },
  {
    base: "#8a6d48",
    specks: [["#a38358", 220, 4, 16, 0.25], ["#6a4f33", 220, 4, 16, 0.25], ["#b49a70", 90, 1.5, 4, 0.5]],
    cracks: 26,
  },
  {
    base: "#777a80",
    specks: [["#8e9096", 200, 5, 20, 0.28], ["#5a5d64", 200, 5, 20, 0.28], ["#a0a2a6", 40, 2, 7, 0.3]],
    cracks: 40,
  },
  {
    base: "#9d9582",
    specks: [["#b9b09a", 700, 1.5, 4.2, 0.9], ["#7d7565", 700, 1.5, 4.2, 0.9], ["#d0c9b6", 260, 1, 2.6, 0.8], ["#5f5a4e", 200, 1.2, 3.4, 0.8]],
  },
];

/** Camada pintada à mão (remaster): JPG repetível de 512 px, com mipmaps e anisotropia. */
function paintedTexture(scene: Scene, i: number): Texture {
  const tex = new Texture(`/terrain/${PAINTED_FILES[i]}.jpg`, scene, false, true, Texture.TRILINEAR_SAMPLINGMODE);
  tex.wrapU = tex.wrapV = Texture.WRAP_ADDRESSMODE;
  tex.anisotropicFilteringLevel = 4;
  return tex;
}

function layerTexture(scene: Scene, i: number): DynamicTexture {
  const S = 256;
  const st = LAYERS[i];
  const tex = new DynamicTexture(`terrainLayer${i}`, { width: S, height: S }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  const rand = rng(1000 + i * 77);
  ctx.fillStyle = st.base;
  ctx.fillRect(0, 0, S, S);
  for (const [color, n, r0, r1, a] of st.specks) {
    ctx.fillStyle = color;
    ctx.globalAlpha = a;
    for (let k = 0; k < n; k++) {
      const x = rand() * S;
      const y = rand() * S;
      const r = r0 + rand() * (r1 - r0);
      wrap(S, x, y, r, (px, py) => {
        ctx.beginPath();
        ctx.ellipse(px, py, r, r * (0.6 + rand() * 0.4), rand() * 3, 0, Math.PI * 2);
        ctx.fill();
      });
    }
  }
  ctx.globalAlpha = 1;
  ctx.lineCap = "round";
  for (const [color, n, len] of st.blades ?? []) {
    ctx.strokeStyle = color;
    ctx.globalAlpha = 0.55;
    for (let k = 0; k < n; k++) {
      const x = rand() * S;
      const y = rand() * S;
      const ang = -Math.PI / 2 + (rand() - 0.5) * 1.5;
      const l = len * (0.5 + rand() * 0.7);
      ctx.lineWidth = 1 + rand() * 1.2;
      wrap(S, x, y, l, (px, py) => {
        ctx.beginPath();
        ctx.moveTo(px, py);
        ctx.lineTo(px + Math.cos(ang) * l, py + Math.sin(ang) * l);
        ctx.stroke();
      });
    }
  }
  if (st.cracks) {
    ctx.strokeStyle = "rgba(40,32,26,0.4)";
    ctx.lineWidth = 1;
    for (let k = 0; k < st.cracks; k++) {
      let x = rand() * S;
      let y = rand() * S;
      let ang = rand() * Math.PI * 2;
      const pts: [number, number][] = [[x, y]];
      for (let s = 0; s < 6; s++) {
        ang += (rand() - 0.5) * 1.1;
        x += Math.cos(ang) * (6 + rand() * 10);
        y += Math.sin(ang) * (6 + rand() * 10);
        pts.push([x, y]);
      }
      wrap(S, 0, 0, S, (ox, oy) => {
        ctx.beginPath();
        pts.forEach(([px, py], j) => (j ? ctx.lineTo(px + ox, py + oy) : ctx.moveTo(px + ox, py + oy)));
        ctx.stroke();
      });
    }
  }
  ctx.globalAlpha = 1;
  tex.update(true);
  tex.wrapU = tex.wrapV = Texture.WRAP_ADDRESSMODE;
  tex.anisotropicFilteringLevel = 4;
  return tex;
}

// ---------------------------------------------------------------------------------------
// Plugin do material
// ---------------------------------------------------------------------------------------

/** 0 = só a grama e a pedra (preset baixo); 1 = as quatro camadas. */
export type TerrainDetail = 0 | 1;

const FRAGMENT_DEFS = `
#ifdef TERRAINSPLAT
uniform sampler2D tsSplat;
uniform sampler2D tsGrass;
uniform sampler2D tsDirt;
uniform sampler2D tsStone;
uniform sampler2D tsGravel;
#endif
`;

const FRAGMENT_DIFFUSE = `
#ifdef TERRAINSPLAT
{
  vec2 tsWp = vPositionW.xz;
  vec4 tsW = texture2D(tsSplat, (tsWp - tsExt.xy) * tsExt.zw);
  vec3 tsGr = texture2D(tsGrass, tsWp / tsTile.x).rgb;
  vec3 tsSt = texture2D(tsStone, tsWp / tsTile.z).rgb;
  #ifdef TERRAINSPLAT_FULL
  vec3 tsDi = texture2D(tsDirt, tsWp / tsTile.y).rgb;
  vec3 tsGv = texture2D(tsGravel, tsWp / tsTile.w).rgb;
  vec3 tsMix = tsGr * tsW.r + tsDi * tsW.g + tsSt * tsW.b + tsGv * tsW.a;
  #else
  vec3 tsMix = tsGr * (tsW.r + tsW.g * 0.6) + tsSt * (tsW.b + tsW.a + tsW.g * 0.4);
  #endif
  // A pintura do chão (cor da região, estradas) segura o tom; as camadas dão o grão.
  vec3 tsMacro = baseColor.rgb;
  float tsL = dot(tsMix, vec3(0.299, 0.587, 0.114));
  #ifdef TERRAINSPLAT_PAINTED
  // Texturas pintadas à mão: mandam mais no tom (a pintura da região só tinge) e o brilho delas fica.
  baseColor.rgb = mix(tsMacro, tsMix * (0.55 + 0.9 * tsMacro), 0.78) * 1.08;
  #else
  baseColor.rgb = mix(tsMacro, tsMix, 0.5) * mix(1.0, 0.8 + 0.5 * tsL, 0.6);
  #endif
}
#endif
`;

class TerrainSplatPlugin extends MaterialPluginBase {
  splat: BaseTexture | null = null;
  layers: BaseTexture[] = [];
  ext: [number, number, number, number] = [0, 0, 1, 1];
  private _full = true;
  private _on = false;
  /** Texturas pintadas à mão (remaster). */
  painted = false;

  constructor(material: Material) {
    super(material, NAME, 250, { TERRAINSPLAT: false, TERRAINSPLAT_FULL: false, TERRAINSPLAT_PAINTED: false });
    this._enable(true);
  }

  get enabled(): boolean {
    return this._on;
  }
  set enabled(v: boolean) {
    if (this._on === v) return;
    this._on = v;
    this.markAllDefinesAsDirty();
  }

  get detail(): TerrainDetail {
    return this._full ? 1 : 0;
  }
  set detail(v: TerrainDetail) {
    const full = v === 1;
    if (this._full === full) return;
    this._full = full;
    this.markAllDefinesAsDirty();
  }

  override getClassName(): string {
    return "TerrainSplatPlugin";
  }

  override isCompatible(shaderLanguage: ShaderLanguage): boolean {
    return shaderLanguage === ShaderLanguage.GLSL;
  }

  override isReadyForSubMesh(): boolean {
    return !this._on || (!!this.splat?.isReady() && this.layers.every((l) => l.isReady()));
  }

  override prepareDefines(defines: MaterialDefines): void {
    defines["TERRAINSPLAT"] = this._on;
    defines["TERRAINSPLAT_FULL"] = this._on && this._full;
    defines["TERRAINSPLAT_PAINTED"] = this._on && this.painted;
  }

  override getUniforms(): { ubo: { name: string; size: number; type: string }[]; fragment: string } {
    return {
      ubo: [
        { name: "tsExt", size: 4, type: "vec4" },
        { name: "tsTile", size: 4, type: "vec4" },
      ],
      fragment: `#ifdef TERRAINSPLAT
uniform vec4 tsExt;
uniform vec4 tsTile;
#endif`,
    };
  }

  override getSamplers(samplers: string[]): void {
    samplers.push("tsSplat", "tsGrass", "tsDirt", "tsStone", "tsGravel");
  }

  override hasTexture(texture: BaseTexture): boolean {
    return texture === this.splat || this.layers.includes(texture);
  }

  override getActiveTextures(active: BaseTexture[]): void {
    if (this.splat) active.push(this.splat);
    active.push(...this.layers);
  }

  override bindForSubMesh(ubo: UniformBuffer): void {
    if (!this._on || !this.splat) return;
    ubo.updateFloat4("tsExt", this.ext[0], this.ext[1], this.ext[2], this.ext[3]);
    const tile = this.painted ? TILE_M_PAINTED : TILE_M;
    ubo.updateFloat4("tsTile", tile[0], tile[1], tile[2], tile[3]);
    ubo.setTexture("tsSplat", this.splat);
    ubo.setTexture("tsGrass", this.layers[0]);
    ubo.setTexture("tsDirt", this.layers[1]);
    ubo.setTexture("tsStone", this.layers[2]);
    ubo.setTexture("tsGravel", this.layers[3]);
  }

  override getCustomCode(shaderType: string, shaderLanguage?: ShaderLanguage): { [pointName: string]: string } | null {
    if (shaderType !== "fragment" || (shaderLanguage ?? ShaderLanguage.GLSL) !== ShaderLanguage.GLSL) return null;
    return { CUSTOM_FRAGMENT_DEFINITIONS: FRAGMENT_DEFS, CUSTOM_FRAGMENT_UPDATE_DIFFUSE: FRAGMENT_DIFFUSE };
  }
}

let registered = false;
/** Registra o plugin do terreno (uma vez; antes de criar a cena). Só vale no GLSL (WebGL). */
export function registerTerrainPlugin(): void {
  if (registered) return;
  registered = true;
  RegisterMaterialPlugin(NAME, (material) => (material instanceof StandardMaterial ? new TerrainSplatPlugin(material) : null));
}

// ---------------------------------------------------------------------------------------
// Malha
// ---------------------------------------------------------------------------------------

export interface Terrain {
  mesh: Mesh;
  material: StandardMaterial;
  /** Preset: 0 = camadas simples (baixo), 1 = as quatro camadas. */
  setDetail(d: TerrainDetail): void;
}

/**
 * Cria a malha do terreno. `macro` é a pintura do chão (cores da região, estradas e praça)
 * que cobre o retângulo jogável; fora dele ela se estica (CLAMP) até o fim da malha.
 */
export function createTerrain(scene: Scene, field: HeightField, macro: BaseTexture): Terrain {
  const e = field.extent;
  const b = field.bounds;
  const nx = Math.ceil((e.maxX - e.minX) / CELL);
  const nz = Math.ceil((e.maxZ - e.minZ) / CELL);
  const dx = (e.maxX - e.minX) / nx;
  const dz = (e.maxZ - e.minZ) / nz;
  const vcount = (nx + 1) * (nz + 1);
  const positions = new Float32Array(vcount * 3);
  const normals = new Float32Array(vcount * 3);
  const uvs = new Float32Array(vcount * 2);
  const n3: [number, number, number] = [0, 1, 0];
  for (let j = 0; j <= nz; j++) {
    const z = e.minZ + j * dz;
    for (let i = 0; i <= nx; i++) {
      const x = e.minX + i * dx;
      const k = j * (nx + 1) + i;
      positions[k * 3] = x;
      positions[k * 3 + 1] = field.heightAt(x, z);
      positions[k * 3 + 2] = z;
      normalAt(field, x, z, n3);
      normals[k * 3] = n3[0];
      normals[k * 3 + 1] = n3[1];
      normals[k * 3 + 2] = n3[2];
      // A pintura cobre o retângulo jogável; u cresce para +X e v para +Z.
      uvs[k * 2] = (x - b.minX) / (b.maxX - b.minX);
      uvs[k * 2 + 1] = (z - b.minZ) / (b.maxZ - b.minZ);
    }
  }
  const indices = new Uint32Array(nx * nz * 6);
  let t = 0;
  for (let j = 0; j < nz; j++) {
    for (let i = 0; i < nx; i++) {
      const a = j * (nx + 1) + i;
      const bb = a + 1;
      const c = a + nx + 1;
      const d = c + 1;
      // Sentido que dá a face para cima (o Babylon é de mão esquerda).
      indices[t++] = a;
      indices[t++] = c;
      indices[t++] = bb;
      indices[t++] = bb;
      indices[t++] = c;
      indices[t++] = d;
    }
  }
  // Confere o sentido: a normal calculada pelo Babylon do primeiro triângulo tem de apontar para cima.
  const probe = new Float32Array(positions.length);
  VertexData.ComputeNormals(positions, indices, probe);
  if (probe[1] < 0) {
    for (let i = 0; i < indices.length; i += 3) {
      const tmp = indices[i + 1];
      indices[i + 1] = indices[i + 2];
      indices[i + 2] = tmp;
    }
  }

  const mesh = new Mesh("ground", scene);
  const data = new VertexData();
  data.positions = positions;
  data.normals = normals;
  data.uvs = uvs;
  data.indices = indices;
  data.applyToMesh(mesh);
  mesh.receiveShadows = true;
  mesh.doNotSyncBoundingInfo = true;
  mesh.freezeWorldMatrix();

  const baked = bakeSplat(field, SPLAT_CELL);
  const splat = RawTexture.CreateRGBATexture(baked.data, baked.width, baked.height, scene, false, false, Texture.BILINEAR_SAMPLINGMODE);
  splat.wrapU = splat.wrapV = Texture.CLAMP_ADDRESSMODE;
  splat.name = "terrainSplat";

  const material = new StandardMaterial("groundMat", scene);
  material.diffuseTexture = macro;
  material.specularColor = new Color3(0.03, 0.03, 0.03);
  mesh.material = material;
  const plugin = material.pluginManager?.getPlugin(NAME) as TerrainSplatPlugin | undefined;
  if (plugin) {
    plugin.splat = splat;
    plugin.painted = REMASTER;
    plugin.layers = LAYERS.map((_, i) => (REMASTER ? paintedTexture(scene, i) : layerTexture(scene, i)));
    // Mapa de mistura: texel i cobre x = minX + (i + 0.5) * cell; a amostra usa (p - min) / tamanho.
    plugin.ext = [e.minX, e.minZ, 1 / (baked.width * SPLAT_CELL), 1 / (baked.height * SPLAT_CELL)];
    plugin.enabled = true;
  }
  return {
    mesh,
    material,
    setDetail(d) {
      if (plugin) plugin.detail = d;
    },
  };
}
