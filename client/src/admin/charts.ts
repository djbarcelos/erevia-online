/**
 * Gráficos dos painéis em SVG desenhado à mão (sem biblioteca): linhas e barras verticais,
 * com grade discreta, legenda quando há mais de uma série, rótulo no fim de cada linha e
 * dica (tooltip) ao passar o dedo ou o mouse. Redesenha na largura real do cartão.
 */

import { h, nf, short } from "./dom";

const SVG = "http://www.w3.org/2000/svg";

/** Paleta categórica (validada para o fundo escuro do portal; ordem fixa, nunca reciclada). */
export const SERIES = ["#3987e5", "#d95926", "#199e70", "#c98500", "#d55181", "#9085e9"];
/** Uma série só: o ouro do portal. */
export const SINGLE = "#d6b46e";

function s<K extends keyof SVGElementTagNameMap>(tag: K, attrs: Record<string, string | number> = {}): SVGElementTagNameMap[K] {
  const el = document.createElementNS(SVG, tag);
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, String(v));
  return el;
}

/** Escala "bonita" de 0 até pelo menos max, com ~4 divisões. */
function niceTicks(max: number, min = 0): number[] {
  const span = Math.max(max - min, 1e-9);
  const raw = span / 4;
  const mag = 10 ** Math.floor(Math.log10(raw));
  const step = [1, 2, 2.5, 5, 10].map((m) => m * mag).find((v) => v >= raw) ?? raw;
  const lo = Math.floor(min / step) * step;
  const out: number[] = [];
  for (let v = lo; v <= max + step * 0.999; v += step) out.push(Math.round(v * 1e6) / 1e6);
  if (out.length < 2) out.push(lo + step);
  return out;
}

export interface Series {
  name: string;
  values: (number | null)[];
  color?: string;
}

export interface ChartOpts {
  labels: string[];
  series: Series[];
  /** Formato dos valores (eixo e dica). */
  format?: (n: number) => string;
  height?: number;
  /** Rótulo de cada ponto na dica (padrão: o próprio label). */
  tipLabel?: (i: number) => string;
  /** Texto da tabela acessível (o leitor de tela lê a tabela, não o desenho). */
  caption: string;
}

/** Envolve um desenho que depende da largura e o refaz quando o cartão muda de tamanho. */
function responsive(box: HTMLElement, draw: (width: number) => void): void {
  let last = 0;
  const ro = new ResizeObserver((entries) => {
    const w = Math.round(entries[0].contentRect.width);
    if (w > 0 && Math.abs(w - last) > 2) {
      last = w;
      draw(w);
    }
  });
  ro.observe(box);
}

function legend(series: Series[]): HTMLElement | null {
  if (series.length < 2) return null;
  return h(
    "ul",
    { class: "adm-legend" },
    ...series.map((se, i) => {
      const sw = h("span", { class: "adm-swatch" });
      sw.style.background = se.color ?? SERIES[i % SERIES.length];
      return h("li", {}, sw, se.name);
    }),
  );
}

/** Tabela escondida com os mesmos números (leitores de tela e quem quiser copiar). */
function srTable(o: ChartOpts): HTMLElement {
  const fmt = o.format ?? ((n: number) => nf.format(n));
  return h(
    "table",
    { class: "adm-sr" },
    h("caption", {}, o.caption),
    h("thead", {}, h("tr", {}, h("th", {}, ""), ...o.series.map((se) => h("th", {}, se.name)))),
    h(
      "tbody",
      {},
      ...o.labels.map((l, i) =>
        h("tr", {}, h("th", {}, l), ...o.series.map((se) => h("td", {}, se.values[i] == null ? "—" : fmt(se.values[i]!)))),
      ),
    ),
  );
}

interface Frame {
  w: number;
  ht: number;
  left: number;
  right: number;
  top: number;
  bottom: number;
  y: (v: number) => number;
  ticks: number[];
}

