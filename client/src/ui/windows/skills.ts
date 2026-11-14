/**
 * Árvore de skills (tecla K): 3 ramos × 5 degraus da classe (níveis 1, 5, 10, 15 e 20), com
 * os dados do servidor (ranks, textos de cada rank, nível de cada degrau). O número de
 * degraus vem das skills, então um degrau novo no servidor aparece sozinho. Clique soma 1 ponto (fica pendente);
 * botão direito desfaz o pendente; "Confirmar" manda LearnSkills (o servidor confere tudo).
 * Skills ativas aprendidas vão para a barra arrastando (ou pelo botão, no celular).
 * Embaixo, o ramo da especialização (níveis 20, 24 e 28): só o do caminho escolhido na
 * Mestra dos Caminhos (Progress.spec); antes disso, um aviso com os dois caminhos.
 */

import type { Progress, SkillBranch, SkillInfo } from "../../gen/erevia/v1/game_pb";
import { formatSeconds, skillIcon } from "../icons";
import { ui, uniqueIds } from "../art";
import { beginDrag } from "../dragdrop";
import { art, button, closeSheet, el, isCompact, isTouch, openSheet, toast, tooltip } from "../kit";
import { specInfo } from "../specs";

export interface SkillsDeps {
  skills: SkillInfo[];
  branches: SkillBranch[];
  level(): number;
  progress(): Progress | null;
  onProgress(fn: () => void): () => void;
  learn(ranks: { skillId: number; rank: number }[]): void;
  /** Põe a skill num atalho (índice opcional = primeiro livre). */
  toHotbar(skillId: number, index?: number): void;
}

