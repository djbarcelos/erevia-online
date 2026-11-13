/**
 * Árvore de skills — CONTEÚDO PROVISÓRIO para revisão.
 *
 * Cada classe tem 3 ramos com 4 níveis (liberados nos níveis 1, 5, 10 e 15). As duas
 * skills que já existem no servidor (server/internal/game/skills.go) são raízes de ramo e
 * usam os ícones reais; as outras são ideias novas, com ícones desenhados em ../art.ts.
 * Para aprender um nó é preciso ter o nível do degrau e pelo menos 1 ponto no nó de cima.
 */

import type { ClassId } from "../../api";
import { skillIcon } from "../../ui/icons";
import { skillArt, uniqueIds, type SkillTheme } from "../../ui/art";

export const TIER_LEVELS = [1, 5, 10, 15];

/** Como a skill aparece na simulação do HUD. */
export type SimKind = "melee" | "magic" | "self" | "area";

export interface SkillNode {
  id: string;
  name: string;
  branch: 0 | 1 | 2;
  tier: 0 | 1 | 2 | 3;
  kind: "active" | "passive";
  maxRank: number;
  /** fx real do jogo (ícone e efeito) ou tema de ícone novo. */
  fx?: string;
  theme?: SkillTheme;
  /** Texto do efeito no rank r (1..maxRank). */
  effect(r: number): string;
  mp?: number;
  cd?: number;
  range?: number;
  sim?: SimKind;
  /** Multiplicador de dano na simulação. */
  mult?: number;
}

export interface ClassTree {
  branches: [Branch, Branch, Branch];
  nodes: SkillNode[];
}

interface Branch {
  name: string;
  desc: string;
  color: string;
}

const t = (glyph: SkillTheme["glyph"], c1: string, c2: string, accent?: string): SkillTheme => ({ glyph, bg: [c1, c2], accent });

const pct = (v: number) => `${v.toLocaleString("pt-BR")}%`;

