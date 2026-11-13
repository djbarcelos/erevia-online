/**
 * Material de paleta (tunnel#92): um plugin nos materiais Standard e PBR do Babylon, em vez
 * de um material novo. Assim continuam valendo esqueleto, thin instances, sombras, névoa e
 * a gradação de cor (render.ts), e nenhum carregador precisa mudar. Três ajustes, no fim do
 * cálculo de luz e antes da névoa (docs/guia-de-arte.md, seção 4):
 * - luz suave em degradê: um pouco mais escuro embaixo e mais claro em cima (pela normal);
 * - luz de borda leve (fresnel) na cor da luz do sol/lua misturada com o destaque da região;
 * - sombra puxada para a cor da paleta: o que está escuro vai para a cor de sombra da região,
 *   em vez do preto.
 *
 * Custo: ~20 instruções por pixel, sem textura. Os valores (cores e forças) são 3 vec4 no
 * uniform buffer de cada material, escritos no bind; nada recompila ao trocar de região.
 * Personagens e monstros ("char") têm borda e degradê mais fortes que o cenário ("scene").
 * Materiais sem luz (efeitos, anéis, céu) e aditivos ficam de fora sozinhos.
 */

import {
  Constants,
  MaterialPluginBase,
  PBRBaseMaterial,
  PBRMaterial,
  RegisterMaterialPlugin,
  ShaderLanguage,
  StandardMaterial,
  type AbstractMesh,
  type Material,
  type MaterialDefines,
  type Scene,
  type UniformBuffer,
} from "@babylonjs/core";

export type PaletteKind = "char" | "scene" | "off";

const NAME = "PaletteLight";

/** Força de cada ajuste por tipo de objeto. */
const KINDS: Record<Exclude<PaletteKind, "off">, { rim: number; shade: number; lo: number; hi: number }> = {
  // Personagens: silhueta legível a 85 px (borda) e volume (degradê).
  char: { rim: 0.38, shade: 0.55, lo: 0.82, hi: 1.06 },
  // Cenário: menos que os personagens, para os olhos irem para quem importa (guia 3.1).
  scene: { rim: 0.1, shade: 0.4, lo: 0.93, hi: 1.02 },
};
/** Luminância (espaço gama) abaixo da qual a cor começa a ir para a sombra da paleta. */
const DARK_LUMA = 0.38;

/** Estado compartilhado: render.ts escreve a cada quadro (cor da região e da hora). */
const state = {
  on: true,
  shade: [0.23, 0.23, 0.32],
  rim: [1, 0.9, 0.75],
};
const plugins = new Set<PaletteLightPlugin>();

/** Cores da paleta do momento (0–1, espaço gama). */
export function setPaletteColors(shade: { r: number; g: number; b: number }, rim: { r: number; g: number; b: number }): void {
  state.shade[0] = shade.r;
  state.shade[1] = shade.g;
  state.shade[2] = shade.b;
  state.rim[0] = rim.r;
  state.rim[1] = rim.g;
  state.rim[2] = rim.b;
}

/** Liga/desliga o material em todos (o ?render=antigo desliga). Recompila os materiais. */
export function setPaletteEnabled(on: boolean): void {
  if (state.on === on) return;
  state.on = on;
  for (const p of plugins) p.markAllDefinesAsDirty();
}

/** Marca o material como personagem/monstro, cenário (o padrão) ou sem ajuste. */
export function setPaletteKind(material: Material | null | undefined, kind: PaletteKind): void {
  const p = material?.pluginManager?.getPlugin(NAME);
  if (p instanceof PaletteLightPlugin) p.kind = kind;
}

/** Código GLSL comum aos dois materiais. `c` em espaço gama. */
const FUNCTIONS = `
#ifdef PALETTELIGHT
vec3 paletteLight(vec3 c, vec3 n, vec3 v) {
  float up = clamp(n.y * 0.5 + 0.5, 0.0, 1.0);
  c *= mix(plGrad.x, plGrad.y, up);
  float fres = 1.0 - abs(dot(n, v));
  c += plRim.rgb * (fres * fres * fres * plRim.a * (0.4 + 0.6 * up));
  float l = dot(c, vec3(0.299, 0.587, 0.114));
  float dark = 1.0 - smoothstep(0.0, plGrad.z, l);
  return mix(c, max(c, plShade.rgb), dark * plShade.a);
}
#endif
`;

