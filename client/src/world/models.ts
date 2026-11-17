import "@babylonjs/loaders/glTF";
import {
  AbstractMesh,
  Animation,
  AnimationGroup,
  AssetContainer,
  Color3,
  Effect,
  LoadAssetContainerAsync,
  Material,
  Mesh,
  Node,
  Observer,
  PBRMaterial,
  ProceduralTexture,
  RawTexture,
  StandardMaterial,
  Texture,
  Quaternion,
  Scene,
  TransformNode,
  Vector3,
} from "@babylonjs/core";
import { reportClientError } from "../net/errors";
import { createGolem, GOLEM_MODEL } from "./golem";
import { setPaletteKind } from "./palette-material";
import { lodFor, REMASTER, REMASTER_FILES, REMASTER_MASKS } from "./remaster";
import { bodyPalettes, colorKey, outfitShade, needsRepaint, normalLook, ORIGINAL_LOOK, paintTexture, type BodyLook, type BodyPalettes } from "./body-look";

/**
 * Personagens e monstros KayKit (CC0). Todos usam o mesmo esqueleto ("Rig_Medium"),
 * então as animações vêm de arquivos separados e são reaplicadas pelo nome dos ossos.
 */

const BASE = "/models/";

/** Shader da repintura do remaster (ver tintMaterial). Cores em gama; `outfit` e `skin` já em 0-1. */
const TINT_SHADER = "remasterTint";
const TINT_FRAGMENT = `
precision highp float;
varying vec2 vUV;
uniform sampler2D baseSampler;
uniform sampler2D maskSampler;
uniform vec3 outfit;
uniform vec3 skin;
uniform vec3 hair;
uniform float hasOutfit;
uniform float hasSkin;
uniform float hasHair;
uniform float baseIsLinear;
void main(void) {
  vec3 c = texture2D(baseSampler, vUV).rgb;
  if (baseIsLinear > 0.5) c = pow(c, vec3(1.0 / 2.2));
  vec4 m = texture2D(maskSampler, vUV);
  float luma = dot(c, vec3(0.299, 0.587, 0.114));
  c = mix(c, outfit * min(1.6, luma * 3.0), m.r * hasOutfit);
  c = mix(c, skin * clamp(luma / max(m.a, 0.05), 0.6, 1.25), m.g * hasSkin);
  c = mix(c, hair * clamp(luma * 2.2, 0.25, 1.4), m.b * hasHair);
  gl_FragColor = vec4(c, 1.0);
}`;
/** Altura desejada do personagem em metros (o modelo é escalado para isso). */
const TARGET_HEIGHT = 1.8;

const ANIM_FILES = [
  "anims-general.glb",
  "anims-movementbasic.glb",
  "anims-combatmelee.glb",
  "anims-combatranged.glb",
  "anims-special.glb",
];

/** Emotes (/acenar, /sentar, /dancar): só os heróis têm. */
export type EmoteRole = "wave" | "sit" | "dance";
/** Papéis que só alguns modelos têm: os emotes e o andar (o remaster; os outros só correm). */
export type OptionalRole = EmoteRole | "walk" | "break1" | "break2" | "break3";
export type AnimRole = "idle" | "run" | "attack" | "skill" | "buff" | "hit" | "death" | OptionalRole;
type AnimSet = Record<Exclude<AnimRole, OptionalRole>, string> & Partial<Record<OptionalRole, string>>;

const HERO_BASE = { idle: "Idle_A", run: "Running_A", buff: "Ranged_Magic_Raise", hit: "Hit_A", death: "Death_A" };
const SKELETON_BASE = {
  idle: "Skeletons_Idle",
  run: "Skeletons_Walking",
  buff: "Ranged_Magic_Raise",
  hit: "Hit_A",
  death: "Skeletons_Death",
};

/** Pose sentada montada na carga a partir de Idle_A e Death_A (ver buildSitPose). */
const SIT_POSE = "Sit_Pose";
/**
 * Os arquivos não têm clipes de emote: acenar é o braço erguido da magia, dançar é a
 * conjuração em loop e sentar é uma pose montada com clipes que existem.
 */
const EMOTES = { wave: "Ranged_Magic_Raise", dance: "Ranged_Magic_Spellcasting", sit: SIT_POSE };

/** Quais animações cada modelo usa em cada papel. */
const ANIM_SETS: Record<string, AnimSet> = {
  knight: { ...HERO_BASE, ...EMOTES, attack: "Melee_1H_Attack_Chop", skill: "Melee_1H_Attack_Stab" },
  barbarian: { ...HERO_BASE, ...EMOTES, attack: "Melee_1H_Attack_Chop", skill: "Melee_2H_Attack_Chop" },
  mage: { ...HERO_BASE, ...EMOTES, attack: "Ranged_Magic_Shoot", skill: "Ranged_Magic_Spellcasting" },
  rogue: { ...HERO_BASE, ...EMOTES, attack: "Melee_Dualwield_Attack_Stab", skill: "Melee_1H_Attack_Stab" },
  skeleton_minion: { ...SKELETON_BASE, attack: "Melee_1H_Attack_Chop", skill: "Melee_1H_Attack_Chop" },
  skeleton_warrior: { ...SKELETON_BASE, attack: "Melee_1H_Attack_Chop", skill: "Melee_2H_Attack_Chop" },
  skeleton_rogue: { ...SKELETON_BASE, attack: "Melee_Dualwield_Attack_Stab", skill: "Melee_1H_Attack_Stab" },
  skeleton_mage: { ...SKELETON_BASE, attack: "Ranged_Magic_Shoot", skill: "Ranged_Magic_Spellcasting" },
  npc_merchant: { ...HERO_BASE, attack: "Melee_1H_Attack_Chop", skill: "Ranged_Magic_Raise" },
  npc_smith: { ...HERO_BASE, attack: "Melee_1H_Attack_Chop", skill: "Melee_1H_Attack_Chop" },
};

