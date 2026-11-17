/**
 * Bestiário (tecla B e menu): uma entrada por tipo de monstro, aberta por abates.
 *
 * - Antes do primeiro abate a entrada é uma silhueta (o servidor nem manda o nome).
 * - 10 abates: nível e vida; 100: o que ele deixa cair; 500: +1% de dano contra o tipo.
 *   Chefes e raid bosses (e os raros, quando existirem) abrem tudo com 1 abate.
 * - Famílias (esqueletos e golens até o 30; as Ancestrais com o conteúdo do 30 ao 40): completar as entradas
 *   que nascem no mapa dá um título e uma cor de roupa exclusiva no barbeiro. Raros e
 *   invocações são extras e não contam.
 *
 * Os retratos são SVG desenhados aqui (nada de 3D por entrada): a silhueta é o mesmo retrato
 * apagado no CSS. Tudo vem do servidor (Bestiary, pedido ao abrir); nada roda por quadro.
 */

import type { Bestiary, BestiaryEntry, BestiaryFamily } from "../../gen/erevia/v1/game_pb";
import { bar, button, el, fmt, fmt1, isCompact, segmented } from "../kit";
import { cachedPalettes } from "../../world/body-look";
import "../../styles/bestiary.css";

/** O estado do bestiário no cliente (o servidor manda o inteiro ao entrar e ao pedir). */
export class BestiaryBook {
  entries: BestiaryEntry[] = [];
  families: BestiaryFamily[] = [];
  damageBonus = 0;
  loaded = false;
  private listeners = new Set<() => void>();

  apply(b: Bestiary): void {
    if (b.full) {
      this.entries = [...b.entries];
      this.families = [...b.families];
      this.loaded = true;
    } else {
      for (const e of b.entries) {
        const i = this.entries.findIndex((x) => x.id === e.id);
        if (i >= 0) this.entries[i] = e;
        else this.entries.push(e);
      }
      for (const f of b.families) {
        const i = this.families.findIndex((x) => x.id === f.id);
        if (i >= 0) this.families[i] = f;
        else this.families.push(f);
      }
    }
    if (b.damageBonus > 0) this.damageBonus = b.damageBonus;
    for (const fn of this.listeners) fn();
  }

  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  /** Texto do título de família com esse id (undefined = não é um). */
  titleOf(id: string): string | undefined {
    return this.families.find((f) => f.titleId === id)?.title;
  }

  /** As cores de roupa exclusivas já liberadas (BodyLook.outfit). */
  unlockedOutfits(): number[] {
    return this.families.filter((f) => f.complete).map((f) => f.outfit);
  }
}

export interface BestiaryDeps {
  book: BestiaryBook;
  /** Pede o bestiário inteiro (BESTIARY_CMD_GET). */
  refresh(): void;
  titleId(): string;
  onTitle(fn: () => void): () => void;
  setTitle(id: string): void;
}

// ---------------------------------------------------------------------------------------
// Retratos (SVG 64×64): crânio dos esqueletos com o acessório do modelo, pedra do golem.
// ---------------------------------------------------------------------------------------

const BONE = "#ece3cc";
const BONE_SHADE = "#b9ad8e";
const HOLE = "#17121e";

function skull(eye: string): string {
  return `
    <path d="M32 12c-11 0-19 8-19 18 0 6 3 10 6 12v6c0 2 1.5 3 3 3h20c1.5 0 3-1 3-3v-6c3-2 6-6 6-12 0-10-8-18-19-18Z" fill="${BONE}"/>
    <path d="M45 42c3-2 6-6 6-12 0-6-3-11-8-14 3 4 4 8 4 12 0 6-2 11-6 14h4Z" fill="${BONE_SHADE}" opacity=".7"/>
    <ellipse cx="25" cy="31" rx="4.6" ry="5.2" fill="${HOLE}"/><ellipse cx="39" cy="31" rx="4.6" ry="5.2" fill="${HOLE}"/>
    <circle cx="25" cy="31.5" r="1.8" fill="${eye}"/><circle cx="39" cy="31.5" r="1.8" fill="${eye}"/>
    <path d="M32 36l-2.6 4.4h5.2Z" fill="${HOLE}"/>
    <path d="M25 45v5M29 45v5M33 45v5M37 45v5" stroke="${HOLE}" stroke-width="1.6" stroke-linecap="round"/>`;
}

