/**
 * Glória (tecla H): ranking e conquistas.
 *
 * - Ranking: top 50 de nível, de vitórias PvP e de clãs, com a posição do próprio
 *   personagem. O servidor calcula no banco a cada poucos minutos; a janela pede quando
 *   abre (se o que tem guardado estiver velho) e só redesenha quando a resposta chega.
 * - Conquistas: o catálogo com o progresso; cada uma dá um título para usar sob o nome.
 *
 * Nada aqui roda por quadro: tudo redesenha só quando chega mensagem ou troca de aba.
 */

import { classInfo } from "../../classes";
import type { AchievementInfo, ClanRankEntry, RankEntry, Ranking } from "../../gen/erevia/v1/game_pb";
import { ui } from "../art";
import { bar, button, el, fmt, fmt1, segmented, tabs } from "../kit";
import "../../styles/glory.css";

export interface GloryDeps {
  localId: number;
  ranking(): Ranking | null;
  onRanking(fn: () => void): () => void;
  /** Pede o ranking se o guardado estiver velho (a janela chama ao abrir). */
  refreshRanking(): void;
  achievements(): AchievementInfo[];
  titleId(): string;
  onAchievements(fn: () => void): () => void;
  setTitle(id: string): void;
}

type Board = "level" | "pvp" | "clans";

const BOARDS: { id: Board; label: string }[] = [
  { id: "level", label: "Nível" },
  { id: "pvp", label: "Vitórias PvP" },
  { id: "clans", label: "Clãs" },
];

const pos = (n: number) => (n > 0 ? `${n}º` : "—");

function ago(ms: number): string {
  const min = Math.floor((Date.now() - ms) / 60_000);
  return min < 1 ? "agora há pouco" : min === 1 ? "há 1 min" : `há ${min} min`;
}

function playerRow(r: RankEntry, board: Board, localId: number): HTMLElement {
  const info = classInfo(r.class);
  const row = el(
    "div",
    `rk-row${Number(r.characterId) === localId ? " me" : ""}`,
    el("span", "rk-pos", String(r.pos)),
    el(
      "div",
      "rk-body",
      el("b", null, r.name, r.title ? el("i", "rk-title", ` «${r.title}»`) : ""),
      el("small", null, [info.label, r.clanName ? `‹${r.clanName}›` : ""].filter(Boolean).join(" · ")),
    ),
    board === "pvp" ? el("span", "rk-val", el("b", null, fmt.format(r.pvpKills)), el("small", null, "vitórias")) : "",
    // No nível máximo, o nível de Maestria ao lado do nível (também desempata a ordem).
    el("span", "rk-val", el("b", null, `Nv ${r.level}`), r.mastery > 0 ? el("small", "rk-mastery", `Maestria ${r.mastery}`) : ""),
  );
  row.style.setProperty("--cc", info.color);
  return row;
}

function clanRow(c: ClanRankEntry): HTMLElement {
  return el(
    "div",
    "rk-row",
    el("span", "rk-pos", String(c.pos)),
    el("div", "rk-body", el("b", "clan-name", `‹${c.name}›`), el("small", null, `Líder: ${c.leader || "—"} · ${c.members} membros`)),
    el("span", "rk-val", el("b", null, fmt.format(c.levelSum)), el("small", null, "níveis")),
    el("span", "rk-val", el("b", null, fmt1.format(c.levelAvg)), el("small", null, "média")),
  );
}

export function buildGlory(d: GloryDeps): HTMLElement {
  const root = el("div", "glory-win");
  const body = el("div", "glory-body");
  let tab: "ranking" | "achievements" = "ranking";
  let board: Board = "level";

  const renderRanking = () => {
    const r = d.ranking();
    const boards = segmented(BOARDS, board, (b) => {
      board = b;
      render();
    });
    if (!r) {
      body.replaceChildren(boards, el("p", "muted", "Carregando o ranking…"));
      return;
    }
    const mine = el(
      "div",
      "rk-mine",
      el("span", null, "Sua posição: ", el("b", null, pos(r.myLevelPos)), " em nível"),
      el("span", null, el("b", null, pos(r.myPvpPos)), " em PvP"),
      el("span", null, "seu clã: ", el("b", null, pos(r.myClanPos))),
    );
    const list = el("div", "rk-list");
    if (board === "clans") for (const c of r.clans) list.append(clanRow(c));
    else for (const e of board === "pvp" ? r.pvp : r.level) list.append(playerRow(e, board, d.localId));
    if (!list.childElementCount) {
      list.append(el("p", "muted", board === "pvp" ? "Ninguém venceu um combate PvP ainda." : board === "clans" ? "Nenhum clã ainda." : "Ninguém no ranking ainda."));
    }
    const updated = Number(r.updatedAtMs);
    const every = Math.round(r.refreshMs / 60_000);
    body.replaceChildren(
      boards,
      mine,
      list,
      el(
        "p",
        "small muted",
        updated ? `Atualizado ${ago(updated)} · recalculado a cada ${every} min (os 50 primeiros).` : "O ranking ainda está sendo calculado. Tente de novo em instantes.",
      ),
    );
  };

  const renderAchievements = () => {
    const list = d.achievements();
    const current = d.titleId();
    const done = list.filter((a) => a.unlocked).length;
    const currentTitle = list.find((a) => a.id === current)?.title;
    const head = el(
      "div",
      "ach-head",
      el("span", null, el("b", null, `${done}/${list.length}`), " conquistas"),
      el("span", null, "Título: ", currentTitle ? el("b", "ach-cur", `«${currentTitle}»`) : el("span", "muted", "nenhum")),
      current ? button("Tirar título", "btn btn-sm", () => d.setTitle("")) : "",
    );
    const items = el("div", "ach-list");
    for (const a of list) {
      const using = a.id === current;
      const side = el("div", "ach-side");
      if (a.unlocked) {
        side.append(
          using ? el("span", "ach-using", "Em uso") : button("Usar título", "btn btn-sm btn-gold", () => d.setTitle(a.id)),
          el("small", "muted", new Date(Number(a.unlockedAtMs)).toLocaleDateString("pt-BR")),
        );
      } else if (a.need > 1) {
        const b = bar("xp", "ach-bar");
        b.set(a.count, a.need, `${fmt.format(a.count)}/${fmt.format(a.need)}`);
        side.append(b.root);
      } else side.append(el("span", "muted small", "Bloqueada"));
      items.append(
        el(
          "div",
          `ach-row${a.unlocked ? " done" : ""}${using ? " using" : ""}`,
          el("span", "ach-mark", a.unlocked ? "★" : "☆"),
          el("div", "ach-body", el("b", null, a.name), el("small", null, a.text), el("small", "ach-title", `Título: «${a.title}»`)),
          side,
        ),
      );
    }
    body.replaceChildren(head, items, el("p", "small muted", "O título escolhido aparece sob o seu nome para todos."));
  };

  const render = () => (tab === "ranking" ? renderRanking() : renderAchievements());
  const nav = tabs(
    [
      { id: "ranking", label: "Ranking", icon: ui("trophy", 16) },
      { id: "achievements", label: "Conquistas", icon: ui("sparkle", 16) },
    ],
    tab,
    (id) => {
      tab = id as typeof tab;
      render();
    },
  );
  root.append(nav.root, body);
  render();
  d.refreshRanking();

  const offR = d.onRanking(() => tab === "ranking" && render());
  const offA = d.onAchievements(() => tab === "achievements" && render());
  root.addEventListener("win-close", () => {
    offR();
    offA();
  });
  return root;
}
