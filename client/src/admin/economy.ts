/** Seção Economia: ouro e itens em circulação contra a telemetria, picos, personagem e gm_log. */

import { daysFrom, lineChart } from "./charts";
import {
  card,
  empty,
  errorBox,
  fmtDateTime,
  fmtDay,
  h,
  header,
  hint,
  nf,
  nf1,
  PERIODS,
  remember,
  segmented,
  short,
  showError,
  stat,
  table,
  timingsNote,
} from "./dom";
import { panelsApi, type Economy, type GMLogEntry, type GMLogFilter, type ItemBalance, type Spike } from "./panels-api";

type Tab = "geral" | "personagem" | "gmlog";
const tabPref = remember<Tab>("economia-aba", "geral");
const daysPref = remember<number>("economia-dias", 7);

export function renderEconomy(main: HTMLElement): () => void {
  let alive = true;
  const body = h("div", { class: "adm-page" });
  const tabs = segmented<Tab>(
    "Mostrar",
    [
      { value: "geral", label: "Circulação" },
      { value: "personagem", label: "Por personagem" },
      { value: "gmlog", label: "Registro de GM" },
    ],
    tabPref.get(),
    (t) => show(t),
  );
  main.append(header("Economia", tabs), body);
  const show = (t: Tab, name?: string) => {
    tabPref.set(t);
    for (const b of tabs.querySelectorAll<HTMLButtonElement>("button")) b.setAttribute("aria-pressed", String(b.dataset.v === t));
    body.replaceChildren();
    if (t === "geral") overview(body, () => alive, (n) => show("personagem", n));
    else if (t === "personagem") characterTab(body, () => alive, name);
    else gmLogTab(body, () => alive);
  };
  show(tabPref.get());
  return () => {
    alive = false;
  };
}

const spikeWhat = (s: Spike) => (s.what === "gold" ? "ouro" : "itens");

function spikesTable(spikes: Spike[], onName?: (n: string) => void): HTMLElement {
  if (!spikes.length) return empty("Nenhum dia fora da curva no período.");
  return table(
    [
      {
        label: "Personagem",
        cell: (s) => {
          if (!onName) return s.name;
          const b = h("button", { type: "button", class: "adm-link" }, s.name);
          b.addEventListener("click", () => onName(s.name));
          return b;
        },
      },
      { label: "Dia", cell: (s) => fmtDay(s.day) },
      { label: "Nível", cell: (s) => String(s.level), num: true },
      { label: "Ganhou", cell: (s) => `${nf.format(s.amount)} ${spikeWhat(s)}`, num: true },
      { label: "Média do nível", cell: (s) => nf1.format(s.baseline), num: true },
      { label: "Vezes", cell: (s) => h("strong", { class: "adm-bad" }, `${nf1.format(s.ratio)}×`), num: true },
    ],
    spikes,
  );
}

