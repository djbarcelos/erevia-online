import {
  AbstractMesh,
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
  type PlayerStatus,
  type Revive,
  type SkillCast,
  type SkillInfo,
} from "../gen/erevia/v1/game_pb";
import { gameAudio } from "../audio";
import { AnimController } from "./anim";
import { toBodyLook, type BodyLook } from "./body-look";
import type { Fx, FxBody, FxHandle } from "./fx";
import type { CharacterInstance, CharacterModels } from "./models";
import { GOLEM_BODY, GOLEM_MODEL } from "./golem";
import { buildTeleportStone } from "./teleport-stone";
import type { Bounds, Collision } from "./world-data";

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
/** Só personagens até esta distância do jogador projetam sombra. */
const SHADOW_DISTANCE = 35;
const BONE_COLOR = "#d9d2c0";
/** Ids dos golpes de área do raid boss nos CombatEvent (zone/raid.go: RaidHitWave…). */
export const RAID_HIT_BASE = 9001;
const NPC_COLOR = "#e0c27a";

/** Velocidade dos projéteis (m/s): o dano só aparece quando o projétil chega. */
const PROJECTILE_SPEED = 26;
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
  /** Jogadores: normal, em combate (roxo) ou com karma (vermelho). */
  pvpState: PvpState;
  /** Jogadores: nome do clã ("" = sem clã). */
  clanName: string;
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

