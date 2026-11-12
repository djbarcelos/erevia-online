import type { SelfStats } from "../gen/erevia/v1/game_pb";
import { EQUIP_SLOTS, el, formatGold, goldIcon, itemIcon, itemLabel, ItemStore, ItemTip, PLACE_BAG, PLACE_EQUIP, type Stack } from "./items";

export interface InventoryActions {
  move(fromPlace: number, fromSlot: number, toPlace: number, toSlot: number): void;
  use(place: number, slot: number): void;
  destroy(slot: number, count: number): void;
  /** Item solto num espaço da barra de atalhos (data-drop="hotbar"). */
  assignHotbar(index: number, itemId: string): void;
  /** Item solto na loja aberta (data-drop="sell"). */
  sell(slot: number): void;
  /** Classe e nível do jogador (dicas e requisitos). */
  me(): { myClass: string; myLevel: number };
}

interface DragSource {
  place: number;
  slot: number;
  stack: Stack;
}

/** Distância (px) que o mouse precisa andar para virar arrasto. */
const DRAG_THRESHOLD = 5;

/**
 * Janela de inventário (tecla I): equipamento, atributos, a bolsa de 40 espaços e as
 * moedas. Arrastar e soltar move, troca, equipa, põe na barra ou na lixeira.
 */
export class InventoryWindow {
  readonly root = el("div", "inv-panel");
  private bagEls: HTMLDivElement[] = [];
  private equipEls: HTMLDivElement[] = [];
  private goldEl = el("span", "inv-gold-value", "0");
  private statsEl = el("ul", "inv-stats");
  private confirm = el("div", "inv-confirm");
  private ghost = el("div", "inv-ghost");
  private unsubscribe: () => void;

