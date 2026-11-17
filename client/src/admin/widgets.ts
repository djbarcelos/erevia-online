/** Peças das telas de ação do portal (Jogadores e Operação): DOM, formatos, diálogo e aviso. */

import { AdminError } from "./api";

export type Child = Node | string | null | undefined | false;

/** Monta um elemento. Texto vira textContent (nada de HTML vindo de dados). */
export function h<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  attrs: Record<string, string | undefined> = {},
  ...children: (Child | Child[])[]
): HTMLElementTagNameMap[K] {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) if (v !== undefined) el.setAttribute(k, v);
  for (const c of children.flat()) if (c) el.append(c);
  return el;
}

export const nf = new Intl.NumberFormat("pt-BR");

export function fmtDateTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
}

export function fmtClock(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
}

/** "3 h 12 min", "4 min 05 s". */
export function fmtDuration(sec: number): string {
  sec = Math.max(0, Math.round(sec));
  const d = Math.floor(sec / 86400);
  const hh = Math.floor((sec % 86400) / 3600);
  const mm = Math.floor((sec % 3600) / 60);
  const ss = sec % 60;
  if (d > 0) return `${d} d ${hh} h`;
  if (hh > 0) return `${hh} h ${mm} min`;
  if (mm > 0) return `${mm} min ${String(ss).padStart(2, "0")} s`;
  return `${ss} s`;
}

/** "em 4 min" / "há 2 h". */
export function fmtRelative(iso: string | null | undefined): string {
  if (!iso) return "—";
  const diff = (new Date(iso).getTime() - Date.now()) / 1000;
  return diff >= 0 ? `em ${fmtDuration(diff)}` : `há ${fmtDuration(-diff)}`;
}

export function errorText(e: unknown): string {
  if (e instanceof AdminError) return e.message;
  return e instanceof Error ? e.message : String(e);
}

/** Cartão com título (e ferramentas à direita). */
export function card(title: string, cls = "", tools?: Child): HTMLElement {
  return h("section", { class: `adm-card adm-box ${cls}`.trim() }, h("div", { class: "adm-box-head" }, h("h2", {}, title), tools));
}

export function badge(text: string, kind: "ok" | "bad" | "warn" | "muted" | "gold" = "muted"): HTMLElement {
  return h("span", { class: `adm-badge is-${kind}` }, text);
}

/** Linha "rótulo: valor" de uma lista de definições. */
export function field(label: string, value: Child | Child[]): HTMLElement[] {
  return [h("dt", {}, label), h("dd", {}, ...(Array.isArray(value) ? value : [value]))];
}

export function button(label: string, onClick: () => void, kind: "" | "danger" | "ghost" | "primary" = ""): HTMLButtonElement {
  const b = h("button", { class: `adm-btn ${kind ? "is-" + kind : ""}`.trim(), type: "button" }, label);
  b.addEventListener("click", onClick);
  return b;
}

/** Tabela que vira lista de fichas no celular (as colunas levam data-label). */
export function table(cols: string[], rows: Child[][], empty = "Nada por aqui."): HTMLElement {
  if (rows.length === 0) return h("p", { class: "adm-empty" }, empty);
  return h(
    "div",
    { class: "adm-table" },
    h(
      "table",
      {},
      h("thead", {}, h("tr", {}, ...cols.map((c) => h("th", { scope: "col" }, c)))),
      h("tbody", {}, ...rows.map((r) => h("tr", {}, ...r.map((v, i) => h("td", { "data-label": cols[i] }, v ?? "—"))))),
    ),
  );
}

// ---------- Aviso (toast) ----------

let toastBox: HTMLElement | null = null;

export function toast(text: string, kind: "ok" | "bad" = "ok"): void {
  if (!toastBox) {
    toastBox = h("div", { class: "adm-toasts", role: "status", "aria-live": "polite" });
    document.body.append(toastBox);
  }
  const t = h("div", { class: `adm-toast is-${kind}` }, text);
  toastBox.append(t);
  window.setTimeout(() => t.classList.add("is-out"), kind === "bad" ? 7000 : 4500);
  window.setTimeout(() => t.remove(), kind === "bad" ? 7400 : 4900);
}

// ---------- Diálogo de confirmação com campos ----------

