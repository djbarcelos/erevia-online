/** Portal de administração (/admin): menu, rotas por #seção e a página inicial. */

import "./admin.css";
import { AdminError, adminApi, type GMLogEntry, type Overview, type ProgressionTable } from "./api";
import { mountPanelsNav, PANELS } from "./panels";
import { mountOps } from "./ops";
import { mountPlayers } from "./players";

type Child = Node | string | null | undefined | false;

/** Monta um elemento. Texto vira textContent (nada de HTML vindo de dados). */
function h<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  attrs: Record<string, string> = {},
  ...children: Child[]
): HTMLElementTagNameMap[K] {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  for (const c of children) if (c) el.append(c);
  return el;
}

/** Ícones de traço (SVG fixo do código, nunca de dados). */
const ICONS: Record<string, string> = {
  inicio: '<path d="M3 11.5 12 4l9 7.5"/><path d="M5.5 10v9.5h13V10"/><path d="M10 19.5v-5h4v5"/>',
  jogadores: '<circle cx="9" cy="8" r="3.2"/><path d="M3 19.5c.6-3.4 3-5.3 6-5.3s5.4 1.9 6 5.3"/><circle cx="17" cy="9" r="2.4"/><path d="M16.5 14.4c2.3.2 3.9 1.8 4.5 4.6"/>',
  relatos: '<path d="M5 4.5h14v11H10l-5 4v-15z"/><path d="M12 8v3.5"/><path d="M12 13.6v.1"/>',
  metricas: '<path d="M4 19.5h16"/><path d="M7 16v-5"/><path d="M12 16V6"/><path d="M17 16v-8"/>',
  progressao: '<path d="M4 19.5h16"/><path d="M5 16l4.5-4.5 3.5 3L19 8"/><path d="M15 8h4v4"/>',
  economia: '<ellipse cx="12" cy="7" rx="7" ry="2.8"/><path d="M5 7v5c0 1.5 3.1 2.8 7 2.8s7-1.3 7-2.8V7"/><path d="M5 12v5c0 1.5 3.1 2.8 7 2.8s7-1.3 7-2.8v-5"/>',
  operacao: '<circle cx="12" cy="12" r="3"/><path d="M12 3v2.5M12 18.5V21M3 12h2.5M18.5 12H21M5.6 5.6l1.8 1.8M16.6 16.6l1.8 1.8M5.6 18.4l1.8-1.8M16.6 7.4l1.8-1.8"/>',
};

function icon(name: string): SVGSVGElement {
  const span = document.createElement("span");
  span.innerHTML = `<svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">${ICONS[name] ?? ""}</svg>`;
  return span.firstElementChild as SVGSVGElement;
}

interface Section {
  id: string;
  label: string;
  /** Descrição da página vazia ("em breve"). */
  soon?: string;
}

const SECTIONS: Section[] = [
  { id: "inicio", label: "Início" },
  {
    id: "jogadores",
    label: "Jogadores",
    soon: "Busca por personagem ou conta, ficha completa, histórico de moderação e as ações de GM (expulsar, silenciar, banir) com registro no gm_log.",
  },
  {
    id: "relatos",
    label: "Relatos",
    soon: "Os relatos de /bug com personagem, posição, versão e navegador, com filtro e o botão de marcar como resolvido.",
  },
  {
    id: "metricas",
    label: "Métricas",
    soon: "Jogadores por hora, tempo até cada nível, mortes por região e o FPS por aparelho, a partir da telemetria.",
  },
  { id: "progressao", label: "Progressão" },
  {
    id: "economia",
    label: "Economia",
    soon: "Ouro e itens que entram e saem por dia e por origem, os mais ricos e alertas de duplicação.",
  },
  {
    id: "operacao",
    label: "Operação",
    soon: "Anúncios no jogo, quem está online agora e o estado da zona em tempo real.",
  },
];

const REFRESH_MS = 15_000;

const root = document.getElementById("admin")!;
const nav = h("nav", { class: "adm-nav", "aria-label": "Seções" });
const main = h("main", { class: "adm-main", id: "conteudo", tabindex: "-1" });
const who = h("span", { class: "adm-who" });

function shell(): void {
  const brand = h(
    "a",
    { class: "adm-brand", href: "#inicio" },
    h("span", { class: "adm-logo" }, "EREVIA"),
    h("span", { class: "adm-sub" }, "Administração"),
  );
  for (const s of SECTIONS) {
    const a = h("a", { href: `#${s.id}`, "data-id": s.id }, icon(s.id), h("span", {}, s.label));
    nav.append(a);
  }
  const side = h(
    "aside",
    { class: "adm-side" },
    brand,
    nav,
    h("div", { class: "adm-foot" }, who, h("a", { href: "/", class: "adm-back" }, "Voltar ao jogo")),
  );
  root.append(side, main);
}