  constructor(
    private store: ItemStore,
    private tip: ItemTip,
    private actions: InventoryActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.setAttribute("aria-label", "Inventário");
    this.root.hidden = true;

    const head = el("div", "panel-head");
    const close = el("button", "minimap-btn", "✕");
    close.type = "button";
    close.title = "Fechar (I)";
    close.onclick = () => this.close();
    head.append(el("h2", "", "Inventário"), close);

    const top = el("div", "inv-top");
    const equip = el("div", "inv-equip");
    EQUIP_SLOTS.forEach((label, i) => {
      const slot = this.makeSlot(PLACE_EQUIP, i);
      slot.classList.add("equip");
      slot.dataset.label = label;
      const cell = el("div", "inv-equip-cell");
      cell.append(slot, el("span", "inv-equip-label", label));
      equip.append(cell);
      this.equipEls.push(slot);
    });
    top.append(equip, this.statsEl);

    const bag = el("div", "inv-bag");
    for (let i = 0; i < 40; i++) {
      const slot = this.makeSlot(PLACE_BAG, i);
      bag.append(slot);
      this.bagEls.push(slot);
    }

    const foot = el("div", "inv-foot");
    const gold = el("div", "inv-gold");
    const gi = el("span", "inv-gold-icon");
    gi.innerHTML = goldIcon();
    gold.append(gi, this.goldEl);
    const trash = el("div", "inv-trash");
    trash.dataset.drop = "trash";
    trash.title = "Arraste um item aqui para destruir";
    trash.innerHTML = `<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 7h14M10 7V4h4v3M7 7l1 13h8l1-13" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>`;
    foot.append(gold, el("span", "inv-hint", "Duplo clique: usar / equipar"), trash);

    this.confirm.hidden = true;
    this.root.append(head, top, bag, foot, this.confirm);
    this.ghost.hidden = true;
    parent.append(this.root, this.ghost);

    this.unsubscribe = store.onChange(() => this.render());
    this.render();
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  open(): void {
    this.root.hidden = false;
    this.render();
  }

  close(): void {
    this.root.hidden = true;
    this.confirm.hidden = true;
    this.tip.hide();
  }

  toggle(): void {
    if (this.isOpen) this.close();
    else this.open();
  }

  dispose(): void {
    this.unsubscribe();
    this.root.remove();
    this.ghost.remove();
  }

  setStats(s: SelfStats): void {
    const rows: [string, string, string?][] = [
      ["Vida", `${s.maxHp}`],
      ["Mana", `${s.maxMp}`, "mana"],
      ["Atq. físico", `${s.pAtk}`],
      ["Atq. mágico", `${s.mAtk}`],
      ["Def. física", `${s.pDef}`],
      ["Def. mágica", `${s.mDef}`],
      ["Crítico", `${Math.round(s.crit * 100)}%`],
    ];
    this.statsEl.replaceChildren(
      ...rows.map(([k, v, cls]) => {
        const li = el("li", cls);
        li.append(el("span", "", k), el("b", "", v));
        return li;
      }),
    );
  }

  render(): void {
    this.goldEl.textContent = formatGold(this.store.gold);
    if (!this.isOpen) return;
    this.store.bag.forEach((s, i) => this.paint(this.bagEls[i], s));
    this.store.equip.forEach((s, i) => this.paint(this.equipEls[i], s));
  }

  private paint(slot: HTMLDivElement | undefined, s: Stack | null): void {
    if (!slot) return;
    const key = s ? `${s.info.id}:${s.count}` : "";
    if (slot.dataset.key === key) return;
    slot.dataset.key = key;
    slot.replaceChildren();
    slot.classList.toggle("filled", !!s);
    if (!s) return;
    const icon = el("div", "inv-icon");
    icon.innerHTML = itemIcon(s.info);
    slot.append(icon);
    if (s.count > 1) slot.append(el("span", "inv-count", String(s.count)));
    slot.style.setProperty("--grade", `var(--grade-${s.info.grade})`);
  }

  private stackAt(place: number, slot: number): Stack | null {
    return (place === PLACE_EQUIP ? this.store.equip[slot] : this.store.bag[slot]) ?? null;
  }

  private makeSlot(place: number, index: number): HTMLDivElement {
    const slot = el("div", "inv-slot");
    slot.dataset.drop = place === PLACE_EQUIP ? "equip" : "bag";
    slot.dataset.slot = String(index);

    slot.addEventListener("mouseenter", () => {
      const s = this.stackAt(place, index);
      if (!s) return;
      const hint = place === PLACE_EQUIP ? "Duplo clique para tirar" : s.info.slot >= 0 ? "Duplo clique para equipar" : s.info.healHp || s.info.healMp ? "Duplo clique para usar · arraste para a barra" : undefined;
      this.tip.show(s.info, slot, { ...this.actions.me(), count: s.count, hint });
    });
    slot.addEventListener("mouseleave", () => this.tip.hide());
    slot.addEventListener("dblclick", () => {
      if (this.stackAt(place, index)) this.actions.use(place, index);
    });
    slot.addEventListener("contextmenu", (ev) => {
      ev.preventDefault();
      if (this.stackAt(place, index)) this.actions.use(place, index);
    });
    slot.addEventListener("pointerdown", (ev) => {
      if (ev.button !== 0) return;
      const s = this.stackAt(place, index);
      if (s) this.beginDrag(ev, { place, slot: index, stack: s });
    });
    return slot;
  }

  private beginDrag(down: PointerEvent, src: DragSource): void {
    let dragging = false;
    const move = (ev: PointerEvent) => {
      if (!dragging && Math.hypot(ev.clientX - down.clientX, ev.clientY - down.clientY) < DRAG_THRESHOLD) return;
      if (!dragging) {
        dragging = true;
        this.tip.hide();
        this.ghost.innerHTML = itemIcon(src.stack.info);
        this.ghost.hidden = false;
        document.body.classList.add("dragging-item");
      }
      this.ghost.style.left = `${ev.clientX}px`;
      this.ghost.style.top = `${ev.clientY}px`;
      for (const e of document.querySelectorAll(".drop-hover")) e.classList.remove("drop-hover");
      this.dropTarget(ev)?.classList.add("drop-hover");
    };
    const up = (ev: PointerEvent) => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", up);
      for (const e of document.querySelectorAll(".drop-hover")) e.classList.remove("drop-hover");
      document.body.classList.remove("dragging-item");
      this.ghost.hidden = true;
      if (dragging) this.drop(src, this.dropTarget(ev));
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", up);
  }

  private dropTarget(ev: PointerEvent): HTMLElement | null {
    const hit = document.elementFromPoint(ev.clientX, ev.clientY);
    return (hit?.closest("[data-drop]") as HTMLElement | null) ?? null;
  }

  private drop(src: DragSource, target: HTMLElement | null): void {
    if (!target) return;
    const slot = Number(target.dataset.slot ?? -1);
    switch (target.dataset.drop) {
      case "bag":
        if (src.place === PLACE_BAG && src.slot === slot) return;
        this.actions.move(src.place, src.slot, PLACE_BAG, slot);
        break;
      case "equip":
        if (src.place === PLACE_BAG) this.actions.move(PLACE_BAG, src.slot, PLACE_EQUIP, slot);
        break;
      case "hotbar":
        if (src.stack.info.healHp || src.stack.info.healMp) this.actions.assignHotbar(slot, src.stack.info.id);
        break;
      case "sell":
        if (src.place === PLACE_BAG) this.actions.sell(src.slot);
        break;
      case "trash":
        if (src.place === PLACE_BAG) this.askDestroy(src);
        break;
    }
  }

  private askDestroy(src: DragSource): void {
    const c = this.confirm;
    c.replaceChildren();
    c.append(el("p", "", `Destruir ${itemLabel(src.stack.info, src.stack.count)}? Não tem volta.`));
    const row = el("div", "inv-confirm-row");
    const yes = el("button", "btn btn-gold", "Destruir");
    const no = el("button", "btn btn-ghost", "Cancelar");
    yes.type = no.type = "button";
    yes.onclick = () => {
      this.actions.destroy(src.slot, src.stack.count);
      c.hidden = true;
    };
    no.onclick = () => (c.hidden = true);
    row.append(yes, no);
    c.append(row);
    c.hidden = false;
  }
}