function overview(root: HTMLElement, alive: () => boolean, onName: (n: string) => void): void {
  let days = daysPref.get();
  let onlySuspicious = true;
  const error = errorBox();
  const stats = h("div", { class: "adm-stats" });
  const content = h("div", { class: "adm-grid2" });
  const foot = h("div", {});
  const period = segmented("Período", PERIODS, days, (d) => {
    days = d;
    daysPref.set(d);
    void load();
  });
  root.append(h("div", { class: "adm-filters" }, period), error, stats, content, foot);
  stats.append(...Array.from({ length: 6 }, () => h("div", { class: "adm-card adm-stat is-loading" })));
  let last: Economy | null = null;

  const itemsCard = (items: ItemBalance[]) => {
    const toggle = h("label", { class: "adm-check" }, h("input", { type: "checkbox" }), " Só os suspeitos");
    const input = toggle.querySelector("input")!;
    input.checked = onlySuspicious;
    const box = h("div", {});
    const paint = () => {
      const rows = onlySuspicious ? items.filter((i) => i.surplus > 0) : items;
      box.replaceChildren(
        rows.length
          ? table(
              [
                { label: "Item", cell: (i) => h("span", {}, i.name || i.item, h("small", { class: "adm-muted" }, ` ${i.item}`)) },
                { label: "Existe", cell: (i) => nf.format(i.existing), num: true },
                { label: "Bolsas", cell: (i) => nf.format(i.bags), num: true },
                { label: "Armazéns", cell: (i) => nf.format(i.storage), num: true },
                { label: "Entrou", cell: (i) => nf.format(i.in), num: true },
                { label: "Saiu", cell: (i) => nf.format(i.out), num: true },
                { label: "Kit (teto)", cell: (i) => nf.format(i.kit), num: true },
                {
                  label: "Sobra",
                  cell: (i) => (i.surplus > 0 ? h("strong", { class: "adm-bad" }, `+${nf.format(i.surplus)}`) : nf.format(i.surplus)),
                  num: true,
                },
              ],
              rows.slice(0, 200),
              (i): Record<string, string> => (i.surplus > 0 ? { class: "is-alert" } : {}),
            )
          : empty(onlySuspicious ? "Nenhum item acima do que a telemetria explica." : "Nenhum item."),
      );
    };
    input.addEventListener("change", () => {
      onlySuspicious = input.checked;
      paint();
    });
    paint();
    return card(
      "Itens: o que existe contra o que entrou e saiu",
      hint(
        "Existe = bolsas, equipamento e armazéns hoje. Esperado = entrou − saiu (todo o histórico da telemetria) + o teto do kit inicial. Sobra acima de zero é item que apareceu sem evento: suspeita de duplicação. Troca entre jogadores e armazém não mudam a conta; poções bebidas e itens perdidos deixam a sobra negativa, o que é normal.",
      ),
      toggle,
      box,
    );
  };

  const paint = (e: Economy) => {
    const s = e.supply;
    stats.replaceChildren(
      stat("Ouro em circulação", short(s.total), `${nf.format(s.total)} moedas`, "is-accent"),
      stat("Nas bolsas", short(s.bags), `${nf.format(s.characters)} personagens`),
      stat("Nos armazéns", short(s.storage), "das contas"),
      stat("Explicado pela telemetria", short(s.expected), "entrou − saiu + ouro inicial"),
      stat(
        "Sobra de ouro",
        `${s.surplus > 0 ? "+" : ""}${short(s.surplus)}`,
        s.surplus > 0 ? "ouro que apareceu sem evento" : "abaixo do esperado (normal)",
        s.surplus > 0 ? "is-bad" : "",
      ),
      stat("Itens suspeitos", nf.format(e.suspicious), e.suspicious ? "com sobra acima de zero" : "nenhuma sobra", e.suspicious ? "is-bad" : ""),
    );
    const daysList = daysFrom(e.since);
    const byDay = new Map((e.goldDaily ?? []).map((d) => [d.day, d]));
    // Circulação no fim de cada dia: a de agora menos o saldo dos dias seguintes.
    let run = s.total;
    const supply: (number | null)[] = new Array(daysList.length).fill(null);
    for (let i = daysList.length - 1; i >= 0; i--) {
      supply[i] = run;
      run -= byDay.get(daysList[i])?.net ?? 0;
    }
    const labels = daysList.map(fmtDay);
    content.replaceChildren(
      card(
        "Ouro em circulação (estimado)",
        lineChart({ labels, caption: "Ouro em circulação no fim de cada dia", series: [{ name: "Em circulação", values: supply }] }),
        hint("A circulação de agora menos o saldo (entrou − saiu) dos dias seguintes."),
      ),
      card(
        "Ouro por dia",
        lineChart({
          labels,
          caption: "Ouro que entrou e saiu por dia",
          series: [
            { name: "Entrou", values: daysList.map((d) => byDay.get(d)?.in ?? 0) },
            { name: "Saiu", values: daysList.map((d) => byDay.get(d)?.out ?? 0) },
          ],
        }),
      ),
      card(
        "Dias fora da curva",
        hint("Um personagem que ganhou num dia 10× a média dos outros do mesmo nível (±1), com pelo menos 500 moedas ou 30 itens."),
        spikesTable(e.spikes ?? [], onName),
      ),
      itemsCard(e.items ?? []),
    );
    content.lastElementChild?.classList.add("is-wide");
    content.children[2]?.classList.add("is-wide");
    foot.replaceChildren(timingsNote(e));
  };

  const load = async () => {
    try {
      const e = await panelsApi.economy(days);
      if (!alive()) return;
      last = e;
      paint(e);
      error.hidden = true;
    } catch (err) {
      if (!last) stats.replaceChildren();
      showError(error, err);
    }
  };
  void load();
}

