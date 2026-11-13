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
import { AnimController } from "./anim";
import { toBodyLook, type BodyLook } from "./body-look";
import type { Fx, FxHandle } from "./fx";
import type { CharacterInstance, CharacterModels } from "./models";
import { GOLEM_BODY, GOLEM_MODEL } from "./golem";
import { buildTeleportStone } from "./teleport-stone";
import type { Bounds, Collision } from "./world-data";

/** Quão rápido a posição desenhada converge para a do servidor (1/s). */
const CORRECTION_RATE = 12;
/** Tempo máximo esperando o servidor confirmar um destino previsto. */
const PENDING_TIMEOUT_MS = 1000;
/** Velocidade de referência da animação de corrida (m/s). */
const RUN_ANIM_SPEED = 5;
/** Nomes e barras só aparecem até esta distância do jogador (o alvo sempre aparece). */
const LABEL_DISTANCE = 30;
/** Além desta distância da câmera (ou fora da tela) o personagem congela a animação. */
const ANIM_DISTANCE = 50;
/** Só personagens até esta distância do jogador projetam sombra. */
const SHADOW_DISTANCE = 35;
const BONE_COLOR = "#d9d2c0";
const NPC_COLOR = "#e0c27a";

/** Velocidade dos projéteis (m/s): o dano só aparece quando o projétil chega. */
const PROJECTILE_SPEED = 26;

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
  /** NPC: função ("Mercadora"). */
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
}

/** Jogadores e monstros na tela: criação, movimento suave, animações e combate. */
export class EntityManager {
  private views = new Map<number, View>();
  private localPending: { x: number; z: number; at: number } | null = null;
  private mats = new Map<string, StandardMaterial>();
  private ringMat: StandardMaterial;
  /** Último evento de skill com área no alvo (para animar uma vez só). */
  private lastSplash = new Map<string, number>();
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

  add(e: Entity): void {
    if (this.views.has(e.id)) return;
    const isLocal = e.id === this.localId;
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
      weaponEnchant: e.weaponEnchant, pvpState: e.pvpState, clanName: e.clanName, baseColor, clanText, pkRing: null,
      server: { x: e.x, z: e.z },
      target: { x: e.targetX, z: e.targetZ },
      root, visual, head, capsule: null, model3d: null, anim: null, meshes: [], castsShadow: false,
      label, nameText, hpBar, hpFill, lastHurt: 0, effects: new Map(),
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
    v.label.dispose();
    v.model3d?.dispose();
    // Materiais são compartilhados (modelo, anel, efeitos): só as malhas são descartadas.
    v.root.dispose(false, false);
    this.views.delete(id);
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
      if (s.id === this.localId) this.localPending = null;
      this.fx.blinkFlash(from, v.root.position.clone(), new Color3(0.7, 0.5, 1));
      if (s.speed > 0) v.speed = s.speed;
      return;
    }
    v.server.x = s.x;
    v.server.z = s.z;
    if (s.speed > 0) v.speed = s.speed;

