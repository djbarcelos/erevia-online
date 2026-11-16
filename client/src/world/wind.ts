/**
 * Vento na grama, nos arbustos e nas árvores (tunnel#99): no vertex shader, sem animação em
 * CPU. É um plugin dos materiais Standard e PBR (como o material de paleta), então vale para
 * as thin instances do cenário sem mudar como elas são desenhadas.
 *
 * O deslocamento sai de uma soma de senos no tempo; a fase vem da posição da instância (a
 * grama não balança toda junta) e a força cresce com o quadrado da altura do vértice (a base
 * fica presa ao chão). Perto da câmera o vento é cheio e some com a distância: o preset
 * escolhe o alcance (no baixo, a grama só mexe perto e as árvores ficam paradas).
 *
 * O mapa de sombras não leva o vento (a sombra balança menos que a árvore; é pouco).
 */

import {
  MaterialPluginBase,
  PBRBaseMaterial,
  RegisterMaterialPlugin,
  ShaderLanguage,
  StandardMaterial,
  type Material,
  type MaterialDefines,
  type UniformBuffer,
} from "@babylonjs/core";

export type WindKind = "grass" | "bush" | "tree" | "off";

const NAME = "WindSway";

/** Força (fração da altura do objeto, no topo) e frequência por tipo. */
const KINDS: Record<Exclude<WindKind, "off">, { amp: number; freq: number }> = {
  grass: { amp: 0.16, freq: 1.0 },
  bush: { amp: 0.045, freq: 0.8 },
  tree: { amp: 0.014, freq: 0.55 },
};

/** Estado global: o tempo e o alcance de cada tipo (o preset de qualidade escolhe). */
const state = {
  time: 0,
  /** Alcance do vento (m) por tipo; 0 desliga. */
  range: { grass: 60, bush: 60, tree: 90 } as Record<Exclude<WindKind, "off">, number>,
};

/** Tempo do vento em segundos (DayNight.update chama todo quadro). */
export function setWindTime(t: number): void {
  state.time = t;
}

/** Alcance do vento em metros: grama/arbusto e árvore (0 = parado). */
export function setWindRange(grass: number, tree: number): void {
  state.range.grass = grass;
  state.range.bush = grass;
  state.range.tree = tree;
}

/** Marca o material como vento de `kind`; `height` é a altura do modelo (unidades do modelo). */
export function setWindKind(material: Material | null | undefined, kind: WindKind, height: number): void {
  const p = material?.pluginManager?.getPlugin(NAME);
  if (p instanceof WindPlugin) {
    p.height = height;
    p.kind = kind;
  }
}

const VERTEX_DEFS = `
#ifdef WINDSWAY
vec3 windOffset(vec3 root, float h, vec4 P, vec4 Q) {
  // P: tempo, força, altura de referência (m), alcance (m); Q: frequência.
  float t = P.x * Q.x;
  float ph = root.x * 0.23 + root.z * 0.17;
  // Rajada longa (sobe e desce devagar) + balanço + tremor curto.
  float gust = 0.55 + 0.45 * sin(t * 0.37 + root.x * 0.045 + root.z * 0.03);
  float s = sin(t * 1.9 + ph) * 0.55 + sin(t * 3.1 + ph * 1.7 + 1.3) * 0.25 + 0.35;
  float f = sin(t * 4.3 + ph * 2.3) * 0.12;
  float k = P.y * P.z * h * h;
  return vec3(s * gust + f, 0.0, (s * 0.35 - f) * gust) * k;
}
#endif
`;

const VERTEX_WORLDPOS = `
#ifdef WINDSWAY
if (windP.w > 0.0) {
  float windDist = length(worldPos.xyz - vEyePosition.xyz);
  float windFade = 1.0 - smoothstep(windP.w * 0.55, windP.w, windDist);
  if (windFade > 0.0) {
    float windSc = length(finalWorld[1].xyz);
    float windH = clamp(positionUpdated.y / max(windQ.y, 0.0001), 0.0, 1.0);
    vec3 windRoot = vec3(finalWorld[3].x, 0.0, finalWorld[3].z);
    vec4 windPs = vec4(windP.x, windP.y * windFade, windQ.y * windSc, windP.w);
    worldPos.xyz += windOffset(windRoot, windH, windPs, windQ);
  }
}
#endif
`;

class WindPlugin extends MaterialPluginBase {
  private _kind: WindKind = "off";
  /** Altura do modelo (unidades do modelo), para saber quão perto do topo está o vértice. */
  height = 1;

  constructor(material: Material) {
    super(material, NAME, 310, { WINDSWAY: false });
    this.doNotSerialize = true;
    this._enable(true);
  }

  get kind(): WindKind {
    return this._kind;
  }
  set kind(v: WindKind) {
    if (v === this._kind) return;
    const was = this._kind === "off";
    this._kind = v;
    if (was || v === "off") this.markAllDefinesAsDirty();
  }

  override getClassName(): string {
    return "WindPlugin";
  }

  override isCompatible(shaderLanguage: ShaderLanguage): boolean {
    return shaderLanguage === ShaderLanguage.GLSL;
  }

  override prepareDefines(defines: MaterialDefines): void {
    defines["WINDSWAY"] = this._kind !== "off";
  }

  override getUniforms(): { ubo: { name: string; size: number; type: string }[]; vertex: string; fragment: string } {
    const decl = `#ifdef WINDSWAY
uniform vec4 windP;
uniform vec4 windQ;
#endif`;
    return {
      ubo: [
        { name: "windP", size: 4, type: "vec4" },
        { name: "windQ", size: 4, type: "vec4" },
      ],
      vertex: decl,
      fragment: "",
    };
  }

  override bindForSubMesh(ubo: UniformBuffer): void {
    if (this._kind === "off") return;
    const k = KINDS[this._kind];
    // windQ.y = altura do modelo; windP.z = a mesma, que o shader multiplica pela escala da instância.
    ubo.updateFloat4("windP", state.time, k.amp, 1, state.range[this._kind]);
    ubo.updateFloat4("windQ", k.freq, this.height, 0, 0);
  }

  override getCustomCode(shaderType: string, shaderLanguage?: ShaderLanguage): { [pointName: string]: string } | null {
    if (shaderType !== "vertex" || (shaderLanguage ?? ShaderLanguage.GLSL) !== ShaderLanguage.GLSL) return null;
    return { CUSTOM_VERTEX_DEFINITIONS: VERTEX_DEFS, CUSTOM_VERTEX_UPDATE_WORLDPOS: VERTEX_WORLDPOS };
  }

  override copyTo(plugin: MaterialPluginBase): void {
    if (plugin instanceof WindPlugin) {
      plugin.height = this.height;
      plugin.kind = this._kind;
    }
  }
}

let registered = false;
/** Registra o vento em todo material Standard/PBR criado daqui em diante (antes de carregar o cenário). */
export function registerWindPlugin(): void {
  if (registered) return;
  registered = true;
  RegisterMaterialPlugin(NAME, (m) => (m instanceof StandardMaterial || m instanceof PBRBaseMaterial ? new WindPlugin(m) : null));
}
