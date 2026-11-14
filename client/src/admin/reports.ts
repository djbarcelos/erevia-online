/** Seção Relatos: os relatos de /bug (lista, filtros, detalhe com minimapa) e os erros do navegador. */

import { AdminError } from "./api";
import {
  browserOf,
  card,
  copyText,
  empty,
  errorBox,
  fmtDateTime,
  fmtTime,
  h,
  header,
  hint,
  nf,
  PERIODS,
  remember,
  segmented,
  shortVersion,
  showError,
  table,
} from "./dom";
import { panelsApi, type Bug, type BugFacets, type BugFilter, type ClientErrorRow } from "./panels-api";

// ---------- Contador de abertos no menu ----------

let badgeEl: HTMLElement | null = null;

function paintBadge(open: number): void {
  if (!badgeEl) return;
  badgeEl.textContent = open > 99 ? "99+" : String(open);
  badgeEl.hidden = open === 0;
  badgeEl.setAttribute("aria-label", `${open} ${open === 1 ? "relato aberto" : "relatos abertos"}`);
}

/** Põe no item "Relatos" do menu o número de relatos abertos (e atualiza a cada minuto). */
export function mountReportsBadge(nav: HTMLElement): void {
  const link = nav.querySelector<HTMLElement>('a[data-id="relatos"]');
  if (!link) return;
  badgeEl = h("span", { class: "adm-badge", hidden: "" });
  link.append(badgeEl);
  const load = () =>
    panelsApi
      .bugs({ status: "open", limit: 1 })
      .then((r) => r.facets && paintBadge(r.facets.open))
      .catch(() => undefined);
  void load();
  window.setInterval(() => {
    if (document.visibilityState === "visible") void load();
  }, 60_000);
}

// ---------- Minimapa ----------

interface MapBase {
  canvas: HTMLCanvasElement;
  cx(x: number): number;
  cy(z: number): number;
}

let mapBase: Promise<MapBase> | null = null;

/** O mapa do jogo (o mesmo desenho do mapa do cliente), feito uma vez por visita. */
function loadMap(): Promise<MapBase> {
  mapBase ??= (async () => {
    const [{ loadWorldData }, painter] = await Promise.all([import("../world/world-data"), import("../world/map-painter")]);
    const world = await loadWorldData();
    const canvas = painter.renderMapCanvas(world, 2);
    const p = painter.projection(world, canvas.width);
    return { canvas, cx: p.cx, cy: p.cy };
  })();
  mapBase.catch(() => {
    mapBase = null;
  });
  return mapBase;
}

function minimap(x: number, z: number, label: string): HTMLElement {
  const fig = h("figure", { class: "adm-minimap" }, h("div", { class: "adm-minimap-loading" }, "Carregando o mapa…"));
  loadMap()
    .then((m) => {
      const c = document.createElement("canvas");
      c.width = m.canvas.width;
      c.height = m.canvas.height;
      c.setAttribute("role", "img");
      c.setAttribute("aria-label", `Mapa com a posição do relato: ${label}`);
      const ctx = c.getContext("2d")!;
      ctx.drawImage(m.canvas, 0, 0);
      const px = m.cx(x);
      const py = m.cy(z);
      ctx.lineWidth = 2;
      ctx.strokeStyle = "rgba(10,8,16,0.85)";
      ctx.fillStyle = "#ff6b5b";
      ctx.beginPath();
      ctx.arc(px, py, 16, 0, Math.PI * 2);
      ctx.strokeStyle = "rgba(255,107,91,0.9)";
      ctx.stroke();
      ctx.beginPath();
      ctx.arc(px, py, 6, 0, Math.PI * 2);
      ctx.fill();
      ctx.strokeStyle = "#0a0810";
      ctx.stroke();
      // Recorte de 160 m em volta do ponto (o mapa inteiro fica pequeno demais no celular).
      const view = document.createElement("canvas");
      const span = 320; // pixels do mapa (2 px/m)
      view.width = span;
      view.height = span;
      const vx = Math.min(Math.max(px - span / 2, 0), c.width - span);
      const vy = Math.min(Math.max(py - span / 2, 0), c.height - span);
      view.getContext("2d")!.drawImage(c, vx, vy, span, span, 0, 0, span, span);
      view.className = "adm-minimap-zoom";
      view.setAttribute("role", "img");
      view.setAttribute("aria-label", `Detalhe do mapa em volta de ${label}`);
      c.className = "adm-minimap-full";
      fig.replaceChildren(view, c, h("figcaption", {}, label));
    })
    .catch(() => fig.replaceChildren(h("p", { class: "adm-empty" }, "Não deu para carregar o mapa.")));
  return fig;
}

