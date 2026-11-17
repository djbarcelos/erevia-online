/**
 * Maestria (aba da janela de skills, tecla K): a progressão depois do nível máximo.
 *
 * No nível máximo (o servidor manda qual é) a XP enche a barra de Maestria e cada barra
 * cheia dá 1 ponto. Quem ganhou pontos quando o máximo era 30 continua gastando e
 * redistribuindo antes dele, mas a barra só volta a encher no máximo. A árvore tem 3 ramos
 * (Caça, Sobrevivência e Utilidade), cada nó com até 5 pontos e retorno decrescente; alguns
 * nós só abrem com pontos no ramo. É mais horizontal que vertical: o que mexe no combate
 * soma no máximo 5% com a árvore inteira. "Redistribuir" devolve tudo, grátis, 1 vez por dia.
 *
 * Os dados vêm do servidor (MasteryTree, pedida ao abrir, e MasteryState). Cada clique manda
 * um MasteryCmd e o servidor confere tudo. Nada roda por quadro: só redesenha quando chega
 * mensagem ou troca de aba.
 */

import type { MasteryNodeInfo, MasteryState, MasteryTree } from "../../gen/erevia/v1/game_pb";
import { GLYPHS, skillArt, ui, uniqueIds, type SkillTheme } from "../art";
import { bar, button, el, fmt, tabs } from "../kit";
import "../../styles/mastery.css";

export interface MasteryDeps {
  tree(): MasteryTree | null;
  state(): MasteryState | null;
  onChange(fn: () => void): () => void;
  /** Pede a árvore e o estado (MASTERY_CMD_GET). */
  request(): void;
  spend(node: string): void;
  respec(): void;
  /** Nível máximo dos jogadores (Progress.maxLevel), para quem ainda não tem MasteryState. */
  maxLevel(): number;
}

const pct = (v: number) => `${(v * 100).toFixed(1).replace(".", ",").replace(",0", "")}%`;

/** Texto do bônus diário guardado ("1,5 barra", "7 barras"). */
export function restedText(s: MasteryState): string {
  if (s.need <= 0n || s.rested <= 0n) return "";
  const bars = Number(s.rested) / Number(s.need);
  const n = bars.toFixed(1).replace(".", ",").replace(",0", "");
  return `${n} ${bars > 1 ? "barras" : "barra"}`;
}

/** Dica da barra de Maestria (HUD e janela). */
export function masteryTip(s: MasteryState): string {
  const into = Number(s.xp);
  const need = Number(s.need);
  let html = `<h4>Maestria ${s.level}</h4><div class="sub">${s.level >= s.maxLevel ? "Teto da Maestria" : `${fmt.format(into)} / ${fmt.format(need)} XP · ${pct(need ? into / need : 0)}`}</div>`;
  html += `<div>Cada barra cheia dá 1 ponto para a árvore (${s.free > 0 ? `<b class="gold">${s.free} para gastar</b>` : "nenhum para gastar"}).</div>`;
  const rested = restedText(s);
  html += `<div class="cmp">`;
  html += rested
    ? `<div class="ok">Bônus diário: ${rested} de XP ${s.restedMult}× mais rápido (${fmt.format(Number(s.rested))} de XP de barra)</div>`
    : `<div class="muted">Bônus diário gasto: volta à meia-noite (Brasília)</div>`;
  html += `<div class="muted">A primeira barra de cada dia sai ${s.restedMult}× mais rápido; quem fica fora guarda até ${s.restedDays} dias.</div>`;
  html += `<div class="muted">Clique para abrir a árvore (aba Maestria das skills)</div></div>`;
  return html;
}

