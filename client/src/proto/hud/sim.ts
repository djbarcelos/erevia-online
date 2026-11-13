/**
 * Simulação local do jogo para o protótipo do HUD (sem servidor): o mundo 3D real,
 * o personagem, alguns esqueletos, ataque automático, skills com os efeitos do jogo,
 * IA simples dos monstros e a auto-caça. Nada disso é a lógica do servidor; serve para o
 * HUD ter o que mostrar.
 */

import { Color3, Mesh, MeshBuilder, TransformNode, Vector3 } from "@babylonjs/core";
import { Rectangle, StackPanel, TextBlock } from "@babylonjs/gui";
import { AnimController } from "../../world/anim";
import { DayNight } from "../../world/daynight";
import { loadEnvironment, type Environment } from "../../world/environment";
import { Fx, type FxBody } from "../../world/fx";
import { CharacterModels, type CharacterInstance } from "../../world/models";
import { createWorldScene, type WorldScene } from "../../world/scene";
import { loadWorldData, type WorldData } from "../../world/world-data";
import { ITEMS } from "../data/items";
import { game, xpNeeded } from "../data/mock";
import type { SkillNode } from "../data/skilltree";
import { settings } from "../../ui/kit";

export interface Unit {
  id: number;
  name: string;
  level: number;
  monster: boolean;
  boss: boolean;
  model: string;
  x: number;
  z: number;
  heading: number;
  hp: number;
  maxHp: number;
  dead: boolean;
  /** Ponto de nascimento (monstros). */
  home: { x: number; z: number };
  atkAt: number;
  respawnAt: number;
  aggro: boolean;
  root: TransformNode;
  head: TransformNode;
  scale: number;
  inst: CharacterInstance | null;
  anim: AnimController | null;
  pick: Mesh;
  label: Rectangle | null;
  hpFill: Rectangle | null;
  hpBar: Rectangle | null;
  nameText: TextBlock | null;
  /** Efeitos ativos (buffs/debuffs) por id → fim (ms). */
  effects: Map<string, { until: number; total: number; name: string; icon: string; debuff: boolean }>;
}

export interface SimEvents {
  onLog(text: string, kind?: "loot" | "xp" | "system"): void;
  onDeath(): void;
  onLevel(): void;
}

const MONSTERS = [
  { name: "Esqueleto Errante", model: "skeleton_minion", level: 12, hp: 620, x: -6, z: 64 },
  { name: "Esqueleto Guerreiro", model: "skeleton_warrior", level: 14, hp: 880, x: 7, z: 68 },
  { name: "Esqueleto Ladino", model: "skeleton_rogue", level: 13, hp: 700, x: -11, z: 76 },
  { name: "Esqueleto Arcano", model: "skeleton_mage", level: 14, hp: 640, x: 12, z: 80 },
  { name: "Esqueleto Errante", model: "skeleton_minion", level: 12, hp: 620, x: 2, z: 86 },
  { name: "Capitão Ossudo", model: "skeleton_warrior", level: 16, hp: 2600, x: -2, z: 98, boss: true },
];

const LOOT = ["mt_bone", "mt_bone", "mt_iron", "pt_hp", "mt_mithril", "mt_gem"];

function isCaster(model: string) {
  return model === "mage" || model === "skeleton_mage";
}

export class Sim {
  world!: WorldScene;
  data!: WorldData;
  fx!: Fx;
  dayNight!: DayNight;
  env: Environment | null = null;
  models: CharacterModels | null = null;
  me!: Unit;
  mp = 0;
  units: Unit[] = [];
  targetId = 0;
  /** Destino de movimento por clique (ou null). */
  dest: { x: number; z: number } | null = null;
  /** Direção do joystick/teclado (relativa à câmera), ou null. */
  stick: { x: number; y: number } | null = null;
  attacking = false;
  /** Skill esperando o alcance para ser usada. */
  queued: SkillNode | null = null;
  cooldowns = new Map<string, { at: number; total: number }>();
  potionAt = 0;
  auto = false;
  autoStats = { kills: 0, since: 0, xp: 0 };
  phase = 0.45;
  ping = 24;
  private nextId = 1;
  private last = performance.now();
  private disposed = false;
  private tickTimer = 0;
  private combatAt = 0;

  constructor(private ev: SimEvents) {}