export const TREES: Record<ClassId, ClassTree> = {
  knight: {
    branches: [
      { name: "Guardião", desc: "Defesa e controle da linha de frente.", color: "#6f93d6" },
      { name: "Lâmina", desc: "Esgrima e dano corpo a corpo.", color: "#d6b46e" },
      { name: "Comando", desc: "Auras e gritos que fortalecem o grupo.", color: "#5fc4a6" },
    ],
    nodes: [
      { id: "k_bash", name: "Golpe de Escudo", branch: 0, tier: 0, kind: "active", maxRank: 3, fx: "shield_bash", mp: 10, cd: 8, range: 2.5, sim: "melee", mult: 1.5, effect: (r) => `Causa ${pct(130 + r * 20)} do ataque e atordoa por ${(1 + r * 0.25).toLocaleString("pt-BR")} s.` },
      { id: "k_iron", name: "Pele de Ferro", branch: 0, tier: 1, kind: "passive", maxRank: 5, theme: t("shield", "#7d8fb0", "#1f2840", "#c9d6ff"), effect: (r) => `+${pct(r * 4)} de defesa física.` },
      { id: "k_taunt", name: "Provocar", branch: 0, tier: 2, kind: "active", maxRank: 3, theme: t("shout", "#c0583a", "#3a120a", "#ffc08a"), mp: 18, cd: 15, range: 6, sim: "area", mult: 0.4, effect: (r) => `Inimigos num raio de 6 m passam a atacar você por ${2 + r * 2} s.` },
      { id: "k_wall", name: "Muralha Viva", branch: 0, tier: 3, kind: "passive", maxRank: 1, theme: t("crystal", "#5e8fe0", "#121e3e", "#bfe0ff"), effect: () => `Ao cair abaixo de 30% de HP, ganha um escudo de 15% do HP máximo (1 vez a cada 60 s).` },

      { id: "k_fencing", name: "Esgrima", branch: 1, tier: 0, kind: "passive", maxRank: 5, theme: t("sword", "#b99a52", "#3a2a10", "#fff0c0"), effect: (r) => `+${pct(r * 3)} de ataque físico com espadas.` },
      { id: "k_spin", name: "Corte Giratório", branch: 1, tier: 1, kind: "active", maxRank: 3, theme: t("swirl", "#d0a040", "#3e2a08", "#fff2c4"), mp: 16, cd: 9, range: 3, sim: "area", mult: 1.1, effect: (r) => `Gira a espada e causa ${pct(90 + r * 15)} do ataque em todos ao redor.` },
      { id: "k_charge", name: "Investida", branch: 1, tier: 2, kind: "active", maxRank: 3, theme: t("dash", "#c48a3a", "#3a1e08", "#ffd28a"), mp: 14, cd: 12, range: 12, sim: "melee", mult: 1.2, effect: (r) => `Avança até o alvo, causa ${pct(100 + r * 20)} do ataque e atordoa por 1 s.` },
      { id: "k_judg", name: "Julgamento", branch: 1, tier: 3, kind: "active", maxRank: 1, theme: t("burst", "#f0c050", "#5a2e08", "#fff6d0"), mp: 40, cd: 45, range: 2.5, sim: "melee", mult: 4, effect: () => `Um golpe de cima para baixo que causa 400% do ataque. Não erra.` },

      { id: "k_fort", name: "Fortaleza", branch: 2, tier: 0, kind: "active", maxRank: 3, fx: "fortress", mp: 15, cd: 25, sim: "self", effect: (r) => `+${pct(30 + r * 10)} de defesa por 10 s.` },
      { id: "k_cry", name: "Grito de Guerra", branch: 2, tier: 1, kind: "active", maxRank: 3, theme: t("shout", "#4fae8e", "#0e2a22", "#c0ffe8"), mp: 20, cd: 30, sim: "self", effect: (r) => `Você e o grupo ganham +${pct(5 + r * 5)} de ataque por 20 s.` },
      { id: "k_aura", name: "Aura de Coragem", branch: 2, tier: 2, kind: "passive", maxRank: 3, theme: t("heart", "#4fb0a0", "#0c2a28", "#e0fff6"), effect: (r) => `Aliados a até 10 m regeneram +${r * 2} HP por segundo.` },
      { id: "k_banner", name: "Estandarte", branch: 2, tier: 3, kind: "active", maxRank: 1, theme: t("banner", "#3fa88a", "#0a2a20", "#ffe9a8"), mp: 45, cd: 90, sim: "self", effect: () => `Finca um estandarte por 20 s: aliados por perto ganham +20% de defesa.` },
    ],
  },
  barbarian: {
    branches: [
      { name: "Massacre", desc: "Golpes pesados que encerram lutas.", color: "#e06a42" },
      { name: "Sangue", desc: "Fúria, roubo de vida e teimosia.", color: "#d04050" },
      { name: "Selvagem", desc: "Resistência e instinto animal.", color: "#a6b84a" },
    ],
    nodes: [
      { id: "b_strike", name: "Golpe Brutal", branch: 0, tier: 0, kind: "active", maxRank: 3, fx: "brutal_strike", mp: 10, cd: 6, range: 2.5, sim: "melee", mult: 2.2, effect: (r) => `Causa ${pct(190 + r * 30)} do ataque.` },
      { id: "b_heavy", name: "Machado Pesado", branch: 0, tier: 1, kind: "passive", maxRank: 5, theme: t("axe", "#c8673a", "#3a160a", "#ffd0a0"), effect: (r) => `+${pct(r * 8)} de dano crítico.` },
      { id: "b_quake", name: "Rachar o Chão", branch: 0, tier: 2, kind: "active", maxRank: 3, theme: t("crack", "#b8743a", "#2e1608", "#ffb060"), mp: 22, cd: 14, range: 5, sim: "area", mult: 1.3, effect: (r) => `Bate no chão: ${pct(110 + r * 15)} do ataque ao redor e lentidão de 40% por 3 s.` },
      { id: "b_exec", name: "Execução", branch: 0, tier: 3, kind: "active", maxRank: 1, theme: t("skull", "#d04a2a", "#300a04", "#ffcf80"), mp: 30, cd: 30, range: 2.5, sim: "melee", mult: 3.5, effect: () => `Causa 350% do ataque; o dobro em alvos abaixo de 30% de HP.` },

      { id: "b_fury", name: "Fúria", branch: 1, tier: 0, kind: "active", maxRank: 3, fx: "fury", mp: 15, cd: 25, sim: "self", effect: (r) => `+${pct(20 + r * 10)} de ataque por 10 s.` },
      { id: "b_thirst", name: "Sede de Sangue", branch: 1, tier: 1, kind: "passive", maxRank: 5, theme: t("drop", "#c0303a", "#300810", "#ff9aa0"), effect: (r) => `Recupera ${pct(r * 2)} do dano causado como HP.` },
      { id: "b_frenzy", name: "Frenesi", branch: 1, tier: 2, kind: "active", maxRank: 3, theme: t("bolt", "#d84a3a", "#3a0a08", "#ffe08a"), mp: 20, cd: 25, sim: "self", effect: (r) => `+${pct(15 + r * 10)} de velocidade de ataque por 12 s.` },
      { id: "b_undying", name: "Imortal", branch: 1, tier: 3, kind: "active", maxRank: 1, theme: t("hourglass", "#b02838", "#28040a", "#ffd0a0"), mp: 50, cd: 120, sim: "self", effect: () => `Por 5 s, o HP não cai abaixo de 1.` },

      { id: "b_hide", name: "Couro Grosso", branch: 2, tier: 0, kind: "passive", maxRank: 5, theme: t("paw", "#8f9a44", "#22280a", "#f0ffa0"), effect: (r) => `+${pct(r * 5)} de HP máximo.` },
      { id: "b_roar", name: "Rugido", branch: 2, tier: 1, kind: "active", maxRank: 3, theme: t("shout", "#a0a83a", "#262a08", "#fff4a0"), mp: 18, cd: 20, range: 6, sim: "area", mult: 0.3, effect: (r) => `Inimigos ao redor perdem ${pct(10 + r * 5)} de ataque por 8 s.` },
      { id: "b_stomp", name: "Pisotear", branch: 2, tier: 2, kind: "active", maxRank: 3, theme: t("fist", "#9a8a3a", "#2a2008", "#ffe08a"), mp: 24, cd: 18, range: 4, sim: "area", mult: 1, effect: (r) => `Atordoa inimigos ao redor por ${(1 + r * 0.5).toLocaleString("pt-BR")} s.` },
      { id: "b_bear", name: "Espírito do Urso", branch: 2, tier: 3, kind: "passive", maxRank: 1, theme: t("paw", "#7a8a3a", "#1a2008", "#ffd28a"), effect: () => `Abaixo de 50% de HP, regenera 3% do HP máximo por segundo.` },
    ],
  },
  mage: {
    branches: [
      { name: "Fogo", desc: "Dano alto à distância.", color: "#ec7a2c" },
      { name: "Gelo", desc: "Controle e proteção.", color: "#6cc4f0" },
      { name: "Arcano", desc: "Mana, mobilidade e poder bruto.", color: "#a87cf0" },
    ],
    nodes: [
      { id: "m_fireball", name: "Bola de Fogo", branch: 0, tier: 0, kind: "active", maxRank: 3, fx: "fireball", mp: 15, cd: 3, range: 20, sim: "magic", mult: 2.5, effect: (r) => `Causa ${pct(220 + r * 30)} do ataque mágico no alvo.` },
      { id: "m_ember", name: "Brasa Viva", branch: 0, tier: 1, kind: "passive", maxRank: 5, theme: t("flame", "#e06a20", "#3a1004", "#ffb040"), effect: (r) => `Skills de fogo queimam o alvo: ${pct(r * 6)} do ataque mágico por segundo, por 3 s.` },
      { id: "m_rain", name: "Chuva de Brasas", branch: 0, tier: 2, kind: "active", maxRank: 3, theme: t("meteor", "#e05a20", "#3a0e04", "#ffcc60"), mp: 35, cd: 12, range: 16, sim: "area", mult: 1.6, effect: (r) => `Brasas caem numa área de 5 m: ${pct(140 + r * 20)} do ataque mágico.` },
      { id: "m_meteor", name: "Meteoro", branch: 0, tier: 3, kind: "active", maxRank: 1, theme: t("meteor", "#f07a20", "#4a1004", "#fff0a0"), mp: 80, cd: 60, range: 20, sim: "magic", mult: 5, effect: () => `Invoca um meteoro: 500% do ataque mágico e atordoa por 2 s.` },

      { id: "m_nova", name: "Nova de Gelo", branch: 1, tier: 0, kind: "active", maxRank: 3, fx: "ice_nova", mp: 30, cd: 10, range: 6, sim: "area", mult: 1.2, effect: (r) => `Explode gelo ao redor: ${pct(100 + r * 20)} do ataque mágico e lentidão de 50%.` },
      { id: "m_lance", name: "Lança de Gelo", branch: 1, tier: 1, kind: "active", maxRank: 3, theme: t("spear", "#5ab4e8", "#0a2238", "#e0f6ff"), mp: 18, cd: 5, range: 18, sim: "magic", mult: 1.8, effect: (r) => `${pct(150 + r * 20)} do ataque mágico e lentidão de 30% por 2 s.` },
      { id: "m_armor", name: "Armadura de Gelo", branch: 1, tier: 2, kind: "active", maxRank: 3, theme: t("crystal", "#6ad0f0", "#0c2a3a", "#ffffff"), mp: 30, cd: 40, sim: "self", effect: (r) => `+${pct(20 + r * 10)} de defesa por 20 s; quem bater em você fica lento.` },
      { id: "m_prison", name: "Prisão Glacial", branch: 1, tier: 3, kind: "active", maxRank: 1, theme: t("cage", "#4ab0e8", "#081e34", "#e0ffff"), mp: 40, cd: 30, range: 15, sim: "magic", mult: 0.5, effect: () => `Prende o alvo no gelo por 4 s.` },

      { id: "m_mind", name: "Mente Clara", branch: 2, tier: 0, kind: "passive", maxRank: 5, theme: t("book", "#8c6ad8", "#1e1240", "#e8d8ff"), effect: (r) => `+${pct(r * 10)} de regeneração de MP.` },
      { id: "m_shield", name: "Escudo Arcano", branch: 2, tier: 1, kind: "active", maxRank: 3, theme: t("shield", "#9a6ae8", "#1e0e40", "#f0e0ff"), mp: 25, cd: 30, sim: "self", effect: (r) => `Absorve dano usando MP no lugar do HP por ${6 + r * 2} s.` },
      { id: "m_blink", name: "Piscar", branch: 2, tier: 2, kind: "active", maxRank: 3, theme: t("blink", "#a07ae8", "#20104a", "#e8dcff"), mp: 20, cd: 18 , sim: "self", effect: (r) => `Teleporta ${6 + r * 2} m para a frente.` },
      { id: "m_over", name: "Sobrecarga", branch: 2, tier: 3, kind: "passive", maxRank: 1, theme: t("star", "#b88af8", "#2a1050", "#fff0a0"), effect: () => `+15% de ataque mágico; skills custam 10% a mais de MP.` },
    ],
  },
  rogue: {
    branches: [
      { name: "Assassinato", desc: "Críticos e golpes letais.", color: "#d6b46e" },
      { name: "Sombras", desc: "Mobilidade e esquiva.", color: "#8c7ae0" },
      { name: "Venenos", desc: "Dano contínuo e controle.", color: "#6ad06a" },
    ],
    nodes: [
      { id: "r_back", name: "Golpe Furtivo", branch: 0, tier: 0, kind: "active", maxRank: 3, fx: "backstab", mp: 10, cd: 6, range: 2.5, sim: "melee", mult: 2, effect: (r) => `Causa ${pct(170 + r * 30)} do ataque com +50% de chance de crítico.` },
      { id: "r_weak", name: "Ponto Fraco", branch: 0, tier: 1, kind: "passive", maxRank: 5, theme: t("eye", "#c09a40", "#2a1e08", "#ffe08a"), effect: (r) => `+${pct(r * 2)} de chance de crítico.` },
      { id: "r_twin", name: "Lâminas Gêmeas", branch: 0, tier: 2, kind: "active", maxRank: 3, theme: t("daggers", "#b89048", "#2a1a08", "#ffe6a0"), mp: 16, cd: 8, range: 2.5, sim: "melee", mult: 2.4, effect: (r) => `Dois golpes rápidos de ${pct(100 + r * 15)} do ataque cada.` },
      { id: "r_mark", name: "Marca da Morte", branch: 0, tier: 3, kind: "active", maxRank: 1, theme: t("mark", "#c02a2a", "#2a0606", "#ffd28a"), mp: 30, cd: 40, range: 12, sim: "magic", mult: 0.2, effect: () => `Marca o alvo: ele recebe +25% de dano de todas as fontes por 10 s.` },

      { id: "r_shadow", name: "Passo Sombrio", branch: 1, tier: 0, kind: "active", maxRank: 3, fx: "shadow_step", mp: 12, cd: 20, sim: "self", effect: (r) => `+${pct(30 + r * 10)} de velocidade por 6 s.` },
      { id: "r_dodge", name: "Esquiva", branch: 1, tier: 1, kind: "passive", maxRank: 5, theme: t("dash", "#7a6ad0", "#1a1240", "#e0d8ff"), effect: (r) => `+${r * 6} de esquiva.` },
      { id: "r_smoke", name: "Bomba de Fumaça", branch: 1, tier: 2, kind: "active", maxRank: 3, theme: t("smoke", "#6a6a90", "#16162a", "#c0a0ff"), mp: 22, cd: 25, range: 4, sim: "area", mult: 0, effect: (r) => `Inimigos na fumaça erram ${pct(20 + r * 10)} dos ataques por 6 s.` },
      { id: "r_vanish", name: "Sumir", branch: 1, tier: 3, kind: "active", maxRank: 1, theme: t("mask", "#5a4ab0", "#100a30", "#e0d0ff"), mp: 35, cd: 60, sim: "self", effect: () => `Fica invisível por 5 s; o próximo golpe é crítico garantido.` },

      { id: "r_poison", name: "Lâmina Envenenada", branch: 2, tier: 0, kind: "passive", maxRank: 5, theme: t("drop", "#4ab04a", "#0a2a0a", "#d0ffa0"), effect: (r) => `${pct(r * 5)} de chance de envenenar: 4% do ataque por segundo, por 5 s.` },
      { id: "r_flask", name: "Frasco Tóxico", branch: 2, tier: 1, kind: "active", maxRank: 3, theme: t("flask", "#5ac05a", "#0e2a0e", "#b0ff60"), mp: 18, cd: 10, range: 14, sim: "area", mult: 1, effect: (r) => `Arremessa veneno numa área de 4 m: ${pct(80 + r * 20)} do ataque em 4 s.` },
      { id: "r_para", name: "Paralisia", branch: 2, tier: 2, kind: "active", maxRank: 3, theme: t("bolt", "#6ab03a", "#14280a", "#f0ff80"), mp: 20, cd: 20, range: 2.5, sim: "melee", mult: 0.8, effect: (r) => `Atordoa o alvo envenenado por ${(1.5 + r * 0.5).toLocaleString("pt-BR")} s.` },
      { id: "r_plague", name: "Praga", branch: 2, tier: 3, kind: "active", maxRank: 1, theme: t("skull", "#3a9a3a", "#081e08", "#c0ff80"), mp: 40, cd: 45, range: 12, sim: "area", mult: 2, effect: () => `Um veneno que se espalha para os inimigos próximos quando o alvo morre.` },
    ],
  },
};

/** SVG do ícone de um nó da árvore. */
export function nodeIcon(n: SkillNode): string {
  return n.fx ? uniqueIds(skillIcon(n.fx)) : skillArt(n.theme!);
}

/** Ranks iniciais do personagem de exemplo (nível 14). */
export function initialRanks(cls: ClassId): Record<string, number> {
  const tree = TREES[cls];
  const ranks: Record<string, number> = {};
  for (const n of tree.nodes) {
    if (n.tier === 0) ranks[n.id] = n.kind === "active" ? 2 : 3;
    else if (n.tier === 1 && n.branch !== 2) ranks[n.id] = n.kind === "active" ? 1 : 2;
    else if (n.tier === 2 && n.branch === 1) ranks[n.id] = 1;
  }
  return ranks;
}

export function skillPointsTotal(level: number): number {
  return level + 2; // 1 por nível + 3 de início (o nível 1 já dá as duas raízes)
}
