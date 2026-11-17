/**
 * Casa de leilões (Severino, o Leiloeiro, na praça). Três abas:
 * - Comprar: busca por nome, tipo, raridade, nível e preço (e "só o que a minha classe
 *   usa"), com ordem e páginas; comprar pede um segundo clique. O item vai para a bolsa (ou
 *   para o correio, se ela estiver cheia).
 * - Vender: escolhe um item da bolsa (ou parte da pilha), o preço total e o prazo; mostra a
 *   taxa (não volta) e quanto chega pelo correio depois da comissão.
 * - Meus anúncios: ativos (com cancelar) e os últimos encerrados, e "Retirar tudo" do
 *   correio (moedas das vendas e itens devolvidos).
 * Quem decide tudo é o servidor; a janela só redesenha quando chega uma resposta.
 */

import {
  AuctionCmdKind,
  AuctionSort,
  AuctionStatus,
  ItemKind,
  type AuctionListing,
  type AuctionQuerySchema,
  type AuctionRules,
  type AuctionUpdate,
} from "../../gen/erevia/v1/game_pb";
import type { MessageInitShape } from "@bufbuild/protobuf";
import { uniqueIds } from "../art";
import { buildItemTip, enchantedName, GRADE_COLOR, GRADE_NAME, goldIcon, itemIcon, type ItemStore } from "../items";
import { art, button, el, fmt, tabs, tooltip } from "../kit";
import "../../styles/auction.css";

export type AuctionTab = "buy" | "sell" | "mine";

export interface AuctionCmdOpts {
  query?: MessageInitShape<typeof AuctionQuerySchema>;
  slot?: number;
  count?: number;
  price?: number;
  hours?: number;
  auctionId?: bigint;
  mailId?: bigint;
}

/** O que a casa de leilões mandou por último (guardado fora da janela). */
export class AuctionState {
  npcId = 0;
  rules: AuctionRules | null = null;
  results: AuctionListing[] = [];
  total = 0;
  page = 0;
  searched = false;
  mine: AuctionListing[] = [];
  mineLoaded = false;
  active = 0;
  pending = 0;
  claimable = 0;
  notice = "";
  private listeners = new Set<() => void>();

  apply(u: AuctionUpdate): void {
    if (u.open) {
      this.npcId = u.npcId;
      if (u.rules) this.rules = u.rules;
    }
    if (u.searched) {
      this.results = u.results;
      this.total = u.total;
      this.page = u.page;
      this.searched = true;
    }
    if (u.mineListed) {
      this.mine = u.mine;
      this.active = u.active;
      this.mineLoaded = true;
    }
    if (u.goneId) this.results = this.results.filter((l) => l.id !== u.goneId);
    if (u.notice) this.notice = u.notice;
    this.pending = u.pending;
    this.claimable = u.claimable;
    for (const fn of this.listeners) fn();
  }

  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }
}

export interface AuctionDeps {
  state: AuctionState;
  store: ItemStore;
  me(): { myClass: string; myLevel: number };
  send(kind: AuctionCmdKind, opts?: AuctionCmdOpts): void;
  tab: AuctionTab;
  onTab(tab: AuctionTab): void;
}

const KINDS: { id: string; label: string }[] = [
  { id: "0", label: "Todos os tipos" },
  { id: String(ItemKind.WEAPON), label: "Armas" },
  { id: String(ItemKind.SHIELD), label: "Escudos" },
  { id: String(ItemKind.ARMOR), label: "Armaduras" },
  { id: String(ItemKind.HELMET), label: "Elmos" },
  { id: String(ItemKind.LEGS), label: "Pernas" },
  { id: String(ItemKind.GLOVES), label: "Luvas" },
  { id: String(ItemKind.BOOTS), label: "Botas" },
  { id: String(ItemKind.CAPE), label: "Capas" },
  { id: String(ItemKind.NECKLACE), label: "Colares" },
  { id: String(ItemKind.EARRING), label: "Brincos" },
  { id: String(ItemKind.RING), label: "Anéis" },
  { id: String(ItemKind.POTION), label: "Poções" },
  { id: String(ItemKind.SCROLL), label: "Pergaminhos" },
  { id: String(ItemKind.MATERIAL), label: "Materiais" },
];