function frame(svg: SVGSVGElement, w: number, ht: number, max: number, min: number, fmt: (n: number) => string, right: number): Frame {
  const ticks = niceTicks(max, min);
  const top = 10;
  const bottom = 24;
  const labelW = Math.max(...ticks.map((t) => fmt(t).length)) * 6.6 + 10;
  const left = Math.min(70, Math.max(30, labelW));
  const lo = ticks[0];
  const hi = ticks[ticks.length - 1];
  const y = (v: number) => top + (ht - top - bottom) * (1 - (v - lo) / (hi - lo || 1));
  for (const t of ticks) {
    const yy = y(t);
    svg.append(s("line", { x1: left, x2: w - right, y1: yy, y2: yy, class: t === 0 ? "adm-axis" : "adm-grid" }));
    const tx = s("text", { x: left - 6, y: yy + 4, "text-anchor": "end", class: "adm-tick" });
    tx.textContent = fmt(t);
    svg.append(tx);
  }
  return { w, ht, left, right, top, bottom, y, ticks };
}

function xLabels(svg: SVGSVGElement, f: Frame, labels: string[], xAt: (i: number) => number): void {
  const room = (f.w - f.left - f.right) / Math.max(labels.length, 1);
  const every = Math.max(1, Math.ceil(42 / room));
  labels.forEach((l, i) => {
    if (i % every !== 0 && i !== labels.length - 1) return;
    if (i !== labels.length - 1 && labels.length - 1 - i < every && i % every === 0 && i !== 0) return;
    const t = s("text", { x: xAt(i), y: f.ht - 6, "text-anchor": "middle", class: "adm-tick" });
    t.textContent = l;
    svg.append(t);
  });
}

/** Dica que segue o ponteiro: mostra o rótulo e o valor de cada série no índice i. */
function attachTip(
  box: HTMLElement,
  svg: SVGSVGElement,
  f: Frame,
  n: number,
  xAt: (i: number) => number,
  render: (i: number) => { title: string; rows: [string, string, string][] },
  onIndex?: (i: number | null) => void,
): void {
  const tip = h("div", { class: "adm-tip", role: "status" });
  tip.hidden = true;
  box.append(tip);
  const hit = s("rect", { x: f.left, y: 0, width: Math.max(0, f.w - f.left - f.right), height: f.ht, fill: "transparent" });
  svg.append(hit);
  const move = (ev: PointerEvent) => {
    const r = svg.getBoundingClientRect();
    const px = ev.clientX - r.left;
    let best = 0;
    let dist = Infinity;
    for (let i = 0; i < n; i++) {
      const d = Math.abs(xAt(i) - px);
      if (d < dist) [best, dist] = [i, d];
    }
    const c = render(best);
    tip.replaceChildren(
      h("strong", {}, c.title),
      ...c.rows.map(([color, name, value]) => {
        const sw = h("span", { class: "adm-swatch" });
        sw.style.background = color;
        return h("span", { class: "adm-tip-row" }, sw, h("span", {}, name), h("b", {}, value));
      }),
    );
    tip.hidden = false;
    const tx = xAt(best);
    const tw = tip.offsetWidth;
    tip.style.left = `${Math.min(Math.max(tx - tw / 2, 0), f.w - tw)}px`;
    tip.style.top = `0px`;
    onIndex?.(best);
  };
  hit.addEventListener("pointermove", move);
  hit.addEventListener("pointerdown", move);
  hit.addEventListener("pointerleave", () => {
    tip.hidden = true;
    onIndex?.(null);
  });
}

