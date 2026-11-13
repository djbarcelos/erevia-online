import {
  Color3,
  Color4,
  Constants,
  CylinderDirectedParticleEmitter,
  CylinderParticleEmitter,
  HemisphericParticleEmitter,
  Material,
  Mesh,
  MeshBuilder,
  ParticleSystem,
  PointLight,
  SphereDirectedParticleEmitter,
  SphereParticleEmitter,
  StandardMaterial,
  TransformNode,
  Vector2,
  Vector3,
  type AbstractMesh,
  type ArcRotateCamera,
  type GlowLayer,
  type IParticleEmitterType,
  type Scene,
  type Texture,
} from "@babylonjs/core";
import { AdvancedDynamicTexture, TextBlock } from "@babylonjs/gui";
import { Effect } from "../../gen/erevia/v1/game_pb";
import { settings } from "../../ui/kit";
import { SKILL_FX, skillFxColors } from "../../ui/skill-themes";
import { ATLAS_CELL, CELL, createFxAtlas } from "./atlas";
import { ParticleBudget, SHORT_LIFE, type FxOwner } from "./budget";
import { HitFlash } from "./hit-flash";
import { DamageNumbers, type NumberKind } from "./numbers";
import { createFxTextures, type FxTextures } from "./textures";
import { WeaponTrails } from "./trail";

export type { NumberKind } from "./numbers";

interface Running {
  /** Devolve false quando terminou. t = segundos desde o início. */
  tick(t: number, dt: number): boolean;
  start: number;
  dispose(): void;
  stopped: boolean;
}

/** Efeito contínuo que pode ser encerrado antes da hora (ex.: o efeito acabou). */
export interface FxHandle {
  stop(): void;
}

/** O que os efeitos precisam saber de um personagem. */
export interface FxBody {
  root: TransformNode;
  head: TransformNode;
  scale: number;
}

type Range = [number, number];

interface ParticleOpts {
  tex: Texture;
  life: Range;
  size: Range;
  speed: Range;
  /** Duas cores de nascimento e a cor ao morrer. */
  colors: [Color4, Color4, Color4];
  emitter?: IParticleEmitterType;
  gravity?: number;
  additive?: boolean;
  stretched?: boolean;
  spin?: number;
  /** Tamanho no fim da vida (multiplicador). */
  grow?: number;
  /** Arrasto (0 = nenhum). */
  drag?: number;
}

const c4 = (r: number, g: number, b: number, a = 1) => new Color4(r, g, b, a);
const rnd = (a: number, b: number) => a + Math.random() * (b - a);
/** Cilindro que lança as partículas para cima (com um leve espalhamento). */
const rising = (radius: number, height: number, spread = 0.25, up = 1) =>
  new CylinderDirectedParticleEmitter(radius, height, 1, new Vector3(-spread, up, -spread), new Vector3(spread, up * 1.3, spread));

/** Distância de câmera em que o tremor foi calibrado (m): mais longe, o deslocamento cresce junto. */
const SHAKE_REF_RADIUS = 18;
/** Tremor máximo (m de deslocamento na distância de referência): "leve" mesmo somando golpes. */
const SHAKE_MAX = 0.22;
/** Faíscas de impacto: sistemas fixos reaproveitados (um impacto por sistema por quadro). */
const SPARK_POOL = 4;
/** Cartões (clarões) e marcas no chão guardados para reaproveitar. */
const QUAD_POOL = 64;

/** Sistema fixo de faíscas e o último quadro em que recebeu uma rajada. */
interface SparkSys {
  ps: ParticleSystem;
  dir: SphereDirectedParticleEmitter;
  frame: number;
}

/**
 * Efeitos visuais: partículas, brilhos, anéis e marcas no chão, com texturas pintadas
 * em código. Uma receita por skill, mais os efeitos que duram (buffs e debuffs).
 *
 * Por cima das receitas (tunnel#101): orçamento global de partículas (budget.ts), números
 * de dano num pool (numbers.ts), flash no alvo (hit-flash.ts), rastro da arma (trail.ts),
 * faíscas de impacto e poeira dos passos em sistemas fixos, e marcas e clarões em malhas
 * reaproveitadas.
 */
export class Fx {
  private running: Running[] = [];
  private mats = new Map<string, StandardMaterial>();
  private tex: FxTextures;
  private light: PointLight;
  private lightUntil = 0;
  private shakeLeft = 0;
  private shakeDur = 0.22;
  private shakePower = 0;
  private shakeT = 0;
  private shakeSeed = 0;
  private shakeOffset = new Vector2();
  /** Orçamento de partículas e de quem é o efeito que está sendo criado agora. */
  readonly budget = new ParticleBudget();
  private owner: FxOwner = "mine";
  private numbers: DamageNumbers;
  private hitFlashes = new HitFlash();
  private trails: WeaponTrails;
  private atlas: Texture;
  private sparks: SparkSys[] = [];
  private dust: ParticleSystem | null = null;
  private frame = 0;
  private freeGround: Mesh[] = [];
  private freeCards: Mesh[] = [];

  constructor(
    private scene: Scene,
    private gui: AdvancedDynamicTexture,
    private glow: GlowLayer | null = null,
    private camera: ArcRotateCamera | null = null,
  ) {
    this.tex = createFxTextures(scene);
    this.atlas = createFxAtlas(scene);
    this.numbers = new DamageNumbers(gui);
    this.trails = new WeaponTrails(scene);
    this.shakeSeed = Math.random() * 100;
    // Os sistemas fixos nascem já ligados (sem partículas): o shader compila na carga, não
    // no primeiro golpe.
    for (let i = 0; i < SPARK_POOL; i++) this.sparks.push(this.createSparkSystem());
    // Uma luz para clarões (explosões). Sol + céu + 3 tochas + esta = 6 luzes por material.
    this.light = new PointLight("fxLight", new Vector3(0, -100, 0), scene);
    this.light.intensity = 0;
    this.light.range = 12;

    let last = performance.now() / 1000;
    scene.onBeforeRenderObservable.add(() => {
      const now = performance.now() / 1000;
      const dt = Math.min(0.1, now - last);
      last = now;
      this.frame++;
      // Mede as partículas vivas antes de qualquer efeito novo pedir espaço.
      this.budget.tick(dt, now * 1000);
      // Efeitos criados durante um tick (ex.: a explosão quando a bola de fogo chega)
      // entram em this.running enquanto a lista atual é percorrida: junta os dois.
      const current = this.running;
      this.running = [];
      const kept = current.filter((r) => {
        if (!r.stopped && r.tick(now - r.start, dt)) return true;
        r.dispose();
        return false;
      });
      this.running = kept.concat(this.running);
      this.numbers.update(dt);
      this.hitFlashes.update(now * 1000);
      this.updateShake(dt);
      if (now > this.lightUntil && this.light.intensity > 0) this.light.intensity = 0;
    });
    if (new URLSearchParams(location.search).has("fxstress")) this.startStress();
  }

  // ------------------------------------------------------------------------------------
  // Dono do efeito (orçamento)
  // ------------------------------------------------------------------------------------

  /**
   * Cria os efeitos de fn como do jogador local (mine) ou de outra pessoa/monstro. Os dos
   * outros encolhem ou somem quando a tela passa do orçamento; os seus sempre aparecem.
   * Fora daqui (cenário, raid, itens) conta como seu.
   */
  as(mine: boolean, fn: () => void): void {
    const prev = this.owner;
    this.owner = mine ? "mine" : "other";
    try {
      fn();
    } finally {
      this.owner = prev;
    }
  }

  /** Embrulha um callback para criar os efeitos com o dono de agora (projéteis, atrasos). */
  private keep<A extends unknown[]>(fn: (...a: A) => void): (...a: A) => void {
    const owner = this.owner;
    return (...a: A) => this.as(owner === "mine", () => fn(...a));
  }

  private get mine(): boolean {
    return this.owner === "mine";
  }

  // ------------------------------------------------------------------------------------
  // Peças básicas
  // ------------------------------------------------------------------------------------

  private run(tick: Running["tick"], dispose: () => void): FxHandle {
    const r: Running = { tick, dispose, start: performance.now() / 1000, stopped: false };
    this.running.push(r);
    return { stop: () => (r.stopped = true) };
  }

  /** Material emissivo (aditivo ou translúcido) compartilhado por cor e textura. */
  private mat(color: Color3, tex: Texture | null, additive = true, alpha = 1): StandardMaterial {
    const key = `${color.toHexString()}-${tex?.name ?? "none"}-${additive}-${alpha}`;
    let m = this.mats.get(key);
    if (!m) {
      m = new StandardMaterial(`fx-${key}`, this.scene);
      m.emissiveColor = color;
      m.diffuseColor = Color3.Black();
      m.specularColor = Color3.Black();
      m.disableLighting = true;
      m.backFaceCulling = false;
      m.alpha = alpha;
      // A textura (forma branca com alfa) só dá o contorno: a cor é a emissiveColor.
      // (No StandardMaterial a emissiveTexture SOMA à cor, o que deixaria tudo branco.)
      if (tex) m.opacityTexture = tex;
      if (additive) m.alphaMode = Constants.ALPHA_ADD;
      m.transparencyMode = Material.MATERIAL_ALPHABLEND;
      // Efeito transparente não grava profundidade: senão um clarão (cartão quadrado)
      // esconde as partículas que estão atrás dele.
      m.disableDepthWrite = true;
      this.mats.set(key, m);
    }
    return m;
  }

  private particles(name: string, capacity: number, o: ParticleOpts): ParticleSystem {
    const ps = new ParticleSystem(name, capacity, this.scene);
    ps.particleTexture = o.tex;
    ps.updateSpeed = 1 / 60; // vida em segundos e velocidade em m/s
    ps.minLifeTime = o.life[0];
    ps.maxLifeTime = o.life[1];
    ps.minSize = o.size[0];
    ps.maxSize = o.size[1];
    ps.minEmitPower = o.speed[0];
    ps.maxEmitPower = o.speed[1];
    // Cor: segura o brilho em boa parte da vida e só então some (o padrão do Babylon
    // apagaria linearmente desde o nascimento, quase invisível de dia).
    // Aditivas: cor um pouco acima de 1 para brilhar mesmo de dia.
    const boost = o.additive === false ? 1 : 1.3;
    const lift = (c: Color4) => new Color4(c.r * boost, c.g * boost, c.b * boost, c.a);
    const [a, b] = [lift(o.colors[0]), lift(o.colors[1])];
    const dead = o.colors[2];
    [ps.color1, ps.color2, ps.colorDead] = [a, b, dead];
    ps.addColorGradient(0, a, b);
    ps.addColorGradient(0.55, new Color4(a.r, a.g, a.b, a.a * 0.9), new Color4(b.r, b.g, b.b, b.a * 0.9));
    ps.addColorGradient(1, dead);
    ps.blendMode = o.additive === false ? ParticleSystem.BLENDMODE_STANDARD : ParticleSystem.BLENDMODE_ADD;
    if (o.emitter) ps.particleEmitterType = o.emitter;
    ps.gravity = new Vector3(0, o.gravity ?? 0, 0);
    if (o.stretched) ps.billboardMode = ParticleSystem.BILLBOARDMODE_STRETCHED;
    if (o.spin) {
      ps.minAngularSpeed = -o.spin;
      ps.maxAngularSpeed = o.spin;
      ps.minInitialRotation = 0;
      ps.maxInitialRotation = Math.PI * 2;
    }
    if (o.grow !== undefined) {
      // Com gradiente, o Babylon usa estes valores como tamanho (ignora minSize/maxSize).
      ps.addSizeGradient(0, o.size[0], o.size[1]);
      ps.addSizeGradient(1, o.size[0] * o.grow, o.size[1] * o.grow);
    }
    if (o.drag) ps.addDragGradient(0, o.drag);
    this.budget.track(ps);
    return ps;
  }