const ACCESSORY: Record<string, string> = {
  skeleton_warrior: `
    <path d="M12 28c0-12 9-20 20-20s20 8 20 20h-6c-2-7-7-11-14-11s-12 4-14 11Z" fill="#8a8794"/>
    <path d="M30 4h4v14h-4Z" fill="#c0463c"/><path d="M12 28h8v4h-8ZM44 28h8v4h-8Z" fill="#6c6977"/>`,
  skeleton_rogue: `
    <path d="M7 44C6 22 17 6 32 6s26 16 25 38l-7-6c0-13-7-21-18-21S14 25 14 38Z" fill="#3b3449"/>
    <path d="M14 38c0-13 7-21 18-21-8 2-13 9-13 20Z" fill="#2a2434"/>`,
  skeleton_mage: `
    <path d="M14 22 L33 1 L50 22 Z" fill="#4c2f7c"/><path d="M8 22h48l-4 5H12Z" fill="#3a2360"/>
    <circle cx="33" cy="12" r="2.4" fill="#c9a2ff"/>`,
  boss: `
    <path d="M16 18l3-11 6 6 7-9 7 9 6-6 3 11Z" fill="#d6b46e"/><path d="M16 18h32v3H16Z" fill="#8a6a32"/>
    <circle cx="32" cy="10" r="1.8" fill="#e2483a"/>`,
};

const EYES: Record<string, string> = {
  skeleton_minion: "#9fd4ff",
  skeleton_warrior: "#ff6b5b",
  skeleton_rogue: "#6fd38a",
  skeleton_mage: "#c9a2ff",
};

function golem(): string {
  return `
    <path d="M14 20l9-9h18l9 9 4 15-5 14H15l-5-14Z" fill="#7f848c"/>
    <path d="M50 20l4 15-5 14h-6l4-14-3-13Z" fill="#5d6168"/>
    <path d="M23 11l4 9-6 6M41 11l-3 8 7 5M26 44l6-5 6 5" stroke="#4c5057" stroke-width="1.6" fill="none"/>
    <rect x="20" y="27" width="9" height="5" rx="1" fill="${HOLE}"/><rect x="35" y="27" width="9" height="5" rx="1" fill="${HOLE}"/>
    <rect x="22" y="28.5" width="5" height="2" fill="#7fe3ff"/><rect x="37" y="28.5" width="5" height="2" fill="#7fe3ff"/>
    <path d="M29 36h6l-3 4Z" fill="#7fe3ff" opacity=".8"/>`;
}

/** Retrato do monstro pelo modelo (o chefe ganha coroa). */
export function portrait(model: string, boss: boolean): string {
  let body: string;
  if (model.startsWith("skeleton_")) {
    const eye = boss ? "#ff4a3a" : (EYES[model] ?? "#9fd4ff");
    if (boss) body = skull(eye) + ACCESSORY.boss;
    else if (model === "skeleton_rogue") body = ACCESSORY.skeleton_rogue + skull(eye); // o capuz atrás do crânio
    else body = skull(eye) + (ACCESSORY[model] ?? "");
  } else if (model.includes("golem")) {
    body = golem() + (boss ? ACCESSORY.boss : "");
  } else {
    body = `<circle cx="32" cy="34" r="20" fill="#6f6880"/><circle cx="25" cy="31" r="3.5" fill="${HOLE}"/><circle cx="39" cy="31" r="3.5" fill="${HOLE}"/>`;
  }
  return `<svg viewBox="0 0 64 64" aria-hidden="true">${body}</svg>`;
}

// ---------------------------------------------------------------------------------------
// Janela
// ---------------------------------------------------------------------------------------

const pct = (v: number) => `${fmt1.format(Math.round(v * 1000) / 10)}%`;
const range = (a: number, b: number) => (a === b ? fmt.format(a) : `${fmt.format(a)}–${fmt.format(b)}`);

const MARK_LABELS = ["Nível e vida", "O que deixa cair", "Bônus de dano"];

