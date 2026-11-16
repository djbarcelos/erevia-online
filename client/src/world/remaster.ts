/**
 * Remaster (tunnel#93, R0): a fatia vertical da direção de arte nova, atrás de `?remaster=1`.
 * Sem o parâmetro, nada daqui roda e o jogo fica como antes.
 *
 * O que muda com o parâmetro:
 * - o cavaleiro e o esqueleto guerreiro usam os modelos do estilo B (`*_b.glb`), com as próprias
 *   animações e LOD0/1/2 (nós com sufixo _LOD1/_LOD2, trocados pela distância da câmera);
 * - a praça ganha o kit novo (`plaza_b.glb`): fonte, piso em rosa dos ventos e postes com estandarte.
 */
export const REMASTER = new URLSearchParams(location.search).get("remaster") === "1";

/** Modelo do jogo → arquivo remasterizado (sem a extensão). */
export const REMASTER_FILES: Record<string, string> = {
  knight: "knight_b",
  skeleton_warrior: "skeleton_warrior_b",
};

/** Modelo do jogo → máscara de repintura (PNG ao lado do GLB; ver scripts/make-remaster-mask.mjs). */
export const REMASTER_MASKS: Record<string, string> = {
  knight: "knight_b.mask.png",
};

/** Distâncias (m) a partir das quais cada LOD entra (guia de arte, seção 5). */
export const LOD_DISTANCES = [20, 35];

/** O LOD que vale a essa distância ao quadrado. */
export function lodFor(distSq: number): number {
  return distSq > LOD_DISTANCES[1] ** 2 ? 2 : distSq > LOD_DISTANCES[0] ** 2 ? 1 : 0;
}