  private disposeParticles(ps: ParticleSystem): void {
    this.budget.untrack(ps);
    ps.dispose(false);
  }

  /**
   * Rajada única de partículas. Pede espaço ao orçamento: a dos outros pode sair menor (e
   * mais curta) ou nem sair quando a tela está cheia.
   */
  private burst(at: Vector3, count: number, o: ParticleOpts): void {
    const g = this.budget.grant(count, this.owner);
    if (g.count <= 0) return;
    const opts = g.short ? { ...o, life: [o.life[0] * SHORT_LIFE, o.life[1] * SHORT_LIFE] as Range } : o;
    const ps = this.particles("fx-burst", g.count, opts);
    ps.emitter = at.clone();
    ps.manualEmitCount = g.count;
    ps.start();
    const life = opts.life[1] + 0.1;
    this.run(
      (t) => t < life,
      () => this.disposeParticles(ps),
    );
  }

  /** Emissão contínua presa a um ponto que se move (ex.: o corpo de alguém). */
  private stream(follow: () => Vector3 | null, rate: number, o: ParticleOpts, seconds = Infinity): FxHandle {
    const ps = this.particles("fx-stream", Math.ceil(rate * o.life[1] * 1.3) + 4, o);
    const pos = follow()?.clone() ?? Vector3.Zero();
    ps.emitter = pos;
    // A taxa segue o orçamento a cada quadro (a dos outros cai quando a tela está cheia).
    this.budget.stream(ps, rate, this.owner);
    ps.start();
    let stopAt = -1;
    let stopped = false;
    this.run(
      (t) => {
        const p = follow();
        if (p) pos.copyFrom(p);
        if (stopAt < 0 && (!p || t > seconds || stopped)) {
          ps.stop(); // as partículas que já saíram terminam a vida
          stopAt = t;
        }
        return stopAt < 0 || t - stopAt < o.life[1] + 0.1;
      },
      () => this.disposeParticles(ps),
    );
    return { stop: () => (stopped = true) };
  }

  /**
   * Cartão (plano virado para a câmera) ou quadrado no chão, do pool: os clarões e as marcas
   * não criam nem descartam malha a cada golpe.
   */
  private quad(kind: "card" | "ground"): Mesh {
    const free = kind === "card" ? this.freeCards : this.freeGround;
    const m = free.pop();
    if (m && !m.isDisposed()) {
      m.setEnabled(true);
      return m;
    }
    const q =
      kind === "card"
        ? MeshBuilder.CreatePlane("fx-card", { size: 1 }, this.scene)
        : MeshBuilder.CreateGround("fx-decal", { width: 1, height: 1 }, this.scene);
    q.isPickable = false;
    if (kind === "card") q.billboardMode = Mesh.BILLBOARDMODE_ALL;
    return q;
  }

  private releaseQuad(m: Mesh, kind: "card" | "ground"): void {
    if (m.isDisposed()) return;
    const free = kind === "card" ? this.freeCards : this.freeGround;
    if (free.length >= QUAD_POOL) {
      m.dispose();
      return;
    }
    m.setEnabled(false);
    m.parent = null;
    m.visibility = 1;
    m.rotation.setAll(0);
    m.scaling.setAll(1);
    free.push(m);
  }

  /** Clarão (cartão virado para a câmera) que cresce e some. */
  private flash(at: Vector3, size: number, color: Color3, dur: number, tex: Texture = this.tex.flare): void {
    const m = this.quad("card");
    m.material = this.mat(color, tex);
    m.position.copyFrom(at);
    m.scaling.setAll(size * 0.4);
    this.run(
      (t) => {
        const k = t / dur;
        m.scaling.setAll(size * (0.4 + 0.8 * Math.sqrt(k)));
        m.visibility = 1 - k * k;
        return k < 1;
      },
      () => this.releaseQuad(m, "card"),
    );
  }

  /** Marca plana no chão (anel, runas, queimado…). */
  private decal(
    at: Vector3,
    tex: Texture,
    color: Color3,
    o: { size: Range; dur: number; fadeIn?: number; additive?: boolean; spin?: number; y?: number; parent?: TransformNode; alpha?: number },
  ): Mesh {
    const m = this.quad("ground");
    m.material = this.mat(color, tex, o.additive ?? true, o.alpha ?? 1);
    if (o.parent) m.parent = o.parent;
    m.position.set(at.x, o.y ?? 0.06, at.z);
    m.rotation.y = Math.random() * Math.PI * 2;
    m.scaling.set(o.size[0], 1, o.size[0]);
    m.visibility = 0;
    const fadeIn = o.fadeIn ?? 0.08;
    this.run(
      (t, dt) => {
        const k = Math.min(1, t / o.dur);
        const s = o.size[0] + (o.size[1] - o.size[0]) * (1 - (1 - k) ** 3);
        m.scaling.set(s, 1, s);
        m.visibility = t < fadeIn ? t / fadeIn : 1 - Math.max(0, (k - 0.55) / 0.45);
        if (o.spin) m.rotation.y += o.spin * dt;
        return k < 1;
      },
      () => this.releaseQuad(m, "ground"),
    );
    return m;
  }

  /** Clarão de luz de verdade (ilumina o chão e quem estiver perto). */
  private lightFlash(at: Vector3, color: Color3, intensity: number, dur: number): void {
    const l = this.light;
    l.position.copyFrom(at);
    l.diffuse = color;
    this.lightUntil = performance.now() / 1000 + dur;
    this.run(
      (t) => {
        l.intensity = intensity * Math.max(0, 1 - t / dur);
        return t < dur;
      },
      () => undefined,
    );
  }

  /**
   * Tremor leve da câmera (seus golpes fortes e críticos, golpes do chefe em você). power é
   * o deslocamento máximo em metros com a câmera a 18 m (SHAKE_MAX no máximo); mais perto ou
   * mais longe, o tremor na tela fica igual. Respeita Configurações → Jogo → Tremor de câmera.
   */
  shake(power: number, seconds = 0.22): void {
    if (!this.camera || !settings.get().cameraShake) return;
    const p = Math.min(SHAKE_MAX, Math.max(0, power));
    // Um golpe mais forte no meio de um tremor reinicia; um mais fraco só soma tempo.
    if (p >= this.shakePower * (this.shakeLeft / this.shakeDur)) {
      this.shakePower = p;
      this.shakeDur = this.shakeLeft = Math.max(seconds, 0.05);
    }
  }

  private updateShake(dt: number): void {
    const cam = this.camera;
    if (!cam) return;
    if (this.shakeLeft <= 0) {
      if (this.shakePower) {
        this.shakePower = 0;
        this.shakeOffset.set(0, 0);
        cam.targetScreenOffset.set(0, 0);
      }
      return;
    }
    if (!settings.get().cameraShake) this.shakeLeft = 0;
    this.shakeLeft = Math.max(0, this.shakeLeft - dt);
    this.shakeT += dt;
    // Duas senoides fora de fase (não um ruído por quadro, que vira zumbido a 120 Hz),
    // caindo em curva até zero.
    const k = this.shakeLeft / this.shakeDur;
    const p = this.shakePower * k * k * (cam.radius / SHAKE_REF_RADIUS);
    const t = this.shakeT + this.shakeSeed;
    this.shakeOffset.set(p * Math.sin(t * 53) * 0.6 + p * Math.sin(t * 31) * 0.4, p * Math.sin(t * 47 + 1.3) * 0.6 + p * Math.sin(t * 29 + 0.7) * 0.4);
    cam.targetScreenOffset.copyFrom(this.shakeOffset);
  }

  // ------------------------------------------------------------------------------------
  // Peças fixas do cenário (Pedra de Teleporte)
  // ------------------------------------------------------------------------------------

  /** Texturas pintadas dos efeitos (brilho, runas…), para peças fixas reaproveitarem. */
  get textures(): FxTextures {
    return this.tex;
  }

  /** Material emissivo aditivo compartilhado (a cor vem daqui; a textura dá o contorno). */
  glowMaterial(color: Color3, tex: Texture | null, alpha = 1): StandardMaterial {
    return this.mat(color, tex, true, alpha);
  }

  /** Põe/tira uma malha da camada de brilho (só as incluídas brilham). */
  glowInclude(m: Mesh): void {
    this.addGlow(m);
  }

  glowExclude(m: Mesh): void {
    this.removeGlow(m);
    this.glowColors.delete(m);
  }

  private glowColors = new Map<AbstractMesh, Color3>();
  private glowSelector = false;

  /**
   * Cor do brilho de uma malha, independente da emissiveColor do material (a camada
   * multiplica esta cor pela textura emissiva). Útil quando o material tem emissiveColor
   * preta porque a cor já está pintada na textura. O objeto pode ser alterado depois
   * (pulsar). As outras malhas seguem o comportamento padrão do Babylon.
   */
  setGlowColor(m: Mesh, color: Color3): void {
    const glow = this.glow;
    if (!glow) return;
    this.glowColors.set(m, color);
    if (this.glowSelector) return;
    this.glowSelector = true;
    glow.customEmissiveColorSelector = (mesh, _sub, material, out) => {
      const own = this.glowColors.get(mesh);
      if (own) {
        out.set(own.r, own.g, own.b, 1);
        return;
      }
      const mat = material as unknown as { emissiveColor?: Color3; emissiveTexture?: { level: number } | null; emissiveIntensity?: number; alpha: number };
      if (mat?.emissiveColor) {
        const level = (mat.emissiveTexture?.level ?? 1) * (mat.emissiveIntensity ?? 1);
        out.set(mat.emissiveColor.r * level, mat.emissiveColor.g * level, mat.emissiveColor.b * level, mat.alpha);
      } else {
        const n = glow.neutralColor;
        out.set(n.r, n.g, n.b, n.a);
      }
    };
  }

  /**
   * Aura da Pedra de Teleporte: grãos de luz subindo devagar em volta da rocha e
   * faíscas caindo do núcleo de luz do topo. Para sozinha quando `root` é descartado.
   */
  teleportAura(root: TransformNode, coreY: number): FxHandle {
    const at = (y: number) => {
      const p = new Vector3();
      return () => (root.isDisposed() ? null : p.copyFrom(root.getAbsolutePosition()).addInPlaceFromFloats(0, y, 0));
    };
    const motes = this.stream(at(0.1), 9, {
      tex: this.tex.flare,
      life: [2.6, 3.8],
      size: [0.05, 0.13],
      speed: [0.3, 0.65],
      colors: [c4(0.45, 0.8, 1, 0.9), c4(0.75, 0.92, 1, 0.75), c4(0.2, 0.45, 1, 0)],
      emitter: rising(1.15, 0.2, 0.12),
    });
    const sparks = this.stream(at(coreY), 5, {
      tex: this.tex.star,
      life: [0.7, 1.4],
      size: [0.07, 0.15],
      speed: [0.05, 0.25],
      colors: [c4(0.85, 0.96, 1, 1), c4(0.5, 0.8, 1, 0.9), c4(0.3, 0.55, 1, 0)],
      emitter: new SphereParticleEmitter(0.38),
      gravity: -0.45,
      spin: 2,
    });
    return {
      stop: () => {
        motes.stop();
        sparks.stop();
      },
    };
  }

