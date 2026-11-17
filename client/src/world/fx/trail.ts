import { Matrix, Mesh, Vector3, VertexBuffer, VertexData, type Material, type Scene, type TransformNode } from "@babylonjs/core";
import { CELL, cellU } from "./atlas";

/**
 * Rastro curto da arma no golpe: uma fita de 8 fatias entre o cabo e a ponta da lâmina,
 * com as posições dos últimos ~0,1 s. As malhas ficam num pool (no máximo POOL rastros ao
 * mesmo tempo) e só as posições mudam a cada quadro.
 */
const SLICES = 8;
/** Quanto tempo de golpe o rastro mostra (s). */
const SPAN_S = 0.1;
const POOL = 6;

interface Trail {
  mesh: Mesh;
  pos: Float32Array;
  /** Amostras (mais nova primeiro): tempo, cabo e ponta. */
  samples: { t: number; base: Vector3; tip: Vector3 }[];
  busy: boolean;
}

export class WeaponTrails {
  private pool: Trail[] = [];
  private tmpInv = new Matrix();

  constructor(private scene: Scene) {}

  /**
   * Começa um rastro preso à mão `hand`, ativo de `from` a `until` (performance.now()).
   * Devolve o quadro (false = terminou) e a devolução ao pool, ou null se não há lâmina na
   * mão ou o pool está cheio.
   */
  start(hand: TransformNode, from: number, until: number, material: Material): { tick(now: number): boolean; release(): void } | null {
    const tipLocal = this.bladeTip(hand);
    if (!tipLocal) return null;
    const baseLocal = tipLocal.scale(0.3);
    const tr = this.acquire();
    if (!tr) return null;
    tr.mesh.material = material;
    tr.samples.length = 0;
    const tick = (now: number): boolean => {
      if (hand.isDisposed()) return false;
      if (now < from) return true;
      if (now <= until) {
        const wm = hand.getWorldMatrix();
        const sm = tr.samples.length >= SLICES ? tr.samples.pop()! : { t: 0, base: new Vector3(), tip: new Vector3() };
        sm.t = now;
        Vector3.TransformCoordinatesToRef(baseLocal, wm, sm.base);
        Vector3.TransformCoordinatesToRef(tipLocal, wm, sm.tip);
        tr.samples.unshift(sm);
      }
      // O que passou do tempo do rastro sai (depois do fim, a fita encolhe até sumir).
      while (tr.samples.length && now - tr.samples[tr.samples.length - 1].t > SPAN_S * 1000) tr.samples.pop();
      if (tr.samples.length < 2) {
        tr.mesh.setEnabled(false);
        return now <= until;
      }
      this.write(tr);
      tr.mesh.setEnabled(true);
      return true;
    };
    const release = () => {
      tr.mesh.setEnabled(false);
      tr.busy = false;
    };
    return { tick, release };
  }

  /**
   * Ponta da lâmina no espaço da mão: o canto da caixa das malhas da arma mais longe da mão.
   * Funciona com qualquer arma, sem saber para que eixo ela aponta.
   */
  private bladeTip(hand: TransformNode): Vector3 | null {
    const meshes = hand.getChildMeshes(false).filter((m) => m.isEnabled() && m.getTotalVertices() > 0);
    if (!meshes.length) return null;
    const origin = hand.getAbsolutePosition();
    let best: Vector3 | null = null;
    let bestD = 0.15; // menos que isso não é lâmina (escudo, punho)
    for (const m of meshes) {
      m.computeWorldMatrix(true);
      for (const v of m.getBoundingInfo().boundingBox.vectorsWorld) {
        const d = Vector3.Distance(v, origin);
        if (d > bestD) {
          bestD = d;
          best = v;
        }
      }
    }
    if (!best) return null;
    hand.getWorldMatrix().invertToRef(this.tmpInv);
    return Vector3.TransformCoordinates(best, this.tmpInv);
  }

  private acquire(): Trail | null {
    let tr = this.pool.find((t) => !t.busy);
    if (!tr) {
      if (this.pool.length >= POOL) return null;
      tr = this.create();
      this.pool.push(tr);
    }
    tr.busy = true;
    return tr;
  }

  private create(): Trail {
    const n = SLICES * 2;
    const pos = new Float32Array(n * 3);
    const uvs = new Float32Array(n * 2);
    const [u0, u1] = cellU(CELL.trail);
    for (let i = 0; i < SLICES; i++) {
      const u = u0 + ((u1 - u0) * i) / (SLICES - 1);
      uvs.set([u, 0, u, 1], i * 4);
    }
    const idx: number[] = [];
    for (let i = 0; i < SLICES - 1; i++) {
      const a = i * 2;
      idx.push(a, a + 1, a + 2, a + 1, a + 3, a + 2);
    }
    const mesh = new Mesh("fx-weapon-trail", this.scene);
    const vd = new VertexData();
    vd.positions = pos;
    vd.uvs = uvs;
    vd.indices = idx;
    vd.applyToMesh(mesh, true);
    mesh.isPickable = false;
    // As posições já estão no mundo e mudam todo quadro: sem teste de visibilidade.
    mesh.alwaysSelectAsActiveMesh = true;
    mesh.doNotSyncBoundingInfo = true;
    mesh.setEnabled(false);
    return { mesh, pos, samples: [], busy: false };
  }

  /** Escreve as fatias: as amostras mais novas primeiro; as que faltam repetem a última. */
  private write(tr: Trail): void {
    const p = tr.pos;
    const s = tr.samples;
    for (let i = 0; i < SLICES; i++) {
      const sm = s[Math.min(i, s.length - 1)];
      const o = i * 6;
      p[o] = sm.base.x;
      p[o + 1] = sm.base.y;
      p[o + 2] = sm.base.z;
      p[o + 3] = sm.tip.x;
      p[o + 4] = sm.tip.y;
      p[o + 5] = sm.tip.z;
    }
    tr.mesh.updateVerticesData(VertexBuffer.PositionKind, p, false, false);
  }
}