function card(e: BestiaryEntry, selected: boolean, onPick: () => void): HTMLButtonElement {
  const seen = e.kills > 0;
  const b = el("button", `bst-card${seen ? "" : " unseen"}${selected ? " sel" : ""}${e.boss ? " boss" : ""}${e.mark >= 3 ? " done" : ""}`);
  b.type = "button";
  b.setAttribute("aria-pressed", String(selected));
  const pips = el("span", "bst-pips");
  for (let i = 0; i < 3; i++) pips.append(el("i", i < e.mark ? "on" : ""));
  const pic = el("span", "bst-portrait");
  pic.innerHTML = portrait(e.model, e.boss);
  b.append(pic, el("b", "bst-name", seen ? e.name : "???"), el("small", "bst-kills", seen ? `${fmt.format(e.kills)} abate${e.kills === 1 ? "" : "s"}` : "não derrotado"), pips);
  b.onclick = onPick;
  return b;
}

function markRow(e: BestiaryEntry, i: number, bonus: number): HTMLElement {
  const need = e.marks[i];
  const open = e.mark > i;
  const label = e.boss ? (i === 0 ? "Entrada completa" : "") : MARK_LABELS[i];
  const row = el("div", `bst-mark${open ? " open" : ""}`);
  const txt = i === 2 && !e.boss ? `${label} (+${pct(bonus)})` : label;
  row.append(el("span", "bst-mark-icon", open ? "✓" : "🔒"), el("span", "bst-mark-need", `${fmt.format(need)}`), el("span", "bst-mark-label", txt));
  if (!open && e.mark === i) {
    const pb = bar("xp", "bst-bar");
    pb.set(Math.min(e.kills, need), need, `${fmt.format(e.kills)}/${fmt.format(need)}`);
    row.append(pb.root);
  }
  return row;
}

function detail(e: BestiaryEntry | undefined, fam: BestiaryFamily | undefined, bonus: number, back: (() => void) | null): HTMLElement {
  const box = el("div", "bst-detail");
  if (back) box.append(button("‹ Voltar", "btn btn-sm bst-back", back));
  if (!e) {
    box.append(el("p", "muted", "Escolha um monstro."));
    return box;
  }
  const seen = e.kills > 0;
  const pic = el("span", `bst-portrait big${seen ? "" : " unseen"}`);
  pic.innerHTML = portrait(e.model, e.boss);
  box.append(
    el(
      "div",
      "bst-detail-head",
      pic,
      el(
        "div",
        "bst-detail-title",
        el("h3", null, seen ? e.name : "Monstro desconhecido"),
        el("small", "muted", [fam?.name ?? "Sem família", e.boss ? "Chefe" : ""].filter(Boolean).join(" · ")),
        el("span", "bst-detail-kills", seen ? `${fmt.format(e.kills)} abate${e.kills === 1 ? "" : "s"}` : "Derrote um para conhecê-lo."),
      ),
    ),
  );
  const marks = el("div", "bst-marks");
  for (let i = 0; i < (e.boss ? 1 : 3); i++) marks.append(markRow(e, i, bonus));
  box.append(marks);
  if (e.boss) box.append(el("p", "small muted", "Chefes e raid bosses abrem a entrada inteira com 1 abate (sem bônus de dano)."));

  if (e.mark >= 1) {
    box.append(
      el(
        "div",
        "bst-info",
        el("h4", null, "Nível e vida"),
        el("div", "bst-stats", el("span", null, "Nível ", el("b", null, range(e.levelMin, e.levelMax))), el("span", null, "Vida ", el("b", null, range(e.hpMin, e.hpMax)))),
        el("small", "muted", "Do menor ao maior nível que você derrotou."),
      ),
    );
  }
  if (e.mark >= 2) {
    const list = el("div", "bst-drops");
    if (e.goldChance > 0) list.append(el("div", "bst-drop", el("span", null, "Moedas"), el("b", null, range(e.goldMin, e.goldMax)), el("small", null, pct(e.goldChance))));
    for (const d of e.drops) {
      list.append(el("div", "bst-drop", el("span", null, d.name), el("b", null, range(d.min, d.max)), el("small", null, pct(d.chance))));
    }
    if (e.gear) list.append(el("div", "bst-drop", el("span", null, "Equipamento"), el("b", null, "às vezes"), el("small", null, "pelo nível")));
    box.append(el("div", "bst-info", el("h4", null, "O que deixa cair"), list));
  }
  if (e.damageBonus > 0) {
    box.append(el("div", "bst-info bst-bonus", el("b", null, `+${pct(e.damageBonus)} de dano contra ${e.name}`), el("small", "muted", "Vale no golpe e nas skills.")));
  }
  if (fam && e.familyNeed > 0) {
    const ok = e.kills >= e.familyNeed;
    box.append(el("p", `small ${ok ? "ok" : "muted"}`, ok ? `Completa para a família ${fam.name}.` : `Para a família ${fam.name}: ${fmt.format(e.familyNeed)} abates.`));
  } else if (fam) {
    box.append(el("p", "small muted", `Extra da família ${fam.name}: não conta para completá-la (raro ou invocação).`));
  }
  return box;
}

