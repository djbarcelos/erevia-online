import { HERO_FISTS, type FistFrame } from "./hero-fists";
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
  Matrix,
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
import { RoleMap } from "./anim";
import { createGolem, GOLEM_MODEL } from "./golem";
import { setPaletteKind } from "./palette-material";
import { lodFor, MODEL_FALLBACK, REMASTER, REMASTER_FILES, REMASTER_LAZY, REMASTER_HEIGHTS, REMASTER_MASKS, REMASTER_NO_PREFETCH, REMASTER_ONLY, REMASTER_WEAPONS, WEAPON_FORM_FILES, remasterWeapon } from "./remaster";
import { armorKey, bodyTier, capeFile, helmFile, NO_ARMOR, pauldronFile, TIER_FILES, type ArmorGrades } from "./armor";
import { ARMOR_ATTACH } from "./armor-attach";
import { bodyPalettes, colorKey, outfitShade, needsRepaint, normalLook, ORIGINAL_LOOK, paintTexture, type BodyLook, type BodyPalettes } from "./body-look";

/**
 * Personagens e monstros KayKit (CC0). Todos usam o mesmo esqueleto ("Rig_Medium"),
 * então as animações vêm de arquivos separados e são reaplicadas pelo nome dos ossos.
 */

const BASE = "/models/";
let warmed = false;

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
/** Variantes (heróis do remaster): o golpe alterna attack → attack2 → attack3; dano e morte sorteiam. */
export type VariantRole = "attack2" | "attack3" | "hit2" | "death2";
export type OptionalRole = EmoteRole | "walk" | "break1" | "break2" | "break3" | VariantRole;
export type AnimRole = "idle" | "run" | "attack" | "skill" | "buff" | "hit" | "death" | OptionalRole;
type AnimSet = Record<Exclude<AnimRole, OptionalRole>, string> & Partial<Record<OptionalRole, string>>;