/** Gráfico de linhas (uma ou mais séries no mesmo eixo). */
export function lineChart(o: ChartOpts): HTMLElement {
  const fmt = o.format ?? short;
  const box = h("div", { class: "adm-chart" });
  const wrap = h("figure", { class: "adm-figure" }, legend(o.series), box, srTable(o));
  const ht = o.height ?? 200;
  const all = o.series.flatMap((se) => se.values.filter((v): v is number => v != null));
  const max = Math.max(0, ...all);
  const min = Math.min(0, ...all);
  responsive(box, (w) => {
    box.replaceChildren();
    const labeled = o.series.length > 1 && o.series.length <= 4 && w > 420;
    const nameW = labeled ? Math.min(110, Math.max(...o.series.map((se) => se.name.length)) * 6.6 + 12) : 0;
    const svg = s("svg", { width: w, height: ht, viewBox: `0 0 ${w} ${ht}`, "aria-hidden": "true" });
    const f = frame(svg, w, ht, max, min, fmt, 8 + nameW);
    const n = o.labels.length;
    const xAt = (i: number) => (n <= 1 ? (f.left + w - f.right) / 2 : f.left + ((w - f.left - f.right) * i) / (n - 1));
    xLabels(svg, f, o.labels, xAt);
    const cross = s("line", { y1: f.top, y2: ht - f.bottom, class: "adm-cross", visibility: "hidden" });
    svg.append(cross);
    const dots: SVGCircleElement[][] = [];
    o.series.forEach((se, si) => {
      const color = se.color ?? (o.series.length === 1 ? SINGLE : SERIES[si % SERIES.length]);
      let d = "";
      let pen = false;
      se.values.forEach((v, i) => {
        if (v == null) {
          pen = false;
          return;
        }
        d += `${pen ? "L" : "M"}${xAt(i).toFixed(1)},${f.y(v).toFixed(1)}`;
        pen = true;
      });
      svg.append(s("path", { d, fill: "none", stroke: color, "stroke-width": 2, "stroke-linejoin": "round", "stroke-linecap": "round" }));
      const ds: SVGCircleElement[] = [];
      se.values.forEach((v, i) => {
        const c = s("circle", { cx: xAt(i), cy: v == null ? -10 : f.y(v), r: n <= 12 ? 3 : 0, fill: color, class: "adm-dot" });
        if (n <= 1 && v != null) c.setAttribute("r", "4");
        svg.append(c);
        ds.push(c);
      });
      dots.push(ds);
      if (labeled) {
        let lastI = se.values.length - 1;
        while (lastI >= 0 && se.values[lastI] == null) lastI--;
        if (lastI >= 0) {
          const t = s("text", { x: xAt(lastI) + 8, y: f.y(se.values[lastI]!) + 4, class: "adm-direct" });
          t.textContent = se.name;
          svg.append(t);
        }
      }
    });
    // Rótulos de fim de linha não podem se sobrepor: empurra para baixo os que colidem.
    const direct = Array.from(svg.querySelectorAll<SVGTextElement>(".adm-direct")).sort(
      (a, b) => Number(a.getAttribute("y")) - Number(b.getAttribute("y")),
    );
    for (let i = 1; i < direct.length; i++) {
      const prev = Number(direct[i - 1].getAttribute("y"));
      if (Number(direct[i].getAttribute("y")) - prev < 13) direct[i].setAttribute("y", String(prev + 13));
    }
    box.append(svg);
    attachTip(
      box,
      svg,
      f,
      n,
      xAt,
      (i) => ({
        title: o.tipLabel?.(i) ?? o.labels[i],
        rows: o.series.map((se, si) => [
          se.color ?? (o.series.length === 1 ? SINGLE : SERIES[si % SERIES.length]),
          se.name,
          se.values[i] == null ? "—" : (o.format ?? ((v: number) => nf.format(v)))(se.values[i]!),
        ]),
      }),
      (i) => {
        cross.setAttribute("visibility", i == null ? "hidden" : "visible");
        if (i != null) {
          cross.setAttribute("x1", String(xAt(i)));
          cross.setAttribute("x2", String(xAt(i)));
        }
        dots.forEach((ds) => ds.forEach((c, j) => c.setAttribute("r", j === i ? "4.5" : n <= 12 ? "3" : "0")));
      },
    );
  });
  return wrap;
}