/**
 * Instante (s) em que o golpe encosta (ou a magia sai da mão) em cada clipe, medido pela
 * posição das mãos nos arquivos de animação: o fim do movimento rápido do golpe.
 */
const CLIP_IMPACT: Record<string, number> = {
  Melee_1H_Attack_Chop: 0.62,
  Melee_1H_Attack_Stab: 0.43,
  Melee_2H_Attack_Chop: 0.85,
  Melee_Dualwield_Attack_Stab: 0.43,
  Ranged_Magic_Shoot: 0.27,
  Ranged_Magic_Spellcasting: 0.4,
  Ranged_Magic_Raise: 0.45,
};

/**
 * Animações dos modelos do remaster v3 (clipes do próprio arquivo; KB_ cavaleiro, SW_ esqueleto). São ações
 * da biblioteca do Meshy (ver docs/arte/remaster-v3.md): parado, andar, correr, golpe de espada, dano
 * (Hit Reaction 1), morte (Dead) e o grito de espada (Sword Shout) para buff/acenar/dançar.
 */
/** Conjunto de clipes de um herói do remaster: prefixo do arquivo + o clipe de cada papel (docs/arte/remaster-herois.md). */
function heroSet(p: string, over: Partial<Record<AnimRole, string>> = {}): AnimSet {
  return {
    idle: `${p}Idle`, walk: `${p}Walk`, run: `${p}Run`, attack: `${p}Attack`, skill: `${p}Attack`, buff: `${p}Raise`, hit: `${p}Hit`, death: `${p}Death`,
    wave: `${p}Wave`, dance: `${p}Dance`, sit: `${p}Sit`, break1: `${p}Break1`, break2: `${p}Break2`, ...over,
  };
}
const REMASTER_SETS: Record<string, AnimSet> = {
  knight: heroSet("KB_", { skill: "KB_Skill" }),
  barbarian: heroSet("BB_", { skill: "BB_Skill" }),
  mage: heroSet("MB_", { skill: "MB_Skill", buff: "MB_Skill" }),
  rogue: heroSet("RB_", { skill: "RB_Skill" }),
  skeleton_warrior: { idle: "SW_Idle", walk: "SW_Walk", run: "SW_Run", attack: "SW_Attack", skill: "SW_Attack", buff: "SW_Raise", hit: "SW_Hit", death: "SW_Death" },
};
/** O golpe encosta quando a mão termina o movimento rápido (quadro 16 de 30, medido na velocidade da mão). */
CLIP_IMPACT.SW_Attack = 16 / 30;
CLIP_IMPACT.KB_Skill = 16 / 30;
// Heróis novos: o fim do movimento rápido da mão, medido no Blender (tools/art/remaster-herois/impact.py).
CLIP_IMPACT.BB_Attack = 34 / 30;
CLIP_IMPACT.KB_Attack = 33 / 30; // corte em arco (Charged Slash); a estocada de antes virou KB_Skill
CLIP_IMPACT.BB_Skill = 41 / 30;
CLIP_IMPACT.MB_Attack = 18 / 30;
CLIP_IMPACT.MB_Skill = 17 / 30;
CLIP_IMPACT.RB_Attack = 20 / 30;
CLIP_IMPACT.RB_Skill = 22 / 30;

export type Slot = "handslot.r" | "handslot.l";

/**
 * Pega de cada arma nos heróis do remaster, por arquivo: rotação (graus, XYZ) e deslocamento (m na escala
 * da arma, ou seja, 0,4 m reais por unidade), os dois no eixo do osso da mão: X lateral, Y ao longo do braço
 * (para os dedos, com o braço pendurado), Z para a frente. Os encaixes
 * `handslot.*` dos GLB deixam a lâmina (Y da arma) apontando para a frente da mão; aqui cada tipo ganha a
 * sua pega: espada para cima e para a frente, escudo preso ao antebraço, cajado em pé. Calibrado no
 * laboratório (client/lab.html). Os modelos KayKit (sem ?remaster=1) não usam esta tabela.
 */
