import { AbstractMesh, Color3, Matrix, Mesh, MeshBuilder, Scene, StandardMaterial, TransformNode, Vector3 } from "@babylonjs/core";
import { AdvancedDynamicTexture, Rectangle, StackPanel, TextBlock } from "@babylonjs/gui";
import { classInfo } from "../classes";
import {
  Effect,
  EntityKind,
  type CombatEvent,
  type EffectEvent,
  type Entity,
  type EntityState,
  type HpUpdate,
  type LevelUp,
  type Revive,
} from "../gen/erevia/v1/game_pb";
import { AnimController } from "./anim";
import type { Fx } from "./fx";
import type { CharacterInstance, CharacterModels } from "./models";

/** Quão rápido a posição desenhada converge para a do servidor (1/s). */
const CORRECTION_RATE = 12;
/** Tempo máximo esperando o servidor confirmar um destino previsto. */
const PENDING_TIMEOUT_MS = 1000;
/** Velocidade de referência da animação de corrida (m/s). */
const RUN_ANIM_SPEED = 5;
/** Nomes e barras só aparecem até esta distância do jogador (o alvo sempre aparece). */
const LABEL_DISTANCE = 30;
const BONE_COLOR = "#d9d2c0";

/** Visual de cada skill (vale para skills de qualquer jogador). */
const SKILL_FX: Record<number, "melee" | "buff" | "fireball" | "nova"> = {
  1: "melee", 2: "buff", 3: "melee", 4: "buff", 5: "fireball", 6: "nova", 7: "melee", 8: "buff",
};

export interface EntityInfo {
  id: number;
  name: string;
  kind: EntityKind;
  level: number;
  hp: number;
  maxHp: number;
  dead: boolean;
}

interface View extends EntityInfo {
  model: string;
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
  label: Rectangle;
  nameText: TextBlock;
  hpBar: Rectangle;
  hpFill: Rectangle;
  lastHurt: number;
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

  constructor(
    private scene: Scene,
    private gui: AdvancedDynamicTexture,
    readonly localId: number,
    private zoneHalf: number,
    private fx: Fx,
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
    nameText.color = isLocal ? "#f0d79a" : e.kind === EntityKind.MONSTER ? "#ffb4a8" : "#e6ecff";
    nameText.shadowColor = "rgba(0,0,0,0.9)";
    nameText.shadowBlur = 3;
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
      id: e.id, name: e.name, kind: e.kind, level: e.level, hp: e.hp, maxHp: e.maxHp, dead: e.dead,
      model: e.model, scale, speed: e.speed,
      server: { x: e.x, z: e.z },
      target: { x: e.targetX, z: e.targetZ },
      root, visual, head, capsule: null, model3d: null, anim: null,
      label, nameText, hpBar, hpFill, lastHurt: 0,
    };
    this.views.set(e.id, v);
    this.refreshLabel(v);
    this.refreshHpBar(v);

