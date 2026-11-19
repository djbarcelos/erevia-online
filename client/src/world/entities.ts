import {
  AbstractMesh,
  BoundingInfo,
  Color3,
  Matrix,
  Mesh,
  MeshBuilder,
  Scene,
  ShadowGenerator,
  StandardMaterial,
  TransformNode,
  Vector3,
} from "@babylonjs/core";
import { AdvancedDynamicTexture, Rectangle, StackPanel, TextBlock } from "@babylonjs/gui";
import { classInfo } from "../classes";
import { noteCorrection, noteDespawn, noteResync, noteSpawn } from "../net/sync-stats";
import {
  Effect,
  EmoteKind,
  EntityKind,
  PvpState,
  SkillTarget,
  type CombatEvent,
  type EffectEnd,
  type EffectEvent,
  type Appearance,
  type Entity,
  type EntityState,
  type HpUpdate,
  type LevelUp,
  type MonsterArchetype,
  type PlayerStatus,
  type Revive,
  type SkillCast,
  type SkillInfo,
} from "../gen/erevia/v1/game_pb";
import { gameAudio } from "../audio";
import { AnimController } from "./anim";
import { updateMotion } from "./motion";
import { toArmorGrades, type ArmorGrades } from "./armor";
import { toBodyLook, type BodyLook } from "./body-look";
import type { Fx, FxBody, FxHandle } from "./fx";
import type { CharacterInstance, CharacterModels } from "./models";
import { GOLEM_BODY, GOLEM_MODEL } from "./golem";
import { buildTeleportStone } from "./teleport-stone";
import type { PickAreas } from "./pick";
import type { Bounds, Collision } from "./world-data";
import { specInfo } from "../ui/specs";
import { GRADE_COLOR } from "../ui/items";
import { settings } from "../ui/kit";
import { skillFxColors } from "../ui/skill-themes";
import type { EntityProbe } from "../net/invariants";
import { FIGHT_MS, type HuntCandidate } from "./huntrules";
import { remasterModelOf, remasterNpcModel } from "./remaster";
import { ARCH, neutralNow } from "./roles";
import { groundClearance, heightAt, terrain } from "./terrain";

/** Quão rápido a posição desenhada converge para a do servidor (1/s). */
const CORRECTION_RATE = 12;
/** Tempo máximo esperando o servidor confirmar um destino previsto. */
const PENDING_TIMEOUT_MS = 1000;
/**
 * Jogador local: diferença entre o servidor e a previsão abaixo da qual não se corrige
 * nada (é só imprecisão da latência medida), e acima da qual se volta a seguir o servidor.
 */
const LOCAL_DEADZONE = 0.4;
const LOCAL_SNAP = 4;
/** Loja pessoal: depois de sentar, o vendedor fica com a pose parada (sem recalcular os ossos). */
const VENDOR_FREEZE_MS = 1500;
/** Quão rápido uma correção da previsão some da tela (1/s): desliza em vez de pular. */
const LOCAL_SMOOTH_RATE = 10;
/** Quanto tempo de previsão guardar para comparar com snapshots atrasados (ms). */
const LOCAL_HISTORY_MS = 2000;
/** Velocidade de referência da animação de corrida (m/s). */
const RUN_ANIM_SPEED = 5;

/** Nomes e barras só aparecem até esta distância do jogador (o alvo sempre aparece). */
const LABEL_DISTANCE = 30;
/** Além desta distância da câmera (ou fora da tela) o personagem congela a animação. */
const ANIM_DISTANCE = 50;
/** Quem anda continua animado (a 15 Hz) até esta distância: parado em pose de parado, o monstro que vem de longe deslizava. */
const ANIM_FAR_DISTANCE = 90;
const ANIM_FAR_STEP = 1 / 15;
/** Só personagens até esta distância do jogador projetam sombra. */
const SHADOW_DISTANCE = 35;
/** Folga (m) em volta da distância de visão: quem está na borda não pisca ao andar. */
const VIEW_SLACK = 3;
const BONE_COLOR = "#d9d2c0";
/** Ids dos golpes de área do raid boss nos CombatEvent (zone/raid.go: RaidHitWave…). */
export const RAID_HIT_BASE = 9001;
const NPC_COLOR = "#e0c27a";

/** Velocidade dos projéteis (m/s): o dano só aparece quando o projétil chega. */
const PROJECTILE_SPEED = 26;
/** Golpe de cura do monstro curandeiro (game.HitMonsterHeal: RAID_HIT_BASE + 4). */
const MONSTER_HEAL_HIT = 9005;
/** Lâmina arremessada do atirador. */
const THROWN_COLOR = new Color3(0.86, 0.8, 0.62);
/**
 * Quanto da preparação do golpe ainda aparece (s). O servidor decide o dano no instante em
 * que o golpe começa; o cliente começa o clipe perto do impacto e mostra dano, barra de vida
 * e morte só quando o golpe encosta (ou o projétil chega).
 */
const SWING_LEAD = 0.18;
/** Segundo golpe que chega enquanto o primeiro ainda sobe: encosta este tempo depois (ms). */
const FOLLOW_UP_MS = 140;
/** Quanto a morte espera depois do último impacto, para o golpe ser visto antes da queda (ms). */
const DEATH_AFTER_HIT_MS = 70;
/** Quão rápido alguém se vira para o alvo do golpe (1/s): ~95% em 0,12 s. */
const FACE_RATE = 25;
/** Corpo sumindo (s) e monstro nascendo (s). */
const CORPSE_SINK_S = 0.9;
const SPAWN_FADE_S = 0.5;
/**
 * Pausa de impacto (s) no seu golpe que acerta: congela só a pose do alvo (e a sua, se o
 * golpe é corpo a corpo). Normal, skill, crítico e o golpe que mata.
 */
const HIT_STOP = { hit: 0.045, skill: 0.06, crit: 0.075, kill: 0.08 };
/** Rastro da arma: começa antes do impacto e acaba logo depois (ms). */
const TRAIL_BEFORE_MS = 140;
const TRAIL_AFTER_MS = 60;
/** Rastro dos outros só até esta distância de você (m). */
const TRAIL_OTHERS_M = 20;
/** Distância de um passo correndo (m): uma nuvem de poeira por passo. */
const STEP_M = 1.6;
/** Abaixo desta velocidade (m/s) é andar, não correr: sem poeira. */
const DUST_MIN_SPEED = 2.5;

/** Efeito ativo numa entidade (para os ícones do HUD). */
export interface ActiveEffect {
  effect: Effect;
  /** performance.now() em que acaba. */
  until: number;
  durationMs: number;
  value: number;
}

interface TrackedEffect extends ActiveEffect {
  visual: FxHandle | null;
}

export interface EntityInfo {
  id: number;
  name: string;
  kind: EntityKind;
  level: number;
  hp: number;
  maxHp: number;
  dead: boolean;
  /** NPC: função ("Mercadora"); jogador: título escolhido (conquista, "" = nenhum). */
  title: string;
  /** Chefe (marcador e quadro do alvo diferentes). */
  boss?: boolean;
  /** Monstro raro (named): nome dourado e selo "RARO" no quadro do alvo. */
  rare?: boolean;
  /** Monstros: papel na luta (Entity.role, tunnel#129; 0 = bruto) e se é neutro. */
  role?: MonsterArchetype;
  neutral?: boolean;
  neutralByDay?: boolean;
  /** Jogadores: normal, em combate (roxo) ou com karma (vermelho). */
  pvpState: PvpState;
  /** Jogadores: nome do clã ("" = sem clã). */
  clanName: string;
  /** Jogadores: especialização (id em Welcome.specs; "" = ainda não escolheu). */
  spec: string;
  /** Jogadores: loja pessoal aberta, a placa ("" = nenhuma; tunnel#161). */
  shopTitle: string;
  /** Jogadores: classe ativa (a subclasse em uso, ou a principal, que é a do modelo). */
  cls: string;
}

/**
 * Corpo dos modelos maiores que um personagem normal: área de clique, altura do nome e
 * raio (m) que o servidor desconta do alcance dos golpes. Sem entrada = tamanho normal.
 */
const MODEL_BODY: Record<string, { radius: number; width: number; height: number; head: number }> = {
  [GOLEM_MODEL]: GOLEM_BODY,
};

/** Modelo das Pedras de Teleporte (desenhadas aqui, sem arquivo 3D). */
export const STONE_MODEL = "teleport_stone";
/** Túmulo do chefe de campo (objeto do servidor, 10 min no corpo): lápide com a dica no nome. */
export const TOMB_MODEL = "tomb";
/** A Lápide do Último Regicida (NPC de conversa nas Terras Mortas, tunnel#137): a mesma lápide de pedra do túmulo, mas clicar conversa. */
export const LAPIDE_MODEL = "lapide";
/** Modelo do barco de Tomé (Brejo de Lamagris): o clique do barco é um poste com lanterna e sino; o bote é cenário (world.json, `boat`). */
export const FERRY_MODEL = "ferry_boat";
/** O barqueiro sentado no bote do cais: o corpo base do Bárbaro (o mesmo do Tomé do posto), sem visual do servidor. */
const FERRYMAN_MODEL = "barbarian";
/** Alcance (m) do bote até o poste: o NPC do barco fica no cais e o bote (cenário `boat` do world.json) um pouco adiante. */
const FERRY_BOAT_REACH = 8;
/** Altura (m) do nome do barqueiro acima do assento, e do assento acima do chão do bote; escala do barqueiro. */
const FERRYMAN_LABEL_Y = 1.9;
const FERRYMAN_SEAT_Y = 0.0;
const FERRYMAN_SCALE = 0.9;
/** Área de clique do bote: raio e altura (m). */
const FERRY_BOAT_PICK = { radius: 1.7, height: 1.9 };
/** O que NUNCA vira cápsula: os desenhos próprios (pedra, túmulo, poste do barco) e todo objeto. */
export function drawnInCode(kind: EntityKind, model: string): boolean {
  return kind === EntityKind.OBJECT || model === STONE_MODEL || model === TOMB_MODEL || model === FERRY_MODEL;
}
/** A função (título) dos NPCs do barco, igual à do servidor (zone/teleport.go): o mapa não os lista como pedra. */
export const FERRY_TITLE = "Barco do Tomé";

/** Cores dos nomes por estado (o Babylon GUI não lê variáveis CSS). */
export const NAME_COLORS = {
  flagged: "#c77dff",
  chaotic: "#ff5a4a",
  party: "#7fd0ff",
  clan: "#e8cf8a",
  title: "#cdb8f2",
  rare: "#ffcf4a",
} as const;

/** O que o mapa precisa saber de cada entidade. */
export interface MapMarker {
  id: number;
  x: number;
  z: number;
  kind: EntityKind;
  boss: boolean;
  local: boolean;
  target: boolean;
  /** Para onde está virado: 0 = norte (+Z), crescendo no sentido horário (π/2 = leste). */
  heading: number;
  /** Colega de grupo. */
  party: boolean;
}

interface View extends EntityInfo {
  model: string;
  /** Armas na mão (jogadores): modelo da principal, da outra mão e raridade. */
  weapon: string;
  offhand: string;
  grade: number;
  weaponEnchant: number;
  /** Jogadores: raridade das peças de armadura (remaster: o corpo, as ombreiras, a capa e o elmo seguem). */
  armor: ArmorGrades;
  /** Jogadores: cor do emblema (nome e anel no chão). */
  emblem: string;
  /** Visual do corpo (heróis: jogadores e o barbeiro). */
  look: BodyLook;
  /** Cor do nome sem estado PvP (emblema, local, tipo). */
  baseColor: string;
  clanText: TextBlock;
  /**
   * Jogadores: título sob o nome (null nos outros). Texto fixo: só muda quando chega um
   * PlayerStatus com outro título, nada por quadro.
   */
  titleText: TextBlock | null;
  /** Anel vermelho nos pés de quem tem karma (criado na hora). */
  pkRing: Mesh | null;
  boss: boolean;
  scale: number;
  speed: number;
  /** Posição desenhada no quadro anterior, velocidade desenhada (m/s, suavizada) e se está andando de fato (com histerese). */
  px: number;
  pz: number;
  vis: number;
  walking: boolean;
  /** Tempo (s) desde a última vez que a animação rodou, para animar a 15 Hz quem anda longe (ANIM_FAR_DISTANCE). */
  animAcc: number;
  /** Além da distância de visão (Configurações → Gráficos): nó desligado, sem desenho, nome nem animação. */
  culled: boolean;
  /** Loja pessoal: quando a placa apareceu (ms; 0 = sem loja). Senta e congela a pose. */
  shopSince: number;
  /** Posição do servidor, extrapolada localmente em direção ao destino. */
  server: { x: number; z: number };
  target: { x: number; z: number };
  root: TransformNode;
  visual: TransformNode;
  head: TransformNode;
  /** Barco de Tomé: onde está o bote do cenário em relação ao poste (m) e a área de clique dele. */
  ferryBoat: { dx: number; dz: number; dy: number; pick: TransformNode } | null;
  capsule: Mesh | null;
  model3d: CharacterInstance | null;
  anim: AnimController | null;
  /** Malhas do modelo (para sombra e teste de visibilidade). */
  meshes: AbstractMesh[];
  /** Para as invariantes: sem malha porque o modelo ainda baixa (atualizado em forEachProbe). */
  awaitingModel: boolean;
  castsShadow: boolean;
  label: Rectangle;
  nameText: TextBlock;
  hpBar: Rectangle;
  hpFill: Rectangle;
  lastHurt: number;
  effects: Map<Effect, TrackedEffect>;
  /** Último golpe ainda a caminho deste alvo (performance.now()); dano e morte esperam. */
  impactUntil: number;
  /** Vida que chegou do servidor enquanto um golpe ainda não tinha encostado. */
  pendingHp: HpUpdate | null;
  /** A morte já aparece na tela (o servidor pode ter avisado antes do golpe encostar). */
  shownDead: boolean;
  /** Quando o último golpe deste atacante começou e quando ele encosta. */
  lastSwing: number;
  swingImpact: number;
  lastLand: number;
  /** Para onde virar (golpes): gira suave até lá; null = segue o movimento. */
  faceYaw: number | null;
  /** NPC do remaster que cumprimentou: até quando fica virado para o jogador (performance.now(); 0 = parado como sempre). */
  greetUntil: number;
  /** Monstros: último golpe entre ele e você (performance.now(); a auto-caça vê quem briga). */
  fightAt: number;
  /** Encaixe da mão da arma (rastro do golpe); undefined = ainda não procurado. */
  hand: TransformNode | null | undefined;
  /**
   * Caixa fixa e generosa do corpo, dividida por todas as malhas com esqueleto do modelo
   * (doNotSyncBoundingInfo): o Babylon não refaz a caixa de cada malha a cada quadro; ela
   * só anda junto com a entidade (ver moveBounds). null = sem modelo com esqueleto.
   */
  bounds: BoundingInfo | null;
  boundsAt: Matrix;
  /** Onde a caixa estava da última vez (NaN = refazer). */
  boundsX: number;
  boundsY: number;
  boundsZ: number;
}

