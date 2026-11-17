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

/**
 * NPCs com identidade (tunnel#NPC, docs/arte/remaster-npcs.md): nome do NPC no Welcome → modelo próprio do estilo B.
 * O servidor continua mandando o modelo de herói (knight, rogue...); com `?remaster=1` o cliente troca pelo mapa abaixo.
 * Carregam sob demanda (CharacterModels.ensure), na primeira vez que o NPC aparece.
 */
export const REMASTER_NPC_BY_NAME: Record<string, string> = {
  Lívia: "npc_livia", Bruno: "npc_bruno", Teodoro: "npc_teodoro", Otto: "npc_otto", Ysolde: "npc_ysolde", Edrian: "npc_edrian",
  Severino: "npc_severino", Iselda: "npc_iselda", Cássio: "npc_cassio", Anselmo: "npc_anselmo", Marga: "npc_marga",
  Dárcio: "npc_darcio", Maelis: "npc_maelis", Gaspar: "npc_gaspar", Bertrand: "npc_guarda", Rovena: "npc_rovena",
};

/** NPCs da praça: carregam junto com os heróis (os das outras regiões, sob demanda). */
export const REMASTER_NPC_EAGER = ["npc_livia", "npc_bruno", "npc_teodoro", "npc_otto", "npc_ysolde", "npc_edrian", "npc_severino", "npc_iselda", "npc_cassio"];

/** Modelo do remaster de um NPC pelo nome (null = sem remaster ou NPC sem modelo próprio). */
export function remasterNpcModel(name: string): string | null {
  return REMASTER ? (REMASTER_NPC_BY_NAME[name] ?? null) : null;
}

/** Modelo do jogo → arquivo remasterizado (sem a extensão). */
export const REMASTER_FILES: Record<string, string> = {
  knight: "knight_b",
  barbarian: "barbarian_b",
  mage: "mage_b",
  rogue: "rogue_b",
  skeleton_warrior: "skeleton_warrior_b",
  // Bestiário (tunnel#98).
  skeleton_minion: "skeleton_minion_b",
  skeleton_rogue: "skeleton_rogue_b",
  skeleton_mage: "skeleton_mage_b",
  stone_golem: "stone_golem_b",
  // Chefes com silhueta própria: não são modelos do servidor, ver remasterModelOf.
  bone_lord: "bone_lord_b",
  ozrhal: "ozrhal_b",
  // NPCs (REMASTER_NPC_BY_NAME): o arquivo é <modelo>_b; o servidor não conhece estes modelos.
  ...Object.fromEntries(Object.values(REMASTER_NPC_BY_NAME).map((m) => [m, `${m}_b`])),
};

/** Modelos que só existem no remaster (sem arquivo do estilo antigo para cair de volta). */
export const REMASTER_ONLY = ["bone_lord", "ozrhal"];

/**
 * Modelo do estilo B para uma entidade: os dois chefes que no servidor usam o esqueleto guerreiro
 * (Senhor dos Ossos) e o esqueleto mago (Ozrhal) ganham silhueta própria. O servidor não muda: o
 * Guarda Ossário e os outros guerreiros/magos comuns não são chefes.
 */
export function remasterModelOf(model: string, boss: boolean): string {
  if (!REMASTER || !boss) return model;
  if (model === "skeleton_warrior") return "bone_lord";
  if (model === "skeleton_mage") return "ozrhal";
  return model;
}

/** Altura (m) em que o modelo do remaster é normalizado antes da escala da entidade (padrão: 1,8). */
export const REMASTER_HEIGHTS: Record<string, number> = {
  stone_golem: 2.65, // a altura do golem em código (GOLEM_BODY), antes da escala 1,25 do servidor
  skeleton_minion: 1.5, // o lacaio é baixo e de crânio grande: lê menor que o guerreiro de longe
  skeleton_rogue: 1.75,
  skeleton_mage: 2.0, // com o chapéu pontudo
  // NPCs: a altura de cada um (a mesma que o rig do Meshy recebeu em height_meters).
  npc_livia: 1.65, npc_bruno: 1.8, npc_teodoro: 1.85, npc_otto: 1.55, npc_ysolde: 1.75, npc_edrian: 1.8, npc_severino: 1.65,
  npc_iselda: 1.72, npc_cassio: 1.8, npc_guarda: 1.78, npc_anselmo: 1.7, npc_marga: 1.7, npc_darcio: 1.8, npc_maelis: 1.7,
  npc_gaspar: 1.65, npc_rovena: 1.8,
};

/** Modelo do jogo → máscara de repintura (PNG ao lado do GLB; ver scripts/make-remaster-mask.mjs). */
export const REMASTER_MASKS: Record<string, string> = {
  knight: "knight_b.mask.png",
  barbarian: "barbarian_b.mask.png",
  mage: "mage_b.mask.png",
  rogue: "rogue_b.mask.png",
  npc_guarda: "npc_guarda_b.mask.png", // o tabardo do guarda: Bertrand (Cavaleiro do Bastião) usa a cor do Bastião
};

/** Distâncias (m) a partir das quais cada LOD entra (guia de arte, seção 5). */
export const LOD_DISTANCES = [20, 35];

/** O LOD que vale a essa distância ao quadrado. */
export function lodFor(distSq: number): number {
  return distSq > LOD_DISTANCES[1] ** 2 ? 2 : distSq > LOD_DISTANCES[0] ** 2 ? 1 : 0;
}

/**
 * Armas e escudos do remaster (tunnel#97): modelo do jogo (nome do KayKit) → arquivo novo em weapons/ e
 * escala sobre ele. Cada arquivo novo já vem na convenção do KayKit (cabo na origem, ponta em +Y, machado com a
 * lâmina para -X, escudo com a face para +Z); a escala só acomoda os modelos que dividem a mesma forma
 * (por exemplo as espadas dos esqueletos, um pouco menores). Forma própria por família: comum (`*_b`) e alta
 * (`*_high_b`); a raridade intermediária se lê pelo brilho do material.
 */
export const REMASTER_WEAPONS: Record<string, { file: string; scale?: number }> = {
  sword_1handed: { file: "sword_b" },
  sword_2handed: { file: "sword_high_b" },
  sword_2handed_color: { file: "sword_high_b" },
  skeleton_blade: { file: "sword_b", scale: 0.84 },
  axe_1handed: { file: "axe_b" },
  axe_2handed: { file: "axe_high_b" },
  skeleton_axe: { file: "axe_b" },
  staff: { file: "staff_b" },
  skeleton_staff: { file: "staff_high_b" },
  wand: { file: "wand_b" },
  dagger: { file: "dagger_b" },
  shield_round_barbarian: { file: "shield_b" },
  shield_spikes: { file: "shield_b", scale: 1.1 },
  shield_spikes_color: { file: "shield_b", scale: 1.1 },
  shield_badge_color: { file: "shield_high_b", scale: 0.86 },
  shield_square: { file: "shield_high_b" },
  skeleton_shield_large_a: { file: "shield_b", scale: 1.25 },
};