export function buildMastery(d: MasteryDeps): HTMLElement {
  const root = el("div", "mastery-win");
  const head = el("div", "ms-head");
  const body = el("div", "ms-body");
  const note = el("p", "ms-note small muted");
  root.append(head, body, note);
  const xpBar = bar("xp", "ms-bar mastery");
  let confirmUntil = 0;

  /** "no nível 40" (ou "no nível máximo", antes de o servidor dizer qual é). */
  const opens = (s: MasteryState | null) => {
    const n = s?.openLevel || d.maxLevel();
    return n ? `no nível ${n}` : "no nível máximo";
  };
  /** Pode gastar e redistribuir: no nível máximo ou com pontos de antes. */
  const usable = (s: MasteryState | null): s is MasteryState => !!s && (s.active || s.level > 0);
  const rank = (s: MasteryState | null, id: string) => s?.ranks.find((r) => r.node === id)?.rank ?? 0;
  const branchPts = (s: MasteryState | null, tree: MasteryTree, b: number) =>
    tree.nodes.filter((n) => n.branch === b).reduce((a, n) => a + rank(s, n.id), 0);

  function blocker(s: MasteryState | null, tree: MasteryTree, n: MasteryNodeInfo): string | null {
    const r = rank(s, n.id);
    if (!usable(s)) return `A Maestria abre ${opens(s)}.`;
    if (r >= tree.nodeMax) return "No máximo.";
    if (branchPts(s, tree, n.branch) - r < n.req) return `Pede ${n.req} pontos em ${tree.branches[n.branch]?.name ?? "neste ramo"}.`;
    if (s.free <= 0) return "Sem pontos para gastar.";
    return null;
  }

  function nodeEl(s: MasteryState | null, tree: MasteryTree, n: MasteryNodeInfo, color: string): HTMLElement {
    const r = rank(s, n.id);
    const why = blocker(s, tree, n);
    const locked = branchPts(s, tree, n.branch) - r < n.req;
    const card = el("div", `ms-node${r > 0 ? " learned" : ""}${locked ? " locked" : ""}${!why ? " available" : ""}`);
    card.style.setProperty("--bc", color);
    const icon = el("span", "ms-icon");
    const glyph = (n.icon in GLYPHS ? n.icon : "star") as SkillTheme["glyph"];
    icon.innerHTML = uniqueIds(skillArt({ glyph, bg: [color, "#120e1a"] }));
    const pips = el("span", "ms-pips");
    for (let i = 0; i < tree.nodeMax; i++) pips.append(el("i", i < r ? "on" : ""));
    const now = r > 0 ? n.ranks[r - 1] : "Nenhum ponto ainda.";
    const next = r < tree.nodeMax ? n.ranks[r] : "";
    const info = el(
      "div",
      "ms-info",
      el("b", null, n.name, n.combat ? el("small", "ms-combat", " · combate") : ""),
      el("span", "ms-now", now),
      next ? el("span", "ms-next", `${r > 0 ? "Próximo" : "Com 1 ponto"}: ${next}`) : el("span", "ms-next ok", "Nó completo."),
      n.req && locked ? el("span", "ms-req bad", `Abre com ${n.req} pontos em ${tree.branches[n.branch]?.name}.`) : "",
    );
    const plus = button("+1", "btn btn-gold btn-sm ms-plus", () => d.spend(n.id));
    plus.disabled = !!why;
    plus.title = why ?? "Gastar 1 ponto";
    plus.setAttribute("aria-label", `Gastar 1 ponto em ${n.name} (${r} de ${tree.nodeMax})`);
    card.append(el("div", "ms-left", icon, pips, el("span", "ms-rank", `${r}/${tree.nodeMax}`)), info, plus);
    return card;
  }

  function render() {
    const tree = d.tree();
    const s = d.state();
    if (!tree) {
      head.replaceChildren(el("h3", "title", "Maestria"));
      body.replaceChildren(el("p", "muted", "Carregando a árvore…"));
      note.textContent = "";
      return;
    }
    // Cabeçalho: nível, barra, bônus diário e Redistribuir.
    const level = s?.level ?? 0;
    if (s?.active) {
      const into = Number(s.xp);
      const need = Number(s.need);
      xpBar.set(into, need, s.level >= s.maxLevel ? "Teto da Maestria" : `${fmt.format(into)} / ${fmt.format(need)} XP`);
    } else xpBar.set(0, 1, usable(s) ? `A barra volta a encher ${opens(s)}` : `Abre ${opens(s)}`);
    const spent = s ? s.ranks.reduce((a, r) => a + r.rank, 0) : 0;
    const respec = button(performance.now() < confirmUntil ? "Confirmar?" : "Redistribuir", "btn btn-sm", () => {
      if (performance.now() < confirmUntil) {
        confirmUntil = 0;
        d.respec();
        return;
      }
      confirmUntil = performance.now() + 4000;
      render();
      window.setTimeout(() => {
        if (confirmUntil && performance.now() >= confirmUntil) render();
      }, 4100);
    });
    respec.disabled = !usable(s) || spent === 0 || !s.respecReady;
    respec.title = !s?.respecReady ? "Já usado hoje: libera à meia-noite (Brasília)" : "Devolve todos os pontos (grátis, 1 vez por dia)";
    const rested = s?.active ? restedText(s) : "";
    head.replaceChildren(
      el(
        "div",
        "ms-title",
        el("h3", "title", `Maestria ${level}`),
        el("span", "ms-points", el("span", "label", "Pontos"), el("b", s && s.free > 0 ? "gold" : "", String(s?.free ?? 0))),
      ),
      xpBar.root,
      el(
        "div",
        "ms-sub",
        s?.active
          ? el("span", rested ? "ok" : "muted", rested ? `Bônus diário: ${rested} ${s.restedMult}× mais rápido` : "Bônus diário gasto (volta à meia-noite)")
          : usable(s)
            ? el("span", "muted", `Seus pontos de Maestria continuam valendo. A XP volta a encher esta barra ${opens(s)}.`)
            : el("span", "muted", `A Maestria abre ${opens(s)}: a XP passa a encher esta barra.`),
        respec,
      ),
    );
    // Os 3 ramos lado a lado (no celular, um embaixo do outro).
    const cols = tree.branches.map((b, i) => {
      const col = el("section", "ms-branch");
      col.style.setProperty("--bc", b.color);
      col.append(el("header", "ms-bhead", el("b", null, b.name), el("small", null, b.description), el("span", "ms-spent", `${branchPts(s, tree, i)} pts`)));
      for (const n of tree.nodes.filter((x) => x.branch === i)) col.append(nodeEl(s, tree, n, b.color));
      return col;
    });
    body.replaceChildren(...cols);
    note.textContent = `Cada nó vai até ${tree.nodeMax} pontos, com retorno decrescente (40%, 25%, 15%, 12% e 8% do total). Os bônus de combate somam no máximo ${pct(tree.combatCap)} com a árvore inteira. Teto: Maestria ${s?.maxLevel ?? tree.nodes.length * tree.nodeMax}.`;
  }

  render();
  d.request();
  const off = d.onChange(render);
  root.addEventListener("win-close", off);
  return root;
}