function familyBox(f: BestiaryFamily, using: boolean, setTitle: (id: string) => void): HTMLElement {
  const pb = bar("xp", "bst-bar");
  pb.set(f.done, f.total, `${f.done}/${f.total}`);
  const side = el("div", "bst-fam-side");
  if (f.complete) side.append(using ? el("span", "ach-using", "Em uso") : button("Usar título", "btn btn-sm btn-gold", () => setTitle(f.titleId)));
  else side.append(el("span", "muted small", "Bloqueado"));
  const sw = el("i", "bst-swatch");
  const shade = cachedPalettes()?.bestiaryOutfits?.find((o) => o.index === f.outfit);
  if (shade) sw.style.background = `linear-gradient(${shade.top}, ${shade.bottom})`;
  return el(
    "div",
    `bst-fam${f.complete ? " done" : ""}`,
    el(
      "div",
      "bst-fam-body",
      el("b", null, f.name),
      el("small", null, "Título ", el("b", null, `«${f.title}»`), " · cor de roupa ", sw, el("b", null, f.outfitName), " no barbeiro"),
      pb.root,
    ),
    side,
  );
}

export function buildBestiary(d: BestiaryDeps): HTMLElement {
  const root = el("div", "bst-win");
  let filter = "all";
  let selected = "";
  let showDetail = false; // celular: a ficha ocupa a janela

  const render = () => {
    const book = d.book;
    if (!book.loaded) {
      root.replaceChildren(el("p", "muted", "Carregando o bestiário…"));
      return;
    }
    const famOf = (id: string) => book.families.find((f) => f.id === id);
    const others = book.entries.some((e) => !e.family);
    const filters = [{ id: "all", label: "Todos" }, ...book.families.map((f) => ({ id: f.id, label: f.name })), ...(others ? [{ id: "", label: "Outros" }] : [])];
    const list = book.entries.filter((e) => filter === "all" || e.family === filter);
    if (!list.some((e) => e.id === selected)) selected = (list.find((e) => e.kills > 0) ?? list[0])?.id ?? "";
    const seen = book.entries.filter((e) => e.kills > 0).length;

    const head = el(
      "div",
      "bst-head",
      segmented(filters, filter, (v) => {
        filter = v;
        showDetail = false;
        render();
      }),
      el("span", "bst-count", el("b", null, `${seen}/${book.entries.length}`), " conhecidos"),
    );
    const fams = el("div", "bst-fams");
    for (const f of book.families) if (filter === "all" || filter === f.id) fams.append(familyBox(f, d.titleId() === f.titleId, d.setTitle));

    const grid = el("div", "bst-grid");
    for (const e of list)
      grid.append(
        card(e, e.id === selected, () => {
          selected = e.id;
          showDetail = true;
          render();
        }),
      );
    const cur = book.entries.find((e) => e.id === selected);
    const compact = isCompact();
    const main = el(
      "div",
      `bst-main${compact && showDetail ? " show-detail" : ""}`,
      grid,
      detail(cur, cur ? famOf(cur.family) : undefined, book.damageBonus, compact ? () => ((showDetail = false), render()) : null),
    );
    root.replaceChildren(
      head,
      fams,
      main,
      el(
        "p",
        "small muted bst-foot",
        `Marcos: 10 abates (nível e vida), 100 (o que deixa cair) e 500 (+${pct(book.damageBonus)} de dano contra o tipo). Conta para quem leva a XP do abate.`,
      ),
    );
  };

  render();
  d.refresh();
  const offB = d.book.onChange(render);
  const offT = d.onTitle(render);
  const onResize = () => render();
  addEventListener("resize", onResize);
  root.addEventListener("win-close", () => {
    offB();
    offT();
    removeEventListener("resize", onResize);
  });
  return root;
}