// ---------- Texto da task ----------

function taskText(b: Bug, extra: Record<string, unknown>): string {
  const first = b.text.split("\n")[0].trim();
  const title = first.length > 70 ? `${first.slice(0, 67)}…` : first;
  const pos = `x ${b.x.toFixed(1)}, z ${b.z.toFixed(1)} (${b.region})`;
  const lines = [
    `## Bug #${b.id}: ${title}`,
    "",
    `**Objetivo:** corrigir o que o jogador relatou com /bug: "${b.text.trim()}"`,
    "",
    `- **Quem:** ${b.characterName} (nível ${b.level}), em ${fmtDateTime(b.at)}`,
    `- **Posição:** ${pos}`,
    `- **Versão do servidor:** ${b.version || "desconhecida"}`,
    `- **Navegador:** ${browserOf(b.userAgent)}${b.userAgent ? ` (${b.userAgent})` : ""}`,
  ];
  const keys = Object.keys(extra);
  if (keys.length) {
    lines.push("", "**Contexto do cliente:**");
    for (const k of keys) lines.push(`- ${k}: ${JSON.stringify(extra[k]).slice(0, 400)}`);
  }
  if (b.shot) lines.push("", `**Print:** /api/admin/bug-reports/${b.id}/shot (${b.shot.width}×${b.shot.height}, no portal)`);
  lines.push(
    "",
    "**Passos para reproduzir:**",
    `1. Rodar a versão ${shortVersion(b.version)} (\`git checkout ${b.version || "<versão>"}\`).`,
    `2. Entrar com um personagem de nível ${b.level} e ir até ${pos}.`,
    `3. ${b.text.trim()}`,
    "",
    "**Pronto quando:** o problema não acontece mais nessa posição, com um teste que cubra o caso.",
    "",
    `Relato #${b.id} no portal: /admin#relatos`,
  );
  return lines.join("\n");
}

/** O print da tela do jogo que o /bug completo manda (só o canvas 3D). */
function shotFigure(b: Bug): HTMLElement | null {
  if (!b.shot) return null;
  const src = `/api/admin/bug-reports/${b.id}/shot`;
  const img = h("img", {
    src,
    alt: `Print da tela do jogo no relato #${b.id}`,
    width: String(b.shot.width),
    height: String(b.shot.height),
    class: "adm-shot",
    loading: "lazy",
  });
  return h(
    "figure",
    { class: "adm-shotfig" },
    h("a", { href: src, target: "_blank", rel: "noopener" }, img),
    h("figcaption", {}, `Print do jogo (${b.shot.width}×${b.shot.height}). Toque para abrir em tamanho real.`),
  );
}

/** Campos extras do relato (o contexto do /bug completo e o que outra migração acrescentar). */
function extraFields(extra: Record<string, unknown>): HTMLElement | null {
  const keys = Object.keys(extra);
  if (!keys.length) return null;
  const items = keys.map((k) => {
    const v = extra[k];
    let body: Node;
    if (typeof v === "string" && /^data:image\/(png|jpeg|webp);base64,/.test(v)) {
      body = h("img", { src: v, alt: `Imagem do relato (${k})`, class: "adm-shot", loading: "lazy" });
    } else if (typeof v === "string" && /^\/[\w./-]+\.(png|jpe?g|webp)$/.test(v)) {
      body = h("img", { src: v, alt: `Imagem do relato (${k})`, class: "adm-shot", loading: "lazy" });
    } else if (v && typeof v === "object" && (v as { omitted?: boolean }).omitted) {
      body = h("span", { class: "adm-muted" }, `${nf.format((v as { bytes: number }).bytes)} bytes (grande demais para mostrar aqui)`);
    } else if (v && typeof v === "object") {
      body = h("pre", { class: "adm-pre" }, JSON.stringify(v, null, 2));
    } else {
      body = h("span", {}, String(v));
    }
    return h("div", { class: "adm-kv" }, h("dt", {}, k), h("dd", {}, body));
  });
  return h("section", { class: "adm-extra" }, h("h3", {}, "Contexto do cliente"), h("dl", {}, ...items));
}

