/**
 * Raid boss no mundo: avisos no chão (o golpe cai no fim do círculo), a aparência do
 * chefe (coroa, aura, tom escuro), as invocações saindo do chão e a barra no topo quando
 * você está perto. Os dados vêm do servidor (BossStatus, Telegraph e Announce).
 */

import { Color3, Mesh, MeshBuilder, PBRMaterial, StandardMaterial, TransformNode, Vector3, type AbstractMesh, type Scene } from "@babylonjs/core";
import { TelegraphKind, type Announce, type BossStatus, type Telegraph } from "../gen/erevia/v1/game_pb";
import type { RaidUi } from "../ui/raid";
import type { EntityManager } from "./entities";
import type { Fx, FxHandle } from "./fx";

/** Distância em que a barra do chefe aparece (m). */
const BAR_DISTANCE = 60;
/** A lança de osso da Chuva leva este tempo para cair do céu (ms, ver Fx.telegraphImpact). */
const RAIN_FALL_MS = 450;

const COLORS: Record<number, Color3> = {
  [TelegraphKind.WAVE]: new Color3(1, 0.22, 0.18),
  [TelegraphKind.RAIN]: new Color3(1, 0.42, 0.2),
  [TelegraphKind.MARK]: new Color3(0.85, 0.2, 1),
};
const AURA = new Color3(0.62, 0.25, 1);
/** Osso um pouco mais escuro e acinzentado que o dos esqueletos comuns. */
const BONE_TINT = new Color3(0.8, 0.77, 0.74);
/** Coroa no espaço do osso da cabeça (o chapéu do mago fica em y 0,54). */
const CROWN = { y: 0.92, radius: 0.42, band: 0.09, spike: 0.34 };
const AURA_ENRAGED = new Color3(1, 0.25, 0.15);

interface Decoration {
  root: TransformNode;
  crown: TransformNode | null;
  aura: FxHandle;
  enraged: boolean;
  tinted: Set<AbstractMesh>;
}

export class RaidDirector {
  private bosses = new Map<number, BossStatus>();
  private telegraphs = new Map<number, { handle: FxHandle; timer: number }>();
  private decorations = new Map<number, Decoration>();
  private listeners: (() => void)[] = [];
  private tintCache = new Map<string, PBRMaterial | StandardMaterial>();

  constructor(
    private scene: Scene,
    private fx: Fx,
    private entities: EntityManager,
    private ui: RaidUi,
    private localId: number,
    private onAnnounceLine: (text: string) => void,
  ) {}

  /** Lista inicial (Welcome). */
  setBosses(list: BossStatus[]): void {
    this.bosses.clear();
    for (const b of list) this.bosses.set(b.id, b);
    this.changed();
  }

  /** Um chefe mudou (nasceu, morreu, trocou de fase…). */
  onStatus(s: BossStatus): void {
    this.bosses.set(s.id, s);
    this.changed();
  }

  /** Estado de um chefe pelo ponto fixo (o mapa conhece os chefes pela posição). */
  statusAt(x: number, z: number): BossStatus | undefined {
    for (const b of this.bosses.values()) if (Math.hypot(b.x - x, b.z - z) < 1) return b;
    return undefined;
  }

  all(): BossStatus[] {
    return [...this.bosses.values()];
  }

  onChange(fn: () => void): () => void {
    this.listeners.push(fn);
    return () => (this.listeners = this.listeners.filter((f) => f !== fn));
  }

  private changed(): void {
    for (const fn of this.listeners) fn();
  }

  announce(a: Announce): void {
    this.ui.announce(a.kind, a.title, a.text);
    this.onAnnounceLine(a.text);
  }

