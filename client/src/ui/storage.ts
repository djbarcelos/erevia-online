import type { Storage } from "../gen/erevia/v1/game_pb";
import { uniqueIds } from "./art";
import { beginDrag } from "./dragdrop";
import { buildItemTip, enchantedName, goldIcon, itemIcon, PLACE_BAG, type ItemStore, type Stack } from "./items";
import { art, button, el, fmt, toast, tooltip } from "./kit";

/** Lugar do armazém nas mensagens (bolsa = 0, equipado = 1). */
export const PLACE_STORAGE = 2;

export interface StorageActions {
  move(npcId: number, fromPlace: number, fromSlot: number, toPlace: number, toSlot: number, count: number): void;
  gold(npcId: number, amount: number): void;
  me(): { myClass: string; myLevel: number };
  /** O armazém abriu: o inventário abre ao lado. */
  onOpen(): void;
}

/**
 * Armazém da conta (Guarda do Armazém): 60 espaços e moedas guardadas. Arrastar da bolsa
 * para cá guarda; daqui para a bolsa (ou duplo clique) retira.
 */
export class StorageWindow {
  readonly root = el("div", "shop-panel storage-panel frame");
  private title = el("h2");
  private grid = el("div", "st-grid");
  private stored = el("b", "st-gold-value");
  private goldInput = el("input", "input st-gold-input");
  private slots: HTMLButtonElement[] = [];
  private items: (Stack | null)[] = [];
  private unsubscribe: () => void;
  npcId = 0;

  constructor(
    private store: ItemStore,
    private actions: StorageActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    this.root.dataset.drop = "storage"; // soltar um item da bolsa no painel = guardar
    const close = button("✕", "x-btn", () => this.close());
    close.title = "Fechar (Esc)";

    this.goldInput.type = "number";
    this.goldInput.min = "1";
    this.goldInput.placeholder = "Moedas";
    this.goldInput.setAttribute("aria-label", "Quantidade de moedas");
    this.goldInput.onkeydown = (ev) => ev.stopPropagation();
    const amount = () => Math.max(0, Math.floor(Number(this.goldInput.value) || 0));
    const put = button("Guardar", "btn btn-sm", () => {
      const n = Math.min(amount(), this.store.gold);
      if (n > 0) this.actions.gold(this.npcId, n);
      this.goldInput.value = "";
    });
    const take = button("Retirar", "btn btn-sm", () => {
      const n = amount();
      if (n > 0) this.actions.gold(this.npcId, -n);
      this.goldInput.value = "";
    });
    const gi = el("span", "shop-gold-icon");
    gi.innerHTML = goldIcon();
    const goldRow = el("div", "st-gold", gi, el("span", "muted", "Guardadas:"), this.stored, this.goldInput, put, take);

    for (let i = 0; i < 60; i++) {
      const b = el("button", "slot st-slot");
      b.type = "button";
      b.dataset.storage = String(i);
      this.wire(b, i);
      this.slots.push(b);
      this.grid.append(b);
    }
    this.root.append(
      el("div", "panel-head", this.title, close),
      this.grid,
      goldRow,
      el("p", "small muted st-hint", "Arraste da bolsa para guardar · arraste de volta ou dê duplo clique para retirar · todos os seus personagens usam o mesmo armazém"),
    );
    parent.append(this.root);
    this.unsubscribe = store.onChange(() => this.paint());
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  open(npcId: number, title: string): void {
    this.npcId = npcId;
    this.title.textContent = title;
    this.root.hidden = false;
    this.actions.onOpen();
  }

  /** Estado vindo do servidor. */
  set(st: Storage): void {
    this.items = Array.from({ length: st.size || 60 }, () => null);
    for (const it of st.items) {
      const info = this.store.get(it.itemId);
      if (info && it.slot < this.items.length) this.items[it.slot] = { info, count: it.count, enchant: it.enchant };
    }
    this.stored.textContent = fmt.format(Number(st.gold));
    this.paint();
  }

  close(): void {
    this.root.hidden = true;
    this.npcId = 0;
  }

  dispose(): void {
    this.unsubscribe();
    this.root.remove();
  }

  private paint(): void {
    this.slots.forEach((b, i) => {
      const s = this.items[i] ?? null;
      b.replaceChildren();
      if (!s) {
        b.removeAttribute("data-grade");
        b.setAttribute("aria-label", "Vazio");
        return;
      }
      b.dataset.grade = String(s.info.grade);
      b.append(art(uniqueIds(itemIcon(s.info))));
      if (s.count > 1) b.append(el("span", "qty", String(s.count)));
      if (s.enchant > 0) b.append(el("span", `ench${s.enchant >= 7 ? " hi" : s.enchant >= 4 ? " mid" : ""}`, `+${s.enchant}`));
      b.setAttribute("aria-label", s.count > 1 ? `${s.count}× ${s.info.name}` : enchantedName(s.info, s.enchant));
    });
  }

  private wire(b: HTMLButtonElement, i: number): void {
    tooltip(b, () => {
      const s = this.items[i];
      return s ? buildItemTip(s.info, { ...this.actions.me(), count: s.count, enchant: s.enchant, hint: "Duplo clique para retirar" }) : null;
    });
    b.addEventListener("dblclick", () => {
      if (this.items[i]) this.actions.move(this.npcId, PLACE_STORAGE, i, PLACE_BAG, 255, 0);
    });
    b.addEventListener("pointerdown", (e) => {
      const s = this.items[i];
      if (!s) return;
      let over: HTMLElement | null = null;
      beginDrag(e, {
        icon: uniqueIds(itemIcon(s.info)),
        onOver: (t) => {
          over?.classList.remove("drop-ok", "drop-bad");
          over = t?.closest<HTMLElement>("[data-bag], [data-storage]") ?? null;
          over?.classList.add("drop-ok");
        },
        onDrop: (t) => {
          over?.classList.remove("drop-ok", "drop-bad");
          const bag = t?.closest<HTMLElement>("[data-bag]");
          const st = t?.closest<HTMLElement>("[data-storage]");
          if (bag) this.actions.move(this.npcId, PLACE_STORAGE, i, PLACE_BAG, Number(bag.dataset.bag), 0);
          else if (st && Number(st.dataset.storage) !== i) this.actions.move(this.npcId, PLACE_STORAGE, i, PLACE_STORAGE, Number(st.dataset.storage), 0);
          else if (t?.closest(".inv-win")) this.actions.move(this.npcId, PLACE_STORAGE, i, PLACE_BAG, 255, 0);
          else if (!st) toast("Solte na bolsa para retirar.", "warn");
        },
      });
    });
  }
}
