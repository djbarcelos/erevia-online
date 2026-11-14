import type { Ray, TransformNode } from "@babylonjs/core";

/**
 * Áreas de clique de entidades e itens no chão: cilindros em pé testados contra o raio do
 * cursor em código. Antes cada uma era uma malha invisível na cena, e cada malha custa na
 * escolha das malhas ativas a cada quadro, mesmo sem ser desenhada.
 *
 * O cilindro fica nos pés do nó (posição dele, sem pai) e sobe `height`. Vale o que estiver
 * mais perto da câmera ao longo do raio, como no picking de malha.
 */
interface Area {
  id: number;
  node: TransformNode;
  radius: number;
  height: number;
}

export interface PickHit {
  entity?: number;
  loot?: number;
}

export class PickAreas {
  private entities = new Map<number, Area>();
  private loot = new Map<number, Area>();
  /** Resultado reaproveitado (o hover testa a cada 80 ms). */
  private hit: PickHit = {};

  addEntity(id: number, node: TransformNode, radius: number, height: number): void {
    this.entities.set(id, { id, node, radius, height });
  }

  removeEntity(id: number): void {
    this.entities.delete(id);
  }

  addLoot(id: number, node: TransformNode, radius: number, height: number): void {
    this.loot.set(id, { id, node, radius, height });
  }

  removeLoot(id: number): void {
    this.loot.delete(id);
  }

  /** O que o raio acerta primeiro (null = nada). O objeto devolvido é reaproveitado. */
  pick(ray: Ray): PickHit | null {
    const o = ray.origin;
    const d = ray.direction;
    let best = ray.length;
    let entity = -1;
    let loot = -1;
    for (const a of this.entities.values()) {
      const p = a.node.position;
      const t = rayCylinder(o.x, o.y, o.z, d.x, d.y, d.z, p.x, p.y, p.z, a.radius, a.height);
      if (t < best) {
        best = t;
        entity = a.id;
      }
    }
    for (const a of this.loot.values()) {
      const p = a.node.position;
      const t = rayCylinder(o.x, o.y, o.z, d.x, d.y, d.z, p.x, p.y, p.z, a.radius, a.height);
      if (t < best) {
        best = t;
        loot = a.id;
        entity = -1;
      }
    }
    if (entity < 0 && loot < 0) return null;
    this.hit.entity = entity >= 0 ? entity : undefined;
    this.hit.loot = entity < 0 ? loot : undefined;
    return this.hit;
  }
}

/**
 * Distância ao longo do raio (origem o, direção d normalizada) até o primeiro ponto de um
 * cilindro em pé com a base em (cx, cy, cz), raio r e altura h, contando as tampas.
 * Infinity se não acerta (ou se só acerta atrás da origem).
 */
export function rayCylinder(
  ox: number, oy: number, oz: number,
  dx: number, dy: number, dz: number,
  cx: number, cy: number, cz: number,
  r: number, h: number,
): number {
  const px = ox - cx;
  const pz = oz - cz;
  const r2 = r * r;
  let best = Infinity;
  // Lateral: (px + t·dx)² + (pz + t·dz)² = r², com a altura dentro do cilindro.
  const a = dx * dx + dz * dz;
  if (a > 1e-12) {
    const b = px * dx + pz * dz;
    const c = px * px + pz * pz - r2;
    const disc = b * b - a * c;
    if (disc >= 0) {
      const s = Math.sqrt(disc);
      const near = (-b - s) / a;
      const far = (-b + s) / a;
      if (near >= 0 && inside(oy + near * dy - cy, h)) best = near;
      else if (far >= 0 && inside(oy + far * dy - cy, h)) best = far;
    }
  }
  // Tampas (de cima, a câmera quase sempre acerta a de cima).
  if (Math.abs(dy) > 1e-12) {
    const top = (cy + h - oy) / dy;
    if (top >= 0 && top < best && onCap(px + top * dx, pz + top * dz, r2)) best = top;
    const bottom = (cy - oy) / dy;
    if (bottom >= 0 && bottom < best && onCap(px + bottom * dx, pz + bottom * dz, r2)) best = bottom;
  }
  return best;
}

function inside(y: number, h: number): boolean {
  return y >= 0 && y <= h;
}

function onCap(x: number, z: number, r2: number): boolean {
  return x * x + z * z <= r2;
}
