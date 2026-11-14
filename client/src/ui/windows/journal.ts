/**
 * Diário de missões (tecla J): todas as missões ativas agrupadas por categoria (história,
 * região, diária e tutorial), cada uma com o texto, os passos com progresso, o lugar, a
 * recompensa e os botões Rastrear, Fixar e Ir; e a aba Concluídas, só com os títulos.
 *
 * A janela só existe enquanto aberta (o WindowManager cria e descarta) e redesenha só
 * quando chega um QuestLog ou muda a fixação (QuestTracker.onPanel), nunca por quadro. Só
 * a missão aberta monta o corpo inteiro.
 */

import { QuestCategory, QuestPlaceKind, type QuestDone, type QuestInfo, type QuestStepInfo } from "../../gen/erevia/v1/game_pb";
import { ui } from "../art";
import { bar, button, el, settings, tabs, toast, tooltip } from "../kit";
import { MAX_PINS } from "../quest-pins";
import { questStepTarget, type QuestTracker } from "../quest-nav";
import "../../styles/journal.css";
import type { ContractsStore } from "../contracts";
import { renderReputation } from "./reputation";

export interface JournalDeps {
  quests: QuestTracker;
  /** Botão "Ir": rastreia e anda até o passo (world/questgo.ts). */
  go(questId: string): void;
  /** Aba Reputação: facções e contratos (contracts.ts). */
  contracts?: ContractsStore;
  level?(): number;
}

/** Ordem dos grupos no Diário. */
const GROUPS: { cat: QuestCategory; label: string }[] = [
  { cat: QuestCategory.STORY, label: "História" },
  { cat: QuestCategory.REGION, label: "Região" },
  { cat: QuestCategory.DAILY, label: "Diárias" },
  { cat: QuestCategory.TUTORIAL, label: "Tutorial" },
];

/** Sem categoria nos dados = história. */
const groupOf = (c: QuestCategory) => (c === QuestCategory.UNSPECIFIED ? QuestCategory.STORY : c);

const PLACE: Record<QuestPlaceKind, string> = {
  [QuestPlaceKind.NONE]: "",
  [QuestPlaceKind.HUNT]: "Área de caça",
  [QuestPlaceKind.NPC]: "Falar com",
  [QuestPlaceKind.GATE]: "Portão",
  [QuestPlaceKind.STONE]: "Pedra de Teleporte",
  [QuestPlaceKind.SHOP]: "Loja",
};

function grouped<T extends { category: QuestCategory }>(list: readonly T[]): { label: string; items: T[] }[] {
  return GROUPS.map((g) => ({ label: g.label, items: list.filter((q) => groupOf(q.category) === g.cat) })).filter((g) => g.items.length > 0);
}

function stepRow(s: QuestStepInfo): HTMLElement {
  const done = s.count >= s.need;
  const row = el("div", `jr-step${done ? " done" : ""}`, el("div", "jr-step-head", el("span", null, s.text), el("span", "jr-count", `${s.count}/${s.need}`)));
  if (!done && s.need > 1) {
    const b = bar("xp", "jr-bar");
    b.set(s.count, s.need, "");
    row.append(b.root);
  }
  if (!done && s.place !== QuestPlaceKind.NONE && s.placeName) {
    row.append(el("small", "jr-place", `${PLACE[s.place]}: `, el("b", null, s.placeName)));
  }
  return row;
}

/** A recompensa vem resumida do servidor ("120 XP · 30 moedas · 3× Poção"): um selo por parte. */
function rewardChips(reward: string): HTMLElement {
  const parts = reward.split(" · ").filter(Boolean);
  return el(
    "div",
    "jr-reward",
    el("span", "jr-label", "Recompensa"),
    ...parts.map((p) => el("span", `jr-chip${/ XP$/.test(p) ? " xp" : / moedas?$/.test(p) ? " gold" : ""}`, p)),
  );
}

