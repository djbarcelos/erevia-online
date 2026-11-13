import { ItemKind, type Inventory, type ItemInfo } from "../gen/erevia/v1/game_pb";
import { classInfo } from "../classes";
import { formatSeconds, render, STROKE, type Theme } from "./icons";

/** Lugares de um item (iguais aos do servidor). */
export const PLACE_BAG = 0;
export const PLACE_EQUIP = 1;

/** Encaixes de equipamento (os números são os do servidor). */
export const SLOT = {
  weapon: 0, offhand: 1, chest: 2, head: 3, legs: 4, hands: 5, feet: 6, cape: 7,
  neck: 8, ear1: 9, ear2: 10, ring1: 11, ring2: 12,
} as const;
export const EQUIP_SLOT_COUNT = 13;
export const EQUIP_SLOTS = ["Arma", "Mão secundária", "Peito", "Cabeça", "Pernas", "Mãos", "Pés", "Capa", "Colar", "Brinco", "Brinco", "Anel", "Anel"];

/** O item cabe no encaixe? (anéis e brincos cabem em dois). */
export function fitsSlot(info: ItemInfo, slot: number): boolean {
  return info.slots.length ? info.slots.includes(slot) : info.slot === slot;
}

export function equippable(info: ItemInfo): boolean {
  return info.slot >= 0;
}

export const GRADE_COLOR: Record<number, string> = { 1: "#e8e1d4", 2: "#7bd88f", 3: "#6fa8ff", 4: "#c08cff" };
export const GRADE_NAME: Record<number, string> = { 1: "Comum", 2: "Bom", 3: "Raro", 4: "Épico" };

export function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls?: string, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

export interface Stack {
  info: ItemInfo;
  count: number;
}

/**
 * Catálogo de itens (vem no Welcome) + estado do inventário do jogador local, com
 * ouvintes para a interface se redesenhar quando o servidor manda um Inventory novo.
 */
export class ItemStore {
  private catalog = new Map<string, ItemInfo>();
  bag: (Stack | null)[] = [];
  equip: (Stack | null)[] = Array.from({ length: EQUIP_SLOT_COUNT }, () => null);
  gold = 0;
  private listeners = new Set<() => void>();

  constructor(list: ItemInfo[]) {
    for (const i of list) this.catalog.set(i.id, i);
  }

  get(id: string): ItemInfo | undefined {
    return this.catalog.get(id);
  }

  setInventory(inv: Inventory | undefined): void {
    if (!inv) return;
    this.gold = Number(inv.gold);
    this.bag = Array.from({ length: inv.bagSize || 40 }, () => null);
    this.equip = Array.from({ length: EQUIP_SLOT_COUNT }, () => null);
    for (const it of inv.items) {
      const info = this.catalog.get(it.itemId);
      if (!info) continue;
      const s = { info, count: it.count };
      if (it.place === PLACE_EQUIP) {
        if (it.slot < EQUIP_SLOT_COUNT) this.equip[it.slot] = s;
      } else this.bag[it.slot] = s;
    }
    for (const l of this.listeners) l();
  }

