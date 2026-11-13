/**
 * Inventário (tecla I): boneco com o equipamento e a miniatura 3D, bolsa com abas e
 * busca, moedas e espaço. Arrastar e soltar move/equipa/vende/põe na barra; duplo clique
 * usa ou equipa; botão direito abre o menu; no celular, toque abre os detalhes.
 * O servidor decide tudo: aqui só mandamos os comandos (moveItem, useItem, destroyItem).
 */

import type { ClassId } from "../../api";
import { ItemKind, type ItemInfo, type SelfStats } from "../../gen/erevia/v1/game_pb";
import { ui, uniqueIds } from "../art";
import { beginDrag } from "../dragdrop";
import { buildItemTip, itemIcon, PLACE_BAG, PLACE_EQUIP, type ItemStore, type Stack } from "../items";
import { art, button, closeSheet, contextMenu, el, fmt, isTouch, openSheet, tabs, toast, tooltip, type MenuItem } from "../kit";
import { createPreview } from "../preview3d";

export interface InventoryDeps {
  store: ItemStore;
  me(): { myClass: string; myLevel: number; name: string; emblem: string };
  stats(): SelfStats | null;
  onStats(fn: () => void): () => void;
  move(fromPlace: number, fromSlot: number, toPlace: number, toSlot: number): void;
  use(place: number, slot: number): void;
  destroy(slot: number, count: number): void;
  /** Põe uma poção num atalho (índice opcional). */
  toHotbar(itemId: string, index?: number): void;
  /** Vender (só com a loja aberta). */
  sell(slot: number): boolean;
  shopOpen(): boolean;
}

/** Encaixes reais (iguais aos do servidor) e os que ainda vão chegar. */
const REAL_SLOTS = [
  { slot: 3, name: "Elmo" },
  { slot: 2, name: "Armadura" },
];
const HAND_SLOTS = [
  { slot: 0, name: "Arma" },
  { slot: 1, name: "Mão secundária" },
];
const SOON = ["Pernas", "Mãos", "Pés", "Capa"];
const SOON_RIGHT = ["Colar", "Brinco", "Brinco", "Anel", "Anel"];

type Cat = "all" | "equip" | "potion" | "material";
const CAT_OF = (i: ItemInfo): Cat =>
  i.kind === ItemKind.POTION ? "potion" : i.kind === ItemKind.MATERIAL ? "material" : i.slot >= 0 ? "equip" : "all";

type Loc = { place: number; slot: number };