/** Cores dos nomes por estado (o Babylon GUI não lê variáveis CSS). */
export const NAME_COLORS = {
  flagged: "#c77dff",
  chaotic: "#ff5a4a",
  party: "#7fd0ff",
  clan: "#e8cf8a",
  title: "#cdb8f2",
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
  /** Posição do servidor, extrapolada localmente em direção ao destino. */
  server: { x: number; z: number };
  target: { x: number; z: number };
  root: TransformNode;
  visual: TransformNode;
  head: TransformNode;
  capsule: Mesh | null;
  model3d: CharacterInstance | null;
  anim: AnimController | null;
  /** Malhas do modelo (para sombra e teste de visibilidade). */
  meshes: AbstractMesh[];
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
  private targetId = 0;
  private catalog = new Map<number, SkillInfo>();

  constructor(
    private scene: Scene,
    private gui: AdvancedDynamicTexture,
    readonly localId: number,
    /** Retângulo jogável (o mapa cresceu só para o sul). */
    private bounds: Bounds,
    private fx: Fx,
    private shadows: ShadowGenerator,
    private collision: Collision,
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

  /** Aliado: do grupo ou do mesmo clã (nunca é atacado). */
  isAlly(id: number): boolean {
    if (id === this.localId || this.partyIds.has(id)) return true;
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
    return force || v.pvpState !== PvpState.NORMAL;
  }

  /** Estado PvP ou clã de um jogador mudou. */
  setStatus(s: PlayerStatus): void {
    const v = this.views.get(s.id);
    if (!v) return;
    v.pvpState = s.pvpState;
    v.clanName = s.clanName;
    v.title = s.title;
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
    if (a.look) v.look = toBodyLook(a.look);
    if (!v.model3d) return;
    v.model3d.setLook(v.look);
    this.setShadow(v, false); // as malhas mudam; a sombra volta no próximo quadro
    v.model3d.setWeapons(v.weapon, v.offhand, v.grade, v.weaponEnchant);
    v.meshes = v.model3d.meshes();
    this.notPickable(v.meshes);
  }

  isEnemy(id: number): boolean {
    const v = this.views.get(id);
    return !!v && v.kind === EntityKind.MONSTER && !v.dead;
  }

  /** Quando os modelos 3D terminam de carregar: troca as cápsulas pelos modelos. */
  setModels(models: CharacterModels): void {
    this.models = models;
    for (const v of this.views.values()) if (v.kind !== EntityKind.OBJECT) this.attachModel(v);
  }

  setTarget(id: number): void {
    this.targetId = id;
    const v = this.views.get(id);
    if (v) {
      this.selectRing.parent = v.root;
      this.selectRing.position.set(0, 0.04, 0);
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
    const isLocal = e.id === this.localId;
    if (isLocal) this.resetLocalPrediction();
    const scale = e.scale > 0 ? e.scale : 1;

    const root = new TransformNode(`e${e.id}`, this.scene);
    root.position.set(e.x, 0, e.z);
    const visual = new TransformNode(`visual${e.id}`, this.scene);
    visual.parent = root;

    // Área de clique estável (invisível): o modelo animado não serve, porque o picking de
    // malha com esqueleto usa a pose parada e braços/pernas entram e saem do cursor.
    const isObject = e.kind === EntityKind.OBJECT;
    const body = MODEL_BODY[e.model];
    if (!isObject) {
      // Monstros grandes (golem) ganham uma área do tamanho do corpo, senão o clique
      // passa pelos ombros e braços sem selecionar.
      const h = (body?.height ?? 2.1) * scale;
      const hit = MeshBuilder.CreateCylinder(`hit${e.id}`, { height: h, diameter: (body?.width ?? 1.2) * scale, tessellation: 12 }, this.scene);
      hit.parent = root;
      hit.position.y = h / 2;
      hit.isVisible = false;
      hit.isPickable = true;
      hit.metadata = { entityId: e.id };
    }

    // Anel no chão: na cor do emblema para jogadores (o seu, mais forte), dourado sem emblema.
    const emblem = e.kind === EntityKind.PLAYER && /^#[0-9a-f]{6}$/i.test(e.emblem) ? e.emblem : "";
    if (isLocal || emblem) {
      const ring = MeshBuilder.CreateTorus(`ring${e.id}`, { diameter: isLocal ? 1.1 : 1.0, thickness: isLocal ? 0.06 : 0.04, tessellation: 32 }, this.scene);
      ring.parent = root;
      ring.position.y = 0.03;
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
        ? "#ffb4a8"
        : e.kind === EntityKind.NPC
          ? "#a8e6a0"
          : e.kind === EntityKind.OBJECT
            ? "#bcd4ff"
            : "#e6ecff";
    nameText.color = baseColor;
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
      model: e.model, boss: e.boss, scale, speed: e.speed, weapon: e.weapon, offhand: e.offhand, grade: e.weaponGrade, emblem, look: toBodyLook(e.look),
      weaponEnchant: e.weaponEnchant, pvpState: e.pvpState, clanName: e.clanName, baseColor, clanText, titleText, pkRing: null,
      server: { x: e.x, z: e.z },
      target: { x: e.targetX, z: e.targetZ },
      root, visual, head, capsule: null, model3d: null, anim: null, meshes: [], castsShadow: false,
      label, nameText, hpBar, hpFill, lastHurt: 0, effects: new Map(),
      impactUntil: 0, pendingHp: null, shownDead: e.dead, lastSwing: 0, swingImpact: 0, lastLand: 0, faceYaw: null,
    };
    this.views.set(e.id, v);
    // NPCs ficam parados, virados para o centro da praça.
    if (e.kind === EntityKind.NPC) visual.rotation.y = Math.atan2(-e.x, -e.z);
    this.refreshLabel(v);
    this.refreshName(v);
    this.refreshHpBar(v);

    if (isObject) {
      this.buildBanner(v);
      return;
    }
    if (e.model === STONE_MODEL) {
      this.buildStone(v);
      return;
    }
    if (this.models) this.attachModel(v);
    if (!v.model3d) v.capsule = this.createCapsule(v);
    if (v.dead) this.showDead(v, true);
    else if (fade && e.kind === EntityKind.MONSTER) this.fadeIn(v);
  }

  /** Pedra de Teleporte: monólito com runas acesas e núcleo de luz (ver teleport-stone.ts). */
  private buildStone(v: View): void {
    v.meshes = buildTeleportStone(this.scene, this.fx, v.id, v.visual, v.root).shadowMeshes;
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
    area.position.y = 0.05;
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
    this.setShadow(v, false);
    if (this.targetId === id) this.setTarget(0);
    if (this.selectRing.parent === v.root) this.selectRing.parent = null;
    this.clearEffects(v);
    this.dropStage(id);
    v.label.dispose();
    this.views.delete(id);
    // Corpo de monstro: afunda e desbota antes de sumir.
    if (v.shownDead && v.kind === EntityKind.MONSTER) this.sinkAndDispose(v);
    else this.disposeView(v);
  }

  private disposeView(v: View): void {
    v.model3d?.dispose();
    // Materiais são compartilhados (modelo, anel, efeitos): só as malhas são descartadas.
    v.root.dispose(false, false);
  }

  /** Corpo afundando no chão e desbotando (CORPSE_SINK_S), depois descartado. */
  private sinkAndDispose(v: View): void {
    for (const m of v.root.getChildMeshes(false)) m.isPickable = false;
    const meshes = v.meshes.length ? v.meshes : v.root.getChildMeshes(false);
    let t = 0;
    const obs = this.scene.onBeforeRenderObservable.add(() => {
      t += this.scene.getEngine().getDeltaTime() / 1000;
      const k = Math.min(1, t / CORPSE_SINK_S);
      v.root.position.y = -0.5 * v.scale * k * k;
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
      v.root.position.set(s.x, 0, s.z);
      v.server.x = v.target.x = s.x;
      v.server.z = v.target.z = s.z;
      if (s.id === this.localId) this.resetLocalPrediction();
      this.fx.blinkFlash(from, v.root.position.clone(), new Color3(0.7, 0.5, 1));
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
    if (err > LOCAL_SNAP) {
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
    // Veneno, queimadura e praga: só o número (sem animação de golpe).
    if (ev.dot !== Effect.UNSPECIFIED) {
      const color = ev.dot === Effect.BURN ? "#ffa04a" : ev.dot === Effect.PLAGUE ? "#9ccf5a" : "#7dff7a";
      this.at(Math.max(now, dst.impactUntil), dst.id, () => {
        this.fx.damageNumber(dst.head, String(ev.damage), color, false);
        if (ev.kill) dst.anim?.play("hit");
      });
      return;
    }
    // Golpes de área do raid boss: o visual veio no aviso do chão; aqui só o número.
    if (ev.skillId >= RAID_HIT_BASE) {
      this.at(Math.max(now, dst.impactUntil), dst.id, () => {
        if (ev.damage > 0) this.fx.damageNumber(dst.head, String(ev.damage), "#d68cff", ev.crit);
        gameAudio.hit("raid", ev.crit, false, dst.root.position, ev.targetId === this.localId);
        dst.anim?.play("hit");
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
    }

    let impact = release;
    if (src && src !== dst && !isArea) {
      const from = src.root.position.add(new Vector3(0, 1.3 * src.scale, 0));
      const to = dst.root.position.add(new Vector3(0, 1 * dst.scale, 0));
      const travel = Vector3.Distance(from, to) / PROJECTILE_SPEED;
      const bolt = BOLT_COLOR[sk?.fx ?? ""];
      // O projétil sai da mão no instante da soltura, do lugar onde cada um estiver.
      const launch = (fn: (from: Vector3, to: Vector3) => void) =>
        this.at(release, dst.id, () => {
          gameAudio.launch(sk?.fx, src.root.position, ev.sourceId === this.localId || ev.targetId === this.localId);
          fn(src.root.position.add(new Vector3(0, 1.3 * src.scale, 0)), dst.root.position.add(new Vector3(0, 1 * dst.scale, 0)));
        });
      if (sk?.fx === "fireball") {
        launch((a, b) => this.fx.fireball(a, b, travel));
        impact = release + travel * 1000;
      } else if (sk?.fx === "m_rain" || sk?.fx === "m_inferno") {
        launch(() => this.fx.emberRain(dst.root.position.clone(), sk.splash));
        impact = release + 650;
      } else if (sk?.fx === "m_meteor") {
        launch(() => this.fx.meteor(dst.root.position.clone(), sk.splash, ev.sourceId === this.localId));
        impact = release + 760;
      } else if (bolt) {
        launch((a, b) => this.fx.bolt(a, b, travel, bolt));
        impact = release + travel * 1000;
      } else if (!ev.skillId && isCaster(src.model)) {
        // Ataque básico de quem usa magia (magos e esqueletos magos).
        launch((a, b) => this.fx.arcaneBolt(a, b, travel));
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
    if (ev.heal && src) this.fx.damageNumber(src.head, `+${ev.heal}`, "#7dff9a", false);
    if (ev.absorbed && !ev.miss) this.fx.damageNumber(dst.head, `(${ev.absorbed} absorvido)`, "#9fc6ff", false);
    const mine = ev.sourceId === this.localId;
    const onMe = ev.targetId === this.localId;
    if (ev.miss) {
      this.fx.damageNumber(dst.head, "Errou", "#b9b4c2", false);
      if (!quiet) gameAudio.miss(dst.root.position, mine || onMe);
      return;
    }
    if (!quiet) gameAudio.hit(sk?.fx, ev.crit, !sk && !!src && isCaster(src.model), dst.root.position, mine || onMe);
    // Impacto (a Bola de Fogo e a magia básica já explodiram ao chegar).
    const at = dst.root.position.add(new Vector3(0, 1 * dst.scale, 0));
    const from = src ? src.root.position.add(new Vector3(0, 1 * src.scale, 0)) : at;
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
        break;
      case "r_flask":
      case "r_plague":
        this.fx.poisonSplash(dst.root.position.clone(), sk?.splash || 1.5, sk?.fx === "r_plague" ? new Color3(0.35, 0.55, 0.2) : new Color3(0.45, 0.9, 0.3));
        break;
      case "k_judg":
      case "k_horizon":
      case "b_exec":
      case "b_seismic":
        this.fx.brutalStrike(from, at, mine);
        this.fx.slam(dst.root.position, 1.8, new Color3(1, 0.85, 0.4), mine);
        break;
      case "k_charge":
        this.fx.shieldBash(at, mine);
        break;
      case "r_twin":
      case "r_para":
      case "r_fatal":
      case "r_shadow":
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
        if (!src || !isCaster(src.model)) this.fx.meleeHit(at, ev.crit);
    }
    let color = onMe ? "#ff6b5a" : ev.crit ? "#ffd34d" : mine ? "#ffffff" : "#e8e1d4";
    if (sk && !onMe && !ev.crit) color = SKILL_NUMBER_COLOR[sk.fx] ?? color;
    if (ev.damage === 0 && sk && sk.power === 0) return; // skill só de efeito (Frasco, Marca…)
    this.fx.damageNumber(dst.head, ev.crit ? `${ev.damage}!` : String(ev.damage), color, ev.crit || !!sk);
    if (!ev.kill && !dst.dead) dst.anim?.play("hit");
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
        this.fx.bolt(a, b, travel, color, () => {
          gameAudio.hit(sk.fx, false, false, dst.root.position, local);
          if (sk.fx === "r_flask" || sk.fx === "r_plague") this.fx.poisonSplash(dst.root.position.clone(), sk.splash || 1.5, color);
        });
      });
      return;
    }
    // O efeito da área sai no instante do golpe da animação; os golpes desse snapshot também.
    const lead = src.anim?.play(sk.target === SkillTarget.AREA ? "skill" : "buff", { lead: SWING_LEAD }) ?? 0;
    const at = now + lead * 1000;
    src.swingImpact = src.lastLand = at;
    src.lastSwing = now;
    this.castImpact.set(`${ev.sourceId}:${ev.skillId}`, { t: now, at });
    this.at(at, src.id, () => this.castFx(src, sk, ev.sourceId === this.localId));
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
        this.fx.shout(src, new Color3(1, 0.65, 0.25), 10);
        break;
      case "b_roar":
        this.fx.shout(src, new Color3(0.7, 0.4, 0.9), sk.range);
        if (mine) this.fx.shake(6);
        break;
      case "k_spin":
      case "b_whirl":
        this.fx.whirl(src, sk.range, mine);
        break;
      case "b_quake":
      case "b_stomp":
      case "b_stone":
        this.fx.slam(src.root.position, sk.range, new Color3(1, 0.7, 0.35), mine);
        break;
      case "k_bulwark":
        this.fx.slam(src.root.position, sk.range, new Color3(0.45, 0.65, 1), mine);
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
      ef.visual = this.fx.effect(ev.effect, v, Math.max(0, ef.until - performance.now()));
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
    v.root.position.set(r.x, 0, r.z);
    if (r.id === this.localId) this.resetLocalPrediction();
    this.showDead(v, false);
    this.refreshHpBar(v);
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
    this.fx.levelUp(v.root);
    gameAudio.levelUp(v.root.position, l.id === this.localId);
  }

  /** Chamado a cada frame. */
  update(dt: number): void {
    this.runStage();
    const blend = 1 - Math.exp(-CORRECTION_RATE * dt);
    const me = this.local?.root.position;
    const cam = this.scene.activeCamera?.position;
    const planes = this.scene.frustumPlanes;
    for (const v of this.views.values()) {
      const always = v.id === this.localId || v.id === this.targetId;
      if (me) {
        const d = Math.hypot(v.root.position.x - me.x, v.root.position.z - me.z);
        v.label.isVisible = d < LABEL_DISTANCE || always;
        this.setShadow(v, d < SHADOW_DISTANCE || always);
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
      if (v.id === this.localId && this.localDriven && !v.dead) {
        // Cliente dirigindo: desenha a previsão (a câmera vai junto, sem trancos) mais o
        // resto de alguma correção, que some suavemente.
        const k = Math.exp(-LOCAL_SMOOTH_RATE * dt);
        this.drawOffset.x *= k;
        this.drawOffset.z *= k;
        pos.x = v.server.x + this.drawOffset.x;
        pos.z = v.server.z + this.drawOffset.z;
      } else {
        // Aproxima suavemente a posição desenhada da posição extrapolada.
        pos.x += (v.server.x - pos.x) * blend;
        pos.z += (v.server.z - pos.z) * blend;
      }
      if (v.id === this.localId) this.recordLocal(v);

      const moving = dist > 0.05 && !v.dead;
      if (moving) {
        v.visual.rotation.y = lerpAngle(v.visual.rotation.y, Math.atan2(dx, dz), blend);
        v.faceYaw = null;
      } else if (v.faceYaw !== null) {
        v.visual.rotation.y = lerpAngle(v.visual.rotation.y, v.faceYaw, 1 - Math.exp(-FACE_RATE * dt));
      }
      if (v.anim) {
        // LOD de animação: quem está fora da tela ou longe não tem os ossos recalculados.
        let active = always;
        if (!active) {
          const near = !cam || Vector3.DistanceSquared(cam, pos) < ANIM_DISTANCE * ANIM_DISTANCE;
          active = near && (!planes || v.meshes.some((m) => m.isInFrustum(planes)));
        }
        // Andar acaba com o emote na hora (o aviso do servidor chega depois).
        if (moving && v.anim.emoting) v.anim.endEmote();
        v.anim.update(dt, moving, Math.max(0.6, v.speed / RUN_ANIM_SPEED), active);
      }
    }
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

  /** Pontos que a câmera precisa enxergar (meio do corpo do jogador e do alvo). */
  focusPoints(): Vector3[] {
    const out: Vector3[] = [];
    for (const id of [this.localId, this.targetId]) {
      const v = id ? this.views.get(id) : undefined;
      if (v) out.push(new Vector3(v.root.position.x, 1.1 * v.scale, v.root.position.z));
    }
    return out;
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
    const inst = this.models.instantiate(v.model, String(v.id), undefined, v.look);
    if (!inst) return;
    if (v.kind === EntityKind.PLAYER) inst.setWeapons(v.weapon, v.offhand, v.grade, v.weaponEnchant);
    inst.root.parent = v.visual;
    inst.root.scaling.scaleInPlace(v.scale);
    const meshes = inst.meshes();
    this.notPickable(meshes);
    this.setShadow(v, false);
    v.meshes = meshes;
    v.model3d = inst;
    v.anim = new AnimController(inst.anims, inst.resetPose, inst.impacts);
    v.capsule?.dispose();
    v.capsule = null;
    if (v.dead) this.showDead(v, true);
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

  /** O clique usa a área invisível do add(); as malhas visíveis ficam de fora. */
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
    if (v.titleText) {
      v.titleText.text = v.title ? `«${v.title}»` : "";
      v.titleText.isVisible = !!v.title;
    }
    const red = v.pvpState === PvpState.CHAOTIC;
    if (red && !v.pkRing) {
      const ring = MeshBuilder.CreateTorus(`pk${v.id}`, { diameter: 1.25, thickness: 0.05, tessellation: 32 }, this.scene);
      ring.parent = v.root;
      ring.position.y = 0.035;
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
};

/** Modelos que atacam com magia à distância. */
function isCaster(model: string): boolean {
  return model === "mage" || model.includes("mage");
}

function clamp(v: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, v));
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