/** Jogadores e monstros na tela: criação, movimento suave, animações e combate. */
export class EntityManager {
  private views = new Map<number, View>();
  /** Destino previsto esperando o servidor confirmar (stop = soltou o WASD/joystick). */
  private localPending: { x: number; z: number; at: number; stop?: boolean } | null = null;
  /**
   * O cliente dirige o próprio movimento (WASD, joystick, clique em linha reta): a posição
   * prevista não é sobrescrita pelos snapshots, que chegam velhos. Eles só a corrigem
   * quando discordam de onde estávamos há um ping. Desligado = segue o servidor (caminho
   * em volta de obstáculos, aproximação de ataque, auto-caça), como os outros personagens.
   */
  /** Só para medir o deslize no navegador (?e2e=1&legacymove=1): volta à regra antiga (destino do servidor e velocidade nominal). */
  private legacyMove = new URLSearchParams(location.search).has("legacymove");
  private localDriven = false;
  /** Posições previstas recentes do jogador local, para comparar com snapshots atrasados. */
  private localHistory: { t: number; x: number; z: number }[] = [];
  /** Diferença desenhada − prevista que ainda está sumindo (correções suaves). */
  private drawOffset = { x: 0, z: 0 };
  /** Atraso de um snapshot em relação à previsão: ping de ida e volta + meio tick (ms). */
  private lookbackMs = 150;
  private mats = new Map<string, StandardMaterial>();
  private ringMat: StandardMaterial;
  /** Último evento de skill com área no alvo (para animar uma vez só) e quando ele encosta. */
  private lastSplash = new Map<string, { t: number; at: number }>();
  /** Skills em área/em si: quando o efeito sai (os golpes do mesmo snapshot vão juntos). */
  private castImpact = new Map<string, { t: number; at: number }>();
  /** Golpes, projéteis e mortes esperando a hora de aparecer, em ordem de horário. */
  private stage: { at: number; id: number; fn: () => void }[] = [];
  /** Quando a última morte agendada aparece (o saque espera por ela). */
  private lastDeathShow = 0;
  private selectRing: Mesh;
  private models: CharacterModels | null = null;
  /** Entrada no mundo: enquanto os modelos não chegam, nenhuma entidade vira cápsula (ver holdCapsules). */
  private capsulesHeld = false;
  /** Os botes do cenário (world.json, `boat`): o barqueiro do cais senta no mais perto. */
  private boats: { x: number; z: number }[] = [];
  /** Entidades esperando o download do próprio modelo (golem, chefes, NPCs do remaster): invisíveis até chegar. */
  private lazyWaits = new Map<number, Promise<boolean>>();
  private targetId = 0;
  private catalog = new Map<number, SkillInfo>();
  /** Passos do jogador local correndo (poeira): fração do passo atual e o pé. */
  private stepPhase = 0;
  private stepSide = 1;

  constructor(
    private scene: Scene,
    private gui: AdvancedDynamicTexture,
    readonly localId: number,
    /** Retângulo jogável (o mapa cresceu só para o sul). */
    private bounds: Bounds,
    private fx: Fx,
    private shadows: ShadowGenerator,
    private collision: Collision,
    /** Áreas de clique (cilindros testados em código, sem malha na cena). */
    private picks: PickAreas,
  ) {
    this.ringMat = new StandardMaterial("localRing", scene);
    this.ringMat.emissiveColor = new Color3(0.95, 0.8, 0.4);
    this.ringMat.disableLighting = true;
    this.ringMat.alpha = 0.8;

    const selMat = new StandardMaterial("selectRing", scene);
    selMat.emissiveColor = new Color3(0.95, 0.25, 0.2);
    selMat.disableLighting = true;
    this.selectRing = MeshBuilder.CreateTorus("selectRing", { diameter: 1.4, thickness: 0.07, tessellation: 32 }, scene);
    this.selectRing.material = selMat;
    this.selectRing.isPickable = false;
    this.selectRing.setEnabled(false);
  }

  get count(): number {
    let n = 0;
    for (const v of this.views.values()) if (v.kind === EntityKind.PLAYER) n++;
    return n;
  }

  get local(): View | undefined {
    return this.views.get(this.localId);
  }

  get localDead(): boolean {
    return this.local?.dead ?? false;
  }

  info(id: number): EntityInfo | undefined {
    return this.views.get(id);
  }

  /** Visual do corpo de alguém (heróis). */
  lookOf(id: number): BodyLook | undefined {
    return this.views.get(id)?.look;
  }

  isNpc(id: number): boolean {
    return this.views.get(id)?.kind === EntityKind.NPC;
  }

  isPlayer(id: number): boolean {
    return this.views.get(id)?.kind === EntityKind.PLAYER;
  }

  /** Colegas de grupo (nome azul-claro, aliados). */
  private partyIds = new Set<number>();

  setPartyIds(ids: Iterable<number>): void {
    const before = this.partyIds;
    this.partyIds = new Set(ids);
    for (const id of new Set([...before, ...this.partyIds])) {
      const v = this.views.get(id);
      if (v) this.refreshName(v);
    }
  }

  inParty(id: number): boolean {
    return this.partyIds.has(id);
  }

  /**
   * Arena (tunnel#81): durante a partida, o time do jogador (aliados) e o outro (alvos sem
   * Ctrl). null = fora de partida, valem o grupo e o clã.
   */
  private arenaTeams: { mine: Set<number>; foes: Set<number> } | null = null;

  setArenaTeams(mine: number[] | null, foes: number[] = []): void {
    this.arenaTeams = mine ? { mine: new Set(mine), foes: new Set(foes) } : null;
  }

  /** Aliado: do grupo ou do mesmo clã (nunca é atacado). Na arena, só o time. */
  isAlly(id: number): boolean {
    if (id === this.localId) return true;
    if (this.arenaTeams) return this.arenaTeams.mine.has(id);
    if (this.partyIds.has(id)) return true;
    const v = this.views.get(id);
    const me = this.local;
    return !!v && !!me && v.kind === EntityKind.PLAYER && v.clanName !== "" && v.clanName === me.clanName;
  }

  /**
   * Dá para atacar? Monstros vivos sempre; jogadores vivos que não são aliados, se já
   * forem hostis (roxo/vermelho) ou com force (Ctrl). O servidor confere o resto
   * (nível mínimo, cidade).
   */
  canAttack(id: number, force: boolean): boolean {
    const v = this.views.get(id);
    if (!v || v.dead) return false;
    if (v.kind === EntityKind.MONSTER) return true;
    if (v.kind !== EntityKind.PLAYER || this.isAlly(id)) return false;
    if (this.arenaTeams?.foes.has(id)) return true; // arena: o outro time é alvo sem Ctrl
    return force || v.pvpState !== PvpState.NORMAL;
  }

  /** Estado PvP ou clã de um jogador mudou. */
  setStatus(s: PlayerStatus): void {
    const v = this.views.get(s.id);
    if (!v) return;
    v.pvpState = s.pvpState;
    v.clanName = s.clanName;
    v.title = s.title;
    v.spec = s.spec;
    if (s.shopTitle !== v.shopTitle) {
      v.shopTitle = s.shopTitle;
      v.shopSince = s.shopTitle ? performance.now() : 0;
      if (!s.shopTitle) v.anim?.endEmote(); // a loja fechou: levanta
    }
    // Trocou de classe (subclasse): a classe e o nível novos (o modelo continua o da principal).
    if (s.activeClass) v.cls = s.activeClass;
    if (s.level > 0 && s.level !== v.level) {
      v.level = s.level;
      this.refreshLabel(v);
    }
    this.refreshName(v);
    // O clã do jogador local mudou: os aliados de clã mudam de cor também.
    if (s.id === this.localId) for (const o of this.views.values()) if (o !== v) this.refreshName(o);
  }

  /** Nó raiz (posição no chão) de uma entidade. */
  rootOf(id: number): TransformNode | undefined {
    return this.views.get(id)?.root;
  }

  /** Ponto acima da cabeça (para textos flutuantes). */
  headOf(id: number): TransformNode | undefined {
    return this.views.get(id)?.head;
  }

  /** Corpo de uma entidade para efeitos presos nela (aura, coroa do raid boss). */
  bodyOf(id: number): FxBody | undefined {
    return this.views.get(id);
  }

  /** Malhas do modelo 3D de uma entidade (vazio enquanto o modelo carrega). */
  meshesOf(id: number): AbstractMesh[] {
    return this.views.get(id)?.model3d?.meshes() ?? [];
  }

  /** A arma ou o visual de alguém mudou. */
  setAppearance(a: Appearance): void {
    const v = this.views.get(a.id);
    if (!v) return;
    v.weapon = a.weapon;
    v.offhand = a.offhand;
    v.grade = a.weaponGrade;
    v.weaponEnchant = a.weaponEnchant;
    v.armor = toArmorGrades(a.armor);
    if (a.look) v.look = toBodyLook(a.look);
    if (!v.model3d) return;
    v.model3d.setLook(v.look);
    this.setShadow(v, false); // as malhas mudam; a sombra volta no próximo quadro
    v.model3d.setWeapons(v.weapon, v.offhand, v.grade, v.weaponEnchant);
    v.model3d.setArmor(v.armor);
    v.meshes = v.model3d.meshes();
    this.notPickable(v.meshes);
    this.fixBounds(v); // a cabeça nova é outra malha
  }

  isEnemy(id: number): boolean {
    const v = this.views.get(id);
    return !!v && v.kind === EntityKind.MONSTER && !v.dead;
  }

  /** Quando os modelos 3D terminam de carregar: troca as cápsulas pelos modelos. */
  setModels(models: CharacterModels): void {
    this.models = models;
    models.onModelReady = () => {
      for (const v of this.views.values()) if (v.kind !== EntityKind.OBJECT) this.attachModel(v);
    };
    for (const v of this.views.values()) if (v.kind !== EntityKind.OBJECT) this.attachModel(v);
    this.releaseCapsules();
  }

  /** O bote do cenário mais perto de (x, z), até FERRY_BOAT_REACH m (null = nenhum). */
  private nearestBoat(x: number, z: number): { x: number; z: number } | null {
    let best: { x: number; z: number } | null = null;
    let bd = FERRY_BOAT_REACH;
    for (const b of this.boats) {
      const d = Math.hypot(b.x - x, b.z - z);
      if (d < bd) {
        bd = d;
        best = b;
      }
    }
    return best;
  }

  /** Os botes do cenário (o barco de Tomé senta o barqueiro no mais perto do poste). */
  setBoats(boats: { x: number; z: number }[]): void {
    this.boats = boats;
  }

  /** Entrada no mundo: segura as cápsulas até os modelos carregarem (ou falharem: releaseCapsules). */
  holdCapsules(): void {
    this.capsulesHeld = true;
  }

  /** Solta as cápsulas: quem ficou sem modelo (e não espera um download) vira cápsula, o último recurso. */
  releaseCapsules(): void {
    this.capsulesHeld = false;
    for (const v of this.views.values()) {
      if (drawnInCode(v.kind, v.model) || v.model3d || v.capsule) continue;
      if (this.wantsCapsule(v)) v.capsule = this.createCapsule(v);
    }
  }

  private wantsCapsule(v: View): boolean {
    return !this.capsulesHeld && !this.lazyWaits.has(v.id) && !drawnInCode(v.kind, v.model);
  }

  /** Resolve quando nenhuma entidade espera mais o download do próprio modelo (só esses downloads, não o pacote todo). */
  async modelsSettled(): Promise<void> {
    while (this.lazyWaits.size) await Promise.allSettled([...this.lazyWaits.values()]);
  }

  setTarget(id: number): void {
    this.targetId = id;
    const v = this.views.get(id);
    if (v) {
      this.selectRing.parent = v.root;
      this.selectRing.position.set(0, 0.1, 0);
      this.selectRing.scaling.setAll(v.scale);
      this.selectRing.setEnabled(true);
    } else {
      this.selectRing.parent = null;
      this.selectRing.setEnabled(false);
    }
    for (const view of this.views.values()) this.refreshHpBar(view);
  }

