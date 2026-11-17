/**
 * Aba Reputação do Diário de missões (J): as facções com o grau e a barra até o próximo,
 * a escada dos graus e os contratos de hoje com o progresso (aceitar é no Quadro de
 * Contratos, na praça). Redesenha só quando chega ContractState ou ReputationState.
 */

import { ContractSlot, ContractStatus } from "../../gen/erevia/v1/game_pb";
import { contractCard, factionTag, repBar, untilText, type ContractsStore } from "../contracts";
import { el, fmt } from "../kit";

export function renderReputation(s: ContractsStore, level: number): HTMLElement[] {
  const rep = s.rep;
  const st = s.state;
  if (!rep || !st) return [el("p", "muted jr-empty", "Carregando a reputação…")];
  const out: HTMLElement[] = [];
  out.push(
    el(
      "p",
      "ct-note",
      "Os contratos do Quadro de Contratos (praça de Vel'Aran, ao sul do poço) dão reputação com a facção de cada um. O grau abre a loja da facção, no mesmo quadro.",
    ),
  );
  for (const f of rep.factions) {
    out.push(
      el(
        "section",
        "jr-group ct-rep-faction",
        el("h3", "jr-group-title", factionTag(f, rep.gradeNames[f.grade])),
        el("p", "ct-text", f.text),
        repBar(f, (g) => s.gradeName(g)),
        el(
          "div",
          "ct-ladder",
          ...rep.gradeNames.map((name, i) =>
            el("span", `ct-step${i <= f.grade ? " on" : ""}`, el("b", null, name), el("small", null, fmt.format(rep.gradePoints[i] ?? 0))),
          ),
        ),
      ),
    );
  }
  const now = Date.now();
  const low = level < st.minLevel;
  const taken = st.contracts.filter((c) => c.status !== ContractStatus.OPEN).length;
  out.push(
    el(
      "section",
      "jr-group",
      el(
        "h3",
        "jr-group-title",
        "Contratos de hoje",
        el("span", "count", `${taken}/${st.contracts.length}`),
      ),
      el(
        "p",
        "ct-reset",
        low
          ? `A partir do nível ${st.minLevel}.`
          : `Diários renovam em ${untilText(Number(st.dailyResetMs) - now)}; o semanal, em ${untilText(Number(st.weeklyResetMs) - now)}. Aceite no Quadro de Contratos.`,
      ),
      ...st.contracts
        .slice()
        .sort((a, b) => Number(a.slot === ContractSlot.WEEKLY) - Number(b.slot === ContractSlot.WEEKLY))
        .map((c) => contractCard(c, s)),
    ),
  );
  return out;
}
