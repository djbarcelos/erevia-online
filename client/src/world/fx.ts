import { Color3, Mesh, MeshBuilder, Scene, StandardMaterial, TransformNode, Vector3 } from "@babylonjs/core";
import { AdvancedDynamicTexture, TextBlock } from "@babylonjs/gui";

interface Running {
  /** Devolve false quando terminou. t = segundos desde o início. */
  tick(t: number): boolean;
  start: number;
  dispose(): void;
  stopped: boolean;
}

/** Efeito contínuo que pode ser encerrado antes da hora (ex.: o efeito acabou). */
export interface FxHandle {
  stop(): void;
}

/** Efeitos visuais simples (formas do Babylon, sem arte extra). */
export class Fx {
  private running: Running[] = [];
  private mats = new Map<string, StandardMaterial>();

  constructor(
    private scene: Scene,
    private gui: AdvancedDynamicTexture,
  ) {
    scene.onBeforeRenderObservable.add(() => {
      const now = performance.now() / 1000;
      this.running = this.running.filter((r) => {
        if (!r.stopped && r.tick(now - r.start)) return true;
        r.dispose();
        return false;
      });
    });
  }

  private run(tick: Running["tick"], dispose: () => void): FxHandle {
    const r: Running = { tick, dispose, start: performance.now() / 1000, stopped: false };
    this.running.push(r);
    return { stop: () => (r.stopped = true) };
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

  private glow(color: Color3, alpha = 1): StandardMaterial {
    const key = `${color.toHexString()}-${alpha}`;
    let m = this.mats.get(key);
    if (!m) {
      m = new StandardMaterial(`fx-${key}`, this.scene);
      m.emissiveColor = color;
      m.diffuseColor = Color3.Black();
      m.disableLighting = true;
      m.alpha = alpha;
      this.mats.set(key, m);
    }
    return m;
  }

  /** Número subindo acima da cabeça. */
  damageNumber(anchor: TransformNode, text: string, color: string, big: boolean): void {
    const tb = new TextBlock();
    tb.text = text;
    tb.color = color;
    tb.fontSize = big ? 22 : 16;
    tb.fontWeight = big ? "bold" : "600";
    tb.outlineWidth = 3;
    tb.outlineColor = "black";
    tb.resizeToFit = true;
    this.gui.addControl(tb);
    tb.linkWithMesh(anchor);
    const drift = (Math.random() - 0.5) * 30;
    const dur = 0.9;
    this.run(
      (t) => {
        const k = t / dur;
        tb.linkOffsetY = -18 - k * 45;
        tb.linkOffsetX = drift * k;
        tb.alpha = k < 0.6 ? 1 : 1 - (k - 0.6) / 0.4;
        return k < 1;
      },
      () => tb.dispose(),
    );
  }

  /** Projétil luminoso de "from" até "to" em `dur` segundos. */
  projectile(from: Vector3, to: Vector3, color: Color3, size = 0.35, dur = Math.max(0.12, Vector3.Distance(from, to) / 30)): void {
    const ball = MeshBuilder.CreateSphere("fx-ball", { diameter: size, segments: 8 }, this.scene);
    ball.material = this.glow(color);
    ball.isPickable = false;
    ball.position.copyFrom(from);
    this.run(
      (t) => {
        Vector3.LerpToRef(from, to, Math.min(1, t / dur), ball.position);
        return t < dur;
      },
      () => ball.dispose(),
    );
  }

  /** Anel que se expande (área). */
  nova(at: Vector3, radius: number, color: Color3): void {
    const ring = MeshBuilder.CreateTorus("fx-nova", { diameter: 2, thickness: 0.12, tessellation: 48 }, this.scene);
    ring.material = this.glow(color, 0.85);
    ring.isPickable = false;
    ring.position.set(at.x, 0.15, at.z);
    const dur = 0.45;
    this.run(
      (t) => {
        const k = t / dur;
        ring.scaling.setAll(0.2 + k * radius);
        ring.visibility = 1 - k;
        return k < 1;
      },
      () => ring.dispose(),
    );
  }

  /** Aura colorida ao redor dos pés durante o efeito. */
  aura(root: TransformNode, color: Color3, ms: number, scale = 1): FxHandle {
    const ring = MeshBuilder.CreateTorus("fx-aura", { diameter: 1.3 * scale, thickness: 0.05, tessellation: 32 }, this.scene);
    ring.material = this.glow(color, 0.9);
    ring.isPickable = false;
    ring.parent = root;
    const dur = ms / 1000;
    return this.run(
      (t) => {
        ring.position.y = 0.1 + 0.6 * (0.5 + 0.5 * Math.sin(t * 4));
        ring.visibility = t > dur - 0.3 ? Math.max(0, (dur - t) / 0.3) : 1;
        return t < dur && !root.isDisposed();
      },
      () => ring.dispose(),
    );
  }

  /** Estrelinhas girando sobre a cabeça (atordoado). */
  stun(head: TransformNode, ms: number): FxHandle {
    const holder = new TransformNode("fx-stun", this.scene);
    holder.parent = head;
    const stars: Mesh[] = [];
    for (let i = 0; i < 3; i++) {
      const s = MeshBuilder.CreatePolyhedron("fx-star", { type: 1, size: 0.08 }, this.scene);
      s.material = this.glow(new Color3(1, 0.9, 0.3));
      s.isPickable = false;
      s.parent = holder;
      stars.push(s);
    }
    const dur = ms / 1000;
    return this.run(
      (t) => {
        stars.forEach((s, i) => {
          const a = t * 5 + (i * Math.PI * 2) / 3;
          s.position.set(Math.cos(a) * 0.35, -0.25, Math.sin(a) * 0.35);
        });
        return t < dur && !head.isDisposed();
      },
      () => holder.dispose(false, false), // material das estrelas é compartilhado (cache)
    );
  }

  /** Coluna dourada ao subir de nível. */
  levelUp(root: TransformNode): void {
    const col = MeshBuilder.CreateCylinder("fx-level", { height: 4, diameterTop: 0.2, diameterBottom: 1.6, tessellation: 24 }, this.scene);
    col.material = this.glow(new Color3(1, 0.85, 0.35), 0.55);
    col.isPickable = false;
    col.parent = root;
    col.position.y = 2;
    const dur = 1.4;
    this.run(
      (t) => {
        const k = t / dur;
        col.scaling.set(1 + k * 0.6, 0.3 + k, 1 + k * 0.6);
        col.visibility = 1 - k;
        return k < 1 && !root.isDisposed();
      },
      () => col.dispose(),
    );
  }
}
