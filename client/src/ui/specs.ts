/**
 * Especializações (segunda classe, nível 20): o catálogo vem no Welcome (as 8, de todas as
 * classes) e fica aqui para o título sob o nome, o quadro do alvo, a janela do personagem,
 * a árvore de skills e a janela da Mestra dos Caminhos. Só leitura depois do Welcome.
 */

import type { SpecInfo } from "../gen/erevia/v1/game_pb";

let specs: SpecInfo[] = [];
let byId = new Map<string, SpecInfo>();

export function setSpecs(list: SpecInfo[]): void {
  specs = list;
  byId = new Map(list.map((s) => [s.id, s]));
}

/** A especialização pelo id ("" ou desconhecida = undefined). */
export function specInfo(id: string | undefined): SpecInfo | undefined {
  return id ? byId.get(id) : undefined;
}

/** Os dois caminhos da classe, na ordem da janela da Mestra. */
export function classSpecs(cls: string): SpecInfo[] {
  return specs.filter((s) => s.class === cls);
}