  async init(canvas: HTMLCanvasElement, onProgress: (text: string) => void): Promise<void> {
    onProgress("Carregando o mundo…");
    this.data = await loadWorldData();
    this.world = await createWorldScene(canvas, this.data);
    if (this.disposed) return;
    this.dayNight = new DayNight(this.world, this.data);
    this.fx = new Fx(this.world.scene, this.world.gui, this.dayNight.glow, this.world.camera);
    this.world.camera.alpha = -Math.PI / 2;
    this.world.camera.beta = 1.05;
    this.world.camera.radius = 14;

    loadEnvironment(this.world.scene, this.data, this.world.shadows)
      .then((e) => (this.env = e))
      .catch((err) => console.warn("cenário indisponível", err));

    const h = game.hero;
    const st = game.stats();
    this.me = this.makeUnit({ name: h.name, level: h.level, model: h.cls, hp: st.maxHp, x: 0, z: 52, monster: false });
    this.mp = st.maxMp * 0.85;
    this.me.hp = st.maxHp * 0.92;
    for (const m of MONSTERS) this.units.push(this.makeUnit({ ...m, monster: true }));

    onProgress("Carregando personagens…");
    try {
      this.models = await CharacterModels.load(this.world.scene);
    } catch (err) {
      console.warn("modelos indisponíveis", err);
    }
    if (this.disposed) return;
    for (const u of [this.me, ...this.units]) this.attachModel(u);

    this.world.scene.onBeforeRenderObservable.add(() => this.frame());
  }

  dispose(): void {
    this.disposed = true;
    try {
      this.world?.engine.stopRenderLoop();
      this.world?.scene.dispose();
      this.world?.engine.dispose();
    } catch {
      // já descartado
    }
  }

  // ------------------------------------------------------------------------------------
  // Criação
  // ------------------------------------------------------------------------------------

  private makeUnit(o: { name: string; level: number; model: string; hp: number; x: number; z: number; monster: boolean; boss?: boolean }): Unit {
    const scene = this.world.scene;
    const id = this.nextId++;
    const scale = o.boss ? 1.35 : 1;
    const root = new TransformNode(`u${id}`, scene);
    root.position.set(o.x, 0, o.z);
    const head = new TransformNode(`head${id}`, scene);
    head.parent = root;
    head.position.y = 2.25 * scale;
    const pick = MeshBuilder.CreateCylinder(`pick${id}`, { height: 2 * scale, diameter: 1.2 * scale }, scene);
    pick.parent = root;
    pick.position.y = scale;
    pick.visibility = 0;
    pick.isPickable = true;
    pick.metadata = { unitId: id };
    const u: Unit = {
      id,
      name: o.name,
      level: o.level,
      monster: o.monster,
      boss: !!o.boss,
      model: o.model,
      x: o.x,
      z: o.z,
      heading: o.monster ? Math.PI : 0,
      hp: o.hp,
      maxHp: o.hp,
      dead: false,
      home: { x: o.x, z: o.z },
      atkAt: 0,
      respawnAt: 0,
      aggro: false,
      root,
      head,
      scale,
      inst: null,
      anim: null,
      pick,
      label: null,
      hpFill: null,
      hpBar: null,
      nameText: null,
      effects: new Map(),
    };
    if (o.monster) this.makeLabel(u);
    return u;
  }

  private makeLabel(u: Unit): void {
    const label = new Rectangle(`label${u.id}`);
    label.adaptWidthToChildren = true;
    label.adaptHeightToChildren = true;
    label.thickness = 0;
    const stack = new StackPanel();
    label.addControl(stack);
    const name = new TextBlock();
    name.resizeToFit = true;
    name.fontSize = 13;
    name.paddingLeft = name.paddingRight = "6px";
    name.color = u.boss ? "#e6a6ff" : "#ffb4a8";
    name.outlineWidth = 3;
    name.outlineColor = "rgba(0,0,0,0.85)";
    name.text = `${u.name}  Nv ${u.level}`;
    stack.addControl(name);
    const bar = new Rectangle();
    bar.width = "64px";
    bar.height = "6px";
    bar.thickness = 1;
    bar.color = "rgba(0,0,0,0.8)";
    bar.background = "rgba(20,10,10,0.75)";
    bar.paddingTop = "2px";
    const fill = new Rectangle();
    fill.horizontalAlignment = Rectangle.HORIZONTAL_ALIGNMENT_LEFT;
    fill.thickness = 0;
    fill.background = "#d8443a";
    fill.width = "100%";
    bar.addControl(fill);
    stack.addControl(bar);
    this.world.gui.addControl(label);
    label.linkWithMesh(u.head);
    label.linkOffsetY = -6;
    u.label = label;
    u.hpFill = fill;
    u.hpBar = bar;
    u.nameText = name;
  }

