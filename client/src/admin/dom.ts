/** Utilitários de DOM e formatação dos painéis de leitura (Relatos, Métricas, Economia). */

export type Child = Node | string | null | undefined | false;

/** Monta um elemento. Texto vira textContent (nada de HTML vindo de dados). */
export function h<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  attrs: Record<string, string> = {},
  ...children: Child[]
): HTMLElementTagNameMap[K] {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v);
  for (const c of children) if (c) el.append(c);
  return el;
}

export const nf = new Intl.NumberFormat("pt-BR");
export const nf1 = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 });

/** Número curto: 1,2 mil, 3,4 mi. */
export function short(n: number): string {
  const a = Math.abs(n);
  if (a >= 1e6) return `${nf1.format(n / 1e6)} mi`;
  if (a >= 1e4) return `${nf1.format(n / 1e3)} mil`;
  return nf.format(n);
}

export function fmtTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
}

export function fmtDateTime(iso: string): string {
  return new Date(iso).toLocaleString("pt-BR", { dateStyle: "short", timeStyle: "short" });
}

/** "2026-11-13" → "13/11". */
export function fmtDay(day: string): string {
  const [, m, d] = day.split("-");
  return `${d}/${m}`;
}

export function shortVersion(v: string): string {
  if (!v) return "—";
  return v.length > 12 ? v.slice(0, 7) : v;
}

/** "Chrome 131 · Windows" a partir do user agent (sem biblioteca: só o que importa). */
export function browserOf(ua: string): string {
  if (!ua) return "—";
  const pick = (re: RegExp) => ua.match(re)?.[1];
  const browser =
    (pick(/Edg\/(\d+)/) && `Edge ${pick(/Edg\/(\d+)/)}`) ||
    (pick(/OPR\/(\d+)/) && `Opera ${pick(/OPR\/(\d+)/)}`) ||
    (pick(/Firefox\/(\d+)/) && `Firefox ${pick(/Firefox\/(\d+)/)}`) ||
    (pick(/Chrome\/(\d+)/) && `Chrome ${pick(/Chrome\/(\d+)/)}`) ||
    (pick(/Version\/(\d+)[\d.]* .*Safari/) && `Safari ${pick(/Version\/(\d+)/)}`) ||
    "Navegador";
  const os = /Android/.test(ua)
    ? "Android"
    : /iPhone|iPad|iPod/.test(ua)
      ? "iOS"
      : /Windows/.test(ua)
        ? "Windows"
        : /Mac OS X/.test(ua)
          ? "macOS"
          : /Linux/.test(ua)
            ? "Linux"
            : "";
  return os ? `${browser} · ${os}` : browser;
}

export function header(title: string, extra?: Node): HTMLElement {
  return h("header", { class: "adm-head" }, h("h1", {}, title), extra);
}

export function errorBox(): HTMLParagraphElement {
  return h("p", { class: "adm-error", role: "alert", hidden: "" });
}

export function showError(box: HTMLElement, e: unknown): void {
  box.textContent = e instanceof Error ? e.message : String(e);
  box.hidden = false;
}

export function stat(label: string, value: string, note?: string, cls = ""): HTMLElement {
  return h(
    "div",
    { class: `adm-card adm-stat ${cls}`.trim() },
    h("span", { class: "adm-stat-label" }, label),
    h("strong", { class: "adm-stat-value" }, value),
    note ? h("span", { class: "adm-stat-note" }, note) : null,
  );
}

/** Cartão com título (e um texto de apoio) e o conteúdo. */
export function card(title: string, ...children: Child[]): HTMLElement {
  return h("section", { class: "adm-card adm-panel" }, h("h2", {}, title), ...children);
}

export function hint(text: string): HTMLElement {
  return h("p", { class: "adm-hint" }, text);
}

export function empty(text: string): HTMLElement {
  return h("p", { class: "adm-empty" }, text);
}

export interface Column<T> {
  label: string;
  cell: (row: T) => Child;
  num?: boolean;
  cls?: string;
}

/** Tabela que vira lista de fichas no celular (data-label em cada célula). */
export function table<T>(cols: Column<T>[], rows: T[], rowAttrs?: (row: T) => Record<string, string>): HTMLElement {
  const head = h("tr", {}, ...cols.map((c) => h("th", { scope: "col", class: c.num ? "is-num" : "" }, c.label)));
  const body = rows.map((r) =>
    h(
      "tr",
      rowAttrs?.(r) ?? {},
      ...cols.map((c) => h("td", { "data-label": c.label, class: [c.num ? "is-num" : "", c.cls ?? ""].join(" ").trim() }, c.cell(r))),
    ),
  );
  return h("div", { class: "adm-table" }, h("table", {}, h("thead", {}, head), h("tbody", {}, ...body)));
}

/** Botões de escolha única (período, abas). */
export function segmented<V extends string | number>(
  label: string,
  options: { value: V; label: string }[],
  value: V,
  onChange: (v: V) => void,
): HTMLElement {
  const box = h("div", { class: "adm-seg", role: "group", "aria-label": label });
  const paint = (v: V) => {
    for (const b of box.querySelectorAll<HTMLButtonElement>("button")) b.setAttribute("aria-pressed", String(b.dataset.v === String(v)));
  };
  for (const o of options) {
    const b = h("button", { type: "button", "data-v": String(o.value) }, o.label);
    b.addEventListener("click", () => {
      paint(o.value);
      onChange(o.value);
    });
    box.append(b);
  }
  paint(value);
  return box;
}

export const PERIODS = [
  { value: 1, label: "24 h" },
  { value: 7, label: "7 dias" },
  { value: 30, label: "30 dias" },
  { value: 90, label: "90 dias" },
];

/** Lembra uma escolha do GM neste navegador (aba, período). Sem armazenamento, não lembra. */
export function remember<T extends string | number>(key: string, fallback: T): { get(): T; set(v: T): void } {
  const k = `erevia-admin:${key}`;
  return {
    get() {
      try {
        const v = localStorage.getItem(k);
        if (v === null) return fallback;
        return (typeof fallback === "number" ? Number(v) : v) as T;
      } catch {
        return fallback;
      }
    },
    set(v: T) {
      try {
        localStorage.setItem(k, String(v));
      } catch {
        /* sem armazenamento */
      }
    },
  };
}

/** Rodapé com quando os dados foram gerados e quanto cada família de consultas levou. */
export function timingsNote(r: { generatedAt: string; timingsMs: Record<string, number>; slowestMs?: Record<string, number>; queries?: Record<string, number> }): HTMLElement {
  const parts = Object.entries(r.timingsMs).map(([k, v]) => {
    const n = r.queries?.[k] ?? 0;
    const slow = r.slowestMs?.[k];
    return n > 1 && slow !== undefined
      ? `${k} ${nf1.format(v)} ms (${n} consultas, a mais lenta ${nf1.format(slow)} ms)`
      : `${k} ${nf1.format(v)} ms`;
  });
  return h(
    "p",
    { class: "adm-timings" },
    `Dados de ${new Date(r.generatedAt).toLocaleTimeString("pt-BR")} (guardados por 1 minuto; os dias que já acabaram ficam guardados no servidor e só o de hoje volta ao banco) · ${parts.join(" · ")}`,
  );
}

/** Copia texto (com a API do navegador ou, sem ela, pelo jeito antigo). */
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    const ta = h("textarea", { readonly: "", class: "adm-offscreen" });
    ta.value = text;
    document.body.append(ta);
    ta.select();
    let ok = false;
    try {
      ok = document.execCommand("copy");
    } catch {
      ok = false;
    }
    ta.remove();
    return ok;
  }
}