  /** fade: monstro que acabou de nascer aparece aos poucos. */
  add(e: Entity, fade = false): void {
    if (this.views.has(e.id)) return;
    noteSpawn(e.id); // dessincronia: sumiu e voltou em menos de 1 s (tunnel#122)
    const isLocal = e.id === this.localId;
    if (isLocal) this.resetLocalPrediction();
    const scale = e.scale > 0 ? e.scale : 1;

    const root = new TransformNode(`e${e.id}`, this.scene);
    root.position.set(e.x, heightAt(e.x, e.z), e.z);
    const visual = new TransformNode(`visual${e.id}`, this.scene);
    visual.parent = root;

    // Área de clique estável: um cilindro nos pés, testado em código (world/pick.ts). O
    // modelo animado não serve, porque o picking de malha com esqueleto usa a pose parada e
    // braços/pernas entram e saem do cursor.
    const isObject = e.kind === EntityKind.OBJECT;
    const body = MODEL_BODY[e.model];
    if (!isObject || e.model === TOMB_MODEL) {
      // Monstros grandes (golem) ganham uma área do tamanho do corpo, senão o clique
      // passa pelos ombros e braços sem selecionar.
      this.picks.addEntity(e.id, root, ((body?.width ?? 1.2) * scale) / 2, (body?.height ?? 2.1) * scale);
    }
    // O barco de Tomé: o clique vale no bote (e no barqueiro nele), não só no poste do cais.
    let ferryBoat: View["ferryBoat"] = null;
    if (e.model === FERRY_MODEL) {
      const b = this.nearestBoat(e.x, e.z);
      if (b) {
        const pick = new TransformNode(`fpick${e.id}`, this.scene);
        pick.position.set(b.x, heightAt(b.x, b.z), b.z);
        this.picks.addEntityExtra(e.id, pick, FERRY_BOAT_PICK.radius, FERRY_BOAT_PICK.height);
        ferryBoat = { dx: b.x - e.x, dz: b.z - e.z, dy: heightAt(b.x, b.z) - heightAt(e.x, e.z), pick };
      }
    }

    // Anel no chão: na cor do emblema para jogadores (o seu, mais forte), dourado sem emblema.
    const emblem = e.kind === EntityKind.PLAYER && /^#[0-9a-f]{6}$/i.test(e.emblem) ? e.emblem : "";
    if (isLocal || emblem) {
      const ring = MeshBuilder.CreateTorus(`ring${e.id}`, { diameter: isLocal ? 1.1 : 1.0, thickness: isLocal ? 0.06 : 0.04, tessellation: 32 }, this.scene);
      ring.parent = root;
      ring.position.y = 0.1;
      ring.material = emblem ? this.ringMaterial(emblem, isLocal ? 0.85 : 0.5) : this.ringMat;
      ring.isPickable = false;
    }

    const head = new TransformNode(`head${e.id}`, this.scene);
    head.parent = root;
    head.position.y = (body?.head ?? 2.25) * scale;

    // Nome (+ nível para monstros) e barra de vida acima da cabeça.
    const label = new Rectangle(`label${e.id}`);
    label.adaptWidthToChildren = true;
    label.adaptHeightToChildren = true;
    label.thickness = 0;
    const stack = new StackPanel();
    stack.isVertical = true;
    label.addControl(stack);
    const nameText = new TextBlock();
    nameText.resizeToFit = true;
    nameText.fontSize = 13;
    nameText.paddingLeft = nameText.paddingRight = "6px";
    const baseColor = emblem
      ? lighten(emblem, 0.45)
      : isLocal
        ? "#f0d79a"
        : e.kind === EntityKind.MONSTER
        ? e.rare
          ? NAME_COLORS.rare
          : "#ffb4a8"
        : e.kind === EntityKind.NPC
          ? "#a8e6a0"
          : e.kind === EntityKind.OBJECT
            ? "#bcd4ff"
            : "#e6ecff";
    nameText.color = baseColor;
    if (e.rare) nameText.fontWeight = "bold"; // raro: nome dourado, em negrito
    // Contorno em vez de shadowBlur: o desfoque do canvas 2D é caro e a GUI redesenha muito.
    nameText.outlineWidth = 3;
    nameText.outlineColor = "rgba(0,0,0,0.85)";
    stack.addControl(nameText);
    let titleText: TextBlock | null = null;
    if (e.kind === EntityKind.PLAYER) {
      titleText = new TextBlock();
      titleText.resizeToFit = true;
      titleText.fontSize = 11;
      titleText.fontStyle = "italic";
      titleText.color = NAME_COLORS.title;
      titleText.outlineWidth = 3;
      titleText.outlineColor = "rgba(0,0,0,0.85)";
      titleText.isVisible = false;
      stack.addControl(titleText);
    }
    const clanText = new TextBlock();
    clanText.resizeToFit = true;
    clanText.fontSize = 11;
    clanText.color = NAME_COLORS.clan;
    clanText.outlineWidth = 3;
    clanText.outlineColor = "rgba(0,0,0,0.85)";
    clanText.isVisible = false;
    stack.addControl(clanText);
    const hpBar = new Rectangle();
    hpBar.width = "64px";
    hpBar.height = "6px";
    hpBar.thickness = 1;
    hpBar.color = "rgba(0,0,0,0.8)";
    hpBar.background = "rgba(20,10,10,0.75)";
    hpBar.paddingTop = "2px";
    const hpFill = new Rectangle();
    hpFill.horizontalAlignment = Rectangle.HORIZONTAL_ALIGNMENT_LEFT;
    hpFill.thickness = 0;
    hpFill.background = e.kind === EntityKind.MONSTER ? "#d2493c" : "#4caf6a";
    hpFill.width = 1;
    hpBar.addControl(hpFill);
    stack.addControl(hpBar);
    this.gui.addControl(label);
    label.linkWithMesh(head);
    label.linkOffsetY = -6;

    const v: View = {
      id: e.id, name: e.name, kind: e.kind, level: e.level, hp: e.hp, maxHp: e.maxHp, dead: e.dead, title: e.title,
      model: e.model, boss: e.boss, rare: e.rare, role: e.role?.archetype, neutral: e.role?.neutral, neutralByDay: e.role?.neutralByDay, scale, speed: e.speed, weapon: e.weapon, offhand: e.offhand, grade: e.weaponGrade, armor: toArmorGrades(e.armor), emblem, look: toBodyLook(e.look),
      weaponEnchant: e.weaponEnchant, pvpState: e.pvpState, clanName: e.clanName, spec: e.spec, cls: e.activeClass || e.model, baseColor, clanText, titleText, pkRing: null,
      shopTitle: e.shopTitle, shopSince: e.shopTitle ? performance.now() : 0,
      px: e.x, pz: e.z, vis: 0, walking: false, animAcc: 0, culled: false,
      server: { x: e.x, z: e.z },
      target: { x: e.targetX, z: e.targetZ },
      root, visual, head, ferryBoat, capsule: null, model3d: null, anim: null, meshes: [], awaitingModel: false, castsShadow: false,
      label, nameText, hpBar, hpFill, lastHurt: 0, effects: new Map(),
      impactUntil: 0, pendingHp: null, shownDead: e.dead, lastSwing: 0, swingImpact: 0, lastLand: 0, faceYaw: null, greetUntil: 0, fightAt: 0, hand: undefined,
      bounds: null, boundsAt: Matrix.Identity(), boundsX: NaN, boundsY: NaN, boundsZ: NaN,
    };
    this.views.set(e.id, v);
    // NPCs ficam parados, virados para o centro da praça.
    if (e.kind === EntityKind.NPC) visual.rotation.y = Math.atan2(-e.x, -e.z);
    this.refreshLabel(v);
    this.refreshName(v);
    this.refreshHpBar(v);

    if (e.model === TOMB_MODEL || e.model === LAPIDE_MODEL) {
      this.buildTomb(v);
      return;
    }
    if (isObject) {
      this.buildBanner(v);
      return;
    }
    if (e.model === STONE_MODEL) {
      this.buildStone(v);
      return;
    }
    if (e.model === FERRY_MODEL) {
      this.buildFerryPost(v);
      return;
    }
    if (this.models) this.attachModel(v);
    if (!v.model3d && this.wantsCapsule(v)) v.capsule = this.createCapsule(v);
    if (v.dead) this.showDead(v, true);
    else if (fade && e.kind === EntityKind.MONSTER) this.fadeIn(v);
  }

  /** Túmulo clicável: nome em cima e a dica (hora da morte e quem mais bateu) na linha de baixo. */
  tomb(id: number): { name: string; title: string } | null {
    const v = this.views.get(id);
    return v?.model === TOMB_MODEL ? { name: v.name, title: v.title } : null;
  }

  /** Lápide de pedra sobre um montinho de terra, com uma luz fria apagando devagar. */
  private buildTomb(v: View): void {
    const mound = MeshBuilder.CreateSphere(`tmound${v.id}`, { diameterX: 1.7, diameterY: 0.5, diameterZ: 2.4, segments: 8 }, this.scene);
    mound.parent = v.visual;
    mound.position.y = 0.05;
    mound.material = this.material("#4d4033");
    const slab = MeshBuilder.CreateBox(`tslab${v.id}`, { width: 0.95, height: 1.25, depth: 0.22 }, this.scene);
    slab.parent = v.visual;
    slab.position.set(0, 0.7, -0.75);
    slab.material = this.material("#8a8f96");
    const cap = MeshBuilder.CreateCylinder(`tcap${v.id}`, { height: 0.22, diameter: 0.95, tessellation: 14 }, this.scene);
    cap.parent = v.visual;
    cap.rotation.x = Math.PI / 2;
    cap.position.set(0, 1.32, -0.75);
    cap.material = this.material("#8a8f96");
    const cross = MeshBuilder.CreateBox(`tcross${v.id}`, { width: 0.1, height: 0.5, depth: 0.05 }, this.scene);
    cross.parent = v.visual;
    cross.position.set(0, 0.85, -0.87);
    cross.material = this.material("#3a3d42");
    const bar = MeshBuilder.CreateBox(`tbar${v.id}`, { width: 0.34, height: 0.1, depth: 0.05 }, this.scene);
    bar.parent = v.visual;
    bar.position.set(0, 0.95, -0.87);
    bar.material = this.material("#3a3d42");
    for (const m of [mound, slab, cap, cross, bar]) m.isPickable = false;
    v.meshes = [mound, slab, cap];
    v.head.position.y = 1.9;
    v.nameText.color = "#d8d2c0";
    v.clanText.text = v.title; // a dica fica sempre à vista, abaixo do nome
    v.clanText.color = "#b9b29c";
    v.clanText.isVisible = !!v.title;
  }

  /** Pedra de Teleporte: monólito com runas acesas e núcleo de luz (ver teleport-stone.ts). */
  private buildStone(v: View): void {
    v.meshes = buildTeleportStone(this.scene, this.fx, v.id, v.visual, v.root).shadowMeshes;
  }

  /** Barco de Tomé: um poste de madeira com a lanterna verde e o sino do barqueiro (o bote em si é cenário do mapa). */
  private buildFerryPost(v: View): void {
    const pole = MeshBuilder.CreateCylinder(`fpole${v.id}`, { height: 2.4, diameterTop: 0.1, diameterBottom: 0.16, tessellation: 8 }, this.scene);
    pole.parent = v.visual;
    pole.position.y = 1.2;
    pole.material = this.material("#5b4630");
    const arm = MeshBuilder.CreateBox(`farm${v.id}`, { width: 0.9, height: 0.07, depth: 0.07 }, this.scene);
    arm.parent = v.visual;
    arm.position.set(0.4, 2.25, 0);
    arm.material = this.material("#5b4630");
    const lamp = MeshBuilder.CreateSphere(`flamp${v.id}`, { diameter: 0.3, segments: 8 }, this.scene);
    lamp.parent = v.visual;
    lamp.position.set(0.8, 2.0, 0);
    const glow = new StandardMaterial(`flamp-mat${v.id}`, this.scene);
    glow.diffuseColor = Color3.FromHexString("#9fe870");
    glow.emissiveColor = Color3.FromHexString("#9fe870").scale(0.85);
    lamp.material = glow;
    const bell = MeshBuilder.CreateCylinder(`fbell${v.id}`, { height: 0.26, diameterTop: 0.12, diameterBottom: 0.3, tessellation: 10 }, this.scene);
    bell.parent = v.visual;
    bell.position.set(-0.3, 2.1, 0);
    bell.material = this.material("#c9a24a");
    for (const m of [pole, arm, lamp, bell]) m.isPickable = false;
    v.meshes = [pole, arm, lamp, bell];
    // O nome fica sobre o bote (onde está o barqueiro), não sobre o poste: não se mistura com o de quem espera no cais.
    if (v.ferryBoat) v.head.position.set(v.ferryBoat.dx, v.ferryBoat.dy + FERRYMAN_LABEL_Y, v.ferryBoat.dz);
    else v.head.position.y = FERRYMAN_LABEL_Y;
    if (this.models) this.seatFerryman(v);
  }

  /** O barqueiro de Tomé, sentado no bote do cenário (sem bote por perto, o poste fica sozinho). */
  private seatFerryman(v: View): void {
    const b = v.ferryBoat;
    if (!b || !this.models || v.model3d) return;
    const inst = this.models.instantiate(FERRYMAN_MODEL, String(v.id));
    if (!inst) return;
    const seat = new TransformNode(`fseat${v.id}`, this.scene);
    seat.parent = v.root;
    seat.position.set(b.dx, b.dy + FERRYMAN_SEAT_Y, b.dz);
    seat.rotation.y = v.visual.rotation.y;
    inst.root.parent = seat;
    inst.root.scaling.scaleInPlace(FERRYMAN_SCALE);
    const meshes = inst.meshes();
    this.notPickable(meshes);
    // Fora do corpo fixo do poste (fixBounds): o barqueiro está a metros dele, e a caixa o cortaria da tela.
    for (const m of meshes) m.alwaysSelectAsActiveMesh = true;
    v.meshes = [...v.meshes, ...meshes];
    v.model3d = inst;
    v.anim = new AnimController(inst.anims, inst.resetPose, inst.impacts);
    v.anim.setBodyScale(FERRYMAN_SCALE);
    v.anim.legacy = this.legacyMove;
    if (this.lazyWaits.has(v.id)) this.fadeIn(v);
  }