export function buildJournal(d: JournalDeps): HTMLElement {
  const root = el("div", "journal-win");
  const body = el("div", "journal-body");
  let tab: "active" | "done" | "rep" = "active";
  /** Missões abertas (corpo inteiro). Começa com a rastreada. */
  const open = new Set<string>();
  const first = d.quests.tracked()?.id;
  if (first) open.add(first);

  const card = (q: QuestInfo): HTMLElement => {
    const qs = d.quests;
    const tracked = qs.tracked()?.id === q.id;
    const pinned = qs.isPinned(q.id);
    const isOpen = open.has(q.id);
    const doneSteps = q.steps.filter((s) => s.count >= s.need).length;
    const head = el(
      "button",
      "jr-head",
      el("span", "jr-mark", tracked ? "◎" : pinned ? "•" : ""),
      el("b", "jr-title", q.title),
      pinned ? el("span", "jr-tag", tracked ? "Rastreada" : "Fixada") : "",
      el("span", "jr-sum", `${doneSteps}/${q.steps.length}`),
      el("i", "jr-caret", "▾"),
    );
    (head as HTMLButtonElement).type = "button";
    head.setAttribute("aria-expanded", String(isOpen));
    head.onclick = () => {
      if (open.has(q.id)) open.delete(q.id);
      else open.add(q.id);
      render();
    };
    const box = el("div", `jr-quest${isOpen ? " open" : ""}${tracked ? " tracked" : ""}`, head);
    if (!isOpen) return box;

    const trackBtn = button(tracked ? "Rastreada" : "Rastrear", tracked ? "btn btn-sm" : "btn btn-sm btn-gold", () => {
      const out = qs.track(q.id);
      if (out) toast(`${out} saiu do painel (continua aqui no Diário).`);
    });
    trackBtn.disabled = tracked;
    tooltip(trackBtn, () =>
      settings.get().questMarkers
        ? "Guia o minimapa, o mapa e a seta da borda da tela. Fica fixada no painel."
        : "Fica fixada no painel. Os marcadores no mapa estão desligados em Configurações → Jogo.",
    );
    const pinBtn = button(pinned ? "Desafixar" : "Fixar", "btn btn-sm", () => {
      if (pinned) return qs.unpin(q.id);
      const out = qs.pin(q.id);
      if (out) toast(`${out} saiu do painel (continua aqui no Diário).`);
    });
    tooltip(pinBtn, () => {
      if (pinned) return tracked ? "Tirar do painel do HUD (o rastreio passa para outra fixada)." : "Tirar do painel do HUD.";
      const out = qs.wouldEvict();
      return out ? `Mostrar no painel do HUD no lugar de <b>${escapeHtml(out)}</b> (máximo ${MAX_PINS}).` : "Mostrar no painel do HUD.";
    });
    const actions = el("div", "jr-actions", trackBtn, pinBtn);
    if (questStepTarget(q)) {
      const go = button("Ir", "btn btn-sm btn-gold", () => d.go(q.id));
      go.title = "Andar até o objetivo (WASD, clique no chão ou ser atacado param)";
      actions.append(go);
    }
    box.append(el("div", "jr-body", el("p", "jr-text", q.text), el("div", "jr-steps", ...q.steps.map(stepRow)), rewardChips(q.reward), actions));
    return box;
  };

  const renderActive = () => {
    const list = d.quests.list();
    if (!list.length) {
      body.replaceChildren(el("p", "muted jr-empty", "Nenhuma missão ativa agora. Novas missões chegam com a história e ao subir de nível."));
      return;
    }
    const tracked = d.quests.tracked();
    const summary = el(
      "div",
      "jr-summary",
      el("span", null, "Fixadas no painel: ", el("b", null, `${d.quests.pinnedCount()}/${MAX_PINS}`)),
      el("span", null, "Rastreada: ", tracked ? el("b", null, tracked.title) : el("span", "muted", "nenhuma")),
    );
    const groups = grouped(list).map((g) => el("section", "jr-group", el("h3", "jr-group-title", g.label, el("span", "count", String(g.items.length))), ...g.items.map(card)));
    body.replaceChildren(summary, ...groups);
  };

  const renderDone = () => {
    const list: readonly QuestDone[] = d.quests.doneList();
    if (!list.length) {
      body.replaceChildren(el("p", "muted jr-empty", "Nenhuma missão concluída ainda."));
      return;
    }
    const groups = grouped(list).map((g) =>
      el(
        "section",
        "jr-group",
        el("h3", "jr-group-title", g.label, el("span", "count", String(g.items.length))),
        el("ul", "jr-done", ...g.items.map((q) => el("li", null, el("span", "jr-check", "✓"), q.title))),
      ),
    );
    body.replaceChildren(...groups);
  };

  const render = () => {
    // As contagens das abas mudam com o QuestLog: a barra é refeita junto (só em evento).
    const nav = tabs(
      [
        { id: "active", label: "Ativas", icon: ui("scroll", 16), count: String(d.quests.list().length) },
        { id: "done", label: "Concluídas", icon: ui("sparkle", 16), count: String(d.quests.doneList().length) },
        ...(d.contracts ? [{ id: "rep", label: "Reputação", icon: ui("trophy", 16) }] : []),
      ],
      tab,
      (id) => {
        tab = id as typeof tab;
        render();
      },
    );
    if (tab === "active") renderActive();
    else if (tab === "rep" && d.contracts) body.replaceChildren(...renderReputation(d.contracts, d.level?.() ?? 0));
    else renderDone();
    root.replaceChildren(nav.root, body);
  };
  render();

  const off = d.quests.onPanel(render);
  root.addEventListener("win-close", off);
  const offRep = d.contracts?.onChange(() => tab === "rep" && render());
  if (offRep) root.addEventListener("win-close", offRep);
  return root;
}

/** Texto para dentro de HTML de dica (os títulos vêm do servidor). */
function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!);
}
