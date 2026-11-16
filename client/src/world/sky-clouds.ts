/**
 * Nuvens procedurais no céu (tunnel#99): um plugin do material do céu (a esfera com o
 * degradê pintado por daynight.ts). Sem textura, sem malha e sem draw call novos: o shader
 * do céu soma as nuvens à cor do degradê, por pixel, com ruído em camadas (fbm).
 *
 * A direção do pixel (da câmera para a esfera) vira uma coordenada no plano das nuvens
 * (dir.xz / altura), que anda com o vento. As nuvens ficam mais finas perto do horizonte.
 * A cor vem da hora do dia (daynight.ts): topo claro na cor da luz do sol ou da lua, parte de
 * baixo na cor do céu da hora, e mais ralas à noite. Como o material do céu passa pela
 * gradação do render (ACES e curvas por região), a transição de cor acompanha a LUT.
 *
 * Presets: 0 = 2 camadas de ruído, nuvens paradas e sem sombreado (baixo); 1 = 3 camadas
 * andando devagar; 2 = 4 camadas, andando, com o lado de baixo escurecido (alto).
 */

import {
  MaterialPluginBase,
  RegisterMaterialPlugin,
  ShaderLanguage,
  StandardMaterial,
  type Color3,
  type Material,
  type MaterialDefines,
  type UniformBuffer,
} from "@babylonjs/core";

const NAME = "SkyClouds";

export type CloudDetail = 0 | 1 | 2;

const state = {
  time: 0,
  light: [1, 0.95, 0.9],
  shade: [0.5, 0.52, 0.62],
  /** Cobertura 0–1 (quanto do céu tem nuvem). */
  cover: 0.5,
  /** Opacidade máxima 0–1. */
  alpha: 0.85,
  /** Direção do sol no plano (para o sombreado). */
  sunX: 0.5,
  sunZ: 0.3,
};

export function setCloudTime(t: number): void {
  state.time = t;
}

/** Cores e cobertura da hora do dia. */
export function setCloudSky(light: Color3, shade: Color3, night: number, sunX: number, sunZ: number): void {
  state.light[0] = light.r;
  state.light[1] = light.g;
  state.light[2] = light.b;
  state.shade[0] = shade.r;
  state.shade[1] = shade.g;
  state.shade[2] = shade.b;
  // À noite as nuvens ralam e escurecem (a lua e as estrelas aparecem).
  state.cover = 0.52 - 0.14 * night;
  state.alpha = 0.88 - 0.32 * night;
  state.sunX = sunX;
  state.sunZ = sunZ;
}

const FRAGMENT_DEFS = `
#ifdef SKYCLOUDS
float scHash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }
float scNoise(vec2 p) {
  vec2 i = floor(p); vec2 f = fract(p); f = f * f * (3.0 - 2.0 * f);
  return mix(mix(scHash(i), scHash(i + vec2(1.0, 0.0)), f.x), mix(scHash(i + vec2(0.0, 1.0)), scHash(i + vec2(1.0, 1.0)), f.x), f.y);
}
float scFbm(vec2 p) {
  float a = 0.5; float s = 0.0;
  for (int i = 0; i < SKYCLOUDS_OCT; i++) { s += a * scNoise(p); p = p * 2.03 + vec2(17.1, 9.2); a *= 0.5; }
  return s;
}
#endif
`;

const FRAGMENT_FOG = `
#ifdef SKYCLOUDS
{
  vec3 scDir = normalize(vPositionW - vEyePosition.xyz);
  if (scDir.y > 0.01) {
    vec2 scP = scDir.xz / (scDir.y + 0.14) * 1.15 + scP0.xy;
    float scN = scFbm(scP);
    float scCover = smoothstep(scP0.z, scP0.z + 0.3, scN);
    // Mais fina perto do horizonte (some na névoa) e no zênite fica inteira.
    scCover *= smoothstep(0.01, 0.2, scDir.y);
    #ifdef SKYCLOUDS_SHADE
    // Sombreado: uma segunda amostra deslocada para o lado do sol; mais denso no lado
    // oposto ao sol fica mais escuro (o topo da nuvem pega a luz).
    float scN2 = scFbm(scP + scSun.xy * 0.09);
    float scLit = clamp(0.55 + (scN - scN2) * 3.2, 0.0, 1.0);
    #else
    float scLit = 0.65;
    #endif
    vec3 scCol = mix(scShade.rgb, scLight.rgb, scLit);
    color.rgb = mix(color.rgb, scCol, scCover * scP0.w);
  }
}
#endif
`;

