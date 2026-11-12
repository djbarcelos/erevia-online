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
  SkillTarget,
  type CombatEvent,
  type EffectEnd,
  type EffectEvent,
  type Appearance,
  type Entity,
  type EntityState,
  type HpUpdate,
  type LevelUp,
  type Revive,
  type SkillCast,
  type SkillInfo,
} from "../gen/erevia/v1/game_pb";
import { AnimController } from "./anim";
import type { Fx, FxHandle } from "./fx";
import type { CharacterInstance, CharacterModels } from "./models";
import type { Collision } from "./world-data";

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
}

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
}

interface View extends EntityInfo {
  model: string;
  /** Armas na mão (jogadores): modelo da principal, da outra mão e raridade. */
  weapon: string;
  offhand: string;
  grade: number;
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
  private selectRing: Mesh;
  private models: CharacterModels | null = null;
  private targetId = 0;
  private catalog = new Map<number, SkillInfo>();

  constructor(
    private scene: Scene,
    private gui: AdvancedDynamicTexture,
    readonly localId: number,
    private zoneHalf: number,
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

  isNpc(id: number): boolean {
    return this.views.get(id)?.kind === EntityKind.NPC;
  }

  /** Nó raiz (posição no chão) de uma entidade. */
  rootOf(id: number): TransformNode | undefined {
    return this.views.get(id)?.root;
  }

  /** Ponto acima da cabeça (para textos flutuantes). */
  headOf(id: number): TransformNode | undefined {
    return this.views.get(id)?.head;
  }

  /** A arma de alguém mudou. */
  setAppearance(a: Appearance): void {
    const v = this.views.get(a.id);
    if (!v) return;
    v.weapon = a.weapon;
    v.offhand = a.offhand;
    v.grade = a.weaponGrade;
    if (!v.model3d) return;
    this.setShadow(v, false); // as malhas mudam; a sombra volta no próximo quadro
    v.model3d.setWeapons(v.weapon, v.offhand, v.grade);
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
    for (const v of this.views.values()) this.attachModel(v);
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
    const hit = MeshBuilder.CreateCylinder(`hit${e.id}`, { height: 2.1 * scale, diameter: 1.2 * scale, tessellation: 10 }, this.scene);
    hit.parent = root;
    hit.position.y = 1.05 * scale;
    hit.isVisible = false;
    hit.isPickable = true;
    hit.metadata = { entityId: e.id };

    if (isLocal) {
      const ring = MeshBuilder.CreateTorus(`ring${e.id}`, { diameter: 1.1, thickness: 0.06, tessellation: 32 }, this.scene);
      ring.parent = root;
      ring.position.y = 0.03;
      ring.material = this.ringMat;
      ring.isPickable = false;
    }

    const head = new TransformNode(`head${e.id}`, this.scene);
    head.parent = root;
    head.position.y = 2.25 * scale;

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
    nameText.color = isLocal ? "#f0d79a" : e.kind === EntityKind.MONSTER ? "#ffb4a8" : e.kind === EntityKind.NPC ? "#a8e6a0" : "#e6ecff";
    // Contorno em vez de shadowBlur: o desfoque do canvas 2D é caro e a GUI redesenha muito.
    nameText.outlineWidth = 3;
    nameText.outlineColor = "rgba(0,0,0,0.85)";
    stack.addControl(nameText);
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
      model: e.model, boss: e.boss, scale, speed: e.speed, weapon: e.weapon, offhand: e.offhand, grade: e.weaponGrade,
      server: { x: e.x, z: e.z },
      target: { x: e.targetX, z: e.targetZ },
      root, visual, head, capsule: null, model3d: null, anim: null, meshes: [], castsShadow: false,
      label, nameText, hpBar, hpFill, lastHurt: 0, effects: new Map(),
    };
    this.views.set(e.id, v);
    // NPCs ficam parados, virados para o centro da praça.
    if (e.kind === EntityKind.NPC) visual.rotation.y = Math.atan2(-e.x, -e.z);
    this.refreshLabel(v);
    this.refreshHpBar(v);

    if (this.models) this.attachModel(v);
    if (!v.model3d) v.capsule = this.createCapsule(v);
    if (v.dead) this.showDead(v, true);
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
    const cx = clamp(x, -this.zoneHalf, this.zoneHalf);
    const cz = clamp(z, -this.zoneHalf, this.zoneHalf);
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
    const sk = ev.skillId ? this.catalog.get(ev.skillId) : undefined;
    const isArea = sk?.target === SkillTarget.AREA; // a animação e o nome já vieram no SkillCast

    if (src && src !== dst && !isArea) {
      this.face(src, dst);
      src.anim?.play(ev.skillId ? "skill" : "attack");
      if (sk && sk.target === SkillTarget.ENEMY) this.fx.skillName(src.head, sk.name);
    }

    // Projétil: Bola de Fogo, ou ataque básico de quem usa magia (magos e esqueletos magos).
    const ranged = !!src && src !== dst && (sk?.fx === "fireball" || (!ev.skillId && isCaster(src.model)));
    const land = () => this.landHit(ev, src, dst, sk);
    if (ranged && src) {
      const from = src.root.position.add(new Vector3(0, 1.3 * src.scale, 0));
      const to = dst.root.position.add(new Vector3(0, 1 * dst.scale, 0));
      const travel = Vector3.Distance(from, to) / PROJECTILE_SPEED;
      if (sk) this.fx.fireball(from, to, travel);
      else this.fx.arcaneBolt(from, to, travel);
      window.setTimeout(land, travel * 1000);
    } else {
      land();
    }
  }

