/**
 * Árvore de skills: 3 ramos × 4 degraus por classe. Clique soma um ponto (fica pendente
 * até confirmar), botão direito tira; arraste uma skill ativa aprendida para a barra de
 * atalhos. No celular, toque abre os detalhes com os botões.
 */

import type { ClassId } from "../../../api";
import { CLASSES, classInfo } from "../../../classes";
import { ui } from "../../art";
import { game } from "../../data/mock";
import { nodeIcon, TIER_LEVELS, TREES, type SkillNode } from "../../data/skilltree";
import { beginDrag } from "../../hud/dragdrop";
import { art, button, closeSheet, el, isCompact, isTouch, openSheet, toast, tooltip } from "../../kit";
import { skillTip } from "../hud";

export function build(): HTMLElement {
  const h = game.hero;
  let viewCls: ClassId = h.cls;
  let pending: Record<string, number> = { ...h.ranks };
  let selected: SkillNode | null = null;

  const root = el("div", "tree-win");
  const head = el("div", "tree-head");
  const classSel = el("div", "seg tree-classes");
  for (const c of CLASSES) {
    const b = button(c.label, "");
    b.setAttribute("aria-pressed", String(c.id === viewCls));
    b.onclick = () => {
      viewCls = c.id;
      classSel.querySelectorAll("button").forEach((x) => x.setAttribute("aria-pressed", String(x === b)));
      selected = null;
      render();
    };
    classSel.append(b);
  }
  const points = el("div", "tree-points");
  const undo = button("Desfazer", "btn btn-sm", () => {
    pending = { ...h.ranks };
    render();
  });
  const confirm = button("Confirmar", "btn btn-gold btn-sm", () => {
    h.ranks = { ...pending };
    game.emit("ranks");
    toast("Skills aprendidas!");
    render();
  });
  head.append(classSel, points, el("div", "tree-actions", undo, confirm));

  const body = el("div", "tree-body");
  const tree = el("div", "tree-grid");
  const detail = el("aside", "tree-detail");
  body.append(tree, detail);
  const note = el("p", "tree-note small muted");
  root.append(head, body, note);

  const mine = () => viewCls === h.cls;
  const used = () => Object.values(pending).reduce((a, b) => a + b, 0);
  const free = () => game.skillPointsFree() + Object.values(h.ranks).reduce((a, b) => a + b, 0) - used();
  const rank = (n: SkillNode) => (mine() ? (pending[n.id] ?? 0) : 0);
  const parent = (n: SkillNode) => TREES[viewCls].nodes.find((x) => x.branch === n.branch && x.tier === n.tier - 1);

  /** Por que não dá para subir (ou null se dá). */
  function blocker(n: SkillNode): string | null {
    if (!mine()) return `Skill de ${classInfo(viewCls).label}.`;
    if (rank(n) >= n.maxRank) return "Rank máximo.";
    if (h.level < TIER_LEVELS[n.tier]) return `Requer nível ${TIER_LEVELS[n.tier]}.`;
    const p = parent(n);
    if (p && rank(p) < 1) return `Requer ${p.name}.`;
    if (free() <= 0) return "Sem pontos de skill.";
    return null;
  }

  function canLower(n: SkillNode): boolean {
    if (!mine()) return false;
    const r = pending[n.id] ?? 0;
    if (r <= (h.ranks[n.id] ?? 0)) return false; // só desfaz o que está pendente
    const child = TREES[viewCls].nodes.find((x) => x.branch === n.branch && x.tier === n.tier + 1);
    return !(r === 1 && child && (pending[child.id] ?? 0) > 0);
  }

  function raise(n: SkillNode) {
    const why = blocker(n);
    if (why) return toast(why, "warn");
    pending[n.id] = rank(n) + 1;
    selected = n;
    render();
  }

  function lower(n: SkillNode) {
    if (!canLower(n)) return;
    pending[n.id] = (pending[n.id] ?? 0) - 1;
    if (!pending[n.id]) delete pending[n.id];
    render();
  }

  function toHotbar(n: SkillNode, at?: number) {
    if ((h.ranks[n.id] ?? 0) < 1) return toast("Aprenda e confirme a skill primeiro.", "warn");
    if (n.kind !== "active") return toast("Passivas funcionam sozinhas.", "warn");
    const bar = h.hotbar;
    const old = bar.indexOf(n.id);
    const i = at ?? (old >= 0 ? old : bar.findIndex((x) => !x));
    if (i < 0) return toast("Barra cheia: arraste para um atalho.", "warn");
    if (old >= 0 && old !== i) bar[old] = bar[i];
    bar[i] = n.id;
    game.emit("hotbar");
    toast(`${n.name} na tecla ${i === 9 ? 0 : i + 1}.`);
  }

  function nodeEl(n: SkillNode): HTMLElement {
    const r = rank(n);
    const learned = mine() && (h.ranks[n.id] ?? 0) > 0;
    const pend = mine() && r > (h.ranks[n.id] ?? 0);
    const state = r > 0 ? "learned" : blocker(n) === null ? "available" : "locked";
    const b = el("button", `tnode ${n.kind} ${state}${pend ? " pending" : ""}${selected?.id === n.id ? " sel" : ""}`);
    b.type = "button";
    b.style.setProperty("--bc", TREES[viewCls].branches[n.branch].color);
    b.append(art(nodeIcon(n), "tn-icon"), el("span", "tn-rank", `${r}/${n.maxRank}`));
    if (state === "locked" && h.level < TIER_LEVELS[n.tier]) b.append(art(ui("lock", 12), "tn-lock"));
    b.setAttribute("aria-label", `${n.name}, rank ${r} de ${n.maxRank}`);
    tooltip(b, () => {
      const why = blocker(n);
      return `${skillTip(n, r)}<div class="cmp">${r < n.maxRank ? `<div class="muted">Próximo rank: ${n.effect(r + 1)}</div>` : ""}${why ? `<div class="bad">${why}</div>` : `<div class="ok">Clique para aprender</div>`}${learned && n.kind === "active" ? `<div class="muted">Arraste para a barra de atalhos</div>` : ""}</div>`;
    });
    b.onclick = () => {
      selected = n;
      if (isTouch() && isCompact()) return details(n);
      raise(n);
    };
    b.oncontextmenu = (e) => {
      e.preventDefault();
      lower(n);
    };
    if (learned && n.kind === "active") {
      b.addEventListener("pointerdown", (e) => {
        if (isTouch()) return;
        beginDrag(e, {
          icon: nodeIcon(n),
          onDrop(t) {
            const hb = t?.closest<HTMLElement>("[data-hotbar]");
            if (hb) toHotbar(n, Number(hb.dataset.hotbar));
          },
        });
      });
    }
    const label = el("div", "tn-wrap", b, el("span", "tn-name", n.name));
    return label;
  }

  function details(n: SkillNode) {
    const r = rank(n);
    const wrap = el("div", "tree-sheet");
    wrap.append(el("div", "ts-head", art(nodeIcon(n), `ts-icon ${n.kind}`), el("div", null, el("h3", "title", n.name), el("div", "muted small", `${n.kind === "active" ? "Ativa" : "Passiva"} · Rank ${r}/${n.maxRank}`))));
    const tip = el("div", "ts-body");
    tip.innerHTML = skillTip(n, r).replace(/<h4>.*?<\/h4><div class="sub">.*?<\/div>/, "");
    wrap.append(tip);
    if (r < n.maxRank) wrap.append(el("p", "muted small", `Próximo rank: ${n.effect(r + 1)}`));
    const why = blocker(n);
    if (why) wrap.append(el("p", "bad small", why));
    const acts = el("div", "is-actions");
    const plus = button("Aprender (+1)", "btn btn-gold", () => {
      closeSheet();
      raise(n);
    });
    plus.disabled = !!why;
    const minus = button("−1", "btn", () => {
      closeSheet();
      lower(n);
    });
    minus.disabled = !canLower(n);
    acts.append(plus, minus);
    if (n.kind === "active" && (h.ranks[n.id] ?? 0) > 0) {
      acts.append(
        button("Colocar no atalho", "btn", () => {
          closeSheet();
          toHotbar(n);
        }),
      );
    }
    wrap.append(acts);
    openSheet(wrap);
  }

  function renderDetail() {
    const n = selected;
    detail.replaceChildren();
    if (!n) {
      detail.append(
        el("h3", "title", "Como funciona"),
        el(
          "ul",
          "how small muted",
          el("li", null, "Clique numa skill para gastar 1 ponto."),
          el("li", null, "Botão direito desfaz um ponto ainda não confirmado."),
          el("li", null, "Cada degrau libera num nível: 1, 5, 10 e 15."),
          el("li", null, "É preciso ter a skill de cima no mesmo ramo."),
          el("li", null, "Arraste skills ativas para a barra de atalhos."),
        ),
      );
      return;
    }
    const r = rank(n);
    detail.append(
      el("div", "td-icon-wrap", art(nodeIcon(n), `td-icon ${n.kind}`)),
      el("h3", "title td-name", n.name),
      el("div", "muted small", `${TREES[viewCls].branches[n.branch].name} · ${n.kind === "active" ? "Ativa" : "Passiva"} · Rank ${r}/${n.maxRank}`),
    );
    const ranks = el("ol", "td-ranks");
    for (let i = 1; i <= n.maxRank; i++) ranks.append(el("li", i <= r ? "on" : "", n.effect(i)));
    detail.append(ranks);
    if (n.kind === "active") detail.append(el("div", "td-meta", ...[n.mp ? `${n.mp} MP` : "", n.cd ? `${n.cd} s de recarga` : "", n.range ? `${n.range} m` : ""].filter(Boolean).map((t) => el("span", null, t))));
    const why = blocker(n);
    const acts = el("div", "td-actions");
    const plus = button("Aprender", "btn btn-gold btn-sm", () => raise(n));
    plus.disabled = !!why;
    acts.append(plus);
    if (n.kind === "active" && (h.ranks[n.id] ?? 0) > 0) acts.append(button("Pôr no atalho", "btn btn-sm", () => toHotbar(n)));
    detail.append(acts);
    if (why) detail.append(el("p", "bad small", why));
  }

  function render() {
    const t = TREES[viewCls];
    tree.replaceChildren();
    // Rótulos de nível (coluna 0)
    tree.append(el("div", "tg-corner"));
    t.branches.forEach((b, i) => {
      const spent = t.nodes.filter((n) => n.branch === i).reduce((a, n) => a + rank(n), 0);
      const col = el("div", "tg-branch", el("b", null, b.name), el("small", null, b.desc), el("span", "tg-spent", `${spent} pts`));
      col.style.setProperty("--bc", b.color);
      tree.append(col);
    });
    for (let tier = 0; tier < 4; tier++) {
      const lvl = TIER_LEVELS[tier];
      tree.append(el("div", `tg-level${h.level < lvl ? " locked" : ""}`, el("span", null, `Nv ${lvl}`)));
      for (let br = 0; br < 3; br++) {
        const n = t.nodes.find((x) => x.branch === br && x.tier === tier)!;
        const cell = el("div", "tg-cell");
        if (tier > 0) {
          const p = parent(n)!;
          const lit = rank(p) > 0;
          const line = el("span", `tg-line${lit ? " lit" : ""}`);
          line.style.setProperty("--bc", t.branches[br].color);
          cell.append(line);
        }
        cell.append(nodeEl(n));
        tree.append(cell);
      }
    }
    const f = free();
    points.replaceChildren(el("span", "label", "Pontos"), el("b", f > 0 ? "gold" : "", String(mine() ? f : 0)));
    const changed = mine() && JSON.stringify(pending) !== JSON.stringify(h.ranks);
    undo.disabled = !changed;
    confirm.disabled = !changed;
    note.textContent = mine()
      ? `Nível ${h.level}: 1 ponto por nível. Conteúdo das skills novas é provisório.`
      : `Visualizando a árvore de ${classInfo(viewCls).label} — só dá para aprender as da sua classe.`;
    renderDetail();
  }

  render();
  return root;
}