    // Jogador local: enquanto o servidor não confirma o destino que já previmos,
    // não deixa um snapshot antigo "puxar" o personagem de volta.
    if (s.id === this.localId && this.localPending) {
      const pend = this.localPending;
      const confirmed = Math.abs(s.targetX - pend.x) < 0.05 && Math.abs(s.targetZ - pend.z) < 0.05;
      const expired = performance.now() - pend.at > PENDING_TIMEOUT_MS;
      if (!confirmed && !expired) return;
      this.localPending = null;
    }
    v.target.x = s.targetX;
    v.target.z = s.targetZ;
  }

  applyHp(u: HpUpdate): void {
    const v = this.views.get(u.id);
    if (!v) return;
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
    if (this.collision.segmentClear(v.server.x, v.server.z, cx, cz)) {
      v.target.x = cx;
      v.target.z = cz;
      this.localPending = { x: cx, z: cz, at: performance.now() };
    } else {
      this.localPending = null;
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
    // Veneno, queimadura e praga: só o número (sem animação de golpe).
    if (ev.dot !== Effect.UNSPECIFIED) {
      const color = ev.dot === Effect.BURN ? "#ffa04a" : ev.dot === Effect.PLAGUE ? "#9ccf5a" : "#7dff7a";
      this.fx.damageNumber(dst.head, String(ev.damage), color, false);
      if (ev.kill) dst.anim?.play("hit");
      return;
    }
    const sk = ev.skillId ? this.catalog.get(ev.skillId) : undefined;
    const isArea = sk?.target === SkillTarget.AREA; // a animação e o nome já vieram no SkillCast
    // Skills com área em volta do alvo mandam um evento por inimigo: a animação sai uma vez só.
    const key = `${ev.sourceId}:${ev.skillId}`;
    const now = performance.now();
    const repeat = !!sk && sk.splash > 0 && now - (this.lastSplash.get(key) ?? 0) < 250;
    if (sk && sk.splash > 0 && !repeat) this.lastSplash.set(key, now);
    if (ev.heal && src) this.fx.damageNumber(src.head, `+${ev.heal}`, "#7dff9a", false);
    if (ev.absorbed && !ev.miss) this.fx.damageNumber(dst.head, `(${ev.absorbed} absorvido)`, "#9fc6ff", false);
    if (repeat) {
      window.setTimeout(() => this.landHit(ev, src, dst, sk, true), 650);
      return;
    }

    if (src && src !== dst && !isArea) {
      this.face(src, dst);
      src.anim?.play(ev.skillId ? "skill" : "attack");
      if (sk && sk.target === SkillTarget.ENEMY) this.fx.skillName(src.head, sk.name);
    }

    const land = () => this.landHit(ev, src, dst, sk);
    if (!src || src === dst) return land();
    const from = src.root.position.add(new Vector3(0, 1.3 * src.scale, 0));
    const to = dst.root.position.add(new Vector3(0, 1 * dst.scale, 0));
    const travel = Vector3.Distance(from, to) / PROJECTILE_SPEED;
    const bolt = BOLT_COLOR[sk?.fx ?? ""];
    switch (sk?.fx) {
      case "fireball":
        this.fx.fireball(from, to, travel);
        window.setTimeout(land, travel * 1000);
        return;
      case "m_rain":
        this.fx.emberRain(dst.root.position.clone(), sk.splash);
        window.setTimeout(land, 650);
        return;
      case "m_meteor":
        this.fx.meteor(dst.root.position.clone(), sk.splash, ev.sourceId === this.localId);
        window.setTimeout(land, 760);
        return;
    }
    if (bolt) {
      this.fx.bolt(from, to, travel, bolt);
      window.setTimeout(land, travel * 1000);
      return;
    }
    // Ataque básico de quem usa magia (magos e esqueletos magos).
    if (!ev.skillId && isCaster(src.model)) {
      this.fx.arcaneBolt(from, to, travel);
      window.setTimeout(land, travel * 1000);
      return;
    }
    land();
  }

  /** Número de dano e reação de quem levou o golpe. */
  private landHit(ev: CombatEvent, src: View | undefined, dst: View, sk: SkillInfo | undefined, quiet = false): void {
    if (!this.views.has(dst.id)) return;
    const mine = ev.sourceId === this.localId;
    const onMe = ev.targetId === this.localId;
    if (ev.miss) {
      this.fx.damageNumber(dst.head, "Errou", "#b9b4c2", false);
      return;
    }
    // Impacto (a Bola de Fogo e a magia básica já explodiram ao chegar).
    const at = dst.root.position.add(new Vector3(0, 1 * dst.scale, 0));
    const from = src ? src.root.position.add(new Vector3(0, 1 * src.scale, 0)) : at;
    switch (quiet ? "quiet" : sk?.fx) {
      case "quiet":
      case "m_rain":
      case "m_meteor":
      case "m_lance":
      case "m_prison":
      case "r_mark":
        break;
      case "r_flask":
      case "r_plague":
        this.fx.poisonSplash(dst.root.position.clone(), sk?.splash || 1.5, sk?.fx === "r_plague" ? new Color3(0.35, 0.55, 0.2) : new Color3(0.45, 0.9, 0.3));
        break;
      case "k_judg":
      case "b_exec":
        this.fx.brutalStrike(from, at, mine);
        this.fx.slam(dst.root.position, 1.8, new Color3(1, 0.85, 0.4), mine);
        break;
      case "k_charge":
        this.fx.shieldBash(at, mine);
        break;
      case "r_twin":
      case "r_para":
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

  /** Skill em si mesmo ou em área (o servidor avisa mesmo quando não acerta ninguém). */
  onSkillCast(ev: SkillCast): void {
    const src = this.views.get(ev.sourceId);
    const sk = this.catalog.get(ev.skillId);
    if (!src || !sk) return;
    this.fx.skillName(src.head, sk.name);
    // Skill de alvo sem dano direto (Frasco Tóxico, Praga): projétil até o alvo.
    const dst = ev.targetId ? this.views.get(ev.targetId) : undefined;
    if (dst) {
      if (src !== dst) this.face(src, dst);
      src.anim?.play("skill");
      const from = src.root.position.add(new Vector3(0, 1.3 * src.scale, 0));
      const to = dst.root.position.add(new Vector3(0, 1 * dst.scale, 0));
      const travel = Vector3.Distance(from, to) / PROJECTILE_SPEED;
      const color = BOLT_COLOR[sk.fx] ?? new Color3(0.8, 0.8, 1);
      this.fx.bolt(from, to, travel, color, () => {
        if (sk.fx === "r_flask" || sk.fx === "r_plague") this.fx.poisonSplash(dst.root.position.clone(), sk.splash || 1.5, color);
      });
      return;
    }
    src.anim?.play(sk.target === SkillTarget.AREA ? "skill" : "buff");
    const mine = ev.sourceId === this.localId;
    switch (sk.fx) {
      case "ice_nova":
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
        this.fx.whirl(src, sk.range, mine);
        break;
      case "b_quake":
      case "b_stomp":
        this.fx.slam(src.root.position, sk.range, new Color3(1, 0.7, 0.35), mine);
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
    v.effects.get(ev.effect)?.visual?.stop(); // renovado: troca o visual
    if (ev.effect === Effect.INVISIBLE) this.setGhost(v, true);
    const visual = this.fx.effect(ev.effect, v, ms);
    v.effects.set(ev.effect, { effect: ev.effect, until: performance.now() + ms, durationMs: ms, value: ev.value, visual });
  }

  onEffectEnd(ev: EffectEnd): void {
    const v = this.views.get(ev.targetId);
    const ef = v?.effects.get(ev.effect);
    if (!v || !ef) return;
    ef.visual?.stop();
    v.effects.delete(ev.effect);
    if (ev.effect === Effect.INVISIBLE) this.setGhost(v, false);
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

  onDeath(id: number): void {
    const v = this.views.get(id);
    if (!v) return;
    this.clearEffects(v);
    v.dead = true;
    v.hp = 0;
    v.target = { ...v.server };
    this.showDead(v, true);
    this.refreshHpBar(v);
  }

  onRevive(r: Revive): void {
    const v = this.views.get(r.id);
    if (!v) return;
    v.dead = false;
    v.hp = r.hp;
    v.maxHp = r.maxHp;
    v.server = { x: r.x, z: r.z };
    v.target = { x: r.x, z: r.z };
    v.root.position.set(r.x, 0, r.z);
    if (r.id === this.localId) this.localPending = null;
    this.showDead(v, false);
    this.refreshHpBar(v);
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
  }

  /** Chamado a cada frame. */
  update(dt: number): void {
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

      // Aproxima suavemente a posição desenhada da posição extrapolada.
      const pos = v.root.position;
      pos.x += (v.server.x - pos.x) * blend;
      pos.z += (v.server.z - pos.z) * blend;

      const moving = dist > 0.05 && !v.dead;
      if (moving) v.visual.rotation.y = lerpAngle(v.visual.rotation.y, Math.atan2(dx, dz), blend);
      if (v.anim) {
        // LOD de animação: quem está fora da tela ou longe não tem os ossos recalculados.
        let active = always;
        if (!active) {
          const near = !cam || Vector3.DistanceSquared(cam, pos) < ANIM_DISTANCE * ANIM_DISTANCE;
          active = near && (!planes || v.meshes.some((m) => m.isInFrustum(planes)));
        }
        v.anim.update(dt, moving, Math.max(0.6, v.speed / RUN_ANIM_SPEED), active);
      }
    }
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

  /** Ponto onde o jogador local está de fato (posição prevista, à frente da desenhada). */
  localMovePosition(): { x: number; z: number } | undefined {
    const v = this.local;
    return v ? { x: v.server.x, z: v.server.z } : undefined;
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

  private face(src: View, dst: View): void {
    const dx = dst.root.position.x - src.root.position.x;
    const dz = dst.root.position.z - src.root.position.z;
    if (Math.hypot(dx, dz) > 0.01) src.visual.rotation.y = Math.atan2(dx, dz);
  }

  private showDead(v: View, dead: boolean): void {
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
    v.hpBar.isVisible = !isLocal && !v.dead && v.kind !== EntityKind.NPC && (v.id === this.targetId || frac < 1);
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
    v.anim = new AnimController(inst.anims, inst.resetPose);
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