export interface Grip {
  rot: [number, number, number];
  pos: [number, number, number];
  /** Escala extra da arma (o cajado dos heróis é maior que eles, pelo guia de arte). */
  scale?: number;
  /** Mão em que os números foram calibrados (padrão: direita). Na outra mão a pega é espelhada. */
  hand?: "r" | "l";
  /** Pega calibrada na outra mão (os ossos das duas mãos do Meshy não são espelhos exatos); sem ela, espelha. */
  other?: { rot: [number, number, number]; pos: [number, number, number] };
}
/** Inverso da rotação dos encaixes (0,5 0,5 0,5 0,5): com ele a arma fica no eixo do osso da mão (X lateral, Y ao longo do braço, Z para a frente). */
const SLOT_INV = new Quaternion(-0.5, -0.5, -0.5, 0.5);
const BLADE: Grip = { rot: [125, 0, 20], pos: [0, 0, 0], scale: 1.2 };
const SHIELD: Grip = { rot: [0, 20, 0], pos: [-0.02, -0.45, 0.12], hand: "l" };
export const WEAPON_GRIPS: Record<string, Grip> = {
  sword_1handed: BLADE, axe_1handed: BLADE, wand: BLADE,
  // adaga: o cabo na palma (a mão do rig fica aberta, sem dedos); a esquerda tem pega própria
  dagger: { rot: [125, 0, 20], pos: [0, 0.2, 0], other: { rot: [90, 0, 20], pos: [0, 0.2, 0] } },
  // Duas mãos: maiores que o normal (guia de arte, seção 2), na mão direita com a lâmina sobre o ombro.
  sword_2handed: { ...BLADE, scale: 1.3 }, sword_2handed_color: { ...BLADE, scale: 1.3 }, axe_2handed: { ...BLADE, scale: 1.3 },
  staff: { rot: [135, 0, 5], pos: [0, 0.35, 0], scale: 1.4 },
  shield_badge_color: SHIELD, shield_round_barbarian: SHIELD, shield_square: SHIELD, shield_spikes: SHIELD,
  shield_spikes_color: SHIELD,
};
/** Armas padrão de cada modelo. Os heróis usam o que o servidor manda (equipamento). */
const WEAPONS: Record<string, Partial<Record<Slot, string>>> = {
  knight: {},
  barbarian: {},
  mage: {},
  rogue: {},
  npc_merchant: {},
  npc_smith: { "handslot.r": "axe_1handed" },
};
/** Todas as armas que os itens podem mostrar na mão. */
const ITEM_WEAPONS = [
  "sword_1handed", "sword_2handed", "sword_2handed_color", "axe_1handed", "axe_2handed", "staff", "wand", "dagger",
  "shield_badge_color", "shield_round_barbarian", "shield_square", "shield_spikes", "shield_spikes_color",
  // Opções da criação de personagem (mesmos arquivos das armas dos esqueletos).
  "skeleton_blade", "skeleton_axe", "skeleton_staff", "skeleton_shield_large_a",
];
/** Brilho das armas raras (3) e épicas (4). */
const GRADE_GLOW: Record<number, Color3> = { 3: new Color3(0.15, 0.35, 0.9), 4: new Color3(0.6, 0.2, 0.95), 5: new Color3(1, 0.5, 0.1) };
/** Encantamento: +4 a +6 brilha dourado; +7 ou mais, dourado forte e pulsando. */
const ENCHANT_GLOW = { light: new Color3(0.45, 0.33, 0.08), strong: new Color3(1, 0.72, 0.22) };

/** Qual brilho a arma usa: o do encantamento vence o da raridade. */
function glowKey(grade: number, enchant: number): string | null {
  if (enchant >= 7) return "e2";
  if (enchant >= 4) return "e1";
  return GRADE_GLOW[grade] ? `g${grade}` : null;
}

/** Modelos de monstro (arquivos opcionais: se faltarem, o jogo usa cápsulas). */
const MONSTER_MODELS = ["skeleton_minion", "skeleton_warrior", "skeleton_rogue", "skeleton_mage"];
const MONSTER_WEAPONS: Record<string, Partial<Record<Slot, string>>> = {
  skeleton_minion: { "handslot.r": "skeleton_blade" },
  skeleton_warrior: { "handslot.r": "skeleton_axe", "handslot.l": "skeleton_shield_large_a" },
  skeleton_rogue: { "handslot.r": "skeleton_blade", "handslot.l": "skeleton_blade" },
  skeleton_mage: { "handslot.r": "skeleton_staff" },
};

export interface CharacterInstance {
  root: TransformNode;
  anims: Map<AnimRole, AnimationGroup>;
  /** Instante (s) do golpe/soltura de cada ação no próprio clipe (sincroniza dano e efeitos). */
  impacts: Partial<Record<AnimRole, number>>;
  /**
   * Devolve os ossos à pose de descanso. As animações de ação (algumas feitas para os
   * heróis) movem canais que a pose parada dos esqueletos não anima — como a posição do
   * quadril —, e sem isso o corpo ficava enterrado no chão depois de um golpe.
   */
  resetPose(): void;
  /** Troca o que está nas mãos (null/"" = mão vazia). grade 3–4 e encantamento +4 fazem a arma brilhar. */
  setWeapons(right: string | null, left: string | null, grade?: number, enchant?: number): void;
  /** Troca o visual do corpo (pele, cabelo, roupa, rosto e chapéu). */
  setLook(look: BodyLook): void;
  /** Malhas atuais (corpo + armas), para sombra e visibilidade. */
  meshes(): AbstractMesh[];
  dispose(): void;
}

interface ModelAssets {
  body: AssetContainer;
  weapons: Partial<Record<Slot, AssetContainer>>;
  scale: number;
  /** Cabeças que o arquivo traz ("Head_<id>"), por id: só a escolhida é copiada para a cena. */
  heads: Map<string, AbstractMesh>;
  /** Modelo do estilo B (?remaster=1): clipes e LODs no próprio arquivo. */
  remaster: boolean;
  /** Máscara de repintura do remaster (R pano, G pele): a base é KTX2 e não dá para ler os pixels. */
  mask: Texture | null;
}

export class CharacterModels {
  private constructor(
    private models: Map<string, ModelAssets>,
    private animTemplates: Map<string, AnimationGroup>,
    private weaponFiles: Map<string, AssetContainer | null>,
    private scene: Scene,
  ) {}

