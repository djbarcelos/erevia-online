/**
 * Ícones das skills da árvore (as que não têm ícone desenhado em icons.ts): glifo e cores.
 * A chave é o "fx" da skill no servidor (server/internal/game/skills.go).
 */

import type { SkillTheme } from "./art";

export const SKILL_THEMES: Record<string, SkillTheme> = {
  k_iron: { glyph: "shield", bg: ["#7d8fb0", "#1f2840"], accent: "#c9d6ff" },
  k_taunt: { glyph: "shout", bg: ["#c0583a", "#3a120a"], accent: "#ffc08a" },
  k_wall: { glyph: "crystal", bg: ["#5e8fe0", "#121e3e"], accent: "#bfe0ff" },
  k_fencing: { glyph: "sword", bg: ["#b99a52", "#3a2a10"], accent: "#fff0c0" },
  k_spin: { glyph: "swirl", bg: ["#d0a040", "#3e2a08"], accent: "#fff2c4" },
  k_charge: { glyph: "dash", bg: ["#c48a3a", "#3a1e08"], accent: "#ffd28a" },
  k_judg: { glyph: "burst", bg: ["#f0c050", "#5a2e08"], accent: "#fff6d0" },
  k_cry: { glyph: "shout", bg: ["#4fae8e", "#0e2a22"], accent: "#c0ffe8" },
  k_aura: { glyph: "heart", bg: ["#4fb0a0", "#0c2a28"], accent: "#e0fff6" },
  k_banner: { glyph: "banner", bg: ["#3fa88a", "#0a2a20"], accent: "#ffe9a8" },
  b_heavy: { glyph: "axe", bg: ["#c8673a", "#3a160a"], accent: "#ffd0a0" },
  b_quake: { glyph: "crack", bg: ["#b8743a", "#2e1608"], accent: "#ffb060" },
  b_exec: { glyph: "skull", bg: ["#d04a2a", "#300a04"], accent: "#ffcf80" },
  b_thirst: { glyph: "drop", bg: ["#c0303a", "#300810"], accent: "#ff9aa0" },
  b_frenzy: { glyph: "bolt", bg: ["#d84a3a", "#3a0a08"], accent: "#ffe08a" },
  b_undying: { glyph: "hourglass", bg: ["#b02838", "#28040a"], accent: "#ffd0a0" },
  b_hide: { glyph: "paw", bg: ["#8f9a44", "#22280a"], accent: "#f0ffa0" },
  b_roar: { glyph: "shout", bg: ["#a0a83a", "#262a08"], accent: "#fff4a0" },
  b_stomp: { glyph: "fist", bg: ["#9a8a3a", "#2a2008"], accent: "#ffe08a" },
  b_bear: { glyph: "paw", bg: ["#7a8a3a", "#1a2008"], accent: "#ffd28a" },
  m_ember: { glyph: "flame", bg: ["#e06a20", "#3a1004"], accent: "#ffb040" },
  m_rain: { glyph: "meteor", bg: ["#e05a20", "#3a0e04"], accent: "#ffcc60" },
  m_meteor: { glyph: "meteor", bg: ["#f07a20", "#4a1004"], accent: "#fff0a0" },
  m_lance: { glyph: "spear", bg: ["#5ab4e8", "#0a2238"], accent: "#e0f6ff" },
  m_armor: { glyph: "crystal", bg: ["#6ad0f0", "#0c2a3a"], accent: "#ffffff" },
  m_prison: { glyph: "cage", bg: ["#4ab0e8", "#081e34"], accent: "#e0ffff" },
  m_mind: { glyph: "book", bg: ["#8c6ad8", "#1e1240"], accent: "#e8d8ff" },
  m_shield: { glyph: "shield", bg: ["#9a6ae8", "#1e0e40"], accent: "#f0e0ff" },
  m_blink: { glyph: "blink", bg: ["#a07ae8", "#20104a"], accent: "#e8dcff" },
  m_over: { glyph: "star", bg: ["#b88af8", "#2a1050"], accent: "#fff0a0" },
  r_weak: { glyph: "eye", bg: ["#c09a40", "#2a1e08"], accent: "#ffe08a" },
  r_twin: { glyph: "daggers", bg: ["#b89048", "#2a1a08"], accent: "#ffe6a0" },
  r_mark: { glyph: "mark", bg: ["#c02a2a", "#2a0606"], accent: "#ffd28a" },
  r_dodge: { glyph: "dash", bg: ["#7a6ad0", "#1a1240"], accent: "#e0d8ff" },
  r_smoke: { glyph: "smoke", bg: ["#6a6a90", "#16162a"], accent: "#c0a0ff" },
  r_vanish: { glyph: "mask", bg: ["#5a4ab0", "#100a30"], accent: "#e0d0ff" },
  r_poison: { glyph: "drop", bg: ["#4ab04a", "#0a2a0a"], accent: "#d0ffa0" },
  r_flask: { glyph: "flask", bg: ["#5ac05a", "#0e2a0e"], accent: "#b0ff60" },
  r_para: { glyph: "bolt", bg: ["#6ab03a", "#14280a"], accent: "#f0ff80" },
  r_plague: { glyph: "skull", bg: ["#3a9a3a", "#081e08"], accent: "#c0ff80" },
  // 5º degrau (nível 20): glifos que já existem, com o fundo mais escuro e o brilho dourado.
  k_bulwark: { glyph: "shield", bg: ["#4a6fc0", "#0c1430"], accent: "#ffe08a" },
  k_horizon: { glyph: "sword", bg: ["#e0b450", "#3a2004"], accent: "#ffffff" },
  k_aegis: { glyph: "crown", bg: ["#3fb08f", "#06241c"], accent: "#ffe9a8" },
  b_seismic: { glyph: "crack", bg: ["#d8602a", "#3a0c04"], accent: "#ffe08a" },
  b_whirl: { glyph: "swirl", bg: ["#c02838", "#2a0408"], accent: "#ffb0a0" },
  b_stone: { glyph: "fist", bg: ["#8a8a7a", "#1e1e16"], accent: "#e8f0a0" },
  m_inferno: { glyph: "flame", bg: ["#f05018", "#480a02"], accent: "#fff0a0" },
  m_zero: { glyph: "snow", bg: ["#8ad8ff", "#06203a"], accent: "#ffffff" },
  m_arcane: { glyph: "burst", bg: ["#9a5af0", "#1a0840"], accent: "#f0e0ff" },
  r_fatal: { glyph: "daggers", bg: ["#d03030", "#2a0404"], accent: "#ffe08a" },
  r_shadow: { glyph: "blink", bg: ["#5a3ab8", "#0a0628"], accent: "#ffd28a" },
  r_miasma: { glyph: "smoke", bg: ["#3aa83a", "#061e06"], accent: "#d0ff80" },
  // Especializações (níveis 20, 24 e 28): glifos que já existem, no tom do caminho.
  kb_defy: { glyph: "shout", bg: ["#5f86d8", "#0e1a38"], accent: "#ffe08a" },
  kb_vigor: { glyph: "heart", bg: ["#6f93d6", "#101c3a"], accent: "#d8e8ff" },
  kb_last: { glyph: "shield", bg: ["#8fb4ff", "#0a1430"], accent: "#ffffff" },
  kh_dawn: { glyph: "burst", bg: ["#e8b840", "#3a2404"], accent: "#fff6d0" },
  kh_bless: { glyph: "star", bg: ["#f0c860", "#3a2a08"], accent: "#ffffff" },
  kh_sun: { glyph: "crown", bg: ["#ffd86a", "#4a2e04"], accent: "#fffbe8" },
  bb_red: { glyph: "axe", bg: ["#d83838", "#300606"], accent: "#ffc0a0" },
  bb_blind: { glyph: "eye", bg: ["#c03040", "#2a0408"], accent: "#ffe08a" },
  bb_carnage: { glyph: "swirl", bg: ["#e85050", "#3a0606"], accent: "#ffd0a0" },
  bw_cry: { glyph: "shout", bg: ["#d89a3a", "#3a2208"], accent: "#fff0b0" },
  bw_charge: { glyph: "dash", bg: ["#c8902a", "#2e1a04"], accent: "#ffe08a" },
  bw_banner: { glyph: "banner", bg: ["#e0a848", "#3a2404"], accent: "#ffffff" },
  me_lance: { glyph: "spear", bg: ["#f07a2c", "#3a1004"], accent: "#fff0a0" },
  me_focus: { glyph: "star", bg: ["#ff9a3c", "#401404"], accent: "#ffffff" },
  me_cata: { glyph: "meteor", bg: ["#ff6a20", "#4a0a02"], accent: "#bfe8ff" },
  mo_rot: { glyph: "skull", bg: ["#8a4ac0", "#1a0830"], accent: "#c0ff80" },
  mo_drain: { glyph: "drop", bg: ["#9a5ad0", "#1e0a38"], accent: "#e8c8ff" },
  mo_abyss: { glyph: "mark", bg: ["#b46ae0", "#200838"], accent: "#ffd28a" },
  rc_blades: { glyph: "daggers", bg: ["#d04a64", "#2a0610"], accent: "#ffe0e8" },
  rc_instinct: { glyph: "eye", bg: ["#c84060", "#28040c"], accent: "#ffd28a" },
  rc_sentence: { glyph: "skull", bg: ["#e0607a", "#30040e"], accent: "#ffffff" },
  rh_knife: { glyph: "spear", bg: ["#7aa840", "#142408"], accent: "#f0ffc0" },
  rh_net: { glyph: "cage", bg: ["#8cc850", "#16280a"], accent: "#ffffff" },
  rh_rain: { glyph: "daggers", bg: ["#6a9a3a", "#0e1e06"], accent: "#e8ffa0" },
};

