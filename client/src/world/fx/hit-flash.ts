import { BaseTexture, Color3, Mesh, PBRMaterial, StandardMaterial, type AbstractMesh, type Material } from "@babylonjs/core";

/**
 * Flash no alvo atingido: por um instante as malhas do alvo trocam o material por uma cópia
 * com emissivo claro. Sem partícula, sem malha e sem passe a mais: só a troca de material.
 *
 * Os materiais dos modelos são compartilhados (todos os esqueletos usam o mesmo), então
 * mexer no emissivo do original acenderia todos. A cópia é feita uma vez por material, na
 * primeira vez que algo com ele é atingido, e compilada antes do primeiro uso: esse
 * primeiro golpe sai sem flash, para nunca esconder a malha enquanto o shader compila.
 */

/** Branco quente (Comum do guia, puxado para cima) e o dourado do crítico. */
const FLASH = new Color3(0.62, 0.58, 0.5);
const FLASH_CRIT = new Color3(0.85, 0.62, 0.3);

interface Entry {
  normal: Material;
  crit: Material;
  ready: number;
}

interface Active {
  orig: Material;
  flash: Material;
  until: number;
}

/** Malhas piscando ao mesmo tempo, no máximo (o resto do grupo sai sem flash). */
const MAX_ACTIVE = 48;

export class HitFlash {
  private copies = new Map<number, Entry | null>();
  private active = new Map<AbstractMesh, Active>();

  flash(meshes: AbstractMesh[], ms: number, crit: boolean): void {
    const until = performance.now() + ms;
    for (const m of meshes) {
      // Instâncias (armas) usam o material da malha de origem: trocar ali acenderia todas.
      if (!(m instanceof Mesh) || m.isDisposed() || !m.isEnabled() || m.visibility <= 0.01) continue;
      const cur = this.active.get(m);
      if (cur) {
        cur.until = Math.max(cur.until, until);
        continue;
      }
      if (this.active.size >= MAX_ACTIVE) return;
      const orig = m.material;
      if (!orig) continue;
      const e = this.copiesOf(orig, m);
      if (!e || e.ready < 2) continue;
      const flash = crit ? e.crit : e.normal;
      m.material = flash;
      this.active.set(m, { orig, flash, until });
    }
  }

  /** Devolve o material de quem já piscou o bastante (a cada quadro). */
  update(now: number): void {
    if (!this.active.size) return;
    for (const [m, a] of this.active) {
      if (now < a.until && !m.isDisposed()) continue;
      this.active.delete(m);
      // Se alguém trocou o material no meio (arma nova, repintura), fica o novo.
      if (!m.isDisposed() && m.material === a.flash) m.material = a.orig;
    }
  }

  private copiesOf(base: Material, mesh: AbstractMesh): Entry | null {
    const id = base.uniqueId;
    const known = this.copies.get(id);
    if (known !== undefined) return known;
    if (!(base instanceof StandardMaterial || base instanceof PBRMaterial)) {
      this.copies.set(id, null);
      return null;
    }
    try {
      const make = (color: Color3, tag: string): Material => {
        const mat = base.clone(`${base.name}-${tag}`) as StandardMaterial | PBRMaterial;
        shareTextures(base, mat);
        mat.emissiveColor = color.clone();
        return mat;
      };
      const e: Entry = { normal: make(FLASH, "flash"), crit: make(FLASH_CRIT, "flash-crit"), ready: 0 };
      e.normal.forceCompilation(mesh, () => e.ready++);
      e.crit.forceCompilation(mesh, () => e.ready++);
      this.copies.set(id, e);
      // Material repintado descartado (ninguém mais usa aquele visual): as cópias vão junto.
      base.onDisposeObservable.addOnce(() => {
        for (const [m, a] of this.active) {
          if (a.flash === e.normal || a.flash === e.crit) this.active.delete(m);
        }
        this.copies.delete(id);
        e.normal.dispose(false, false);
        e.crit.dispose(false, false);
      });
      return e;
    } catch {
      // Material que não aceita cópia: esse alvo só não pisca.
      this.copies.set(id, null);
      return null;
    }
  }
}

/**
 * O clone do Babylon copia as texturas (uma textura repintada em RawTexture viraria uma
 * textura vazia). A cópia usa as mesmas do original.
 */
function shareTextures(base: Material, copy: Material): void {
  const b = base as unknown as Record<string, unknown>;
  const c = copy as unknown as Record<string, unknown>;
  for (const k of Object.keys(c)) {
    const v = c[k];
    if (v instanceof BaseTexture && b[k] !== v) {
      c[k] = b[k];
      v.dispose();
    }
  }
  copy.markAsDirty(1); // Material.TextureDirtyFlag
}