  onChange(fn: () => void): () => void {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  /** Quantas unidades do item há na bolsa. */
  countOf(id: string): number {
    let n = 0;
    for (const s of this.bag) if (s?.info.id === id) n += s.count;
    return n;
  }

  /** Primeiro espaço da bolsa com o item (-1 = nenhum). */
  slotOf(id: string): number {
    return this.bag.findIndex((s) => s?.info.id === id);
  }
}

// ---------------------------------------------------------------------------------------
// Ícones
// ---------------------------------------------------------------------------------------

const metal = "#d9dee8";
const wood = "#8a5a32";

const GLYPHS: Record<string, string> = {
  scroll: `<path d="M18 14 H44 C48 14 50 17 50 20 V46 C50 50 47 52 44 52 H22 C18 52 16 49 16 46 V20" fill="#e8d9b0"/>
    <path d="M16 20 C16 16 18 14 22 14 C26 14 27 17 27 20 V24 H16 Z" fill="#c9b07a"/>
    <path d="M24 28 H42 M24 34 H42 M24 40 H36" stroke="#8a6a3a" stroke-width="2.2" ${STROKE}/>
    <circle cx="42" cy="44" r="4.5" fill="#c0392b"/>`,
  sword: `<path d="M44 12 L52 12 L52 20 L28 44 L20 36 Z" fill="${metal}"/><path d="M47 15 L25 37" stroke="#9aa6b8" stroke-width="1.5"/>
    <path d="M16 34 L30 48" stroke="#c9a24a" stroke-width="5" ${STROKE}/><path d="M22 42 L12 52" stroke="${wood}" stroke-width="5" ${STROKE}/>`,
  axe: `<path d="M18 52 L42 18" stroke="${wood}" stroke-width="5" ${STROKE}/>
    <path d="M34 12 C44 8 56 14 54 26 C49 23 44 24 40 28 C39 22 37 17 34 12 Z" fill="${metal}"/>`,
  staff: `<path d="M20 54 L40 22" stroke="${wood}" stroke-width="4.5" ${STROKE}/>
    <circle cx="43" cy="17" r="7" fill="#8fd0ff"/><circle cx="41" cy="15" r="2.5" fill="#fff"/>
    <path d="M36 12 C38 6 48 6 50 12" stroke="#c9a24a" stroke-width="2.5" fill="none" ${STROKE}/>`,
  daggers: `<path d="M14 14 L32 32" stroke="${metal}" stroke-width="5" ${STROKE}/><path d="M30 36 L38 28 M34 32 L42 40" stroke="#c9a24a" stroke-width="3.5" ${STROKE}/>
    <path d="M50 14 L32 32" stroke="${metal}" stroke-width="5" ${STROKE} opacity=".85"/><path d="M26 40 L20 46 M38 40 L44 46" stroke="${wood}" stroke-width="4" ${STROKE}/>`,
  shield: `<path d="M32 10 L50 16 V30 C50 41 42 49 32 54 C22 49 14 41 14 30 V16 Z" fill="#7d93b8"/>
    <path d="M32 16 L44 20 V30 C44 38 39 44 32 47 Z" fill="#b8c8e6"/><path d="M32 22 V40 M24 30 H40" stroke="#c9a24a" stroke-width="3" ${STROKE}/>`,
  shield_round: `<circle cx="32" cy="32" r="21" fill="#9a6a3a"/><circle cx="32" cy="32" r="21" fill="none" stroke="${metal}" stroke-width="3.5"/>
    <circle cx="32" cy="32" r="6" fill="${metal}"/><path d="M32 13 V26 M32 38 V51 M13 32 H26 M38 32 H51" stroke="#6a4424" stroke-width="2"/>`,
  armor: `<path d="M22 12 L32 16 L42 12 L52 20 L46 28 L44 52 H20 L18 28 L12 20 Z" fill="#8a7a64"/>
    <path d="M32 16 V50 M22 30 H42 M22 40 H42" stroke="#5a4a38" stroke-width="2"/><path d="M22 12 L32 16 L42 12" stroke="${metal}" stroke-width="2.5" fill="none" ${STROKE}/>`,
  helmet: `<path d="M14 40 C14 22 22 12 32 12 C42 12 50 22 50 40 Z" fill="#9aa4b4"/>
    <path d="M14 40 H50 V46 H14 Z" fill="#6c7484"/><path d="M28 24 H36 V40 H28 Z" fill="#2a2e38"/><path d="M32 12 V8" stroke="#c9a24a" stroke-width="3" ${STROKE}/>`,
  potion_hp: `<path d="M27 12 H37 V22 C45 25 49 32 47 40 C45 49 38 53 32 53 C26 53 19 49 17 40 C15 32 19 25 27 22 Z" fill="#f0e8dc" opacity=".35"/>
    <path d="M19 38 C21 47 27 51 32 51 C37 51 43 47 45 38 Z" fill="#e0453a"/><path d="M26 10 H38" stroke="${wood}" stroke-width="4" ${STROKE}/>
    <circle cx="25" cy="32" r="3" fill="#fff" opacity=".6"/>`,
  potion_mp: `<path d="M27 12 H37 V22 C45 25 49 32 47 40 C45 49 38 53 32 53 C26 53 19 49 17 40 C15 32 19 25 27 22 Z" fill="#f0e8dc" opacity=".35"/>
    <path d="M19 38 C21 47 27 51 32 51 C37 51 43 47 45 38 Z" fill="#3a7ae0"/><path d="M26 10 H38" stroke="${wood}" stroke-width="4" ${STROKE}/>
    <circle cx="25" cy="32" r="3" fill="#fff" opacity=".6"/>`,
  bone: `<path d="M20 44 L44 20" stroke="#efe6d2" stroke-width="7" ${STROKE}/>
    <circle cx="16" cy="44" r="5" fill="#efe6d2"/><circle cx="20" cy="48" r="5" fill="#efe6d2"/><circle cx="44" cy="16" r="5" fill="#efe6d2"/><circle cx="48" cy="20" r="5" fill="#efe6d2"/>`,
  skull: `<path d="M32 12 C20 12 14 21 15 31 C15 37 19 40 21 42 V50 H43 V42 C45 40 49 37 49 31 C50 21 44 12 32 12 Z" fill="#efe6d2"/>
    <circle cx="25" cy="31" r="5" fill="#2a2420"/><circle cx="39" cy="31" r="5" fill="#2a2420"/><path d="M32 36 L29 42 H35 Z" fill="#2a2420"/>
    <path d="M38 14 L34 24 L40 28" stroke="#8a7a64" stroke-width="2" fill="none" ${STROKE}/>`,
  essence: `<path d="M24 22 H40 L44 50 C38 54 26 54 20 50 Z" fill="#f0e8dc" opacity=".3"/>
    <path d="M23 36 C28 30 26 26 32 22 C34 30 42 32 40 40 C38 48 26 48 23 36 Z" fill="#7a4ad0"/><path d="M24 18 H40" stroke="${wood}" stroke-width="4" ${STROKE}/>`,
  crown: `<path d="M12 44 L16 20 L26 32 L32 14 L38 32 L48 20 L52 44 Z" fill="#d6b46e"/><path d="M12 44 H52 V50 H12 Z" fill="#a8843a"/>
    <path d="M30 14 L36 50" stroke="#2a2420" stroke-width="2.5"/><circle cx="22" cy="44" r="2.5" fill="#e0453a"/><circle cx="42" cy="44" r="2.5" fill="#3a7ae0"/>`,
  gold: `<ellipse cx="26" cy="42" rx="12" ry="5" fill="#a8843a"/><ellipse cx="26" cy="39" rx="12" ry="5" fill="#e8c260"/>
    <ellipse cx="38" cy="34" rx="12" ry="5" fill="#a8843a"/><ellipse cx="38" cy="31" rx="12" ry="5" fill="#e8c260"/>
    <ellipse cx="30" cy="24" rx="12" ry="5" fill="#a8843a"/><ellipse cx="30" cy="21" rx="12" ry="5" fill="#f3d488"/>`,
};
Object.assign(GLYPHS, {
  legs: `<path d="M20 12 H44 L46 30 L42 54 H34 L32 32 L30 54 H22 L18 30 Z" fill="#7a6a58"/>
    <path d="M20 12 H44 V18 H20 Z" fill="#c9a24a"/><path d="M24 38 H29 M35 38 H40" stroke="#4a3e30" stroke-width="2.5" ${STROKE}/>`,
  gloves: `<path d="M20 54 V34 L16 24 C15 20 20 19 22 22 L25 29 V14 C25 10 30 10 30 14 V28 V12 C30 8 35 8 35 12 V28 V15 C35 11 40 11 40 15 V33 L44 28 C46 25 51 27 49 31 L42 44 V54 Z" fill="#9a7048"/>
    <path d="M20 46 H42" stroke="#c9a24a" stroke-width="3.5"/>`,
  boots: `<path d="M20 10 H34 V38 L50 44 C54 45 54 53 50 53 H17 L19 38 Z" fill="#8a5a32"/>
    <path d="M19 16 H35" stroke="#c9a24a" stroke-width="3.5"/><path d="M17 49 H52" stroke="#4a2e18" stroke-width="3"/>`,
  cape: `<path d="M24 10 H40 C42 22 48 36 52 52 C44 49 39 52 32 49 C25 52 20 49 12 52 C16 36 22 22 24 10 Z" fill="#6a3a8a"/>
    <path d="M24 12 H40" stroke="#c9a24a" stroke-width="4" ${STROKE}/><path d="M28 22 C26 32 22 40 20 47 M36 22 C38 32 42 40 44 47" stroke="#3a1e4e" stroke-width="2" fill="none"/>`,
  necklace: `<path d="M16 12 C16 30 24 38 32 38 C40 38 48 30 48 12" fill="none" stroke="#e2c070" stroke-width="3" stroke-dasharray="3 2"/>
    <path d="M32 36 L41 45 L32 56 L23 45 Z" fill="#5fb8e8"/><path d="M30 42 L32 40" stroke="#fff" stroke-width="2" ${STROKE}/>`,
  earring: `<circle cx="32" cy="14" r="6" fill="none" stroke="#e2c070" stroke-width="3"/><path d="M32 20 V28" stroke="#e2c070" stroke-width="3"/>
    <path d="M32 28 C40 36 42 44 32 54 C22 44 24 36 32 28 Z" fill="#e89a3a"/><circle cx="29" cy="40" r="2" fill="#fff" opacity=".7"/>`,
  ring: `<ellipse cx="32" cy="40" rx="15" ry="12" fill="none" stroke="#e2c070" stroke-width="5"/>
    <path d="M25 24 L32 14 L39 24 L32 31 Z" fill="#d04050"/><path d="M29 22 L32 18" stroke="#fff" stroke-width="1.8" ${STROKE}/>`,
});
GLYPHS.potion_hp_big = GLYPHS.potion_hp.replace('r="3"', 'r="4"') + `<path d="M44 14 L48 10 M48 18 L54 16" stroke="#ffd36b" stroke-width="2.5" ${STROKE}/>`;
GLYPHS.potion_mp_big = GLYPHS.potion_mp.replace('r="3"', 'r="4"') + `<path d="M44 14 L48 10 M48 18 L54 16" stroke="#ffd36b" stroke-width="2.5" ${STROKE}/>`;

/** Fundo por raridade (comum neutro → épico roxo). */
const GRADE_BG: Record<number, [string, string]> = {
  1: ["#4a4250", "#18141e"],
  2: ["#3f6a48", "#121e14"],
  3: ["#3f5a8a", "#10182a"],
  4: ["#6a428a", "#1c1028"],
};

const iconCache = new Map<string, string>();

/** SVG do ícone do item (cacheado por ícone + raridade). */
export function itemIcon(info: ItemInfo): string {
  const key = `${info.icon}:${info.grade}`;
  let svg = iconCache.get(key);
  if (!svg) {
    const theme: Theme = { bg: GRADE_BG[info.grade] ?? GRADE_BG[1], ink: "#fff", glyph: GLYPHS[info.icon] ?? GLYPHS.bone };
    svg = render(theme, `it-${info.icon}-${info.grade}`);
    iconCache.set(key, svg);
  }
  return svg;
}

export function goldIcon(): string {
  return itemIcon({ icon: "gold", grade: 1 } as ItemInfo);
}

/** Nome com quantidade: "Osso Velho" / "3× Osso Velho" / "25 moedas". */
export function itemLabel(info: ItemInfo, count: number): string {
  if (info.kind === ItemKind.GOLD) return `${count} moedas`;
  return count > 1 ? `${count}× ${info.name}` : info.name;
}

export function formatGold(n: number): string {
  return n.toLocaleString("pt-BR");
}

// ---------------------------------------------------------------------------------------
// Dica do item
// ---------------------------------------------------------------------------------------

const KIND_LABEL: Partial<Record<ItemKind, string>> = {
  [ItemKind.WEAPON]: "Arma",
  [ItemKind.SHIELD]: "Escudo",
  [ItemKind.ARMOR]: "Armadura",
  [ItemKind.HELMET]: "Elmo",
  [ItemKind.POTION]: "Poção",
  [ItemKind.MATERIAL]: "Material",
  [ItemKind.GOLD]: "Moedas",
  [ItemKind.LEGS]: "Pernas",
  [ItemKind.GLOVES]: "Luvas",
  [ItemKind.BOOTS]: "Botas",
  [ItemKind.CAPE]: "Capa",
  [ItemKind.NECKLACE]: "Colar",
  [ItemKind.EARRING]: "Brinco",
  [ItemKind.RING]: "Anel",
};

const WEAPON_LABEL: Record<string, string> = { sword: "Espada", axe: "Machado", staff: "Cajado", daggers: "Adagas" };

export interface TipContext {
  myClass: string;
  myLevel: number;
  /** Mostrar preço de compra ou de venda. */
  price?: "buy" | "sell";
  count?: number;
  /** Linha extra de ajuda ("Duplo clique para equipar"). */
  hint?: string;
  /** Mostrar ao lado (à direita) do elemento, em vez de acima. */
  side?: boolean;
}

/** Conteúdo da dica de um item (usado pela dica flutuante e pelas janelas novas). */
export function buildItemTip(info: ItemInfo, ctx: TipContext): HTMLElement {
    const t = el("div", "item-tip-body");
    const head = el("div", "tip-head");
    const icon = el("div", "tip-icon");
    icon.innerHTML = itemIcon(info);
    const title = el("div", "tip-title");
    const name = el("strong", "", info.name);
    name.style.color = GRADE_COLOR[info.grade] ?? GRADE_COLOR[1];
    const kind = [(info.kind === ItemKind.WEAPON && WEAPON_LABEL[info.weaponType]) || KIND_LABEL[info.kind] || "Item"];
    if (info.slot >= 0) kind.push(GRADE_NAME[info.grade] ?? "");
    if (info.slots.length > 1) kind.push("2 encaixes");
    title.append(name, el("span", "tip-kind", kind.filter(Boolean).join(" · ")));
    head.append(icon, title);
    t.append(head);

    const stats = el("ul", "tip-stats");
    const stat = (k: string, v: string, cls = "") => {
      const li = el("li", cls);
      li.append(el("span", "", k), el("b", "", v));
      stats.append(li);
    };
    const b = info.bonus;
    if (b) {
      if (b.pAtk) stat("Ataque físico", `+${b.pAtk}`);
      if (b.mAtk) stat("Ataque mágico", `+${b.mAtk}`);
      if (b.pDef) stat("Defesa física", `+${b.pDef}`);
      if (b.mDef) stat("Defesa mágica", `+${b.mDef}`);
      if (b.maxHp) stat("Vida máxima", `+${b.maxHp}`);
      if (b.maxMp) stat("Mana máxima", `+${b.maxMp}`, "mana");
      if (b.crit) stat("Crítico", `+${Math.round(b.crit * 100)}%`);
      const sign = (v: number, txt: string) => `${v > 0 ? "+" : "−"}${txt}`;
      const pct = (v: number) => `${Math.round(Math.abs(v) * 100)}%`;
      const num = (v: number) => (Math.abs(v) % 1 ? Math.abs(v).toFixed(1).replace(".", ",") : String(Math.abs(v)));
      if (b.critDamage) stat("Dano crítico", sign(b.critDamage, pct(b.critDamage)));
      if (b.accuracy) stat("Precisão", sign(b.accuracy, num(b.accuracy)), b.accuracy < 0 ? "bad" : "");
      if (b.evasion) stat("Esquiva", sign(b.evasion, num(b.evasion)), b.evasion < 0 ? "bad" : "");
      if (b.atkSpeed) stat("Vel. de ataque", sign(b.atkSpeed, pct(b.atkSpeed)));
      if (b.castSpeed) stat("Vel. de conjuração", sign(b.castSpeed, pct(b.castSpeed)));
      if (b.speed) stat("Vel. de movimento", sign(b.speed, `${num(b.speed)} m/s`));
      if (b.hpRegen) stat("Regeneração de vida", sign(b.hpRegen, `${num(b.hpRegen)}/s`));
      if (b.mpRegen) stat("Regeneração de mana", sign(b.mpRegen, `${num(b.mpRegen)}/s`), "mana");
    }
    if (info.healHp) stat("Cura", `${info.healHp} de vida`);
    if (info.healMp) stat("Restaura", `${info.healMp} de mana`, "mana");
    if (info.cooldownMs) stat("Recarga", `${formatSeconds(info.cooldownMs)} s`);
    if (ctx.price === "buy") stat("Preço", `${formatGold(info.price)} moedas`, "gold");
    if (ctx.price === "sell" || (!ctx.price && info.sellPrice > 0)) {
      const n = ctx.count && ctx.count > 1 ? ctx.count : 1;
      stat("Venda", n > 1 ? `${formatGold(info.sellPrice)} cada · ${formatGold(info.sellPrice * n)}` : `${formatGold(info.sellPrice)} moedas`, "gold");
    }
    if (stats.childElementCount) t.append(stats);

    if (info.description) t.append(el("p", "tip-desc", info.description));
    if (info.slot >= 0) {
      const req: string[] = [];
      if (info.classes.length) req.push(info.classes.map((c) => classInfo(c).label).join(", "));
      if (info.reqLevel > 1) req.push(`nível ${info.reqLevel}`);
      if (req.length) {
        const ok = (!info.classes.length || info.classes.includes(ctx.myClass)) && ctx.myLevel >= info.reqLevel;
        t.append(el("p", `tip-req${ok ? "" : " bad"}`, `Requer: ${req.join(" · ")}`));
      }
    }
    if (ctx.hint) t.append(el("p", "tip-key", ctx.hint));
    return t;
}

/** Uma dica flutuante compartilhada por inventário, loja e barra. */
export class ItemTip {
  readonly root = el("div", "skill-tip item-tip");

  constructor(parent: HTMLElement) {
    this.root.hidden = true;
    parent.append(this.root);
  }

  hide(): void {
    this.root.hidden = true;
  }

  show(info: ItemInfo, anchor: HTMLElement, ctx: TipContext): void {
    const t = this.root;
    t.replaceChildren(...buildItemTip(info, ctx).childNodes);
    t.hidden = false;
    const r = anchor.getBoundingClientRect();
    const w = t.offsetWidth;
    const h = t.offsetHeight;
    // Acima do item; se não couber, ao lado.
    let left = r.left + r.width / 2 - w / 2;
    let top = r.top - h - 8;
    if (ctx.side) {
      left = r.right + 10 + w > window.innerWidth ? r.left - w - 10 : r.right + 10;
      top = Math.min(window.innerHeight - h - 8, r.top);
    } else if (top < 8) {
      top = Math.min(window.innerHeight - h - 8, r.top);
      left = r.left - w - 10 < 8 ? r.right + 10 : r.left - w - 10;
    }
    t.style.left = `${Math.max(8, Math.min(window.innerWidth - w - 8, left))}px`;
    t.style.top = `${Math.max(8, top)}px`;
    t.style.bottom = "auto";
  }
}