const HERO_BASE = { idle: "Idle_A", run: "Running_A", buff: "Ranged_Magic_Raise", hit: "Hit_A", death: "Death_A" };
const SKELETON_BASE = {
  idle: "Skeletons_Idle",
  // O Skeletons_Walking é um arrastar de pés de 0,4 m/s: a 3,5 m/s o esqueleto deslizava 9 vezes. O correr do KayKit acompanha o chão.
  run: "Running_A",
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
/**
 * Velocidade natural dos clipes de correr e andar de cada modelo, em m/s do mundo com a escala do modelo (o pé de
 * apoio em relação ao quadril; medida no laboratório com tools/art/remaster-herois/foot-speeds.mjs e clip-speeds.mjs).
 * O `AnimController` toca o clipe a `velocidade desenhada / natural`. O KayKit é um boneco chibi: o correr dele
 * é de 2 m/s, e a 5 m/s tocado a 1x os pés deslizavam 2,5 vezes mais rápido que o chão.
 */
const FOOT_SPEEDS_KAYKIT: Record<string, { walk?: number; run: number }> = {
  knight: { run: 1.98 }, barbarian: { run: 2.18 }, mage: { run: 1.9 }, rogue: { run: 2.4 },
  npc_merchant: { run: 2.41 }, npc_smith: { run: 2.3 },
  skeleton_minion: { run: 2.41 }, skeleton_warrior: { run: 1.94 }, skeleton_rogue: { run: 2.27 }, skeleton_mage: { run: 1.91 },
};
const FOOT_SPEEDS_REMASTER: Record<string, { walk?: number; run: number }> = {
  // tunnel#181: andar e correr novos (Meshy, conta atual) medidos com lab.footSpeeds; o andar do bárbaro e do mago
  // continua o do rig (os da biblioteca eram lentos demais, 0,4-0,6 m/s).
  knight: { walk: 0.99, run: 4.18 }, barbarian: { walk: 1.39, run: 4.31 }, mage: { walk: 1.29, run: 3.96 }, rogue: { walk: 1.39, run: 7.66 },
  skeleton_warrior: { walk: 1.52, run: 4.77 }, skeleton_minion: { walk: 0.92, run: 2.42 }, skeleton_rogue: { walk: 1.42, run: 4.45 },
  skeleton_mage: { walk: 1.5, run: 4.91 },
  brejeiro: { walk: 1.4, run: 4.4 }, // o rig mixamo de 1,8 m, como o ladino (tunnel#139)
  // Famílias (tunnel#135): medido no Blender, tools/art/familias-monstro/speeds.py. Aranha e lodo correm com uma cópia do andar (clipe com nome próprio: o Babylon guarda um grupo por nome).
  wolf: { walk: 0.87, run: 1.3 }, spider: { walk: 0.66, run: 0.66 }, slime: { walk: 0.9, run: 0.9 }, frog: { walk: 0.85, run: 1.46 },
  stone_golem: { walk: 0.56, run: 5.63 }, bone_lord: { walk: 1.36, run: 4.15 }, ozrhal: { walk: 1.45, run: 4.43 },
};

/** Conjunto de clipes de um herói do remaster: prefixo do arquivo + o clipe de cada papel (docs/arte/remaster-herois.md). */
function heroSet(p: string, over: Partial<Record<AnimRole, string>> = {}): AnimSet {
  return {
    idle: `${p}Idle`, walk: `${p}Walk`, run: `${p}Run`, attack: `${p}Attack`, skill: `${p}Attack`, buff: `${p}Raise`, hit: `${p}Hit`, death: `${p}Death`,
    wave: `${p}Wave`, dance: `${p}Dance`, sit: `${p}Sit`, break1: `${p}Break1`, break2: `${p}Break2`,
    // variantes (tunnel#181): só entram as que existem no arquivo
    attack2: `${p}Attack2`, attack3: `${p}Attack3`, hit2: `${p}Hit2`, death2: `${p}Death2`, ...over,
  };
}
const REMASTER_SETS: Record<string, AnimSet> = {
  knight: heroSet("KB_", { skill: "KB_Skill" }),
  barbarian: heroSet("BB_", { skill: "BB_Skill" }),
  mage: heroSet("MB_", { skill: "MB_Skill", buff: "MB_Skill" }),
  rogue: heroSet("RB_", { skill: "RB_Skill" }),
  skeleton_warrior: { idle: "SW_Idle", walk: "SW_Walk", run: "SW_Run", attack: "SW_Attack", skill: "SW_Attack", buff: "SW_Raise", hit: "SW_Hit", death: "SW_Death" },
  // Bestiário (tunnel#98): ver docs/arte/remaster-monstros.md para a escolha de cada ação.
  skeleton_minion: { idle: "MN_Idle", walk: "MN_Walk", run: "MN_Run", attack: "MN_Attack", skill: "MN_Attack", buff: "MN_Idle", hit: "MN_Hit", death: "MN_Death" },
  skeleton_rogue: { idle: "RG_Idle", walk: "RG_Walk", run: "RG_Run", attack: "RG_Attack", skill: "RG_Skill", buff: "RG_Idle", hit: "RG_Hit", death: "RG_Death" },
  skeleton_mage: { idle: "MG_Idle", walk: "MG_Walk", run: "MG_Run", attack: "MG_Attack", skill: "MG_Cast", buff: "MG_Cast", hit: "MG_Hit", death: "MG_Death" },
  stone_golem: { idle: "GL_Idle", walk: "GL_Walk", run: "GL_Run", attack: "GL_Attack", skill: "GL_Slam", buff: "GL_Roar", hit: "GL_Hit", death: "GL_Death" },
  // Famílias (tunnel#135): ver docs/arte/familias-monstro.md. Sem rugido nem esmagar nos pacotes: o buff cai no parado.
  wolf: { idle: "WF_Idle", walk: "WF_Walk", run: "WF_Run", attack: "WF_Attack", skill: "WF_Attack", buff: "WF_Idle", hit: "WF_Hit", death: "WF_Death", break1: "WF_Break1", break2: "WF_Break2" },
  spider: { idle: "SP_Idle", walk: "SP_Walk", run: "SP_Run", attack: "SP_Attack", skill: "SP_Attack", buff: "SP_Idle", hit: "SP_Hit", death: "SP_Death" },
  slime: { idle: "SL_Idle", walk: "SL_Walk", run: "SL_Run", attack: "SL_Attack", skill: "SL_Attack", buff: "SL_Idle", hit: "SL_Hit", death: "SL_Death" },
  frog: { idle: "FR_Idle", walk: "FR_Walk", run: "FR_Run", attack: "FR_Attack", skill: "FR_Attack", buff: "FR_Idle", hit: "FR_Hit", death: "FR_Death" },
  bone_lord: { idle: "BL_Idle", walk: "BL_Walk", run: "BL_Run", attack: "BL_Attack", skill: "BL_Slam", buff: "BL_Raise", hit: "BL_Hit", death: "BL_Death" },
  ozrhal: { idle: "OZ_Idle", walk: "OZ_Walk", run: "OZ_Run", attack: "OZ_Attack", skill: "OZ_Stomp", buff: "OZ_Roar", hit: "OZ_Hit", death: "OZ_Death" },
  // Brejeiro (tunnel#139): as ações do Meshy são Idle, Thrust Slash (a estocada de lança, golpe e habilidade), Hit Reaction 1 e Dead.
  brejeiro: { idle: "BJ_Idle", walk: "BJ_Walk", run: "BJ_Run", attack: "BJ_Attack", skill: "BJ_Attack", buff: "BJ_Idle", hit: "BJ_Hit", death: "BJ_Death" },
};
/**
 * NPCs do remaster (docs/arte/remaster-npcs.md): parado próprio, duas pausas (olhar em volta e o gesto do ofício) e o
 * "cumprimento" (`buff`) que toca quando o jogador abre a conversa ou a loja. NPC não anda nem apanha: os papéis de
 * combate caem no cumprimento só para o conjunto ficar completo.
 */
function npcSet(p: string): AnimSet {
  return {
    idle: `${p}Idle`, walk: `${p}Walk`, run: `${p}Run`, attack: `${p}Greet`, skill: `${p}Greet`, buff: `${p}Greet`, hit: `${p}Greet`, death: `${p}Greet`,
    break1: `${p}Break1`, break2: `${p}Break2`,
  };
}
/** Prefixo dos clipes no arquivo de cada NPC (o adaptador guarda os clipes de todos os modelos num mapa só). */
export const NPC_CLIP_PREFIX: Record<string, string> = {
  npc_livia: "LV_", npc_bruno: "BR_", npc_teodoro: "TD_", npc_otto: "OT_", npc_ysolde: "YS_", npc_edrian: "ED_", npc_severino: "SV_",
  npc_iselda: "IS_", npc_cassio: "CS_", npc_guarda: "GD_", npc_anselmo: "AN_", npc_marga: "MG_", npc_darcio: "DC_", npc_maelis: "ML_",
  npc_gaspar: "GS_", npc_rovena: "RV_",
};
for (const [m, p] of Object.entries(NPC_CLIP_PREFIX)) REMASTER_SETS[m] = npcSet(p);

/**
 * Conjunto de clipes quando o herói segura uma arma de duas mãos (e a esquerda está vazia): parado, andar e
 * correr com as duas mãos juntas na frente do peito (compose-2h-clips.mjs). O bárbaro é o único hoje; sem
 * pausas do parado (os braços ficam presos à arma). null tira o papel do conjunto.
 */
const REMASTER_2H: Record<string, Partial<Record<AnimRole, string | null>>> = {
  barbarian: { idle: "BB_2_Idle", walk: "BB_2_Walk", run: "BB_2_Run", break1: null, break2: null },
};
/**
 * Conjunto de clipes com arco ou besta na mão (Patrulheiro, Fase 3): o golpe e a skill viram o tiro
 * (Meshy Archery_Shot_1) e o dano tem a reação com o arco. Sem as variantes do golpe corpo a corpo.
 */
const REMASTER_RANGED: Record<string, Partial<Record<AnimRole, string | null>>> = {
  rogue: { attack: "RB_Shoot", attack2: null, attack3: null, skill: "RB_Shoot", hit: "RB_HitBow", hit2: null },
};
/** Besta: o tiro de uma mão (Meshy Cowboy_Quick_Draw_Shooting, recortado), por cima do conjunto do arco. */
const REMASTER_CROSSBOW: Record<string, Partial<Record<AnimRole, string | null>>> = {
  rogue: { attack: "RB_ShootCB", skill: "RB_ShootCB" },
};
/** Armas de longe: com elas o herói usa o conjunto REMASTER_RANGED. */
const RANGED_WEAPONS = new Set(["bow_withString", "crossbow_1handed", "crossbow_2handed"]);
/** Armas de duas mãos dos heróis do remaster: o cabo passa pelas duas mãos (aimTwoHand). */
const TWO_HANDED = new Set(["sword_2handed", "sword_2handed_color", "axe_2handed"]);

/** O golpe encosta quando a mão termina o movimento rápido (quadro 16 de 30, medido na velocidade da mão). */
CLIP_IMPACT.SW_Attack = 16 / 30;
CLIP_IMPACT.KB_Skill = 16 / 30;
// Bestiário: o quadro de impacto de cada golpe, medido na velocidade das mãos (tools/art/remaster-monstros/speeds.py).
CLIP_IMPACT.MN_Attack = 0.6;
CLIP_IMPACT.RG_Attack = 0.53;
CLIP_IMPACT.RG_Skill = 0.5;
CLIP_IMPACT.BJ_Attack = 0.5; // a estocada do Brejeiro (o mesmo Thrust Slash do ladino)
CLIP_IMPACT.MG_Attack = 0.47;
CLIP_IMPACT.MG_Cast = 1.3;
// Famílias (tunnel#135): fim do movimento rápido do osso que bate, medido no Blender (tools/art/familias-monstro/impact.py).
CLIP_IMPACT.WF_Attack = 1.08;
CLIP_IMPACT.SP_Attack = 0.46;
CLIP_IMPACT.SL_Attack = 0.33;
CLIP_IMPACT.FR_Attack = 0.46;
CLIP_IMPACT.GL_Attack = 1.23;
CLIP_IMPACT.GL_Slam = 1.3;
CLIP_IMPACT.GL_Roar = 1.7;
CLIP_IMPACT.BL_Attack = 0.8;
CLIP_IMPACT.BL_Slam = 1.3;
CLIP_IMPACT.OZ_Attack = 0.47;
CLIP_IMPACT.OZ_Stomp = 1.0;
CLIP_IMPACT.OZ_Roar = 0.8;
// Heróis novos: o fim do movimento rápido da mão, medido no Blender (tools/art/remaster-herois/impact.py).
CLIP_IMPACT.BB_Attack = 34 / 30;
CLIP_IMPACT.KB_Attack = 33 / 30; // corte em arco (Charged Slash); a estocada de antes virou KB_Skill
CLIP_IMPACT.BB_Skill = 41 / 30;
CLIP_IMPACT.MB_Attack = 18 / 30;
CLIP_IMPACT.MB_Skill = 17 / 30;
CLIP_IMPACT.RB_Attack = 20 / 30;
CLIP_IMPACT.RB_Skill = 22 / 30;
// Variantes do golpe (tunnel#181): trechos de combos do Meshy (rig da conta nova, add-remaster-clips.mjs), impacto no
// pico da mão direita menos o início do trecho (scripts/clip-peaks.mjs).
CLIP_IMPACT.KB_Attack2 = 0.57; // combo triplo, 1º corte
CLIP_IMPACT.KB_Attack3 = 0.38; // combo triplo, 3º corte
CLIP_IMPACT.BB_Attack2 = 0.63; // Weapon Combo 2, 1º golpe
CLIP_IMPACT.BB_Attack3 = 0.55; // Weapon Combo 2, 3º golpe
CLIP_IMPACT.MB_Attack2 = 0.73;
CLIP_IMPACT.MB_Attack3 = 0.63;
CLIP_IMPACT.RB_Attack2 = 0.57; // combo triplo, 1º corte
CLIP_IMPACT.RB_Attack3 = 0.53; // giro das lâminas
CLIP_IMPACT.RB_Shoot = 0.53; // arco: a mão da corda solta a flecha (pico da mão direita)
CLIP_IMPACT.RB_ShootCB = 0.53; // besta: o disparo, no meio da mira (recorte 0,5–2,0 s do clipe do Meshy)

export type Slot = "handslot.r" | "handslot.l";

/**
 * Pega de cada arma nos heróis do remaster, por arquivo: rotação (graus, XYZ) e deslocamento (m na escala
 * da arma, ou seja, 0,4 m reais por unidade), os dois no eixo do osso da mão: X lateral, Y ao longo do braço
 * (para os dedos, com o braço pendurado), Z para a frente. Os encaixes
 * `handslot.*` dos GLB deixam a lâmina (Y da arma) apontando para a frente da mão; aqui cada tipo ganha a
 * sua pega: espada para cima e para a frente, escudo preso ao antebraço, cajado em pé. Calibrado no
 * laboratório (client/lab.html). Os modelos KayKit (com ?remaster=0) não usam esta tabela.
 */
export interface Grip {
  rot: [number, number, number];
  pos: [number, number, number];
  /** Escala extra da arma (o cajado dos heróis é maior que eles, pelo guia de arte). */
  scale?: number;
  /** Mão em que os números foram calibrados (padrão: direita). Na outra mão a pega é espelhada. */
  hand?: "r" | "l";
  /** Pega calibrada na outra mão (os ossos das duas mãos do Meshy não são espelhos exatos); sem ela, espelha. */
  other?: { rot: [number, number, number]; pos: [number, number, number]; q?: [number, number, number, number] };
  /** Rotação como quatérnio (x, y, z, w), no lugar de `rot`. */
  q?: [number, number, number, number];
  /**
   * Cabo dentro do punho fechado dos heróis (HERO_FISTS, medido por scripts/make-remaster-fists.mjs): a pega
   * vem do punho de cada modelo e de cada mão, sem calibrar. `slide` corre a arma ao longo do cabo (positivo
   * = a mão mais perto da ponta, para cajados), e `rot`/`pos`/`q` ficam só para os modelos sem punho.
   */
  fist?: {
    slide?: number;
    /** Inclina o cabo para o lado do braço (graus): o cajado sai em diagonal para cima. */ tilt?: number;
    /** Gira o modelo antes de encaixar (graus, X Y Z): o arco tem o comprimento no X, não no Y. */ pre?: [number, number, number];
  };
}
/** Inverso da rotação dos encaixes (0,5 0,5 0,5 0,5): com ele a arma fica no eixo do osso da mão (X lateral, Y ao longo do braço, Z para a frente). */
const SLOT_INV = new Quaternion(-0.5, -0.5, -0.5, 0.5);
const BLADE: Grip = { rot: [125, 0, 20], pos: [0, 0, 0], scale: 1.2, fist: {} };
const SHIELD: Grip = { rot: [0, 20, 0], pos: [-0.02, -0.45, 0.12], hand: "l" };
export const WEAPON_GRIPS: Record<string, Grip> = {
  sword_1handed: BLADE, axe_1handed: BLADE, wand: BLADE,
  // adaga: o cabo na palma (a mão do rig fica aberta, sem dedos); a esquerda tem pega própria
  dagger: { rot: [125, 0, 20], pos: [0, 0.2, 0], other: { rot: [90, 0, 20], pos: [0, 0.2, 0] }, fist: {} },
  // Duas mãos: maiores que o normal (guia de arte, seção 2), na mão direita com a lâmina sobre o ombro.
  sword_2handed: { ...BLADE, scale: 1.3 }, sword_2handed_color: { ...BLADE, scale: 1.3 }, axe_2handed: { ...BLADE, scale: 1.3 },
  staff: { rot: [135, 0, 5], pos: [0, 0.35, 0], scale: 1.4, fist: { slide: 0.2, tilt: 60 } },
  shield_badge_color: SHIELD, shield_round_barbarian: SHIELD, shield_square: SHIELD, shield_spikes: SHIELD,
  shield_spikes_color: SHIELD,
  // Patrulheiro (Fase 3, calibrado no laboratório): o arco na mão esquerda, pelo meio, de pé ao
  // lado do corpo; as bestas na direita, ao longo do antebraço (penduradas no descanso, para a frente no tiro).
  // Arco atravessando o punho, em pé no tiro (RB_Shoot) com a corda para o lado do corpo.
  bow_withString: { ...BLADE, scale: 1.1, fist: { pre: [45, 180, 0], tilt: 0 } },
  crossbow_1handed: { ...BLADE, scale: 1.1, fist: { pre: [-90, 0, 0], tilt: -90 } },
  crossbow_2handed: { ...BLADE, scale: 1.2, fist: { pre: [-90, 0, 0], tilt: -90 } },
};
/** Arcos: o servidor manda na mão principal, mas o arco se segura com a esquerda. */
const LEFT_HAND_WEAPONS = new Set(["bow_withString"]);
/** Armas padrão de cada modelo. Os heróis usam o que o servidor manda (equipamento). */
const WEAPONS: Record<string, Partial<Record<Slot, string>>> = {
  knight: {},
  barbarian: {},
  mage: {},
  rogue: {},
  npc_merchant: {},
  npc_smith: { "handslot.r": "axe_1handed" },
};
/** Ferramenta na mão dos NPCs do remaster (arquivo da arma): o ferreiro leva o martelo (machado de uma mão). */
const NPC_WEAPONS: Record<string, Partial<Record<Slot, string>>> = {
  npc_bruno: { "handslot.r": "axe_1handed" }, // o gesto do ofício é o martelo; sem martelo no jogo, a ferramenta de uma mão de sempre
};
/** Todas as armas que os itens podem mostrar na mão. */
const ITEM_WEAPONS = [
  "sword_1handed", "sword_2handed", "sword_2handed_color", "axe_1handed", "axe_2handed", "staff", "wand", "dagger",
  "shield_badge_color", "shield_round_barbarian", "shield_square", "shield_spikes", "shield_spikes_color",
  "bow_withString", "crossbow_1handed", "crossbow_2handed",
  // Opções da criação de personagem (mesmos arquivos das armas dos esqueletos).
  "skeleton_blade", "skeleton_axe", "skeleton_staff", "skeleton_shield_large_a",
];
/** Brilho das armas raras (3) e épicas (4). */
const GRADE_GLOW: Record<number, Color3> = { 3: new Color3(0.15, 0.35, 0.9), 4: new Color3(0.6, 0.2, 0.95), 5: new Color3(1, 0.5, 0.1) };
/** Brilho sutil do corpo lendário (somado à luz; pulsa de leve). */
const LEGEND_GLOW = new Color3(0.1, 0.075, 0.02);
/** Encantamento: +4 a +6 brilha dourado; +7 ou mais, dourado forte e pulsando. */
const ENCHANT_GLOW = { light: new Color3(0.45, 0.33, 0.08), strong: new Color3(1, 0.72, 0.22) };

/** Fração do brilho de raridade e de encantamento nas armas do remaster. */
const SOFT_GLOW = 0.12;

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
  // Só no remaster (remasterModelOf): os chefes com silhueta própria.
  bone_lord: { "handslot.r": "skeleton_blade", "handslot.l": "skeleton_shield_large_a" },
  ozrhal: { "handslot.r": "skeleton_staff" },
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
  /** Armadura por raridade (remaster): textura do corpo, ombreiras, capa e variante de elmo. Sem efeito fora do remaster. */
  setArmor(armor: ArmorGrades): void;
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
  /** Modelo do estilo B (remaster, o padrão): clipes e LODs no próprio arquivo. */
  remaster: boolean;
  /** Máscara de repintura do remaster (R pano, G pele): a base é KTX2 e não dá para ler os pixels. */
  mask: Texture | null;
}