  private attachModel(u: Unit): void {
    if (!this.models) return;
    const look = game.look();
    const inst = u.monster
      ? this.models.instantiate(u.model, `m${u.id}`)
      : this.models.instantiate(u.model, `m${u.id}`, { "handslot.r": look.right, "handslot.l": look.left });
    if (!inst) return;
    inst.root.parent = u.root;
    inst.root.scaling.scaleInPlace(u.scale);
    for (const m of inst.root.getChildMeshes()) {
      m.isPickable = false;
      this.world.shadows.addShadowCaster(m);
    }
    u.inst = inst;
    u.anim = new AnimController(inst.anims, inst.resetPose, inst.impacts);
  }

  /** Recria o modelo do jogador (ex.: trocou a arma no inventário). */
  refreshPlayerModel(): void {
    if (!this.models || !this.me) return;
    const rot = this.me.inst?.root.rotation.y ?? this.me.heading;
    this.me.inst?.dispose();
    this.me.inst = null;
    this.me.anim = null;
    this.attachModel(this.me);
    // Variável nova: o TS não enxerga que attachModel preencheu inst/anim.
    const me: Unit = this.me;
    if (me.inst) me.inst.root.rotation.y = rot;
    if (me.dead) me.anim?.play("death", { hold: true });
  }

  // ------------------------------------------------------------------------------------
  // Consultas
  // ------------------------------------------------------------------------------------

  unit(id: number): Unit | undefined {
    return id === this.me?.id ? this.me : this.units.find((u) => u.id === id);
  }

  target(): Unit | undefined {
    const t = this.units.find((u) => u.id === this.targetId);
    return t && !t.dead ? t : t;
  }

  dist(a: { x: number; z: number }, b: { x: number; z: number }): number {
    return Math.hypot(a.x - b.x, a.z - b.z);
  }

  /** Monstro mais próximo vivo (opcionalmente dentro de um raio). */
  nearest(maxDist = Infinity, exclude = 0): Unit | null {
    let best: Unit | null = null;
    let bd = maxDist;
    for (const u of this.units) {
      if (u.dead || u.id === exclude) continue;
      const d = this.dist(u, this.me);
      if (d < bd) {
        bd = d;
        best = u;
      }
    }
    return best;
  }

  /** Unidade sob um ponto da tela (coordenadas do canvas em px CSS). */
  pickAt(x: number, y: number): Unit | null {
    const scene = this.world.scene;
    const pick = scene.pick(x, y, (m) => m.metadata?.unitId !== undefined);
    if (!pick?.hit || !pick.pickedMesh) return null;
    const u = this.unit(pick.pickedMesh.metadata.unitId as number);
    return u && u.monster ? u : null;
  }

  groundAt(x: number, y: number): { x: number; z: number } | null {
    const pick = this.world.scene.pick(x, y, (m) => m === this.world.ground);
    return pick?.hit && pick.pickedPoint ? { x: pick.pickedPoint.x, z: pick.pickedPoint.z } : null;
  }

  /** Direção da câmera no mapa: 0 = norte, horário. */
  cameraHeading(): number {
    const a = this.world.camera.alpha;
    return Math.atan2(-Math.cos(a), -Math.sin(a));
  }

  // ------------------------------------------------------------------------------------
  // Comandos
  // ------------------------------------------------------------------------------------

  setTarget(id: number): void {
    this.targetId = id;
  }

  attackTarget(id?: number): void {
    if (this.me.dead) return;
    if (id) this.targetId = id;
    const t = this.units.find((u) => u.id === this.targetId && !u.dead);
    if (!t) {
      const n = this.nearest(30);
      if (!n) return;
      this.targetId = n.id;
    }
    this.attacking = true;
    this.dest = null;
  }

  moveTo(x: number, z: number): void {
    if (this.me.dead) return;
    this.dest = { x, z };
    this.attacking = false;
    this.queued = null;
  }

  cycleTarget(): void {
    const alive = this.units.filter((u) => !u.dead).sort((a, b) => this.dist(a, this.me) - this.dist(b, this.me));
    if (!alive.length) return;
    const i = alive.findIndex((u) => u.id === this.targetId);
    this.targetId = alive[(i + 1) % alive.length].id;
  }

