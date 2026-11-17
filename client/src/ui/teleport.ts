import type { TeleportDest } from "../gen/erevia/v1/game_pb";
import { button, el, fmt } from "./kit";
import { goldIcon, type ItemStore } from "./items";

export interface TeleportActions {
  travel(npcId: number, destId: number): void;
  /** Karma impede o teleporte (o servidor confere de novo). */
  karma(): number;
  /** Viajante (Maestria): desconto no preço (0–1; o servidor cobra o mesmo). */
  discount(): number;
}

/**
 * Pedra de Teleporte: lista de destinos com o preço; viajar leva 2 s parado. Um destino só vale
 * depois que o personagem chegou a pé à pedra dele (o servidor manda os ids conhecidos em
 * StonesKnown); os outros aparecem em cinza, só com a região.
 */
export class TeleportWindow {
  readonly root = el("div", "shop-panel tp-panel frame");
  private title = el("h2");
  private list = el("div", "shop-list");
  private note = el("p", "tp-note small");
  private goldEl = el("span", "shop-gold-value");
  private dests: TeleportDest[] = [];
  /** Ids dos destinos já descobertos (StonesKnown). */
  private known = new Set<number>();
  /** Ids dos destinos de postos caídos (AreaEventStatus.closed): fechados até o posto reabrir. */
  private closed = new Map<number, string>();
  /** Destino que o "Ir" da missão sugere (destacado; quem escolhe é o jogador). */
  private suggested = 0;
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

  /** Os destinos descobertos mudaram (ao entrar e a cada pedra nova). */
  setKnown(ids: Iterable<number>): void {
    this.known = new Set(ids);
    this.render();
  }

  /** Os postos fechados mudaram (id do destino -> nome do posto). */
  setClosed(closed: Map<number, string>): void {
    this.closed = closed;
    this.render();
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  /** `suggested`: destino destacado pelo "Ir" da missão (0 = nenhum). */
  open(npcId: number, title: string, dests: TeleportDest[], suggested = 0): void {
    this.npcId = npcId;
    this.suggested = suggested;
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

  /** Preço com o desconto, arredondado como no servidor. */
  private priceOf(base: number): number {
    const d = this.actions.discount();
    return d > 0 ? Math.round(base * (1 - d)) : base;
  }

  private render(): void {
    this.goldEl.textContent = fmt.format(this.store.gold);
    if (!this.isOpen) return;
    const karma = this.actions.karma() > 0;
    this.note.textContent = karma ? "A pedra não responde a quem tem karma. Cace monstros para limpar o seu nome." : "";
    this.note.hidden = !karma;
    this.list.replaceChildren();
    for (const d of this.dests) {
      if (!this.known.has(d.id)) {
        // Ainda não descoberta: cinza, com a região (chegue a pé até a pedra).
        this.list.append(
          el(
            "div",
            "shop-row tp-row unknown",
            el("div", "tp-icon", "?"),
            el("div", "shop-name", el("strong", null, "Pedra não descoberta"), el("span", "tp-region", d.region), el("span", "shop-price", "Chegue a pé até ela")),
          ),
        );
        continue;
      }
      if (this.closed.has(d.id)) {
        // Posto caído: a pedra está apagada até ele reabrir (o servidor recusa de qualquer jeito).
        this.list.append(
          el(
            "div",
            "shop-row tp-row unknown",
            el("div", "tp-icon", "✕"),
            el("div", "shop-name", el("strong", null, d.name), el("span", "tp-region", d.region), el("span", "shop-price", "O posto caiu: pedra fechada")),
          ),
        );
        continue;
      }
      const price = this.priceOf(d.price);
      const poor = this.store.gold < price;
      const go = button("Viajar", "btn btn-gold shop-btn", () => {
        this.actions.travel(this.npcId, d.id);
        this.close();
      });
      go.disabled = poor || karma;
      const row = el(
        "div",
        `shop-row tp-row${poor ? " poor" : ""}${d.id === this.suggested ? " suggested" : ""}`,
        el("div", "tp-icon", "✦"),
        el(
          "div",
          "shop-name",
          el("strong", null, d.name),
          d.region ? el("span", "tp-region", d.region) : null,
          el("span", "shop-price", `${fmt.format(price)} moedas${price < d.price ? " (Viajante)" : ""}`),
          d.id === this.suggested ? el("span", "tp-quest", "◎ Caminho da missão") : null,
        ),
        go,
      );
      this.list.append(row);
    }
  }
}
