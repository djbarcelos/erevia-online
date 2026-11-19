import { ArenaPhase, type ArenaMatch, type ArenaState } from "../gen/erevia/v1/game_pb";
import { classInfo } from "../classes";
import { button, el } from "./kit";

export interface ArenaActions {
  queue(npcId: number, mode: number): void;
  leave(npcId: number): void;
}

const MODES = [
  { mode: 1, label: "1x1", hint: "Sozinho, contra outro jogador." },
  { mode: 3, label: "3x3", hint: "Um grupo de 3: o líder põe o time na fila." },
];

/**
 * Arena PvP ranqueada (tunnel#81).
 *  - Janela do Mestre da Arena: a temporada, a sua pontuação em cada modo (vitórias e
 *    derrotas), entrar e sair da fila e o ranking da temporada.
 *  - Na partida: um painel no alto da tela com a fase, o tempo e quem está de pé em cada time.
 * Só redesenha quando chega um ArenaState (o tempo do painel desce sozinho, 1×/s).
 */
export class ArenaWindow {
  readonly root = el("div", "shop-panel arena-panel frame");
  private title = el("h2", null, "Arena de Vel'Aran");
  private season = el("p", "small muted arena-season");
  private modes = el("div", "arena-modes");
  private ranking = el("div", "arena-ranking");
  private tab = 1;
  private state: ArenaState | null = null;
  npcId = 0;

  /** O painel da partida (fica no HUD, fora da janela). */
  readonly matchEl = el("div", "arena-match frame glass");
  private matchTimer = 0;
  private matchEnd = 0;

  constructor(
    private actions: ArenaActions,
    parent: HTMLElement,
    private myId: () => number,
    /** Os times da partida para quem ataca quem (null = fora de partida). */
    private onTeams: (mine: number[] | null, foes: number[]) => void = () => {},
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    const close = button("✕", "x-btn", () => this.close());
    close.title = "Fechar (Esc)";
    this.root.append(
      el("div", "panel-head", this.title, close),
      this.season,
      this.modes,
      el("h3", "arena-sub", "Ranking da temporada"),
      this.ranking,
      el(
        "p",
        "small muted arena-rules",
        "Na arena não há karma, perda de XP nem itens caídos. Quem cai fica fora até o fim. Vence o time que derrubar o outro; no fim dos 3 minutos, quem tiver mais vida.",
      ),
    );
    this.matchEl.hidden = true;
    parent.append(this.root, this.matchEl);
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  close(): void {
    this.root.hidden = true;
  }

  set(s: ArenaState): void {
    if (s.open) {
      this.npcId = s.npcId;
      this.root.hidden = false;
    }
    if (s.modes.length) this.state = s;
    if (this.isOpen && this.state) this.paint(this.state);
    this.paintMatch(s.match);
  }

  private paint(s: ArenaState): void {
    this.season.textContent = s.seasonLabel ? `Temporada de ${s.seasonLabel}. A pontuação recomeça todo mês.` : "";
    this.modes.replaceChildren();
    const queued = s.modes.some((m) => m.queued);
    for (const def of MODES) {
      const m = s.modes.find((x) => x.mode === def.mode);
      if (!m) continue;
      const action = m.queued
        ? button("Sair da fila", "btn btn-sm", () => this.actions.leave(this.npcId))
        : button("Entrar na fila", "btn btn-gold btn-sm", () => this.actions.queue(this.npcId, def.mode));
      if (!m.queued && queued) action.disabled = true;
      this.modes.append(
        el(
          "section",
          `arena-mode${m.queued ? " queued" : ""}`,
          el("div", "arena-mode-head", el("b", null, def.label), el("span", "arena-rating", String(m.rating))),
          el("p", "small muted", def.hint),
          el("p", "small", `${m.wins} ${m.wins === 1 ? "vitória" : "vitórias"} · ${m.losses} ${m.losses === 1 ? "derrota" : "derrotas"}`),
          el("p", "small muted", m.queued ? "Na fila: esperando outro time…" : `${m.queueSize} ${m.queueSize === 1 ? "time" : "times"} na fila`),
          action,
        ),
      );
    }
    this.ranking.replaceChildren();
    const tabs = el("div", "arena-tabs");
    for (const def of MODES) {
      const b = button(def.label, `arena-tab${this.tab === def.mode ? " on" : ""}`, () => {
        this.tab = def.mode;
        this.paint(s);
      });
      tabs.append(b);
    }
    const rows = s.rankings.find((r) => r.mode === this.tab)?.rows ?? [];
    const list = el("ol", "arena-rank-list");
    if (!rows.length) list.append(el("li", "muted small", "Ninguém lutou neste modo ainda."));
    for (const r of rows) {
      list.append(
        el(
          "li",
          null,
          el("span", "arena-rank-name", r.name),
          el("span", "muted small", ` ${classInfo(r.class).label} nv ${r.level}`),
          el("b", "arena-rank-rating", String(r.rating)),
          el("span", "muted small", ` ${r.wins}V ${r.losses}D`),
        ),
      );
    }
    this.ranking.append(tabs, list);
  }

  /** O painel da partida: fase, tempo e quem está de pé. */
  private paintMatch(m: ArenaMatch | undefined): void {
    window.clearInterval(this.matchTimer);
    if (!m || m.phase === ArenaPhase.NONE) {
      this.matchEl.hidden = true;
      this.onTeams(null, []);
      return;
    }
    this.onTeams(
      m.fighters.filter((f) => f.team === m.myTeam).map((f) => f.id),
      m.phase === ArenaPhase.FIGHT ? m.fighters.filter((f) => f.team !== m.myTeam && f.alive).map((f) => f.id) : [],
    );
    this.matchEl.hidden = false;
    this.matchEnd = performance.now() + m.secondsLeft * 1000;
    const timer = el("b", "arena-timer");
    const phase =
      m.phase === ArenaPhase.COUNTDOWN
        ? "Prepare-se"
        : m.phase === ArenaPhase.FIGHT
          ? "Lutem!"
          : m.winner < 0
            ? "Empate"
            : m.winner === m.myTeam
              ? "Vitória!"
              : "Derrota";
    const team = (t: number) =>
      el(
        "div",
        `arena-team ${t === m.myTeam ? "mine" : "theirs"}`,
        ...m.fighters.filter((f) => f.team === t).map((f) => el("span", f.alive ? "alive" : "out", f.name + (f.id === this.myId() ? " (você)" : ""))),
      );
    const rating = m.ratingAfter ? el("span", "small", ` Pontuação: ${m.ratingBefore} → ${m.ratingAfter}`) : "";
    this.matchEl.replaceChildren(
      el("div", "arena-match-head", el("span", null, `Arena ${m.mode === 3 ? "3x3" : "1x1"} · ${phase}`), timer, rating),
      el("div", "arena-teams", team(m.myTeam), el("span", "arena-vs", "×"), team(1 - m.myTeam)),
    );
    const tick = () => {
      const left = Math.max(0, Math.ceil((this.matchEnd - performance.now()) / 1000));
      timer.textContent = `${Math.floor(left / 60)}:${String(left % 60).padStart(2, "0")}`;
    };
    tick();
    this.matchTimer = window.setInterval(tick, 1000);
  }

  dispose(): void {
    window.clearInterval(this.matchTimer);
    this.root.remove();
    this.matchEl.remove();
  }
}