  isReady(id: string): boolean {
    const c = this.cooldowns.get(id);
    return !c || performance.now() >= c.at;
  }

  /** Usa uma skill. Devolve uma mensagem de erro (ou null se deu certo/entrou na fila). */
  useSkill(n: SkillNode): string | null {
    if (this.me.dead) return "Você está morto.";
    if (n.kind !== "active") return "Skill passiva.";
    if (!this.isReady(n.id)) return "Ainda em recarga.";
    if (this.mp < (n.mp ?? 0)) return "Mana insuficiente.";
    if (n.sim === "melee" || n.sim === "magic") {
      const t = this.units.find((u) => u.id === this.targetId && !u.dead);
      if (!t) {
        const near = this.nearest(25);
        if (!near) return "Selecione um monstro como alvo.";
        this.targetId = near.id;
      }
      this.queued = n;
      this.attacking = true;
      this.dest = null;
      return null;
    }
    this.cast(n, null);
    return null;
  }

  usePotion(kind: "hp" | "mp" = "hp"): string | null {
    if (this.me.dead) return "Você está morto.";
    const id = kind === "hp" ? "pt_hp" : "pt_mp";
    if (performance.now() < this.potionAt) return "Poção em recarga.";
    if (!game.consume(id)) return `Sem ${ITEMS[id].name}.`;
    const st = game.stats();
    const use = ITEMS[id].use!;
    if (use.hp) {
      this.me.hp = Math.min(st.maxHp, this.me.hp + use.hp);
      this.fx.damageNumber(this.me.head, `+${use.hp}`, "#7dff9a", false);
    }
    if (use.mp) {
      this.mp = Math.min(st.maxMp, this.mp + use.mp);
      this.fx.damageNumber(this.me.head, `+${use.mp}`, "#7fbaff", false);
    }
    this.fx.castGlow(this.me.root.position, kind === "hp" ? new Color3(1, 0.3, 0.25) : new Color3(0.3, 0.55, 1));
    this.potionAt = performance.now() + 3000;
    this.cooldowns.set(`item:${id}`, { at: this.potionAt, total: 3000 });
    this.cooldowns.set(`item:${kind === "hp" ? "pt_mp" : "pt_hp"}`, { at: this.potionAt, total: 3000 });
    return null;
  }

  respawn(): void {
    const st = game.stats();
    this.me.dead = false;
    this.me.hp = st.maxHp;
    this.mp = st.maxMp;
    this.me.x = 0;
    this.me.z = 46;
    this.me.anim?.reset();
    this.attacking = false;
    this.dest = null;
    for (const u of this.units) u.aggro = false;
    this.fx.levelUp(this.me.root);
  }

  /** Só para a prévia da tela de morte. */
  killMe(): void {
    this.me.hp = 0;
    this.die(this.me);
  }

  setAuto(on: boolean): void {
    this.auto = on;
    if (on) this.autoStats = { kills: 0, since: performance.now(), xp: 0 };
    else {
      this.attacking = false;
      this.queued = null;
    }
  }

  // ------------------------------------------------------------------------------------
  // Laço
  // ------------------------------------------------------------------------------------

