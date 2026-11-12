/**
 * Inventário: boneco de equipamento + mochila 6×8 com abas, busca, ordenação, peso e ouro.
 * Arrastar e soltar (mouse e toque), duplo clique para equipar/usar, menu de contexto e,
 * no celular, toque abre os detalhes num painel inferior.
 */

import { ui } from "../../art";
import type { DerivedId } from "../../data/attributes";
import {
  fits,
  INVENTORY_SIZE,
  itemIcon,
  itemName,
  itemStats,
  ITEMS,
  MAX_WEIGHT,
  RARITY_NAME,
  SLOT_NAME,
  STAT_LABEL,
  statText,
  type Category,
  type EquipSlot,
  type ItemStack,
} from "../../data/items";
import { game } from "../../data/mock";
import { beginDrag } from "../../hud/dragdrop";
import { art, button, closeSheet, contextMenu, el, fmt, isTouch, openSheet, tabs, toast, tooltip, type MenuItem } from "../../kit";

const LEFT: EquipSlot[] = ["head", "chest", "legs", "hands", "feet", "cape"];
const RIGHT: EquipSlot[] = ["neck", "ear1", "ear2", "ring1", "ring2"];
const HANDS: EquipSlot[] = ["weapon", "offhand"];

type Loc = { kind: "bag"; i: number } | { kind: "equip"; slot: EquipSlot };

/** Equipado que seria trocado por este item (para comparação). */
function equippedFor(s: ItemStack): ItemStack | undefined {
  const d = ITEMS[s.id];
  if (!d?.fit) return undefined;
  const eq = game.hero.equip;
  if (d.fit === "ring") return eq.ring1 ?? eq.ring2;
  if (d.fit === "ear") return eq.ear1 ?? eq.ear2;
  return eq[d.fit as EquipSlot];
}

export function itemTip(s: ItemStack, compare = true): HTMLElement {
  const d = ITEMS[s.id];
  const box = el("div");
  box.append(el("h4", `rar-${d.rarity}`, itemName(s)));
  box.append(el("div", "sub", `${RARITY_NAME[d.rarity]} · ${d.type}${d.level ? ` · Nível ${d.level}` : ""}`));
  const stats = itemStats(s);
  if (Object.keys(stats).length) {
    const list = el("div", "stats");
    for (const [k, v] of Object.entries(stats) as [DerivedId, number][]) {
      list.append(el("div", null, el("span", null, STAT_LABEL[k]), el("b", v < 0 ? "bad" : "", statText(k, v))));
    }
    box.append(list);
  }
  if (d.use) box.append(el("div", "ok", d.use.text));
  if (d.desc) box.append(el("div", "desc", `“${d.desc}”`));
  const meta = el("div", "meta", el("span", null, "Peso ", el("b", null, String(d.weight))), el("span", null, "Valor ", el("b", null, `${fmt.format(d.price)} ouro`)));
  if (s.qty > 1) meta.append(el("span", null, "Qtd. ", el("b", null, String(s.qty))));
  box.append(meta);

  const other = compare ? equippedFor(s) : undefined;
  if (other && other.uid !== s.uid) {
    const a = itemStats(s);
    const b = itemStats(other);
    const keys = new Set([...Object.keys(a), ...Object.keys(b)] as DerivedId[]);
    const cmp = el("div", "cmp", el("div", "muted", `Comparado com ${itemName(other)}:`));
    for (const k of keys) {
      const diff = (a[k] ?? 0) - (b[k] ?? 0);
      if (Math.abs(diff) < 1e-6) continue;
      cmp.append(el("div", diff > 0 ? "ok" : "bad", `${diff > 0 ? "▲" : "▼"} ${STAT_LABEL[k]} ${statText(k, diff)}`));
    }
    box.append(cmp);
  }
  if (d.level && d.level > game.hero.level) box.append(el("div", "bad", `Requer nível ${d.level}`));
  return box;
}