/** Barras verticais (uma série; ou várias lado a lado). */
export function barChart(o: ChartOpts): HTMLElement {
  const fmt = o.format ?? short;
  const box = h("div", { class: "adm-chart" });
  const wrap = h("figure", { class: "adm-figure" }, legend(o.series), box, srTable(o));
  const ht = o.height ?? 200;
  const all = o.series.flatMap((se) => se.values.filter((v): v is number => v != null));
  const max = Math.max(0, ...all);
  responsive(box, (w) => {
    box.replaceChildren();
    const svg = s("svg", { width: w, height: ht, viewBox: `0 0 ${w} ${ht}`, "aria-hidden": "true" });
    const f = frame(svg, w, ht, max, 0, fmt, 8);
    const n = Math.max(o.labels.length, 1);
    const band = (w - f.left - f.right) / n;
    const xAt = (i: number) => f.left + band * (i + 0.5);
    xLabels(svg, f, o.labels, xAt);
    const k = o.series.length;
    const gap = 2; // espaço da cor de fundo entre barras vizinhas
    const groupW = Math.min(band * 0.78, 18 * k + gap * (k - 1));
    const barW = Math.max(1, (groupW - gap * (k - 1)) / k);
    const bars: SVGPathElement[][] = [];
    o.series.forEach((se, si) => {
      const color = se.color ?? (k === 1 ? SINGLE : SERIES[si % SERIES.length]);
      const row: SVGPathElement[] = [];
      se.values.forEach((v, i) => {
        if (v == null || v <= 0) return;
        const x = xAt(i) - groupW / 2 + si * (barW + gap);
        const y0 = f.y(0);
        const y1 = f.y(v);
        const r = Math.min(4, barW / 2, (y0 - y1) / 1.5);
        // Ponta arredondada (4 px) só em cima; a base fica reta no zero.
        const d = `M${x},${y0}V${y1 + r}Q${x},${y1} ${x + r},${y1}H${x + barW - r}Q${x + barW},${y1} ${x + barW},${y1 + r}V${y0}Z`;
        const p = s("path", { d, fill: color, class: "adm-bar" });
        p.dataset.i = String(i);
        svg.append(p);
        row.push(p);
      });
      bars.push(row);
    });
    box.append(svg);
    attachTip(
      box,
      svg,
      f,
      o.labels.length,
      xAt,
      (i) => ({
        title: o.tipLabel?.(i) ?? o.labels[i],
        rows: o.series.map((se, si) => [
          se.color ?? (k === 1 ? SINGLE : SERIES[si % SERIES.length]),
          se.name,
          se.values[i] == null ? "—" : (o.format ?? ((v: number) => nf.format(v)))(se.values[i]!),
        ]),
      }),
      (i) => bars.forEach((row) => row.forEach((p) => p.classList.toggle("is-dim", i != null && p.dataset.i !== String(i)))),
    );
  });
  return wrap;
}

/** Lista de barras horizontais em HTML (rótulos longos: regiões, origens, aparelhos). */
export function hbars(rows: { label: string; value: number; note?: string }[], format: (n: number) => string = (n) => nf.format(n)): HTMLElement {
  const max = Math.max(1, ...rows.map((r) => r.value));
  return h(
    "ul",
    { class: "adm-hbars" },
    ...rows.map((r) => {
      const fill = h("span", { class: "adm-hbar-fill" });
      fill.style.width = `${Math.max(0.5, (100 * r.value) / max)}%`;
      return h(
        "li",
        {},
        h("span", { class: "adm-hbar-label" }, r.label, r.note ? h("small", {}, ` ${r.note}`) : null),
        h("span", { class: "adm-hbar-track" }, fill),
        h("b", { class: "adm-hbar-value" }, format(r.value)),
      );
    }),
  );
}

/** Lista de dias de since até hoje (UTC), para os eixos não pularem os dias vazios. */
export function daysFrom(sinceIso: string): string[] {
  const out: string[] = [];
  const d = new Date(sinceIso);
  const end = new Date();
  for (let t = Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()); t <= end.getTime(); t += 86_400_000) {
    out.push(new Date(t).toISOString().slice(0, 10));
  }
  return out;
}