  private frame(): void {
    const now = performance.now();
    const dt = Math.min(0.1, (now - this.last) / 1000);
    this.last = now;
    const st = game.stats();
    const me = this.me;

    // Relógio do mundo: um dia inteiro em 20 minutos.
    this.phase = (this.phase + dt / 1200) % 1;
    this.ping = Math.round(22 + Math.sin(now / 3000) * 4 + Math.random() * 3);

    // Movimento do jogador
    let moving = false;
    if (!me.dead) {
      let vx = 0;
      let vz = 0;
      if (this.stick && (this.stick.x || this.stick.y)) {
        const a = this.world.camera.alpha;
        // Frente = direção para onde a câmera olha; direita = perpendicular.
        const fx = -Math.cos(a);
        const fz = -Math.sin(a);
        const rx = fz;
        const rz = -fx;
        vx = fx * -this.stick.y + rx * this.stick.x;
        vz = fz * -this.stick.y + rz * this.stick.x;
        const len = Math.hypot(vx, vz);
        if (len > 1) {
          vx /= len;
          vz /= len;
        }
        this.dest = null;
        this.attacking = false;
        this.queued = null;
      } else {
        const goal = this.moveGoal();
        if (goal) {
          const dx = goal.x - me.x;
          const dz = goal.z - me.z;
          const d = Math.hypot(dx, dz);
          if (d > goal.stop) {
            vx = dx / d;
            vz = dz / d;
          } else if (this.dest && !this.attacking) this.dest = null;
        }
      }
      if (vx || vz) {
        const speed = st.speed * this.speedMult();
        me.x += vx * speed * dt;
        me.z += vz * speed * dt;
        me.heading = Math.atan2(vx, vz);
        moving = true;
      }
    }
    me.root.position.set(me.x, 0, me.z);
    if (me.inst) me.inst.root.rotation.y = lerpAngle(me.inst.root.rotation.y, me.heading, 1 - Math.exp(-12 * dt));
    me.anim?.update(dt, moving, this.speedMult());

    // Combate do jogador
    if (!me.dead) this.playerCombat(now, st);

    // Monstros
    for (const u of this.units) this.monsterTick(u, now, dt, st);

    // Regeneração (mais rápida fora de combate)
    if (!me.dead) {
      const k = now - this.combatAt > 5000 ? 3 : 1;
      me.hp = Math.min(st.maxHp, me.hp + st.hpRegen * k * dt);
      this.mp = Math.min(st.maxMp, this.mp + st.mpRegen * k * dt);
    }

    // Efeitos que acabaram
    for (const u of [me, ...this.units]) for (const [k, e] of u.effects) if (now > e.until) u.effects.delete(k);

    // Auto-caça (4× por segundo)
    this.tickTimer += dt;
    if (this.auto && this.tickTimer > 0.25) {
      this.tickTimer = 0;
      this.autoTick(st);
    }

    // Câmera, sombra, dia/noite e árvores
    const cam = this.world.camera;
    cam.target.x += (me.x - cam.target.x) * Math.min(1, dt * 10);
    cam.target.y = 1.4;
    cam.target.z += (me.z - cam.target.z) * Math.min(1, dt * 10);
    this.world.followShadows(me.x, me.z);
    this.dayNight.update(this.phase, me.root.position);
    this.env?.fader.update(dt, cam.position, [me.root.position.add(new Vector3(0, 1, 0))]);

    // Rótulos
    const showNames = settings.get().showNames;
    for (const u of this.units) {
      if (!u.label) continue;
      const d = this.dist(u, me);
      u.label.isVisible = !u.dead && (showNames || u.id === this.targetId) && d < 45;
      u.hpFill!.width = `${Math.max(0, (u.hp / u.maxHp) * 100)}%`;
      u.hpBar!.isVisible = u.id === this.targetId || u.hp < u.maxHp;
      u.nameText!.color = u.id === this.targetId ? "#ffe7a8" : u.boss ? "#e6a6ff" : "#ffb4a8";
    }
  }

  private speedMult(): number {
    const e = this.me.effects.get("speed");
    return e ? 1.4 : 1;
  }

  /** Para onde andar agora: alcance do alvo (se atacando) ou o destino clicado. */
  private moveGoal(): { x: number; z: number; stop: number } | null {
    if (this.attacking) {
      const t = this.units.find((u) => u.id === this.targetId && !u.dead);
      if (!t) return null;
      const range = this.queued?.range ?? (isCaster(this.me.model) ? 11 : 2);
      return { x: t.x, z: t.z, stop: Math.max(1.4, range - 0.4) };
    }
    if (this.dest) return { ...this.dest, stop: 0.2 };
    return null;
  }

  private playerCombat(now: number, st: ReturnType<typeof game.stats>): void {
    const t = this.units.find((u) => u.id === this.targetId && !u.dead);
    if (!t || !this.attacking) {
      if (!t) this.queued = null;
      return;
    }
    const d = this.dist(t, this.me);
    const caster = isCaster(this.me.model);

    // Skill na fila
    if (this.queued) {
      const range = this.queued.range ?? 2.5;
      if (d <= range) {
        const n = this.queued;
        this.queued = null;
        this.cast(n, t);
      }
      return;
    }

    const range = caster ? 11 : 2.4;
    if (d > range) return;
    this.me.heading = Math.atan2(t.x - this.me.x, t.z - this.me.z);
    if (now < this.me.atkAt) return;
    this.me.atkAt = now + 1000 / st.atkSpeed;
    this.combatAt = now;
    this.me.anim?.play("attack");
    t.aggro = true;
    const from = this.me.head.getAbsolutePosition().clone();
    if (caster) {
      const to = t.head.getAbsolutePosition().clone().subtract(new Vector3(0, 0.6, 0));
      const travel = Math.min(0.6, d / 22);
      setTimeout(() => this.fx.arcaneBolt(from.subtract(new Vector3(0, 0.6, 0)), to, travel), 250);
      setTimeout(() => this.hit(t, st.mAtk, 1, st.crit, false), 250 + travel * 1000);
    } else {
      setTimeout(() => {
        if (t.dead) return;
        const h = this.hit(t, st.pAtk, 1, st.crit, false);
        if (h) this.fx.meleeHit(t.root.position.add(new Vector3(0, 1.1, 0)), h.crit);
      }, 300);
    }
  }