export function buildInventory(d: InventoryDeps): HTMLElement {
  const { store } = d;
  const root = el("div", "inv-win");

  // ---------- Boneco ----------
  const doll = el("section", "inv-doll");
  const equipEls = new Map<number, HTMLButtonElement>();
  const mkEquip = (slot: number, name: string) => {
    const b = el("button", "slot equip-slot empty-hint");
    b.type = "button";
    b.dataset.equip = String(slot);
    b.dataset.hint = name;
    equipEls.set(slot, b);
    return b;
  };
  const mkSoon = (name: string) => {
    const b = el("span", "slot equip-slot empty-hint soon");
    b.dataset.hint = name;
    b.append(art(ui("lock", 14), "soon-lock"));
    tooltip(b, () => `<b>${name}</b><div class="muted">Encaixe novo chegando numa próxima atualização.</div>`);
    return b;
  };
  const dollCanvas = el("canvas", "doll-canvas");
  dollCanvas.setAttribute("aria-label", "Seu personagem — arraste para girar");
  const figure = el("div", "doll-figure", el("div", "doll-stage", dollCanvas), el("div", "doll-name"));
  const colL = el("div", "doll-col", ...REAL_SLOTS.map((s) => mkEquip(s.slot, s.name)), ...SOON.map(mkSoon));
  const colR = el("div", "doll-col", ...SOON_RIGHT.map(mkSoon));
  const hands = el("div", "doll-hands", ...HAND_SLOTS.map((s) => mkEquip(s.slot, s.name)));
  const summary = el("div", "doll-summary");
  doll.append(el("div", "doll-grid", colL, el("div", "doll-mid", figure, hands), colR), summary);

  // ---------- Bolsa ----------
  const bagSec = el("section", "inv-bag");
  let cat: Cat = "all";
  let query = "";
  const catTabs = tabs(
    [
      { id: "all", label: "Tudo" },
      { id: "equip", label: "Equipamento" },
      { id: "potion", label: "Poções" },
      { id: "material", label: "Materiais" },
    ],
    "all",
    (id) => {
      cat = id as Cat;
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
  // Teclas digitadas na busca não podem acionar skills.
  search.addEventListener("keydown", (e) => e.stopPropagation());
  const space = el("span", "inv-space muted small");
  const grid = el("div", "inv-grid");
  const bagEls: HTMLButtonElement[] = [];
  const gold = el("div", "inv-gold");
  const hint = el("span", "muted small inv-hint", isTouch() ? "Toque num item para ver as opções" : "Duplo clique usa/equipa · arraste para a barra de atalhos");
  bagSec.append(el("div", "inv-tools", search, space), catTabs.root, grid, el("div", "inv-foot", hint, gold));

  const mtabs = tabs(
    [
      { id: "bag", label: "Bolsa" },
      { id: "doll", label: "Equipamento" },
    ],
    "bag",
    (id) => (root.dataset.mtab = id),
    "tabs inv-mtabs",
  );
  root.dataset.mtab = "bag";
  root.append(mtabs.root, doll, bagSec);

  // ---------- 3D ----------
  const me0 = d.me();
  figure.style.setProperty("--emblem", me0.emblem);
  const pv = createPreview(dollCanvas, { radius: 3.7 });
  let lastLook = "";
  const showModel = () => {
    const w = store.equip[0]?.info;
    const o = store.equip[1]?.info;
    const look = { right: w?.model || null, left: o?.model || w?.offModel || null, emblem: me0.emblem };
    const key = JSON.stringify(look);
    if (key === lastLook) return;
    lastLook = key;
    void pv.show(me0.myClass as ClassId, look);
  };

  // ---------- Ações ----------
  const get = (l: Loc): Stack | null => (l.place === PLACE_EQUIP ? store.equip[l.slot] : store.bag[l.slot]) ?? null;
  const isPotion = (i: ItemInfo) => !!(i.healHp || i.healMp);

  const actions = (l: Loc): MenuItem[] => {
    const s = get(l);
    if (!s) return [];
    const list: MenuItem[] = [];
    if (l.place === PLACE_EQUIP) list.push({ label: "Desequipar", run: () => d.use(l.place, l.slot) });
    else {
      if (s.info.slot >= 0) list.push({ label: "Equipar", run: () => d.use(l.place, l.slot) });
      if (s.info.kind === ItemKind.POTION) list.push({ label: "Usar", run: () => d.use(l.place, l.slot) });
      if (isPotion(s.info)) list.push({ label: "Pôr na barra de atalhos", run: () => d.toHotbar(s.info.id) });
      if (d.shopOpen() && s.info.sellPrice > 0) list.push({ label: `Vender (${fmt.format(s.info.sellPrice * s.count)} moedas)`, run: () => d.sell(l.slot) });
      list.push({ label: "Destruir…", danger: true, run: () => confirmDestroy(l, s) });
    }
    return list;
  };

  const confirmDestroy = (l: Loc, s: Stack) => {
    const r = (l.place === PLACE_BAG ? bagEls[l.slot] : equipEls.get(l.slot))?.getBoundingClientRect();
    contextMenu(
      [
        { label: `Destruir ${s.count > 1 ? `${s.count}× ` : ""}${s.info.name}`, danger: true, run: () => d.destroy(l.slot, s.count) },
        { label: "Cancelar", run: () => undefined },
      ],
      (r?.left ?? innerWidth / 2) + 20,
      (r?.top ?? innerHeight / 2) + 20,
    );
  };

  const details = (l: Loc) => {
    const s = get(l);
    if (!s) return;
    const slotIcon = el("div", "slot", art(uniqueIds(itemIcon(s.info))));
    slotIcon.dataset.grade = String(s.info.grade);
    const acts = el("div", "is-actions");
    for (const a of actions(l)) {
      const b = button(a.label, `btn ${a.danger ? "btn-danger" : a.label === "Equipar" || a.label === "Usar" ? "btn-gold" : ""}`, () => {
        closeSheet();
        a.run();
      });
      acts.append(b);
    }
    openSheet(el("div", "item-sheet", el("div", "is-head", slotIcon, buildItemTip(s.info, { ...d.me(), count: s.count })), acts));
  };

  const locOf = (t: HTMLElement | null): Loc | null => {
    const s = t?.closest<HTMLElement>("[data-bag], [data-equip]");
    if (!s) return null;
    if (s.dataset.bag !== undefined) return { place: PLACE_BAG, slot: Number(s.dataset.bag) };
    return { place: PLACE_EQUIP, slot: Number(s.dataset.equip) };
  };

  const wire = (b: HTMLButtonElement, l: Loc) => {
    tooltip(b, () => {
      const s = get(l);
      if (!s) return null;
      const hintText = l.place === PLACE_EQUIP ? "Duplo clique para desequipar" : s.info.slot >= 0 ? "Duplo clique para equipar" : s.info.kind === ItemKind.POTION ? "Duplo clique para usar" : "";
      return buildItemTip(s.info, { ...d.me(), count: s.count, price: d.shopOpen() ? "sell" : undefined, hint: hintText });
    });
    b.addEventListener("dblclick", () => {
      if (get(l)) d.use(l.place, l.slot);
    });
    b.addEventListener("click", () => {
      if (isTouch() && get(l)) details(l);
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
        icon: uniqueIds(itemIcon(s.info)),
        onOver(t) {
          over?.classList.remove("drop-ok", "drop-bad");
          over = t?.closest<HTMLElement>("[data-bag], [data-equip], [data-hotbar], [data-drop='sell']") ?? null;
          if (!over) return;
          let ok = true;
          if (over.dataset.hotbar !== undefined) ok = isPotion(s.info);
          else if (over.dataset.equip !== undefined) ok = s.info.slot === Number(over.dataset.equip);
          else if (over.dataset.drop === "sell") ok = l.place === PLACE_BAG && s.info.sellPrice > 0;
          over.classList.add(ok ? "drop-ok" : "drop-bad");
        },
        onDrop(t) {
          over?.classList.remove("drop-ok", "drop-bad");
          const hb = t?.closest<HTMLElement>("[data-hotbar]");
          if (hb) {
            if (isPotion(s.info)) d.toHotbar(s.info.id, Number(hb.dataset.hotbar));
            else toast("Só poções vão na barra de atalhos.", "warn");
            return;
          }
          if (t?.closest("[data-drop='sell']")) {
            if (l.place === PLACE_BAG && !d.sell(l.slot)) toast("Esse item não pode ser vendido.", "warn");
            return;
          }
          const dest = locOf(t);
          if (!dest || (dest.place === l.place && dest.slot === l.slot)) return;
          if (dest.place === PLACE_EQUIP && s.info.slot !== dest.slot) {
            toast(`${s.info.name} não vai nesse encaixe.`, "warn");
            return;
          }
          d.move(l.place, l.slot, dest.place, dest.slot);
        },
      });
    });
  };

  // ---------- Desenho ----------
  const paint = (b: HTMLButtonElement, s: Stack | null) => {
    b.querySelectorAll(".art, .qty").forEach((x) => x.remove());
    if (!s) {
      b.removeAttribute("data-grade");
      b.setAttribute("aria-label", b.dataset.hint ? `${b.dataset.hint} vazio` : "Vazio");
      return;
    }
    b.dataset.grade = String(s.info.grade);
    b.append(art(uniqueIds(itemIcon(s.info))));
    if (s.count > 1) b.append(el("span", "qty", String(s.count)));
    b.setAttribute("aria-label", s.count > 1 ? `${s.count}× ${s.info.name}` : s.info.name);
  };

  function render() {
    // A bolsa pode mudar de tamanho (vem do servidor).
    while (bagEls.length < store.bag.length) {
      const i = bagEls.length;
      const b = el("button", "slot bag-slot");
      b.type = "button";
      b.dataset.bag = String(i);
      wire(b, { place: PLACE_BAG, slot: i });
      bagEls.push(b);
      grid.append(b);
    }
    bagEls.forEach((b, i) => {
      const s = store.bag[i] ?? null;
      paint(b, s);
      const match = !s || ((cat === "all" || CAT_OF(s.info) === cat) && (!query || s.info.name.toLowerCase().includes(query)));
      b.classList.toggle("dim", !match);
    });
    for (const [slot, b] of equipEls) {
      const s = store.equip[slot] ?? null;
      paint(b, s);
      b.classList.toggle("empty-hint", !s);
    }
    space.textContent = `Espaço ${store.bag.filter(Boolean).length}/${store.bag.length}`;
    gold.innerHTML = `<span class="coin"></span><b>${fmt.format(store.gold)}</b><span class="muted small">moedas</span>`;
    const me = d.me();
    figure.querySelector(".doll-name")!.textContent = `${me.name} · Nv ${me.myLevel}`;
    const st = d.stats();
    summary.replaceChildren(
      ...(
        [
          ["Atq. físico", st?.pAtk],
          ["Atq. mágico", st?.mAtk],
          ["Def. física", st?.pDef],
          ["Def. mágica", st?.mDef],
        ] as const
      ).map(([k, v]) => el("div", "sum", el("span", "muted", k), el("b", null, v === undefined ? "–" : fmt.format(Math.round(v))))),
    );
    showModel();
  }

  equipEls.forEach((b, slot) => wire(b, { place: PLACE_EQUIP, slot }));
  render();
  const offStore = store.onChange(render);
  const offStats = d.onStats(render);
  root.addEventListener("win-close", () => {
    offStore();
    offStats();
    pv.dispose();
  });
  return root;
}