  /** Aviso no chão: começo, golpe ou cancelamento. */
  onTelegraph(t: Telegraph): void {
    const at = new Vector3(t.x, 0, t.z);
    const color = COLORS[t.kind] ?? COLORS[TelegraphKind.WAVE];
    const kind = t.kind === TelegraphKind.RAIN ? "rain" : t.kind === TelegraphKind.MARK ? "mark" : "wave";
    const mine = this.distanceToMe(t.x, t.z) <= t.radius + 1;
    if (t.cancelled || t.resolved) {
      const cur = this.telegraphs.get(t.id);
      if (cur) {
        cur.handle.stop();
        window.clearTimeout(cur.timer);
        this.telegraphs.delete(t.id);
      }
      // A Chuva já começou a cair antes (para encostar junto com o dano).
      if (t.resolved && kind !== "rain") this.fx.telegraphImpact(at, t.radius, color, kind, mine);
      return;
    }
    const follow = t.followId ? () => this.entities.rootOf(t.followId)?.position ?? null : undefined;
    const handle = this.fx.telegraph(at, t.radius, t.durationMs, color, follow);
    let timer = 0;
    if (kind === "rain") {
      timer = window.setTimeout(() => this.fx.telegraphImpact(at, t.radius, color, "rain", this.distanceToMe(t.x, t.z) <= t.radius + 1), Math.max(0, t.durationMs - RAIN_FALL_MS));
    }
    this.telegraphs.set(t.id, { handle, timer });
  }

  /** Um Guarda Ossário nasceu: fumaça e runas no chão. */
  onSpawn(id: number, model: string, x: number, z: number): void {
    if (model === "skeleton_warrior" && this.nearLair(x, z)) this.fx.summonRise(new Vector3(x, 0, z));
    void id;
  }

  /** Por quadro: decora o chefe quando ele aparece e atualiza a barra. */
  update(): void {
    let bar: Parameters<RaidUi["setBar"]>[0] = null;
    for (const b of this.bosses.values()) {
      if (!b.raid) continue;
      const info = this.entities.info(b.id);
      if (!info || info.dead || !b.alive) {
        this.undecorate(b.id);
        continue;
      }
      this.decorate(b);
      const root = this.entities.rootOf(b.id);
      if (root && this.distanceToMe(root.position.x, root.position.z) <= BAR_DISTANCE) {
        bar = { name: info.name, hp: info.hp, maxHp: info.maxHp, phase: b.phase, enraged: b.enraged, final: b.finalEnrage, fightStartMs: Number(b.fightStartMs) };
      }
    }
    this.ui.setBar(bar);
  }

  dispose(): void {
    for (const t of this.telegraphs.values()) {
      t.handle.stop();
      window.clearTimeout(t.timer);
    }
    this.telegraphs.clear();
    for (const id of [...this.decorations.keys()]) this.undecorate(id);
    this.ui.setBar(null);
  }

  private nearLair(x: number, z: number): boolean {
    for (const b of this.bosses.values()) if (b.raid && Math.hypot(b.x - x, b.z - z) < 40) return true;
    return false;
  }

  private distanceToMe(x: number, z: number): number {
    const me = this.entities.rootOf(this.localId)?.position;
    return me ? Math.hypot(me.x - x, me.z - z) : Infinity;
  }

  // ---------- Aparência do chefe ----------

  private decorate(b: BossStatus): void {
    const body = this.entities.bodyOf(b.id);
    if (!body) return;
    const enraged = b.enraged || b.finalEnrage;
    let d = this.decorations.get(b.id);
    if (d && d.root !== body.root) {
      this.undecorate(b.id); // a entidade foi recriada (renasceu)
      d = undefined;
    }
    if (!d) {
      d = { root: body.root, crown: null, aura: this.fx.raidAura(body, enraged ? AURA_ENRAGED : AURA), enraged, tinted: new Set() };
      this.decorations.set(b.id, d);
    } else if (d.enraged !== enraged) {
      d.aura.stop();
      d.aura = this.fx.raidAura(body, enraged ? AURA_ENRAGED : AURA);
      d.enraged = enraged;
    }
    // O modelo 3D carrega depois da entidade: tira o chapéu, põe a coroa no osso da cabeça
    // e tinge as malhas que forem aparecendo.
    for (const m of this.entities.meshesOf(b.id)) {
      if (d.tinted.has(m)) continue;
      d.tinted.add(m);
      if (m.name.startsWith("Skeleton_Mage_Hat")) {
        m.setEnabled(false);
        if (!d.crown && m.parent instanceof TransformNode) d.crown = this.buildCrown(m.parent);
        continue;
      }
      this.tint(m);
    }
  }

