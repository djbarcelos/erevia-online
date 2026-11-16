/**
 * Textos da XP do grupo na janela do grupo (tunnel#151): o bônus de quem está por perto e
 * o aviso quando a diferença de nível corta a XP. Os números vêm do servidor (PartyState).
 */

export interface PartyXPState {
  members: readonly unknown[];
  xpNear: number;
  xpBonusPct: number;
  xpLevelGap: number;
  xpGapCutPct: number;
}

/** Distância (m) do monstro para contar na XP do grupo (game.PartyXPRange). */
export const PARTY_XP_RANGE = 40;

export interface PartyXPText {
  /** "Bônus de grupo +20% (3 por perto)" ou por que não há bônus ("" = sem grupo). */
  bonus: string;
  /** O corte pela diferença de nível ("" = sem corte). */
  warn: string;
}

export function partyXPText(st: PartyXPState): PartyXPText {
  if (st.members.length === 0) return { bonus: "", warn: "" };
  if (st.xpNear === 0) return { bonus: "Morto: sem XP do grupo até voltar.", warn: "" };
  const bonus =
    st.xpNear <= 1
      ? `Sem bônus de grupo: ninguém do grupo a até ${PARTY_XP_RANGE} m.`
      : `Bônus de grupo +${st.xpBonusPct}% (${st.xpNear} por perto)`;
  const warn =
    st.xpGapCutPct > 0
      ? `Diferença de ${st.xpLevelGap} níveis no grupo: a XP de todos cai ${st.xpGapCutPct}% (o corte começa com mais de 8).`
      : "";
  return { bonus, warn };
}