  /** Número de dano e reação de quem levou o golpe. */
  private landHit(ev: CombatEvent, src: View | undefined, dst: View, sk: SkillInfo | undefined): void {
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
    switch (sk?.fx) {
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
    this.fx.damageNumber(dst.head, ev.crit ? `${ev.damage}!` : String(ev.damage), color, ev.crit || !!sk);
    if (!ev.kill && !dst.dead) dst.anim?.play("hit");
  }

  /** Skill em si mesmo ou em área (o servidor avisa mesmo quando não acerta ninguém). */
  onSkillCast(ev: SkillCast): void {
    const src = this.views.get(ev.sourceId);
    const sk = this.catalog.get(ev.skillId);
    if (!src || !sk) return;
    this.fx.skillName(src.head, sk.name);
    src.anim?.play(sk.target === SkillTarget.AREA ? "skill" : "buff");
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
    }
  }

  onEffect(ev: EffectEvent): void {
    const v = this.views.get(ev.targetId);
    if (!v || v.dead) return;
    const ms = ev.durationMs;
    v.effects.get(ev.effect)?.visual?.stop(); // renovado: troca o visual
    const visual = this.fx.effect(ev.effect, v, ms);
    v.effects.set(ev.effect, { effect: ev.effect, until: performance.now() + ms, durationMs: ms, value: ev.value, visual });
  }

  onEffectEnd(ev: EffectEnd): void {
    const v = this.views.get(ev.targetId);
    const ef = v?.effects.get(ev.effect);
    if (!v || !ef) return;
    ef.visual?.stop();
    v.effects.delete(ev.effect);
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

  /** Entidades vivas para o mapa (posição desenhada, não a do servidor). */
  forEachMarker(cb: (m: MapMarker) => void): void {
    for (const v of this.views.values()) {
      if (v.dead) continue;
      cb({
        id: v.id, x: v.root.position.x, z: v.root.position.z, kind: v.kind, boss: v.boss,
        local: v.id === this.localId, target: v.id === this.targetId, heading: v.visual.rotation.y,
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
      v.kind === EntityKind.MONSTER ? `${v.name}  Nv ${v.level}` : v.kind === EntityKind.NPC ? `${v.title} ${v.name}` : v.name;
  }

  private refreshHpBar(v: View): void {
    const frac = v.maxHp > 0 ? Math.max(0, Math.min(1, v.hp / v.maxHp)) : 0;
    v.hpFill.width = frac;
    const isLocal = v.id === this.localId;
    v.hpBar.isVisible = !isLocal && !v.dead && v.kind !== EntityKind.NPC && (v.id === this.targetId || frac < 1);
  }

  private attachModel(v: View): void {
    if (v.model3d || !this.models) return;
    const inst = this.models.instantiate(v.model, String(v.id));
    if (!inst) return;
    if (v.kind === EntityKind.PLAYER) inst.setWeapons(v.weapon, v.offhand, v.grade);
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
const SKILL_NUMBER_COLOR: Record<string, string> = {
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