  /** Estandarte: mastro, bandeira com a cor da ordem e o círculo da área (8 m). */
  private buildBanner(v: View): void {
    const pole = MeshBuilder.CreateCylinder(`pole${v.id}`, { height: 3.2, diameter: 0.12, tessellation: 8 }, this.scene);
    pole.parent = v.visual;
    pole.position.y = 1.6;
    pole.material = this.material("#6b4a2a");
    const top = MeshBuilder.CreateSphere(`poletop${v.id}`, { diameter: 0.24, segments: 8 }, this.scene);
    top.parent = v.visual;
    top.position.y = 3.25;
    top.material = this.material("#e2c070");
    const flag = MeshBuilder.CreatePlane(`flag${v.id}`, { width: 1.1, height: 1.4, sideOrientation: Mesh.DOUBLESIDE }, this.scene);
    flag.parent = v.visual;
    flag.position.set(0.6, 2.45, 0);
    flag.material = this.material("#3e5da8");
    const emblem = MeshBuilder.CreateDisc(`emblem${v.id}`, { radius: 0.28, tessellation: 20, sideOrientation: Mesh.DOUBLESIDE }, this.scene);
    emblem.parent = flag;
    emblem.position.z = -0.01;
    emblem.material = this.material("#e2c070");
    const area = MeshBuilder.CreateTorus(`area${v.id}`, { diameter: 16, thickness: 0.08, tessellation: 64 }, this.scene);
    area.parent = v.root;
    // O círculo de 16 m sobe o que as colinas pedirem, para não afundar.
    area.position.y = 0.05 + groundClearance(terrain(), v.root.position.x, v.root.position.z, 8);
    const areaMat = new StandardMaterial(`areaMat${v.id}`, this.scene);
    areaMat.emissiveColor = new Color3(0.45, 0.7, 1);
    areaMat.disableLighting = true;
    areaMat.alpha = 0.6;
    area.material = areaMat;
    const meshes = [pole, top, flag, emblem, area];
    this.notPickable(meshes);
    v.meshes = [pole, top, flag];
    // A bandeira balança.
    const t0 = performance.now();
    const wave = this.scene.onBeforeRenderObservable.add(() => {
      if (flag.isDisposed()) {
        this.scene.onBeforeRenderObservable.remove(wave);
        return;
      }
      const t = (performance.now() - t0) / 1000;
      flag.rotation.y = Math.sin(t * 2.2) * 0.25;
      areaMat.alpha = 0.45 + 0.15 * Math.sin(t * 3);
    });
    this.fx.bannerPlant(v.root.position.clone(), new Color3(0.45, 0.7, 1));
  }

  remove(id: number): void {
    const v = this.views.get(id);
    if (!v) return;
    noteDespawn(id);
    this.setShadow(v, false);
    if (this.targetId === id) this.setTarget(0);
    if (this.selectRing.parent === v.root) this.selectRing.parent = null;
    this.clearEffects(v);
    this.dropStage(id);
    v.label.dispose();
    this.views.delete(id);
    this.picks.removeEntity(id);
    // Corpo de monstro: afunda e desbota antes de sumir.
    if (v.shownDead && v.kind === EntityKind.MONSTER) this.sinkAndDispose(v);
    else this.disposeView(v);
  }

  private disposeView(v: View): void {
    this.lazyWaits.delete(v.id);
    v.model3d?.dispose();
    v.ferryBoat?.pick.dispose();
    // Materiais são compartilhados (modelo, anel, efeitos): só as malhas são descartadas.
    v.root.dispose(false, false);
  }

  /** Corpo afundando no chão e desbotando (CORPSE_SINK_S), depois descartado. */
  private sinkAndDispose(v: View): void {
    for (const m of v.root.getChildMeshes(false)) m.isPickable = false;
    const meshes = v.meshes.length ? v.meshes : v.root.getChildMeshes(false);
    let t = 0;
    const y0 = v.root.position.y; // o chão onde caiu (o relevo)
    const obs = this.scene.onBeforeRenderObservable.add(() => {
      t += this.scene.getEngine().getDeltaTime() / 1000;
      const k = Math.min(1, t / CORPSE_SINK_S);
      v.root.position.y = y0 - 0.5 * v.scale * k * k;
      for (const m of meshes) m.visibility = 1 - k;
      if (k >= 1) {
        this.scene.onBeforeRenderObservable.remove(obs);
        this.disposeView(v);
      }
    });
  }

  /** Monstro que acabou de nascer aparece aos poucos. */
  private fadeIn(v: View): void {
    const meshes = v.meshes;
    if (!meshes.length) return;
    let t = 0;
    for (const m of meshes) m.visibility = 0;
    const obs = this.scene.onBeforeRenderObservable.add(() => {
      t += this.scene.getEngine().getDeltaTime() / 1000;
      const k = Math.min(1, t / SPAWN_FADE_S);
      for (const m of meshes) if (!m.isDisposed()) m.visibility = k;
      if (k >= 1 || v.root.isDisposed()) this.scene.onBeforeRenderObservable.remove(obs);
    });
  }

  applyState(s: EntityState): void {
    const v = this.views.get(s.id);
    if (!v) return;
    if (s.teleport) {
      // Piscar/Investida: pula direto (sem deslizar), com clarão nos dois pontos.
      const from = v.root.position.clone();
      v.root.position.set(s.x, heightAt(s.x, s.z), s.z);
      v.server.x = v.target.x = s.x;
      v.server.z = v.target.z = s.z;
      if (s.id === this.localId) this.resetLocalPrediction();
      this.fx.as(s.id === this.localId, () => this.fx.blinkFlash(from, v.root.position.clone(), new Color3(0.7, 0.5, 1)));
      if (s.speed > 0) v.speed = s.speed;
      return;
    }
    if (s.speed > 0) v.speed = s.speed;
    if (s.id === this.localId && this.localDriven && this.reconcileLocal(v, s)) return;
    v.server.x = s.x;
    v.server.z = s.z;

    // Jogador local: enquanto o servidor não confirma o destino que já previmos,
    // não deixa um snapshot antigo "puxar" o personagem de volta.
    if (s.id === this.localId && this.localPending) {
      const pend = this.localPending;
      const expired = performance.now() - pend.at > PENDING_TIMEOUT_MS;
      if (pend.stop) {
        // Parando: o servidor para onde estiver, sempre um pouco à frente do que vemos.
        // Até o snapshot dele parado chegar, segue a posição dele sem andar para trás.
        const stopped = Math.abs(s.targetX - s.x) < 0.05 && Math.abs(s.targetZ - s.z) < 0.05;
        if (!stopped && !expired) {
          v.target.x = s.x;
          v.target.z = s.z;
          return;
        }
      } else {
        const confirmed = Math.abs(s.targetX - pend.x) < 0.05 && Math.abs(s.targetZ - pend.z) < 0.05;
        if (!confirmed && !expired) return;
      }
      this.localPending = null;
    }
    v.target.x = s.targetX;
    v.target.z = s.targetZ;
  }

  /**
   * Snapshot do jogador local enquanto o cliente dirige. Devolve false quando o servidor
   * assumiu o movimento (alvo que não é o nosso, ou diferença grande): aí o snapshot é
   * aplicado do jeito normal e o modo cliente desliga.
   */
  private reconcileLocal(v: View, s: EntityState): boolean {
    const now = performance.now();
    const near = (ax: number, az: number, bx: number, bz: number) => Math.abs(ax - bx) < 0.05 && Math.abs(az - bz) < 0.05;
    const serverStopped = near(s.targetX, s.targetZ, s.x, s.z);
    const pend = this.localPending;
    let ours: boolean;
    if (pend && now - pend.at <= PENDING_TIMEOUT_MS) {
      // Esperando o servidor: snapshots antigos ainda mostram o destino anterior.
      ours = true;
      if (pend.stop ? serverStopped : near(s.targetX, s.targetZ, pend.x, pend.z)) this.localPending = null;
    } else {
      this.localPending = null;
      const weStopped = near(v.target.x, v.target.z, v.server.x, v.server.z);
      ours = near(s.targetX, s.targetZ, v.target.x, v.target.z) || (serverStopped && weStopped);
    }
    if (!ours) {
      this.localDriven = false;
      return false;
    }
    // O snapshot mostra onde o servidor estava; a previsão estava lá há um ping.
    const past = this.localPredictionAt(now - this.lookbackMs);
    const ex = s.x - past.x;
    const ez = s.z - past.z;
    const err = Math.hypot(ex, ez);
    noteCorrection(err); // dessincronia (tunnel#122): só conta acima de 0,5 m
    if (err > LOCAL_SNAP) {
      noteResync();
      this.localDriven = false;
      return false;
    }
    if (err > LOCAL_DEADZONE) {
      // Move a previsão (e o histórico, para não corrigir de novo no próximo snapshot);
      // a diferença entra no drawOffset e some aos poucos da tela.
      const stopped = near(v.target.x, v.target.z, v.server.x, v.server.z);
      v.server.x += ex;
      v.server.z += ez;
      if (stopped) {
        v.target.x = v.server.x;
        v.target.z = v.server.z;
        if (this.localPending) {
          this.localPending.x = v.server.x;
          this.localPending.z = v.server.z;
        }
      }
      for (const h of this.localHistory) {
        h.x += ex;
        h.z += ez;
      }
      this.drawOffset.x -= ex;
      this.drawOffset.z -= ez;
    }
    return true;
  }

  /** Posição prevista do jogador local no instante t (interpolada no histórico). */
  private localPredictionAt(t: number): { x: number; z: number } {
    const h = this.localHistory;
    if (!h.length) return this.local?.server ?? { x: 0, z: 0 };
    if (t <= h[0].t) return h[0];
    for (let i = h.length - 1; i >= 0; i--) {
      if (h[i].t <= t) {
        const a = h[i];
        const b = h[i + 1];
        if (!b) return a;
        const k = (t - a.t) / Math.max(1e-6, b.t - a.t);
        return { x: a.x + (b.x - a.x) * k, z: a.z + (b.z - a.z) * k };
      }
    }
    return h[0];
  }

  /** Começa a dirigir pelo cliente (sem pulo: a diferença para o desenho vira drawOffset). */
  private driveLocal(v: View): void {
    if (this.localDriven) return;
    this.localDriven = true;
    this.drawOffset.x = v.root.position.x - v.server.x;
    this.drawOffset.z = v.root.position.z - v.server.z;
  }

  /** Teleporte, morte, renascer, entrar: volta a seguir o servidor do zero. */
  private resetLocalPrediction(): void {
    this.localPending = null;
    this.localDriven = false;
    this.localHistory.length = 0;
    this.drawOffset.x = this.drawOffset.z = 0;
  }

  /** Ping de ida e volta (ms) e ticks por segundo do servidor. */
  setLatency(rttMs: number, tickRate: number): void {
    if (rttMs > 0) this.lookbackMs = rttMs + (tickRate > 0 ? 500 / tickRate : 25);
  }

  applyHp(u: HpUpdate): void {
    const v = this.views.get(u.id);
    if (!v) return;
    // Um golpe ainda não encostou: a barra só desce quando ele chegar.
    if (v.impactUntil > performance.now()) {
      v.pendingHp = u;
      return;
    }
    v.pendingHp = null;
    this.setHp(v, u);
  }

  private setHp(v: View, u: HpUpdate): void {
    if (u.hp < v.hp) v.lastHurt = performance.now();
    v.hp = u.hp;
    v.maxHp = u.maxHp;
    this.refreshHpBar(v);
  }

  /** Previsão: o jogador local começa a andar na hora, sem esperar o servidor. */
  predictLocalMove(x: number, z: number): { x: number; z: number } | null {
    const v = this.local;
    if (!v || v.dead) return null;
    const cx = clamp(x, this.bounds.minX, this.bounds.maxX);
    const cz = clamp(z, this.bounds.minZ, this.bounds.maxZ);
    // Só prevê em linha reta se não houver obstáculo; senão espera o caminho do servidor
    // (que contorna casas e muralhas) chegar no próximo snapshot.
    // Atordoado ou enraizado o servidor não anda: prever faria o personagem sair e voltar.
    const now = performance.now();
    const held = [Effect.STUN, Effect.ROOT].some((k) => (v.effects.get(k)?.until ?? 0) > now);
    if (!held && this.collision.segmentClear(v.server.x, v.server.z, cx, cz)) {
      this.driveLocal(v);
      v.target.x = cx;
      v.target.z = cz;
      this.localPending = { x: cx, z: cz, at: performance.now() };
    } else {
      // O servidor vai contornar o obstáculo: seguimos o caminho dele.
      this.localPending = null;
      this.localDriven = false;
    }
    return { x: cx, z: cz };
  }

  /** Todas as skills do jogo (para desenhar as skills dos outros jogadores). */
  setSkillCatalog(list: SkillInfo[]): void {
    for (const sk of list) this.catalog.set(sk.id, sk);
  }

  skill(id: number): SkillInfo | undefined {
    return this.catalog.get(id);
  }