function current(): Section {
  const id = location.hash.replace(/^#\/?/, "").split(/[/?]/)[0]; // #jogadores/12 → jogadores
  return SECTIONS.find((s) => s.id === id) ?? SECTIONS[0];
}

let refreshTimer = 0;
let tickTimer = 0;
let panelCleanup: (() => void) | undefined;

/** Seções com tela própria (#seção ou #seção/…): montam em main e devolvem a limpeza. */
const PAGES: Record<string, (main: HTMLElement) => () => void> = { jogadores: mountPlayers, operacao: mountOps };
let leavePage: (() => void) | null = null;

function route(): void {
  const s = current();
  for (const a of nav.querySelectorAll("a")) {
    if (a.dataset.id === s.id) a.setAttribute("aria-current", "page");
    else a.removeAttribute("aria-current");
  }
  nav.querySelector<HTMLElement>('[aria-current="page"]')?.scrollIntoView({ block: "nearest", inline: "nearest" });
  document.title = `${s.label} · Administração · Erevia`;
  clearInterval(refreshTimer);
  clearInterval(tickTimer);
  panelCleanup?.();
  panelCleanup = undefined;
  leavePage?.();
  leavePage = null;
  main.replaceChildren();
  if (s.id === "inicio") home();
  else if (s.id === "progressao") progression();
  else if (PANELS[s.id]) panelCleanup = PANELS[s.id](main);
  else if (PAGES[s.id]) leavePage = PAGES[s.id](main);
  else soon(s);
}

function header(title: string, extra?: Node): HTMLElement {
  return h("header", { class: "adm-head" }, h("h1", {}, title), extra);
}

function soon(s: Section): void {
  main.append(
    header(s.label),
    h(
      "section",
      { class: "adm-card adm-soon" },
      icon(s.id),
      h("h2", {}, "Em breve"),
      h("p", {}, s.soon ?? ""),
    ),
  );
}

// ---------- Início ----------

const nf = new Intl.NumberFormat("pt-BR");

function fmtUptime(sec: number): string {
  const d = Math.floor(sec / 86400);
  const hh = Math.floor((sec % 86400) / 3600);
  const mm = Math.floor((sec % 3600) / 60);
  if (d > 0) return `${d} d ${hh} h`;
  if (hh > 0) return `${hh} h ${mm} min`;
  return `${mm} min ${Math.floor(sec % 60)} s`;
}

function fmtTime(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
}

function stat(label: string, value: string, note?: string, accent = false): HTMLElement {
  return h(
    "div",
    { class: accent ? "adm-card adm-stat is-accent" : "adm-card adm-stat" },
    h("span", { class: "adm-stat-label" }, label),
    h("strong", { class: "adm-stat-value" }, value),
    note ? h("span", { class: "adm-stat-note" }, note) : null,
  );
}

function home(): void {
  const updated = h("span", { class: "adm-updated" });
  const refresh = h("button", { class: "adm-btn", type: "button" }, "Atualizar");
  const stats = h("div", { class: "adm-stats", "aria-live": "polite" });
  const log = h("section", { class: "adm-card adm-log" });
  const error = h("p", { class: "adm-error", role: "alert", hidden: "" });
  main.append(header("Início", h("div", { class: "adm-head-tools" }, updated, refresh)), error, stats, log);

  let uptimeEl: HTMLElement | null = null;
  let base = { uptime: 0, at: 0 };

  const paint = (ov: Overview) => {
    const s = ov.summary;
    const ver = ov.version && ov.version !== "dev" ? ov.version.slice(0, 7) : ov.version || "—";
    base = { uptime: ov.uptimeSec, at: performance.now() };
    const up = stat("No ar há", fmtUptime(ov.uptimeSec), `desde ${fmtTime(ov.startedAt)}`);
    uptimeEl = up.querySelector(".adm-stat-value");
    const verCard = stat("Versão no ar", ver, ov.version.length > 7 ? "commit do deploy" : "servidor local");
    const verValue = verCard.querySelector(".adm-stat-value")!;
    verValue.classList.add("is-mono");
    verValue.setAttribute("title", ov.version);
    stats.replaceChildren(
      stat("Online agora", ov.online === null ? "—" : nf.format(ov.online), ov.online === null ? "a zona não respondeu" : "jogadores na zona", true),
      verCard,
      up,
      stat("Contas", nf.format(s.accounts), `+${nf.format(s.accounts24h)} nas últimas 24 h`),
      stat("Personagens", nf.format(s.characters), `+${nf.format(s.characters24h)} nas últimas 24 h`),
      stat("Relatos de /bug", nf.format(s.bugs24h), `nas últimas 24 h · ${nf.format(s.bugsOpen)} ${s.bugsOpen === 1 ? "aberto" : "abertos"}`),
    );
  };

  const paintLog = (entries: GMLogEntry[]) => {
    const title = h("h2", {}, "Últimas ações de GM");
    if (entries.length === 0) {
      log.replaceChildren(title, h("p", { class: "adm-empty" }, "Nenhuma ação registrada ainda."));
      return;
    }
    const rows = entries.map((e) =>
      h(
        "tr",
        {},
        h("td", { "data-label": "Quando" }, fmtTime(e.at)),
        h("td", { "data-label": "GM" }, e.gmName),
        h("td", { "data-label": "Ação" }, h("code", {}, e.command)),
        h("td", { "data-label": "Alvo" }, e.target || "—"),
        h("td", { "data-label": "Detalhe", class: "adm-detail" }, e.detail || "—"),
      ),
    );
    const table = h(
      "table",
      {},
      h("thead", {}, h("tr", {}, ...["Quando", "GM", "Ação", "Alvo", "Detalhe"].map((t) => h("th", { scope: "col" }, t)))),
      h("tbody", {}, ...rows),
    );
    log.replaceChildren(title, h("div", { class: "adm-table" }, table));
  };

  const load = async () => {
    refresh.disabled = true;
    try {
      const [ov, gl] = await Promise.all([adminApi.overview(), adminApi.gmLog(10)]);
      paint(ov);
      paintLog(gl.entries);
      error.hidden = true;
      updated.textContent = `Atualizado às ${new Date().toLocaleTimeString("pt-BR")}`;
    } catch (e) {
      error.textContent = e instanceof Error ? e.message : String(e);
      error.hidden = false;
      if (e instanceof AdminError && e.status === 404) clearInterval(refreshTimer);
    } finally {
      refresh.disabled = false;
    }
  };

  refresh.addEventListener("click", load);
  stats.append(...Array.from({ length: 6 }, () => h("div", { class: "adm-card adm-stat is-loading" })));
  void load();
  refreshTimer = window.setInterval(() => {
    if (document.visibilityState === "visible") void load();
  }, REFRESH_MS);
  tickTimer = window.setInterval(() => {
    if (uptimeEl) uptimeEl.textContent = fmtUptime(base.uptime + (performance.now() - base.at) / 1000);
  }, 1000);
}

// ---------- Progressão ----------

/** "minutos_mediana" → "minutos mediana" (o nome da coluna da view, legível). */
function colLabel(c: string): string {
  return c.replace(/_/g, " ");
}

function fmtCell(v: string | number | boolean | null): string {
  if (v === null) return "—";
  if (typeof v === "number") return nf.format(v);
  if (typeof v === "boolean") return v ? "sim" : "não";
  return v;
}

function progressionTable(t: ProgressionTable): HTMLElement {
  const card = h("section", { class: "adm-card adm-log adm-prog" }, h("h2", {}, t.title), h("p", { class: "adm-prog-note" }, t.note));
  if (t.rows.length === 0) {
    card.append(h("p", { class: "adm-empty" }, "Ainda sem dados."));
    return card;
  }
  const labels = t.columns.map(colLabel);
  const rows = t.rows.map((r) =>
    h("tr", {}, ...r.map((v, i) => h("td", { "data-label": labels[i], class: typeof v === "number" ? "is-num" : "" }, fmtCell(v)))),
  );
  const table = h(
    "table",
    {},
    h("thead", {}, h("tr", {}, ...labels.map((l) => h("th", { scope: "col" }, l)))),
    h("tbody", {}, ...rows),
  );
  card.append(h("div", { class: "adm-table" }, table));
  return card;
}

/** As views da progressão (docs/progressao.md). Sem atualização automática: são consultas pesadas. */
function progression(): void {
  const updated = h("span", { class: "adm-updated" });
  const refresh = h("button", { class: "adm-btn", type: "button" }, "Atualizar");
  const error = h("p", { class: "adm-error", role: "alert", hidden: "" });
  const body = h("div", { class: "adm-prog-list", "aria-live": "polite" });
  main.append(header("Progressão", h("div", { class: "adm-head-tools" }, updated, refresh)), error, body);

  const load = async () => {
    refresh.disabled = true;
    try {
      const { tables } = await adminApi.progression();
      body.replaceChildren(...tables.map(progressionTable));
      error.hidden = true;
      updated.textContent = `Atualizado às ${new Date().toLocaleTimeString("pt-BR")}`;
    } catch (e) {
      error.textContent = e instanceof Error ? e.message : String(e);
      error.hidden = false;
    } finally {
      refresh.disabled = false;
    }
  };
  refresh.addEventListener("click", load);
  body.append(h("div", { class: "adm-card adm-stat is-loading" }));
  void load();
}

// ---------- Início do app ----------

shell();
mountPanelsNav(nav);
window.addEventListener("hashchange", route);
route();
adminApi
  .me()
  .then((me) => {
    who.textContent = `Conectado como ${me.displayName || `conta ${me.accountId}`}`;
  })
  .catch(() => {
    who.textContent = "";
  });
