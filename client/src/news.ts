/**
 * Novidades (changelog): os dados de public/novidades.json, gerado do CHANGELOG.md da raiz
 * por scripts/build-changelog.mjs, e o desenho de uma versão em HTML.
 *
 * Usado pela janela "O que há de novo" (ui/windows/whatsnew.ts) e pela página pública
 * /novidades (novidades.html). Não depende do kit nem do 3D, para a página ficar leve.
 */

export interface NewsSection {
  title: string;
  items: string[];
}

export interface NewsVersion {
  /** AAAA-MM-DD (ou AAAA-MM-DD.2 para a segunda versão do dia). */
  version: string;
  date: string;
  title: string;
  sections: NewsSection[];
}

export interface News {
  /** A versão mais nova (a primeira de `versions`). */
  latest: string;
  versions: NewsVersion[];
}

let pending: Promise<News | null> | null = null;

/** Busca o novidades.json uma vez por página (null se falhar; tenta de novo na próxima). */
export function loadNews(): Promise<News | null> {
  pending ??= fetch("/novidades.json", { cache: "no-cache" })
    .then((r) => (r.ok ? (r.json() as Promise<News>) : null))
    .then((n) => (n && typeof n.latest === "string" && Array.isArray(n.versions) && n.versions.length ? n : null))
    .catch(() => null)
    .then((n) => {
      if (!n) pending = null;
      return n;
    });
  return pending;
}

const dateFmt = new Intl.DateTimeFormat("pt-BR", { day: "numeric", month: "long", year: "numeric", timeZone: "UTC" });

/** "2026-11-13" → "13 de novembro de 2026". */
export function newsDate(date: string): string {
  const t = Date.parse(`${date}T00:00:00Z`);
  return Number.isNaN(t) ? date : dateFmt.format(t);
}

function make<K extends keyof HTMLElementTagNameMap>(tag: K, cls: string | null, ...kids: (Node | string)[]): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  e.append(...kids);
  return e;
}

/** Texto do item com **negrito** (o único destaque do CHANGELOG), sem innerHTML. */
export function newsText(text: string): DocumentFragment {
  const f = document.createDocumentFragment();
  text.split(/\*\*(.+?)\*\*/g).forEach((part, i) => {
    if (part) f.append(i % 2 ? make("strong", null, part) : part);
  });
  return f;
}

/** Classe de cor de cada seção (news-novidades, news-melhorias, news-correcoes). */
const sectionClass = (title: string) =>
  `news-${title
    .toLowerCase()
    .normalize("NFD")
    .replace(/[^a-z]/g, "")}`;

/** Uma versão: título com a data e as seções com os itens. */
export function newsVersionEl(v: NewsVersion, heading: "h2" | "h3" = "h2"): HTMLElement {
  const root = make("section", "news-version");
  root.dataset.version = v.version;
  root.append(make(heading, "news-title", make("span", "news-date", newsDate(v.date)), " · ", v.title));
  for (const s of v.sections) {
    const list = make("ul", "news-items");
    for (const it of s.items) list.append(make("li", null, newsText(it)));
    root.append(make("div", `news-section ${sectionClass(s.title)}`, make("h4", "news-section-title", s.title), list));
  }
  return root;
}
