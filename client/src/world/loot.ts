import "@babylonjs/loaders/glTF";
import {
  Color3,
  InstancedMesh,
  LoadAssetContainerAsync,
  Mesh,
  MeshBuilder,
  Scene,
  StandardMaterial,
  TransformNode,
  Vector3,
} from "@babylonjs/core";
import { AdvancedDynamicTexture, TextBlock } from "@babylonjs/gui";
import { ItemKind, type GroundItem } from "../gen/erevia/v1/game_pb";
import { GRADE_COLOR, itemLabel, type ItemStore } from "../ui/items";

/** Até que distância o nome do item no chão aparece (m). */
const LABEL_DISTANCE = 22;
/** Altura e cor do feixe de luz dos itens bons, raros e épicos. */
const BEAM_COLORS: Record<number, Color3> = {
  2: new Color3(0.35, 0.85, 0.45),
  3: new Color3(0.35, 0.6, 1),
  4: new Color3(0.75, 0.4, 1),
};

interface Drop {
  data: GroundItem;
  root: TransformNode;
  sack: InstancedMesh;
  beam: InstancedMesh | null;
  label: TextBlock;
  /** performance.now() em que a reserva acaba. */
  freeAt: number;
  phase: number;
}

/**
 * Itens no chão: um saquinho (instância: todos num só desenho), um feixe de luz na cor da
 * raridade e o nome flutuando. Clicar pega (o servidor faz o personagem andar até lá).
 */
export class GroundLoot {
  private drops = new Map<number, Drop>();
  private sack: Mesh | null = null;
  private goldSack: Mesh | null = null;
  private beams = new Map<number, Mesh>();
  private pending: GroundItem[] = [];

  constructor(
    private scene: Scene,
    private gui: AdvancedDynamicTexture,
    private store: ItemStore,
    private localId: number,
  ) {
    void this.loadModel();
  }

  private async loadModel(): Promise<void> {
    let proto: Mesh;
    try {
      const c = await LoadAssetContainerAsync("/models/props/loot_sack.glb", this.scene);
      c.addAllToScene();
      const mesh = c.meshes.find((m): m is Mesh => m instanceof Mesh && m.getTotalVertices() > 0)!;
      mesh.setParent(null);
      mesh.bakeCurrentTransformIntoVertices();
      for (const r of c.rootNodes) if (r !== mesh) r.dispose();
      proto = mesh;
    } catch {
      // Sem o modelo: uma caixinha.
      proto = MeshBuilder.CreateBox("loot-box", { size: 0.4 }, this.scene);
      const m = new StandardMaterial("loot-box", this.scene);
      m.diffuseColor = new Color3(0.6, 0.45, 0.28);
      proto.material = m;
    }
    // O saco do cenário é grande (escala do kit): normaliza para ~65 cm.
    proto.refreshBoundingInfo();
    const size = proto.getBoundingInfo().boundingBox.extendSize.length() * 2;
    const k = size > 0 ? 0.65 / size : 1;
    proto.scaling.setAll(k);
    proto.bakeCurrentTransformIntoVertices();
    proto.isPickable = false;
    proto.setEnabled(false);
    this.sack = proto;

    // Moedas: o mesmo saco, dourado.
    const gold = proto.clone("loot-gold")!;
    const gm = new StandardMaterial("loot-gold", this.scene);
    gm.diffuseColor = new Color3(0.95, 0.75, 0.3);
    gm.emissiveColor = new Color3(0.25, 0.18, 0.04);
    gm.specularColor = new Color3(0.9, 0.8, 0.4);
    gold.material = gm;
    gold.setEnabled(false);
    this.goldSack = gold;

    for (const [grade, color] of Object.entries(BEAM_COLORS)) {
      const beam = MeshBuilder.CreateCylinder(`loot-beam-${grade}`, { height: 3, diameterTop: 0.02, diameterBottom: 0.35, tessellation: 12 }, this.scene);
      const bm = new StandardMaterial(`loot-beam-${grade}`, this.scene);
      bm.emissiveColor = color;
      bm.disableLighting = true;
      bm.alpha = Number(grade) >= 3 ? 0.4 : 0.25;
      beam.material = bm;
      beam.isPickable = false;
      beam.setEnabled(false);
      this.beams.set(Number(grade), beam);
    }
    const waiting = this.pending.splice(0);
    this.add(waiting);
  }