  onCombat(ev: CombatEvent): void {
    const src = this.views.get(ev.sourceId);
    const dst = this.views.get(ev.targetId);
    if (!dst) return;
    const now = performance.now();
    // Cura do monstro curandeiro (tunnel#129): o número verde sai no aliado curado.
    if (ev.skillId === MONSTER_HEAL_HIT) {
      if (src && src !== dst) {
        this.face(src, dst);
        src.anim?.play("skill", { lead: SWING_LEAD });
      }
      this.at(Math.max(now, dst.impactUntil), dst.id, () => {
        if (ev.heal && showDamage()) this.fx.number(dst.head, "heal", `+${ev.heal}`);
      });
      return;
    }
    // Quem briga com você (a auto-caça prefere e não pula, mesmo neutro).
    if (src && ev.targetId === this.localId && src.kind === EntityKind.MONSTER) src.fightAt = now;
    if (ev.sourceId === this.localId && dst.kind === EntityKind.MONSTER) dst.fightAt = now;
    // Veneno, queimadura e praga: só o número (sem animação de golpe).
    if (ev.dot !== Effect.UNSPECIFIED) {
      // Queimadura no Lendário do guia, veneno no Bom, praga num verde mais seco.
      const color = ev.dot === Effect.BURN ? "#ff9a3c" : ev.dot === Effect.PLAGUE ? "#9ccf5a" : "#7bd88f";
      this.at(Math.max(now, dst.impactUntil), dst.id, () => {
        if (showDamage()) this.fx.number(dst.head, "dot", String(ev.damage), color);
        if (ev.kill) dst.anim?.play("hit");
      });
      return;
    }
    // Golpes de área do raid boss: o visual veio no aviso do chão; aqui o número e, em
    // você, o tremor e o flash.
    if (ev.skillId >= RAID_HIT_BASE) {
      this.at(Math.max(now, dst.impactUntil), dst.id, () => {
        const onMe = ev.targetId === this.localId;
        if (ev.damage > 0 && showDamage()) this.fx.number(dst.head, ev.crit ? "crit" : "raid", ev.crit ? `${ev.damage}!` : String(ev.damage));
        gameAudio.hit("raid", ev.crit, false, dst.root.position, onMe);
        dst.anim?.play("hit");
        if (onMe && ev.damage > 0) {
          this.fx.shake(0.12, 0.25);
          this.fx.hitFlash(dst.meshes, false);
        }
      });
      return;
    }
    const sk = ev.skillId ? this.catalog.get(ev.skillId) : undefined;
    const isArea = sk?.target === SkillTarget.AREA; // a animação e o nome já vieram no SkillCast
    // Skills com área em volta do alvo mandam um evento por inimigo: a animação sai uma vez só
    // e os outros alvos levam o golpe junto com o principal.
    const key = `${ev.sourceId}:${ev.skillId}`;
    const last = this.lastSplash.get(key);
    const repeat = !!sk && sk.splash > 0 && !!last && now - last.t < 250;
    if (repeat) {
      this.hitAt(last!.at, ev, src, dst, sk, true);
      return;
    }

    // Quando o golpe encosta (ou a magia sai da mão).
    let release = now;
    if (isArea) {
      // Mesmo instante do SkillCast desse snapshot (que chega antes).
      const cast = this.castImpact.get(key);
      if (cast && now - cast.t < 250) release = cast.at;
    } else if (src && src !== dst) {
      release = this.swing(src, dst, ev.skillId ? "skill" : "attack", now);
      if (sk && sk.target === SkillTarget.ENEMY) this.fx.skillName(src.head, sk.name);
      if (!isCaster(src.model)) this.startTrail(src, release, sk);
    }
    const local = ev.sourceId === this.localId || ev.targetId === this.localId;

    let impact = release;
    if (src && src !== dst && !isArea) {
      const from = src.root.position.add(new Vector3(0, 1.3 * src.scale, 0));
      const to = dst.root.position.add(new Vector3(0, 1 * dst.scale, 0));
      const travel = Vector3.Distance(from, to) / PROJECTILE_SPEED;
      const bolt = BOLT_COLOR[sk?.fx ?? ""];
      // O projétil sai da mão no instante da soltura, do lugar onde cada um estiver.
      const launch = (fn: (from: Vector3, to: Vector3) => void) =>
        this.at(release, dst.id, () => {
          gameAudio.launch(sk?.fx, src.root.position, local);
          this.fx.as(local, () => fn(src.root.position.add(new Vector3(0, 1.3 * src.scale, 0)), dst.root.position.add(new Vector3(0, 1 * dst.scale, 0))));
        });
      if (sk?.fx === "fireball" || sk?.fx === "me_lance") {
        launch((a, b) => this.fx.fireball(a, b, travel));
        impact = release + travel * 1000;
      } else if (sk?.fx === "m_rain" || sk?.fx === "m_inferno") {
        launch(() => this.fx.emberRain(dst.root.position.clone(), sk.splash));
        impact = release + 650;
      } else if (sk?.fx === "m_meteor" || sk?.fx === "me_cata") {
        launch(() => this.fx.meteor(dst.root.position.clone(), sk.splash, ev.sourceId === this.localId));
        impact = release + 760;
      } else if (bolt) {
        launch((a, b) => this.fx.bolt(a, b, travel, bolt));
        impact = release + travel * 1000;
      } else if (!ev.skillId && isCaster(src.model)) {
        // Ataque básico de quem usa magia (magos e esqueletos magos).
        launch((a, b) => this.fx.arcaneBolt(a, b, travel));
        impact = release + travel * 1000;
      } else if (!ev.skillId && src.role === ARCH.archer) {
        // Atirador (tunnel#129): a lâmina arremessada voa até o alvo.
        launch((a, b) => this.fx.bolt(a, b, travel, THROWN_COLOR));
        impact = release + travel * 1000;
      }
    }
    if (sk && sk.splash > 0) this.lastSplash.set(key, { t: now, at: impact });
    this.hitAt(impact, ev, src, dst, sk);
  }

  /**
   * Toca o golpe de src e devolve quando ele encosta (performance.now()). O clipe começa
   * já no meio, para o golpe chegar SWING_LEAD depois; se os ataques vêm mais rápido que o
   * clipe, ele acelera para não recomeçar no meio. Um golpe que chega enquanto o anterior
   * ainda está subindo (skill + ataque básico no tick seguinte, skills de vários golpes) não
   * reinicia a animação: vira um segundo impacto logo depois do primeiro.
   */
  private swing(src: View, dst: View, role: "attack" | "skill", now: number): number {
    this.face(src, dst);
    if (now < src.swingImpact) {
      src.swingImpact = Math.max(src.swingImpact, src.lastLand) + FOLLOW_UP_MS;
      src.lastLand = src.swingImpact;
      return src.swingImpact;
    }
    const interval = (now - src.lastSwing) / 1000;
    src.lastSwing = now;
    let lead = 0;
    if (src.anim) {
      let speed = 1;
      const rest = src.anim.remaining(role, SWING_LEAD);
      if (interval > 0.3 && interval < 3 && rest > interval * 0.9) speed = clamp(rest / (interval * 0.9), 1, 2);
      lead = src.anim.play(role, { lead: SWING_LEAD, speed });
    }
    src.swingImpact = src.lastLand = now + lead * 1000;
    return src.swingImpact;
  }

  /** Agenda o golpe em dst para `at`: dano, HP e morte esperam por ele. */
  private hitAt(at: number, ev: CombatEvent, src: View | undefined, dst: View, sk: SkillInfo | undefined, quiet = false): void {
    dst.impactUntil = Math.max(dst.impactUntil, at);
    this.at(at, dst.id, () => this.landHit(ev, src, dst, sk, quiet));
  }

  /** Roda fn em `at` (performance.now()), na ordem; some se a entidade `id` sair. */
  private at(at: number, id: number, fn: () => void): void {
    if (at <= performance.now()) {
      this.runStage(); // o que já venceu antes sai antes
      fn();
      return;
    }
    // Inserção ordenada e estável (quem foi agendado antes para o mesmo instante sai antes).
    let i = this.stage.length;
    while (i > 0 && this.stage[i - 1].at > at) i--;
    this.stage.splice(i, 0, { at, id, fn });
  }

  /** Roda o que já chegou a hora (chamado a cada frame). */
  private runStage(): void {
    const now = performance.now();
    while (this.stage.length && this.stage[0].at <= now) this.stage.shift()!.fn();
  }

  /** Algo de uma entidade que saiu ou reviveu não acontece mais. */
  private dropStage(id: number): void {
    this.stage = this.stage.filter((s) => s.id !== id);
  }

  /**
   * Agenda fn para depois da última morte que ainda vai aparecer (o saque cai com o corpo).
   * Chamado também para remoções de saque, para não sair antes de entrar.
   */
  afterDeaths(fn: () => void): void {
    this.at(Math.max(performance.now(), this.lastDeathShow), 0, fn);
  }

  /** Número de dano e reação de quem levou o golpe. */
  private landHit(ev: CombatEvent, src: View | undefined, dst: View, sk: SkillInfo | undefined, quiet = false): void {
    if (!this.views.has(dst.id)) return;
    // Último golpe a caminho chegou: a barra mostra a vida que o servidor mandou.
    this.flushHp(dst);
    const numbers = showDamage();
    if (ev.heal && src && numbers) this.fx.number(src.head, "heal", `+${ev.heal}`);
    const mine = ev.sourceId === this.localId;
    const onMe = ev.targetId === this.localId;
    if (ev.miss) {
      // Em você: você esquivou; nos outros: o golpe errou.
      if (numbers) this.fx.number(dst.head, onMe ? "dodge" : "miss", onMe ? "Esquivou" : "Errou");
      if (!quiet) gameAudio.miss(dst.root.position, mine || onMe);
      return;
    }
    // O escudo segurou: tudo ("Bloqueado") ou parte (o número sai também, logo abaixo).
    if (ev.absorbed && numbers) this.fx.number(dst.head, "block", ev.damage === 0 ? "Bloqueado" : `(${ev.absorbed} absorvido)`);
    if (!quiet) gameAudio.hit(sk?.fx, ev.crit, !sk && !!src && isCaster(src.model), dst.root.position, mine || onMe);
    if (!quiet) this.feel(ev, src, dst, sk, mine, onMe);
    // Impacto (a Bola de Fogo e a magia básica já explodiram ao chegar).
    const at = dst.root.position.add(new Vector3(0, 1 * dst.scale, 0));
    const from = src ? src.root.position.add(new Vector3(0, 1 * src.scale, 0)) : at;
    // Marca no chão da área em volta do alvo (uma vez por skill, não por inimigo atingido).
    if (!quiet && sk && sk.splash > 0) this.fx.as(mine || onMe, () => this.fx.areaMark(dst.root.position, sk.splash, sk.fx));
    this.fx.as(mine || onMe, () => this.impactFx(quiet, ev, src, dst, sk, at, from, mine));
    const color = onMe || ev.crit || !sk ? undefined : (SKILL_NUMBER_COLOR[sk.fx] ?? skillFxColors(sk.fx)?.trail);
    if (ev.damage === 0 && sk && sk.power === 0) return; // skill só de efeito (Frasco, Marca…)
    if (ev.damage > 0 || !ev.absorbed) {
      const kind = onMe ? "taken" : ev.crit ? "crit" : sk ? "skill" : mine ? "mine" : "damage";
      if (numbers) this.fx.number(dst.head, kind, ev.crit ? `${ev.damage}!` : String(ev.damage), color);
    }
    if (!ev.kill && !dst.dead) dst.anim?.play("hit");
  }

  /**
   * Sensação do golpe que encostou: pausa de impacto e flash no seu golpe; flash e tremor
   * quando o golpe é em você (tremor só do chefe) e tremor leve no seu crítico.
   */
  private feel(ev: CombatEvent, src: View | undefined, dst: View, sk: SkillInfo | undefined, mine: boolean, onMe: boolean): void {
    if (mine && src && src !== dst && ev.damage > 0) {
      const s = ev.kill ? HIT_STOP.kill : ev.crit ? HIT_STOP.crit : sk ? HIT_STOP.skill : HIT_STOP.hit;
      dst.anim?.hitStop(s);
      // O atacante só congela se está no instante do golpe (corpo a corpo); quem soltou um
      // projétil já está em outra pose quando ele chega.
      if (Math.abs(performance.now() - src.swingImpact) < 120) src.anim?.hitStop(s);
      this.fx.hitFlash(dst.meshes, ev.crit);
      if (ev.crit) this.fx.shake(0.07, 0.18);
    } else if (onMe && ev.damage > 0) {
      this.fx.hitFlash(dst.meshes, false);
      if (src?.boss) this.fx.shake(0.12, 0.25);
    }
  }