    if (this.models) this.attachModel(v);
    if (!v.model3d) v.capsule = this.createCapsule(v);
    if (v.dead) this.showDead(v, true);
  }

  remove(id: number): void {
    const v = this.views.get(id);
    if (!v) return;
    if (this.targetId === id) this.setTarget(0);
    v.label.dispose();
    v.model3d?.dispose();
    v.root.dispose(false, true);
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
    v.target.x = cx;
    v.target.z = cz;
    this.localPending = { x: cx, z: cz, at: performance.now() };
    return { x: cx, z: cz };
  }

  onCombat(ev: CombatEvent): void {
    const src = this.views.get(ev.sourceId);
    const dst = this.views.get(ev.targetId);
    const self = ev.sourceId === ev.targetId;
    const kind = ev.skillId ? SKILL_FX[ev.skillId] : undefined;

    if (src && dst && !self) this.face(src, dst);

    if (src) {
      if (self) {
        if (kind === "nova") {
          src.anim?.play("skill");
          this.fx.nova(src.root.position, 6, new Color3(0.55, 0.85, 1));
        } else {
          src.anim?.play("buff");
        }
        return;
      }
      src.anim?.play(ev.skillId ? "skill" : "attack");
    }
    if (!dst) return;

    if (kind === "fireball" && src) {
      this.fx.projectile(src.root.position.add(new Vector3(0, 1.2, 0)), dst.root.position.add(new Vector3(0, 1, 0)), new Color3(1, 0.5, 0.15));
    } else if (src && src.model === "mage" && !ev.skillId) {
      this.fx.projectile(src.root.position.add(new Vector3(0, 1.4, 0)), dst.root.position.add(new Vector3(0, 1, 0)), new Color3(0.6, 0.5, 1), 0.25);
    }

    const mine = ev.sourceId === this.localId;
    const onMe = ev.targetId === this.localId;
    if (ev.miss) {
      this.fx.damageNumber(dst.head, "Errou", "#b9b4c2", false);
    } else {
      const color = onMe ? "#ff6b5a" : ev.crit ? "#ffd34d" : mine ? "#ffffff" : "#e8e1d4";
      this.fx.damageNumber(dst.head, ev.crit ? `${ev.damage}!` : String(ev.damage), color, ev.crit);
      if (!ev.kill) dst.anim?.play("hit");
    }
  }

  onEffect(ev: EffectEvent): void {
    const v = this.views.get(ev.targetId);
    if (!v) return;
    const ms = ev.durationMs;
    switch (ev.effect) {
      case Effect.STUN:
        this.fx.stun(v.head, ms);
        break;
      case Effect.SLOW:
        this.fx.aura(v.root, new Color3(0.5, 0.8, 1), ms, v.scale);
        break;
      case Effect.BUFF_DEF:
        this.fx.aura(v.root, new Color3(0.45, 0.65, 1), ms, v.scale);
        break;
      case Effect.BUFF_ATK:
        this.fx.aura(v.root, new Color3(1, 0.35, 0.25), ms, v.scale);
        break;
      case Effect.BUFF_SPEED:
        this.fx.aura(v.root, new Color3(0.4, 1, 0.55), ms, v.scale);
        break;
    }
  }

  onDeath(id: number): void {
    const v = this.views.get(id);
    if (!v) return;
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
    for (const v of this.views.values()) {
      if (me) {
        const near = Math.hypot(v.root.position.x - me.x, v.root.position.z - me.z) < LABEL_DISTANCE;
        v.label.isVisible = near || v.id === this.targetId;
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
      v.anim?.update(dt, moving, Math.max(0.6, v.speed / RUN_ANIM_SPEED));
    }
  }

  localPosition(): Vector3 | undefined {
    return this.local?.root.position;
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
    v.nameText.text = v.kind === EntityKind.MONSTER ? `${v.name}  Nv ${v.level}` : v.name;
  }

  private refreshHpBar(v: View): void {
    const frac = v.maxHp > 0 ? Math.max(0, Math.min(1, v.hp / v.maxHp)) : 0;
    v.hpFill.width = frac;
    const isLocal = v.id === this.localId;
    v.hpBar.isVisible = !isLocal && !v.dead && (v.id === this.targetId || frac < 1);
  }

  private attachModel(v: View): void {
    if (v.model3d || !this.models) return;
    const inst = this.models.instantiate(v.model, String(v.id));
    if (!inst) return;
    inst.root.parent = v.visual;
    inst.root.scaling.scaleInPlace(v.scale);
    this.makePickable(inst.root.getChildMeshes(false), v.id);
    v.model3d = inst;
    v.anim = new AnimController(inst.anims);
    v.capsule?.dispose();
    v.capsule = null;
    if (v.dead) this.showDead(v, true);
  }

  private createCapsule(v: View): Mesh {
    const body = MeshBuilder.CreateCapsule(`body${v.id}`, { height: 1.8 * v.scale, radius: 0.35 * v.scale }, this.scene);
    body.parent = v.visual;
    body.position.y = 0.9 * v.scale;
    body.material = this.material(v.kind === EntityKind.MONSTER ? BONE_COLOR : classInfo(v.model).color);
    this.makePickable([body], v.id);
    return body;
  }

  private makePickable(meshes: AbstractMesh[], id: number): void {
    for (const m of meshes) {
      m.isPickable = true;
      m.metadata = { ...(m.metadata ?? {}), entityId: id };
    }
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

function clamp(v: number, lo: number, hi: number): number {
  return Math.max(lo, Math.min(hi, v));
}

function lerpAngle(a: number, b: number, t: number): number {
  let d = (b - a) % (Math.PI * 2);
  if (d > Math.PI) d -= Math.PI * 2;
  if (d < -Math.PI) d += Math.PI * 2;
  return a + d * t;
}
