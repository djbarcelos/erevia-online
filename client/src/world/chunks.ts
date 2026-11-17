/**
 * Recortes do cenário do remaster (docs/desempenho-remaster.md). DESLIGADO por padrão: `?remaster=1&chunks=1` liga.
 *
 * Medido numa GPU NVIDIA e numa Intel Arc (com a CPU 4x mais lenta, como um celular médio), cortar os triângulos pela
 * metade NÃO ganhou FPS: o recorte soma draw calls (de 106 para 134 com as células de 64 m) e o quadro é limitado pela
 * CPU, não pelos triângulos. Fica no código, ligado por parâmetro, para medir num celular de verdade (tunnel#64), onde a
 * GPU pode pesar mais que aqui.
 *
 * A natureza do remaster é desenhada com thin instances: todas as árvores de um tipo, do mapa
 * inteiro, numa malha só. O Babylon só corta uma malha pelo campo de visão como um todo, e a
 * caixa dela cobre o mapa, então a GPU recebia as ~1,0 milhão de triângulos do mapa a cada
 * passe, e o mapa de sombras (que só vê 70 m) pagava de novo o preço cheio.
 *
 * Aqui cada tipo vira vários recortes por célula do mapa (CHUNK m de lado), cada um com a
 * própria caixa: o corte do campo de visão passa a valer por recorte, e o mapa de sombras só
 * desenha os recortes que cabem no volume da luz (corte sem perda: o que fica de fora o mapa
 * de sombras já descartava).
 */
import type { DirectionalLight, Mesh, ShadowGenerator } from "@babylonjs/core";

const params = typeof location === "undefined" ? new URLSearchParams() : new URLSearchParams(location.search);

/** Ligado por `?chunks=1` (e só no remaster, ver environment.ts). */
export const CHUNKS_ON = params.get("chunks") === "1";

/** Lado (m) da célula de um recorte. Maior = menos draw calls, menos corte. (?chunk=N muda, para medir.) */
export const CHUNK = Number(params.get("chunk")) || 64;

/**
 * Só vira recortes o tipo que pesa: triângulos de todas as instâncias somados (?chunkmin=N muda, para medir). Cada recorte
 * visível custa um draw call; para o mato (6 triângulos por tufo) isso não se paga.
 */
export const CHUNK_MIN_TRIS = params.get("chunkmin") !== null ? Number(params.get("chunkmin")) : 100_000;

/** Chave da célula de uma posição. */
export function chunkKey(x: number, z: number): string {
  return `${Math.floor(x / CHUNK)},${Math.floor(z / CHUNK)}`;
}

/** Divide uma lista de itens por célula (a ordem dentro de cada célula se mantém). */
export function splitByChunk<T extends { x: number; z: number }>(list: T[]): Map<string, T[]> {
  const out = new Map<string, T[]>();
  for (const o of list) {
    const k = chunkKey(o.x, o.z);
    let g = out.get(k);
    if (!g) out.set(k, (g = []));
    g.push(o);
  }
  return out;
}

/**
 * Mantém na lista do mapa de sombras só os recortes que a luz enxerga: o volume da luz
 * direcional é uma caixa de `shadowFrustumSize` de lado que vai até `shadowMaxZ` de
 * profundidade. Quem está antes do plano de perto continua (o ShadowGenerator prende a
 * profundidade e eles ainda projetam sombra); quem está além do fundo ou para os lados não
 * aparecia no mapa de qualquer jeito.
 */
export class ShadowCuller {
  private entries: { mesh: Mesh; inList: boolean }[] = [];

  private shadows: ShadowGenerator;
  private light: DirectionalLight;

  constructor(shadows: ShadowGenerator, light: DirectionalLight) {
    this.shadows = shadows;
    this.light = light;
  }

  /** Recorte que projeta sombra. */
  add(mesh: Mesh): void {
    this.shadows.addShadowCaster(mesh, false);
    this.entries.push({ mesh, inList: true });
  }

  get count(): number {
    return this.entries.length;
  }

  /** Reavalia (a cada quadro, depois que a luz seguiu o jogador). */
  update(): void {
    if (!this.entries.length) return;
    const list = this.shadows.getShadowMap()?.renderList;
    if (!list) return;
    const pos = this.light.getAbsolutePosition();
    const d = this.light.getShadowDirection();
    const len = Math.hypot(d.x, d.y, d.z) || 1;
    const dx = d.x / len;
    const dy = d.y / len;
    const dz = d.z / len;
    // Eixos da caixa da luz: direita = cima x direção (cai para o X se a luz for vertical).
    let rx = dz;
    let rz = -dx;
    const rl = Math.hypot(rx, rz);
    if (rl < 1e-4) {
      rx = 1;
      rz = 0;
    } else {
      rx /= rl;
      rz /= rl;
    }
    // cima2 = direção x direita.
    const ux = dy * rz;
    const uy = dz * rx - dx * rz;
    const uz = -dy * rx;
    const half = (this.light.shadowFrustumSize || 0) / 2 + 2;
    const far = this.light.shadowMaxZ;
    for (const e of this.entries) {
      const bi = e.mesh.getBoundingInfo().boundingSphere;
      const c = bi.centerWorld;
      const r = bi.radiusWorld;
      const vx = c.x - pos.x;
      const vy = c.y - pos.y;
      const vz = c.z - pos.z;
      const u = vx * rx + vz * rz;
      const w = vx * ux + vy * uy + vz * uz;
      const depth = vx * dx + vy * dy + vz * dz;
      const visible = Math.abs(u) <= half + r && Math.abs(w) <= half + r && depth <= far + r;
      if (visible === e.inList) continue;
      e.inList = visible;
      if (visible) list.push(e.mesh);
      else {
        const i = list.indexOf(e.mesh);
        if (i >= 0) list.splice(i, 1);
      }
    }
  }
}
