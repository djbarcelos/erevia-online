import { el, formatGold, goldIcon, GRADE_COLOR, itemIcon, ItemStore, ItemTip } from "./items";
import { ItemKind, type ItemInfo } from "../gen/erevia/v1/game_pb";

/** Categorias do filtro da aba Comprar (só aparecem as que a loja tem). */
type ShopCat = "all" | "weapon" | "armor" | "jewel" | "potion";
const SHOP_CATS: { id: ShopCat; label: string }[] = [
  { id: "all", label: "Tudo" },
  { id: "weapon", label: "Armas" },
  { id: "armor", label: "Defesa" },
  { id: "jewel", label: "Joias" },
  { id: "potion", label: "Poções" },
];
function shopCat(i: ItemInfo): ShopCat {
  switch (i.kind) {
    case ItemKind.WEAPON:
      return "weapon";
    case ItemKind.NECKLACE:
    case ItemKind.EARRING:
    case ItemKind.RING:
      return "jewel";
    case ItemKind.POTION:
      return "potion";
  }
  return i.slot >= 0 ? "armor" : "all";
}

export interface ShopActions {
  buy(npcId: number, itemId: string, count: number): void;
  sell(npcId: number, slot: number, count: number): void;
  me(): { myClass: string; myLevel: number };
  /** A loja abriu: o inventário abre ao lado (para arrastar e vender). */
  onOpen(): void;
}

/** Janela da loja de um NPC: comprar da lista dele ou vender o que está na bolsa. */
export class ShopWindow {
  readonly root = el("div", "shop-panel frame");
  private title = el("h2");
  private list = el("div", "shop-list");
  private goldEl = el("span", "shop-gold-value");
  private tabs: HTMLButtonElement[] = [];
  private mode: "buy" | "sell" = "buy";
  private stock: ItemInfo[] = [];
  private filters = el("div", "shop-filters");
  private cat: ShopCat = "all";
  /** Esconde equipamentos que a sua classe não usa. */
  private mineOnly = true;
  private unsubscribe: () => void;
  npcId = 0;