  /** Efeito do impacto de cada skill (a receita de cada uma). */
  private impactFx(quiet: boolean, ev: CombatEvent, src: View | undefined, dst: View, sk: SkillInfo | undefined, at: Vector3, from: Vector3, mine: boolean): void {
    switch (quiet ? "quiet" : sk?.fx) {
      case "quiet":
      case "m_rain":
      case "m_inferno":
      case "m_meteor":
      case "m_lance":
      case "m_prison":
      case "m_arcane":
      case "r_mark":
      case "b_whirl": // 2 golpes por alvo: o giro já mostra o corte (sem faísca dobrada)
      case "bb_carnage": // 3 golpes: idem
      case "me_lance":
      case "me_cata":
      case "mo_rot":
      case "mo_drain":
      case "mo_abyss":
        break;
      case "r_flask":
      case "r_plague":
        this.fx.poisonSplash(dst.root.position.clone(), sk?.splash || 1.5, sk?.fx === "r_plague" ? new Color3(0.35, 0.55, 0.2) : new Color3(0.45, 0.9, 0.3));
        break;
      case "k_judg":
      case "k_horizon":
      case "b_exec":
      case "b_seismic":
      case "kh_dawn":
      case "bb_red":
        this.fx.brutalStrike(from, at, mine);
        this.fx.slam(dst.root.position, 1.8, new Color3(1, 0.85, 0.4), mine);
        break;
      case "k_charge":
      case "bw_charge":
        this.fx.shieldBash(at, mine);
        break;
      case "r_twin":
      case "r_para":
      case "r_fatal":
      case "r_shadow":
      case "rc_blades":
      case "rc_sentence":
        this.fx.backstab(from, at, ev.crit);
        break;
      case "shield_bash":
        this.fx.shieldBash(at, mine);
        break;
      case "brutal_strike":
        this.fx.brutalStrike(from, at, mine);
        break;
      case "backstab":
        this.fx.backstab(from, at, ev.crit);
        break;
      case "ice_nova":
      case "m_zero":
        this.fx.frostHit(at);
        break;
      case "fireball":
        break;
      default:
        // Faíscas de impacto (sistemas fixos) espirrando para longe de quem bateu.
        if (!src || !isCaster(src.model)) this.fx.meleeHit(at, ev.crit, src ? from : undefined);
    }
  }

  /**
   * Rastro da arma de quem bate corpo a corpo, em volta do instante do impacto. A cor é a
   * do tema da skill ou, no ataque básico, a da raridade da arma (guia de arte, 3.2).
   */
  private startTrail(src: View, impact: number, sk: SkillInfo | undefined): void {
    const mine = src.id === this.localId;
    // Dos outros, só perto de você (longe, o rastro tem poucos pixels e o pool é pequeno).
    if (!mine && (this.distanceTo(src.id) ?? Infinity) > TRAIL_OTHERS_M) return;
    if (src.hand === undefined) src.hand = this.findHand(src);
    if (!src.hand) return;
    const hex = (sk && skillFxColors(sk.fx)?.trail) || GRADE_COLOR[src.grade] || GRADE_COLOR[1];
    const hand = src.hand;
    this.fx.as(mine, () =>
      this.fx.weaponTrail(hand, impact - TRAIL_BEFORE_MS, impact + TRAIL_AFTER_MS, Color3.FromHexString(hex).scale(0.8)),
    );
  }

  /** Encaixe da arma na mão direita do modelo (null = sem modelo ou sem encaixe). */
  private findHand(v: View): TransformNode | null {
    const root = v.model3d?.root;
    if (!root) return null;
    const hand = root.getDescendants(false, (n) => n.name.split("#")[0] === "handslot.r")[0];
    return hand instanceof TransformNode ? hand : null;
  }

  /** Mostra a vida guardada se nenhum golpe estiver mais a caminho (ex.: o golpe errou). */
  private flushHp(v: View): void {
    if (v.pendingHp && performance.now() >= v.impactUntil) {
      this.setHp(v, v.pendingHp);
      v.pendingHp = null;
    }
  }

  /** Skill em si mesmo ou em área (o servidor avisa mesmo quando não acerta ninguém). */
  onSkillCast(ev: SkillCast): void {
    const src = this.views.get(ev.sourceId);
    const sk = this.catalog.get(ev.skillId);
    if (!src || !sk) return;
    const now = performance.now();
    this.fx.skillName(src.head, sk.name);
    // Skill de alvo sem dano direto (Frasco Tóxico, Praga): projétil até o alvo.
    const dst = ev.targetId ? this.views.get(ev.targetId) : undefined;
    if (dst) {
      const release = src !== dst ? this.swing(src, dst, "skill", now) : now + (src.anim?.play("skill", { lead: SWING_LEAD }) ?? 0) * 1000;
      const from = src.root.position.add(new Vector3(0, 1.3 * src.scale, 0));
      const to = dst.root.position.add(new Vector3(0, 1 * dst.scale, 0));
      const travel = Vector3.Distance(from, to) / PROJECTILE_SPEED;
      const color = BOLT_COLOR[sk.fx] ?? new Color3(0.8, 0.8, 1);
      // O efeito no alvo (veneno, marca…) só aparece quando o projétil chega.
      dst.impactUntil = Math.max(dst.impactUntil, release + travel * 1000);
      const local = ev.sourceId === this.localId || ev.targetId === this.localId;
      this.at(release, dst.id, () => {
        const a = src.root.position.add(new Vector3(0, 1.3 * src.scale, 0));
        const b = dst.root.position.add(new Vector3(0, 1 * dst.scale, 0));
        gameAudio.cast(sk.fx, src.root.position, local);
        this.fx.as(local, () =>
          this.fx.bolt(a, b, travel, color, () => {
            gameAudio.hit(sk.fx, false, false, dst.root.position, local);
            if (sk.fx === "r_flask" || sk.fx === "r_plague") this.fx.poisonSplash(dst.root.position.clone(), sk.splash || 1.5, color);
          }),
        );
      });
      return;
    }
    // O efeito da área sai no instante do golpe da animação; os golpes desse snapshot também.
    const lead = src.anim?.play(sk.target === SkillTarget.AREA ? "skill" : "buff", { lead: SWING_LEAD }) ?? 0;
    const at = now + lead * 1000;
    src.swingImpact = src.lastLand = at;
    src.lastSwing = now;
    this.castImpact.set(`${ev.sourceId}:${ev.skillId}`, { t: now, at });
    // Giros e pancadas corpo a corpo: o rastro da arma acompanha o golpe.
    if (sk.target === SkillTarget.AREA && sk.power > 0 && !isCaster(src.model)) this.startTrail(src, at, sk);
    this.at(at, src.id, () => {
      const mine = ev.sourceId === this.localId;
      this.fx.as(mine, () => {
        // Marca no chão do tamanho real da área (biblioteca nova), por baixo da receita.
        if (sk.target === SkillTarget.AREA) this.fx.areaMark(src.root.position, sk.range, sk.fx);
        this.castFx(src, sk, mine);
      });
    });
  }

  /** Visual de uma skill em si mesmo ou em área. */
  private castFx(src: View, sk: SkillInfo, mine: boolean): void {
    gameAudio.cast(sk.fx, src.root.position, mine);
    switch (sk.fx) {
      case "ice_nova":
      case "m_zero":
        this.fx.iceNova(src.root.position, sk.range);
        break;
      case "fortress":
        this.fx.fortress(src);
        break;
      case "fury":
        this.fx.fury(src);
        break;
      case "shadow_step":
        this.fx.shadowStep(src);
        break;
      case "k_taunt":
        this.fx.shout(src, new Color3(1, 0.3, 0.2), sk.range);
        break;
      case "k_cry":
      case "bw_cry":
        this.fx.shout(src, new Color3(1, 0.65, 0.25), 10);
        break;
      case "kb_defy":
        this.fx.shout(src, new Color3(0.45, 0.65, 1), sk.range);
        break;
      case "b_roar":
        this.fx.shout(src, new Color3(0.7, 0.4, 0.9), sk.range);
        if (mine) this.fx.shake(0.08);
        break;
      case "k_spin":
      case "b_whirl":
      case "bb_carnage":
        this.fx.whirl(src, sk.range, mine);
        break;
      case "b_quake":
      case "b_stomp":
      case "b_stone":
      case "bw_banner":
        this.fx.slam(src.root.position, sk.range, new Color3(1, 0.7, 0.35), mine);
        break;
      case "k_bulwark":
      case "kb_last":
        this.fx.slam(src.root.position, sk.range, new Color3(0.45, 0.65, 1), mine);
        break;
      case "kh_sun":
        this.fx.slam(src.root.position, sk.range, new Color3(1, 0.85, 0.45), mine);
        break;
      case "r_miasma":
        this.fx.poisonSplash(src.root.position.clone(), sk.range, new Color3(0.45, 0.9, 0.3));
        break;
      case "r_smoke":
        this.fx.smokeCloud(src.root.position.clone(), sk.range, (sk.effectMs || 6000) / 1000);
        break;
      case "m_armor":
        this.fx.castGlow(src.root.position.add(new Vector3(0, 1, 0)), new Color3(0.6, 0.9, 1));
        break;
      case "m_shield":
        this.fx.castGlow(src.root.position.add(new Vector3(0, 1, 0)), new Color3(0.45, 0.55, 1));
        break;
      case "b_frenzy":
      case "b_undying":
        this.fx.castGlow(src.root.position.add(new Vector3(0, 1, 0)), new Color3(1, 0.35, 0.2));
        break;
      case "r_vanish":
      case "m_blink":
      case "k_banner":
        break; // o próprio efeito (fumaça, teleporte, estandarte) já aparece
      default:
        this.fx.castGlow(src.root.position.add(new Vector3(0, 1, 0)), new Color3(1, 0.85, 0.5));
    }
  }

  onEffect(ev: EffectEvent): void {
    const v = this.views.get(ev.targetId);
    if (!v || v.dead) return;
    const ms = ev.durationMs;
    const now = performance.now();
    v.effects.get(ev.effect)?.visual?.stop(); // renovado: troca o visual
    // O registro vale na hora (ícones, previsão de movimento); o visual nasce quando o
    // golpe/projétil que causou o efeito encosta.
    const ef: TrackedEffect = { effect: ev.effect, until: now + ms, durationMs: ms, value: ev.value, visual: null };
    v.effects.set(ev.effect, ef);
    this.at(Math.max(now, v.impactUntil), v.id, () => {
      if (v.effects.get(ev.effect) !== ef || v.shownDead) return; // acabou ou foi renovado antes
      if (ev.effect === Effect.INVISIBLE) this.setGhost(v, true);
      this.fx.as(v.id === this.localId, () => (ef.visual = this.fx.effect(ev.effect, v, Math.max(0, ef.until - performance.now()))));
    });
  }

  onEffectEnd(ev: EffectEnd): void {
    const v = this.views.get(ev.targetId);
    const ef = v?.effects.get(ev.effect);
    if (!v || !ef) return;
    v.effects.delete(ev.effect);
    // Morreu com o efeito: o visual some com a morte, não antes do golpe encostar.
    this.at(Math.max(performance.now(), v.impactUntil), v.id, () => {
      ef.visual?.stop();
      if (ev.effect === Effect.INVISIBLE && !v.effects.has(Effect.INVISIBLE)) this.setGhost(v, false);
    });
  }

  /** Invisível: você se vê translúcido; os outros quase não veem (nem o nome). */
  private setGhost(v: View, on: boolean): void {
    const alpha = !on ? 1 : v.id === this.localId ? 0.35 : 0.08;
    for (const m of v.meshes) m.visibility = alpha;
    if (v.capsule) v.capsule.visibility = alpha;
    v.label.alpha = on && v.id !== this.localId ? 0 : 1;
  }

  /** Efeitos ativos (para os ícones do HUD), do que acaba antes para o que acaba depois. */
  effectsOf(id: number): ActiveEffect[] {
    const v = this.views.get(id);
    if (!v) return [];
    const now = performance.now();
    const out: ActiveEffect[] = [];
    for (const ef of v.effects.values()) {
      if (ef.until > now) out.push({ effect: ef.effect, until: ef.until, durationMs: ef.durationMs, value: ef.value });
    }
    return out.sort((a, b) => a.until - b.until);
  }

  /** Distância (no chão) entre o jogador local e uma entidade. */
  /**
   * Distância até a borda do corpo (o que conta para o alcance dos golpes e skills no
   * servidor): igual a distanceTo, menos o raio dos monstros grandes.
   */
  reachDistanceTo(id: number): number | null {
    const d = this.distanceTo(id);
    const v = this.views.get(id);
    if (d === null || !v) return d;
    return Math.max(0, d - (MODEL_BODY[v.model]?.radius ?? 0));
  }

  distanceTo(id: number): number | null {
    const me = this.local;
    const v = this.views.get(id);
    if (!me || !v) return null;
    return Math.hypot(v.root.position.x - me.root.position.x, v.root.position.z - me.root.position.z);
  }

  private clearEffects(v: View): void {
    for (const ef of v.effects.values()) ef.visual?.stop();
    v.effects.clear();
  }

  /**
   * Morreu: para de andar na hora, mas a queda só aparece depois que o golpe que matou
   * encosta (o servidor manda a morte junto com o golpe).
   */
  onDeath(id: number): void {
    const v = this.views.get(id);
    if (!v) return;
    v.dead = true;
    v.target = { ...v.server };
    if (id === this.localId) this.resetLocalPrediction();
    const now = performance.now();
    const at = v.impactUntil > now ? v.impactUntil + DEATH_AFTER_HIT_MS : now;
    this.lastDeathShow = Math.max(this.lastDeathShow, at);
    this.at(at, id, () => {
      if (!v.dead) return;
      this.clearEffects(v);
      v.pendingHp = null;
      v.hp = 0;
      this.showDead(v, true);
      this.refreshHpBar(v);
      gameAudio.death(v.model, v.kind === EntityKind.MONSTER, v.boss, v.root.position, id === this.localId);
    });
  }

  onRevive(r: Revive): void {
    const v = this.views.get(r.id);
    if (!v) return;
    this.dropStage(r.id);
    this.clearEffects(v);
    v.impactUntil = 0;
    v.pendingHp = null;
    v.dead = false;
    v.hp = r.hp;
    v.maxHp = r.maxHp;
    v.server = { x: r.x, z: r.z };
    v.target = { x: r.x, z: r.z };
    v.root.position.set(r.x, heightAt(r.x, r.z), r.z);
    if (r.id === this.localId) this.resetLocalPrediction();
    this.showDead(v, false);
    this.refreshHpBar(v);
  }

