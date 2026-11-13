/**
 * Aparência dos personagens: o que cada classe leva nas mãos por padrão e as opções
 * de personalização (usadas na criação e no protótipo).
 */

import type { ClassId } from "../api";
import type { Appearance } from "./preview3d";

export const DEFAULT_LOOK: Record<ClassId, Appearance> = {
  knight: { right: "sword_1handed", left: "shield_badge_color", emblem: "#d6b46e" },
  barbarian: { right: "axe_1handed", left: "shield_round_barbarian", emblem: "#e2674a" },
  mage: { right: "staff", left: null, emblem: "#8c6fd6" },
  rogue: { right: "dagger", left: "dagger", emblem: "#5fae7d" },
};

/** Cores do emblema oferecidas na criação. */
export const EMBLEMS = ["#d6b46e", "#e2674a", "#d04050", "#8c6fd6", "#4f9dff", "#58c7d6", "#5fae7d", "#e8e4dc"];

/** Papel, dificuldade e perfil de cada classe (só para a tela de criação). */
export const CLASS_EXTRA: Record<ClassId, { role: string; diff: number; bars: [number, number, number, number] }> = {
  knight: { role: "Tanque · corpo a corpo", diff: 1, bars: [55, 95, 15, 45] },
  barbarian: { role: "Dano · corpo a corpo", diff: 2, bars: [90, 60, 10, 55] },
  mage: { role: "Dano · à distância", diff: 3, bars: [20, 30, 100, 45] },
  rogue: { role: "Dano · crítico e mobilidade", diff: 2, bars: [75, 40, 15, 95] },
};
export const CLASS_BAR_NAMES = ["Ataque", "Defesa", "Magia", "Mobilidade"];