  add(items: GroundItem[]): void {
    if (!this.sack || !this.goldSack) {
      this.pending.push(...items);
      return;
    }
    for (const it of items) {
      if (this.drops.has(it.id)) continue;
      const info = this.store.get(it.itemId);
      if (!info) continue;
      const gold = info.kind === ItemKind.GOLD;
      const root = new TransformNode(`loot${it.id}`, this.scene);
      root.position.set(it.x, 0, it.z);
      const sack = (gold ? this.goldSack : this.sack).createInstance(`loot-sack${it.id}`);
      sack.parent = root;
      sack.rotation.y = Math.random() * Math.PI * 2;
      sack.isPickable = false;
      // Área de clique maior que o saquinho (invisível: não é desenhada).
      const hit = MeshBuilder.CreateCylinder(`loot-hit${it.id}`, { height: 0.9, diameter: 1 }, this.scene);
      hit.parent = root;
      hit.position.y = 0.45;
      hit.isVisible = false;
      hit.isPickable = true;
      hit.metadata = { lootId: it.id };
      const beamProto = this.beams.get(info.grade);
      let beam: InstancedMesh | null = null;
      if (beamProto && !gold) {
        beam = beamProto.createInstance(`loot-beam${it.id}`);
        beam.parent = root;
        beam.position.y = 1.5;
        beam.isPickable = false;
      }
      const label = new TextBlock(`loot-label${it.id}`, it.enchant > 0 ? `+${it.enchant} ${info.name}` : itemLabel(info, it.count));
      label.resizeToFit = true;
      label.fontSize = 12;
      label.fontWeight = "600";
      label.outlineWidth = 3;
      label.outlineColor = "rgba(0,0,0,0.85)";
      label.color = gold ? "#f3d488" : (GRADE_COLOR[info.grade] ?? GRADE_COLOR[1]);
      this.gui.addControl(label);
      label.linkWithMesh(root);
      label.linkOffsetY = -28;
      this.drops.set(it.id, {
        data: it, root, sack, beam, label,
        freeAt: performance.now() + (it.ownerId ? it.reservedMs : 0),
        phase: Math.random() * Math.PI * 2,
      });
    }
  }

  remove(ids: number[]): void {
    this.pending = this.pending.filter((p) => !ids.includes(p.id));
    for (const id of ids) {
      const d = this.drops.get(id);
      if (!d) continue;
      d.label.dispose();
      d.root.dispose(false, false); // instâncias; o material é compartilhado
      this.drops.delete(id);
    }
  }

  /** Posição de um item (para o efeito ao pegar). */
  position(id: number): Vector3 | null {
    return this.drops.get(id)?.root.position.clone() ?? null;
  }

  info(id: number): GroundItem | undefined {
    return this.drops.get(id)?.data;
  }

  /** Item do chão mais perto do jogador (testes no navegador). */
  nearest(from: Vector3): { id: number; dist: number; itemId: string } | null {
    let best: { id: number; dist: number; itemId: string } | null = null;
    for (const d of this.drops.values()) {
      const dist = Vector3.Distance(from, d.root.position);
      if (!best || dist < best.dist) best = { id: d.data.id, dist, itemId: d.data.itemId };
    }
    return best;
  }

  update(me: Vector3 | undefined): void {
    const now = performance.now();
    const t = now / 1000;
    for (const d of this.drops.values()) {
      const near = !me || Vector3.DistanceSquared(me, d.root.position) < LABEL_DISTANCE * LABEL_DISTANCE;
      d.label.isVisible = near;
      // Reservado para outro jogador: nome apagado até a reserva acabar.
      const reservedForOther = now < d.freeAt && d.data.ownerId !== this.localId;
      d.label.alpha = reservedForOther ? 0.45 : 1;
      if (d.beam) {
        d.beam.rotation.y = t * 0.8;
        d.beam.scaling.y = 0.9 + 0.1 * Math.sin(t * 2.2 + d.phase);
      }
      d.sack.position.y = 0.04 * Math.sin(t * 2 + d.phase) + 0.02;
    }
  }

  dispose(): void {
    this.remove([...this.drops.keys()]);
  }
}
