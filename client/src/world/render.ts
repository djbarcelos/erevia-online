/**
 * Render novo (tunnel#92): presets de qualidade, mapeamento de tons ACES, gradação de cor e
 * névoa por região e hora do dia, bloom, FXAA, vinheta e SSAO.
 *
 * Como fica barato:
 * - Tons (ACES), exposição, curvas de cor (a gradação) e vinheta rodam **dentro dos
 *   materiais** (ImageProcessingConfiguration sem passe de pós), como o Babylon já fazia
 *   para a conversão de gama: são algumas contas por pixel, sem textura nem passe extra.
 *   A gradação por região e hora é feita mexendo em ~13 números (ColorCurves), sem LUT.
 * - O bloom é a GlowLayer que já existia (só as malhas incluídas brilham: chamas, efeitos,
 *   runas e armas raras). O bloom por limiar do DefaultRenderingPipeline fica desligado:
 *   sem HDR ele pegaria o céu e tudo que é claro, e somaria um segundo desfoque.
 * - O DefaultRenderingPipeline (FXAA) só entra no preset alto, junto com o SSAO: com um
 *   passe de pós o MSAA do canvas deixa de valer, então o FXAA o substitui. Sem pós
 *   (baixa e média), o MSAA do canvas continua, como hoje.
 *
 * Custos estimados (ver o PR): baixa < hoje; média ≈ hoje; alta = hoje + SSAO + FXAA.
 *
 * URL (para medir com ?bench=1): ?quality=baixa|media|alta força o preset sem salvar;
 * ?render=antigo desliga tudo isto e reproduz o render de antes, para comparar.
 */

import {
  Color3,
  Color4,
  ColorCurves,
  DefaultRenderingPipeline,
  ImageProcessingConfiguration,
  SSAO2RenderingPipeline,
  ShadowGenerator,
  type GlowLayer,
  type Vector3,
} from "@babylonjs/core";
import { QUALITIES, settings, type Quality, type Settings } from "../ui/kit";
import { RAID_LAIR, regionAt, type Region } from "../ui/regions";
import { setPaletteColors, setPaletteEnabled } from "./palette-material";
import type { WorldScene } from "./scene";

// ---------------------------------------------------------------------------------------
// Presets
// ---------------------------------------------------------------------------------------

interface Preset {
  /** Nível de escala do render (1 = resolução do canvas; 1,25 = 80% de cada lado, 64% dos pixels). */
  scale: number;
  /** Bloom: a GlowLayer (camada de brilho das chamas, efeitos e armas). */
  glow: boolean;
  glowKernel: number;
  shadowSize: number;
  shadowQuality: number;
  /** Vinheta leve (nos materiais, quase de graça). */
  vignette: boolean;
  /** Passe de pós (DefaultRenderingPipeline) com FXAA. */
  fxaa: boolean;
  /** Oclusão de ambiente em tela (SSAO2, meia resolução). */
  ssao: boolean;
  /** Gradação (ACES, curvas, névoa por região) e o material de paleta. Só o "antigo" desliga. */
  grade: boolean;
}

const PRESETS: Record<Quality | "antigo", Preset> = {
  baixa: { scale: 1.25, glow: false, glowKernel: 32, shadowSize: 1024, shadowQuality: ShadowGenerator.QUALITY_LOW, vignette: false, fxaa: false, ssao: false, grade: true },
  media: { scale: 1, glow: true, glowKernel: 32, shadowSize: 2048, shadowQuality: ShadowGenerator.QUALITY_LOW, vignette: true, fxaa: false, ssao: false, grade: true },
  alta: { scale: 1, glow: true, glowKernel: 48, shadowSize: 2048, shadowQuality: ShadowGenerator.QUALITY_MEDIUM, vignette: true, fxaa: true, ssao: true, grade: true },
  // O render de antes do tunnel#92 (para comparar no ?bench=1).
  antigo: { scale: 1, glow: true, glowKernel: 48, shadowSize: 2048, shadowQuality: ShadowGenerator.QUALITY_MEDIUM, vignette: false, fxaa: false, ssao: false, grade: false },
};