class SkyCloudsPlugin extends MaterialPluginBase {
  private _detail: CloudDetail = 1;
  private _on = false;

  constructor(material: Material) {
    super(material, NAME, 320, { SKYCLOUDS: false, SKYCLOUDS_OCT: 3, SKYCLOUDS_SHADE: false });
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

  get detail(): CloudDetail {
    return this._detail;
  }
  set detail(v: CloudDetail) {
    if (v === this._detail) return;
    this._detail = v;
    this.markAllDefinesAsDirty();
  }

  override getClassName(): string {
    return "SkyCloudsPlugin";
  }

  override isCompatible(shaderLanguage: ShaderLanguage): boolean {
    return shaderLanguage === ShaderLanguage.GLSL;
  }

  override prepareDefines(defines: MaterialDefines): void {
    defines["SKYCLOUDS"] = this._on;
    defines["SKYCLOUDS_OCT"] = this._detail === 0 ? 2 : this._detail === 1 ? 3 : 4;
    defines["SKYCLOUDS_SHADE"] = this._on && this._detail === 2;
  }

  override getUniforms(): { ubo: { name: string; size: number; type: string }[]; fragment: string } {
    return {
      ubo: [
        { name: "scP0", size: 4, type: "vec4" },
        { name: "scLight", size: 4, type: "vec4" },
        { name: "scShade", size: 4, type: "vec4" },
        { name: "scSun", size: 4, type: "vec4" },
      ],
      fragment: `#ifdef SKYCLOUDS
uniform vec4 scP0;
uniform vec4 scLight;
uniform vec4 scShade;
uniform vec4 scSun;
#endif`,
    };
  }

  override bindForSubMesh(ubo: UniformBuffer): void {
    if (!this._on) return;
    // Baixo: nuvens paradas. As outras andam devagar para o oeste e um pouco para o sul.
    const t = this._detail === 0 ? 0 : state.time;
    ubo.updateFloat4("scP0", t * 0.006, t * 0.0025, 1 - state.cover - 0.15, state.alpha);
    ubo.updateFloat4("scLight", state.light[0], state.light[1], state.light[2], 1);
    ubo.updateFloat4("scShade", state.shade[0], state.shade[1], state.shade[2], 1);
    ubo.updateFloat4("scSun", state.sunX, state.sunZ, 0, 0);
  }

  override getCustomCode(shaderType: string, shaderLanguage?: ShaderLanguage): { [pointName: string]: string } | null {
    if (shaderType !== "fragment" || (shaderLanguage ?? ShaderLanguage.GLSL) !== ShaderLanguage.GLSL) return null;
    return { CUSTOM_FRAGMENT_DEFINITIONS: FRAGMENT_DEFS, CUSTOM_FRAGMENT_BEFORE_FOG: FRAGMENT_FOG };
  }
}

let registered = false;
/** Registra o plugin das nuvens (uma vez; antes de criar a cena). */
export function registerCloudsPlugin(): void {
  if (registered) return;
  registered = true;
  RegisterMaterialPlugin(NAME, (m) => (m instanceof StandardMaterial ? new SkyCloudsPlugin(m) : null));
}

/** Liga as nuvens no material do céu (e devolve a troca de preset). */
export function enableClouds(material: Material): (d: CloudDetail) => void {
  const p = material.pluginManager?.getPlugin(NAME) as SkyCloudsPlugin | undefined;
  if (p) p.enabled = true;
  return (d) => {
    if (p) p.detail = d;
  };
}