function characterTab(root: HTMLElement, alive: () => boolean, initial?: string): void {
  let days = daysPref.get();
  const error = errorBox();
  const input = h("input", { type: "search", placeholder: "Nome ou id do personagem", "aria-label": "Personagem", maxlength: "40" });
  const go = h("button", { type: "submit", class: "adm-btn is-primary" }, "Ver histórico");
  const form = h("form", { class: "adm-filters" }, input, go);
  const period = segmented("Período", PERIODS, days, (d) => {
    days = d;
    if (input.value.trim()) void load(input.value.trim());
  });
  form.append(period);
  const out = h("div", { class: "adm-grid2" });
  root.append(form, error, out);

  const load = async (name: string) => {
    go.disabled = true;
    try {
      const r = await panelsApi.character(name, days);
      if (!alive()) return;
      error.hidden = true;
      const c = r.character;
      const daysList = daysFrom(r.since);
      const by = new Map(r.history.map((d) => [d.day, d]));
      const labels = daysList.map(fmtDay);
      const gold = r.flows.filter((f) => f.kind === "gold");
      const items = r.flows.filter((f) => f.kind !== "gold");
      out.replaceChildren(
        h(
          "div",
          { class: "adm-stats is-wide" },
          stat(c.name, `nível ${c.level}`, `${c.class} · id ${c.id} · conta ${c.accountId}`, "is-accent"),
          stat("Ouro agora", nf.format(c.gold), "na bolsa (último salvamento)"),
          stat("Alertas", nf.format(r.spikes.length), r.spikes.length ? "dias fora da curva" : "nenhum dia fora da curva", r.spikes.length ? "is-bad" : ""),
        ),
        card(
          "Ouro por dia",
          lineChart({
            labels,
            caption: `Ouro de ${c.name} por dia`,
            series: [
              { name: "Ganhou", values: daysList.map((d) => by.get(d)?.goldIn ?? 0) },
              { name: "Gastou", values: daysList.map((d) => by.get(d)?.goldOut ?? 0) },
            ],
          }),
        ),
        card(
          "Itens por dia",
          lineChart({
            labels,
            caption: `Itens de ${c.name} por dia`,
            series: [
              { name: "Entraram", values: daysList.map((d) => by.get(d)?.itemsIn ?? 0) },
              { name: "Saíram", values: daysList.map((d) => by.get(d)?.itemsOut ?? 0) },
            ],
          }),
        ),
        card(
          "De onde veio o ouro",
          gold.length
            ? table(
                [
                  { label: "Origem", cell: (f) => h("code", {}, f.ref || "—") },
                  { label: "Moedas", cell: (f) => (f.amount > 0 ? `+${nf.format(f.amount)}` : nf.format(f.amount)), num: true },
                  { label: "Vezes", cell: (f) => nf.format(f.times), num: true },
                ],
                gold,
              )
            : empty("Nenhum ouro no período."),
        ),
        card(
          "Itens",
          items.length
            ? table(
                [
                  { label: "Item", cell: (f) => h("code", {}, f.ref) },
                  { label: "Como", cell: (f) => `${f.kind === "item_in" ? "entrou" : "saiu"} · ${f.how || "—"}` },
                  { label: "Quantos", cell: (f) => nf.format(f.amount), num: true },
                ],
                items,
              )
            : empty("Nenhum item no período."),
        ),
        card("Dias fora da curva", spikesTable(r.spikes)),
      );
      out.lastElementChild?.classList.add("is-wide");
    } catch (e) {
      out.replaceChildren();
      showError(error, e);
    } finally {
      go.disabled = false;
    }
  };
  form.addEventListener("submit", (ev) => {
    ev.preventDefault();
    if (input.value.trim()) void load(input.value.trim());
  });
  if (initial) {
    input.value = initial;
    void load(initial);
  } else {
    out.append(card("Histórico do personagem", empty("Busque um personagem pelo nome (ou pelo id) para ver o ouro e os itens dele por dia.")));
  }
}

