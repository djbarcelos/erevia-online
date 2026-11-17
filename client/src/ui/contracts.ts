/**
 * Contratos do nível máximo e loja de reputação (tunnel#157).
 *
 * O Quadro de Contratos é um NPC da praça (ContractState.boardId): clicar nele abre esta
 * janela, com duas abas — os contratos do dia (3 diários e 1 semanal, aceitos aqui) e a
 * loja das facções (o grau abre as ofertas, que custam moedas). O servidor manda o
 * ContractState ao entrar, ao aceitar e quando um contrato anda ou termina, e o
 * ReputationState ao entrar, ao pedir a loja e quando a reputação muda. A janela só
 * redesenha nesses momentos (e a cada 30 s com ela aberta, para o relógio da renovação).
 */

import {
  ContractSlot,
  ContractStatus,
  type ContractInfo,
  type ContractState,
  type FactionInfo,
  type RepOffer,
  type ReputationState,
} from "../gen/erevia/v1/game_pb";
import { bar, button, el, fmt } from "./kit";
import { formatGold, goldIcon, GRADE_COLOR, itemIcon, type ItemStore, type ItemTip } from "./items";
import { ui } from "./art";
import "../styles/contracts.css";

export const SLOT_LABEL: Record<number, string> = {
  [ContractSlot.HUNT]: "Caça",
  [ContractSlot.BOSS]: "Chefe de campo",
  [ContractSlot.COLLECT]: "Coleta",
  [ContractSlot.WEEKLY]: "Semanal",
};

/** O que o servidor mandou por último (contratos, facções e a loja). */
export class ContractsStore {
  state: ContractState | null = null;
  rep: ReputationState | null = null;
  /** A loja vem só na resposta ao pedido (e depois de comprar): guarda a última. */
  offers: RepOffer[] = [];
  private listeners = new Set<() => void>();

  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  setContracts(s: ContractState): void {
    this.state = s;
    this.emit();
  }

  setReputation(r: ReputationState): void {
    this.rep = r;
    if (r.offers.length) this.offers = r.offers;
    this.emit();
  }

  isBoard(npcId: number): boolean {
    return npcId !== 0 && this.state?.boardId === npcId;
  }

  faction(id: string): FactionInfo | undefined {
    return this.rep?.factions.find((f) => f.id === id);
  }

  gradeName(g: number): string {
    return this.rep?.gradeNames[g] ?? "";
  }

  private emit(): void {
    for (const fn of this.listeners) fn();
  }
}

/** "5 h 12 min", "2 d 4 h" ou "menos de 1 min". */
export function untilText(ms: number): string {
  const min = Math.floor(ms / 60000);
  if (min < 1) return "menos de 1 min";
  const d = Math.floor(min / 1440);
  const h = Math.floor((min % 1440) / 60);
  const m = min % 60;
  if (d > 0) return `${d} d ${h} h`;
  return h > 0 ? `${h} h ${m} min` : `${m} min`;
}

/** Selo da facção (cor e nome). */
export function factionTag(f: FactionInfo | undefined, grade?: string): HTMLElement {
  const tag = el("span", "ct-faction", el("i", "ct-dot"), f?.name ?? "");
  if (f) tag.style.setProperty("--fc", f.color);
  if (grade) tag.append(el("b", null, ` · ${grade}`));
  return tag;
}

/** Barra da reputação até o próximo grau. */
export function repBar(f: FactionInfo, gradeName: (g: number) => string): HTMLElement {
  const b = bar("xp", "ct-rep-bar");
  if (f.gradeNext > 0) b.set(f.points - f.gradeStart, f.gradeNext - f.gradeStart, `${fmt.format(f.points)} / ${fmt.format(f.gradeNext)} · próximo: ${gradeName(f.grade + 1)}`);
  else b.set(1, 1, `${fmt.format(f.points)} · ${gradeName(f.grade)} (o máximo)`);
  b.root.style.setProperty("--fc", f.color);
  return b.root;
}