class PaletteLightPlugin extends MaterialPluginBase {
  private _kind: PaletteKind = "scene";
  private readonly pbr: boolean;

  constructor(material: Material) {
    super(material, NAME, 300, { PALETTELIGHT: false });
    this.pbr = material instanceof PBRBaseMaterial;
    // A cópia de um material (clone) ganha um plugin novo pela fábrica; o tipo vem em copyTo.
    this.doNotSerialize = true;
    this._enable(true);
    plugins.add(this);
  }

  get kind(): PaletteKind {
    return this._kind;
  }

  set kind(v: PaletteKind) {
    if (v === this._kind) return;
    const wasOff = this._kind === "off";
    this._kind = v;
    if (wasOff || v === "off") this.markAllDefinesAsDirty();
  }

  override getClassName(): string {
    return "PaletteLightPlugin";
  }

  override isCompatible(): boolean {
    return true; // o código só existe em GLSL; no WGSL (WebGPU) o plugin não faz nada
  }

  private active(): boolean {
    if (!state.on || this._kind === "off") return false;
    const m = this._material;
    if (m.shaderLanguage !== ShaderLanguage.GLSL) return false;
    if (m instanceof StandardMaterial && m.disableLighting) return false;
    if (m instanceof PBRMaterial && m.unlit) return false;
    return m.alphaMode !== Constants.ALPHA_ADD;
  }

  override prepareDefines(defines: MaterialDefines, _scene: Scene, _mesh: AbstractMesh): void {
    defines["PALETTELIGHT"] = this.active();
  }

  override getUniforms(): { ubo: { name: string; size: number; type: string }[]; fragment: string } {
    return {
      ubo: [
        { name: "plRim", size: 4, type: "vec4" },
        { name: "plShade", size: 4, type: "vec4" },
        { name: "plGrad", size: 4, type: "vec4" },
      ],
      fragment: `#ifdef PALETTELIGHT
uniform vec4 plRim;
uniform vec4 plShade;
uniform vec4 plGrad;
#endif`,
    };
  }

  override bindForSubMesh(ubo: UniformBuffer): void {
    if (this._kind === "off") return;
    const k = KINDS[this._kind];
    ubo.updateFloat4("plRim", state.rim[0], state.rim[1], state.rim[2], k.rim);
    ubo.updateFloat4("plShade", state.shade[0], state.shade[1], state.shade[2], k.shade);
    ubo.updateFloat4("plGrad", k.lo, k.hi, DARK_LUMA, 0);
  }

  override getCustomCode(shaderType: string, shaderLanguage?: ShaderLanguage): { [pointName: string]: string } | null {
    if (shaderType !== "fragment" || (shaderLanguage ?? ShaderLanguage.GLSL) !== ShaderLanguage.GLSL) return null;
    // Os dois shaders têm o ponto "antes da névoa" (no PBR, dentro de pbrBlockFinalColorComposition).
    // O PBR trabalha em espaço linear: converte para gama e de volta.
    const apply = this.pbr
      ? "finalColor.rgb = toLinearSpace(paletteLight(toGammaSpace(max(finalColor.rgb, vec3(0.0))), normalW, viewDirectionW));"
      : "color.rgb = paletteLight(max(color.rgb, vec3(0.0)), normalW, viewDirectionW);";
    return {
      CUSTOM_FRAGMENT_DEFINITIONS: FUNCTIONS,
      CUSTOM_FRAGMENT_BEFORE_FOG: `#ifdef PALETTELIGHT\n${apply}\n#endif`,
    };
  }

  override copyTo(plugin: MaterialPluginBase): void {
    if (plugin instanceof PaletteLightPlugin) plugin.kind = this._kind;
  }

  override dispose(forceDisposeTextures?: boolean): void {
    plugins.delete(this);
    super.dispose(forceDisposeTextures);
  }
}

let registered = false;

/**
 * Registra o plugin em todo material Standard/PBR criado daqui em diante (inclusive os do
 * glTF e as cópias). Chamado antes de criar a cena; não vale para o WebGPU (?webgpu=1).
 */
export function registerPaletteMaterial(): void {
  if (registered) return;
  registered = true;
  RegisterMaterialPlugin(NAME, (material) =>
    material instanceof StandardMaterial || material instanceof PBRBaseMaterial ? new PaletteLightPlugin(material) : null,
  );
}
