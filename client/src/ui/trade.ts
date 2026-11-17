import type { TradeOffer, TradeState } from "../gen/erevia/v1/game_pb";
import { uniqueIds } from "./art";
import { buildItemTip, enchantedName, goldIcon, itemIcon, type ItemStore } from "./items";
import { art, button, el, fmt, tooltip } from "./kit";

const MAX_ITEMS = 8;

export interface TradeActions {
  remove(bagSlot: number): void;
  gold(amount: number): void;
  lock(): void;
  confirm(): void;
  cancel(): void;
  me(): { myClass: string; myLevel: number };
  /** A troca abriu: o inventário abre ao lado (arraste dele para oferecer). */
  onOpen(): void;
}

/**
 * Troca entre jogadores: a sua oferta à esquerda, a do outro à direita. Arraste da bolsa
 * (ou "Oferecer na troca" no menu do item) para oferecer; clique num item da sua oferta
 * para tirar. Qualquer mudança destrava as duas ofertas. Só redesenha quando chega um
 * TradeState (nada por quadro).
 */
export class TradeWindow {
  readonly root = el("div", "shop-panel trade-panel frame");
  private title = el("h2", null, "Troca");
  private mine = new OfferView(true);
  private theirs = new OfferView(false);
  private goldInput = el("input", "input tr-gold-input");
  private lockBtn: HTMLButtonElement;
  private confirmBtn: HTMLButtonElement;
  private state: TradeState | null = null;

  constructor(
    private store: ItemStore,
    private actions: TradeActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    this.root.dataset.drop = "trade"; // soltar um item da bolsa aqui = oferecer
    const close = button("✕", "x-btn", () => this.actions.cancel());
    close.title = "Cancelar a troca (Esc)";

    this.goldInput.type = "number";
    this.goldInput.min = "0";
    this.goldInput.placeholder = "Moedas";
    this.goldInput.setAttribute("aria-label", "Moedas oferecidas");
    this.goldInput.onkeydown = (ev) => {
      ev.stopPropagation();
      if (ev.key === "Enter") this.sendGold();
    };
    this.goldInput.onchange = () => this.sendGold();
    this.lockBtn = button("Travar oferta", "btn btn-sm", () => this.actions.lock());
    this.confirmBtn = button("Confirmar troca", "btn btn-gold btn-sm", () => this.actions.confirm());
    const cancel = button("Cancelar", "btn btn-sm", () => this.actions.cancel());

    this.mine.onRemove = (slot) => this.actions.remove(slot);
    this.mine.tip = this.theirs.tip = (o, i) => {
      const it = o.items[i];
      const info = it && this.store.get(it.itemId);
      return info ? buildItemTip(info, { ...this.actions.me(), count: it.count, enchant: it.enchant }) : null;
    };
    this.mine.store = this.theirs.store = store;

    const gi = el("span", "shop-gold-icon");
    gi.innerHTML = goldIcon();
    this.root.append(
      el("div", "panel-head", this.title, close),
      el("div", "tr-cols", this.mine.root, this.theirs.root),
      el("div", "tr-gold", gi, el("span", "muted", "Suas moedas na troca:"), this.goldInput),
      el("div", "tr-actions", cancel, this.lockBtn, this.confirmBtn),
      el("p", "small muted tr-hint", "Arraste itens da bolsa para cá · clique num item seu para tirar · qualquer mudança destrava as duas ofertas"),
    );
    parent.append(this.root);
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  set(st: TradeState): void {
    const wasOpen = this.isOpen;
    this.state = st.open ? st : null;
    this.root.hidden = !st.open;
    if (!st.open) return;
    this.title.textContent = `Troca com ${st.partnerName}`;
    const mine = st.mine!;
    const theirs = st.theirs!;
    this.mine.paint(mine, "Você oferece");
    this.theirs.paint(theirs, `${st.partnerName} oferece`);
    if (document.activeElement !== this.goldInput) this.goldInput.value = mine.gold > 0n ? String(mine.gold) : "";
    this.lockBtn.disabled = mine.locked;
    this.lockBtn.textContent = mine.locked ? "Oferta travada" : "Travar oferta";
    this.confirmBtn.disabled = !(mine.locked && theirs.locked) || mine.confirmed;
    this.confirmBtn.textContent = mine.confirmed ? "Esperando o outro…" : "Confirmar troca";
    if (!wasOpen) this.actions.onOpen();
  }

  dispose(): void {
    this.root.remove();
  }

  private sendGold(): void {
    if (!this.state) return;
    const n = Math.max(0, Math.floor(Number(this.goldInput.value) || 0));
    if (BigInt(n) !== this.state.mine?.gold) this.actions.gold(n);
  }
}

/** Uma coluna da troca: 8 espaços, as moedas e o estado (travada/confirmada). */
class OfferView {
  readonly root = el("section", "tr-offer");
  private head = el("h3", "tr-head");
  private status = el("span", "tr-status");
  private grid = el("div", "tr-grid");
  private gold = el("div", "tr-offer-gold");
  private slots: HTMLButtonElement[] = [];
  private offer: TradeOffer | null = null;
  onRemove: (bagSlot: number) => void = () => {};
  tip: (o: TradeOffer, i: number) => HTMLElement | string | null = () => null;
  store!: ItemStore;

  constructor(private own: boolean) {
    for (let i = 0; i < MAX_ITEMS; i++) {
      const b = el("button", "slot tr-slot");
      b.type = "button";
      tooltip(b, () => (this.offer ? this.tip(this.offer, i) : null));
      if (own)
        b.onclick = () => {
          const it = this.offer?.items[i];
          if (it) this.onRemove(it.slot);
        };
      this.slots.push(b);
      this.grid.append(b);
    }
    this.root.append(el("div", "tr-title", this.head, this.status), this.grid, this.gold);
  }

  paint(o: TradeOffer, title: string): void {
    this.offer = o;
    this.head.textContent = title;
    this.status.textContent = o.confirmed ? "Confirmada" : o.locked ? "Travada" : "";
    this.root.classList.toggle("locked", o.locked);
    this.slots.forEach((b, i) => {
      const it = o.items[i];
      const info = it && this.store.get(it.itemId);
      b.replaceChildren();
      if (!it || !info) {
        b.removeAttribute("data-grade");
        b.setAttribute("aria-label", "Vazio");
        return;
      }
      b.dataset.grade = String(info.grade);
      b.append(art(uniqueIds(itemIcon(info))));
      if (it.count > 1) b.append(el("span", "qty", String(it.count)));
      if (it.enchant > 0) b.append(el("span", `ench${it.enchant >= 7 ? " hi" : it.enchant >= 4 ? " mid" : ""}`, `+${it.enchant}`));
      const name = it.count > 1 ? `${it.count}× ${info.name}` : enchantedName(info, it.enchant);
      b.setAttribute("aria-label", this.own ? `${name} (clique para tirar)` : name);
    });
    this.gold.textContent = `${fmt.format(Number(o.gold))} moedas`;
  }
}
