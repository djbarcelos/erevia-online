/** Seção Métricas: a zona ao vivo e os gráficos da telemetria (game_events), com período. */

import { barChart, daysFrom, hbars, lineChart } from "./charts";
import {
  card,
  empty,
  errorBox,
  fmtDay,
  fmtTime,
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
import { panelsApi, type Live, type Metrics } from "./panels-api";

const LIVE_MS = 5_000;
const daysPref = remember<number>("metricas-dias", 7);

function fmtUptime(sec: number): string {
  const d = Math.floor(sec / 86400);
  const hh = Math.floor((sec % 86400) / 3600);
  const mm = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d} d ${hh} h`;
  if (hh > 0) return `${hh} h ${mm} min`;
  return `${mm} min`;
}

const ms = (v: number) => `${nf1.format(v)} ms`;

export function renderMetrics(main: HTMLElement): () => void {
  let alive = true;
  let days = daysPref.get();
  const liveError = errorBox();
  const liveBox = h("div", { class: "adm-stats", "aria-live": "polite" });
  const regions = h("div", {});
  const liveNote = h("span", { class: "adm-updated" });
  const error = errorBox();
  const charts = h("div", { class: "adm-grid2" });
  const foot = h("div", {});
  const period = segmented("Período", PERIODS, days, (d) => {
    days = d;
    daysPref.set(d);
    void loadCharts();
  });
  main.append(
    header("Métricas", liveNote),
    liveError,
    liveBox,
    regions,
    h("div", { class: "adm-section-head" }, h("h2", {}, "Telemetria"), period),
    error,
    charts,
    foot,
  );
  liveBox.append(...Array.from({ length: 6 }, () => h("div", { class: "adm-card adm-stat is-loading" })));

  const paintLive = (l: Live) => {
    const z = l.zone;
    const t = z?.lastTick ?? null;
    const cur = z?.currentTick;
    const tickNote = t
      ? `média de ${nf.format(t.ticks)} ticks (30 s até ${fmtTime(t.until)}); agora ${cur && cur.ticks ? ms(cur.avgMs) : "—"}`
      : "o primeiro período de 30 s ainda não fechou";
    const worst = t ? (t.worstMs > t.budgetMs ? "is-bad" : t.worstMs > t.budgetMs * 0.6 ? "is-warn" : "") : "";
    const ver = l.version && l.version !== "dev" ? l.version.slice(0, 7) : l.version || "—";
    const verCard = stat("Versão no ar", ver, `no ar há ${fmtUptime(l.uptimeSec)}`);
    verCard.querySelector(".adm-stat-value")!.classList.add("is-mono");
    verCard.querySelector(".adm-stat-value")!.setAttribute("title", l.version);
    liveBox.replaceChildren(
      stat("Online agora", z ? nf.format(z.online) : "—", z ? "jogadores na zona" : "a zona não respondeu", "is-accent"),
      stat("Monstros", z ? nf.format(z.monsters) : "—", z ? `vivos · ${nf.format(z.monstersDead)} esperando renascer` : ""),
      stat("Itens no chão", z ? nf.format(z.ground) : "—", "drops e itens largados"),
      stat("Tick médio", t ? ms(t.avgMs) : "—", tickNote),
      stat(
        "Pior tick",
        t ? ms(t.worstMs) : "—",
        t ? `orçamento ${ms(t.budgetMs)} · ${nf.format(t.slowTicks)} acima · ${nf.format(t.eventsPerSec)} eventos/s` : "",
        worst,
      ),
      verCard,
    );
    if (z) {
      const rows = l.regions.map((r) => ({ label: r, value: z.byRegion[r] ?? 0 }));
      regions.replaceChildren(card("Online por região", hbars(rows)));
    } else regions.replaceChildren();
    liveNote.textContent = `Ao vivo · ${new Date().toLocaleTimeString("pt-BR")}`;
  };

  const loadLive = async () => {
    try {
      const l = await panelsApi.live();
      if (!alive) return;
      paintLive(l);
      liveError.hidden = true;
    } catch (e) {
      if (alive) showError(liveError, e);
    }
  };

  const loadCharts = async () => {
    charts.classList.add("is-busy");
    try {
      const m = await panelsApi.metrics(days);
      if (!alive) return;
      paintCharts(charts, m);
      foot.replaceChildren(timingsNote(m));
      error.hidden = true;
    } catch (e) {
      if (alive) showError(error, e);
    } finally {
      charts.classList.remove("is-busy");
    }
  };

  void loadLive();
  void loadCharts();
  const timer = window.setInterval(() => {
    if (document.visibilityState === "visible") void loadLive();
  }, LIVE_MS);
  return () => {
    alive = false;
    clearInterval(timer);
  };
}

/** Soma por dia uma lista de linhas {day, …}. */
function byDay<T extends { day: string }>(rows: T[] | null, labels: string[], pick: (r: T) => number): number[] {
  const m = new Map<string, number>();
  for (const r of rows ?? []) m.set(r.day, (m.get(r.day) ?? 0) + pick(r));
  return labels.map((d) => m.get(d) ?? 0);
}

const SERVER_KINDS: [string, string][] = [
  ["server_error", "Panics"],
  ["slow_tick", "Ticks lentos"],
  ["db_error", "Erros do banco"],
  ["invariant", "Invariantes"],
];

function paintCharts(root: HTMLElement, m: Metrics): void {
  const days = daysFrom(m.since);
  const labels = days.map(fmtDay);
  const tipDay = (i: number) => new Date(`${days[i]}T12:00:00Z`).toLocaleDateString("pt-BR", { weekday: "short", day: "2-digit", month: "2-digit" });
  const out: HTMLElement[] = [];

  out.push(
    card(
      "Quem jogou por dia",
      lineChart({
        labels,
        tipLabel: tipDay,
        caption: "Personagens distintos por dia",
        series: [{ name: "Personagens", values: byDay(m.players, days, (r) => r.characters) }],
      }),
      hint(
        `${nf.format((m.players ?? []).reduce((a, r) => a + r.sessions, 0))} sessões e ${short((m.players ?? []).reduce((a, r) => a + r.minutes, 0))} minutos jogados no período.`,
      ),
    ),
  );

  out.push(
    card(
      "Ouro que entrou e saiu",
      lineChart({
        labels,
        tipLabel: tipDay,
        caption: "Ouro por dia",
        series: [
          { name: "Entrou", values: byDay(m.gold, days, (r) => r.in) },
          { name: "Saiu", values: byDay(m.gold, days, (r) => r.out) },
        ],
      }),
    ),
  );

  const bySource = new Map<string, { in: number; out: number }>();
  for (const g of m.gold ?? []) {
    const v = bySource.get(g.source) ?? { in: 0, out: 0 };
    v.in += g.in;
    v.out += g.out;
    bySource.set(g.source, v);
  }
  const sources = [...bySource.entries()].sort((a, b) => b[1].in + b[1].out - (a[1].in + a[1].out));
  out.push(
    card(
      "Ouro por origem",
      sources.length
        ? hbars(
            sources.map(([k, v]) => ({ label: k || "—", value: v.in + v.out, note: v.in >= v.out ? `+${short(v.in - v.out)}` : `−${short(v.out - v.in)}` })),
            short,
          )
        : empty("Nenhum ouro no período."),
      hint("Barra: ouro movimentado (entrou + saiu). Ao lado do nome, o saldo da origem."),
    ),
  );

  out.push(
    card(
      "Itens que entraram e saíram do jogo",
      lineChart({
        labels,
        tipLabel: tipDay,
        caption: "Itens por dia",
        series: [
          { name: "Entraram", values: byDay(m.itemsDaily, days, (r) => r.in) },
          { name: "Saíram", values: byDay(m.itemsDaily, days, (r) => r.out) },
        ],
      }),
      (m.itemsTop ?? []).length
        ? table(
            [
              { label: "Item", cell: (r) => h("code", {}, r.item) },
              { label: "Entrou", cell: (r) => nf.format(r.in), num: true },
              { label: "Saiu", cell: (r) => nf.format(r.out), num: true },
            ],
            (m.itemsTop ?? []).slice(0, 8),
          )
        : null,
    ),
  );

  const levels = m.levels ?? [];
  out.push(
    card(
      "Tempo até cada nível",
      levels.length
        ? barChart({
            labels: levels.map((l) => String(l.level)),
            caption: "Minutos médios para chegar a cada nível",
            format: (v) => `${nf1.format(v)} min`,
            tipLabel: (i) => `Nível ${levels[i].level} · ${nf.format(levels[i].characters)} personagens`,
            series: [{ name: "Minutos", values: levels.map((l) => l.avgMinutes) }],
          })
        : empty("Nenhuma subida de nível no período (com a anterior também no período)."),
      hint("Minutos de relógio entre uma subida e a seguinte do mesmo personagem."),
    ),
  );

  const deathsByRegion = new Map<string, number>();
  for (const d of m.deaths ?? []) deathsByRegion.set(d.region, (deathsByRegion.get(d.region) ?? 0) + d.deaths);
  const causes = (m.deaths ?? []).slice(0, 6);
  out.push(
    card(
      "Mortes por região",
      deathsByRegion.size
        ? hbars([...deathsByRegion.entries()].sort((a, b) => b[1] - a[1]).map(([label, value]) => ({ label, value })))
        : empty("Nenhuma morte no período."),
      causes.length
        ? table(
            [
              { label: "Região", cell: (d) => d.region },
              { label: "Quem matou", cell: (d) => h("code", {}, d.cause || "—") },
              { label: "Mortes", cell: (d) => nf.format(d.deaths), num: true },
              { label: "Nível médio", cell: (d) => nf1.format(d.avgLevel), num: true },
            ],
            causes,
          )
        : null,
    ),
  );

  const quit = m.quit ?? [];
  out.push(
    card(
      "Onde as pessoas param",
      quit.length
        ? barChart({
            labels: quit.map((q) => String(q.level)),
            caption: "Último nível de quem não joga há mais de 7 dias",
            tipLabel: (i) => `Nível ${quit[i].level}`,
            series: [{ name: "Personagens", values: quit.map((q) => q.characters) }],
          })
        : empty("Ninguém parado há mais de 7 dias."),
      hint("Último nível de quem não joga há mais de 7 dias (não depende do período)."),
    ),
  );

  const fps = [...(m.fps ?? [])].sort((a, b) => a.avgFps - b.avgFps);
  out.push(
    card(
      "FPS por região e aparelho",
      fps.length
        ? hbars(
            fps.map((f) => ({ label: `${f.region} · ${f.device || "?"}`, value: f.avgFps, note: `piores ${nf1.format(f.avgLow)} · ${nf.format(f.samples)} amostras` })),
            (v) => nf1.format(v),
          )
        : empty("Nenhuma amostra de FPS no período."),
      hint("Média do FPS (amostras de 60 s dos clientes), do pior para o melhor; ao lado, a média dos piores momentos."),
    ),
  );

  const serverSeries = SERVER_KINDS.map(([kind, name]) => ({
    name,
    values: byDay(
      (m.serverDays ?? []).filter((r) => r.kind === kind),
      days,
      (r) => r.count,
    ),
  }));
  const top = m.serverTop ?? [];
  out.push(
    card(
      "Erros do servidor",
      lineChart({ labels, tipLabel: tipDay, caption: "Erros do servidor por dia", series: serverSeries }),
      top.length
        ? table(
            [
              { label: "Tipo", cell: (r) => h("code", {}, r.kind) },
              { label: "Grupo", cell: (r) => h("span", { class: "adm-ref", title: r.ref }, r.where ? `${r.where} · ${r.ref}` : r.ref) },
              { label: "Vezes", cell: (r) => nf.format(r.count), num: true },
              { label: "Última", cell: (r) => fmtTime(r.last) },
              { label: "Mensagem", cell: (r) => r.message || "—", cls: "adm-detail" },
            ],
            top.slice(0, 10),
          )
        : empty("Nenhum erro do servidor no período."),
    ),
  );
  out[out.length - 1].classList.add("is-wide");
  root.replaceChildren(...out);
}