const params = new URLSearchParams(location.search);
/** ?render=antigo: o render de antes, para comparar. */
export const LEGACY_RENDER = params.get("render") === "antigo";
/** ?quality=baixa|media|alta: força o preset (sem salvar), para medir. */
const URL_QUALITY = QUALITIES.find((q) => q === params.get("quality")) ?? null;

/** Preset em uso agora: o da URL vence o salvo; o render antigo conta como o alto (efeitos de antes). */
export function activeQuality(): Quality {
  return LEGACY_RENDER ? "alta" : (URL_QUALITY ?? settings.get().quality);
}

/**
 * Exposição antes do ACES. O ACES leva o branco (1,0) a ~0,62 e o cinza médio (0,2) a ~0,12;
 * com 1,5 os tons médios ficam onde estavam (0,2 → 0,21; 0,5 → 0,52) e só o alto comprime.
 * Fica fixa: mudar a exposição liga/desliga um define e recompila todos os materiais.
 */
const EXPOSURE = 1.5;

// ---------------------------------------------------------------------------------------
// Cor por região (docs/guia-de-arte.md, seção 3.3)
// ---------------------------------------------------------------------------------------

interface Look {
  /** Sombra/névoa da tabela: puxa as sombras, a névoa e a luz de baixo para esta cor. */
  shade: string;
  /** Destaque da tabela: puxa as luzes (realces) para esta cor. */
  accent: string;
  /** Saturação global das curvas (-100 a 100). */
  sat: number;
  /** Densidade do matiz nas sombras e nas luzes (0 a 100; 45 ≈ 10% de tinta). */
  shadowTint: number;
  lightTint: number;
  /** Exposição das curvas (-100 a 100, não linear: -15 ≈ 1% e -40 ≈ 8% mais escuro). */
  exposure: number;
  /** Multiplica a densidade da névoa da hora. */
  fog: number;
  /** Quanto a névoa vai para a cor da sombra (0–1), mantendo o brilho da hora... */
  fogTint: number;
  /** ...vezes este fator (1 = o brilho da hora). */
  fogValue: number;
}

/** Nome da região (ui/regions.ts) → visual. "Cidade" é Vel'Aran. */
const LOOKS: Record<string, Look> = {
  Cidade: { shade: "#3b3a52", accent: "#d6b46e", sat: 8, shadowTint: 45, lightTint: 30, exposure: 0, fog: 0.9, fogTint: 0.2, fogValue: 1 },
  Arredores: { shade: "#2f4a3a", accent: "#e8c25a", sat: 10, shadowTint: 40, lightTint: 25, exposure: 0, fog: 1, fogTint: 0.2, fogValue: 1 },
  "Campos dos Ossos": { shade: "#4a3f4f", accent: "#b04a2e", sat: -18, shadowTint: 50, lightTint: 20, exposure: -15, fog: 1.15, fogTint: 0.35, fogValue: 0.9 },
  "Ermos Partidos": { shade: "#3d2a2a", accent: "#d8873a", sat: -6, shadowTint: 45, lightTint: 38, exposure: 0, fog: 1.1, fogTint: 0.35, fogValue: 0.95 },
  "Vale das Pedras Vivas": { shade: "#2c3440", accent: "#f0a640", sat: -8, shadowTint: 55, lightTint: 22, exposure: -15, fog: 1.2, fogTint: 0.4, fogValue: 0.9 },
  [RAID_LAIR.name]: { shade: "#14121c", accent: "#8a5cff", sat: -22, shadowTint: 65, lightTint: 30, exposure: -40, fog: 1.6, fogTint: 0.6, fogValue: 0.5 },
};
/** Anéis de fora que o guia ainda não cobre: usam o visual mais próximo. */
const LOOK_ALIASES: Record<string, string> = { "Terras Mortas": "Ermos Partidos", Confins: "Ermos Partidos" };
const DEFAULT_LOOK = "Arredores";

/** Hora do dia: sombra azul-violeta à noite (guia 3.1), luz dourada no nascer e no pôr do sol. */
const NIGHT_SHADE = Color3.FromHexString("#262a4a");
const MOON_LIGHT = Color3.FromHexString("#a6baff");
const GOLDEN_LIGHT = Color3.FromHexString("#ff9a5a");