  /** Materiais com brilho, por material original + raridade (compartilhados, nunca descartados). */
  private glowMats = new Map<string, Material>();
  /** Materiais com a textura repintada, por classe + cores; descartados quando ninguém usa. */
  private paintMats = new Map<string, { mat: Promise<Material | null>; refs: number }>();
  /** Cópias do remaster com LOD, reavaliadas a cada 250 ms pela distância da câmera. */
  private lodInstances = new Set<{ root: TransformNode; level: number; set: (l: number) => void }>();
  private lodTick: Observer<Scene> | null = null;
  /** Nome do arquivo de cada arma carregada (para achar a pega em WEAPON_GRIPS). */
  private _wfo: Map<AssetContainer, string> | null = null;
  private get weaponFileOf(): Map<AssetContainer, string> {
    return (this._wfo ??= new Map([...this.weaponFiles].filter(([, c]) => c).map(([f, c]) => [c!, f])));
  }

  private watchLod(e: { root: TransformNode; level: number; set: (l: number) => void }): () => void {
    this.lodInstances.add(e);
    this.lodTick ??= (() => {
      let acc = 0;
      return this.scene.onBeforeRenderObservable.add(() => {
        acc += this.scene.getEngine().getDeltaTime();
        if (acc < 250) return;
        acc = 0;
        const cam = this.scene.activeCamera;
        if (!cam) return;
        for (const i of this.lodInstances) {
          const lv = lodFor(Vector3.DistanceSquared(cam.globalPosition, i.root.getAbsolutePosition()));
          if (lv !== i.level) {
            i.level = lv;
            i.set(lv);
          }
        }
      });
    })();
    return () => this.lodInstances.delete(e);
  }

  /** Pega (criando se preciso) o material repintado; devolva com releasePaint. */
  private acquirePaint(model: string, base: Material, look: BodyLook): Promise<Material | null> {
    const key = colorKey(model, look);
    let e = this.paintMats.get(key);
    if (!e) {
      e = { mat: this.paintMaterial(base, model, look), refs: 0 };
      this.paintMats.set(key, e);
    }
    e.refs++;
    return e.mat;
  }

  private releasePaint(model: string, look: BodyLook): void {
    const key = colorKey(model, look);
    const e = this.paintMats.get(key);
    if (!e || --e.refs > 0) return;
    this.paintMats.delete(key);
    void e.mat.then((m) => {
      if (!m) return;
      const tex = m instanceof PBRMaterial ? m.albedoTexture : m instanceof StandardMaterial ? m.diffuseTexture : null;
      tex?.dispose();
      m.dispose();
    });
  }

  /**
   * Remaster: a textura-base é KTX2 (a GPU lê, a CPU não). O pano e a pele são tingidos por um shader
   * que mistura a base com a máscara pequena (R pano, G pele) numa textura nova, desenhada uma vez por
   * combinação de cores. A luz pintada da base é mantida (o brilho do pixel original escala a cor nova).
   */
  private async tintMaterial(base: Material, src: Texture, mask: Texture, model: string, look: BodyLook, pal: BodyPalettes): Promise<Material | null> {
    await new Promise<void>((ok) => (mask.isReady() ? ok() : mask.onLoadObservable.addOnce(() => ok())));
    const scene = base.getScene();
    Effect.ShadersStore[`${TINT_SHADER}PixelShader`] ??= TINT_FRAGMENT;
    const { width } = src.getSize();
    const tex = new ProceduralTexture(`${src.name}-${colorKey(model, look)}`, width || 1024, TINT_SHADER, scene, null, true, false);
    const mid = (s: { top: string; bottom: string } | undefined): Color3 => {
      if (!s) return Color3.White();
      const a = Color3.FromHexString(s.top);
      const b = Color3.FromHexString(s.bottom);
      return new Color3((a.r + b.r) / 2, (a.g + b.g) / 2, (a.b + b.b) / 2);
    };
    const outfit = look.outfit > 0 ? outfitShade(pal, look.outfit) : undefined;
    const skin = look.skin > 0 ? pal.skins[look.skin - 1] : undefined;
    tex.setTexture("baseSampler", src);
    tex.setTexture("maskSampler", mask);
    tex.setColor3("outfit", mid(outfit));
    tex.setColor3("skin", skin ? Color3.FromHexString(skin.top) : Color3.White());
    const hair = look.hair > 0 ? pal.hairs[look.hair - 1] : undefined;
    tex.setColor3("hair", hair ? Color3.FromHexString(hair.top) : Color3.White());
    tex.setFloat("hasHair", hair ? 1 : 0);
    tex.setFloat("hasOutfit", outfit ? 1 : 0);
    tex.setFloat("hasSkin", skin ? 1 : 0);
    // A base em KTX2 sRGB já chega linear ao shader; sem o formato sRGB chega em gama.
    tex.setFloat("baseIsLinear", (src.getInternalTexture() as { _useSRGBBuffer?: boolean } | null)?._useSRGBBuffer ? 1 : 0);
    tex.refreshRate = 0; // desenha uma vez
    tex.gammaSpace = true; // a saída é gama; o PBR converte para linear
    tex.wrapU = src.wrapU;
    tex.wrapV = src.wrapV;
    const mat = base.clone(`${base.name}-${colorKey(model, look)}`);
    if (mat instanceof PBRMaterial) mat.albedoTexture = tex;
    else if (mat instanceof StandardMaterial) mat.diffuseTexture = tex;
    return mat;
  }