// ---------- Página ----------

type Tab = "bugs" | "errors";
const tabPref = remember<Tab>("relatos-aba", "bugs");
const errDaysPref = remember<number>("relatos-erros-dias", 7);

export function renderReports(main: HTMLElement): () => void {
  let alive = true;
  const body = h("div", { class: "adm-page" });
  const tabs = segmented<Tab>(
    "Mostrar",
    [
      { value: "bugs", label: "Relatos de /bug" },
      { value: "errors", label: "Erros do navegador" },
    ],
    tabPref.get(),
    (t) => {
      tabPref.set(t);
      show(t);
    },
  );
  main.append(header("Relatos", tabs), body);
  const show = (t: Tab) => {
    body.replaceChildren();
    if (t === "bugs") bugsTab(body, () => alive);
    else errorsTab(body, () => alive);
  };
  show(tabPref.get());
  return () => {
    alive = false;
  };
}

function bugsTab(root: HTMLElement, alive: () => boolean): void {
  const filter: BugFilter = { status: "open" };
  const error = errorBox();
  const status = segmented(
    "Estado",
    [
      { value: "open", label: "Abertos" },
      { value: "resolved", label: "Resolvidos" },
      { value: "all", label: "Todos" },
    ],
    "open",
    (v) => {
      filter.status = v;
      void load();
    },
  );
  const version = h("select", { "aria-label": "Versão" }, h("option", { value: "" }, "Todas as versões"));
  const region = h("select", { "aria-label": "Região" }, h("option", { value: "" }, "Todas as regiões"));
  const search = h("input", { type: "search", placeholder: "Buscar no texto ou personagem", "aria-label": "Buscar", maxlength: "100" });
  const counts = h("span", { class: "adm-updated" });
  const filters = h("div", { class: "adm-filters" }, status, version, region, search, counts);
  const list = h("div", { class: "adm-buglist", "aria-live": "polite" });
  const more = h("button", { type: "button", class: "adm-btn", hidden: "" }, "Carregar mais");
  const detail = h("aside", { class: "adm-card adm-bugdetail", "data-empty": "" }, empty("Escolha um relato para ver os detalhes."));
  root.append(error, filters, h("div", { class: "adm-split" }, h("div", {}, list, more), detail));

  let rows: Bug[] = [];
  let selected = 0;
  let facetsPainted = false;

  const paintFacets = (f: BugFacets) => {
    paintBadge(f.open);
    counts.textContent = `${nf.format(f.open)} abertos · ${nf.format(f.resolved)} resolvidos`;
    if (facetsPainted) return;
    facetsPainted = true;
    for (const v of f.versions) version.append(h("option", { value: v.value }, `${shortVersion(v.value)} (${v.open}/${v.total})`));
    for (const r of f.regions) region.append(h("option", { value: r.value }, `${r.value} (${r.open}/${r.total})`));
  };

  const item = (b: Bug) => {
    const li = h(
      "button",
      { type: "button", class: `adm-bug${b.id === selected ? " is-selected" : ""}`, "data-id": String(b.id) },
      h(
        "span",
        { class: "adm-bug-head" },
        h("strong", {}, `#${b.id} · ${b.characterName}`),
        h("span", { class: b.resolved ? "adm-pill is-ok" : "adm-pill" }, b.resolved ? "resolvido" : "aberto"),
      ),
      h("span", { class: "adm-bug-text" }, b.text),
      h(
        "span",
        { class: "adm-bug-meta" },
        `${fmtTime(b.at)} · nível ${b.level} · ${b.region} · ${shortVersion(b.version)}${b.shot ? " · com print" : ""}`,
      ),
    );
    li.addEventListener("click", () => {
      selected = b.id;
      for (const el of list.querySelectorAll(".adm-bug")) el.classList.toggle("is-selected", el === li);
      void openDetail(b.id);
    });
    return li;
  };

  const paintList = () => {
    if (!rows.length) {
      list.replaceChildren(empty(filter.status === "open" ? "Nenhum relato aberto com esses filtros." : "Nenhum relato com esses filtros."));
      return;
    }
    list.replaceChildren(...rows.map(item));
  };

  const load = async (append = false) => {
    try {
      const r = await panelsApi.bugs({ ...filter, before: append ? rows[rows.length - 1]?.id : undefined });
      if (!alive()) return;
      rows = append ? rows.concat(r.bugs) : r.bugs;
      if (r.facets) paintFacets(r.facets);
      more.hidden = r.bugs.length < 50;
      paintList();
      error.hidden = true;
    } catch (e) {
      showError(error, e);
    }
  };

  const openDetail = async (id: number) => {
    detail.removeAttribute("data-empty");
    detail.replaceChildren(h("p", { class: "adm-empty" }, "Carregando…"));
    if (matchMedia("(max-width: 820px)").matches) detail.scrollIntoView({ behavior: "smooth", block: "start" });
    try {
      const { bug, extra } = await panelsApi.bug(id);
      if (!alive() || selected !== id) return;
      paintDetail(bug, extra ?? {});
    } catch (e) {
      detail.replaceChildren(h("p", { class: "adm-error" }, e instanceof Error ? e.message : String(e)));
    }
  };

  const paintDetail = (b: Bug, extra: Record<string, unknown>) => {
    const note = h("textarea", { rows: "2", maxlength: "500", placeholder: "Nota (o que foi feito, em qual commit…)", "aria-label": "Nota" });
    const act = h("button", { type: "button", class: "adm-btn is-primary" }, b.resolved ? "Reabrir" : "Marcar como resolvido");
    const copy = h("button", { type: "button", class: "adm-btn" }, "Copiar como task");
    const msg = h("span", { class: "adm-updated", role: "status" });
    copy.addEventListener("click", async () => {
      msg.textContent = (await copyText(taskText(b, extra))) ? "Texto da task copiado." : "Não deu para copiar.";
    });
    act.addEventListener("click", async () => {
      act.disabled = true;
      try {
        const r = await panelsApi.resolveBug(b.id, !b.resolved, note.value);
        rows = rows.map((x) => (x.id === b.id ? r.bug : x)).filter((x) => filter.status === "all" || (filter.status === "resolved") === x.resolved);
        paintList();
        paintDetail(r.bug, extra);
        const f = await panelsApi.bugs({ status: "open", limit: 1 });
        if (f.facets) paintFacets(f.facets);
      } catch (e) {
        msg.textContent = e instanceof AdminError ? e.message : String(e);
        act.disabled = false;
      }
    });
    const resolvedInfo = b.resolved
      ? h(
          "p",
          { class: "adm-resolved" },
          `Resolvido por ${b.resolvedByName || `conta ${b.resolvedBy}`} em ${fmtDateTime(b.resolvedAt ?? b.at)}`,
          b.resolvedNote ? h("span", {}, `: ${b.resolvedNote}`) : null,
        )
      : null;
    const meta = h(
      "dl",
      { class: "adm-meta" },
      ...(
        [
          ["Quando", fmtDateTime(b.at)],
          ["Personagem", `${b.characterName} (nível ${b.level})${b.characterId ? ` · id ${b.characterId}` : ""} · conta ${b.accountId}`],
          ["Posição", `x ${b.x.toFixed(1)}, z ${b.z.toFixed(1)} · ${b.region}`],
          ["Versão", b.version || "—"],
          ["Navegador", browserOf(b.userAgent)],
        ] as const
      ).map(([k, v]) => h("div", { class: "adm-kv" }, h("dt", {}, k), h("dd", { class: k === "Versão" ? "is-mono" : "" }, v))),
    );
    const parts: (Node | null)[] = [
      h("h2", {}, `Relato #${b.id}`),
      h("blockquote", { class: "adm-quote" }, b.text),
      meta,
      b.userAgent ? h("p", { class: "adm-ua" }, b.userAgent) : null,
      shotFigure(b),
      minimap(b.x, b.z, `${b.region} (x ${b.x.toFixed(0)}, z ${b.z.toFixed(0)})`),
      extraFields(extra),
      resolvedInfo,
      b.resolved ? null : note,
      h("div", { class: "adm-actions" }, act, copy, msg),
    ];
    detail.replaceChildren(...parts.filter((p): p is Node => p !== null));
  };

  let debounce = 0;
  search.addEventListener("input", () => {
    clearTimeout(debounce);
    debounce = window.setTimeout(() => {
      filter.q = search.value.trim();
      void load();
    }, 300);
  });
  version.addEventListener("change", () => {
    filter.version = version.value;
    void load();
  });
  region.addEventListener("change", () => {
    filter.region = region.value;
    void load();
  });
  more.addEventListener("click", () => void load(true));
  void load();
}