  private addGlow(m: Mesh): void {
    this.glow?.addIncludedOnlyMesh(m);
  }

  private removeGlow(m: Mesh): void {
    this.glow?.removeIncludedOnlyMesh(m);
  }

  // ------------------------------------------------------------------------------------
  // Texto
  // ------------------------------------------------------------------------------------

  /** Número subindo acima da cabeça (textos do jogo: item pego, "+XP"…). */
  damageNumber(anchor: TransformNode, text: string, color: string, big: boolean): void {
    this.numbers.show(anchor, "info", text, color, big ? 22 : 16);
  }

  /**
   * Número de combate no estilo do tipo (numbers.ts: normal, crítico, cura, errou,
   * bloqueio…). color troca a cor do estilo (skills com tema, dano por segundo).
   */
  number(anchor: TransformNode, kind: NumberKind, text: string, color?: string): void {
    this.numbers.show(anchor, kind, text, color);
  }

  /** Nome da skill subindo devagar sobre quem a usou. */
  skillName(anchor: TransformNode, text: string): void {
    const tb = new TextBlock();
    tb.text = text;
    tb.color = "#ffe7a8";
    tb.fontSize = 13;
    tb.fontStyle = "italic";
    tb.fontWeight = "600";
    tb.outlineWidth = 3;
    tb.outlineColor = "rgba(0,0,0,0.85)";
    tb.resizeToFit = true;
    this.gui.addControl(tb);
    tb.linkWithMesh(anchor);
    const dur = 1.1;
    this.run(
      (t) => {
        const k = t / dur;
        tb.linkOffsetY = -34 - k * 18;
        tb.alpha = k < 0.15 ? k / 0.15 : k < 0.7 ? 1 : 1 - (k - 0.7) / 0.3;
        return k < 1 && !anchor.isDisposed();
      },
      () => tb.dispose(),
    );
  }

  // ------------------------------------------------------------------------------------
  // Golpes básicos
  // ------------------------------------------------------------------------------------

  /**
   * Faíscas de um golpe corpo a corpo (maior e dourado no crítico). Com `from`, as faíscas
   * espirram para longe de quem bateu. Usa os sistemas fixos de faíscas (nada é criado).
   */
  meleeHit(at: Vector3, crit: boolean, from?: Vector3): void {
    this.impactSparks(at, from ?? null, crit ? new Color3(1, 0.78, 0.3) : new Color3(1, 0.88, 0.62), crit);
    this.flash(at, crit ? 1.5 : 0.8, crit ? new Color3(1, 0.82, 0.35) : new Color3(1, 0.95, 0.85), crit ? 0.22 : 0.14);
    if (crit) this.flash(at, 1.8, new Color3(1, 0.85, 0.35), 0.35, this.tex.star);
  }

  // ------------------------------------------------------------------------------------
  // Biblioteca: faíscas de impacto, poeira, rastro da arma, marcas de área e flash no alvo
  // ------------------------------------------------------------------------------------

  /** Sistema fixo de faíscas (célula de risco do atlas), reaproveitado por todos os impactos. */
  private createSparkSystem(): SparkSys {
    const ps = new ParticleSystem("fx-sparks", 64, this.scene, null, true);
    ps.particleTexture = this.atlas;
    ps.spriteCellWidth = ps.spriteCellHeight = ATLAS_CELL;
    ps.startSpriteCellID = ps.endSpriteCellID = CELL.spark;
    ps.spriteCellChangeSpeed = 0;
    ps.updateSpeed = 1 / 60;
    ps.minLifeTime = 0.16;
    ps.maxLifeTime = 0.34;
    ps.blendMode = ParticleSystem.BLENDMODE_ADD;
    ps.billboardMode = ParticleSystem.BILLBOARDMODE_STRETCHED;
    ps.gravity = new Vector3(0, -9, 0);
    const dir = new SphereDirectedParticleEmitter(0.15, new Vector3(-1, 0.2, -1), new Vector3(1, 1, 1));
    ps.particleEmitterType = dir;
    ps.emitter = new Vector3(0, -100, 0);
    ps.emitRate = 0;
    ps.manualEmitCount = 0;
    this.budget.track(ps);
    ps.start();
    return { ps, dir, frame: -1 };
  }

  /**
   * Faíscas de impacto: espirram para longe de `from` (ou em volta, sem direção), na cor
   * dada. Cada sistema fixo aceita um impacto por quadro; se os quatro já foram usados neste
   * quadro, a sua faísca vira uma rajada comum e a dos outros fica de fora.
   */
  impactSparks(at: Vector3, from: Vector3 | null, color: Color3, crit = false): void {
    const want = crit ? 24 : 12;
    const sys = this.sparks.find((x) => x.frame !== this.frame);
    if (!sys) {
      if (this.mine) {
        this.burst(at, want, {
          tex: this.tex.spark,
          life: [0.16, 0.34],
          size: crit ? [0.12, 0.26] : [0.08, 0.18],
          speed: crit ? [5, 9] : [3.5, 6],
          colors: [c4(color.r, color.g, color.b), c4(1, 1, 0.9), c4(color.r, color.g * 0.6, color.b * 0.3, 0)],
          emitter: new SphereParticleEmitter(0.15, 1, 0.2),
          gravity: -9,
          stretched: true,
        });
      }
      return;
    }
    const g = this.budget.grant(want, this.owner);
    if (g.count <= 0) return;
    sys.frame = this.frame;
    const ps = sys.ps;
    (ps.emitter as Vector3).copyFrom(at);
    // Para longe de quem bateu e um pouco para cima, bem espalhado.
    const d = from ? at.subtract(from) : new Vector3();
    d.y = 0;
    if (d.lengthSquared() > 1e-4) d.normalize();
    d.y = 0.45;
    const spread = from ? 0.75 : 1;
    sys.dir.direction1.set(d.x - spread, d.y - 0.25, d.z - spread);
    sys.dir.direction2.set(d.x + spread, d.y + 0.65, d.z + spread);
    ps.minSize = crit ? 0.12 : 0.08;
    ps.maxSize = crit ? 0.26 : 0.18;
    ps.minEmitPower = crit ? 5 : 3.5;
    ps.maxEmitPower = crit ? 9 : 6.5;
    const life = g.short ? SHORT_LIFE : 1;
    ps.minLifeTime = 0.16 * life;
    ps.maxLifeTime = (crit ? 0.4 : 0.32) * life;
    ps.color1.set(color.r * 1.3, color.g * 1.3, color.b * 1.3, 1);
    ps.color2.set(1.3, 1.3, 1.15, 1);
    ps.colorDead.set(color.r, color.g * 0.45, color.b * 0.2, 0);
    ps.manualEmitCount = g.count;
  }

  /**
   * Poeira nos passos do próprio jogador correndo (só no médio e no alto). Um sistema fixo,
   * criado no primeiro passo; `side` (±1) é o pé que pisou.
   */
  footDust(at: Vector3, yaw: number, side: number): void {
    if (!this.budget.preset.dust) return;
    let ps = this.dust;
    if (!ps) {
      ps = new ParticleSystem("fx-dust", 48, this.scene, null, true);
      ps.particleTexture = this.atlas;
      ps.spriteCellWidth = ps.spriteCellHeight = ATLAS_CELL;
      ps.startSpriteCellID = ps.endSpriteCellID = CELL.puff;
      ps.spriteCellChangeSpeed = 0;
      ps.updateSpeed = 1 / 60;
      ps.minLifeTime = 0.45;
      ps.maxLifeTime = 0.75;
      ps.addSizeGradient(0, 0.22, 0.32);
      ps.addSizeGradient(1, 0.5, 0.75);
      ps.minEmitPower = 0.25;
      ps.maxEmitPower = 0.6;
      // Terra seca e clara (grama seca e pedra quente do guia, 3.3), translúcida.
      ps.color1 = new Color4(0.64, 0.6, 0.45, 0.32);
      ps.color2 = new Color4(0.79, 0.72, 0.61, 0.26);
      ps.colorDead = new Color4(0.64, 0.6, 0.5, 0);
      ps.blendMode = ParticleSystem.BLENDMODE_STANDARD;
      ps.gravity = new Vector3(0, 0.4, 0);
      ps.minInitialRotation = 0;
      ps.maxInitialRotation = Math.PI * 2;
      ps.particleEmitterType = new HemisphericParticleEmitter(0.12, 0);
      ps.addDragGradient(0, 0.6);
      ps.emitter = new Vector3(0, -100, 0);
      ps.emitRate = 0;
      ps.manualEmitCount = 0;
      this.budget.track(ps);
      ps.start();
      this.dust = ps;
    }
    // Um pouco atrás e ao lado do pé que pisou.
    const ox = Math.cos(yaw) * 0.14 * side - Math.sin(yaw) * 0.2;
    const oz = -Math.sin(yaw) * 0.14 * side - Math.cos(yaw) * 0.2;
    (ps.emitter as Vector3).set(at.x + ox, 0.05, at.z + oz);
    ps.manualEmitCount = this.budget.grant(3, "mine").count;
  }

  /**
   * Rastro curto da arma no golpe, de `from` a `until` (performance.now()). Os seus no médio
   * e no alto; os dos outros só no alto e com folga no orçamento. false = não saiu (sem
   * lâmina na mão, pool cheio ou preset).
   */
  weaponTrail(hand: TransformNode, from: number, until: number, color: Color3): boolean {
    const p = this.budget.preset;
    if (this.mine ? !p.trailMine : !p.trailOthers || !this.budget.relaxed) return false;
    let tr: ReturnType<WeaponTrails["start"]> = null;
    try {
      tr = this.trails.start(hand, from, until, this.mat(color, this.atlas, true));
    } catch {
      tr = null; // arma com malha estranha: segue sem rastro
    }
    if (!tr) return false;
    const { tick, release } = tr;
    this.run(() => tick(performance.now()), release);
    return true;
  }