  /** Cópia do material com as faixas de pele, cabelo e roupa repintadas. */
  private async paintMaterial(base: Material, model: string, look: BodyLook): Promise<Material | null> {
    const pal = await bodyPalettes();
    const src = base instanceof PBRMaterial ? base.albedoTexture : base instanceof StandardMaterial ? base.diffuseTexture : null;
    if (!pal || !(src instanceof Texture)) return null;
    await new Promise<void>((ok) => (src.isReady() ? ok() : src.onLoadObservable.addOnce(() => ok())));
    const mask = this.models.get(model)?.mask;
    if (REMASTER && mask) return this.tintMaterial(base, src, mask, model, look, pal);
    const { width, height } = src.getSize();
    const pixels = await src.readPixels()?.catch(() => null);
    if (!pixels || !width) return null;
    const data = new Uint8Array(pixels.buffer, pixels.byteOffset, width * height * 4).slice();
    paintTexture(data, width, height, model, look, pal);
    const scene = base.getScene();
    const tex = RawTexture.CreateRGBATexture(data, width, height, scene, true, src.invertY, Texture.TRILINEAR_SAMPLINGMODE);
    tex.name = `${src.name}-${colorKey(model, look)}`;
    tex.gammaSpace = src.gammaSpace;
    tex.wrapU = src.wrapU;
    tex.wrapV = src.wrapV;
    const mat = base.clone(`${base.name}-${colorKey(model, look)}`);
    if (mat instanceof PBRMaterial) mat.albedoTexture = tex;
    else if (mat instanceof StandardMaterial) mat.diffuseTexture = tex;
    return mat;
  }

  /** Baixa modelos, armas e animações em paralelo. Arquivos que faltarem são ignorados. */
  static async load(scene: Scene): Promise<CharacterModels> {
    // Toda falha de carga vai ao relatório de erros; as opcionais viram null (cápsula/sem arma).
    const load = (file: string) =>
      LoadAssetContainerAsync(BASE + file, scene).catch((err: unknown) => {
        reportClientError("model", `modelo não carregou: ${file}`, { file, error: String((err as Error)?.message ?? err) }, err);
        throw err;
      });
    const tryLoad = (file: string) => load(file).catch(() => null);

    const allWeapons = { ...WEAPONS, ...MONSTER_WEAPONS };
    const weaponFiles = [...new Set([...Object.values(allWeapons).flatMap((w) => Object.values(w)), ...ITEM_WEAPONS])];
    const modelIds = [...Object.keys(WEAPONS), ...MONSTER_MODELS];

    const [animContainers, weaponList, bodies] = await Promise.all([
      Promise.all(ANIM_FILES.map(load)),
      Promise.all(weaponFiles.map((w) => tryLoad(`weapons/${w}.glb`).then((c) => [w, c] as const))),
      Promise.all(
        modelIds.map(async (id) => {
          // ?remaster=1: o arquivo do estilo B, se existir; senão, o KayKit de sempre.
          const rm = REMASTER && REMASTER_FILES[id] ? await tryLoad(`${REMASTER_FILES[id]}.glb`) : null;
          return [id, rm ?? (await tryLoad(`${id}.glb`)), !!rm] as const;
        }),
      ),
    ]);
    const weaponByFile = new Map(weaponList);
    // Material de paleta: personagens, monstros e armas com a borda e o degradê mais fortes
    // que o cenário. As cópias (repintura, brilho) herdam o tipo.
    for (const c of [...bodies.map(([, b]) => b), ...weaponList.map(([, w]) => w)]) {
      for (const m of c?.materials ?? []) setPaletteKind(m, "char");
    }

    const models = new Map<string, ModelAssets>();
    for (const [id, body, remaster] of bodies) {
      if (!body) continue;
      const weapons: ModelAssets["weapons"] = {};
      for (const [slot, file] of Object.entries(allWeapons[id] ?? {}) as [Slot, string][]) {
        const w = weaponByFile.get(file);
        if (w) weapons[slot] = w;
      }
      const heads = new Map<string, AbstractMesh>();
      for (const m of body.meshes) if (m.name.startsWith("Head_")) heads.set(m.name.slice(5), m);
      const maskFile = remaster ? REMASTER_MASKS[id] : undefined;
      const mask = maskFile ? new Texture(`${BASE}${maskFile}`, scene, true, false) : null;
      models.set(id, { body, weapons, scale: measureScale(body, id), heads, remaster, mask });
    }

    // Os grupos de animação servem de molde; não tocam sozinhos.
    const templates = new Map<string, AnimationGroup>();
    for (const c of [...animContainers, ...bodies.filter(([, b, rm]) => b && rm).map(([, b]) => b!)]) {
      for (const g of c.animationGroups) {
        g.stop();
        templates.set(g.name, g);
      }
    }
    const idle = templates.get("Idle_A");
    const death = templates.get("Death_A");
    if (idle && death) templates.set(SIT_POSE, buildSitPose(idle, death, scene));
    return new CharacterModels(models, templates, weaponByFile, scene);
  }

  /** Cópia do material com brilho (uma por material + tipo de brilho; "e2" pulsa). */
  private glowMaterial(base: Material, glow: string): Material {
    const key = `${base.uniqueId}:${glow}`;
    let mat = this.glowMats.get(key);
    if (!mat) {
      mat = base.clone(`${base.name}-${glow}`) ?? base;
      const c =
        glow === "e2" ? ENCHANT_GLOW.strong.clone() : glow === "e1" ? ENCHANT_GLOW.light : GRADE_GLOW[Number(glow.slice(1))];
      const target = mat instanceof PBRMaterial || mat instanceof StandardMaterial ? mat : null;
      if (target) target.emissiveColor = c;
      if (glow === "e2" && target) {
        // Um pulso por material (todas as armas +7 pulsam juntas).
        const scene = mat.getScene();
        scene.onBeforeRenderObservable.add(() => {
          const k = 0.55 + 0.45 * Math.sin(performance.now() / 260);
          target.emissiveColor.copyFromFloats(ENCHANT_GLOW.strong.r * k, ENCHANT_GLOW.strong.g * k, ENCHANT_GLOW.strong.b * k);
        });
      }
      this.glowMats.set(key, mat);
    }
    return mat;
  }