  private cast(n: SkillNode, t: Unit | null): void {
    const now = performance.now();
    const st = game.stats();
    this.mp -= n.mp ?? 0;
    const cdMs = (n.cd ?? 1) * 1000 / st.castSpeed;
    this.cooldowns.set(n.id, { at: now + cdMs, total: cdMs });
    this.fx.skillName(this.me.head, n.name);
    this.combatAt = now;
    const me = this.me;
    const body: FxBody = { root: me.root, head: me.head, scale: 1 };
    const mult = n.mult ?? 1;
    const caster = isCaster(me.model);
    const atk = caster ? st.mAtk : st.pAtk;

    if (n.sim === "self") {
      me.anim?.play("buff");
      switch (n.fx) {
        case "fortress":
          this.fx.fortress(body);
          break;
        case "fury":
          this.fx.fury(body);
          break;
        case "shadow_step":
          this.fx.shadowStep(body);
          break;
        default:
          this.fx.castGlow(me.root.position, Color3.FromHexString("#ffd36b"));
          this.fx.levelUp(me.root);
      }
      const kind = n.fx === "shadow_step" || n.id === "m_blink" ? "speed" : n.fx === "fortress" || /armor|shield|wall/.test(n.id) ? "def" : "atk";
      const icon = n.id;
      me.effects.set(kind, { until: now + 10000, total: 10000, name: n.name, icon, debuff: false });
      return;
    }

    if (!t) return;
    me.heading = Math.atan2(t.x - me.x, t.z - me.z);
    me.anim?.play("skill");
    t.aggro = true;
    const at = t.root.position.add(new Vector3(0, 1.1, 0));
    const from = me.head.getAbsolutePosition().clone().subtract(new Vector3(0, 0.5, 0));

    if (n.sim === "area") {
      const radius = n.range ?? 5;
      setTimeout(() => {
        if (n.fx === "ice_nova") this.fx.iceNova(me.root.position.clone(), radius);
        else {
          this.fx.castGlow(me.root.position, Color3.FromHexString("#ffb060"));
          this.fx.shake(6);
        }
        for (const u of this.units) {
          if (u.dead || this.dist(u, me) > radius + 1) continue;
          u.aggro = true;
          const h = this.hit(u, atk, mult, st.crit, true);
          if (h && n.fx !== "ice_nova") this.fx.meleeHit(u.root.position.add(new Vector3(0, 1.1, 0)), h.crit);
          if (n.fx === "ice_nova") {
            this.fx.frostHit(u.root.position.add(new Vector3(0, 1, 0)));
            u.effects.set("slow", { until: performance.now() + 3000, total: 3000, name: "Lentidão", icon: "slow", debuff: true });
          }
        }
      }, 350);
      return;
    }

    if (n.sim === "magic") {
      const d = this.dist(t, me);
      const travel = Math.min(0.7, d / 20);
      setTimeout(() => {
        if (n.fx === "fireball" || /meteor|rain|ember/.test(n.id)) this.fx.fireball(from, at, travel);
        else this.fx.arcaneBolt(from, at, travel);
      }, 280);
      setTimeout(() => this.hit(t, atk, mult, st.crit, true), 280 + travel * 1000);
      return;
    }

    // Corpo a corpo
    setTimeout(() => {
      if (t.dead) return;
      const extra = n.fx === "backstab" ? 0.5 : 0;
      const h = this.hit(t, atk, mult, st.crit + extra, true);
      if (!h) return;
      switch (n.fx) {
        case "shield_bash":
          this.fx.shieldBash(at, true);
          t.effects.set("stun", { until: performance.now() + 1500, total: 1500, name: "Atordoado", icon: "stun", debuff: true });
          break;
        case "brutal_strike":
          this.fx.brutalStrike(me.root.position, at, true);
          break;
        case "backstab":
          this.fx.backstab(me.root.position, at, h.crit);
          break;
        default:
          this.fx.meleeHit(at, h.crit);
          this.fx.shake(4);
      }
    }, 320);
  }

