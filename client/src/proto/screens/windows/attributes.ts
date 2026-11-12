/**
 * Personagem e atributos com pontos livres: modelo 3D, 6 atributos com +/−, prévia da
 * diferença nos atributos de combate antes de confirmar, desfazer e sugestão da classe.
 */

import { classInfo } from "../../../classes";
import { ATTRS, CLASS_BASE_ATTRS, CLASS_SUGGEST, DERIVED, derive, type AttrId, type Attrs } from "../../data/attributes";
import { game, xpNeeded } from "../../data/mock";
import { bar, button, el, fmt, tooltip, toast } from "../../kit";
import { createPreview } from "../../preview3d";

export function build(): HTMLElement {
  const h = game.hero;
  const info = classInfo(h.cls);
  let draft: Attrs = { str: 0, dex: 0, con: 0, int: 0, wis: 0, spi: 0 };
  const root = el("div", "attr-win");

  // ---------- Retrato 3D ----------
  const portrait = el("section", "attr-portrait");
  const canvas = el("canvas", "attr-canvas");
  const pv = createPreview(canvas, { radius: 5.2 });
  void pv.show(h.cls, h.look);
  const xp = bar("xp");
  xp.set(h.xp, xpNeeded(h.level), `${fmt.format(h.xp)} / ${fmt.format(xpNeeded(h.level))} XP`);
  portrait.append(
    el("div", "ap-stage", canvas),
    el("div", "ap-info", el("h3", "title ap-name", h.name), el("div", "ap-sub", `${info.label} · Nível ${h.level}`), xp.root),
  );
  portrait.style.setProperty("--emblem", h.look.emblem);

  // ---------- Atributos base ----------
  const attrs = el("section", "attr-base");
  const pointsEl = el("div", "attr-points");
  const rows = new Map<AttrId, { val: HTMLElement; plus: HTMLButtonElement; minus: HTMLButtonElement; add: HTMLElement; fill: HTMLElement }>();
  const list = el("div", "attr-list");
  for (const a of ATTRS) {
    const val = el("span", "a-val");
    const add = el("span", "a-add");
    const fill = el("i");
    const minus = button("−", "a-btn", () => change(a.id, -1));
    const plus = button("+", "a-btn plus", () => change(a.id, 1));
    minus.setAttribute("aria-label", `Tirar ponto de ${a.name}`);
    plus.setAttribute("aria-label", `Pôr ponto em ${a.name}`);
    // Segurar o + adiciona vários.
    let hold = 0;
    plus.addEventListener("pointerdown", () => (hold = window.setTimeout(function rep() {
      change(a.id, 1);
      hold = window.setTimeout(rep, 90);
    }, 380)));
    for (const ev of ["pointerup", "pointerleave", "pointercancel"]) plus.addEventListener(ev, () => clearTimeout(hold));
    const name = el("div", "a-name", el("span", "a-short", a.short), el("span", null, a.name));
    name.style.setProperty("--ac", a.color);
    tooltip(name, () => `<h4 style="color:${a.color}">${a.name}</h4><div class="desc">${a.desc}</div><div class="cmp">${a.affects}</div>`);
    const row = el("div", "a-row", name, el("div", "a-meter", fill), minus, el("span", "a-num", val, add), plus);
    row.style.setProperty("--ac", a.color);
    rows.set(a.id, { val, plus, minus, add, fill });
    list.append(row);
  }
  const undo = button("Desfazer", "btn btn-sm", () => {
    draft = { str: 0, dex: 0, con: 0, int: 0, wis: 0, spi: 0 };
    render();
  });
  const suggest = button("Sugestão da classe", "btn btn-sm", () => {
    let left = game.attrPointsFree() - sum(draft);
    const w = CLASS_SUGGEST[h.cls];
    const order = (Object.entries(w) as [AttrId, number][]).flatMap(([k, n]) => Array(n).fill(k) as AttrId[]);
    let i = 0;
    while (left-- > 0) draft[order[i++ % order.length]]++;
    render();
  });
  const confirm = button("Confirmar", "btn btn-gold btn-sm", () => {
    for (const a of ATTRS) h.spent[a.id] += draft[a.id];
    draft = { str: 0, dex: 0, con: 0, int: 0, wis: 0, spi: 0 };
    game.emit("attrs");
    toast("Atributos distribuídos!");
    render();
  });
  attrs.append(
    el("div", "attr-head", el("h3", "title", "Atributos"), pointsEl),
    list,
    el("div", "attr-actions", undo, suggest, confirm),
    el("p", "small muted attr-note", "Cada nível dá 3 pontos. A base vem da classe; os pontos somam em cima."),
  );

  // ---------- Atributos de combate ----------
  const derived = el("section", "attr-derived");
  const dRows = new Map<string, { val: HTMLElement; diff: HTMLElement }>();
  let group = "";
  for (const d of DERIVED) {
    if (d.group !== group) {
      group = d.group;
      derived.append(el("h4", "d-group", group));
    }
    const val = el("b", "d-val");
    const diff = el("span", "d-diff");
    const row = el("div", "d-row", el("span", "d-name", d.name), diff, val);
    tooltip(row, () => {
      const noGear = derive(h.cls, h.level, current(), {});
      const full = derive(h.cls, h.level, current(), game.gearBonus());
      const g = full[d.id] - noGear[d.id];
      return `<h4>${d.name}</h4><div class="stats"><div><span>Classe e atributos</span><b>${d.fmt(noGear[d.id])}</b></div><div><span>Equipamento</span><b class="${g >= 0 ? "ok" : "bad"}">${g >= 0 ? "+" : ""}${d.fmt(g)}</b></div></div>`;
    });
    dRows.set(d.id, { val, diff });
    derived.append(row);
  }

  root.append(portrait, attrs, el("div", "attr-right", el("h3", "title", "Combate"), derived));

  const sum = (a: Attrs) => Object.values(a).reduce((x, y) => x + y, 0);
  const current = (): Attrs => game.attrs();

  function change(id: AttrId, d: number) {
    const free = game.attrPointsFree() - sum(draft);
    if (d > 0 && free <= 0) return;
    if (d < 0 && draft[id] <= 0) return;
    draft[id] += d;
    render();
  }

  function render() {
    const base = current();
    const free = game.attrPointsFree() - sum(draft);
    pointsEl.replaceChildren(el("span", "label", "Pontos livres"), el("b", free > 0 ? "gold pulse" : "", String(free)));
    const max = Math.max(...ATTRS.map((a) => base[a.id] + draft[a.id]), 30);
    for (const a of ATTRS) {
      const r = rows.get(a.id)!;
      const v = base[a.id] + draft[a.id];
      r.val.textContent = String(v);
      r.add.textContent = draft[a.id] ? ` +${draft[a.id]}` : "";
      r.plus.disabled = free <= 0;
      r.minus.disabled = draft[a.id] <= 0;
      r.fill.style.width = `${(v / max) * 100}%`;
      r.fill.style.setProperty("--base", `${((CLASS_BASE_ATTRS[h.cls][a.id]) / v) * 100}%`);
    }
    const gear = game.gearBonus();
    const now = derive(h.cls, h.level, base, gear);
    const next = derive(h.cls, h.level, { ...Object.fromEntries(ATTRS.map((a) => [a.id, base[a.id] + draft[a.id]])) } as Attrs, gear);
    for (const d of DERIVED) {
      const r = dRows.get(d.id)!;
      r.val.textContent = d.fmt(next[d.id]);
      const diff = next[d.id] - now[d.id];
      r.diff.textContent = Math.abs(diff) > d.eps ? `${diff > 0 ? "▲" : "▼"} ${d.fmt(Math.abs(diff))}` : "";
      r.diff.className = `d-diff ${diff > 0 ? "ok" : "bad"}`;
      r.val.classList.toggle("changed", Math.abs(diff) > d.eps);
    }
    const dirty = sum(draft) > 0;
    undo.disabled = !dirty;
    confirm.disabled = !dirty;
    suggest.disabled = free <= 0;
  }

  render();
  const off = game.onChange((w) => {
    if (w === "equip" || w === "xp") render();
  });
  root.addEventListener("win-close", () => {
    off();
      pv.dispose();
  });
  return root;
}