  /**
   * Marca no chão do tamanho da área de uma skill (anel, runas ou rachadura, nas cores do
   * tema: ui/skill-themes.ts, SKILL_FX). As suas sempre; as dos outros a partir do médio e
   * com folga no orçamento.
   */
  areaMark(at: Vector3, radius: number, fx: string): void {
    const kind = SKILL_FX[fx]?.mark ?? "ring";
    if (kind === "none" || radius <= 0) return;
    if (!this.mine && (!this.budget.preset.marksOthers || !this.budget.relaxed)) return;
    const colors = skillFxColors(fx);
    const color = colors ? Color3.FromHexString(colors.mark) : new Color3(1, 0.85, 0.5);
    const ground = new Vector3(at.x, 0, at.z);
    const d = radius * 2;
    // Borda no raio real (o anel da textura fica a ~93% do quadrado).
    this.decal(ground, this.tex.ring, color, { size: [d * 0.9, d * 1.07], dur: 0.9, fadeIn: 0.05, y: 0.055 });
    if (kind === "runes") this.decal(ground, this.tex.runes, color, { size: [d * 0.75, d * 0.95], dur: 1.2, spin: 0.9, y: 0.065, alpha: 0.7 });
    else if (kind === "crack") this.decal(ground, this.tex.crack, color.scale(0.25), { size: [d * 0.7, d * 0.85], dur: 1.8, additive: false, y: 0.05 });
  }

  /**
   * Flash rápido no alvo atingido: as malhas trocam por um instante para uma cópia do
   * material com emissivo claro (hit-flash.ts). Sem partícula nem malha nova.
   */
  hitFlash(meshes: AbstractMesh[], crit: boolean): void {
    if (!meshes.length) return;
    try {
      this.hitFlashes.flash(meshes, crit ? 110 : 80, crit);
    } catch {
      // material sem suporte: o alvo só não pisca
    }
  }

  /**
   * ?fxstress=1: a cada 120 ms dispara uma receita pesada perto da câmera, como se fosse de
   * outros jogadores (o raid com o grupo cheio, sem precisar de seis pessoas). Serve para
   * ver no F3 que o pico de partículas fica no teto do preset; com ?fxbudget=off, o de antes.
   */
  private startStress(): void {
    const cam = this.camera;
    if (!cam) return;
    const up = (p: Vector3, y = 1) => p.add(new Vector3(0, y, 0));
    const recipes: ((p: Vector3) => void)[] = [
      (p) => this.fireExplosion(up(p)),
      (p) => this.iceNova(p, 5),
      (p) => this.slam(p, 4, new Color3(1, 0.7, 0.35), false),
      (p) => this.brutalStrike(up(p).add(new Vector3(1, 0, 0)), up(p), false),
      (p) => this.shieldBash(up(p), false),
      (p) => this.poisonSplash(up(p, 0.5), 3, new Color3(0.45, 0.9, 0.3)),
      (p) => this.meleeHit(up(p), Math.random() < 0.3, up(p).add(new Vector3(1, 0, 0))),
      (p) => this.fireball(up(p, 2).add(new Vector3(-6, 0, 0)), up(p), 0.3),
    ];
    console.info("fx: ?fxstress=1 (efeitos de outros jogadores em volta da câmera)");
    window.setInterval(() => {
      if (this.scene.isDisposed) return;
      const a = Math.random() * Math.PI * 2;
      const r = 2 + Math.random() * 7;
      const p = new Vector3(cam.target.x + Math.cos(a) * r, 0, cam.target.z + Math.sin(a) * r);
      this.as(false, () => recipes[Math.floor(Math.random() * recipes.length)](p));
    }, 120);
  }

  /** Orbe arcano (magia básica). */
  arcaneBolt(from: Vector3, to: Vector3, dur: number): void {
    this.orb(from, to, dur, {
      core: new Color3(0.85, 0.75, 1),
      halo: new Color3(0.55, 0.35, 1),
      size: 0.28,
      arc: 0.3,
      trail: {
        tex: this.tex.flare,
        life: [0.18, 0.32],
        size: [0.12, 0.26],
        speed: [0, 0.3],
        colors: [c4(0.75, 0.6, 1, 0.9), c4(0.5, 0.4, 1, 0.8), c4(0.3, 0.2, 0.8, 0)],
        grow: 0.2,
      },
      rate: 70,
      onArrive: (at) => {
        this.burst(at, 14, {
          tex: this.tex.flare,
          life: [0.2, 0.4],
          size: [0.1, 0.22],
          speed: [1.5, 3.5],
          colors: [c4(0.85, 0.75, 1), c4(0.55, 0.45, 1), c4(0.3, 0.2, 0.9, 0)],
          emitter: new SphereParticleEmitter(0.1),
          grow: 0.3,
        });
        this.flash(at, 0.9, new Color3(0.7, 0.55, 1), 0.18);
      },
    });
  }

  /** Brilho na mão de quem conjura. */
  castGlow(at: Vector3, color: Color3): void {
    this.flash(at, 1.2, color, 0.3);
    this.burst(at, 10, {
      tex: this.tex.flare,
      life: [0.25, 0.45],
      size: [0.06, 0.14],
      speed: [0.5, 1.5],
      colors: [c4(color.r, color.g, color.b), c4(1, 1, 1, 0.8), c4(color.r, color.g, color.b, 0)],
      emitter: new SphereParticleEmitter(0.25),
      gravity: 1.5,
    });
  }

  /** Item pego do chão: brilho e faíscas subindo. */
  pickup(at: Vector3, color: Color3): void {
    const p = at.add(new Vector3(0, 0.3, 0));
    this.flash(p, 0.9, color, 0.25);
    this.burst(p, 14, {
      tex: this.tex.flare,
      life: [0.35, 0.6],
      size: [0.05, 0.12],
      speed: [0.8, 2],
      colors: [c4(color.r, color.g, color.b), c4(1, 1, 1, 0.8), c4(color.r, color.g, color.b, 0)],
      emitter: new SphereParticleEmitter(0.2),
      gravity: -2,
    });
  }

  /** Poção bebida: anel no chão e bolhas subindo na cor da poção. */
  potion(root: TransformNode, color: Color3): void {
    const at = root.position.clone();
    this.decal(at, this.tex.ring, color, { size: [0.4, 2], dur: 0.6 });
    this.stream(() => (root.isDisposed() ? null : root.position), 40, {
      tex: this.tex.flare,
      life: [0.5, 0.9],
      size: [0.06, 0.14],
      speed: [1.2, 2.4],
      colors: [c4(color.r, color.g, color.b), c4(1, 1, 1, 0.7), c4(color.r, color.g, color.b, 0)],
      emitter: rising(0.45, 0.1, 0.1),
      gravity: 0.5,
    }, 0.6);
  }

  // ------------------------------------------------------------------------------------
  // Skills
  // ------------------------------------------------------------------------------------

  /** Bola de Fogo: núcleo brilhante com rastro de fogo e fumaça, explode no alvo. */
  fireball(from: Vector3, to: Vector3, dur: number): void {
    this.castGlow(from, new Color3(1, 0.55, 0.15));
    this.orb(from, to, dur, {
      core: new Color3(1, 0.92, 0.6),
      halo: new Color3(1, 0.42, 0.08),
      size: 0.42,
      arc: 0.8,
      trail: {
        tex: this.tex.flame,
        life: [0.22, 0.42],
        size: [0.28, 0.5],
        speed: [0.2, 0.8],
        colors: [c4(1, 0.75, 0.25), c4(1, 0.45, 0.08), c4(0.4, 0.05, 0, 0)],
        grow: 0.25,
        spin: 3,
      },
      rate: 150,
      smoke: true,
      onArrive: (at) => this.fireExplosion(at),
    });
  }

  private fireExplosion(at: Vector3): void {
    this.flash(at, 3.6, new Color3(1, 0.55, 0.18), 0.3);
    this.lightFlash(at.add(new Vector3(0, 0.5, 0)), new Color3(1, 0.55, 0.2), 3.5, 0.45);
    // Bola de fogo que se abre.
    this.burst(at, 36, {
      tex: this.tex.flame,
      life: [0.3, 0.55],
      size: [0.45, 0.9],
      speed: [2, 4.5],
      colors: [c4(1, 0.62, 0.18), c4(0.95, 0.32, 0.05), c4(0.35, 0.05, 0, 0)],
      emitter: new SphereParticleEmitter(0.25),
      grow: 0.3,
      spin: 4,
      drag: 0.5,
    });
    // Brasas voando com gravidade.
    this.burst(at, 30, {
      tex: this.tex.spark,
      life: [0.5, 1.0],
      size: [0.06, 0.13],
      speed: [4, 8],
      colors: [c4(1, 0.9, 0.5), c4(1, 0.55, 0.15), c4(1, 0.2, 0, 0)],
      emitter: new HemisphericParticleEmitter(0.2),
      gravity: -9.8,
      stretched: true,
    });
    // Fumaça escura subindo.
    this.burst(at.add(new Vector3(0, 0.3, 0)), 12, {
      tex: this.tex.smoke,
      life: [0.9, 1.6],
      size: [0.8, 1.4],
      speed: [0.5, 1.4],
      colors: [c4(0.25, 0.2, 0.18, 0.55), c4(0.15, 0.12, 0.1, 0.45), c4(0.1, 0.1, 0.1, 0)],
      emitter: new HemisphericParticleEmitter(0.4),
      gravity: 1.2,
      additive: false,
      grow: 2.2,
      spin: 1,
    });
    // Onda de fogo no chão e marca de queimado.
    this.decal(at, this.tex.ring, new Color3(0.85, 0.35, 0.08), { size: [0.5, 3.6], dur: 0.45 });
    this.decal(at, this.tex.scorch, new Color3(0.06, 0.035, 0.02), { size: [1.8, 2.6], dur: 3.2, additive: false });
  }

  /** Nova de Gelo: anel de gelo no raio real, cristais brotando do chão e neve. */
  iceNova(at: Vector3, radius: number): void {
    const ground = new Vector3(at.x, 0, at.z);
    this.flash(ground.add(new Vector3(0, 1, 0)), 2.4, new Color3(0.4, 0.65, 0.9), 0.25);
    this.lightFlash(ground.add(new Vector3(0, 1.5, 0)), new Color3(0.55, 0.8, 1), 2.2, 0.5);
    // Anel principal que corre até o raio, mais um anel de onda.
    this.decal(ground, this.tex.frostRing, new Color3(0.38, 0.62, 0.85), { size: [0.6, radius * 2.15], dur: 1.1, fadeIn: 0.04 });
    this.decal(ground, this.tex.ring, new Color3(0.45, 0.7, 0.95), { size: [0.4, radius * 2.4], dur: 0.4 });
    // Neve e cristais voando para fora, rente ao chão.
    this.burst(ground.add(new Vector3(0, 0.4, 0)), 90, {
      tex: this.tex.flake,
      life: [0.45, 0.8],
      size: [0.12, 0.3],
      speed: [radius * 1.2, radius * 1.9],
      colors: [c4(0.85, 0.95, 1), c4(0.6, 0.85, 1), c4(0.5, 0.75, 1, 0)],
      emitter: new CylinderParticleEmitter(0.3, 0.6, 1, 0.15),
      drag: 0.6,
      spin: 4,
    });
    // Névoa fria rente ao chão.
    this.burst(ground.add(new Vector3(0, 0.25, 0)), 24, {
      tex: this.tex.smoke,
      life: [0.9, 1.4],
      size: [1.2, 2],
      speed: [radius * 0.5, radius * 0.9],
      colors: [c4(0.75, 0.9, 1, 0.35), c4(0.6, 0.8, 1, 0.3), c4(0.6, 0.8, 1, 0)],
      emitter: new CylinderParticleEmitter(0.5, 0.2, 1, 0.1),
      grow: 1.6,
      drag: 0.7,
      spin: 0.8,
    });
    // Cristais de gelo que brotam e quebram.
    const iceMat = this.mat(new Color3(0.55, 0.85, 1), null, false, 0.82);
    const count = Math.round(10 + radius * 1.6);
    for (let i = 0; i < count; i++) {
      const a = (i / count) * Math.PI * 2 + rnd(-0.15, 0.15);
      const r = radius * rnd(0.45, 0.95);
      const h = rnd(0.7, 1.5);
      const shard = MeshBuilder.CreateCylinder("fx-shard", { height: h, diameterTop: 0, diameterBottom: rnd(0.25, 0.45), tessellation: 5 }, this.scene);
      shard.material = iceMat;
      shard.isPickable = false;
      shard.position.set(ground.x + Math.cos(a) * r, 0, ground.z + Math.sin(a) * r);
      shard.rotation.set(rnd(-0.35, 0.35), rnd(0, Math.PI), rnd(-0.35, 0.35));
      this.addGlow(shard);
      const delay = (r / radius) * 0.18;
      this.run(
        (t) => {
          const k = Math.max(0, t - delay);
          const grow = Math.min(1, k / 0.12);
          const fade = k > 0.7 ? 1 - (k - 0.7) / 0.35 : 1;
          shard.scaling.set(1, Math.max(0.001, grow), 1);
          shard.position.y = (h / 2) * grow - 0.05;
          shard.visibility = Math.max(0, fade);
          return k < 1.05;
        },
        () => {
          this.removeGlow(shard);
          shard.dispose();
        },
      );
    }
  }