/** Tempo (s) da transição ao trocar de região (constante da exponencial). */
const TRANSITION_S = 1.2;
/** Intervalo entre as consultas de região (ms): regionAt não precisa rodar todo quadro. */
const REGION_CHECK_MS = 250;

// Vetor de números interpolados: sombra (rgb), destaque (rgb) e os campos numéricos do Look.
const N = 13;
function lookVector(l: Look): Float32Array {
  const s = Color3.FromHexString(l.shade);
  const a = Color3.FromHexString(l.accent);
  return Float32Array.from([s.r, s.g, s.b, a.r, a.g, a.b, l.sat, l.shadowTint, l.lightTint, l.exposure, l.fog, l.fogTint, l.fogValue]);
}
const LOOK_VECTORS = new Map(Object.entries(LOOKS).map(([k, l]) => [k, lookVector(l)]));

const lum = (c: Color3) => 0.299 * c.r + 0.587 * c.g + 0.114 * c.b;

/** Matiz (0–360) de uma cor, para as curvas. */
function hueOf(c: Color3): number {
  const max = Math.max(c.r, c.g, c.b);
  const min = Math.min(c.r, c.g, c.b);
  const d = max - min;
  if (d < 1e-5) return 0;
  let h: number;
  if (max === c.r) h = ((c.g - c.b) / d) % 6;
  else if (max === c.g) h = (c.b - c.r) / d + 2;
  else h = (c.r - c.g) / d + 4;
  return (h * 60 + 360) % 360;
}

/** `c` com o matiz de `tint` e o brilho (luminância) de `c` vezes `value`. */
function tintKeepingValue(c: Color3, tint: Color3, value: number, out: Color3): Color3 {
  const k = (lum(c) * value) / Math.max(lum(tint), 0.02);
  out.set(Math.min(1, tint.r * k), Math.min(1, tint.g * k), Math.min(1, tint.b * k));
  return out;
}

/** Cores e números da hora (daynight.ts) que a região ajusta antes de ir para a cena. */
export interface TimeLook {
  fog: Color3;
  fogDensity: number;
  hemiSky: Color3;
  hemiGround: Color3;
  /** Cor da luz do sol/lua (dá a cor da luz de borda do material). */
  light: Color3;
  /** 0 (dia) a 1 (noite). */
  night: number;
  /** 0–1: nascer ou pôr do sol. */
  golden: number;
}

// ---------------------------------------------------------------------------------------
// Sistema
// ---------------------------------------------------------------------------------------

/**
 * Aplica o preset (e reaplica na hora quando o jogador troca), e mistura a cor da região
 * com a da hora a cada quadro. Criado pelo DayNight, que chama update() todo quadro.
 */
export class RenderQuality {
  private regions: Region[] = [];
  private curves = new ColorCurves();
  private preset: Preset | null = null;
  private presetName = "";
  private ssao: SSAO2RenderingPipeline | null = null;
  private pipeline: DefaultRenderingPipeline | null = null;
  private cur = Float32Array.from(LOOK_VECTORS.get(DEFAULT_LOOK)!);
  private target: Float32Array = LOOK_VECTORS.get(DEFAULT_LOOK)!;
  private snap = true;
  private lastFrame = performance.now();
  private nextRegionCheck = 0;
  private lastQuality: Quality | null = null;
  private lastShadows: boolean | null = null;
  // Cores de trabalho (sem alocar por quadro).
  private shade = new Color3();
  private accent = new Color3();
  private tmp = new Color3();
  private fogOut = new Color3();
  private skyOut = new Color3();
  private groundOut = new Color3();
  private rim = new Color3();