const SORTS: { id: string; label: string }[] = [
  { id: String(AuctionSort.PRICE_ASC), label: "Mais baratos" },
  { id: String(AuctionSort.PRICE_DESC), label: "Mais caros" },
  { id: String(AuctionSort.NEWEST), label: "Mais novos" },
  { id: String(AuctionSort.ENDING), label: "Vencem antes" },
];

function selectEl(options: { id: string; label: string }[], label: string): HTMLSelectElement {
  const s = el("select", "input");
  s.setAttribute("aria-label", label);
  for (const o of options) {
    const opt = el("option", null, o.label);
    opt.value = o.id;
    s.append(opt);
  }
  return s;
}

function numInput(placeholder: string, label: string): HTMLInputElement {
  const i = el("input", "input");
  i.type = "number";
  i.min = "0";
  i.inputMode = "numeric";
  i.placeholder = placeholder;
  i.setAttribute("aria-label", label);
  i.onkeydown = (ev) => ev.stopPropagation();
  return i;
}

const num = (i: HTMLInputElement) => Math.max(0, Math.floor(Number(i.value) || 0));

function timeLeft(ms: number): string {
  const left = ms - Date.now();
  if (left <= 0) return "vencendo";
  const h = Math.floor(left / 3_600_000);
  const m = Math.floor((left % 3_600_000) / 60_000);
  return h > 0 ? `${h} h ${m} min` : `${Math.max(1, m)} min`;
}

/** Botão que pede um segundo clique para confirmar. */
function confirmButton(label: string, confirmLabel: string, cls: string, run: () => void): HTMLButtonElement {
  let armed = 0;
  const b = button(label, cls, () => {
    if (armed) {
      window.clearTimeout(armed);
      armed = 0;
      b.textContent = label;
      run();
      return;
    }
    b.textContent = confirmLabel;
    b.classList.add("armed");
    armed = window.setTimeout(() => {
      armed = 0;
      b.textContent = label;
      b.classList.remove("armed");
    }, 3000);
  });
  return b;
}