  /** Acerto da Nova em cada inimigo: estilhaços de gelo. */
  frostHit(at: Vector3): void {
    this.burst(at, 12, {
      tex: this.tex.flake,
      life: [0.3, 0.55],
      size: [0.1, 0.22],
      speed: [2, 4],
      colors: [c4(0.9, 0.97, 1), c4(0.6, 0.85, 1), c4(0.5, 0.75, 1, 0)],
      emitter: new SphereParticleEmitter(0.3),
      gravity: -5,
      spin: 5,
    });
  }

  /** Golpe de Escudo: clarão metálico, onda de choque e faíscas. */
  shieldBash(at: Vector3, mine: boolean): void {
    this.flash(at, 2.2, new Color3(0.8, 0.9, 1), 0.2);
    this.burst(at, 28, {
      tex: this.tex.spark,
      life: [0.2, 0.45],
      size: [0.1, 0.22],
      speed: [5, 10],
      colors: [c4(1, 1, 1), c4(0.75, 0.88, 1), c4(0.5, 0.7, 1, 0)],
      emitter: new SphereParticleEmitter(0.2, 1, 0.1),
      gravity: -8,
      stretched: true,
    });
    const ground = new Vector3(at.x, 0, at.z);
    this.decal(ground, this.tex.ring, new Color3(0.75, 0.88, 1), { size: [0.3, 3.4], dur: 0.35 });
    this.burst(ground.add(new Vector3(0, 0.15, 0)), 14, {
      tex: this.tex.smoke,
      life: [0.4, 0.7],
      size: [0.5, 0.8],
      speed: [2, 3.5],
      colors: [c4(0.65, 0.6, 0.55, 0.45), c4(0.5, 0.45, 0.4, 0.4), c4(0.5, 0.45, 0.4, 0)],
      emitter: new CylinderParticleEmitter(0.3, 0.1, 1, 0),
      additive: false,
      grow: 1.8,
      drag: 0.8,
    });
    if (mine) this.shake(0.06);
  }

  /** Golpe Brutal: corte vermelho em arco, faíscas laranja e chão rachado. */
  brutalStrike(from: Vector3, at: Vector3, mine: boolean): void {
    this.slash(from, at, new Color3(1, 0.32, 0.15), [0.55], 2.6);
    this.flash(at, 2, new Color3(1, 0.5, 0.25), 0.2);
    this.burst(at, 30, {
      tex: this.tex.spark,
      life: [0.25, 0.55],
      size: [0.1, 0.24],
      speed: [5, 10],
      colors: [c4(1, 0.85, 0.5), c4(1, 0.45, 0.15), c4(0.8, 0.15, 0, 0)],
      emitter: new SphereParticleEmitter(0.2, 1, 0.2),
      gravity: -10,
      stretched: true,
    });
    const ground = new Vector3(at.x, 0, at.z);
    this.decal(ground, this.tex.crack, new Color3(0.09, 0.06, 0.04), { size: [1.6, 2.3], dur: 2.4, additive: false });
    this.burst(ground.add(new Vector3(0, 0.2, 0)), 10, {
      tex: this.tex.smoke,
      life: [0.5, 0.9],
      size: [0.5, 0.9],
      speed: [1, 2.2],
      colors: [c4(0.55, 0.45, 0.35, 0.5), c4(0.45, 0.38, 0.3, 0.4), c4(0.4, 0.35, 0.3, 0)],
      emitter: new HemisphericParticleEmitter(0.4),
      additive: false,
      grow: 1.8,
      gravity: 0.6,
    });
    if (mine) this.shake(0.08);
  }

  /** Golpe Furtivo: corte em X roxo-escuro e nuvem de sombra. */
  backstab(from: Vector3, at: Vector3, crit: boolean): void {
    const color = crit ? new Color3(1, 0.75, 0.35) : new Color3(0.75, 0.35, 1);
    this.slash(from, at, color, [0.8, -0.8], 1.9);
    this.flash(at, 1.4, color, 0.18);
    this.burst(at, 18, {
      tex: this.tex.smoke,
      life: [0.4, 0.8],
      size: [0.4, 0.8],
      speed: [0.8, 2],
      colors: [c4(0.25, 0.12, 0.35, 0.65), c4(0.12, 0.05, 0.2, 0.6), c4(0.05, 0.02, 0.1, 0)],
      emitter: new SphereParticleEmitter(0.35),
      additive: false,
      grow: 1.7,
      spin: 2,
    });
    this.burst(at, 16, {
      tex: this.tex.spark,
      life: [0.2, 0.4],
      size: [0.07, 0.15],
      speed: [3, 6],
      colors: [c4(0.9, 0.7, 1), c4(0.7, 0.4, 1), c4(0.4, 0.1, 0.8, 0)],
      emitter: new SphereParticleEmitter(0.15),
      stretched: true,
    });
  }

  /** Fortaleza: cúpula de escudo azul que sobe e runas sob os pés. */
  fortress(body: FxBody): void {
    const at = body.root.position.clone();
    const dome = MeshBuilder.CreateSphere("fx-dome", { diameter: 2.6 * body.scale, segments: 20, slice: 0.5 }, this.scene);
    dome.material = this.mat(new Color3(0.35, 0.6, 1), this.tex.ring, true, 0.9);
    dome.isPickable = false;
    dome.position.copyFrom(at);
    this.addGlow(dome);
    this.run(
      (t) => {
        const k = t / 0.7;
        dome.position.copyFrom(body.root.position);
        dome.scaling.setAll(0.4 + 0.75 * Math.min(1, k * 1.6));
        dome.visibility = k < 0.35 ? 1 : 1 - (k - 0.35) / 0.65;
        return k < 1 && !body.root.isDisposed();
      },
      () => {
        this.removeGlow(dome);
        dome.dispose();
      },
    );
    this.burst(at.add(new Vector3(0, 0.2, 0)), 30, {
      tex: this.tex.flare,
      life: [0.6, 1.1],
      size: [0.08, 0.18],
      speed: [1, 2.5],
      colors: [c4(0.6, 0.8, 1), c4(0.85, 0.92, 1), c4(0.3, 0.5, 1, 0)],
      emitter: rising(0.7 * body.scale, 0.1, 0.35),
      gravity: 1,
    });
    this.decal(at, this.tex.ring, new Color3(0.5, 0.75, 1), { size: [0.5, 3.2 * body.scale], dur: 0.45 });
  }

  /** Fúria: explosão vermelha e chamas subindo do corpo. */
  fury(body: FxBody): void {
    const at = body.root.position.clone();
    this.flash(at.add(new Vector3(0, 1, 0)), 2.6, new Color3(1, 0.3, 0.12), 0.3);
    this.decal(at, this.tex.ring, new Color3(1, 0.3, 0.1), { size: [0.4, 4 * body.scale], dur: 0.4 });
    this.burst(at.add(new Vector3(0, 0.2, 0)), 70, {
      tex: this.tex.flame,
      life: [0.45, 0.8],
      size: [0.45, 0.85],
      speed: [1.5, 3.5],
      colors: [c4(1, 0.6, 0.2), c4(1, 0.25, 0.08), c4(0.4, 0, 0, 0)],
      emitter: rising(0.45 * body.scale, 0.2, 0.9),
      gravity: 2,
      grow: 0.3,
      spin: 3,
    });
  }

  /** Passo Sombrio: explosão de sombra em volta. */
  shadowStep(body: FxBody): void {
    const at = body.root.position.clone();
    this.decal(at, this.tex.ring, new Color3(0.45, 0.22, 0.8), { size: [0.4, 3.2 * body.scale], dur: 0.45 });
    this.flash(at.add(new Vector3(0, 1, 0)), 2, new Color3(0.4, 0.2, 0.75), 0.25);
    this.burst(at.add(new Vector3(0, 0.6, 0)), 34, {
      tex: this.tex.smoke,
      life: [0.5, 1.0],
      size: [0.6, 1.1],
      speed: [1.5, 3.5],
      colors: [c4(0.2, 0.1, 0.3, 0.7), c4(0.08, 0.04, 0.14, 0.65), c4(0.05, 0.02, 0.08, 0)],
      emitter: new CylinderParticleEmitter(0.4, 1, 1, 0.2),
      additive: false,
      grow: 1.8,
      drag: 0.6,
      spin: 2,
    });
    this.burst(at.add(new Vector3(0, 0.8, 0)), 20, {
      tex: this.tex.flare,
      life: [0.3, 0.6],
      size: [0.06, 0.14],
      speed: [2, 4],
      colors: [c4(0.75, 0.55, 1), c4(0.5, 0.3, 0.9), c4(0.3, 0.1, 0.6, 0)],
      emitter: new SphereParticleEmitter(0.5),
    });
  }

  /** Coluna dourada ao subir de nível. */
  levelUp(root: TransformNode): void {
    const at = root.position.clone();
    this.flash(at.add(new Vector3(0, 1.2, 0)), 3, new Color3(1, 0.85, 0.4), 0.4);
    this.decal(at, this.tex.runes, new Color3(1, 0.82, 0.4), { size: [1.5, 3.2], dur: 1.6, spin: 1.2 });
    this.decal(at, this.tex.ring, new Color3(1, 0.85, 0.45), { size: [0.5, 5], dur: 0.7 });
    this.stream(() => (root.isDisposed() ? null : root.position), 70, {
      tex: this.tex.flare,
      life: [0.8, 1.3],
      size: [0.08, 0.2],
      speed: [2.5, 4.5],
      colors: [c4(1, 0.9, 0.55), c4(1, 0.75, 0.3), c4(1, 0.6, 0.2, 0)],
      emitter: rising(0.6, 0.1, 0.12),
      gravity: 1,
    }, 1.0);
    // Coluna translúcida, como antes, mas mais suave.
    const col = MeshBuilder.CreateCylinder("fx-level", { height: 4, diameterTop: 0.3, diameterBottom: 1.6, tessellation: 24, cap: Mesh.NO_CAP }, this.scene);
    col.material = this.mat(new Color3(1, 0.82, 0.35), this.tex.flare, true, 0.6);
    col.isPickable = false;
    col.parent = root;
    col.position.y = 2;
    this.run(
      (t) => {
        const k = t / 1.4;
        col.scaling.set(1 + k * 0.6, 0.3 + k, 1 + k * 0.6);
        col.visibility = 1 - k;
        return k < 1 && !root.isDisposed();
      },
      () => col.dispose(),
    );
  }