function errorsTab(root: HTMLElement, alive: () => boolean): void {
  let days = errDaysPref.get();
  let version = "";
  const error = errorBox();
  const period = segmented("Período", PERIODS, days, (d) => {
    days = d;
    errDaysPref.set(d);
    void load();
  });
  const versionSel = h("select", { "aria-label": "Versão" }, h("option", { value: "" }, "Todas as versões"));
  versionSel.addEventListener("change", () => {
    version = versionSel.value;
    void load();
  });
  const top = h("div", {});
  const byVersion = h("div", {});
  root.append(
    error,
    h("div", { class: "adm-filters" }, period, versionSel),
    card(
      "Os mais frequentes",
      hint("Erros iguais (mensagem e primeira linha da stack) de versões diferentes aparecem juntos. Toque num erro para ver a stack."),
      top,
    ),
    card("Por versão do cliente", byVersion),
  );

  const errRow = (e: ClientErrorRow) => {
    const det = h(
      "details",
      { class: "adm-errdet" },
      h("summary", {}, h("code", {}, e.kind), " ", e.message),
      h("pre", { class: "adm-pre" }, e.stack || "(sem stack)"),
      h(
        "p",
        { class: "adm-hint" },
        `Última vez num ${browserOf(e.userAgent)}${e.region ? `, em ${e.region}` : ""}. Para traduzir a stack minificada (ver README, Erros do cliente), com os .map da versão ${shortVersion(e.versions[0] ?? "")}: `,
        h("code", {}, `cd client && node scripts/resolve-stack.mjs --maps maps --id ${e.id}`),
      ),
    );
    return det;
  };

  const load = async () => {
    try {
      const r = await panelsApi.clientErrors(days, version);
      if (!alive()) return;
      error.hidden = true;
      if (versionSel.options.length === 1) {
        for (const v of r.versions) versionSel.append(h("option", { value: v.version }, `${shortVersion(v.version)} (${nf.format(v.total)})`));
      }
      top.replaceChildren(
        r.errors.length
          ? table(
              [
                { label: "Erro", cell: (e) => errRow(e), cls: "adm-wide" },
                { label: "Vezes", cell: (e) => nf.format(e.count), num: true },
                { label: "Versões", cell: (e) => e.versions.map(shortVersion).join(", "), cls: "is-mono" },
                { label: "Última", cell: (e) => fmtTime(e.lastSeen) },
              ],
              r.errors,
            )
          : empty("Nenhum erro do navegador no período."),
      );
      byVersion.replaceChildren(
        r.versions.length
          ? table(
              [
                { label: "Versão", cell: (v) => h("code", { title: v.version }, shortVersion(v.version)) },
                { label: "Erros diferentes", cell: (v) => nf.format(v.distinct), num: true },
                { label: "Ocorrências", cell: (v) => nf.format(v.total), num: true },
                { label: "Última", cell: (v) => fmtTime(v.lastSeen) },
              ],
              r.versions,
            )
          : empty("Nada no período."),
      );
    } catch (e) {
      showError(error, e);
    }
  };
  void load();
}