export type SkillsTab = "tree" | "mastery";

/**
 * Janela de skills com duas abas: a árvore da classe e a Maestria. Cada aba é montada uma
 * vez (a da Maestria só quando abre) e fica escondida quando a outra está à frente.
 */
export function skillsWithMastery(o: {
  tree: HTMLElement;
  mastery: () => HTMLElement;
  tab: SkillsTab;
  onTab(t: SkillsTab): void;
}): { root: HTMLElement; select(t: SkillsTab): void } {
  const root = el("div", "skills-tabs");
  const panes = el("div", "skills-panes", o.tree);
  let masteryEl: HTMLElement | null = null;
  const show = (t: SkillsTab) => {
    if (t === "mastery" && !masteryEl) {
      masteryEl = o.mastery();
      panes.append(masteryEl);
    }
    o.tree.hidden = t !== "tree";
    if (masteryEl) masteryEl.hidden = t !== "mastery";
    o.onTab(t);
  };
  const nav = tabs(
    [
      { id: "tree", label: "Árvore", icon: ui("book", 16) },
      { id: "mastery", label: "Maestria", icon: ui("sparkle", 16) },
    ],
    o.tab,
    (id) => show(id as SkillsTab),
  );
  root.append(nav.root, panes);
  show(o.tab);
  // O gerenciador de janelas avisa só o primeiro filho: repassa para as duas abas.
  root.addEventListener("win-close", () => {
    o.tree.dispatchEvent(new Event("win-close"));
    masteryEl?.dispatchEvent(new Event("win-close"));
  });
  return { root, select: (t) => nav.select(t) };
}