  constructor(
    private store: ItemStore,
    private tip: ItemTip,
    private actions: ShopActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    this.root.dataset.drop = "sell"; // soltar um item da bolsa aqui = vender

    const head = el("div", "panel-head");
    const close = el("button", "x-btn", "✕");
    close.type = "button";
    close.title = "Fechar (Esc)";
    close.onclick = () => this.close();
    head.append(this.title, close);

    const tabs = el("div", "shop-tabs");
    for (const [mode, label] of [["buy", "Comprar"], ["sell", "Vender"]] as const) {
      const b = el("button", "shop-tab", label);
      b.type = "button";
      b.onclick = () => this.setMode(mode);
      tabs.append(b);
      this.tabs.push(b);
    }

    const foot = el("div", "shop-foot");
    const gold = el("div", "shop-gold");
    const gi = el("span", "shop-gold-icon");
    gi.innerHTML = goldIcon();
    gold.append(gi, this.goldEl);
    foot.append(gold, el("span", "shop-hint", "Arraste itens da bolsa para cá para vender"));

    this.root.append(head, tabs, this.filters, this.list, foot);
    parent.append(this.root);
    this.unsubscribe = store.onChange(() => this.render());
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  open(npcId: number, title: string, stock: string[]): void {
    this.npcId = npcId;
    this.title.textContent = title;
    this.stock = stock.map((id) => this.store.get(id)).filter((i): i is ItemInfo => !!i);
    this.cat = "all";
    this.root.hidden = false;
    this.setMode("buy");
    this.actions.onOpen();
  }

  close(): void {
    this.root.hidden = true;
    this.npcId = 0;
    this.tip.hide();
  }

  dispose(): void {
    this.unsubscribe();
    this.root.remove();
  }

  setMode(mode: "buy" | "sell"): void {
    this.mode = mode;
    this.tabs.forEach((t, i) => t.classList.toggle("active", (i === 0) === (mode === "buy")));
    this.renderFilters();
    this.render(true);
  }

  /** Filtros da aba Comprar: categorias que a loja tem e "só da minha classe". */
  private renderFilters(): void {
    const cats = SHOP_CATS.filter((c) => c.id === "all" || this.stock.some((i) => shopCat(i) === c.id));
    const hasOtherClass = this.stock.some((i) => i.classes.length > 0 && !i.classes.includes(this.actions.me().myClass));
    this.filters.hidden = this.mode !== "buy" || (cats.length <= 2 && !hasOtherClass);
    if (this.filters.hidden) return;
    const chips = cats.length > 2 ? cats : [];
    this.filters.replaceChildren(
      ...chips.map((c) => {
        const b = el("button", "shop-chip", c.label);
        b.type = "button";
        b.setAttribute("aria-pressed", String(c.id === this.cat));
        b.onclick = () => {
          this.cat = c.id;
          this.renderFilters();
          this.render(true);
        };
        return b;
      }),
    );
    if (hasOtherClass) {
      const label = el("label", "shop-mine");
      const box = el("input");
      box.type = "checkbox";
      box.checked = this.mineOnly;
      box.onchange = () => {
        this.mineOnly = box.checked;
        this.render(true);
      };
      label.append(box, document.createTextNode("Só da minha classe"));
      this.filters.append(label);
    }
  }

  private visible(i: ItemInfo): boolean {
    if (this.cat !== "all" && shopCat(i) !== this.cat) return false;
    return !this.mineOnly || i.classes.length === 0 || i.classes.includes(this.actions.me().myClass);
  }

  private render(force = false): void {
    this.goldEl.textContent = formatGold(this.store.gold);
    if (!this.isOpen) return;
    // Na aba Comprar a lista não muda; só o ouro (e o que dá para pagar).
    if (this.mode === "buy" && !force) {
      for (const row of this.list.querySelectorAll<HTMLElement>(".shop-row")) {
        row.classList.toggle("poor", Number(row.dataset.price) > this.store.gold);
      }
      return;
    }
    this.list.replaceChildren();
    if (this.mode === "buy") {
      for (const info of this.stock) {
        if (this.visible(info)) this.list.append(this.row(info, info.price, info.stack > 1 ? 999 : 1, (n) => this.actions.buy(this.npcId, info.id, n), "Comprar"));
      }
      this.render();
      return;
    }
    let any = false;
    this.store.bag.forEach((s, slot) => {
      if (!s || s.info.sellPrice <= 0) return;
      any = true;
      this.list.append(this.row(s.info, s.info.sellPrice, s.count, (n) => this.actions.sell(this.npcId, slot, n), "Vender", s.count));
    });
    if (!any) this.list.append(el("p", "shop-empty", "Nada na bolsa que a loja compre."));
  }

  private row(info: ItemInfo, price: number, maxQty: number, act: (n: number) => void, verb: string, have = 0): HTMLElement {
    const row = el("div", "shop-row");
    row.dataset.price = String(price);
    const icon = el("div", "shop-icon");
    icon.innerHTML = itemIcon(info);
    if (have > 1) icon.append(el("span", "qty", String(have)));
    const name = el("div", "shop-name");
    const n = el("strong", "", info.name);
    n.style.color = GRADE_COLOR[info.grade] ?? GRADE_COLOR[1];
    const cost = el("span", "shop-price", `${formatGold(price)} moedas`);
    name.append(n, cost);
    row.append(icon, name);

    let qty: HTMLInputElement | null = null;
    if (maxQty > 1) {
      qty = el("input", "shop-qty");
      qty.type = "number";
      qty.min = "1";
      qty.max = String(maxQty);
      qty.value = this.mode === "sell" ? String(maxQty) : "1";
      qty.setAttribute("aria-label", "Quantidade");
      qty.onkeydown = (ev) => {
        ev.stopPropagation(); // números aqui não usam skills
        if (ev.key === "Enter") btn.click();
        if (ev.key === "Escape") qty?.blur();
      };
      row.append(qty);
    }
    const btn = el("button", "btn btn-gold shop-btn", verb);
    btn.type = "button";
    btn.onclick = () => {
      const count = qty ? Math.max(1, Math.min(maxQty, Math.floor(Number(qty.value) || 1))) : 1;
      act(count);
    };
    row.append(btn);
    row.onmouseenter = () => this.tip.show(info, this.root, { ...this.actions.me(), price: this.mode, count: have, side: true });
    row.onmouseleave = () => this.tip.hide();
    return row;
  }
}