  has(model: string): boolean {
    return model === GOLEM_MODEL || this.models.has(model);
  }

  /** Arquivos de arma carregados (podem ser usados em `weapons` do instantiate). */
  weaponNames(): string[] {
    return [...this.weaponFiles].filter(([, c]) => c).map(([w]) => w);
  }

  /**
   * Cria uma cópia animável do modelo (ou null se ele não existir).
   * `weapons` troca o que vai em cada mão (null = mão vazia); sem ele, usa as armas padrão.
   */
  instantiate(model: string, id: string, weapons?: Partial<Record<Slot, string | null>>, look?: BodyLook): CharacterInstance | null {
    // Golem: modelado e animado em código (sem arquivo glTF).
    if (model === GOLEM_MODEL) return createGolem(this.scene, id);
    const assets = this.models.get(model);
    const set = assets?.remaster ? REMASTER_SETS[model] : ANIM_SETS[model];
    if (!assets || !set) return null;
    let handItems = assets.weapons;
    if (weapons) {
      handItems = { ...assets.weapons };
      for (const [slot, file] of Object.entries(weapons) as [Slot, string | null][]) {
        const c = file ? this.weaponFiles.get(file) : null;
        if (c) handItems[slot] = c;
        else delete handItems[slot];
      }
    }

    // Cabeça: o arquivo dos heróis traz cinco, mas só a escolhida vai para a cena (cada
    // malha a mais custa na escolha das malhas ativas a cada quadro, mesmo desligada).
    const headOf = (l: BodyLook) => {
      const h = normalLook(model, l).head;
      return assets.heads.has(h) ? h : model;
    };
    const firstHead = headOf(look ?? ORIGINAL_LOOK);
    const srcHead = assets.heads.get(firstHead);
    // Malhas com esqueleto precisam ser clonadas (não instanciadas). Os nós "Head_*" sem
    // malha (o glTF cria um por malha com pele, vazio) também ficam de fora.
    const entries = assets.body.instantiateModelsToScene((n) => `${n}#${id}`, false, {
      doNotInstantiate: true,
      predicate: (e: unknown) => !(e instanceof TransformNode && e.name.startsWith("Head_") && e !== srcHead),
    });
    // Os clipes do remaster vêm no próprio arquivo; os da cópia não servem (tocamos clones dos moldes).
    if (assets.remaster) for (const g of entries.animationGroups) g.dispose();
    const gltfRoot = asNode(entries.rootNodes[0] as TransformNode);

    const root = new TransformNode(`model#${id}`, gltfRoot.getScene());
    gltfRoot.parent = root;
    root.scaling.setAll(assets.scale);

    const byName = new Map<string, Node>();
    for (const n of gltfRoot.getDescendants(false)) byName.set(n.name.split("#")[0], n);

    // Armas presas nos encaixes das mãos (trocadas quando o equipamento muda).
    const held = new Map<Slot, TransformNode[]>();
    const attach = (slot: Slot, container: AssetContainer | null | undefined, grade: number, enchant = 0) => {
      for (const n of held.get(slot) ?? []) n.dispose(false, false);
      held.delete(slot);
      const hand = byName.get(slot);
      if (!hand || !container) return;
      const w = container.instantiateModelsToScene((n) => `${n}#${id}`, false);
      const wRoot = w.rootNodes[0] as TransformNode;
      // O __root__ do glTF converte o sistema de coordenadas; dentro do esqueleto isso já foi
      // feito, então prendemos o filho direto e descartamos o __root__ da arma.
      const nodes: TransformNode[] = [];
      for (const child of wRoot.getChildren() as TransformNode[]) {
        child.parent = hand as TransformNode;
        child.position = Vector3.Zero();
        const grip = assets.remaster ? WEAPON_GRIPS[this.weaponFileOf.get(container) ?? ""] : undefined;
        if (grip) {
          const d = Math.PI / 180;
          // Rotação e deslocamento no eixo do osso da mão (o encaixe é desfeito antes).
          const swapped = (grip.hand ?? "r") !== (slot === "handslot.l" ? "l" : "r");
          const mirror = swapped && !grip.other ? -1 : 1;
          const use = swapped && grip.other ? grip.other : grip;
          const q = Quaternion.FromEulerAngles(use.rot[0] * d, use.rot[1] * d * mirror, use.rot[2] * d * mirror);
          child.rotationQuaternion = SLOT_INV.multiply(q).multiply(child.rotationQuaternion ?? Quaternion.Identity());
          new Vector3(use.pos[0] * mirror, use.pos[1], use.pos[2]).rotateByQuaternionToRef(SLOT_INV, child.position);
          if (grip.scale) child.scaling.scaleInPlace(grip.scale);
        }
        nodes.push(child);
        const glow = glowKey(grade, enchant);
        if (glow) {
          for (const m of child.getChildMeshes(false)) if (m.material) m.material = this.glowMaterial(m.material, glow);
          if (child instanceof AbstractMesh && child.material) child.material = this.glowMaterial(child.material, glow);
        }
      }
      wRoot.dispose();
      held.set(slot, nodes);
    };
    for (const [slot, container] of Object.entries(handItems) as [Slot, AssetContainer][]) attach(slot, container, 0);

    // Visual do corpo: só os heróis têm várias cabeças e o chapéu separado.
    const hat = byName.get("Hat");
    // Só as malhas com esqueleto (as armas presas nas mãos não entram).
    const bodyMeshes = gltfRoot.getChildMeshes(false).filter((m) => !!m.skeleton && !!m.material);
    // LOD (remaster): nós com sufixo _LOD1/_LOD2; o elmo ("Hat", "Hat_LOD1"...) some com hideHat.
    let lodLevel = 0;
    let hatHidden = false;
    const lodNodes = assets.remaster
      ? bodyMeshes.map((m) => {
          const base = m.name.split("#")[0].replace(/\.\d+$/, "");
          return { m, lod: Number(/_LOD(\d)$/.exec(base)?.[1] ?? 0), hat: /^Hat(_LOD\d)?$/.test(base) };
        })
      : [];
    const refreshVis = () => {
      for (const n of lodNodes) n.m.setEnabled(n.lod === lodLevel && !(n.hat && hatHidden));
    };
    refreshVis();
    const unwatchLod = assets.remaster
      ? this.watchLod({ root, level: 0, set: (l) => ((lodLevel = l), refreshVis()) })
      : () => {};
    let head = { id: firstHead, mesh: bodyMeshes.find((m) => m.name.split("#")[0] === `Head_${firstHead}`) };
    // O elmo separável (remaster) tem material próprio (aço liso, cor de vértice): a repintura não o toca.
    const isHatMesh = (m: AbstractMesh) => /^Hat(_LODd)?(.d+)?$/.test(m.name.split("#")[0]);
    const paintable = () => bodyMeshes.filter((m) => !isHatMesh(m));
    const baseMat = bodyMeshes.find((m) => m !== head.mesh && !isHatMesh(m))?.material ?? bodyMeshes[0]?.material ?? null;
    let painted: BodyLook | null = null; // cores em uso (material repintado)
    let disposed = false;
    /** Troca a cabeça (Barbeiro, criação): a nova é copiada do arquivo e a velha, descartada. */
    const swapHead = (want: string) => {
      const src = assets.heads.get(want);
      const old = head.mesh;
      if (want === head.id || !(src instanceof Mesh) || !old) return;
      const mesh = src.clone(`${src.name}#${id}`, old.parent, true);
      mesh.skeleton = old.skeleton;
      mesh.material = old.material;
      mesh.isPickable = old.isPickable;
      mesh.receiveShadows = old.receiveShadows;
      const i = bodyMeshes.indexOf(old);
      if (i >= 0) bodyMeshes[i] = mesh;
      old.dispose(false, false);
      head = { id: want, mesh };
    };
    const applyLook = (l: BodyLook) => {
      if (head.mesh) {
        swapHead(headOf(l));
        head.mesh!.setEnabled(true);
      }
      l = normalLook(model, l);
      if (assets.remaster) {
        hatHidden = l.hideHat;
        refreshVis();
      } else if (hat instanceof TransformNode) hat.setEnabled(!l.hideHat);
      if (!baseMat || (painted && colorKey(model, painted) === colorKey(model, l))) return;
      if (painted) this.releasePaint(model, painted);
      painted = null;
      for (const m of paintable()) m.material = baseMat;
      if (!needsRepaint(l)) return;
      painted = l;
      void this.acquirePaint(model, baseMat, l).then((mat) => {
        if (!mat || disposed || painted !== l) return;
        for (const m of paintable()) m.material = mat;
      });
    };
    if (look) applyLook(look);
    else if (head.mesh) applyLook(ORIGINAL_LOOK);

    // Animações (só as do conjunto do modelo), reaplicadas pelo nome dos ossos.
    const anims = new Map<AnimRole, AnimationGroup>();
    const cloned = new Map<string, AnimationGroup>();
    for (const [role, animName] of Object.entries(set) as [AnimRole, string][]) {
      let g = cloned.get(animName);
      if (!g) {
        const tpl = this.animTemplates.get(animName);
        if (!tpl) continue;
        g = tpl.clone(`${animName}#${id}`, (target: Node) => byName.get(target.name) ?? null);
        g.stop();
        cloned.set(animName, g);
      }
      anims.set(role, g);
    }

    // Pose de descanso de tudo que alguma animação mexe (antes de qualquer animação tocar).
    const rest: { node: TransformNode; pos: Vector3; rot: Quaternion | null; scale: Vector3 }[] = [];
    const seen = new Set<TransformNode>();
    for (const g of cloned.values()) {
      for (const ta of g.targetedAnimations) {
        const node = ta.target as TransformNode;
        if (!(node instanceof TransformNode) || seen.has(node)) continue;
        seen.add(node);
        rest.push({ node, pos: node.position.clone(), rot: node.rotationQuaternion?.clone() ?? null, scale: node.scaling.clone() });
      }
    }

    const impacts: Partial<Record<AnimRole, number>> = {};
    for (const [role, animName] of Object.entries(set) as [AnimRole, string][]) {
      if (CLIP_IMPACT[animName] !== undefined) impacts[role] = CLIP_IMPACT[animName];
    }

    return {
      root,
      anims,
      impacts,
      resetPose: () => {
        for (const r of rest) {
          r.node.position.copyFrom(r.pos);
          if (r.rot && r.node.rotationQuaternion) r.node.rotationQuaternion.copyFrom(r.rot);
          r.node.scaling.copyFrom(r.scale);
        }
      },
      setWeapons: (right, left, grade = 0, enchant = 0) => {
        attach("handslot.r", right ? this.weaponFiles.get(right) : null, grade, enchant);
        attach("handslot.l", left ? this.weaponFiles.get(left) : null, grade, enchant);
      },
      setLook: (l) => {
        if (!disposed) applyLook(l);
      },
      meshes: () => root.getChildMeshes(false),
      dispose: () => {
        disposed = true;
        unwatchLod();
        if (painted) this.releasePaint(model, painted);
        painted = null;
        for (const a of cloned.values()) a.dispose();
        for (const s of entries.skeletons) s.dispose();
        // Nunca descartar materiais/texturas aqui: são compartilhados por todas as cópias
        // do modelo (descartá-los deixava os outros personagens brancos).
        root.dispose(false, false);
      },
    };
  }
}