  /**
   * O jogador abriu a conversa ou a loja de um NPC do remaster: ele se vira para o jogador e faz o gesto de
   * cumprimento (clipe `buff`); alguns segundos depois volta a olhar para a praça. NPC de outro estilo: nada muda.
   */
  greetNpc(id: number): void {
    const v = this.views.get(id);
    if (v?.kind !== EntityKind.NPC || !v.anim || !remasterNpcModel(v.name)) return;
    const me = this.localPosition();
    if (me) {
      v.faceYaw = Math.atan2(me.x - v.root.position.x, me.z - v.root.position.z);
      v.greetUntil = performance.now() + 6000;
    }
    v.anim.play("buff");
  }

  /** Emote de um jogador: acenar toca uma vez; sentar e dançar ficam até ele andar ou atacar. */
  emote(id: number, kind: EmoteKind): void {
    const v = this.views.get(id);
    if (!v?.anim || v.shownDead) return;
    if (kind === EmoteKind.WAVE) v.anim.play("wave");
    else if (kind === EmoteKind.SIT) v.anim.play("sit", { loop: true });
    else if (kind === EmoteKind.DANCE) v.anim.play("dance", { loop: true });
    else v.anim.endEmote();
  }

  onLevelUp(l: LevelUp): void {
    const v = this.views.get(l.id);
    if (!v) return;
    v.level = l.level;
    v.maxHp = l.maxHp;
    v.hp = l.maxHp;
    this.refreshLabel(v);
    this.refreshHpBar(v);
    this.fx.as(l.id === this.localId, () => this.fx.levelUp(v.root));
    gameAudio.levelUp(v.root.position, l.id === this.localId);
  }

  /** Chamado a cada frame. */
  update(dt: number): void {
    this.runStage();
    const blend = 1 - Math.exp(-CORRECTION_RATE * dt);
    const me = this.local?.root.position;
    const cam = this.scene.activeCamera?.position;
    const planes = this.scene.frustumPlanes;
    const view = settings.get().viewDistance;
    for (const v of this.views.values()) {
      const always = v.id === this.localId || v.id === this.targetId;
      // Vendedor da loja pessoal (tunnel#161): sem sombra e, depois de sentar, pose parada.
      const vendor = v.shopSince > 0 && !always;
      if (me) {
        const d = Math.hypot(v.root.position.x - me.x, v.root.position.z - me.z);
        // Distância de visão: jogadores e monstros além dela somem (a névoa de render.ts
        // esconde a borda). Ficam sempre: você, o alvo, o grupo e os chefes; NPCs e objetos
        // (pedras, barco) são do cenário e ficam também.
        const cullable = !always && !v.boss && (v.kind === EntityKind.PLAYER || v.kind === EntityKind.MONSTER) && !this.partyIds.has(v.id);
        const culled = cullable && d > view + (v.culled ? -VIEW_SLACK : VIEW_SLACK);
        if (culled !== v.culled) {
          v.culled = culled;
          v.root.setEnabled(!culled);
        }
        v.label.isVisible = !culled && (d < LABEL_DISTANCE || always);
        this.setShadow(v, !culled && !vendor && (d < SHADOW_DISTANCE || always));
      }
      // Extrapola a posição do servidor (mesma regra de movimento do Go).
      const dx = v.target.x - v.server.x;
      const dz = v.target.z - v.server.z;
      const dist = Math.hypot(dx, dz);
      const stepLen = v.speed * dt;
      if (dist > 0 && !v.dead) {
        if (dist <= stepLen) {
          v.server.x = v.target.x;
          v.server.z = v.target.z;
        } else {
          v.server.x += (dx / dist) * stepLen;
          v.server.z += (dz / dist) * stepLen;
        }
      }

      const pos = v.root.position;
      // Só escreve na posição quando ela muda: escrever (mesmo o mesmo valor) marca o nó
      // como sujo, e aí o Babylon refaz a matriz de tudo que está embaixo dele no quadro.
      // As aproximações suaves encostam no destino abaixo de 1 mm (invisível) e param.
      if (v.id === this.localId && this.localDriven && !v.dead) {
        // Cliente dirigindo: desenha a previsão (a câmera vai junto, sem trancos) mais o
        // resto de alguma correção, que some suavemente.
        const k = Math.exp(-LOCAL_SMOOTH_RATE * dt);
        this.drawOffset.x = settle(this.drawOffset.x * k, 0);
        this.drawOffset.z = settle(this.drawOffset.z * k, 0);
        setIfChanged(pos, v.server.x + this.drawOffset.x, v.server.z + this.drawOffset.z);
      } else {
        // Aproxima suavemente a posição desenhada da posição extrapolada.
        setIfChanged(pos, settle(pos.x + (v.server.x - pos.x) * blend, v.server.x), settle(pos.z + (v.server.z - pos.z) * blend, v.server.z));
      }
      if (v.id === this.localId) this.recordLocal(v);

      // Anda de verdade o que se vê andar: a velocidade vem da posição DESENHADA, não do destino do servidor (que chega
      // antes de a posição desenhada alcançar e fazia o personagem escorregar tocando o parado, ou correr no lugar).
      updateMotion(v, pos.x, pos.z, dt, v.dead);
      const steering = dist > 0.05 && !v.dead;
      const moving = this.legacyMove ? steering : v.walking;
      if (v.id === this.localId) this.footsteps(v, moving, dt);
      if (v.greetUntil && performance.now() > v.greetUntil) {
        v.greetUntil = 0;
        v.faceYaw = Math.atan2(-pos.x, -pos.z); // de volta para o centro da praça
      }
      if (steering) {
        v.visual.rotation.y = lerpAngle(v.visual.rotation.y, Math.atan2(dx, dz), blend);
        v.faceYaw = null;
      } else if (v.faceYaw !== null) {
        const yaw = lerpAngle(v.visual.rotation.y, v.faceYaw, 1 - Math.exp(-FACE_RATE * dt));
        // Já virou: fixa o ângulo e para de girar (senão o nó fica sujo todo quadro).
        if (Math.abs(lerpAngle(yaw, v.faceYaw, 1) - yaw) < 1e-3) {
          v.visual.rotation.y = v.faceYaw;
          v.faceYaw = null;
        } else {
          v.visual.rotation.y = yaw;
        }
      }
      if (v.bounds) this.moveBounds(v);
      if (v.anim) {
        // LOD de animação: quem está fora da tela ou longe não tem os ossos recalculados.
        let active = always;
        if (!active && !v.culled) {
          const d2 = cam ? Vector3.DistanceSquared(cam, pos) : 0;
          const near = d2 < ANIM_DISTANCE * ANIM_DISTANCE;
          const far = !near && v.walking && d2 < ANIM_FAR_DISTANCE * ANIM_FAR_DISTANCE;
          active = (near || far) && (!planes || (v.bounds ? v.bounds.isInFrustum(planes) : v.meshes.some((m) => m.isInFrustum(planes))));
          // Entre 50 e 90 m anda a 15 Hz: o tempo parado é recuperado ao acordar (AnimController.catchUp).
          if (active && far) {
            v.animAcc += dt;
            if (v.animAcc < ANIM_FAR_STEP) active = false;
            else v.animAcc = 0;
          }
        }
        // Andar acaba com o emote na hora (o aviso do servidor chega depois).
        if (moving && v.anim.emoting) v.anim.endEmote();
        if (v.shopSince > 0 && !moving) {
          if (!v.anim.emoting) v.anim.play("sit", { loop: true });
          if (vendor && performance.now() - v.shopSince > VENDOR_FREEZE_MS) active = false;
        }
        v.anim.update(dt, moving, Math.max(0.6, v.speed / RUN_ANIM_SPEED), active, this.legacyMove ? v.speed : v.vis);
      }
    }
  }

  /**
   * Poeira nos passos do jogador local correndo: uma nuvem pequena a cada passo, alternando
   * o pé (o Fx só desenha no médio e no alto). Invisível não levanta poeira.
   */
  private footsteps(v: View, moving: boolean, dt: number): void {
    if (!moving || v.speed < DUST_MIN_SPEED || v.effects.has(Effect.INVISIBLE)) {
      this.stepPhase = 0.6; // o primeiro passo depois de parar sai logo
      return;
    }
    this.stepPhase += (v.speed * dt) / STEP_M;
    if (this.stepPhase < 1) return;
    this.stepPhase -= Math.floor(this.stepPhase);
    this.stepSide = -this.stepSide;
    this.fx.footDust(v.root.position, v.visual.rotation.y, this.stepSide);
  }

  /** Guarda a posição prevista do jogador local (para a reconciliação). */
  private recordLocal(v: View): void {
    const now = performance.now();
    const h = this.localHistory;
    h.push({ t: now, x: v.server.x, z: v.server.z });
    while (h.length > 2 && h[0].t < now - LOCAL_HISTORY_MS) h.shift();
  }

  /** Liga/desliga a sombra do personagem (só quem está perto projeta). */
  private setShadow(v: View, on: boolean): void {
    if (v.castsShadow === on) return;
    v.castsShadow = on;
    for (const m of v.meshes) {
      if (on) this.shadows.addShadowCaster(m, false);
      else this.shadows.removeShadowCaster(m, false);
    }
  }

  localPosition(): Vector3 | undefined {
    return this.local?.root.position;
  }

  /** Soltou o WASD/joystick: para a previsão na hora e espera o servidor dizer onde parou. */
  stopLocal(): void {
    const v = this.local;
    if (!v || v.dead) return;
    this.driveLocal(v);
    v.target.x = v.server.x;
    v.target.z = v.server.z;
    this.localPending = { x: v.server.x, z: v.server.z, at: performance.now(), stop: true };
  }

  /** Entidades vivas para o mapa (posição desenhada, não a do servidor). */
  forEachMarker(cb: (m: MapMarker) => void): void {
    for (const v of this.views.values()) {
      if (v.dead || v.kind === EntityKind.OBJECT) continue;
      cb({
        id: v.id, x: v.root.position.x, z: v.root.position.z, kind: v.kind, boss: v.boss,
        local: v.id === this.localId, target: v.id === this.targetId, heading: v.visual.rotation.y,
        party: this.partyIds.has(v.id),
      });
    }
  }

  /** Todas as entidades, para as invariantes do cliente (net/invariants.ts, 1×/s). */
  forEachProbe(cb: (e: EntityProbe) => void): void {
    for (const v of this.views.values()) {
      v.awaitingModel = !v.model3d && !v.capsule && (this.capsulesHeld || this.lazyWaits.has(v.id));
      cb(v);
    }
  }

  /** Nó raiz de cada entidade (medição de FPS, ?bench=1: separa o que é cenário do que é entidade). */
  forEachRoot(cb: (e: { root: TransformNode; kind: EntityKind; name: string; local: boolean }) => void): void {
    for (const v of this.views.values()) cb({ root: v.root, kind: v.kind, name: v.name, local: v.id === this.localId });
  }

  /** Pontos que a câmera precisa enxergar (meio do corpo do jogador e do alvo). */
  focusPoints(): Vector3[] {
    const out: Vector3[] = [];
    for (const id of [this.localId, this.targetId]) {
      const v = id ? this.views.get(id) : undefined;
      if (v) out.push(new Vector3(v.root.position.x, 1.1 * v.scale, v.root.position.z));
    }
    return out;
  }

  /** Estado de movimento de uma entidade neste quadro (testes no navegador: deslize = velocidade desenhada contra clipes). */
  motionProbe(id: number): Record<string, number | boolean> | null {
    const v = this.views.get(id);
    if (!v) return null;
    return {
      x: v.root.position.x, z: v.root.position.z, vis: v.vis, walking: v.walking, speed: v.speed,
      dist: Math.hypot(v.target.x - v.server.x, v.target.z - v.server.z), scale: v.scale, ...(v.anim?.debug() ?? {}),
    };
  }

  /** Posição na tela (pixels CSS) do meio do corpo. Usado pelos testes no navegador. */
  screenPos(id: number, scene: Scene): { x: number; y: number } | null {
    const v = this.views.get(id);
    const cam = scene.activeCamera;
    if (!v || !cam) return null;
    const engine = scene.getEngine();
    const p = Vector3.Project(
      v.root.position.add(new Vector3(0, 0.9 * v.scale, 0)),
      Matrix.Identity(),
      scene.getTransformMatrix(),
      cam.viewport.toGlobal(engine.getRenderWidth(), engine.getRenderHeight()),
    );
    const ratio = engine.getRenderWidth() / engine.getRenderingCanvasClientRect()!.width;
    return { x: p.x / ratio, y: p.y / ratio };
  }

  /** Monstro vivo mais próximo do jogador local. Usado pelos testes no navegador. */
  nearestMonster(): { id: number; dist: number; level: number } | null {
    const me = this.local;
    if (!me) return null;
    let best: { id: number; dist: number; level: number } | null = null;
    for (const v of this.views.values()) {
      if (v.kind !== EntityKind.MONSTER || v.dead) continue;
      const d = Vector3.Distance(me.root.position, v.root.position);
      if (!best || d < best.dist) best = { id: v.id, dist: d, level: v.level };
    }
    return best;
  }

  /**
   * Monstros vivos para a auto-caça (huntrules.ts): distância, vida, se está neutro agora
   * (night = a hora do mundo), invisível e brigando com você. 4×/s no máximo.
   */
  huntCandidates(night: boolean): HuntCandidate[] {
    const me = this.local;
    if (!me) return [];
    const now = performance.now();
    const out: HuntCandidate[] = [];
    for (const v of this.views.values()) {
      if (v.kind !== EntityKind.MONSTER || v.dead) continue;
      const inv = v.effects.get(Effect.INVISIBLE);
      out.push({
        id: v.id, name: v.name, hp: v.hp, dist: Vector3.Distance(me.root.position, v.root.position),
        neutral: neutralNow(v, night), hidden: !!inv && inv.until > now, fightingMe: now - v.fightAt < FIGHT_MS,
      });
    }
    return out;
  }