/** Cartão de um contrato (o quadro e o Diário usam o mesmo). */
export function contractCard(c: ContractInfo, s: ContractsStore, accept?: (id: string) => void, canAccept = true): HTMLElement {
  const done = c.status === ContractStatus.DONE;
  const active = c.status === ContractStatus.ACTIVE;
  const card = el(
    "article",
    `ct-card${done ? " done" : active ? " active" : ""}`,
    el("div", "ct-card-head", el("span", `ct-slot slot-${c.slot}`, SLOT_LABEL[c.slot] ?? ""), factionTag(s.faction(c.faction))),
    el("h3", "ct-title", c.title, done ? el("span", "ct-check", " ✓") : null),
    el("p", "ct-text", c.text),
    el("small", "ct-place", "Onde: ", el("b", null, c.place)),
  );
  if (active || done) {
    const b = bar("xp", "ct-progress");
    b.set(c.count, c.need, `${fmt.format(c.count)} / ${fmt.format(c.need)}`);
    card.append(b.root);
  }
  const chips = el(
    "div",
    "ct-reward",
    el("span", "jr-chip gold", `${fmt.format(Number(c.gold))} moedas`),
    el("span", "jr-chip xp", `${fmt.format(Number(c.xp))} ${c.reward.includes("Maestria") ? "XP de Maestria" : "XP"}`),
    el("span", "jr-chip rep", `+${fmt.format(c.rep)} reputação`),
  );
  card.append(chips);
  if (accept && c.status === ContractStatus.OPEN) {
    const btn = button("Aceitar", "btn btn-gold btn-sm ct-accept", () => accept(c.id));
    btn.disabled = !canAccept;
    card.append(btn);
  } else if (active) {
    card.append(el("span", "ct-state", "Aceito: o progresso anda sozinho"));
  } else if (done) {
    card.append(el("span", "ct-state ok", "Concluído — recompensa entregue"));
  }
  return card;
}

export interface ContractsActions {
  accept(npcId: number, contract: string): void;
  buy(npcId: number, offer: string): void;
  /** Pede o estado dos contratos e a loja (a janela abriu ou o dia virou). */
  request(): void;
  me(): { myClass: string; myLevel: number };
}

type Tab = "contracts" | "shop";

/** A janela do Quadro de Contratos (painel de NPC, como a loja). */
export class ContractsWindow {
  readonly root = el("div", "shop-panel ct-panel frame");
  private title = el("h2");
  private body = el("div", "shop-list ct-body");
  private goldEl = el("span", "shop-gold-value");
  private tabBtns: HTMLButtonElement[] = [];
  private tab: Tab = "contracts";
  private timer = 0;
  private offs: (() => void)[] = [];
  npcId = 0;

