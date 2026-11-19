import type { PrivateShopView } from "../gen/erevia/v1/game_pb";
import { uniqueIds } from "./art";
import { buildItemTip, enchantedName, goldIcon, itemIcon, type ItemStore } from "./items";
import { art, button, el, fmt, toast, tooltip } from "./kit";

/** Limites (os mesmos do servidor, game/pshop.go). */
export const PSHOP_MAX_ITEMS = 8;
const TITLE_LEN = 24;
const MAX_PRICE = 1_000_000_000;

export interface PrivateShopActions {
  open(title: string, items: { slot: number; count: number; price: number }[]): void;
  close(): void;
  buy(sellerId: number, slot: number, count: number, price: bigint): void;
  me(): { myClass: string; myLevel: number };
  gold(): number;
  /** Montando a loja: o inventário abre ao lado (para escolher os itens). */
  onSetup(): void;
}

type Draft = { slot: number; count: number; price: number };

/**
 * Loja pessoal (tunnel#161), três jeitos:
 *  - montar: a placa e até 8 itens da bolsa com quantidade e preço por unidade ("Vender na
 *    loja pessoal" no menu do item da bolsa); "Abrir loja" manda para o servidor, que só
 *    aceita na área do mercado;
 *  - a própria loja aberta: o que ainda está à venda e "Fechar loja";
 *  - a loja de outro: cada item com o preço, a quantidade e "Comprar".
 * Só redesenha quando chega um PrivateShopView ou quando a montagem muda.
 */
export class PrivateShopWindow {
  readonly root = el("div", "shop-panel pshop-panel frame");
  private head = el("h2", null, "Loja pessoal");
  private sub = el("p", "small muted pshop-sub");
  private titleInput = el("input", "input pshop-title");
  private list = el("div", "pshop-list");
  private foot = el("div", "pshop-foot");
  private draft: Draft[] = [];
  private view: PrivateShopView | null = null;
  /** O que a janela mostra agora. */
  private mode: "setup" | "mine" | "theirs" = "setup";