  /** Vira para o alvo do golpe (suave, no update). */
  private face(src: View, dst: View): void {
    const dx = dst.root.position.x - src.root.position.x;
    const dz = dst.root.position.z - src.root.position.z;
    if (Math.hypot(dx, dz) > 0.01) src.faceYaw = Math.atan2(dx, dz);
  }

  private showDead(v: View, dead: boolean): void {
    v.shownDead = dead;
    if (v.anim) {
      if (dead) v.anim.play("death", { hold: true });
      else v.anim.reset();
    }
    if (v.capsule) {
      v.capsule.rotation.x = dead ? Math.PI / 2 : 0;
      v.capsule.position.y = dead ? 0.35 * v.scale : 0.9 * v.scale;
    }
    v.label.alpha = dead ? 0.5 : 1;
  }

  private refreshLabel(v: View): void {
    v.nameText.text =
      v.kind === EntityKind.MONSTER
        ? `${v.name}  Nv ${v.level}`
        : v.model === STONE_MODEL
          ? `${v.title} · ${v.name}`
          : v.kind === EntityKind.NPC
            ? `${v.title} ${v.name}`
            : v.name;
  }

  private refreshHpBar(v: View): void {
    const frac = v.maxHp > 0 ? Math.max(0, Math.min(1, v.hp / v.maxHp)) : 0;
    v.hpFill.width = frac;
    const isLocal = v.id === this.localId;
    v.hpBar.isVisible = !isLocal && !v.shownDead && v.kind !== EntityKind.NPC && (v.id === this.targetId || frac < 1);
  }

  private attachModel(v: View): void {
    if (v.model3d || !this.models) return;
    if (v.model === FERRY_MODEL) {
      this.seatFerryman(v);
      return;
    }
    // Remaster (padrão): os chefes com silhueta própria (Senhor dos Ossos, Ozrhal) têm modelo à parte.
    const bossModel = v.boss ? remasterModelOf(v.model, true) : v.model;
    // Remaster (padrão): NPC com modelo próprio (pelo nome; o servidor manda um herói).
    const npcModel = v.kind === EntityKind.NPC ? remasterNpcModel(v.name) : null;
    // Remaster: golem, chefes e NPCs baixam sob demanda; até chegar a entidade fica como cápsula (não o modelo de outro).
    for (const m of new Set([npcModel, bossModel, v.model])) {
      if (!m || !this.models.pending(m)) continue;
      // Sem cápsula: a entidade fica invisível, pede o modelo na hora e aparece com fade quando ele chega.
      if (!this.lazyWaits.has(v.id)) {
        const p = this.models.ensure(m);
        this.lazyWaits.set(v.id, p);
        void p.then((ok) => {
          if (this.lazyWaits.get(v.id) !== p) return;
          this.lazyWaits.delete(v.id);
          // O modelo falhou de vez: aí sim, a cápsula.
          if (!ok && this.views.get(v.id) === v && !v.model3d && !v.capsule) v.capsule = this.createCapsule(v);
        });
      }
      return;
    }
    // O visual do servidor (pele, cabelo, roupa) é de herói: só vale nos modelos de NPC que têm máscara de repintura.
    const inst =
      (npcModel && this.models.has(npcModel) ? this.models.instantiate(npcModel, String(v.id), undefined, npcModel === "npc_guarda" ? v.look : undefined) : null) ??
      (bossModel !== v.model ? this.models.instantiate(bossModel, String(v.id), undefined, v.look) : null) ??
      this.models.instantiate(v.model, String(v.id), undefined, v.look);
    if (!inst) return;
    if (v.kind === EntityKind.PLAYER) {
      inst.setWeapons(v.weapon, v.offhand, v.grade, v.weaponEnchant);
      inst.setArmor(v.armor);
    }
    inst.root.parent = v.visual;
    inst.root.scaling.scaleInPlace(v.scale);
    const meshes = inst.meshes();
    this.notPickable(meshes);
    this.setShadow(v, false);
    v.meshes = meshes;
    v.model3d = inst;
    this.fixBounds(v);
    v.anim = new AnimController(inst.anims, inst.resetPose, inst.impacts);
    v.anim.setBodyScale(v.scale);
    v.anim.legacy = this.legacyMove;
    v.hand = undefined; // modelo novo: procura o encaixe da mão de novo no próximo golpe
    v.capsule?.dispose();
    v.capsule = null;
    if (v.dead) this.showDead(v, true);
    else if (this.lazyWaits.has(v.id)) this.fadeIn(v); // chegou depois de a entidade aparecer: entra aos poucos
  }

  /**
   * Malhas com esqueleto: caixa fixa e generosa (cobre qualquer pose, inclusive deitado na
   * morte), a mesma para todas, sem sincronizar com a matriz de cada uma (o Babylon refazia
   * a caixa de cada malha a cada quadro em que a entidade anda). As armas seguem normais.
   */
  private fixBounds(v: View): void {
    if (!v.bounds) {
      const body = MODEL_BODY[v.model];
      const h = (body?.height ?? 2.1) * v.scale;
      const w = 0.85 * h; // deitado: o corpo inteiro para um lado
      v.bounds = new BoundingInfo(new Vector3(-w, -0.3 * v.scale, -w), new Vector3(w, h + 0.5 * v.scale, w));
    }
    let any = false;
    for (const m of v.meshes) {
      if (!m.skeleton || (m.subMeshes?.length ?? 0) > 1) continue;
      if (m.getBoundingInfo() === v.bounds) {
        any = true;
        continue;
      }
      m.doNotSyncBoundingInfo = true;
      m.setBoundingInfo(v.bounds);
      any = true;
    }
    if (!any) v.bounds = null;
    v.boundsX = NaN; // reposiciona no próximo quadro
  }

  /** Leva a caixa fixa até onde a entidade está (só quando ela se mexeu). */
  private moveBounds(v: View): void {
    const p = v.root.position;
    if (p.x === v.boundsX && p.y === v.boundsY && p.z === v.boundsZ) return;
    v.boundsX = p.x;
    v.boundsY = p.y;
    v.boundsZ = p.z;
    Matrix.TranslationToRef(p.x, p.y, p.z, v.boundsAt);
    v.bounds!.update(v.boundsAt);
  }

  private createCapsule(v: View): Mesh {
    const body = MeshBuilder.CreateCapsule(`body${v.id}`, { height: 1.8 * v.scale, radius: 0.35 * v.scale }, this.scene);
    body.parent = v.visual;
    body.position.y = 0.9 * v.scale;
    body.material = this.material(
      v.kind === EntityKind.MONSTER ? BONE_COLOR : v.kind === EntityKind.NPC ? NPC_COLOR : classInfo(v.model).color,
    );
    this.notPickable([body]);
    this.setShadow(v, false);
    v.meshes = [body];
    return body;
  }

  /** O clique usa o cilindro do add() (world/pick.ts); as malhas ficam fora do picking. */
  private notPickable(meshes: AbstractMesh[]): void {
    for (const m of meshes) m.isPickable = false;
  }

  /** Material do anel no chão (brilho próprio, sem luz), um por cor + transparência. */
  /** Cor do nome (estado PvP > grupo > emblema), linha do clã e anel do karma. */
  private refreshName(v: View): void {
    if (v.kind !== EntityKind.PLAYER) return;
    v.nameText.color =
      v.pvpState === PvpState.CHAOTIC
        ? NAME_COLORS.chaotic
        : v.pvpState === PvpState.FLAGGED
          ? NAME_COLORS.flagged
          : this.partyIds.has(v.id)
            ? NAME_COLORS.party
            : v.baseColor;
    v.clanText.text = v.clanName ? `‹${v.clanName}›` : "";
    v.clanText.isVisible = !!v.clanName;
    // Os setters do TextBlock ignoram o mesmo valor: sem redesenho se nada mudou.
    // Linha de baixo: a especialização e o título (texto fixo, só muda num PlayerStatus).
    if (v.titleText) {
      // Com a loja pessoal aberta, a placa no lugar do título (tunnel#161).
      const line = v.shopTitle
        ? `Loja · ${v.shopTitle}`
        : [specInfo(v.spec)?.name ?? "", v.title ? `«${v.title}»` : ""].filter(Boolean).join(" · ");
      v.titleText.text = line;
      v.titleText.isVisible = !!line;
    }
    const red = v.pvpState === PvpState.CHAOTIC;
    if (red && !v.pkRing) {
      const ring = MeshBuilder.CreateTorus(`pk${v.id}`, { diameter: 1.25, thickness: 0.05, tessellation: 32 }, this.scene);
      ring.parent = v.root;
      ring.position.y = 0.1;
      ring.material = this.ringMaterial(NAME_COLORS.chaotic, 0.55);
      ring.isPickable = false;
      v.pkRing = ring;
    }
    v.pkRing?.setEnabled(red);
  }

  private ringMaterial(hex: string, alpha: number): StandardMaterial {
    const key = `ring:${hex}:${alpha}`;
    let mat = this.mats.get(key);
    if (!mat) {
      mat = new StandardMaterial(`mat-${key}`, this.scene);
      mat.emissiveColor = Color3.FromHexString(hex);
      mat.disableLighting = true;
      mat.alpha = alpha;
      this.mats.set(key, mat);
    }
    return mat;
  }

  private material(hex: string): StandardMaterial {
    let mat = this.mats.get(hex);
    if (!mat) {
      mat = new StandardMaterial(`mat-${hex}`, this.scene);
      mat.diffuseColor = Color3.FromHexString(hex);
      this.mats.set(hex, mat);
    }
    return mat;
  }
}

/** Cor do número de dano de cada skill. */
/** Cor do projétil das skills de alvo à distância (as que não têm visual próprio). */
const BOLT_COLOR: Record<string, Color3> = {
  m_lance: new Color3(0.55, 0.85, 1),
  m_prison: new Color3(0.4, 0.75, 1),
  r_mark: new Color3(1, 0.2, 0.2),
  r_flask: new Color3(0.45, 0.95, 0.3),
  r_plague: new Color3(0.35, 0.6, 0.2),
  m_arcane: new Color3(0.7, 0.45, 1),
  mo_rot: new Color3(0.5, 0.75, 0.3),
  mo_drain: new Color3(0.65, 0.4, 0.9),
  mo_abyss: new Color3(0.75, 0.35, 1),
  rh_knife: new Color3(0.85, 0.9, 0.95),
  rh_net: new Color3(0.7, 0.85, 0.45),
  rh_rain: new Color3(0.85, 0.9, 0.95),
};

const SKILL_NUMBER_COLOR: Record<string, string> = {
  m_lance: "#9fe0ff",
  m_prison: "#9fe0ff",
  m_rain: "#ffb14a",
  m_meteor: "#ffcf6a",
  k_judg: "#ffe9a0",
  b_exec: "#ff7a5a",
  r_twin: "#e0b4ff",
  fireball: "#ffb14a",
  ice_nova: "#9fe0ff",
  shield_bash: "#cfe0ff",
  brutal_strike: "#ff9a6a",
  backstab: "#e0b4ff",
  k_horizon: "#ffe9a0",
  b_seismic: "#ff7a5a",
  b_whirl: "#ff9a8a",
  m_inferno: "#ffb14a",
  m_zero: "#9fe0ff",
  m_arcane: "#d8b4ff",
  r_fatal: "#ffd28a",
  r_shadow: "#e0b4ff",
  kh_dawn: "#ffe9a0",
  bb_red: "#ff7a6a",
  bb_carnage: "#ff9a8a",
  me_lance: "#ffb14a",
  me_cata: "#ffd08a",
  mo_rot: "#c8a8ff",
  mo_drain: "#d8b4ff",
  mo_abyss: "#d8b4ff",
  rc_blades: "#ffc0d0",
  rc_sentence: "#ffd28a",
  rh_knife: "#e8f0ff",
  rh_rain: "#e8ffa0",
};

/** Modelos que atacam com magia à distância. */
function isCaster(model: string): boolean {
  return model === "mage" || model.includes("mage");
}

/** Configurações → Jogo → Números de dano. */
function showDamage(): boolean {
  return settings.get().showDamage;
}

function clamp(v: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, v));
}

/** Valor de uma aproximação suave, já no destino quando falta menos de 1 mm. */
function settle(v: number, target: number): number {
  return Math.abs(target - v) < 1e-3 ? target : v;
}

/** Muda x/z só se mudaram (escrever no Vector3 marca o nó como sujo). */
function setIfChanged(p: Vector3, x: number, z: number): void {
  if (p.x !== x) p.x = x;
  if (p.z !== z) p.z = z;
  // Na altura do chão (o relevo é só visual: o servidor continua em x, z).
  const y = heightAt(x, z);
  if (p.y !== y) p.y = y;
}

function lerpAngle(a: number, b: number, t: number): number {
  let d = (b - a) % (Math.PI * 2);
  if (d > Math.PI) d -= Math.PI * 2;
  if (d < -Math.PI) d += Math.PI * 2;
  return a + d * t;
}

/** Mistura a cor com branco (0 = igual, 1 = branco), para o nome ficar legível. */
function lighten(hex: string, k: number): string {
  const c = Color3.FromHexString(hex);
  return new Color3(c.r + (1 - c.r) * k, c.g + (1 - c.g) * k, c.b + (1 - c.b) * k).toHexString();
}