  // ------------------------------------------------------------------------------------
  // Efeitos que duram
  // ------------------------------------------------------------------------------------

  /** Visual enquanto o efeito dura. O handle encerra antes da hora (fim ou morte). */
  // ------------------------------------------------------------------------------------
  // Skills da árvore (Fase B)
  // ------------------------------------------------------------------------------------

  /** Projétil colorido (Lança de Gelo, Marca da Morte, Prisão Glacial, Frasco Tóxico…). */
  bolt(from: Vector3, to: Vector3, dur: number, color: Color3, onArrive?: (at: Vector3) => void): void {
    const light = new Color3(Math.min(1, color.r + 0.35), Math.min(1, color.g + 0.35), Math.min(1, color.b + 0.35));
    this.orb(from, to, dur, {
      core: light,
      halo: color,
      size: 0.3,
      arc: 0.2,
      trail: {
        tex: this.tex.flare,
        life: [0.18, 0.32],
        size: [0.12, 0.26],
        speed: [0, 0.3],
        colors: [c4(light.r, light.g, light.b, 0.9), c4(color.r, color.g, color.b, 0.8), c4(color.r * 0.5, color.g * 0.5, color.b * 0.5, 0)],
        grow: 0.2,
      },
      rate: 70,
      onArrive: (at) => {
        this.burst(at, 16, {
          tex: this.tex.flare,
          life: [0.2, 0.45],
          size: [0.1, 0.24],
          speed: [1.5, 3.5],
          colors: [c4(light.r, light.g, light.b), c4(color.r, color.g, color.b), c4(color.r, color.g, color.b, 0)],
          emitter: new SphereParticleEmitter(0.1),
          grow: 0.3,
        });
        this.flash(at, 1, color, 0.2);
        onArrive?.(at);
      },
    });
  }

  /** Grito (Provocar, Grito de Guerra, Rugido): ondas que saem do corpo. */
  shout(body: FxBody, color: Color3, radius: number): void {
    const at = body.root.position.clone();
    const wave = this.keep(() => this.decal(at, this.tex.ring, color, { size: [0.6, radius * 2], dur: 0.55 }));
    for (let i = 0; i < 3; i++) window.setTimeout(wave, i * 110);
    this.burst(body.head.getAbsolutePosition(), 18, {
      tex: this.tex.spark,
      life: [0.3, 0.55],
      size: [0.12, 0.3],
      speed: [3, 6],
      colors: [c4(color.r, color.g, color.b), c4(1, 1, 1, 0.8), c4(color.r, color.g, color.b, 0)],
      emitter: new SphereParticleEmitter(0.2),
      stretched: true,
    });
  }

  /** Pancada no chão (Corte Giratório, Rachar o Chão, Pisotear, Execução). */
  slam(at: Vector3, radius: number, color: Color3, mine: boolean, crack = true): void {
    const ground = new Vector3(at.x, 0, at.z);
    if (crack) this.decal(ground, this.tex.crack, new Color3(0.25, 0.2, 0.18), { size: [radius * 1.2, radius * 1.6], dur: 1.6, additive: false });
    this.decal(ground, this.tex.ring, color, { size: [0.5, radius * 2.1], dur: 0.45 });
    this.burst(ground.add(new Vector3(0, 0.2, 0)), 26, {
      tex: this.tex.smoke,
      life: [0.5, 0.9],
      size: [0.3, 0.6],
      speed: [1.5, 3],
      colors: [c4(0.55, 0.48, 0.4, 0.7), c4(0.4, 0.35, 0.3, 0.6), c4(0.3, 0.25, 0.2, 0)],
      emitter: new CylinderParticleEmitter(radius * 0.5, 0.1, 1, 0),
      additive: false,
      grow: 0.8,
      drag: 2,
    });
    this.flash(ground.add(new Vector3(0, 0.3, 0)), radius * 0.8, color, 0.22);
    if (mine) this.shake(0.1, 0.3);
  }

  /** Corte em volta (Corte Giratório). */
  whirl(body: FxBody, radius: number, mine: boolean): void {
    const at = body.root.position.add(new Vector3(0, 1, 0));
    for (let i = 0; i < 4; i++) {
      const a = (i / 4) * Math.PI * 2;
      const from = at.add(new Vector3(Math.cos(a) * 0.4, 0, Math.sin(a) * 0.4));
      const to = at.add(new Vector3(Math.cos(a) * radius, 0, Math.sin(a) * radius));
      this.slash(from, to, new Color3(1, 0.9, 0.6), [0], 1.3);
    }
    this.slam(body.root.position, radius, new Color3(1, 0.85, 0.5), mine, false);
  }

  /** Meteoro: pedra em chamas caindo do céu. */
  meteor(at: Vector3, radius: number, mine: boolean): void {
    const from = at.add(new Vector3(-7, 22, -5));
    this.orb(from, at, 0.75, {
      core: new Color3(1, 0.85, 0.4),
      halo: new Color3(1, 0.4, 0.1),
      size: 1.1,
      arc: 0,
      smoke: true,
      trail: {
        tex: this.tex.flame,
        life: [0.3, 0.6],
        size: [0.6, 1.2],
        speed: [0, 0.5],
        colors: [c4(1, 0.7, 0.2), c4(1, 0.3, 0.05), c4(0.3, 0.05, 0, 0)],
        grow: 0.6,
      },
      rate: 120,
      onArrive: (p) => {
        this.fireExplosion(p);
        this.slam(p, radius + 1, new Color3(1, 0.45, 0.1), mine);
        this.decal(new Vector3(p.x, 0, p.z), this.tex.scorch, new Color3(0.15, 0.1, 0.08), { size: [radius * 2, radius * 2.4], dur: 4, additive: false });
        this.lightFlash(p.add(new Vector3(0, 2, 0)), new Color3(1, 0.5, 0.2), 4, 0.6);
        if (mine) this.shake(0.16, 0.45);
      },
    });
  }

  /** Chuva de Brasas: várias bolas de fogo pequenas caindo em volta do alvo. */
  emberRain(at: Vector3, radius: number): void {
    const drop = this.keep(() => {
      const a = Math.random() * Math.PI * 2;
      const r = Math.sqrt(Math.random()) * radius;
      const to = at.add(new Vector3(Math.cos(a) * r, 0.2, Math.sin(a) * r));
      this.fireball(to.add(new Vector3(-2, 12, -1.5)), to, 0.45);
    });
    for (let i = 0; i < 8; i++) window.setTimeout(drop, i * 90);
  }

  /** Teleporte (Piscar, Investida): clarão onde saiu e onde chegou. */
  blinkFlash(from: Vector3, to: Vector3, color: Color3): void {
    for (const p of [from, to]) {
      const at = p.add(new Vector3(0, 1, 0));
      this.flash(at, 2, color, 0.3);
      this.burst(at, 20, {
        tex: this.tex.flare,
        life: [0.3, 0.6],
        size: [0.08, 0.2],
        speed: [1, 3],
        colors: [c4(color.r, color.g, color.b), c4(1, 1, 1, 0.8), c4(color.r, color.g, color.b, 0)],
        emitter: new CylinderParticleEmitter(0.4, 1.6, 1, 0),
      });
    }
    // Rastro no chão entre os dois pontos.
    const n = Math.max(2, Math.round(Vector3.Distance(from, to) / 1.2));
    for (let i = 1; i < n; i++) {
      const p = Vector3.Lerp(from, to, i / n);
      this.decal(p, this.tex.flare, color, { size: [0.6, 1], dur: 0.5 });
    }
  }

  /** Nuvem de fumaça parada no chão (Bomba de Fumaça). */
  smokeCloud(at: Vector3, radius: number, secs: number): void {
    const p = at.clone();
    this.stream(() => p, 40, {
      tex: this.tex.smoke,
      life: [1.2, 2],
      size: [1, 1.8],
      speed: [0.1, 0.4],
      colors: [c4(0.18, 0.16, 0.22, 0.7), c4(0.1, 0.08, 0.12, 0.65), c4(0.05, 0.05, 0.08, 0)],
      emitter: new CylinderParticleEmitter(radius * 0.8, 0.4, 1, 0),
      additive: false,
      grow: 0.5,
    }, secs);
  }

  /** Estandarte cravado no chão. */
  bannerPlant(at: Vector3, color: Color3): void {
    this.slam(at, 1.5, color, false, false);
    this.decal(at, this.tex.runes, color, { size: [1, 3], dur: 0.8, spin: 1 });
  }

  /** Frasco que estoura em veneno (Frasco Tóxico, Praga). */
  poisonSplash(at: Vector3, radius: number, color: Color3): void {
    this.decal(new Vector3(at.x, 0, at.z), this.tex.scorch, color.scale(0.6), { size: [radius, radius * 1.6], dur: 2.5, additive: false });
    this.burst(at, 30, {
      tex: this.tex.smoke,
      life: [0.6, 1.2],
      size: [0.4, 0.8],
      speed: [1, 2.5],
      colors: [c4(color.r, color.g, color.b, 0.7), c4(color.r * 0.6, color.g * 0.6, color.b * 0.6, 0.6), c4(0.1, 0.15, 0.05, 0)],
      emitter: new SphereParticleEmitter(radius * 0.4),
      additive: false,
      grow: 0.6,
    });
  }

  /** Bolha em volta do corpo (escudos). */
  private bubble(body: FxBody, color: Color3, secs: number, tex: Texture | null = null): FxHandle {
    const m = MeshBuilder.CreateSphere("fx-bubble", { diameter: 2.1 * body.scale, segments: 18 }, this.scene);
    m.material = this.mat(color, tex ?? this.tex.ring, true, 0.55);
    m.isPickable = false;
    m.parent = body.root;
    m.position.y = 0.95 * body.scale;
    this.addGlow(m);
    let fadeAt = -1;
    let stop = false;
    this.run(
      (t) => {
        if (fadeAt < 0 && (t > secs - 0.3 || stop)) fadeAt = t;
        const intro = Math.min(1, t / 0.25);
        const out = fadeAt < 0 ? 1 : Math.max(0, 1 - (t - fadeAt) / 0.3);
        m.scaling.setAll(0.7 + 0.3 * intro + 0.03 * Math.sin(t * 4));
        m.visibility = intro * out * (0.55 + 0.15 * Math.sin(t * 3));
        return out > 0 && !body.root.isDisposed();
      },
      () => {
        this.removeGlow(m);
        m.dispose();
      },
    );
    return { stop: () => (stop = true) };
  }