export interface DialogField {
  name: string;
  label: string;
  type?: "text" | "number" | "textarea" | "select" | "datetime-local";
  value?: string;
  placeholder?: string;
  required?: boolean;
  min?: string;
  max?: string;
  step?: string;
  options?: { value: string; label: string }[];
  /** Lista de sugestões (datalist) para campos de texto. */
  suggestions?: { value: string; label: string }[];
  hint?: string;
}

export interface DialogSpec {
  title: string;
  text?: string;
  fields?: DialogField[];
  confirm: string;
  danger?: boolean;
  /** Roda ao confirmar; erro mostrado no próprio diálogo (que fica aberto). */
  submit: (values: Record<string, string>) => Promise<string | void>;
}

let dialogSeq = 0;

/** Abre o diálogo; resolve com a mensagem de sucesso (ou null se cancelou). */
export function openDialog(spec: DialogSpec): Promise<string | null> {
  return new Promise((resolve) => {
    const dlg = h("dialog", { class: "adm-dialog", "aria-labelledby": `dlg-${++dialogSeq}` });
    const form = h("form", { method: "dialog", novalidate: "" });
    const err = h("p", { class: "adm-error", role: "alert", hidden: "" });
    const inputs: Record<string, HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement> = {};
    const fields = (spec.fields ?? []).map((f) => {
      const id = `dlg-${dialogSeq}-${f.name}`;
      let input: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement;
      if (f.type === "textarea") {
        input = h("textarea", { id, name: f.name, rows: "3", placeholder: f.placeholder, maxlength: "300" });
        input.value = f.value ?? "";
      } else if (f.type === "select") {
        input = h("select", { id, name: f.name }, ...(f.options ?? []).map((o) => h("option", { value: o.value }, o.label)));
        input.value = f.value ?? f.options?.[0]?.value ?? "";
      } else {
        input = h("input", {
          id,
          name: f.name,
          type: f.type ?? "text",
          placeholder: f.placeholder,
          min: f.min,
          max: f.max,
          step: f.step,
          inputmode: f.type === "number" ? "decimal" : undefined,
          autocomplete: "off",
        });
        input.value = f.value ?? "";
        if (f.suggestions) {
          const listID = `${id}-list`;
          input.setAttribute("list", listID);
          form.append(h("datalist", { id: listID }, ...f.suggestions.map((s) => h("option", { value: s.value }, s.label))));
        }
      }
      if (f.required) input.required = true;
      inputs[f.name] = input;
      return h("label", { class: "adm-field", for: id }, h("span", {}, f.label), input, f.hint ? h("small", {}, f.hint) : null);
    });
    const cancel = h("button", { class: "adm-btn is-ghost", type: "button" }, "Cancelar");
    const ok = h("button", { class: `adm-btn ${spec.danger ? "is-danger" : "is-primary"}`, type: "submit" }, spec.confirm);
    form.append(
      h("h2", { id: `dlg-${dialogSeq}` }, spec.title),
      spec.text ? h("p", { class: "adm-dialog-text" }, spec.text) : "",
      ...fields,
      err,
      h("div", { class: "adm-dialog-actions" }, cancel, ok),
    );
    dlg.append(form);
    document.body.append(dlg);
    let done = false;
    const close = (v: string | null) => {
      if (done) return;
      done = true;
      dlg.close();
      dlg.remove();
      resolve(v);
    };
    cancel.addEventListener("click", () => close(null));
    dlg.addEventListener("cancel", (ev) => {
      ev.preventDefault();
      if (!ok.disabled) close(null);
    });
    form.addEventListener("submit", async (ev) => {
      ev.preventDefault();
      for (const f of spec.fields ?? []) {
        if (f.required && !inputs[f.name].value.trim()) {
          err.textContent = `Preencha: ${f.label}.`;
          err.hidden = false;
          inputs[f.name].focus();
          return;
        }
      }
      const values: Record<string, string> = {};
      for (const [k, v] of Object.entries(inputs)) values[k] = v.value.trim();
      ok.disabled = cancel.disabled = true;
      ok.textContent = "Enviando…";
      try {
        const msg = await spec.submit(values);
        close(msg ?? "");
      } catch (e) {
        err.textContent = errorText(e);
        err.hidden = false;
        ok.disabled = cancel.disabled = false;
        ok.textContent = spec.confirm;
      }
    });
    dlg.showModal();
    (Object.values(inputs)[0] ?? ok).focus();
  });
}

/** Resposta das ações (POST) do portal. */
export interface ActionResult {
  ok: boolean;
  message: string;
  where?: string;
}