function gmLogTab(root: HTMLElement, alive: () => boolean): void {
  const filter: GMLogFilter = {};
  const error = errorBox();
  const gm = h("input", { type: "search", placeholder: "GM", "aria-label": "GM", maxlength: "100" });
  const command = h("select", { "aria-label": "Comando" }, h("option", { value: "" }, "Todos os comandos"), h("option", { value: "portal:" }, "Só o portal"));
  const target = h("input", { type: "search", placeholder: "Alvo", "aria-label": "Alvo", maxlength: "100" });
  const q = h("input", { type: "search", placeholder: "Qualquer campo", "aria-label": "Buscar", maxlength: "100" });
  const period = h(
    "select",
    { "aria-label": "Período" },
    h("option", { value: "" }, "Todo o histórico"),
    ...PERIODS.map((p) => h("option", { value: String(p.value) }, p.label)),
  );
  const list = h("div", {});
  const more = h("button", { type: "button", class: "adm-btn", hidden: "" }, "Carregar mais");
  root.append(h("div", { class: "adm-filters" }, gm, command, target, q, period), error, card("Ações de GM (jogo e portal)", list, more));
  let rows: GMLogEntry[] = [];
  let commandsPainted = false;

  const paint = () => {
    list.replaceChildren(
      rows.length
        ? table(
            [
              { label: "Quando", cell: (e) => fmtDateTime(e.at) },
              { label: "GM", cell: (e) => e.gmName },
              { label: "Ação", cell: (e) => h("code", {}, e.command) },
              { label: "Alvo", cell: (e) => e.target || "—" },
              { label: "Detalhe", cell: (e) => e.detail || "—", cls: "adm-detail" },
            ],
            rows,
          )
        : empty("Nenhuma ação com esses filtros."),
    );
  };
  const load = async (append = false) => {
    try {
      const r = await panelsApi.gmLog({ ...filter, before: append ? rows[rows.length - 1]?.id : undefined });
      if (!alive()) return;
      rows = append ? rows.concat(r.entries) : r.entries;
      if (r.commands && !commandsPainted) {
        commandsPainted = true;
        for (const c of r.commands) command.append(h("option", { value: c.value }, `${c.value} (${nf.format(c.total)})`));
      }
      more.hidden = r.entries.length < 100;
      paint();
      error.hidden = true;
    } catch (e) {
      showError(error, e);
    }
  };
  let debounce = 0;
  const later = () => {
    clearTimeout(debounce);
    debounce = window.setTimeout(() => {
      filter.gm = gm.value.trim();
      filter.target = target.value.trim();
      filter.q = q.value.trim();
      void load();
    }, 300);
  };
  for (const i of [gm, target, q]) i.addEventListener("input", later);
  command.addEventListener("change", () => {
    filter.command = command.value;
    void load();
  });
  period.addEventListener("change", () => {
    filter.days = period.value ? Number(period.value) : undefined;
    void load();
  });
  more.addEventListener("click", () => void load(true));
  void load();
}