  /** Aplica um golpe. Devolve null se errou ou o alvo já morreu. */
  private hit(t: Unit, atk: number, mult: number, crit: number, skill: boolean): { crit: boolean } | null {
    if (t.dead) return null;
    const show = settings.get().showDamage;
    if (Math.random() < 0.05) {
      if (show) this.fx.damageNumber(t.head, "Errou", "#b9b4c2", false);
      return null;
    }
    const def = 20 + t.level * 3;
    let dmg = atk * mult * (100 / (100 + def)) * (0.9 + Math.random() * 0.2) * 3.2;
    const isCrit = Math.random() < crit;
    if (isCrit) dmg *= 2;
    dmg = Math.round(dmg);
    t.hp = Math.max(0, t.hp - dmg);
    if (show) this.fx.damageNumber(t.head, isCrit ? `${dmg}!` : String(dmg), isCrit ? "#ffd36b" : skill ? "#ffe7a8" : "#ffffff", isCrit || skill);
    t.anim?.play("hit");
    if (t.hp <= 0) this.die(t);
    return { crit: isCrit };
  }

  private die(u: Unit): void {
    u.dead = true;
    u.anim?.play("death", { hold: true });
    u.aggro = false;
    u.effects.clear();
    if (u === this.me) {
      this.attacking = false;
      this.queued = null;
      this.ev.onDeath();
      return;
    }
    u.respawnAt = performance.now() + (u.boss ? 30000 : 9000);
    if (this.targetId === u.id && !this.auto) this.attacking = false;
    // XP, ouro e saque
    const h = game.hero;
    const xp = Math.round((u.boss ? 4200 : 820) * (1 + (u.level - h.level) * 0.1));
    const gold = Math.round(40 + Math.random() * 80) * (u.boss ? 8 : 1);
    h.xp += xp;
    h.gold += gold;
    this.autoStats.kills++;
    this.autoStats.xp += xp;
    this.ev.onLog(`+${xp.toLocaleString("pt-BR")} XP · +${gold} ouro`, "xp");
    const drops = u.boss ? ["mt_gem", "pt_big", "mt_mithril"] : [LOOT[Math.floor(Math.random() * LOOT.length)]];
    setTimeout(() => {
      for (const d of drops) {
        const qty = d === "mt_bone" ? 1 + Math.floor(Math.random() * 3) : 1;
        game.addItem(d, qty);
        this.ev.onLog(`${ITEMS[d].name}${qty > 1 ? ` ×${qty}` : ""}`, "loot");
      }
    }, 500);
    if (h.xp >= xpNeeded(h.level)) {
      h.xp -= xpNeeded(h.level);
      h.level++;
      this.fx.levelUp(this.me.root);
      this.ev.onLevel();
    }
    game.emit("xp");
  }

  private monsterTick(u: Unit, now: number, dt: number, st: ReturnType<typeof game.stats>): void {
    const me = this.me;
    let moving = false;
    if (u.dead) {
      if (now > u.respawnAt) {
        u.dead = false;
        u.hp = u.maxHp;
        u.x = u.home.x;
        u.z = u.home.z;
        u.anim?.reset();
        u.inst?.resetPose();
      }
    } else {
      const stunned = u.effects.has("stun");
      const dMe = this.dist(u, me);
      const dHome = this.dist(u, u.home);
      if (me.dead || dHome > 32) u.aggro = false;
      if (!u.aggro && !me.dead && dMe < (u.boss ? 7 : 4.5)) u.aggro = true;
      const range = u.model === "skeleton_mage" ? 9 : 1.8;
      let gx = u.home.x;
      let gz = u.home.z;
      let stop = 0.3;
      if (u.aggro) {
        gx = me.x;
        gz = me.z;
        stop = range;
      } else if (dHome < 0.5) {
        u.hp = Math.min(u.maxHp, u.hp + u.maxHp * 0.1 * dt);
      }
      const dx = gx - u.x;
      const dz = gz - u.z;
      const d = Math.hypot(dx, dz);
      if (!stunned && d > stop) {
        const speed = (u.aggro ? 3.6 : 2.4) * (u.effects.has("slow") ? 0.5 : 1);
        u.x += (dx / d) * speed * dt;
        u.z += (dz / d) * speed * dt;
        u.heading = Math.atan2(dx, dz);
        moving = true;
      } else if (u.aggro && !stunned && !me.dead && dMe <= range + 0.3) {
        u.heading = Math.atan2(me.x - u.x, me.z - u.z);
        if (now > u.atkAt) {
          u.atkAt = now + (u.boss ? 1500 : 1900);
          u.anim?.play("attack");
          this.combatAt = now;
          const lvl = u.level;
          setTimeout(() => this.monsterHits(u, lvl, st), u.model === "skeleton_mage" ? 600 : 350);
          if (u.model === "skeleton_mage") {
            const from = u.head.getAbsolutePosition().subtract(new Vector3(0, 0.6, 0));
            this.fx.arcaneBolt(from, me.root.position.add(new Vector3(0, 1.1, 0)), 0.35);
          }
        }
      }
    }
    u.root.position.set(u.x, 0, u.z);
    if (u.inst) u.inst.root.rotation.y = lerpAngle(u.inst.root.rotation.y, u.heading, 1 - Math.exp(-10 * dt));
    const visible = this.dist(u, me) < 70;
    u.anim?.update(dt, moving, u.aggro ? 1.3 : 0.8, visible);
  }