/**
 * Pose "sentado no chão" para o /sentar, montada uma vez a partir de dois clipes que
 * existem: o quadril fica em pé (como no Idle_A), mas na altura do fim do Death_A (no
 * chão); coxas, canelas e pés ficam com a orientação que têm no fim do Death_A (deitado de
 * costas, joelhos para cima). Com o tronco em pé, isso vira sentado com os joelhos
 * dobrados e os pés no chão. O resto do corpo é o Idle_A parado.
 */
function buildSitPose(idle: AnimationGroup, death: AnimationGroup, scene: Scene): AnimationGroup {
  const key = (target: unknown, prop: string) => `${(target as Node).name}|${prop}`;
  const deathEnd = new Map<string, Vector3 | Quaternion>();
  for (const ta of death.targetedAnimations) {
    deathEnd.set(key(ta.target, ta.animation.targetProperty), ta.animation.evaluate(ta.animation.getHighestFrame()));
  }
  const first = (a: Animation) => a.evaluate(a.getKeys()[0]?.frame ?? 0);
  const hipsIdle = idle.targetedAnimations.find((ta) => (ta.target as Node).name === "hips" && ta.animation.targetProperty === "rotationQuaternion");
  const hipsRest = hipsIdle ? (first(hipsIdle.animation) as Quaternion) : null;
  const hipsDown = deathEnd.get("hips|rotationQuaternion") as Quaternion | undefined;
  const hipsLow = deathEnd.get("hips|position") as Vector3 | undefined;

  const g = new AnimationGroup(SIT_POSE, scene);
  for (const ta of idle.targetedAnimations) {
    const name = (ta.target as Node).name;
    const prop = ta.animation.targetProperty;
    let value = first(ta.animation) as Vector3 | Quaternion;
    const down = deathEnd.get(key(ta.target, prop));
    if (name === "hips" && prop === "position" && hipsLow) {
      value = new Vector3((value as Vector3).x, hipsLow.y, (value as Vector3).z);
    } else if (prop === "rotationQuaternion" && down && /^(upperleg|lowerleg|foot|toes)\./.test(name)) {
      // A coxa é filha do quadril: mantém no mundo a orientação do Death_A com o quadril em pé.
      value = name.startsWith("upperleg.") && hipsRest && hipsDown ? Quaternion.Inverse(hipsRest).multiply(hipsDown).multiply(down as Quaternion) : down;
    }
    const anim = new Animation(`${SIT_POSE}.${name}.${prop}`, prop, 60, ta.animation.dataType, Animation.ANIMATIONLOOPMODE_CYCLE);
    anim.setKeys([
      { frame: 0, value: value.clone() },
      { frame: 60, value: value.clone() },
    ]);
    g.addTargetedAnimation(anim, ta.target);
  }
  g.stop();
  return g;
}