/** Escala extra das armas do remaster que dividem o arquivo com outra (REMASTER_WEAPONS). */
const weaponScale = new Map<AssetContainer, number>();
/** Formas por raridade das armas do remaster (WEAPON_FORM_FILES), por arquivo. */
const weaponForms = new Map<string, AssetContainer>();

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
  /** Materiais de corpo por classe e faixa de raridade (textura própria, KTX2 sob demanda). */
  private tierMats = new Map<string, Promise<Material | null>>();
  /** "Bom": o mesmo corpo com mais metal; um por material-base. */
  private goodMats = new Map<number, Material>();
  /** Peças anexadas (ombreiras, capas, elmos) carregadas sob demanda. */
  private pieceFiles = new Map<string, Promise<AssetContainer | null>>();
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
  private acquirePaint(model: string, tier: number, base: Material, look: BodyLook): Promise<Material | null> {
    const key = `${colorKey(model, look)}|t${tier}`;
    let e = this.paintMats.get(key);
    if (!e) {
      e = { mat: this.paintMaterial(base, model, look, tier), refs: 0 };
      this.paintMats.set(key, e);
    }
    e.refs++;
    return e.mat;
  }

  private releasePaint(model: string, tier: number, look: BodyLook): void {
    const key = `${colorKey(model, look)}|t${tier}`;
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
  private async tintMaterial(base: Material, src: Texture, mask: Texture, model: string, look: BodyLook, pal: BodyPalettes, tier = 0): Promise<Material | null> {
    await new Promise<void>((ok) => (mask.isReady() ? ok() : mask.onLoadObservable.addOnce(() => ok())));
    const scene = base.getScene();
    Effect.ShadersStore[`${TINT_SHADER}PixelShader`] ??= TINT_FRAGMENT;
    const { width } = src.getSize();
    const tex = new ProceduralTexture(`${src.name}-${colorKey(model, look)}-t${tier}`, width || 1024, TINT_SHADER, scene, null, true, false);
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

  /**
   * Material do corpo de uma faixa de raridade (2 raro, 3 épico, 4 lendário): cópia do material-base com a textura
   * KTX2 da faixa (public/models/armor/<classe>_<faixa>.ktx2, mesma UV do corpo). Um por classe e faixa, compartilhado
   * por todos que vestem aquilo (nenhum draw call a mais). A lendária ganha um brilho dourado que pulsa de leve.
   */
  private tierMaterial(model: string, tier: number, base: Material): Promise<Material | null> {
    const key = `${model}:${tier}`;
    let p = this.tierMats.get(key);
    if (!p) {
      p = (async () => {
        const tex = new Texture(`${BASE}armor/${model}_${TIER_FILES[tier]}.ktx2`, this.scene, { noMipmap: false, invertY: false, samplingMode: Texture.TRILINEAR_SAMPLINGMODE });
        const ok = await new Promise<boolean>((done) => {
          if (tex.isReady()) return done(true);
          tex.onLoadObservable.addOnce(() => done(true));
          (tex as Texture & { onErrorObservable?: { addOnce(f: () => void): void } }).onErrorObservable?.addOnce(() => done(false));
          setTimeout(() => done(false), 15000);
        });
        if (!ok) {
          tex.dispose();
          return null;
        }
        const mat = base.clone(`${base.name}-t${tier}`);
        if (mat instanceof PBRMaterial) {
          mat.albedoTexture = tex;
          if (tier === 4) {
            mat.emissiveColor = LEGEND_GLOW.clone();
            this.scene.onBeforeRenderObservable.add(() => {
              const k = 0.6 + 0.4 * Math.sin(performance.now() / 700);
              mat.emissiveColor.copyFromFloats(LEGEND_GLOW.r * k, LEGEND_GLOW.g * k, LEGEND_GLOW.b * k);
            });
          }
        } else if (mat instanceof StandardMaterial) mat.diffuseTexture = tex;
        return mat;
      })();
      this.tierMats.set(key, p);
    }
    return p;
  }

  /** "Bom": a mesma textura mais clara e lisa, com um fio de aço azulado (um material por base). */
  private goodMaterial(base: Material): Material {
    let m = this.goodMats.get(base.uniqueId);
    if (!m) {
      m = base.clone(`${base.name}-good`) ?? base;
      if (m instanceof PBRMaterial) {
        // Sem mapa de ambiente a cena não reflete, então o "metal" é lido por cor: mais claro, frio e liso.
        m.albedoColor = new Color3(1.2, 1.2, 1.3);
        m.roughness = 0.45;
        m.emissiveColor = new Color3(0.03, 0.04, 0.06);
      }
      this.goodMats.set(base.uniqueId, m);
    }
    return m;
  }

  private pieceContainer(file: string): Promise<AssetContainer | null> {
    let p = this.pieceFiles.get(file);
    if (!p) {
      p = LoadAssetContainerAsync(`${BASE}armor/${file}.glb`, this.scene).then(
        (c) => {
          for (const m of c.materials) setPaletteKind(m, "char");
          return c;
        },
        (err: unknown) => {
          reportClientError("model", `peça de armadura não carregou: ${file}`, { file, error: String((err as Error)?.message ?? err) }, err);
          return null;
        },
      );
      this.pieceFiles.set(file, p);
    }
    return p;
  }

  /** Cópia do material com as faixas de pele, cabelo e roupa repintadas. */
  private async paintMaterial(base: Material, model: string, look: BodyLook, tier = 0): Promise<Material | null> {
    const pal = await bodyPalettes();
    const src = base instanceof PBRMaterial ? base.albedoTexture : base instanceof StandardMaterial ? base.diffuseTexture : null;
    if (!pal || !(src instanceof Texture)) return null;
    await new Promise<void>((ok) => (src.isReady() ? ok() : src.onLoadObservable.addOnce(() => ok())));
    const mask = this.models.get(model)?.mask;
    if (REMASTER && mask) return this.tintMaterial(base, src, mask, model, look, pal, tier);
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

  /**
   * Aquece o cache HTTP com os .glb que o load() vai pedir (tela de escolha de personagem), sem criar cena.
   * Quando o load() pede o mesmo arquivo, vem do cache. Falhas são ignoradas (o load() reporta as dele).
   */
  static warmCache(): void {
    if (warmed) return;
    warmed = true;
    const allWeapons = { ...WEAPONS, ...MONSTER_WEAPONS, ...NPC_WEAPONS };
    const weaponFiles = [...new Set([...Object.values(allWeapons).flatMap((w) => Object.values(w)), ...ITEM_WEAPONS])];
    const ids = [...Object.keys(WEAPONS), ...MONSTER_MODELS, ...(REMASTER ? ["stone_golem", ...REMASTER_ONLY] : [])].filter(
      (id) => !(REMASTER && REMASTER_LAZY.includes(id)),
    );
    const files = [
      ...ANIM_FILES,
      ...weaponFiles.map((w) => (REMASTER && REMASTER_WEAPONS[w] ? `weapons/${REMASTER_WEAPONS[w].file}.glb` : `weapons/${w}.glb`)),
      ...(REMASTER ? WEAPON_FORM_FILES.map((f) => `weapons/${f}.glb`) : []),
      ...ids.map((id) => (REMASTER && REMASTER_FILES[id] ? `${REMASTER_FILES[id]}.glb` : `${id}.glb`)),
    ];
    // Poucos de cada vez, para não brigar com a tela de personagens.
    const queue = [...new Set(files)];
    const worker = async () => {
      for (let f = queue.shift(); f; f = queue.shift()) await fetch(BASE + f).then((r) => r.arrayBuffer()).catch(() => undefined);
    };
    for (let i = 0; i < 3; i++) void worker();
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

    const allWeapons = { ...WEAPONS, ...MONSTER_WEAPONS, ...NPC_WEAPONS };
    const weaponFiles = [...new Set([...Object.values(allWeapons).flatMap((w) => Object.values(w)), ...ITEM_WEAPONS])];
    // stone_golem e os chefes só têm arquivo no remaster; sem ele o golem é o de código e os chefes reaproveitam os esqueletos.
    // Os mais pesados e raros (golem, chefes: ~14 MB) ficam de fora e baixam sob demanda (ensure).
    const modelIds = [...Object.keys(WEAPONS), ...MONSTER_MODELS, ...(REMASTER ? ["stone_golem", ...REMASTER_ONLY] : [])].filter(
      (id) => !(REMASTER && REMASTER_LAZY.includes(id)),
    );

    const [animContainers, weaponList, bodies, formList] = await Promise.all([
      Promise.all(ANIM_FILES.map(load)),
      // Remaster (padrão): a arma nova no lugar (e o KayKit de sempre se o arquivo faltar).
      Promise.all(
        weaponFiles.map(async (w) => {
          const rw = REMASTER ? REMASTER_WEAPONS[w] : undefined;
          // Um container por nome (as escalas diferem entre quem divide o arquivo).
          const c = rw ? await tryLoad(`weapons/${rw.file}.glb`) : null;
          if (c && rw?.scale) weaponScale.set(c, rw.scale);
          return [w, c ?? (await tryLoad(`weapons/${w}.glb`))] as const;
        }),
      ),
      Promise.all(
        modelIds.map(async (id) => {
          // Remaster (padrão): o arquivo do estilo B, se existir; senão, o KayKit de sempre.
          const rm = REMASTER && REMASTER_FILES[id] ? await tryLoad(`${REMASTER_FILES[id]}.glb`) : null;
          return [id, rm ?? (await tryLoad(`${id}.glb`)), !!rm] as const;
        }),
      ),
      // Remaster (padrão): as formas por raridade (comum, rara e alta de cada família); o grau do item escolhe uma.
      Promise.all(WEAPON_FORM_FILES.filter(() => REMASTER).map(async (f) => [f, await tryLoad(`weapons/${f}.glb`)] as const)),
    ]);
    for (const [f, c] of formList) if (c) weaponForms.set(f, c);
    const weaponByFile = new Map(weaponList);
    // Material de paleta: personagens, monstros e armas com a borda e o degradê mais fortes
    // que o cenário. As cópias (repintura, brilho) herdam o tipo.
    for (const c of [...bodies.map(([, b]) => b), ...weaponList.map(([, w]) => w), ...formList.map(([, w]) => w)]) {
      for (const m of c?.materials ?? []) setPaletteKind(m, "char");
    }

    const models = new Map<string, ModelAssets>();
    const register = (id: string, body: AssetContainer, remaster: boolean) => {
      const weapons: ModelAssets["weapons"] = {};
      for (const [slot, file] of Object.entries(allWeapons[id] ?? {}) as [Slot, string][]) {
        const w = weaponByFile.get(file);
        if (w) weapons[slot] = w;
      }
      const heads = new Map<string, AbstractMesh>();
      for (const m of body.meshes) if (m.name.startsWith("Head_")) heads.set(m.name.slice(5), m);
      const maskFile = remaster ? REMASTER_MASKS[id] : undefined;
      const mask = maskFile ? new Texture(`${BASE}${maskFile}`, scene, true, false) : null;
      models.set(id, { body, weapons, scale: measureScale(body, id, remaster ? REMASTER_HEIGHTS[id] : undefined), heads, remaster, mask });
    };
    for (const [id, body, remaster] of bodies) if (body) register(id, body, remaster);

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
    const result = new CharacterModels(models, templates, weaponByFile, scene);
    // Corpos sob demanda (REMASTER_LAZY): mesmo caminho dos de sempre.
    result.lazyLoader = async (id) => {
      const file = REMASTER_FILES[id];
      const body = file ? await tryLoad(`${file}.glb`) : null;
      if (!body) return false;
      for (const m of body.materials) setPaletteKind(m, "char");
      register(id, body, true);
      for (const g of body.animationGroups) {
        g.stop();
        templates.set(g.name, g);
      }
      return true;
    };
    return result;
  }

  /** Carrega o corpo de ensure(). */
  private lazyLoader: ((id: string) => Promise<boolean>) | null = null;
  private lazyLoads = new Map<string, Promise<boolean>>();
  /** Avisa quando um corpo sob demanda chegou (as entidades que esperavam trocam a cápsula pelo modelo). */
  onModelReady: ((id: string) => void) | null = null;

  /** O modelo existe no jogo mas ainda não foi baixado (golem e chefes do remaster)? */
  pending(model: string): boolean {
    return REMASTER && REMASTER_LAZY.includes(model) && !this.models.has(model);
  }

  /** Baixa um corpo sob demanda (uma vez só). Resolve true quando está pronto. */
  ensure(model: string): Promise<boolean> {
    if (!this.pending(model) || !this.lazyLoader) return Promise.resolve(this.models.has(model));
    let p = this.lazyLoads.get(model);
    if (!p) {
      p = this.lazyLoader(model).then((ok) => {
        if (ok) this.onModelReady?.(model);
        return ok;
      });
      this.lazyLoads.set(model, p);
    }
    return p;
  }

  /**
   * Baixa os corpos sob demanda no tempo ocioso, um de cada vez, para o chefe já estar pronto quando o jogador
   * chegar. Fica de fora com "economizar dados" e em rede lenta (2G/3G): lá só baixam quando aparecem.
   */
  prefetchLazy(): void {
    const c = (navigator as unknown as { connection?: { saveData?: boolean; effectiveType?: string } }).connection;
    if (c?.saveData || (c?.effectiveType && c.effectiveType !== "4g")) return;
    void (async () => {
      for (const id of REMASTER_LAZY) if (!REMASTER_NO_PREFETCH.includes(id) && this.pending(id)) await this.ensure(id);
    })();
  }

  /** Cópia do material com brilho (uma por material + tipo de brilho; "e2" pulsa). */
  private glowMaterial(base: Material, glow: string, soft = false): Material {
    const key = `${base.uniqueId}:${glow}${soft ? "s" : ""}`;
    let mat = this.glowMats.get(key);
    if (!mat) {
      mat = base.clone(`${base.name}-${glow}`) ?? base;
      // Remaster (soft): as armas têm luz pintada e a forma é o que diz a raridade; o brilho é um toque, não uma mancha.
      const f = soft ? SOFT_GLOW : 1;
      const c = (
        glow === "e2" ? ENCHANT_GLOW.strong.clone() : glow === "e1" ? ENCHANT_GLOW.light : GRADE_GLOW[Number(glow.slice(1))]
      ).scale(f);
      const target = mat instanceof PBRMaterial || mat instanceof StandardMaterial ? mat : null;
      if (target) target.emissiveColor = c;
      if (glow === "e2" && target) {
        // Um pulso por material (todas as armas +7 pulsam juntas).
        const scene = mat.getScene();
        scene.onBeforeRenderObservable.add(() => {
          const k = 0.55 + 0.45 * Math.sin(performance.now() / 260);
          target.emissiveColor.copyFromFloats(ENCHANT_GLOW.strong.r * k * f, ENCHANT_GLOW.strong.g * k * f, ENCHANT_GLOW.strong.b * k * f);
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
    if (!this.models.has(model) && MODEL_FALLBACK[model]) model = MODEL_FALLBACK[model]; // família ainda sem arquivo (remaster.ts)
    // Golem: modelado e animado em código (sem arquivo glTF).
    if (model === GOLEM_MODEL && !this.models.get(model)?.remaster) return createGolem(this.scene, id);
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
    /** Arquivo da arma em cada mão (para saber se é de duas mãos). */
    const heldFile = new Map<Slot, string>();
    const attach = (slot: Slot, container: AssetContainer | null | undefined, grade: number, enchant = 0) => {
      for (const n of held.get(slot) ?? []) n.dispose(false, false);
      held.delete(slot);
      heldFile.delete(slot);
      const hand = byName.get(slot);
      if (!hand || !container) return;
      // Remaster: o grau do item troca a forma (comum, rara ou alta) e a escala que o nome do modelo pede.
      const name = this.weaponFileOf.get(container) ?? "";
      const form = assets.remaster && grade > 0 ? remasterWeapon(name, grade) : undefined;
      const formC = form ? weaponForms.get(form.file) : undefined;
      const w = (formC ?? container).instantiateModelsToScene((n) => `${n}#${id}`, false);
      const wRoot = w.rootNodes[0] as TransformNode;
      // O __root__ do glTF converte o sistema de coordenadas; dentro do esqueleto isso já foi
      // feito, então prendemos o filho direto e descartamos o __root__ da arma.
      const nodes: TransformNode[] = [];
      for (const child of wRoot.getChildren() as TransformNode[]) {
        child.parent = hand as TransformNode;
        child.position = Vector3.Zero();
        const grip = assets.remaster ? WEAPON_GRIPS[this.weaponFileOf.get(container) ?? ""] : undefined;
        const fist = grip?.fist ? HERO_FISTS[model]?.[slot === "handslot.l" ? "l" : "r"] : undefined;
        if (grip && fist) {
          // Punho fechado: o cabo (Y da arma) segue o eixo do punho e o fio (X) acompanha os dedos.
          const a0 = Vector3.FromArray(fist.axis), f0 = Vector3.FromArray(fist.fingers).scale(-1);
          const tl = ((grip.fist?.tilt ?? 0) * Math.PI) / 180;
          const ax = a0.scale(Math.cos(tl)).addInPlace(f0.scale(Math.sin(tl)));
          const fx = f0.scale(Math.cos(tl)).subtractInPlace(a0.scale(Math.sin(tl)));
          const z = Vector3.Cross(fx, ax);
          const q = Quaternion.FromRotationMatrix(Matrix.FromValues(fx.x, fx.y, fx.z, 0, ax.x, ax.y, ax.z, 0, z.x, z.y, z.z, 0, 0, 0, 0, 1));
          const pre = grip.fist?.pre;
          const preQ = pre ? Quaternion.FromEulerAngles((pre[0] * Math.PI) / 180, (pre[1] * Math.PI) / 180, (pre[2] * Math.PI) / 180) : Quaternion.Identity();
          child.rotationQuaternion = SLOT_INV.multiply(q).multiply(preQ).multiply(child.rotationQuaternion ?? Quaternion.Identity());
          Vector3.FromArray(fist.pos).addInPlace(ax.scale(grip.fist?.slide ?? 0)).rotateByQuaternionToRef(SLOT_INV, child.position);
          if (grip.scale) child.scaling.scaleInPlace(grip.scale);
        } else if (grip) {
          const d = Math.PI / 180;
          // Rotação e deslocamento no eixo do osso da mão (o encaixe é desfeito antes).
          const swapped = (grip.hand ?? "r") !== (slot === "handslot.l" ? "l" : "r");
          const mirror = swapped && !grip.other ? -1 : 1;
          const use = swapped && grip.other ? grip.other : grip;
          const q = use.q
            ? new Quaternion(use.q[0], use.q[1] * mirror, use.q[2] * mirror, use.q[3])
            : Quaternion.FromEulerAngles(use.rot[0] * d, use.rot[1] * d * mirror, use.rot[2] * d * mirror);
          child.rotationQuaternion = SLOT_INV.multiply(q).multiply(child.rotationQuaternion ?? Quaternion.Identity());
          new Vector3(use.pos[0] * mirror, use.pos[1], use.pos[2]).rotateByQuaternionToRef(SLOT_INV, child.position);
          if (grip.scale) child.scaling.scaleInPlace(grip.scale);
        }
        const ws = formC ? form?.scale : weaponScale.get(container);
        if (ws) child.scaling.scaleInPlace(ws);
        nodes.push(child);
        const glow = glowKey(grade, enchant);
        if (glow) {
          for (const m of child.getChildMeshes(false)) if (m.material) m.material = this.glowMaterial(m.material, glow, assets.remaster);
          if (child instanceof AbstractMesh && child.material) child.material = this.glowMaterial(child.material, glow, assets.remaster);
        }
      }
      wRoot.dispose();
      held.set(slot, nodes);
      heldFile.set(slot, this.weaponFileOf.get(container) ?? "");
    };
    for (const [slot, container] of Object.entries(handItems) as [Slot, AssetContainer][]) attach(slot, container, 0);

    // Visual do corpo: só os heróis têm várias cabeças e o chapéu separado.
    const hat = byName.get("Hat");
    // Só as malhas com esqueleto (as armas presas nas mãos não entram).
    const bodyMeshes = gltfRoot.getChildMeshes(false).filter((m) => !!m.skeleton && !!m.material);
    // LOD (remaster): nós com sufixo _LOD1/_LOD2; o elmo ("Hat", "Hat_LOD1"...) some com hideHat.
    let lodLevel = 0;
    let hatHidden = false;
    let helmOverride = false; // elmo/chapéu de raridade no lugar do Hat do corpo
    let onVis: () => void = () => {};
    const lodNodes = assets.remaster
      ? bodyMeshes.map((m) => {
          const base = m.name.split("#")[0].replace(/\.\d+$/, "");
          return { m, lod: Number(/_LOD(\d)$/.exec(base)?.[1] ?? 0), hat: /^Hat(_LOD\d)?$/.test(base) };
        })
      : [];
    const refreshVis = () => {
      for (const n of lodNodes) n.m.setEnabled(n.lod === lodLevel && !(n.hat && (hatHidden || helmOverride)));
      onVis();
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
    let painted: { look: BodyLook; tier: number } | null = null; // cores em uso (material repintado)
    let disposed = false;
    // Armadura por raridade (remaster): material do corpo da faixa e peças anexadas.
    let tierMat: Material | null = null;
    let tierId = 0;
    let lookNow: BodyLook = ORIGINAL_LOOK;
    let armorNow: ArmorGrades = NO_ARMOR;
    let armorApplied = "";
    const pieces = new Map<string, { file: string; nodes: TransformNode[] }>();
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
      lookNow = l;
      refreshMaterial();
    };
    /** O material do corpo: a faixa de raridade (ou o base) e, por cima, as cores do Barbeiro. */
    const refreshMaterial = () => {
      const base = tierMat ?? baseMat;
      if (!base) return;
      const l = lookNow;
      if (painted && painted.tier === tierId && colorKey(model, painted.look) === colorKey(model, l)) return;
      if (painted) this.releasePaint(model, painted.tier, painted.look);
      painted = null;
      for (const m of paintable()) m.material = base;
      if (!needsRepaint(l)) return;
      const mine = { look: l, tier: tierId };
      painted = mine;
      void this.acquirePaint(model, tierId, base, l).then((mat) => {
        if (!mat || disposed || painted !== mine) return;
        for (const m of paintable()) m.material = mat;
      });
    };
    if (look) applyLook(look);
    else if (head.mesh) applyLook(ORIGINAL_LOOK);

    // Animações (só as do conjunto do modelo), reaplicadas pelo nome dos ossos.
    const anims = new RoleMap();
    anims.footSpeeds = (assets.remaster ? FOOT_SPEEDS_REMASTER : FOOT_SPEEDS_KAYKIT)[model];
    const cloned = new Map<string, AnimationGroup>();
    const impacts: Partial<Record<AnimRole, number>> = {};
    const groupFor = (animName: string): AnimationGroup | null => {
      let g = cloned.get(animName);
      if (!g) {
        const tpl = this.animTemplates.get(animName);
        if (!tpl) return null;
        g = tpl.clone(`${animName}#${id}`, (target: Node) => byName.get(target.name) ?? null);
        g.stop();
        cloned.set(animName, g);
      }
      return g;
    };
    /** Monta os papéis do conjunto normal ou do de duas mãos (o AnimController é avisado pelo RoleMap). */
    const applySet = (mode: "normal" | "two" | "ranged" | "crossbow") => {
      const over = !assets.remaster
        ? undefined
        : mode === "two"
          ? REMASTER_2H[model]
          : mode === "ranged" || mode === "crossbow"
            ? { ...REMASTER_RANGED[model], ...(mode === "crossbow" ? REMASTER_CROSSBOW[model] : {}) }
            : undefined;
      anims.clear();
      for (const k of Object.keys(impacts)) delete impacts[k as AnimRole];
      for (const [role, base] of Object.entries(set) as [AnimRole, string][]) {
        const name = over && role in over ? over[role] : base;
        const g = name ? groupFor(name) : null;
        if (!name || !g) continue;
        anims.set(role, g);
        if (CLIP_IMPACT[name] !== undefined) impacts[role] = CLIP_IMPACT[name];
      }
      anims.onChange?.();
    };
    applySet("normal");
    let setMode: "normal" | "two" | "ranged" | "crossbow" = "normal";
    let twoWeapon: TransformNode | null = null;
    /** Arma de duas mãos na direita e a esquerda vazia: usa o conjunto de duas mãos e o cabo passa pelas duas mãos. */
    const refreshTwoHand = () => {
      const r = heldFile.get("handslot.r");
      const two = !!assets.remaster && !!r && TWO_HANDED.has(r) && !heldFile.has("handslot.l") && !!REMASTER_2H[model];
      twoWeapon = two ? (held.get("handslot.r")?.[0] ?? null) : null;
      const ranged = !!assets.remaster && !!REMASTER_RANGED[model] && [...heldFile.values()].some((f) => RANGED_WEAPONS.has(f));
      const crossbow = ranged && [...heldFile.values()].some((f) => f.startsWith("crossbow"));
      const mode = two ? "two" : crossbow ? "crossbow" : ranged ? "ranged" : "normal";
      if (mode !== setMode) {
        setMode = mode;
        applySet(mode);
      }
    };
    refreshTwoHand();
    const handR = byName.get("handslot.r") as TransformNode | undefined;
    const handL = byName.get("handslot.l") as TransformNode | undefined;
    const aimObs =
      assets.remaster && handR && handL
        ? gltfRoot.getScene().onAfterAnimationsObservable.add(() => {
            if (!disposed && twoWeapon) aimTwoHand(handR, handL, twoWeapon, HERO_FISTS[model]);
          })
        : null;

    // ---- Armadura por raridade (remaster): peças presas nos ossos pelas posições de armor-attach.ts ----
    const pieceVisible = (kind: string, node: TransformNode) => {
      const lod1 = /_LOD1$/.test(node.name.split("#")[0]);
      const hasLod1 = (pieces.get(kind)?.nodes ?? []).some((n) => /_LOD1$/.test(n.name.split("#")[0]));
      const want = hasLod1 && lodLevel >= 1 ? lod1 : !lod1;
      return want && !(kind === "helm" && hatHidden);
    };
    onVis = () => {
      for (const [kind, p] of pieces) for (const n of p.nodes) n.setEnabled(pieceVisible(kind, n));
    };
    const setPiece = (kind: string, file: string | null) => {
      const cur = pieces.get(kind);
      if ((cur?.file ?? null) === file) return;
      for (const n of cur?.nodes ?? []) n.dispose(false, false);
      pieces.delete(kind);
      if (!file) return;
      const at = ARMOR_ATTACH[model]?.[file];
      const bone = at ? byName.get(at.bone) : undefined;
      if (!at || !bone) return;
      const entry = { file, nodes: [] as TransformNode[] };
      pieces.set(kind, entry);
      void this.pieceContainer(file).then((c) => {
        if (!c || disposed || pieces.get(kind) !== entry) return;
        const w = c.instantiateModelsToScene((n) => `${n}#${id}`, false);
        const wRoot = w.rootNodes[0] as TransformNode;
        for (const child of wRoot.getChildren() as TransformNode[]) {
          child.parent = bone as TransformNode;
          child.position.set(at.pos[0], at.pos[1], at.pos[2]);
          child.rotationQuaternion = new Quaternion(at.rot[0], at.rot[1], at.rot[2], at.rot[3]);
          child.scaling.setAll(at.scale);
          entry.nodes.push(child);
        }
        wRoot.dispose();
        onVis();
      });
    };
    const setArmor = (a: ArmorGrades) => {
      if (!assets.remaster || disposed) return;
      const key = armorKey(model, a);
      if (key === armorApplied) return;
      armorApplied = key;
      armorNow = a;
      setPiece("shoulder", pauldronFile(a));
      setPiece("cape", capeFile(a));
      const helm = helmFile(model, a);
      helmOverride = !!helm;
      setPiece("helm", helm);
      refreshVis();
      if (!baseMat) return;
      const tier = bodyTier(a);
      if (tier <= 1) {
        tierMat = tier === 1 ? this.goodMaterial(baseMat) : null;
        tierId = tier;
        refreshMaterial();
      } else {
        void this.tierMaterial(model, tier, baseMat).then((m) => {
          if (!m || disposed || bodyTier(armorNow) !== tier) return;
          tierMat = m;
          tierId = tier;
          refreshMaterial();
        });
      }
    };

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
        if (right && !left && LEFT_HAND_WEAPONS.has(right)) [right, left] = [null, right];
        attach("handslot.r", right ? this.weaponFiles.get(right) : null, grade, enchant);
        attach("handslot.l", left ? this.weaponFiles.get(left) : null, grade, enchant);
        refreshTwoHand();
      },
      setLook: (l) => {
        if (!disposed) applyLook(l);
      },
      setArmor,
      meshes: () => root.getChildMeshes(false),
      dispose: () => {
        disposed = true;
        aimObs?.remove();
        unwatchLod();
        if (painted) this.releasePaint(model, painted.tier, painted.look);
        painted = null;
        for (const p of pieces.values()) for (const n of p.nodes) n.dispose(false, false);
        pieces.clear();
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
 * Arma de duas mãos: o cabo passa pelas duas mãos. Depois das animações, a arma (filha da mão direita) gira
 * para ficar na linha que vai da mão esquerda à direita, com a esquerda perto do pé do cabo; o giro em torno
 * do cabo vem da pega calibrada (WEAPON_GRIPS), então só a direção do cabo muda.
 */
function aimTwoHand(handR: TransformNode, handL: TransformNode, weapon: TransformNode, fists?: { r?: FistFrame; l?: FistFrame }): void {
  const chain = (n: TransformNode): TransformNode[] => {
    const out: TransformNode[] = [];
    for (let p: Node | null = n; p; p = p.parent) if (p instanceof TransformNode) out.unshift(p);
    return out;
  };
  for (const n of [...chain(handR), ...chain(handL), ...chain(weapon)]) n.computeWorldMatrix(true);
  // Com as mãos fechadas, o cabo passa pelo centro de cada punho (não pela palma).
  const grab = (hand: TransformNode, f?: FistFrame) =>
    f ? Vector3.TransformCoordinates(Vector3.FromArray(f.pos).rotateByQuaternionToRef(SLOT_INV, new Vector3()), hand.getWorldMatrix()) : hand.getAbsolutePosition();
  const pr = grab(handR, fists?.r);
  const pl = grab(handL, fists?.l);
  const along = pr.subtract(pl);
  const dist = along.length();
  if (dist < 0.04) return;
  const w = weapon.getWorldMatrix();
  const xAxis = new Vector3(w.m[0], w.m[1], w.m[2]);
  const yCur = new Vector3(w.m[4], w.m[5], w.m[6]);
  const zAxis = new Vector3(w.m[8], w.m[9], w.m[10]);
  const unit = yCur.length();
  const t = along.scale(1 / dist);
  const y = yCur.normalizeToNew();
  if (Vector3.Dot(t, y) < 0) t.scaleInPlace(-1); // a lâmina continua para o lado em que a pega a pôs
  const x = xAxis.subtract(t.scale(Vector3.Dot(xAxis, t))).normalize();
  // mesma "mão" (reflexão do glTF) da matriz atual
  const handed = Vector3.Dot(Vector3.Cross(xAxis, yCur), zAxis) < 0 ? -1 : 1;
  const z = Vector3.Cross(x, t).scale(handed);
  // a mão esquerda segura o cabo 0,2 unidades abaixo do ponto da direita
  const origin = pl.add(t.scale(0.2 * unit));
  const m = Matrix.FromValues(
    x.x * unit, x.y * unit, x.z * unit, 0,
    t.x * unit, t.y * unit, t.z * unit, 0,
    z.x * unit, z.y * unit, z.z * unit, 0,
    origin.x, origin.y, origin.z, 1,
  );
  const parent = weapon.parent instanceof TransformNode ? weapon.parent.getWorldMatrix() : Matrix.Identity();
  const local = m.multiply(parent.clone().invert());
  const scale = new Vector3();
  const rot = new Quaternion();
  const pos = new Vector3();
  if (!local.decompose(scale, rot, pos)) return;
  weapon.position.copyFrom(pos);
  weapon.rotationQuaternion = rot;
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
function measureScale(container: AssetContainer, model: string, height = TARGET_HEIGHT): number {
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
  return h > 0.1 && Number.isFinite(h) ? height / h : 1;
}