  constructor(
    private store: ItemStore,
    private actions: PrivateShopActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    const close = button("✕", "x-btn", () => this.hide());
    close.title = "Fechar a janela (a loja continua aberta)";
    this.titleInput.maxLength = TITLE_LEN;
    this.titleInput.placeholder = "Placa da loja (ex.: Poções baratas)";
    this.titleInput.setAttribute("aria-label", "Placa da loja");
    this.titleInput.onkeydown = (ev) => ev.stopPropagation();
    this.root.append(el("div", "panel-head", this.head, close), this.sub, this.titleInput, this.list, this.foot);
    parent.append(this.root);
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  /** É a loja de sellerId que está na tela? */
  showing(sellerId: number): boolean {
    return this.isOpen && this.view?.sellerId === sellerId;
  }

  hide(): void {
    this.root.hidden = true;
  }

  /** Montar a loja; slot != undefined põe esse item da bolsa na lista. */
  setup(slot?: number): void {
    if (this.view?.mine && this.view.open) {
      toast("Sua loja já está aberta. Feche antes de mudar os itens.", "warn");
      this.mode = "mine";
      this.root.hidden = false;
      this.paint();
      return;
    }
    if (slot !== undefined && !this.draft.some((d) => d.slot === slot)) {
      const s = this.store.bag[slot];
      if (!s) return;
      if (this.draft.length >= PSHOP_MAX_ITEMS) {
        toast(`No máximo ${PSHOP_MAX_ITEMS} itens por loja.`, "warn");
      } else this.draft.push({ slot, count: s.count, price: Math.max(1, s.info.price || s.info.sellPrice || 1) });
    }
    const wasOpen = this.isOpen && this.mode === "setup";
    this.mode = "setup";
    this.root.hidden = false;
    this.paint();
    if (!wasOpen) this.actions.onSetup();
  }

  /** Chegou o estado de uma loja (a minha ou a de outro). */
  set(v: PrivateShopView): void {
    if (v.mine) {
      const was = this.view?.mine && this.view.open;
      this.view = v;
      if (v.open) {
        this.draft = [];
        this.mode = "mine";
        this.root.hidden = false;
      } else if (was && this.mode === "mine") {
        this.mode = "setup";
      }
      if (this.isOpen) this.paint();
      return;
    }
    if (!v.open) {
      if (this.showing(v.sellerId)) {
        toast(v.sellerName ? `A loja de ${v.sellerName} fechou.` : "Essa loja fechou.", "warn");
        this.hide();
      } else if (!v.sellerName) toast("Esse jogador não tem loja aberta.", "warn");
      return;
    }
    this.view = v;
    this.mode = "theirs";
    this.root.hidden = false;
    this.paint();
  }

  dispose(): void {
    this.root.remove();
  }

  private paint(): void {
    this.list.replaceChildren();
    this.foot.replaceChildren();
    this.titleInput.hidden = this.mode !== "setup";
    const v = this.view;
    if (this.mode === "setup") {
      this.head.textContent = "Montar loja pessoal";
      this.sub.textContent = "Escolha itens da bolsa (menu do item → Vender na loja pessoal), ponha o preço de cada unidade e abra a loja parado na área do mercado, entre a praça e o prédio do Mercado. Andar fecha a loja.";
      this.draft = this.draft.filter((d) => this.store.bag[d.slot]);
      if (!this.draft.length) this.list.append(el("p", "muted pshop-empty", "Nenhum item ainda."));
      for (const d of this.draft) this.list.append(this.draftRow(d));
      const openBtn = button("Abrir loja", "btn btn-gold btn-sm", () => this.actions.open(this.titleInput.value.trim(), this.draft));
      openBtn.disabled = !this.draft.length;
      this.foot.append(button("Limpar", "btn btn-sm", () => ((this.draft = []), this.paint())), openBtn);
      return;
    }
    if (!v) return;
    const mine = this.mode === "mine";
    this.head.textContent = mine ? `Sua loja · ${v.title}` : `${v.title} · ${v.sellerName}`;
    this.sub.textContent = mine
      ? "Sua loja está aberta enquanto você estiver aqui, parado. Quem compra paga na hora; as moedas vão para a sua bolsa."
      : "Compre na hora: as moedas saem da sua bolsa e o item entra nela.";
    for (const it of v.items) this.list.append(this.itemRow(v, it, mine));
    if (mine) this.foot.append(button("Fechar loja", "btn btn-danger btn-sm", () => this.actions.close()));
  }

  private icon(itemId: string, count: number, enchant: number): HTMLElement {
    const info = this.store.get(itemId);
    const b = el("div", "slot pshop-slot");
    if (info) {
      b.dataset.grade = String(info.grade);
      b.append(art(uniqueIds(itemIcon(info))));
      if (count > 1) b.append(el("span", "qty", String(count)));
      if (enchant > 0) b.append(el("span", "ench", `+${enchant}`));
      tooltip(b, () => buildItemTip(info, { ...this.actions.me(), count, enchant }));
    }
    return b;
  }

  private draftRow(d: Draft): HTMLElement {
    const s = this.store.bag[d.slot]!;
    const count = numInput(d.count, 1, s.count, "Quantidade");
    count.onchange = () => (d.count = clamp(count.value, 1, s.count));
    const price = numInput(d.price, 1, MAX_PRICE, "Preço por unidade");
    price.onchange = () => (d.price = clamp(price.value, 1, MAX_PRICE));
    const remove = button("✕", "btn btn-sm", () => {
      this.draft = this.draft.filter((x) => x !== d);
      this.paint();
    });
    remove.title = "Tirar da loja";
    return el(
      "div",
      "pshop-row",
      this.icon(s.info.id, s.count, s.enchant),
      el("span", "pshop-name", enchantedName(s.info, s.enchant)),
      el("label", "pshop-field", "Qtd.", count),
      el("label", "pshop-field", "Preço", price),
      remove,
    );
  }

  private itemRow(v: PrivateShopView, it: PrivateShopView["items"][number], mine: boolean): HTMLElement {
    const info = this.store.get(it.itemId);
    const name = info ? enchantedName(info, it.enchant) : it.itemId;
    const gi = el("span", "shop-gold-icon");
    gi.innerHTML = goldIcon();
    const row = el(
      "div",
      "pshop-row",
      this.icon(it.itemId, it.count, it.enchant),
      el("span", "pshop-name", name, el("small", "muted", ` · ${it.count} à venda`)),
      el("span", "pshop-price", gi, `${fmt.format(Number(it.price))} cada`),
    );
    if (!mine) {
      const qty = numInput(1, 1, it.count, "Quantidade");
      const buy = button("Comprar", "btn btn-gold btn-sm", () => {
        const n = clamp(qty.value, 1, it.count);
        const total = Number(it.price) * n;
        if (total > this.actions.gold()) {
          toast(`Faltam ${fmt.format(total - this.actions.gold())} moedas.`, "warn");
          return;
        }
        this.actions.buy(v.sellerId, it.slot, n, it.price);
      });
      row.append(qty, buy);
    }
    return row;
  }
}

function numInput(value: number, min: number, max: number, label: string): HTMLInputElement {
  const i = el("input", "input pshop-num");
  i.type = "number";
  i.min = String(min);
  i.max = String(max);
  i.value = String(value);
  i.setAttribute("aria-label", label);
  i.onkeydown = (ev) => ev.stopPropagation();
  return i;
}

function clamp(v: string, min: number, max: number): number {
  const n = Math.floor(Number(v));
  return Number.isFinite(n) ? Math.min(max, Math.max(min, n)) : min;
}