/**
 * O __root__ do glTF é uma malha vazia que só converte o sistema de coordenadas. Como malha
 * ela entra na escolha das malhas ativas a cada quadro (uma por personagem); como nó, não.
 */
function asNode(gltfRoot: TransformNode): TransformNode {
  if (!(gltfRoot instanceof AbstractMesh) || gltfRoot.getTotalVertices() > 0) return gltfRoot;
  const node = new TransformNode(gltfRoot.name, gltfRoot.getScene());
  node.position.copyFrom(gltfRoot.position);
  node.rotation.copyFrom(gltfRoot.rotation);
  node.rotationQuaternion = gltfRoot.rotationQuaternion?.clone() ?? null;
  node.scaling.copyFrom(gltfRoot.scaling);
  node.parent = gltfRoot.parent;
  for (const c of gltfRoot.getChildren()) (c as TransformNode).parent = node;
  gltfRoot.dispose(true);
  return node;
}

/** Escala que deixa o modelo com TARGET_HEIGHT de altura (com a cabeça da própria classe). */
function measureScale(container: AssetContainer, model: string): number {
  let minY = Infinity;
  let maxY = -Infinity;
  for (const m of container.meshes) {
    if (!m.getTotalVertices()) continue;
    if (m.name.startsWith("Head_") && m.name !== `Head_${model}`) continue;
    m.computeWorldMatrix(true);
    const b = m.getBoundingInfo().boundingBox;
    minY = Math.min(minY, b.minimumWorld.y);
    maxY = Math.max(maxY, b.maximumWorld.y);
  }
  const h = maxY - minY;
  return h > 0.1 && Number.isFinite(h) ? TARGET_HEIGHT / h : 1;
}