export function build(): HTMLElement {
  const h = game.hero;
  const root = el("div", "inv-win");

  // ---------------- Boneco ----------------
  const doll = el("section", "inv-doll");
  const slotEls = new Map<EquipSlot, HTMLButtonElement>();
  const mkEquip = (slot: EquipSlot) => {
    const b = el("button", "slot equip-slot empty-hint");
    b.type = "button";
    b.dataset.equip = slot;
    b.dataset.hint = SLOT_NAME[slot];
    slotEls.set(slot, b);
    return b;
  };
  const colL = el("div", "doll-col", ...LEFT.map(mkEquip));
  const colR = el("div", "doll-col", ...RIGHT.map(mkEquip));
  const figure = el("div", "doll-figure", el("div", "doll-sil"), el("div", "doll-name"));
  const hands = el("div", "doll-hands", ...HANDS.map(mkEquip));
  const summary = el("div", "doll-summary");
  doll.append(el("div", "doll-grid", colL, el("div", "doll-mid", figure, hands), colR), summary);

  // ---------------- Mochila ----------------
  const bagSec = el("section", "inv-bag");
  let cat: "all" | Category = "all";
  let query = "";
  const catTabs = tabs(
    [
      { id: "all", label: "Tudo" },
      { id: "equip", label: "Equipamento" },
      { id: "consumable", label: "Consumíveis" },
      { id: "material", label: "Materiais" },
      { id: "quest", label: "Missão" },
    ],
    "all",
    (id) => {
      cat = id as typeof cat;
      render();
    },
    "tabs inv-tabs",
  );
  const search = el("input", "input inv-search");
  search.placeholder = "Buscar item…";
  search.setAttribute("aria-label", "Buscar item");
  search.oninput = () => {
    query = search.value.trim().toLowerCase();
    render();
  };
  const sortBtn = button("", "btn btn-sm btn-icon", () => sortBag());
  sortBtn.innerHTML = ui("sort", 18);
  sortBtn.title = "Ordenar";
  sortBtn.setAttribute("aria-label", "Ordenar mochila");
  const grid = el("div", "inv-grid");
  const bagEls: HTMLButtonElement[] = [];
  for (let i = 0; i < INVENTORY_SIZE; i++) {
    const b = el("button", "slot bag-slot");
    b.type = "button";
    b.dataset.bag = String(i);
    bagEls.push(b);
    grid.append(b);
  }
  const weight = el("div", "inv-weight", el("span", "label", "Peso"), el("div", "wbar", el("i")), el("span", "wtext"));
  const gold = el("div", "inv-gold");
  const space = el("span", "inv-space muted small");
  bagSec.append(el("div", "inv-tools", search, space, sortBtn), catTabs.root, grid, el("div", "inv-foot", weight, gold));

  // Abas do celular (Mochila / Equipamento)
  const mtabs = tabs(
    [
      { id: "bag", label: "Mochila" },
      { id: "doll", label: "Equipamento" },
    ],
    "bag",
    (id) => (root.dataset.mtab = id),
    "tabs inv-mtabs",
  );
  root.dataset.mtab = "bag";
  root.append(mtabs.root, doll, bagSec);

  // ---------------- Estado ----------------
  const get = (l: Loc): ItemStack | null => (l.kind === "bag" ? h.bag[l.i] : (h.equip[l.slot] ?? null));
  const put = (l: Loc, s: ItemStack | null) => {
    if (l.kind === "bag") h.bag[l.i] = s;
    else if (s) h.equip[l.slot] = s;
    else delete h.equip[l.slot];
  };
  const changed = () => {
    render();
    game.emit("equip");
  };

  /** Move de a → b, trocando se preciso. Valida encaixe de equipamento. */
  function move(a: Loc, b: Loc): boolean {
    const sa = get(a);
    const sb = get(b);
    if (!sa) return false;
    if (b.kind === "equip" && !fits(ITEMS[sa.id].fit, b.slot)) {
      toast(`${ITEMS[sa.id].name} não vai em ${SLOT_NAME[b.slot]}.`, "warn");
      return false;
    }
    if (a.kind === "equip" && sb && !fits(ITEMS[sb.id].fit, a.slot)) {
      toast("Esse item não cabe no lugar do outro.", "warn");
      return false;
    }
    // Empilhar iguais
    if (sb && sb.id === sa.id && a.kind === "bag" && b.kind === "bag" && ITEMS[sa.id].stack) {
      sb.qty += sa.qty;
      put(a, null);
      changed();
      return true;
    }
    put(a, sb);
    put(b, sa);
    changed();
    return true;
  }

  function equip(i: number) {
    const s = h.bag[i];
    if (!s) return;
    const d = ITEMS[s.id];
    if (!d.fit) return;
    if (d.level && d.level > h.level) return toast(`Requer nível ${d.level}.`, "warn");
    let slot: EquipSlot = d.fit as EquipSlot;
    if (d.fit === "ring") slot = !h.equip.ring1 ? "ring1" : !h.equip.ring2 ? "ring2" : "ring1";
    if (d.fit === "ear") slot = !h.equip.ear1 ? "ear1" : !h.equip.ear2 ? "ear2" : "ear1";
    if (move({ kind: "bag", i }, { kind: "equip", slot })) toast(`${itemName(s)} equipado.`);
  }

  function unequip(slot: EquipSlot) {
    const free = h.bag.findIndex((x) => !x);
    if (free < 0) return toast("Mochila cheia.", "bad");
    move({ kind: "equip", slot }, { kind: "bag", i: free });
  }

  function use(i: number) {
    const s = h.bag[i];
    if (!s) return;
    const d = ITEMS[s.id];
    if (!d.use) return;
    s.qty--;
    if (s.qty <= 0) h.bag[i] = null;
    toast(`${d.name}: ${d.use.text}`);
    changed();
    game.emit("bag");
  }

  function destroy(l: Loc) {
    const s = get(l);
    if (!s) return;
    put(l, null);
    toast(`${itemName(s)} destruído.`, "bad");
    changed();
  }

  function split(i: number) {
    const s = h.bag[i];
    const free = h.bag.findIndex((x) => !x);
    if (!s || s.qty < 2 || free < 0) return;
    const half = Math.floor(s.qty / 2);
    s.qty -= half;
    h.bag[free] = { uid: Date.now(), id: s.id, qty: half };
    changed();
  }

  function toHotbar(id: string, at?: number) {
    const bar = h.hotbar;
    const i = at ?? bar.findIndex((x, k) => !x && k >= 6);
    const slot = i >= 0 ? i : bar.length - 1;
    bar[slot] = `item:${id}`;
    game.emit("hotbar");
    toast(`${ITEMS[id].name} na tecla ${slot === 9 ? 0 : slot + 1}.`);
  }

  function sortBag() {
    const order: Category[] = ["equip", "consumable", "material", "quest"];
    const rar = ["legend", "epic", "rare", "uncommon", "common"];
    const items = h.bag.filter((x): x is ItemStack => !!x);
    items.sort((a, b) => {
      const da = ITEMS[a.id];
      const db = ITEMS[b.id];
      return order.indexOf(da.category) - order.indexOf(db.category) || rar.indexOf(da.rarity) - rar.indexOf(db.rarity) || da.name.localeCompare(db.name);
    });
    h.bag.fill(null);
    items.forEach((s, i) => (h.bag[i] = s));
    changed();
  }

  function actions(l: Loc): MenuItem[] {
    const s = get(l);
    if (!s) return [];
    const d = ITEMS[s.id];
    const list: MenuItem[] = [];
    if (l.kind === "equip") list.push({ label: "Desequipar", run: () => unequip(l.slot) });
    else {
      if (d.fit) list.push({ label: "Equipar", run: () => equip(l.i) });
      if (d.use) list.push({ label: "Usar", run: () => use(l.i) });
      if (d.use && (d.use.hp || d.use.mp)) list.push({ label: "Colocar na barra de atalhos", run: () => toHotbar(s.id) });
      if (s.qty > 1) list.push({ label: "Dividir pilha", run: () => split(l.i) });
    }
    if (d.category !== "quest") list.push({ label: "Destruir", danger: true, run: () => destroy(l) });
    return list;
  }

  function details(l: Loc) {
    const s = get(l);
    if (!s) return;
    const wrap = el("div", "item-sheet");
    const head = el("div", "is-head", el("div", `slot`, art(itemIcon(s.id))), itemTip(s));
    head.querySelector(".slot")!.setAttribute("data-rarity", ITEMS[s.id].rarity);
    const acts = el("div", "is-actions");
    for (const a of actions(l)) acts.append(button(a.label, `btn ${a.danger ? "btn-danger" : a.label === "Equipar" || a.label === "Usar" ? "btn-gold" : ""}`, () => {
      closeSheet();
      a.run();
    }));
    wrap.append(head, acts);
    openSheet(wrap);
  }

  // ---------------- Eventos dos slots ----------------
  const locOf = (e: HTMLElement | null): Loc | null => {
    const s = e?.closest<HTMLElement>("[data-bag], [data-equip], [data-hotbar]");
    if (!s) return null;
    if (s.dataset.bag !== undefined) return { kind: "bag", i: Number(s.dataset.bag) };
    if (s.dataset.equip) return { kind: "equip", slot: s.dataset.equip as EquipSlot };
    return null;
  };

  const wire = (b: HTMLButtonElement, l: Loc) => {
    tooltip(b, () => {
      const s = get(l);
      return s ? itemTip(s) : null;
    });
    b.addEventListener("dblclick", () => {
      if (l.kind === "equip") return unequip(l.slot);
      const s = get(l);
      if (!s) return;
      if (ITEMS[s.id].fit) equip(l.i);
      else if (ITEMS[s.id].use) use(l.i);
    });
    b.addEventListener("click", () => {
      if (!isTouch() || !get(l)) return;
      details(l);
    });
    b.addEventListener("contextmenu", (e) => {
      e.preventDefault();
      const a = actions(l);
      if (a.length) contextMenu(a, e.clientX, e.clientY);
    });
    b.addEventListener("pointerdown", (e) => {
      const s = get(l);
      if (!s) return;
      let over: HTMLElement | null = null;
      beginDrag(e, {
        icon: itemIcon(s.id),
        onOver(t) {
          over?.classList.remove("drop-ok", "drop-bad");
          over = t?.closest<HTMLElement>("[data-bag], [data-equip], [data-hotbar]") ?? null;
          if (!over) return;
          const dest = locOf(over);
          const ok = over.dataset.hotbar !== undefined ? !!ITEMS[s.id].use?.hp || !!ITEMS[s.id].use?.mp : dest?.kind === "equip" ? fits(ITEMS[s.id].fit, dest.slot) : true;
          over.classList.add(ok ? "drop-ok" : "drop-bad");
        },
        onDrop(t) {
          over?.classList.remove("drop-ok", "drop-bad");
          const hb = t?.closest<HTMLElement>("[data-hotbar]");
          if (hb) {
            const d = ITEMS[s.id];
            if (d.use?.hp || d.use?.mp) toHotbar(s.id, Number(hb.dataset.hotbar));
            else toast("Só consumíveis vão na barra de atalhos.", "warn");
            return;
          }
          const dest = locOf(t);
          if (dest) move(l, dest);
        },
      });
    });
  };
  bagEls.forEach((b, i) => wire(b, { kind: "bag", i }));
  slotEls.forEach((b, slot) => wire(b, { kind: "equip", slot }));

  // ---------------- Desenho ----------------
  function paint(b: HTMLButtonElement, s: ItemStack | null) {
    b.querySelectorAll(".art, .qty, .ench").forEach((x) => x.remove());
    if (!s) {
      b.removeAttribute("data-rarity");
      b.setAttribute("aria-label", b.dataset.hint ? `${b.dataset.hint} vazio` : "Vazio");
      return;
    }
    const d = ITEMS[s.id];
    b.dataset.rarity = d.rarity;
    b.append(art(itemIcon(s.id)));
    if (s.qty > 1) b.append(el("span", "qty", String(s.qty)));
    if (s.ench) b.append(el("span", "ench", `+${s.ench}`));
    b.setAttribute("aria-label", itemName(s));
  }

  function render() {
    let w = 0;
    bagEls.forEach((b, i) => {
      const s = h.bag[i];
      paint(b, s);
      if (s) w += ITEMS[s.id].weight * s.qty;
      const match = !s || ((cat === "all" || ITEMS[s.id].category === cat) && (!query || itemName(s).toLowerCase().includes(query)));
      b.classList.toggle("dim", !match);
    });
    for (const [slot, b] of slotEls) {
      const s = h.equip[slot] ?? null;
      paint(b, s);
      b.classList.toggle("empty-hint", !s);
      if (s) w += ITEMS[s.id].weight;
    }
    const frac = w / MAX_WEIGHT;
    weight.style.setProperty("--w", `${Math.min(100, frac * 100)}%`);
    weight.classList.toggle("heavy", frac > 0.8);
    weight.querySelector(".wtext")!.textContent = `${fmt.format(w)} / ${fmt.format(MAX_WEIGHT)}`;
    gold.innerHTML = `<span class="coin"></span><b>${fmt.format(h.gold)}</b><span class="muted small">ouro</span>`;
    space.textContent = `Espaço ${h.bag.filter(Boolean).length}/${INVENTORY_SIZE}`;

    // Resumo do boneco
    const st = game.stats();
    figure.querySelector(".doll-name")!.textContent = `${h.name} · Nv ${h.level}`;
    summary.replaceChildren(
      ...(
        [
          ["Atq. físico", Math.round(st.pAtk)],
          ["Atq. mágico", Math.round(st.mAtk)],
          ["Def. física", Math.round(st.pDef)],
          ["Def. mágica", Math.round(st.mDef)],
        ] as const
      ).map(([k, v]) => el("div", "sum", el("span", "muted", k), el("b", null, fmt.format(v)))),
    );
  }

  render();
  const off = game.onChange((w) => {
    if (w === "bag" || w === "xp") render();
  });
  // Para de ouvir quando a janela fecha.
  root.addEventListener("win-close", () => {
    off();
  });
  return root;
}