  constructor(
    private store: ItemStore,
    private tip: ItemTip,
    private data: ContractsStore,
    private actions: ContractsActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    const close = button("✕", "x-btn", () => this.close());
    close.title = "Fechar (Esc)";
    const tabs = el("div", "shop-tabs");
    for (const [id, label] of [["contracts", "Contratos"], ["shop", "Loja de reputação"]] as const) {
      const b = button(label, "shop-tab", () => this.setTab(id));
      b.dataset.tab = id;
      tabs.append(b);
      this.tabBtns.push(b);
    }
    const gi = el("span", "shop-gold-icon");
    gi.innerHTML = goldIcon();
    this.root.append(
      el("div", "panel-head", this.title, close),
      tabs,
      this.body,
      el("div", "shop-foot", el("div", "shop-gold", gi, this.goldEl), el("span", "shop-hint", "Renova à meia-noite de Brasília")),
    );
    parent.append(this.root);
    this.offs.push(
      data.onChange(() => this.render()),
      store.onChange(() => this.render()),
    );
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  open(npcId: number, title: string): void {
    this.npcId = npcId;
    this.title.textContent = title;
    this.root.hidden = false;
    this.actions.request();
    this.setTab(this.tab);
    clearInterval(this.timer);
    // O relógio da renovação (e o pedido quando ela passa), só com a janela aberta.
    this.timer = window.setInterval(() => {
      const s = this.data.state;
      if (s && Date.now() >= Number(s.dailyResetMs)) this.actions.request();
      else this.render();
    }, 30_000);
  }

  close(): void {
    this.root.hidden = true;
    this.npcId = 0;
    clearInterval(this.timer);
    this.tip.hide();
  }

  dispose(): void {
    this.close();
    for (const off of this.offs) off();
    this.root.remove();
  }

  setTab(t: Tab): void {
    this.tab = t;
    for (const b of this.tabBtns) b.classList.toggle("active", b.dataset.tab === t);
    this.render();
  }

  private render(): void {
    this.goldEl.textContent = formatGold(this.store.gold);
    if (!this.isOpen) return;
    const top = this.body.scrollTop;
    if (this.tab === "contracts") this.renderContracts();
    else this.renderShop();
    this.body.scrollTop = top;
  }

  private renderContracts(): void {
    const s = this.data.state;
    if (!s) {
      this.body.replaceChildren(el("p", "shop-empty", "Carregando o quadro…"));
      return;
    }
    const low = this.actions.me().myLevel < s.minLevel;
    const now = Date.now();
    const daily = s.contracts.filter((c) => c.slot !== ContractSlot.WEEKLY);
    const weekly = s.contracts.filter((c) => c.slot === ContractSlot.WEEKLY);
    const accept = (id: string) => this.actions.accept(this.npcId, id);
    const out: HTMLElement[] = [];
    if (low) out.push(el("p", "ct-note warn", `Os contratos são para quem já chegou ao nível ${s.minLevel}. Volte quando estiver pronto.`));
    out.push(
      el("div", "ct-section", el("h3", null, "Diários"), el("span", "ct-reset", `Renovam em ${untilText(Number(s.dailyResetMs) - now)}`)),
      ...daily.map((c) => contractCard(c, this.data, accept, !low)),
      el("div", "ct-section", el("h3", null, "Semanal"), el("span", "ct-reset", `Renova em ${untilText(Number(s.weeklyResetMs) - now)} (segunda-feira)`)),
      ...weekly.map((c) => contractCard(c, this.data, accept, !low)),
    );
    this.body.replaceChildren(...out);
  }

  private renderShop(): void {
    const rep = this.data.rep;
    if (!rep || !this.data.offers.length) {
      this.body.replaceChildren(el("p", "shop-empty", "Carregando a loja…"));
      return;
    }
    const out: HTMLElement[] = [
      el("p", "ct-note", "A reputação sobe com os contratos e nunca é gasta: o grau abre as ofertas, pagas em moedas. Só itens que não deixam ninguém mais forte que o equipamento do chefe."),
    ];
    for (const f of rep.factions) {
      out.push(el("div", "ct-shop-faction", factionTag(f, rep.gradeNames[f.grade]), repBar(f, (g) => this.data.gradeName(g))));
      for (const o of this.data.offers.filter((x) => x.faction === f.id)) out.push(this.offerRow(o, f));
    }
    this.body.replaceChildren(...out);
  }

  private offerRow(o: RepOffer, f: FactionInfo): HTMLElement {
    const info = o.itemId ? this.store.get(o.itemId) : undefined;
    const icon = el("div", "shop-icon");
    if (info) {
      icon.innerHTML = itemIcon(info);
      if (o.count > 1) icon.append(el("span", "qty", String(o.count)));
    } else {
      icon.classList.add("ct-title-icon");
      icon.innerHTML = ui("trophy", 26);
    }
    const name = info ? (o.count > 1 ? `${o.count}× ${info.name}` : info.name) : `Título «${o.title}»`;
    const locked = f.grade < o.grade;
    const poor = this.store.gold < Number(o.price);
    const strong = el("strong", null, name);
    if (info) strong.style.color = GRADE_COLOR[info.grade] ?? GRADE_COLOR[1];
    else strong.style.color = "#d9b8ff";
    const row = el(
      "div",
      `shop-row ct-offer${locked ? " locked" : ""}${poor ? " poor" : ""}`,
      icon,
      el(
        "div",
        "shop-name",
        strong,
        el("span", "shop-price", `${formatGold(Number(o.price))} moedas`),
        el("span", `ct-grade${locked ? " need" : ""}`, locked ? `Pede ${this.data.gradeName(o.grade)}` : this.data.gradeName(o.grade)),
      ),
    );
    if (o.owned) row.append(el("span", "ct-state ok", "Comprado"));
    else {
      const btn = button("Comprar", "btn btn-gold shop-btn", () => this.actions.buy(this.npcId, o.id));
      btn.disabled = locked || poor;
      row.append(btn);
    }
    row.title = o.text;
    if (info) {
      row.onmouseenter = () => this.tip.show(info, this.root, { ...this.actions.me(), side: true });
      row.onmouseleave = () => this.tip.hide();
    }
    return row;
  }
}

let noticeEl: HTMLElement | null = null;
let noticeTimer = 0;

/**
 * Aviso do HUD quando um contrato termina (ou a reputação sobe de grau): fica 5 s no alto da
 * tela, embaixo do quadro do alvo, sem disputar o banner do nível e da Maestria.
 */
export function contractNotice(parent: HTMLElement, kicker: string, title: string, sub = ""): void {
  if (!noticeEl || !noticeEl.isConnected) {
    noticeEl = el("div", "ct-notice");
    noticeEl.setAttribute("role", "status");
    parent.append(noticeEl);
  }
  const n = noticeEl;
  n.replaceChildren(el("span", "ct-notice-kicker", kicker), el("b", null, title));
  if (sub) n.append(el("small", null, sub));
  n.classList.remove("show");
  void n.offsetWidth;
  n.classList.add("show");
  clearTimeout(noticeTimer);
  noticeTimer = window.setTimeout(() => n.classList.remove("show"), 5000);
}