  /** Sinal girando acima da cabeça (Marca da Morte, Provocar). */
  private headSigil(body: FxBody, color: Color3, secs: number, tex: Texture): FxHandle {
    const m = MeshBuilder.CreatePlane("fx-sigil", { size: 0.9 }, this.scene);
    m.material = this.mat(color, tex);
    m.billboardMode = Mesh.BILLBOARDMODE_ALL;
    m.isPickable = false;
    m.parent = body.head;
    m.position.y = 0.35;
    let fadeAt = -1;
    let stop = false;
    this.run(
      (t) => {
        if (fadeAt < 0 && (t > secs - 0.25 || stop)) fadeAt = t;
        const out = fadeAt < 0 ? 1 : Math.max(0, 1 - (t - fadeAt) / 0.25);
        m.scaling.setAll((0.9 + 0.12 * Math.sin(t * 5)) * out);
        m.rotation.z = t * 1.5;
        return out > 0 && !body.head.isDisposed();
      },
      () => m.dispose(),
    );
    return { stop: () => (stop = true) };
  }

  // ------------------------------------------------------------------------------------
  // Raid boss
  // ------------------------------------------------------------------------------------

  // O aviso no chão (círculo, cone, linha e anel) fica em ground-warnings.ts: um pool fixo
  // de quadrados com a forma no shader, sem malha nova por aviso.

  /** O golpe do aviso caiu: ossos explodindo do chão (Onda/Chuva) ou a marca estourando. */
  telegraphImpact(at: Vector3, radius: number, color: Color3, kind: "wave" | "rain" | "mark", mine: boolean): void {
    const ground = new Vector3(at.x, 0, at.z);
    if (kind === "rain") {
      // Uma lança de osso caindo do céu.
      this.orb(ground.add(new Vector3(-2, 16, -1.5)), ground, 0.45, {
        core: new Color3(0.95, 0.92, 0.82),
        halo: color,
        size: 0.7,
        arc: 0,
        smoke: false,
        trail: {
          tex: this.tex.spark,
          life: [0.15, 0.3],
          size: [0.2, 0.4],
          speed: [0, 0.3],
          colors: [c4(0.9, 0.85, 0.75), c4(color.r, color.g, color.b), c4(0.2, 0.1, 0.2, 0)],
        },
        rate: 90,
        onArrive: (p) => this.slam(p, radius, color, mine),
      });
      return;
    }
    this.slam(ground, radius, color, mine, kind === "wave");
    // Estilhaços de osso.
    this.burst(ground.add(new Vector3(0, 0.3, 0)), kind === "wave" ? 60 : 36, {
      tex: this.tex.spark,
      life: [0.4, 0.8],
      size: [0.15, 0.3],
      speed: [4, 9],
      colors: [c4(0.95, 0.92, 0.82), c4(0.85, 0.8, 0.7), c4(0.4, 0.35, 0.3, 0)],
      emitter: new HemisphericParticleEmitter(radius * 0.4, 0),
      gravity: -14,
      stretched: true,
    });
    this.lightFlash(ground.add(new Vector3(0, 2, 0)), color, 3, 0.4);
  }

  /** Guardas Ossários saindo do chão. */
  summonRise(at: Vector3): void {
    const ground = new Vector3(at.x, 0, at.z);
    this.decal(ground, this.tex.runes, new Color3(0.55, 0.25, 0.85), { size: [0.5, 2.6], dur: 1.6, spin: 2 });
    this.decal(ground, this.tex.crack, new Color3(0.2, 0.16, 0.14), { size: [1.4, 2], dur: 2.5, additive: false });
    this.burst(ground.add(new Vector3(0, 0.2, 0)), 30, {
      tex: this.tex.smoke,
      life: [0.7, 1.2],
      size: [0.4, 0.8],
      speed: [0.6, 1.6],
      colors: [c4(0.3, 0.22, 0.35, 0.75), c4(0.2, 0.15, 0.25, 0.6), c4(0.1, 0.08, 0.12, 0)],
      emitter: rising(0.8, 0.1, 0.3),
      additive: false,
      grow: 0.7,
    });
  }

  /** Aura do raid boss: névoa e faíscas subindo e um anel de runas girando aos pés. */
  raidAura(body: FxBody, color: Color3): FxHandle {
    const feet = () => (body.root.isDisposed() ? null : body.root.position);
    return this.all([
      this.footRing(body, this.tex.runes, color, Infinity, 2.2, 0.4),
      this.stream(feet, 26, {
        tex: this.tex.flare,
        life: [1, 1.8],
        size: [0.12, 0.3],
        speed: [0.6, 1.4],
        colors: [c4(color.r, color.g, color.b, 0.9), c4(color.r * 0.7, color.g * 0.7, color.b * 0.7, 0.8), c4(0.1, 0, 0.1, 0)],
        emitter: rising(0.9 * body.scale, 0.2, 0.2),
      }),
      this.stream(feet, 10, {
        tex: this.tex.smoke,
        life: [1.4, 2.2],
        size: [0.6, 1.2],
        speed: [0.2, 0.6],
        colors: [c4(0.15, 0.08, 0.2, 0.45), c4(0.1, 0.05, 0.12, 0.35), c4(0.05, 0.02, 0.06, 0)],
        emitter: rising(1.1 * body.scale, 0.1, 0.3),
        additive: false,
        grow: 1.4,
      }),
    ]);
  }

  effect(kind: Effect, body: FxBody, ms: number): FxHandle | null {
    const secs = ms / 1000;
    const feet = () => (body.root.isDisposed() ? null : body.root.position);
    const chest = () => (body.root.isDisposed() ? null : body.root.position.add(new Vector3(0, 0.9 * body.scale, 0)));
    switch (kind) {
      case Effect.STUN:
        return this.stunStars(body, secs);
      case Effect.SLOW:
        return this.all([
          this.stream(chest, 30, {
            tex: this.tex.flake,
            life: [0.6, 1],
            size: [0.16, 0.28],
            speed: [0.1, 0.4],
            colors: [c4(0.9, 0.97, 1), c4(0.65, 0.88, 1), c4(0.5, 0.8, 1, 0)],
            emitter: rising(0.5 * body.scale, 1.4 * body.scale, 0.3, -0.4),
            gravity: -1.2,
            spin: 3,
          }, secs),
          this.footRing(body, this.tex.frostRing, new Color3(0.5, 0.8, 1), secs, 1.5),
        ]);
      case Effect.BUFF_DEF:
        return this.all([
          this.footRing(body, this.tex.runes, new Color3(0.4, 0.65, 1), secs, 2, 0.6),
          this.stream(feet, 16, {
            tex: this.tex.flare,
            life: [0.8, 1.2],
            size: [0.1, 0.18],
            speed: [0.8, 1.4],
            colors: [c4(0.6, 0.8, 1), c4(0.4, 0.6, 1), c4(0.3, 0.5, 1, 0)],
            emitter: rising(0.6 * body.scale, 0.1, 0.1),
          }, secs),
        ]);
      case Effect.BUFF_ATK:
        return this.all([
          this.stream(feet, 60, {
            tex: this.tex.flame,
            life: [0.45, 0.75],
            size: [0.35, 0.6],
            speed: [1, 2],
            colors: [c4(1, 0.6, 0.18), c4(1, 0.3, 0.06), c4(0.4, 0, 0, 0)],
            emitter: rising(0.5 * body.scale, 0.1, 0.2),
            gravity: 1.5,
            grow: 0.4,
            spin: 2,
          }, secs),
          this.footRing(body, this.tex.ring, new Color3(1, 0.3, 0.12), secs, 1.6),
        ]);
      case Effect.BUFF_SPEED:
        // Rastro de sombras que fica para trás quando anda.
        return this.stream(() => (body.root.isDisposed() ? null : body.root.position.add(new Vector3(0, 0.5, 0))), 28, {
          tex: this.tex.smoke,
          life: [0.35, 0.6],
          size: [0.45, 0.75],
          speed: [0, 0.2],
          colors: [c4(0.3, 0.18, 0.45, 0.5), c4(0.12, 0.06, 0.2, 0.45), c4(0.1, 0.05, 0.15, 0)],
          emitter: new CylinderParticleEmitter(0.3, 0.9, 1, 0),
          additive: false,
          grow: 0.4,
        }, secs);
      case Effect.ROOT:
        return this.all([
          this.footRing(body, this.tex.frostRing, new Color3(0.55, 0.85, 1), secs, 2.2),
          this.bubble(body, new Color3(0.6, 0.9, 1), secs, this.tex.flake),
        ]);
      case Effect.POISON:
      case Effect.PLAGUE: {
        const plague = kind === Effect.PLAGUE;
        return this.stream(chest, plague ? 26 : 18, {
          tex: plague ? this.tex.smoke : this.tex.flare,
          life: [0.6, 1.1],
          size: plague ? [0.25, 0.45] : [0.08, 0.18],
          speed: [0.3, 0.8],
          colors: plague
            ? [c4(0.3, 0.45, 0.15, 0.7), c4(0.15, 0.25, 0.08, 0.6), c4(0.05, 0.08, 0.03, 0)]
            : [c4(0.55, 1, 0.35), c4(0.3, 0.85, 0.2), c4(0.1, 0.4, 0.05, 0)],
          emitter: rising(0.45 * body.scale, 1.2 * body.scale, 0.2),
          additive: !plague,
          gravity: 0.6,
        }, secs);
      }
      case Effect.BURN:
        return this.stream(chest, 40, {
          tex: this.tex.flame,
          life: [0.35, 0.6],
          size: [0.25, 0.45],
          speed: [0.8, 1.5],
          colors: [c4(1, 0.65, 0.2), c4(1, 0.3, 0.06), c4(0.3, 0, 0, 0)],
          emitter: rising(0.35 * body.scale, 1.2 * body.scale, 0.2),
          gravity: 1.5,
          grow: 0.3,
        }, secs);
      case Effect.TAUNT:
        return this.all([
          this.headSigil(body, new Color3(1, 0.3, 0.2), secs, this.tex.star),
          this.footRing(body, this.tex.ring, new Color3(1, 0.3, 0.2), secs, 1.6),
        ]);
      case Effect.SHIELD:
        return this.bubble(body, new Color3(1, 0.82, 0.35), secs);
      case Effect.MANA_SHIELD:
        return this.bubble(body, new Color3(0.4, 0.55, 1), secs, this.tex.runes);
      case Effect.INVISIBLE:
        // A transparência do modelo é feita pelo EntityManager; aqui, só a fumaça de entrada.
        this.burst(body.root.position.add(new Vector3(0, 0.8, 0)), 30, {
          tex: this.tex.smoke,
          life: [0.6, 1],
          size: [0.5, 0.9],
          speed: [0.5, 1.5],
          colors: [c4(0.2, 0.12, 0.3, 0.7), c4(0.1, 0.06, 0.15, 0.6), c4(0.05, 0.03, 0.08, 0)],
          emitter: new CylinderParticleEmitter(0.5, 1.4, 1, 0),
          additive: false,
          grow: 0.6,
        });
        return null;
      case Effect.MARK:
        return this.headSigil(body, new Color3(1, 0.15, 0.15), secs, this.tex.runes);
      case Effect.DEATH_MARK:
        return this.all([
          this.headSigil(body, new Color3(0.85, 0.2, 1), secs, this.tex.runes),
          this.footRing(body, this.tex.ring, new Color3(0.85, 0.2, 1), secs, 2.2, 2),
        ]);
      case Effect.UNDYING:
        return this.all([
          this.footRing(body, this.tex.runes, new Color3(1, 0.8, 0.3), secs, 2.4, 1),
          this.stream(feet, 30, {
            tex: this.tex.flare,
            life: [0.7, 1.1],
            size: [0.1, 0.22],
            speed: [1, 1.8],
            colors: [c4(1, 0.9, 0.5), c4(1, 0.7, 0.2), c4(0.6, 0.3, 0, 0)],
            emitter: rising(0.6 * body.scale, 0.1, 0.15),
          }, secs),
        ]);
      case Effect.BLIND:
        return this.stream(() => (body.head.isDisposed() ? null : body.head.getAbsolutePosition()), 16, {
          tex: this.tex.smoke,
          life: [0.6, 1],
          size: [0.35, 0.6],
          speed: [0.05, 0.25],
          colors: [c4(0.12, 0.1, 0.15, 0.7), c4(0.06, 0.05, 0.08, 0.6), c4(0, 0, 0, 0)],
          emitter: new SphereParticleEmitter(0.3),
          additive: false,
          grow: 0.4,
        }, secs);
      case Effect.BUFF_ATK_SPEED:
        return this.stream(chest, 40, {
          tex: this.tex.spark,
          life: [0.2, 0.35],
          size: [0.1, 0.25],
          speed: [2, 4],
          colors: [c4(1, 0.5, 0.3), c4(1, 0.2, 0.1), c4(0.5, 0, 0, 0)],
          emitter: new SphereParticleEmitter(0.6 * body.scale),
          stretched: true,
        }, secs);
      case Effect.WEAKEN:
        return this.stream(chest, 18, {
          tex: this.tex.flare,
          life: [0.7, 1.1],
          size: [0.1, 0.2],
          speed: [0.2, 0.6],
          colors: [c4(0.6, 0.35, 0.8), c4(0.35, 0.15, 0.5), c4(0.1, 0, 0.2, 0)],
          emitter: rising(0.4 * body.scale, 1.4 * body.scale, 0.2, -0.6),
          gravity: -1.5,
        }, secs);
      case Effect.FROST_ARMOR:
        return this.all([
          this.bubble(body, new Color3(0.6, 0.9, 1), secs, this.tex.flake),
          this.stream(chest, 14, {
            tex: this.tex.flake,
            life: [0.6, 1],
            size: [0.12, 0.22],
            speed: [0.2, 0.5],
            colors: [c4(0.9, 0.97, 1), c4(0.7, 0.9, 1), c4(0.5, 0.8, 1, 0)],
            emitter: new SphereParticleEmitter(0.8 * body.scale),
            spin: 2,
          }, secs),
        ]);
      case Effect.WAR_CRY:
        return this.footRing(body, this.tex.ring, new Color3(1, 0.55, 0.2), secs, 1.8, 0.5);
      case Effect.BANNER:
        return this.footRing(body, this.tex.runes, new Color3(0.45, 0.7, 1), secs, 1.6, 0.8);
      case Effect.COURAGE:
        return this.stream(feet, 6, {
          tex: this.tex.flare,
          life: [0.8, 1.2],
          size: [0.08, 0.14],
          speed: [0.8, 1.2],
          colors: [c4(0.5, 1, 0.7), c4(0.3, 0.9, 0.5), c4(0.1, 0.5, 0.2, 0)],
          emitter: rising(0.5 * body.scale, 0.1, 0.1),
        }, secs);
      default:
        return null;
    }
  }