/**
 * Peças da biblioteca de efeitos (world/fx: rastro da arma, marca de área no chão) que cada
 * skill usa por cima da receita dela. As cores saem do tema acima (o fundo para a marca, o
 * brilho para o rastro); `color` só existe para as skills com ícone desenhado à mão (sem
 * tema).
 */
export interface SkillFx {
  /** Cor da marca e do rastro (padrão: o tema da skill). */
  color?: string;
  /**
   * Marca no chão do tamanho da área (skills em área e com respingo no alvo): anel
   * (padrão), runas, rachadura, ou nenhuma quando a receita já desenha a área.
   */
  mark?: "ring" | "runes" | "crack" | "none";
}

export const SKILL_FX: Record<string, SkillFx> = {
  // Skills básicas (ícones em icons.ts).
  fireball: { color: "#ff8a3a", mark: "none" },
  ice_nova: { color: "#9fe0ff", mark: "none" }, // o anel de gelo já vai até o raio
  shield_bash: { color: "#cfe0ff" },
  brutal_strike: { color: "#ff7a4a" },
  backstab: { color: "#c08cff" },
  fortress: { color: "#6fa8ff" },
  fury: { color: "#ff5a3a" },
  shadow_step: { color: "#8a5cff" },
  // Gritos: as ondas da receita já mostram o alcance.
  k_taunt: { mark: "none" },
  k_cry: { mark: "none" },
  bw_cry: { mark: "none" },
  kb_defy: { mark: "none" },
  b_roar: { mark: "none" },
  // Giros: anel no raio do corte.
  k_spin: { mark: "ring" },
  b_whirl: { mark: "ring" },
  bb_carnage: { mark: "ring" },
  // Pancadas no chão: rachadura no raio.
  b_quake: { mark: "crack" },
  b_stomp: { mark: "crack" },
  b_stone: { mark: "crack" },
  b_seismic: { mark: "crack" },
  b_exec: { mark: "crack" },
  k_judg: { mark: "crack" },
  k_horizon: { mark: "crack" },
  kh_dawn: { mark: "crack" },
  bb_red: { mark: "crack" },
  // Proteções e estandartes: círculo de runas.
  k_bulwark: { mark: "runes" },
  kb_last: { mark: "runes" },
  kh_sun: { mark: "runes" },
  bw_banner: { mark: "runes" },
  // Magias que caem do céu: runas na área do alvo.
  m_rain: { mark: "runes" },
  m_inferno: { mark: "runes" },
  m_meteor: { mark: "runes" },
  me_cata: { mark: "runes" },
  m_zero: { mark: "none" },
  // Venenos e fumaça: a poça e a nuvem já marcam a área.
  r_miasma: { mark: "none" },
  r_smoke: { mark: "none" },
  r_flask: { mark: "none" },
  r_plague: { mark: "none" },
};

/** Cores de uma skill na biblioteca de efeitos: a marca no chão (o fundo do tema) e o rastro (o brilho). */
export function skillFxColors(fx: string): { mark: string; trail: string } | null {
  const own = SKILL_FX[fx]?.color;
  const t = SKILL_THEMES[fx];
  if (own) return { mark: own, trail: own };
  return t ? { mark: t.bg[0], trail: t.accent ?? t.bg[0] } : null;
}
