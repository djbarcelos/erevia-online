import type { ClassId } from "./api";

export interface ClassInfo {
  id: ClassId;
  label: string;
  desc: string;
  /** Cor usada no ponto da lista e na cápsula reserva (antes do modelo carregar). */
  color: string;
}

export const CLASSES: ClassInfo[] = [
  { id: "knight", label: "Cavaleiro", desc: "Armadura pesada e escudo. Aguenta a linha de frente.", color: "#8fa7c8" },
  { id: "barbarian", label: "Bárbaro", desc: "Força bruta e machado. Dano alto de perto.", color: "#c8734f" },
  { id: "mage", label: "Mago", desc: "Magia à distância. Frágil, mas devastador.", color: "#8c6fd6" },
  { id: "rogue", label: "Ladino", desc: "Rápido e furtivo. Golpes precisos.", color: "#5fae7d" },
];

export function classInfo(id: string): ClassInfo {
  return CLASSES.find((c) => c.id === id) ?? CLASSES[0];
}