  private all(handles: (FxHandle | null)[]): FxHandle {
    return { stop: () => handles.forEach((h) => h?.stop()) };
  }

  /** Anel/círculo preso aos pés enquanto o efeito dura. */
  private footRing(body: FxBody, tex: Texture, color: Color3, secs: number, size: number, spin = 0): FxHandle {
    const m = MeshBuilder.CreateGround("fx-foot", { width: 1, height: 1 }, this.scene);
    m.material = this.mat(color, tex, true, 0.85);
    m.isPickable = false;
    m.parent = body.root;
    m.position.y = 0.05;
    const s = size * body.scale;
    let fadeAt = -1;
    let stopRequested = false;
    this.run(
      (t, dt) => {
        if (fadeAt < 0 && (t > secs - 0.3 || stopRequested)) fadeAt = t;
        const intro = Math.min(1, t / 0.2);
        const outro = fadeAt < 0 ? 1 : Math.max(0, 1 - (t - fadeAt) / 0.3);
        m.scaling.set(s * (0.6 + 0.4 * intro), 1, s * (0.6 + 0.4 * intro));
        m.visibility = intro * outro * (0.75 + 0.25 * Math.sin(t * 5));
        m.rotation.y += spin * dt;
        return outro > 0 && !body.root.isDisposed();
      },
      () => m.dispose(),
    );
    return { stop: () => (stopRequested = true) };
  }

  /** Estrelinhas brilhantes girando sobre a cabeça (atordoado). */
  private stunStars(body: FxBody, secs: number): FxHandle {
    const holder = new TransformNode("fx-stun", this.scene);
    holder.parent = body.head;
    const stars: Mesh[] = [];
    const mat = this.mat(new Color3(1, 0.88, 0.35), this.tex.star);
    for (let i = 0; i < 4; i++) {
      const s = MeshBuilder.CreatePlane("fx-star", { size: 0.32 }, this.scene);
      s.material = mat;
      s.billboardMode = Mesh.BILLBOARDMODE_ALL;
      s.isPickable = false;
      s.parent = holder;
      stars.push(s);
    }
    let fadeAt = -1;
    let stop = false;
    this.run(
      (t) => {
        if (fadeAt < 0 && (t > secs - 0.2 || stop)) fadeAt = t;
        const out = fadeAt < 0 ? 1 : Math.max(0, 1 - (t - fadeAt) / 0.2);
        stars.forEach((s, i) => {
          const a = t * 5 + (i * Math.PI * 2) / stars.length;
          s.position.set(Math.cos(a) * 0.38, -0.2 + Math.sin(t * 6 + i) * 0.05, Math.sin(a) * 0.38);
          s.scaling.setAll((0.8 + 0.3 * Math.sin(t * 12 + i * 2)) * out);
        });
        return out > 0 && !body.head.isDisposed();
      },
      () => holder.dispose(false, false), // material compartilhado (cache)
    );
    return { stop: () => (stop = true) };
  }

  // ------------------------------------------------------------------------------------
  // Auxiliares das receitas
  // ------------------------------------------------------------------------------------

  /** Projétil: núcleo + halo com brilho, rastro de partículas e leve arco. */
  private orb(
    from: Vector3,
    to: Vector3,
    dur: number,
    o: { core: Color3; halo: Color3; size: number; arc: number; trail: ParticleOpts; rate: number; smoke?: boolean; onArrive(at: Vector3): void },
  ): void {
    const core = MeshBuilder.CreateSphere("fx-orb", { diameter: o.size, segments: 10 }, this.scene);
    core.material = this.mat(o.core, null, false);
    core.isPickable = false;
    const halo = MeshBuilder.CreatePlane("fx-orb-halo", { size: o.size * 4.5 }, this.scene);
    halo.material = this.mat(o.halo, this.tex.flare);
    halo.billboardMode = Mesh.BILLBOARDMODE_ALL;
    halo.isPickable = false;
    halo.parent = core;
    this.addGlow(core);
    core.position.copyFrom(from);
    const pos = core.position;

    // A explosão na chegada é do mesmo dono do projétil.
    const arrive = this.keep(o.onArrive);
    const trail = this.particles("fx-trail", Math.ceil(o.rate * o.trail.life[1]) + 8, o.trail);
    trail.emitter = pos;
    this.budget.stream(trail, o.rate, this.owner);
    trail.particleEmitterType = new SphereParticleEmitter(o.size * 0.4);
    trail.start();
    let smoke: ParticleSystem | null = null;
    if (o.smoke) {
      smoke = this.particles("fx-smoke", 60, {
        tex: this.tex.smoke,
        life: [0.5, 0.9],
        size: [0.3, 0.55],
        speed: [0.1, 0.4],
        colors: [c4(0.3, 0.25, 0.22, 0.45), c4(0.2, 0.17, 0.15, 0.35), c4(0.15, 0.15, 0.15, 0)],
        additive: false,
        grow: 2,
        gravity: 0.8,
        spin: 1,
      });
      smoke.emitter = pos;
      this.budget.stream(smoke, 45, this.owner);
      smoke.start();
    }

    let arrived = -1;
    const tmp = new Vector3();
    this.run(
      (t) => {
        if (arrived < 0) {
          const k = Math.min(1, t / Math.max(0.05, dur));
          Vector3.LerpToRef(from, to, k, tmp);
          tmp.y += Math.sin(Math.PI * k) * o.arc;
          pos.copyFrom(tmp);
          core.scaling.setAll(1 + 0.12 * Math.sin(t * 40));
          if (k >= 1) {
            arrived = t;
            core.setEnabled(false);
            trail.stop();
            smoke?.stop();
            arrive(to.clone());
          }
          return true;
        }
        // Espera o rastro terminar de sumir.
        return t - arrived < 1;
      },
      () => {
        this.removeGlow(core);
        this.disposeParticles(trail);
        if (smoke) this.disposeParticles(smoke);
        core.dispose();
      },
    );
  }

  /** Corte varrendo na frente do alvo (um ou mais, com inclinações diferentes). */
  private slash(from: Vector3, at: Vector3, color: Color3, tilts: number[], size: number): void {
    const dx = at.x - from.x;
    const dz = at.z - from.z;
    const len = Math.hypot(dx, dz) || 1;
    const heading = Math.atan2(dx, dz);
    // Um pouco antes do alvo, do lado de quem bate.
    const pos = new Vector3(at.x - (dx / len) * 0.45, at.y + 0.1, at.z - (dz / len) * 0.45);
    tilts.forEach((tilt, i) => {
      const p = MeshBuilder.CreatePlane("fx-slash", { size: 1 }, this.scene);
      p.material = this.mat(color, this.tex.crescent);
      p.isPickable = false;
      p.position.copyFrom(pos);
      p.rotation.set(0, heading, tilt);
      this.addGlow(p);
      const delay = i * 0.07;
      const dir = tilt >= 0 ? 1 : -1;
      this.run(
        (t) => {
          const k = Math.max(0, t - delay) / 0.26;
          p.scaling.setAll(size * (0.85 + 0.25 * k));
          // Varre rápido no começo e desacelera.
          p.rotation.z = tilt + dir * (1.1 - 1.6 * (1 - (1 - Math.min(1, k)) ** 3));
          p.visibility = t < delay ? 0 : k < 0.2 ? k / 0.2 : Math.max(0, 1 - (k - 0.2) / 0.8);
          return k < 1;
        },
        () => {
          this.removeGlow(p);
          p.dispose();
        },
      );
    });
  }
}
