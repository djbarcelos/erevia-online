/**
 * Variantes de monstro (tunnel#149): as elites do 30 ao 40 reaproveitam os modelos de hoje
 * (esqueletos KayKit e golem) com outra escala e outra cor (Entity.tint).
 *
 * Custo: nada por quadro. As malhas são tingidas uma vez, quando o modelo termina de
 * carregar; um timer de 250 ms só existe enquanto há variante esperando e para sozinho. O
 * material tingido é uma cópia, guardada por material e cor: todas as instâncias da mesma
 * variante dividem a mesma cópia (os outros monstros do mesmo modelo não mudam).
 */

import { Color3, PBRMaterial, StandardMaterial, type AbstractMesh, type Material } from "@babylonjs/core";
import type { EntityManager } from "./entities";

/** Intervalo da conferência enquanto há modelo carregando (ms). */
const CHECK_MS = 250;
/** Depois das primeiras malhas, mais algumas conferências pegam as que chegam por último (armas). */
const EXTRA_CHECKS = 6;
/** Quanto da cor da variante entra no material (1 = multiplica pela cor inteira). */
const STRENGTH = 0.75;

interface Pending {
  color: Color3;
  key: string;
  /** Conferências que faltam depois de achar as primeiras malhas (-1 = ainda nenhuma). */
  left: number;
}

export class MonsterVariants {
  private pending = new Map<number, Pending>();
  private cache = new Map<string, Material>();
  private done = new WeakSet<AbstractMesh>();
  private timer = 0;

  constructor(private entities: EntityManager) {}

  /** Uma entidade entrou (Welcome ou Spawn): se for variante, tinge quando o modelo chegar. */
  add(id: number, tint: string): void {
    if (!/^#[0-9a-f]{6}$/i.test(tint)) return;
    const white = Color3.White();
    const color = Color3.Lerp(white, Color3.FromHexString(tint), STRENGTH);
    this.pending.set(id, { color, key: tint.toLowerCase(), left: -1 });
    if (!this.timer) this.timer = window.setInterval(() => this.check(), CHECK_MS);
  }

  dispose(): void {
    window.clearInterval(this.timer);
    this.timer = 0;
    this.pending.clear();
  }

  private check(): void {
    for (const [id, p] of this.pending) {
      if (!this.entities.info(id)) {
        this.pending.delete(id); // saiu da área de interesse antes de carregar
        continue;
      }
      const meshes = this.entities.meshesOf(id);
      if (meshes.length === 0) continue;
      for (const m of meshes) {
        if (this.done.has(m)) continue;
        this.done.add(m);
        this.tint(m, p);
      }
      if (p.left < 0) p.left = EXTRA_CHECKS;
      if (--p.left <= 0) this.pending.delete(id);
    }
    if (this.pending.size === 0) {
      window.clearInterval(this.timer);
      this.timer = 0;
    }
  }

  /** Troca o material da malha pela cópia tingida (os olhos que brilham ficam como estão). */
  private tint(m: AbstractMesh, p: Pending): void {
    const src = m.material;
    if (!src || src.name.toLowerCase().startsWith("glow")) return;
    const key = `${src.uniqueId}:${p.key}`;
    let t = this.cache.get(key);
    if (!t) {
      if (src instanceof PBRMaterial) {
        const c = src.clone(`${src.name}-${p.key}`);
        c.albedoColor = src.albedoColor.multiply(p.color);
        t = c;
      } else if (src instanceof StandardMaterial) {
        const c = src.clone(`${src.name}-${p.key}`);
        c.diffuseColor = src.diffuseColor.multiply(p.color);
        t = c;
      } else return;
      this.cache.set(key, t);
    }
    m.material = t;
  }
}