export function buildSkills(d: SkillsDeps): HTMLElement {
  const byId = new Map(d.skills.map((s) => [s.id, s]));
  // Árvore da classe (3 ramos) e skills dos dois caminhos da especialização (spec preenchido).
  const classTree = d.skills.filter((s) => !s.spec);
  const specIds = [...new Set(d.skills.filter((s) => s.spec).map((s) => s.spec))];
  const tiers = classTree.reduce((n, s) => Math.max(n, s.tier + 1), 0);
  // Nível que libera cada degrau (o mesmo para as 3 skills dele).
  const tierLevels = Array.from({ length: tiers }, (_, t) => classTree.find((s) => s.tier === t)?.levelReq ?? 1);
  const saved = (): Map<number, number> => new Map((d.progress()?.ranks ?? []).map((r) => [r.skillId, r.rank]));
  /** "Cada nível até o 30 dá 1 ponto de skill." (o último nível com ponto vem do servidor). */
  const pointRule = (): string => {
    const p = d.progress();
    const last = p?.skillPointMaxLevel ?? 0;
    if (!last || last >= (p?.maxLevel ?? 0)) return "Cada nível dá 1 ponto de skill.";
    return `Cada nível até o ${last} dá 1 ponto de skill; depois, só pontos de atributo.`;
  };
  let pending = saved();
  let selected: SkillInfo | null = null;

  const root = el("div", "tree-win");
  const points = el("div", "tree-points");
  const undo = button("Desfazer", "btn btn-sm", () => {
    pending = saved();
    render();
  });
  const confirm = button("Confirmar", "btn btn-gold btn-sm", () => {
    const cur = saved();
    const changes = [...pending].filter(([id, r]) => r !== (cur.get(id) ?? 0)).map(([skillId, rank]) => ({ skillId, rank }));
    if (changes.length) d.learn(changes);
  });
  const head = el("div", "tree-head", el("h3", "title tree-title", "Sua árvore"), points, el("div", "tree-actions", undo, confirm));
  const tree = el("div", "tree-grid");
  const detail = el("aside", "tree-detail");
  const note = el("p", "tree-note small muted");
  root.append(head, el("div", "tree-body", tree, detail), note);

  const rank = (s: SkillInfo) => pending.get(s.id) ?? 0;
  const learned = (s: SkillInfo) => (saved().get(s.id) ?? 0) > 0;
  const totalFree = () => {
    const p = d.progress();
    if (!p) return 0;
    const used = [...saved().values()].reduce((a, b) => a + b, 0);
    return p.freeSkillPoints + used; // pontos totais
  };
  const free = () => totalFree() - [...pending.values()].reduce((a, b) => a + b, 0);
  const parent = (s: SkillInfo) => (s.parentId ? byId.get(s.parentId) : undefined);
  const child = (s: SkillInfo) => d.skills.find((x) => x.parentId === s.id);
  const rankInfo = (s: SkillInfo, r: number) => s.ranks[Math.max(1, Math.min(r, s.ranks.length)) - 1];

  function blocker(s: SkillInfo): string | null {
    if (rank(s) >= s.maxRank) return "Rank máximo.";
    if (d.level() < s.levelReq) return `Requer nível ${s.levelReq}.`;
    const p = parent(s);
    if (p && rank(p) < 1) return `Requer ${p.name}.`;
    if (free() <= 0) return "Sem pontos de skill.";
    return null;
  }

  function canLower(s: SkillInfo): boolean {
    const r = rank(s);
    if (r <= (saved().get(s.id) ?? 0)) return false; // só desfaz o que está pendente
    const c = child(s);
    return !(r === 1 && c && rank(c) > 0);
  }

  function raise(s: SkillInfo) {
    const why = blocker(s);
    if (why) return toast(why, "warn");
    pending.set(s.id, rank(s) + 1);
    selected = s;
    render();
  }

  function lower(s: SkillInfo) {
    if (!canLower(s)) return;
    const r = rank(s) - 1;
    if (r > 0) pending.set(s.id, r);
    else pending.delete(s.id);
    render();
  }

  function tipHtml(s: SkillInfo): string {
    const r = rank(s);
    const cur = rankInfo(s, r || 1);
    const meta = s.passive
      ? `<div class="meta"><span><b>Passiva</b></span></div>`
      : `<div class="meta"><span>Mana <b>${cur.mpCost}</b></span><span>Recarga <b>${formatSeconds(cur.cooldownMs)} s</b></span>${s.range ? `<span>${s.target === 3 ? "Raio" : "Alcance"} <b>${formatSeconds(s.range * 1000)} m</b></span>` : ""}</div>`;
    let html = `<h4>${s.name}</h4><div class="sub">${s.passive ? "Passiva" : "Ativa"} · Rank ${r}/${s.maxRank}</div>${meta}<div>${cur.text}</div>`;
    if (s.description) html += `<div class="desc">${s.description}</div>`;
    const why = blocker(s);
    html += `<div class="cmp">`;
    if (r > 0 && r < s.maxRank) html += `<div class="muted">Próximo rank: ${rankInfo(s, r + 1).text}</div>`;
    html += why ? `<div class="bad">${why}</div>` : `<div class="ok">${isTouch() ? "Toque para ver" : "Clique para aprender"}</div>`;
    if (learned(s) && !s.passive) html += `<div class="muted">Arraste para a barra de atalhos</div>`;
    return html + `</div>`;
  }

  function nodeEl(s: SkillInfo, color: string): HTMLElement {
    const r = rank(s);
    const pend = r > (saved().get(s.id) ?? 0);
    const state = r > 0 ? "learned" : blocker(s) === null ? "available" : "locked";
    const b = el("button", `tnode ${s.passive ? "passive" : "active"} ${state}${pend ? " pending" : ""}${selected?.id === s.id ? " sel" : ""}`);
    b.type = "button";
    b.style.setProperty("--bc", color);
    b.append(art(uniqueIds(skillIcon(s.fx)), "tn-icon"), el("span", "tn-rank", `${r}/${s.maxRank}`));
    if (state === "locked" && d.level() < s.levelReq) b.append(art(ui("lock", 12), "tn-lock"));
    b.setAttribute("aria-label", `${s.name}, rank ${r} de ${s.maxRank}`);
    tooltip(b, () => tipHtml(s));
    b.onclick = () => {
      selected = s;
      if (isTouch() && isCompact()) return details(s);
      raise(s);
    };
    b.oncontextmenu = (e) => {
      e.preventDefault();
      lower(s);
    };
    if (learned(s) && !s.passive) {
      b.addEventListener("pointerdown", (e) => {
        if (isTouch()) return;
        beginDrag(e, {
          icon: uniqueIds(skillIcon(s.fx)),
          onDrop(t) {
            const hb = t?.closest<HTMLElement>("[data-hotbar]");
            if (hb) d.toHotbar(s.id, Number(hb.dataset.hotbar));
          },
        });
      });
    }
    return el("div", "tn-wrap", b, el("span", "tn-name", s.name));
  }

  function details(s: SkillInfo) {
    const why = blocker(s);
    const body = el("div", "ts-body");
    body.innerHTML = tipHtml(s).replace(/<h4>.*?<\/h4><div class="sub">.*?<\/div>/, "");
    const acts = el("div", "is-actions");
    const plus = button("Aprender (+1)", "btn btn-gold", () => {
      closeSheet();
      raise(s);
    });
    plus.disabled = !!why;
    const minus = button("−1", "btn", () => {
      closeSheet();
      lower(s);
    });
    minus.disabled = !canLower(s);
    acts.append(plus, minus);
    if (!s.passive && learned(s)) acts.append(button("Pôr na barra", "btn", () => (closeSheet(), d.toHotbar(s.id))));
    openSheet(
      el(
        "div",
        "tree-sheet",
        el("div", "ts-head", art(uniqueIds(skillIcon(s.fx)), `ts-icon ${s.passive ? "passive" : "active"}`), el("div", null, el("h3", "title", s.name), el("div", "muted small", `${s.passive ? "Passiva" : "Ativa"} · Rank ${rank(s)}/${s.maxRank}`))),
        body,
        acts,
      ),
    );
  }

  function renderDetail() {
    const s = selected;
    detail.replaceChildren();
    if (!s) {
      detail.append(
        el("h3", "title", "Como funciona"),
        el(
          "ul",
          "how small muted",
          el("li", null, pointRule()),
          el("li", null, "Clique numa skill para gastar 1 ponto; botão direito desfaz um ponto ainda não confirmado."),
          el("li", null, `Os degraus liberam nos níveis ${listPt(tierLevels)}, e cada um exige a skill de cima no mesmo ramo.`),
          el("li", null, "Arraste skills ativas para a barra de atalhos."),
        ),
      );
      return;
    }
    const r = rank(s);
    const ranks = el("ol", "td-ranks");
    s.ranks.forEach((ri, i) => ranks.append(el("li", i < r ? "on" : "", ri.text)));
    const cur = rankInfo(s, r || 1);
    detail.append(
      el("div", "td-icon-wrap", art(uniqueIds(skillIcon(s.fx)), `td-icon ${s.passive ? "passive" : "active"}`)),
      el("h3", "title td-name", s.name),
      el("div", "muted small", `${(s.spec ? specInfo(s.spec)?.name : d.branches[s.branch]?.name) ?? ""} · ${s.passive ? "Passiva" : "Ativa"} · Rank ${r}/${s.maxRank}`),
      ranks,
    );
    if (!s.passive) {
      detail.append(
        el(
          "div",
          "td-meta",
          ...[`${cur.mpCost} MP`, `${formatSeconds(cur.cooldownMs)} s de recarga`, s.range ? `${formatSeconds(s.range * 1000)} m` : "", s.splash ? `área de ${formatSeconds(s.splash * 1000)} m` : ""]
            .filter(Boolean)
            .map((t) => el("span", null, t)),
        ),
      );
    }
    const why = blocker(s);
    const plus = button("Aprender", "btn btn-gold btn-sm", () => raise(s));
    plus.disabled = !!why;
    const acts = el("div", "td-actions", plus);
    if (!s.passive && learned(s)) acts.append(button("Pôr no atalho", "btn btn-sm", () => d.toHotbar(s.id)));
    detail.append(acts);
    if (why) detail.append(el("p", "bad small", why));
  }

  function render() {
    tree.replaceChildren(el("div", "tg-corner"));
    d.branches.forEach((b, i) => {
      const spent = classTree.filter((s) => s.branch === i).reduce((a, s) => a + rank(s), 0);
      const col = el("div", "tg-branch", el("b", null, b.name), el("small", null, b.description), el("span", "tg-spent", `${spent} pts`));
      col.style.setProperty("--bc", b.color);
      tree.append(col);
    });
    for (let tier = 0; tier < tiers; tier++) {
      const lvl = tierLevels[tier];
      tree.append(el("div", `tg-level${d.level() < lvl ? " locked" : ""}`, el("span", null, `Nv ${lvl}`)));
      for (let br = 0; br < 3; br++) {
        const s = classTree.find((x) => x.branch === br && x.tier === tier);
        const cell = el("div", "tg-cell");
        if (s) {
          const color = d.branches[br]?.color ?? "#d6b46e";
          const p = parent(s);
          if (p) {
            const line = el("span", `tg-line${rank(p) > 0 ? " lit" : ""}`);
            line.style.setProperty("--bc", color);
            cell.append(line);
          }
          cell.append(nodeEl(s, color));
        }
        tree.append(cell);
      }
    }
    tree.append(specRow());
    const f = free();
    points.replaceChildren(el("span", "label", "Pontos"), el("b", f > 0 ? "gold" : "", String(f)));
    const cur = saved();
    const changed = [...pending].some(([id, r]) => r !== (cur.get(id) ?? 0)) || [...cur].some(([id, r]) => r !== (pending.get(id) ?? 0));
    undo.disabled = !changed;
    confirm.disabled = !changed;
    note.textContent = `Nível ${d.level()}: ${pointRule().toLowerCase()}${changed ? " Há mudanças não confirmadas." : ""}`;
    renderDetail();
  }

  /** Ramo da especialização: as 3 skills do caminho escolhido, lado a lado. */
  function specRow(): HTMLElement {
    const mine = specInfo(d.progress()?.spec);
    const row = el("div", "tg-spec");
    if (!mine) {
      const names = specIds.map((id) => specInfo(id)?.name).filter(Boolean);
      row.classList.add("locked");
      row.append(
        el("b", null, "Especialização"),
        el("small", null, `No nível 20, a missão A Encruzilhada (Mestra Ysolde, na praça) libera um destes caminhos: ${names.join(" ou ")}. Cada um traz um bônus de atributos e 3 skills novas.`),
      );
      return row;
    }
    row.style.setProperty("--bc", mine.color);
    const nodes = el("div", "tg-spec-nodes");
    d.skills
      .filter((s) => s.spec === mine.id)
      .sort((a, b) => a.tier - b.tier)
      .forEach((s, i) => {
        const cell = el("div", "tg-spec-cell", el("span", `tg-spec-lvl${d.level() < s.levelReq ? " locked" : ""}`, `Nv ${s.levelReq}`), nodeEl(s, mine.color));
        if (i > 0) {
          const p = parent(s);
          nodes.append(el("span", `tg-spec-link${p && rank(p) > 0 ? " lit" : ""}`));
        }
        nodes.append(cell);
      });
    const spent = d.skills.filter((s) => s.spec === mine.id).reduce((a, s) => a + rank(s), 0);
    row.append(el("div", "tg-spec-head", el("b", null, mine.name), el("small", null, `${mine.bonusText} · ${mine.description}`), el("span", "tg-spent", `${spent} pts`)), nodes);
    return row;
  }

  render();
  // Resposta do servidor (aprendeu ou recusou): recomeça do que está salvo.
  const off = d.onProgress(() => {
    pending = saved();
    render();
  });
  root.addEventListener("win-close", off);
  return root;
}

/** "1, 5, 10, 15 e 20". */
function listPt(n: number[]): string {
  return n.length < 2 ? n.join("") : `${n.slice(0, -1).join(", ")} e ${n[n.length - 1]}`;
}