  constructor(
    private world: WorldScene,
    private glow: GlowLayer,
  ) {
    const scene = world.scene;
    setPaletteEnabled(!LEGACY_RENDER);
    if (!LEGACY_RENDER) {
      const ip = scene.imageProcessingConfiguration;
      ip.toneMappingEnabled = true;
      ip.toneMappingType = ImageProcessingConfiguration.TONEMAPPING_ACES;
      ip.exposure = EXPOSURE;
      ip.colorCurves = this.curves;
      ip.colorCurvesEnabled = true;
      ip.vignetteBlendMode = ImageProcessingConfiguration.VIGNETTEMODE_MULTIPLY;
      ip.vignetteWeight = 1.2;
      ip.vignetteColor = new Color4(0, 0, 0, 0);
    }
    this.apply(settings.get());
    const off = settings.on((s) => {
      if (scene.isDisposed) return;
      if (s.quality !== this.lastQuality || s.shadows !== this.lastShadows) this.apply(s);
    });
    scene.onDisposeObservable.addOnce(() => off());
  }

  /** Regiões do mapa (do Welcome), para a cor por região. */
  setRegions(regions: Region[]): void {
    this.regions = regions;
    this.nextRegionCheck = 0;
    this.snap = true; // a primeira região entra sem transição
  }

  /** Nome do preset em uso ("baixa", "media", "alta" ou "antigo"). */
  get name(): string {
    return this.presetName;
  }

  private apply(s: Settings): void {
    this.lastQuality = s.quality;
    this.lastShadows = s.shadows;
    const name = LEGACY_RENDER ? "antigo" : (URL_QUALITY ?? s.quality);
    const p = PRESETS[name];
    const { engine, scene, shadows, sun } = this.world;

    if (engine.getHardwareScalingLevel() !== p.scale) engine.setHardwareScalingLevel(p.scale);
    this.glow.isEnabled = p.glow;
    if (this.glow.blurKernelSize !== p.glowKernel) this.glow.blurKernelSize = p.glowKernel;
    if (shadows.mapSize !== p.shadowSize) shadows.mapSize = p.shadowSize;
    if (shadows.filteringQuality !== p.shadowQuality) shadows.filteringQuality = p.shadowQuality;
    sun.shadowEnabled = s.shadows;
    if (p.grade && scene.imageProcessingConfiguration.vignetteEnabled !== p.vignette) {
      scene.imageProcessingConfiguration.vignetteEnabled = p.vignette;
    }
    if (!this.preset || this.preset.fxaa !== p.fxaa || this.preset.ssao !== p.ssao) this.buildPost(p);
    this.preset = p;
    this.presetName = name;
    console.info(`render: preset ${name} (escala ${p.scale}, brilho ${p.glow ? "sim" : "não"}, sombra ${p.shadowSize}, SSAO ${p.ssao && !!this.ssao ? "sim" : "não"}, FXAA ${p.fxaa ? "sim" : "não"})`);
  }

  /**
   * (Re)monta o pós-processamento. A ordem importa: SSAO antes do FXAA. Só roda ao
   * trocar de preset; o baixo e o médio não têm passe de pós nenhum.
   */
  private buildPost(p: Preset): void {
    const { scene, camera } = this.world;
    this.ssao?.dispose(true); // true: desliga também o passe de geometria (GeometryBufferRenderer)
    this.ssao = null;
    this.pipeline?.dispose();
    this.pipeline = null;
    if (p.ssao && SSAO2RenderingPipeline.IsSupported) {
      // GeometryBufferRenderer próprio (forceGeometryBuffer): um passe a mais de geometria,
      // mas sem mexer no passe principal (o pre-pass mudaria o shader de todos os materiais).
      const ssao = new SSAO2RenderingPipeline("ssao", scene, { ssaoRatio: 0.5, blurRatio: 0.5 }, [camera], true);
      ssao.radius = 1.2;
      ssao.totalStrength = 0.9;
      ssao.base = 0.15;
      ssao.samples = 8;
      ssao.maxZ = 60;
      ssao.minZAspect = 0.4;
      ssao.expensiveBlur = false;
      this.ssao = ssao;
    }
    if (p.fxaa) {
      // Sem HDR: o ImageProcessing continua nos materiais (o pipeline só o move para um
      // passe próprio quando é HDR). Bloom desligado: o bloom é a GlowLayer.
      const pipeline = new DefaultRenderingPipeline("render", false, scene, [camera]);
      pipeline.bloomEnabled = false;
      pipeline.fxaaEnabled = true;
      this.pipeline = pipeline;
    }
    // O céu cobre a tela inteira (scene.ts); o pipeline liga a limpeza ao ser descartado.
    scene.autoClear = false;
  }