export function buildAuction(d: AuctionDeps): HTMLElement {
  const root = el("div", "auc-win");
  const st = d.state;
  let tab: AuctionTab = d.tab;
  const rules = () => st.rules;

  // ---------- Cabeçalho: moedas, avisos ----------
  const goldVal = el("b");
  const pendingEl = el("span", "small auc-pending");
  const noticeEl = el("p", "small auc-notice");
  noticeEl.setAttribute("role", "status");
  const head = el("div", "auc-head", el("span", "auc-gold", art(uniqueIds(goldIcon()), "auc-coin"), goldVal), pendingEl);

  const panes: Record<AuctionTab, HTMLElement> = { buy: el("div", "auc-pane"), sell: el("div", "auc-pane"), mine: el("div", "auc-pane") };
  const bar = tabs(
    [
      { id: "buy", label: "Comprar" },
      { id: "sell", label: "Vender" },
      { id: "mine", label: "Meus anúncios" },
    ],
    tab,
    (id) => showTab(id as AuctionTab),
  );

  const itemCell = (itemId: string, count: number, enchant: number) => {
    const info = d.store.get(itemId);
    const icon = el("span", "slot auc-icon");
    const name = el("b", "auc-name");
    if (info) {
      icon.dataset.grade = String(info.grade);
      icon.append(art(uniqueIds(itemIcon(info))));
      if (count > 1) icon.append(el("span", "qty", String(count)));
      if (enchant > 0) icon.append(el("span", `ench${enchant >= 7 ? " hi" : enchant >= 4 ? " mid" : ""}`, `+${enchant}`));
      name.textContent = count > 1 ? `${count}× ${info.name}` : enchantedName(info, enchant);
      name.style.color = GRADE_COLOR[info.grade] ?? GRADE_COLOR[1];
      tooltip(icon, () => buildItemTip(info, { ...d.me(), count, enchant }));
    } else name.textContent = itemId;
    return { icon, name, info };
  };

  const priceTag = (n: bigint | number) => el("span", "auc-price", art(uniqueIds(goldIcon()), "auc-coin"), fmt.format(Number(n)));

  // ---------- Comprar ----------
  const qName = el("input", "input");
  qName.placeholder = "Nome do item";
  qName.maxLength = 40;
  qName.setAttribute("aria-label", "Nome do item");
  const qKind = selectEl(KINDS, "Tipo");
  const qGrade = selectEl(
    [{ id: "0", label: "Qualquer raridade" }, ...[1, 2, 3, 4, 5].map((g) => ({ id: String(g), label: GRADE_NAME[g] }))],
    "Raridade",
  );
  const qMinLv = numInput("Nv mín.", "Nível mínimo");
  const qMaxLv = numInput("Nv máx.", "Nível máximo");
  const qMinP = numInput("Preço mín.", "Preço mínimo");
  const qMaxP = numInput("Preço máx.", "Preço máximo");
  const qSort = selectEl(SORTS, "Ordem");
  const qMine = el("input");
  qMine.type = "checkbox";
  const qMineLabel = el("label", "auc-check", qMine, "Só o que a minha classe usa");
  let page = 0;
  const query = (): MessageInitShape<typeof AuctionQuerySchema> => ({
    name: qName.value.trim(),
    kind: Number(qKind.value) as ItemKind,
    grade: Number(qGrade.value),
    minLevel: num(qMinLv),
    maxLevel: num(qMaxLv),
    minPrice: BigInt(num(qMinP)),
    maxPrice: BigInt(num(qMaxP)),
    myClass: qMine.checked,
    sort: Number(qSort.value) as AuctionSort,
    page,
  });
  const search = (p = 0) => {
    page = p;
    d.send(AuctionCmdKind.AUCTION_CMD_SEARCH, { query: query() });
  };
  qName.onkeydown = (ev) => {
    ev.stopPropagation();
    if (ev.key === "Enter") search(0);
  };
  qSort.onchange = () => search(0);
  const filters = el(
    "div",
    "auc-filters",
    el("div", "auc-row auc-row-main", qName, button("Buscar", "btn btn-gold btn-sm", () => search(0))),
    el("div", "auc-row", qKind, qGrade, qSort),
    el("div", "auc-row", qMinLv, qMaxLv, qMinP, qMaxP),
    el("div", "auc-row", qMineLabel, button("Limpar", "btn btn-sm", () => {
      qName.value = qMinLv.value = qMaxLv.value = qMinP.value = qMaxP.value = "";
      qKind.value = qGrade.value = qSort.value = "0";
      qMine.checked = false;
      search(0);
    })),
  );
  const results = el("div", "auc-list");
  results.setAttribute("aria-label", "Anúncios");
  const pager = el("div", "auc-pager");
  panes.buy.append(filters, results, pager);

  const renderBuy = () => {
    results.replaceChildren();
    if (!st.searched) results.append(el("p", "muted", "Buscando…"));
    else if (!st.results.length) results.append(el("p", "muted", "Nenhum anúncio encontrado."));
    for (const l of st.results) {
      const c = itemCell(l.itemId, l.count, l.enchant);
      const can = d.store.gold >= Number(l.price);
      const act = l.mine
        ? el("span", "small muted", "seu anúncio")
        : confirmButton("Comprar", "Confirmar?", `btn btn-sm ${can ? "btn-gold" : ""}`, () =>
            d.send(AuctionCmdKind.AUCTION_CMD_BUY, { auctionId: l.id, price: Number(l.price) }),
          );
      if (!can && !l.mine) (act as HTMLButtonElement).disabled = true;
      results.append(
        el(
          "div",
          "auc-item",
          c.icon,
          el("div", "auc-body", c.name, el("small", "muted", `${l.sellerName} · vence em ${timeLeft(Number(l.expiresAtMs))}`)),
          priceTag(l.price),
          act,
        ),
      );
    }
    const size = rules()?.pageSize || 20;
    const pages = Math.max(1, Math.ceil(st.total / size));
    const prev = button("‹ Anterior", "btn btn-sm", () => search(Math.max(0, st.page - 1)));
    const next = button("Próxima ›", "btn btn-sm", () => search(st.page + 1));
    prev.disabled = st.page <= 0;
    next.disabled = st.page + 1 >= pages;
    pager.replaceChildren(prev, el("span", "small muted", `Página ${st.page + 1} de ${pages} · ${st.total} anúncios`), next);
  };

  // ---------- Vender ----------
  let sellSlot = -1;
  let hours = 24;
  const bagGrid = el("div", "auc-bag");
  const sellBox = el("div", "auc-sell");
  const sCount = numInput("Quantidade", "Quantidade");
  const sPrice = numInput("Preço total", "Preço total em moedas");
  const feeLine = el("p", "small auc-fee");
  const hourBtns = el("div", "seg auc-hours");
  const hoursList = () => (rules()?.hours.length ? rules()!.hours : [12, 24, 48]);
  const paintHours = () => {
    hourBtns.replaceChildren();
    for (const h of hoursList()) {
      const b = button(`${h} h`, "");
      b.setAttribute("aria-pressed", String(h === hours));
      b.onclick = () => {
        hours = h;
        paintHours();
      };
      hourBtns.append(b);
    }
  };
  const fee = (price: number) => {
    const r = rules();
    return Math.max(Number(r?.feeMin ?? 10), Math.ceil((price * (r?.feePercent ?? 2)) / 100));
  };
  const commission = (price: number) => Math.floor((price * (rules()?.commissionPercent ?? 5)) / 100);
  const updateFee = () => {
    const p = num(sPrice);
    const r = rules();
    if (!p) {
      feeLine.textContent = `Taxa do anúncio: ${r?.feePercent ?? 2}% do preço (mínimo ${r?.feeMin ?? 10}), não volta. Comissão de ${r?.commissionPercent ?? 5}% na venda.`;
      return;
    }
    const f = fee(p);
    feeLine.textContent = `Taxa: ${fmt.format(f)} moedas agora (não volta) · se vender, você recebe ${fmt.format(p - commission(p))} pelo correio.`;
    feeLine.classList.toggle("bad", f > d.store.gold);
  };
  sPrice.oninput = updateFee;
  const listBtn = button("Anunciar", "btn btn-gold", () => {
    const s = d.store.bag[sellSlot];
    if (!s) return;
    const count = Math.min(num(sCount) || s.count, s.count);
    d.send(AuctionCmdKind.AUCTION_CMD_LIST, { slot: sellSlot, count, price: num(sPrice), hours });
    sellSlot = -1;
    sPrice.value = "";
    renderSell();
  });
  const limitLine = el("p", "small muted");
  panes.sell.append(el("p", "small muted", "Escolha um item da bolsa:"), bagGrid, sellBox, limitLine);

  const renderSell = () => {
    bagGrid.replaceChildren();
    const bag = d.store.bag;
    if (sellSlot >= 0 && !bag[sellSlot]) sellSlot = -1;
    let any = false;
    bag.forEach((s, i) => {
      if (!s || s.info.kind === ItemKind.GOLD) return;
      any = true;
      const b = el("button", `slot auc-bag-slot${i === sellSlot ? " sel" : ""}`);
      b.type = "button";
      b.dataset.grade = String(s.info.grade);
      b.append(art(uniqueIds(itemIcon(s.info))));
      if (s.count > 1) b.append(el("span", "qty", String(s.count)));
      if (s.enchant > 0) b.append(el("span", "ench", `+${s.enchant}`));
      b.setAttribute("aria-label", s.count > 1 ? `${s.count}× ${s.info.name}` : enchantedName(s.info, s.enchant));
      b.setAttribute("aria-pressed", String(i === sellSlot));
      tooltip(b, () => buildItemTip(s.info, { ...d.me(), count: s.count, enchant: s.enchant, hint: "Clique para anunciar" }));
      b.onclick = () => {
        sellSlot = i;
        sCount.value = String(s.count);
        renderSell();
      };
      bagGrid.append(b);
    });
    if (!any) bagGrid.append(el("p", "muted", "Sua bolsa está vazia."));
    sellBox.replaceChildren();
    const s = sellSlot >= 0 ? bag[sellSlot] : null;
    if (s) {
      const c = itemCell(s.info.id, s.count, s.enchant);
      sCount.max = String(s.count);
      sCount.disabled = s.count <= 1;
      const r = rules();
      sPrice.min = String(r?.minPrice ?? 10);
      sPrice.max = String(r?.maxPrice ?? 100000000);
      paintHours();
      sellBox.append(
        el("div", "auc-item auc-chosen", c.icon, el("div", "auc-body", c.name, el("small", "muted", `${s.count} na bolsa`))),
        el("div", "auc-row", el("label", "auc-field", "Quantidade", sCount), el("label", "auc-field", "Preço total", sPrice)),
        el("div", "auc-row", el("span", "small muted", "Prazo"), hourBtns),
        feeLine,
        el("div", "auc-row auc-row-end", listBtn),
      );
      updateFee();
    } else sellBox.append(el("p", "small muted auc-hint", "O item sai da bolsa na hora e fica com o leiloeiro até vender, você cancelar ou o prazo vencer (aí volta pelo correio)."));
    const r = rules();
    limitLine.textContent = `${st.active}/${r?.maxListings ?? 10} anúncios ativos na conta · preço de ${fmt.format(Number(r?.minPrice ?? 10))} a ${fmt.format(Number(r?.maxPrice ?? 100000000))} moedas.`;
  };

  // ---------- Meus anúncios ----------
  const mineList = el("div", "auc-list");
  const claimRow = el("div", "auc-claim");
  panes.mine.append(claimRow, mineList);
  const statusText = (l: AuctionListing) => {
    switch (l.status) {
      case AuctionStatus.SOLD:
        return `vendido para ${l.buyerName}`;
      case AuctionStatus.CANCELLED:
        return "cancelado";
      case AuctionStatus.EXPIRED:
        return "vencido";
      default:
        return `ativo · vence em ${timeLeft(Number(l.expiresAtMs))}`;
    }
  };
  const renderMine = () => {
    claimRow.replaceChildren(
      el("span", "small", st.claimable ? `${st.claimable} ${st.claimable === 1 ? "anexo" : "anexos"} no correio para retirar` : "Nada esperando no correio."),
      button("Retirar tudo", `btn btn-sm ${st.claimable ? "btn-gold" : ""}`, () => d.send(AuctionCmdKind.AUCTION_CMD_CLAIM, { mailId: 0n })),
    );
    mineList.replaceChildren();
    if (!st.mineLoaded) mineList.append(el("p", "muted", "Carregando…"));
    else if (!st.mine.length) mineList.append(el("p", "muted", "Você ainda não anunciou nada."));
    for (const l of st.mine) {
      const c = itemCell(l.itemId, l.count, l.enchant);
      const active = l.status === AuctionStatus.ACTIVE;
      mineList.append(
        el(
          "div",
          `auc-item${active ? "" : " closed"}`,
          c.icon,
          el("div", "auc-body", c.name, el("small", "muted", statusText(l))),
          priceTag(l.price),
          active
            ? confirmButton("Cancelar", "Confirmar?", "btn btn-sm btn-danger", () => d.send(AuctionCmdKind.AUCTION_CMD_CANCEL, { auctionId: l.id }))
            : el("span"),
        ),
      );
    }
  };

  root.append(head, noticeEl, bar.root, panes.buy, panes.sell, panes.mine);

  const renderHead = () => {
    goldVal.textContent = fmt.format(d.store.gold);
    pendingEl.textContent = st.pending ? "registrando…" : "";
    noticeEl.textContent = st.notice;
    noticeEl.hidden = !st.notice;
  };
  const renderAll = () => {
    renderHead();
    renderBuy();
    renderSell();
    renderMine();
  };
  const showTab = (id: AuctionTab) => {
    tab = id;
    d.onTab(id);
    for (const k of Object.keys(panes) as AuctionTab[]) panes[k].hidden = k !== id;
    if (id === "mine") d.send(AuctionCmdKind.AUCTION_CMD_MINE);
    if (id === "buy" && !st.searched) search(0);
  };

  renderAll();
  bar.select(tab, false);
  showTab(tab);
  if (tab !== "buy") search(0);
  const offState = st.onChange(renderAll);
  const offStore = d.store.onChange(() => {
    renderHead();
    renderSell();
    renderBuy();
  });
  root.addEventListener("win-close", () => {
    offState();
    offStore();
  });
  return root;
}