  private monsterHits(u: Unit, lvl: number, st: ReturnType<typeof game.stats>): void {
    if (u.dead || this.me.dead) return;
    const me = this.me;
    const defMult = me.effects.has("def") ? 1.5 : 1;
    let dmg = (lvl * 7 + (u.boss ? 40 : 0)) * (100 / (100 + st.pDef * defMult * 0.5)) * (0.85 + Math.random() * 0.3);
    if (Math.random() < st.evasion / 1000) {
      if (settings.get().showDamage) this.fx.damageNumber(me.head, "Esquivou", "#b9b4c2", false);
      return;
    }
    dmg = Math.round(dmg);
    me.hp = Math.max(0, me.hp - dmg);
    if (settings.get().showDamage) this.fx.damageNumber(me.head, String(dmg), "#ff6b5b", false);
    me.anim?.play("hit");
    if (me.hp <= 0) this.die(me);
  }

  // ------------------------------------------------------------------------------------
  // Auto-caça
  // ------------------------------------------------------------------------------------

  private autoTick(st: ReturnType<typeof game.stats>): void {
    const s = settings.get();
    const me = this.me;
    if (me.dead) return;
    // Poções
    if (me.hp / st.maxHp < s.autoPotion / 100 && performance.now() >= this.potionAt) {
      if (this.usePotion("hp") === null) this.ev.onLog("Auto: Poção de Vida usada", "system");
      else if (s.autoReturn && game.countItem("pt_hp") === 0) {
        this.ev.onLog("Auto: sem poções — voltando à cidade", "system");
        this.setAuto(false);
        this.moveTo(0, 44);
        return;
      }
    } else if (this.mp / st.maxMp < 0.25 && performance.now() >= this.potionAt) {
      this.usePotion("mp");
    }
    // Alvo
    let t = this.units.find((u) => u.id === this.targetId && !u.dead);
    if (!t) {
      // Prefere quem já está batendo em você.
      t = this.units.find((u) => u.aggro && !u.dead) ?? this.nearest(s.autoRadius) ?? undefined;
      if (!t) return;
      this.targetId = t.id;
    }
    this.attacking = true;
    this.dest = null;
    // Skills: buffs que não estão ativos e golpes prontos.
    if (!s.autoSkills || this.queued) return;
    const now = performance.now();
    for (const id of game.hero.hotbar) {
      if (!id || id.startsWith("item:")) continue;
      const n = game.node(id);
      if (!n || n.kind !== "active" || (game.hero.ranks[n.id] ?? 0) < 1 || !this.isReady(n.id) || this.mp < (n.mp ?? 0)) continue;
      if (n.sim === "self") {
        if (now - this.combatAt > 3000) continue; // buffs só em combate
        this.cast(n, null);
        return;
      }
      if (n.sim === "area" && this.units.filter((u) => !u.dead && this.dist(u, me) < (n.range ?? 5)).length < 1) continue;
      this.useSkill(n);
      return;
    }
  }
}

function lerpAngle(a: number, b: number, k: number): number {
  let d = b - a;
  while (d > Math.PI) d -= Math.PI * 2;
  while (d < -Math.PI) d += Math.PI * 2;
  return a + d * k;
}
