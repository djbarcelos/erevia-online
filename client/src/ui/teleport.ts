import type { TeleportDest } from "../gen/erevia/v1/game_pb";
import { button, el, fmt } from "./kit";
import { goldIcon, type ItemStore } from "./items";

export interface TeleportActions {
  travel(npcId: number, destId: number): void;
  /** Karma impede o teleporte (o servidor confere de novo). */
  karma(): number;
}

/** Pedra de Teleporte: lista de destinos com o preço; viajar leva 2 s parado. */
export class TeleportWindow {
  readonly root = el("div", "shop-panel tp-panel frame");
  private title = el("h2");
  private list = el("div", "shop-list");
  private note = el("p", "tp-note small");
  private goldEl = el("span", "shop-gold-value");
  private dests: TeleportDest[] = [];
  private unsubscribe: () => void;
  npcId = 0;

  constructor(
    private store: ItemStore,
    private actions: TeleportActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    const close = button("✕", "x-btn", () => this.close());
    close.title = "Fechar (Esc)";
    const gi = el("span", "shop-gold-icon");
    gi.innerHTML = goldIcon();
    this.root.append(
      el("div", "panel-head", this.title, close),
      this.list,
      this.note,
      el("div", "shop-foot", el("div", "shop-gold", gi, this.goldEl), el("span", "shop-hint", "Levar dano ou andar cancela a viagem")),
    );
    parent.append(this.root);
    this.unsubscribe = store.onChange(() => this.render());
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  open(npcId: number, title: string, dests: TeleportDest[]): void {
    this.npcId = npcId;
    this.title.textContent = title;
    this.dests = dests;
    this.root.hidden = false;
    this.render();
  }

  close(): void {
    this.root.hidden = true;
    this.npcId = 0;
  }

  dispose(): void {
    this.unsubscribe();
    this.root.remove();
  }

  private render(): void {
    this.goldEl.textContent = fmt.format(this.store.gold);
    if (!this.isOpen) return;
    const karma = this.actions.karma() > 0;
    this.note.textContent = karma ? "A pedra não responde a quem tem karma. Cace monstros para limpar o seu nome." : "";
    this.note.hidden = !karma;
    this.list.replaceChildren();
    for (const d of this.dests) {
      const poor = this.store.gold < d.price;
      const go = button("Viajar", "btn btn-gold shop-btn", () => {
        this.actions.travel(this.npcId, d.id);
        this.close();
      });
      go.disabled = poor || karma;
      const row = el(
        "div",
        `shop-row tp-row${poor ? " poor" : ""}`,
        el("div", "tp-icon", "✦"),
        el("div", "shop-name", el("strong", null, d.name), el("span", "shop-price", `${fmt.format(d.price)} moedas`)),
        go,
      );
      this.list.append(row);
    }
  }
}