  private undecorate(id: number): void {
    const d = this.decorations.get(id);
    if (!d) return;
    d.aura.stop();
    d.crown?.dispose(false, true);
    this.decorations.delete(id);
  }

  /**
   * Coroa dourada com pontas e uma joia roxa, presa ao osso da cabeça (acompanha as
   * animações). Medidas no espaço do osso (o mesmo do chapéu que ela substitui).
   */
  private buildCrown(headBone: TransformNode): TransformNode {
    const holder = new TransformNode("raid-crown", this.scene);
    holder.parent = headBone;
    holder.position.y = CROWN.y;
    const gold = this.crownMaterial("raid-gold", new Color3(0.95, 0.72, 0.25), new Color3(0.32, 0.2, 0.03));
    const gem = this.crownMaterial("raid-gem", new Color3(0.7, 0.2, 1), new Color3(0.55, 0.15, 0.9));
    const r = CROWN.radius;
    const band = MeshBuilder.CreateTorus("raid-crown-band", { diameter: r * 2, thickness: CROWN.band, tessellation: 28 }, this.scene);
    band.material = gold;
    band.parent = holder;
    const spikes = 7;
    for (let i = 0; i < spikes; i++) {
      const a = (i / spikes) * Math.PI * 2;
      const spike = MeshBuilder.CreateCylinder("raid-crown-spike", { height: CROWN.spike, diameterTop: 0, diameterBottom: CROWN.band * 1.7, tessellation: 6 }, this.scene);
      spike.material = gold;
      spike.parent = holder;
      spike.position.set(Math.cos(a) * r, CROWN.spike / 2, Math.sin(a) * r);
    }
    const jewel = MeshBuilder.CreateSphere("raid-crown-gem", { diameter: CROWN.band * 1.6, segments: 8 }, this.scene);
    jewel.material = gem;
    jewel.parent = holder;
    jewel.position.set(0, CROWN.band * 0.4, r * 1.02);
    for (const m of holder.getChildMeshes()) {
      m.isPickable = false;
      if (m instanceof Mesh) this.fx.glowInclude(m);
    }
    return holder;
  }

  private crownMaterial(name: string, color: Color3, emissive: Color3): StandardMaterial {
    const key = `crown-${name}`;
    let m = this.tintCache.get(key) as StandardMaterial | undefined;
    if (!m) {
      m = new StandardMaterial(name, this.scene);
      m.diffuseColor = color;
      m.emissiveColor = emissive;
      m.specularColor = new Color3(0.9, 0.8, 0.5);
      this.tintCache.set(key, m);
    }
    return m;
  }

  /**
   * Osso envelhecido (um pouco mais escuro) e olhos brilhando em roxo. Cópia do material:
   * os outros esqueletos magos não mudam.
   */
  private tint(m: AbstractMesh): void {
    const src = m.material;
    if (!src) return;
    let t = this.tintCache.get(src.uniqueId.toString());
    if (!t) {
      const eyes = src.name.toLowerCase().startsWith("glow");
      if (src instanceof PBRMaterial) {
        const c = src.clone(`${src.name}-raid`);
        if (eyes) c.emissiveColor = new Color3(0.85, 0.3, 1);
        else c.albedoColor = src.albedoColor.multiply(BONE_TINT);
        t = c;
      } else if (src instanceof StandardMaterial) {
        const c = src.clone(`${src.name}-raid`);
        if (eyes) c.emissiveColor = new Color3(0.85, 0.3, 1);
        else c.diffuseColor = src.diffuseColor.multiply(BONE_TINT);
        t = c;
      } else return;
      this.tintCache.set(src.uniqueId.toString(), t);
    }
    m.material = t;
  }
}
