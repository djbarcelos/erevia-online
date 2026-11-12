import { Color3, Mesh, MeshBuilder, Scene, StandardMaterial, TransformNode, Vector3 } from "@babylonjs/core";
import { AdvancedDynamicTexture, Rectangle, TextBlock } from "@babylonjs/gui";
import { classInfo } from "../classes";
import type { Entity, EntityState } from "../gen/erevia/v1/game_pb";

/** Quão rápido a posição desenhada converge para a do servidor (1/s). */
const CORRECTION_RATE = 12;
/** Tempo máximo esperando o servidor confirmar um destino previsto. */
const PENDING_TIMEOUT_MS = 1000;

interface Player {
  id: number;
  name: string;
  speed: number;
  /** Posição do servidor, extrapolada localmente em direção ao destino. */
  server: { x: number; z: number };
  target: { x: number; z: number };
  root: TransformNode;
  body: Mesh;
  label: Rectangle;
}

/** Jogadores na tela: cria/remove malhas e suaviza o movimento entre snapshots. */
export class EntityManager {
  private players = new Map<number, Player>();
  private localPending: { x: number; z: number; at: number } | null = null;
  private classMats = new Map<string, StandardMaterial>();
  private ringMat: StandardMaterial;

  constructor(
    private scene: Scene,
    private gui: AdvancedDynamicTexture,
    readonly localId: number,
    private zoneHalf: number,
  ) {
    this.ringMat = new StandardMaterial("localRing", scene);
    this.ringMat.emissiveColor = new Color3(0.95, 0.8, 0.4);
    this.ringMat.disableLighting = true;
    this.ringMat.alpha = 0.8;
  }

  private classMaterial(cls: string): StandardMaterial {
    let mat = this.classMats.get(cls);
    if (!mat) {
      mat = new StandardMaterial(`cls-${cls}`, this.scene);
      mat.diffuseColor = Color3.FromHexString(classInfo(cls).color);
      this.classMats.set(cls, mat);
    }
    return mat;
  }

  get count(): number {
    return this.players.size;
  }

  get local(): Player | undefined {
    return this.players.get(this.localId);
  }

  add(e: Entity): void {
    if (this.players.has(e.id)) return;
    const isLocal = e.id === this.localId;

    const root = new TransformNode(`p${e.id}`, this.scene);
    root.position.set(e.x, 0, e.z);

    const body = MeshBuilder.CreateCapsule(`body${e.id}`, { height: 1.8, radius: 0.35 }, this.scene);
    body.parent = root;
    body.position.y = 0.9;
    body.material = this.classMaterial(e.class);
    body.isPickable = false;

    // Anel dourado no chão marca o seu personagem.
    if (isLocal) {
      const ring = MeshBuilder.CreateTorus(`ring${e.id}`, { diameter: 1.1, thickness: 0.06, tessellation: 32 }, this.scene);
      ring.parent = root;
      ring.position.y = 0.03;
      ring.material = this.ringMat;
      ring.isPickable = false;
    }

    // "Nariz" para mostrar para onde o personagem está virado.
    const nose = MeshBuilder.CreateBox(`nose${e.id}`, { width: 0.2, height: 0.12, depth: 0.3 }, this.scene);
    nose.parent = body;
    nose.position.set(0, 0.45, 0.35);
    nose.material = body.material;
    nose.isPickable = false;

    const head = new TransformNode(`head${e.id}`, this.scene);
    head.parent = root;
    head.position.y = 2.3;

    const label = new Rectangle(`label${e.id}`);
    label.width = `${Math.max(60, e.name.length * 9 + 16)}px`;
    label.height = "22px";
    label.cornerRadius = 4;
    label.thickness = 0;
    label.background = "rgba(10,8,14,0.55)";
    const text = new TextBlock();
    text.text = e.name;
    text.color = isLocal ? "#f0d79a" : "#e6ecff";
    text.fontSize = 13;
    label.addControl(text);
    this.gui.addControl(label);
    label.linkWithMesh(head);

    this.players.set(e.id, {
      id: e.id,
      name: e.name,
      speed: e.speed,
      server: { x: e.x, z: e.z },
      target: { x: e.targetX, z: e.targetZ },
      root,
      body,
      label,
    });
  }

  remove(id: number): void {
    const p = this.players.get(id);
    if (!p) return;
    p.label.dispose();
    p.root.dispose(false, true);
    this.players.delete(id);
  }

  applyState(s: EntityState): void {
    const p = this.players.get(s.id);
    if (!p) return;
    p.server.x = s.x;
    p.server.z = s.z;

    // Jogador local: enquanto o servidor não confirma o destino que já previmos,
    // não deixa um snapshot antigo "puxar" o personagem de volta.
    if (s.id === this.localId && this.localPending) {
      const pend = this.localPending;
      const confirmed = Math.abs(s.targetX - pend.x) < 0.05 && Math.abs(s.targetZ - pend.z) < 0.05;
      const expired = performance.now() - pend.at > PENDING_TIMEOUT_MS;
      if (!confirmed && !expired) return;
      this.localPending = null;
    }
    p.target.x = s.targetX;
    p.target.z = s.targetZ;
  }

  /** Previsão: o jogador local começa a andar na hora, sem esperar o servidor. */
  predictLocalMove(x: number, z: number): { x: number; z: number } {
    const cx = clamp(x, -this.zoneHalf, this.zoneHalf);
    const cz = clamp(z, -this.zoneHalf, this.zoneHalf);
    const p = this.local;
    if (p) {
      p.target.x = cx;
      p.target.z = cz;
    }
    this.localPending = { x: cx, z: cz, at: performance.now() };
    return { x: cx, z: cz };
  }

  /** Chamado a cada frame. */
  update(dt: number): void {
    const blend = 1 - Math.exp(-CORRECTION_RATE * dt);
    for (const p of this.players.values()) {
      // Extrapola a posição do servidor (mesma regra de movimento do Go).
      const dx = p.target.x - p.server.x;
      const dz = p.target.z - p.server.z;
      const dist = Math.hypot(dx, dz);
      const stepLen = p.speed * dt;
      if (dist > 0) {
        if (dist <= stepLen) {
          p.server.x = p.target.x;
          p.server.z = p.target.z;
        } else {
          p.server.x += (dx / dist) * stepLen;
          p.server.z += (dz / dist) * stepLen;
        }
      }

      // Aproxima suavemente a posição desenhada da posição extrapolada.
      const pos = p.root.position;
      pos.x += (p.server.x - pos.x) * blend;
      pos.z += (p.server.z - pos.z) * blend;

      if (dist > 0.05) {
        const want = Math.atan2(dx, dz);
        p.body.rotation.y = lerpAngle(p.body.rotation.y, want, blend);
      }
    }
  }

  localPosition(): Vector3 | undefined {
    return this.local?.root.position;
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