  /**
   * Um quadro: troca de região (com transição suave), mistura com a hora e escreve névoa,
   * luz ambiente e curvas. Só interpola ~13 números; regionAt roda 4 vezes por segundo.
   */
  update(focus: Vector3 | undefined, t: TimeLook): void {
    const { scene, hemi } = this.world;
    if (LEGACY_RENDER) {
      scene.fogColor = t.fog;
      scene.fogDensity = t.fogDensity;
      scene.clearColor = new Color4(t.fog.r, t.fog.g, t.fog.b, 1);
      hemi.diffuse = t.hemiSky;
      hemi.groundColor = t.hemiGround;
      return;
    }
    const now = performance.now();
    const dt = Math.min((now - this.lastFrame) / 1000, 0.25);
    this.lastFrame = now;
    if (focus && this.regions.length && now >= this.nextRegionCheck) {
      this.nextRegionCheck = now + REGION_CHECK_MS;
      this.target = this.lookAt(focus.x, focus.z);
      if (this.snap) this.cur.set(this.target);
      this.snap = false;
    }
    const cur = this.cur;
    const tg = this.target;
    const k = 1 - Math.exp(-dt / TRANSITION_S);
    for (let i = 0; i < N; i++) cur[i] += (tg[i] - cur[i]) * k;

    // Hora do dia por cima da região.
    const n = t.night;
    const g = t.golden;
    const shade = this.shade.set(cur[0], cur[1], cur[2]);
    Color3.LerpToRef(shade, NIGHT_SHADE, n * 0.5, shade);
    const accent = this.accent.set(cur[3], cur[4], cur[5]);
    Color3.LerpToRef(accent, GOLDEN_LIGHT, g * 0.5, accent);
    Color3.LerpToRef(accent, MOON_LIGHT, n * 0.6, accent);

    const c = this.curves;
    c.globalSaturation = cur[6] - 10 * n + 6 * g;
    c.shadowsHue = hueOf(shade);
    c.shadowsDensity = cur[7] + 10 * n;
    c.highlightsHue = hueOf(accent);
    c.highlightsDensity = cur[8] + 10 * g;
    c.globalExposure = cur[9];
    const ip = scene.imageProcessingConfiguration;
    ip.vignetteColor.set(shade.r, shade.g, shade.b, 0);

    // Névoa: o brilho da hora com o matiz da sombra da região.
    const fog = this.fogOut;
    Color3.LerpToRef(t.fog, tintKeepingValue(t.fog, shade, cur[12], this.tmp), cur[11], fog);
    scene.fogColor = fog;
    scene.fogDensity = t.fogDensity * cur[10];
    scene.clearColor.set(fog.r, fog.g, fog.b, 1);
    // Luz de baixo (o que pinta as sombras) puxada para a sombra da região; o céu, de leve.
    Color3.LerpToRef(t.hemiGround, tintKeepingValue(t.hemiGround, shade, 1.1, this.tmp), 0.5, this.groundOut);
    hemi.groundColor = this.groundOut;
    Color3.LerpToRef(t.hemiSky, tintKeepingValue(t.hemiSky, shade, 1, this.tmp), 0.12, this.skyOut);
    hemi.diffuse = this.skyOut;

    // Material de paleta: sombra da região e borda na cor da luz com o destaque.
    Color3.LerpToRef(t.light, accent, 0.35, this.rim);
    setPaletteColors(shade, this.rim);
  }

  /** Vetor do visual da região no ponto (o covil do raid vence o Vale, onde ele fica). */
  private lookAt(x: number, z: number): Float32Array {
    const lair = this.regions.find((r) => r.name === RAID_LAIR.name && r.area && Math.hypot(x - r.area.x, z - r.area.z) < r.area.r);
    const name = lair ? lair.name : regionAt(this.regions, x, z).name;
    return LOOK_VECTORS.get(name) ?? LOOK_VECTORS.get(LOOK_ALIASES[name] ?? DEFAULT_LOOK)!;
  }
}
