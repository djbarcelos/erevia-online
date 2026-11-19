import { ItemKind, ScrollKind, type EnchantRules, type Inventory, type ItemBonus, type ItemInfo, type ItemSetInfo } from "../gen/erevia/v1/game_pb";
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

export const GRADE_COLOR: Record<number, string> = { 1: "#e8e1d4", 2: "#7bd88f", 3: "#6fa8ff", 4: "#c08cff", 5: "#ff9a3c" };
export const GRADE_NAME: Record<number, string> = { 1: "Comum", 2: "Bom", 3: "Raro", 4: "Épico", 5: "Lendário" };

// ---------- Graus de equipamento (tunnel#153) ----------

/** Letra do grau ("" = sem grau). O grau sai do nível exigido da peça. */
export const TIER_LETTER: Record<number, string> = { 1: "D", 2: "C", 3: "B" };
export const WEIGHT_NAME: Record<number, string> = { 1: "Pesada", 2: "Leve", 3: "Manto" };

export function tierLetter(info: ItemInfo): string {
  return TIER_LETTER[info.tier] ?? "";
}

/** Selo do grau ao lado do nome ("D", "C", "B"); null = sem grau. */
export function tierBadge(info: ItemInfo): HTMLElement | null {
  const t = tierLetter(info);
  if (!t) return null;
  const b = el("span", `tier-badge tier-${t.toLowerCase()}`, t);
  b.title = `Grau ${t}`;
  return b;
}

/** Conjuntos (o servidor manda o conjunto inteiro na primeira peça; nas outras, só o id). */
const itemSets = new Map<string, ItemSetInfo>();
/** O inventário do jogador local (a dica conta as peças do conjunto que ele veste). */
let activeStore: ItemStore | undefined;

/** As linhas de um bônus ("+10 de precisão"), na ordem da dica. */
export function bonusParts(b: ItemBonus | undefined): string[] {
  if (!b) return [];
  const out: string[] = [];
  const sign = (v: number) => (v > 0 ? "+" : "−");
  const pct = (v: number) => `${sign(v)}${Math.round(Math.abs(v) * 100)}%`;
  const num = (v: number) => `${sign(v)}${Math.abs(v) % 1 ? Math.abs(v).toFixed(1).replace(".", ",") : Math.abs(v)}`;
  if (b.pAtk) out.push(`${num(b.pAtk)} de ataque físico`);
  if (b.mAtk) out.push(`${num(b.mAtk)} de ataque mágico`);
  if (b.pDef) out.push(`${num(b.pDef)} de defesa física`);
  if (b.mDef) out.push(`${num(b.mDef)} de defesa mágica`);
  if (b.maxHp) out.push(`${num(b.maxHp)} de vida`);
  if (b.maxMp) out.push(`${num(b.maxMp)} de mana`);
  if (b.crit) out.push(`${pct(b.crit)} de crítico`);
  if (b.critDamage) out.push(`${pct(b.critDamage)} de dano crítico`);
  if (b.accuracy) out.push(`${num(b.accuracy)} de precisão`);
  if (b.evasion) out.push(`${num(b.evasion)} de esquiva`);
  if (b.atkSpeed && Math.abs(b.atkSpeed - b.castSpeed) < 1e-6) out.push(`${pct(b.atkSpeed)} de velocidade de ataque e de conjuração`);
  else {
    if (b.atkSpeed) out.push(`${pct(b.atkSpeed)} de velocidade de ataque`);
    if (b.castSpeed) out.push(`${pct(b.castSpeed)} de velocidade de conjuração`);
  }
  if (b.speed) out.push(`${num(b.speed)} m/s de movimento`);
  if (b.hpRegen) out.push(`${num(b.hpRegen)}/s de vida`);
  if (b.mpRegen) out.push(`${num(b.mpRegen)}/s de mana`);
  return out;
}

export function el<K extends keyof HTMLElementTagNameMap>(tag: K, cls?: string, text?: string): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

export interface Stack {
  info: ItemInfo;
  count: number;
  /** +N de encantamento (0 = sem). */
  enchant: number;
}

// ---------- Encantamento (regras vindas do servidor no Welcome) ----------

let enchantRules: EnchantRules | undefined;

export function setEnchantRules(r: EnchantRules | undefined): void {
  enchantRules = r;
}

/** Chance (0–1) de chegar a +target, no grau da peça (0 sem grau, 1 D, 2 C, 3 B). */
export function enchantChance(target: number, tier = 0): number {
  return enchantRules?.tiers[tier]?.chance[target] ?? enchantRules?.chance[target] ?? 0;
}

/** Moedas para tentar levar uma peça do grau a +target (tunnel#154). */
export function enchantFee(target: number, tier = 0): number {
  return (enchantRules?.tiers[tier]?.feePerLevel ?? 0) * Math.max(0, target);
}

/** O pergaminho é do grau da peça (o do D também serve nas sem grau). */
export function scrollFits(scroll: ItemInfo, target: ItemInfo): boolean {
  return scroll.scrollTier === target.tier || (scroll.scrollTier === 1 && target.tier === 0);
}

/** Cristalizar: quantos cristais a peça +enchant rende (0 = não cristaliza). */
export function crystalYield(info: ItemInfo, enchant: number): number {
  const base = info.crystalCount;
  if (!base || !info.crystalId) return 0;
  const per = enchantRules?.crystalPerEnchant || 5;
  return base + Math.floor((base * Math.max(0, enchant)) / per);
}

export const enchantSafe = (): number => enchantRules?.safe ?? 3;
export const enchantMax = (): number => enchantRules?.max ?? 10;

/** O que o encantamento soma (mesma conta do servidor: acima do seguro, cada nível vale o dobro). */
export function enchantBonus(info: ItemInfo, level: number): { pAtk: number; mAtk: number; pDef: number; mDef: number } {
  const out = { pAtk: 0, mAtk: 0, pDef: 0, mDef: 0 };
  if (!enchantRules || level <= 0) return out;
  const steps = level + Math.max(0, level - enchantRules.safe);
  if (info.graded) {
    // Peças dos graus: uma fração da própria peça por degrau (graded_step).
    const f = enchantRules.gradedStep * steps;
    const b = info.bonus;
    if (info.kind === ItemKind.WEAPON) {
      out.pAtk = Math.round((b?.pAtk ?? 0) * f);
      out.mAtk = Math.round((b?.mAtk ?? 0) * f);
    } else if (info.kind === ItemKind.NECKLACE || info.kind === ItemKind.EARRING || info.kind === ItemKind.RING) {
      out.mDef = Math.round((b?.mDef ?? 0) * f);
    } else {
      out.pDef = Math.round((b?.pDef ?? 0) * f);
      out.mDef = Math.round((b?.mDef ?? 0) * f);
    }
    return out;
  }
  const per = (enchantRules.weaponBonus[info.grade] ?? 0) * steps;
  if (info.kind === ItemKind.WEAPON) {
    const b = info.bonus;
    if ((b?.mAtk ?? 0) > 0) out.mAtk = per;
    if ((b?.pAtk ?? 0) > 0 || (b?.mAtk ?? 0) <= 0) out.pAtk = per;
  } else if (info.kind === ItemKind.NECKLACE || info.kind === ItemKind.EARRING || info.kind === ItemKind.RING) {
    out.mDef = Math.round(per / 2);
  } else {
    out.pDef = Math.round(per / 2);
    out.mDef = Math.round(per / 2);
  }
  return out;
}

/** O pergaminho encanta este item (o tipo; o grau é scrollFits)? */
export function canEnchantWith(scroll: ItemInfo, target: ItemInfo): boolean {
  if (target.stack > 1 || !equippable(target)) return false;
  if (scroll.scroll === ScrollKind.ENCHANT_WEAPON) return target.kind === ItemKind.WEAPON;
  if (scroll.scroll === ScrollKind.ENCHANT_ARMOR) return target.kind !== ItemKind.WEAPON;
  return false;
}

/** "+5 Espada de Aço" (sem encantamento, só o nome). */
export function enchantedName(info: ItemInfo, level: number): string {
  return level > 0 ? `+${level} ${info.name}` : info.name;
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
    for (const i of list) {
      this.catalog.set(i.id, i);
      if (i.set?.pieces.length) itemSets.set(i.set.id, i.set);
    }
    activeStore = this;
  }

  /** Quantas peças de cada conjunto o jogador veste. */
  setPiecesWorn(): Map<string, number> {
    const out = new Map<string, number>();
    for (const s of this.equip) {
      const id = s?.info.set?.id;
      if (id) out.set(id, (out.get(id) ?? 0) + 1);
    }
    return out;
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
      const s = { info, count: it.count, enchant: it.enchant };
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

/** Cristal da cristalização (tunnel#154): um prisma com a cor do grau. */
function crystalGlyph(body: string, light: string): string {
  return `<path d="M32 6 L48 24 L40 56 L24 56 L16 24 Z" fill="${body}"/>
    <path d="M32 6 L40 24 L32 56 L24 24 Z" fill="${light}" opacity=".7"/>
    <path d="M16 24 H48" stroke="${light}" stroke-width="2" ${STROKE}/>`;
}

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
  crystal_d: crystalGlyph("#b8c4d0", "#e8f0f8"),
  crystal_c: crystalGlyph("#5aa0e0", "#c8e4ff"),
  crystal_b: crystalGlyph("#b070e0", "#ecd4ff"),
  rune_core: `<path d="M32 8 L50 20 L52 40 L36 56 L18 50 L12 28 Z" fill="#7d776e"/>
    <path d="M32 8 L50 20 L40 30 L24 26 Z" fill="#9a948a"/><path d="M12 28 L24 26 L26 44 L18 50 Z" fill="#5e5952"/>
    <circle cx="33" cy="36" r="9" fill="#ff8c2a"/><circle cx="33" cy="36" r="4.5" fill="#ffe0a0"/>
    <path d="M33 27 V20 M42 36 H49 M33 45 V52 M24 36 H17" stroke="#ffb050" stroke-width="2.2" ${STROKE}/>`,
  crown: `<path d="M12 44 L16 20 L26 32 L32 14 L38 32 L48 20 L52 44 Z" fill="#d6b46e"/><path d="M12 44 H52 V50 H12 Z" fill="#a8843a"/>
    <path d="M30 14 L36 50" stroke="#2a2420" stroke-width="2.5"/><circle cx="22" cy="44" r="2.5" fill="#e0453a"/><circle cx="42" cy="44" r="2.5" fill="#3a7ae0"/>`,
  // Insígnia do Capitão (raro): distintivo de ferro com um crânio e a fita dourada.
  insignia: `<path d="M24 6 H40 L36 20 H28 Z" fill="#b8333a"/><path d="M30 6 H34 L33 20 H31 Z" fill="#d6b46e"/>
    <path d="M32 18 L48 24 V36 C48 46 40 52 32 56 C24 52 16 46 16 36 V24 Z" fill="#7d776e"/>
    <path d="M32 22 L44 27 V36 C44 43 38 48 32 51 C26 48 20 43 20 36 V27 Z" fill="#9a948a"/>
    <path d="M32 28 C27 28 25 31 25 35 C25 38 27 39 28 40 V43 H36 V40 C37 39 39 38 39 35 C39 31 37 28 32 28 Z" fill="#efe6d2"/>
    <circle cx="29.5" cy="35" r="1.8" fill="#2a2420"/><circle cx="34.5" cy="35" r="1.8" fill="#2a2420"/>`,
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
// Brejo de Lamagris (tunnel#139): o Antídoto e os materiais do pântano.
GLYPHS.potion_antidote = GLYPHS.potion_hp.replace('#e0453a', '#6fcf4a') + `<path d="M31 41 L33 37 L35 41 M33 37 V45" stroke="#e8f8d0" stroke-width="2" ${STROKE}/>`;
Object.assign(GLYPHS, {
  brejo_scale: `<path d="M32 10 C46 10 54 20 52 34 C50 46 40 54 32 54 C24 54 14 46 12 34 C10 20 18 10 32 10 Z" fill="#4f7a4a"/>
    <path d="M32 16 C41 16 47 23 46 33 C45 41 38 47 32 47 C26 47 19 41 18 33 C17 23 23 16 32 16 Z" fill="#7fb06a"/>
    <path d="M24 28 Q32 22 40 28 M22 36 Q32 30 42 36 M26 43 Q32 39 38 43" stroke="#2f4a2c" stroke-width="2" fill="none" ${STROKE}/>`,
  brejo_mud: `<path d="M12 46 C10 38 16 32 22 34 C22 24 30 18 38 22 C46 20 54 28 50 36 C56 40 54 50 44 50 H20 C16 50 12 50 12 46 Z" fill="#4a4730"/>
    <path d="M20 40 C22 34 28 32 32 34" stroke="#7a7650" stroke-width="2.5" fill="none" ${STROKE}/>
    <circle cx="40" cy="32" r="2.5" fill="#9fe870" opacity=".8"/><circle cx="28" cy="44" r="2" fill="#9fe870" opacity=".6"/>`,
  brejo_gland: `<path d="M32 12 C46 14 52 28 48 40 C44 52 34 54 28 52 C18 50 12 40 16 28 C18 18 24 12 32 12 Z" fill="#c9c04a"/>
    <path d="M30 18 C38 18 44 26 42 36 C40 44 34 48 28 46 C22 44 20 36 22 28 C24 22 26 18 30 18 Z" fill="#e8e27a"/>
    <circle cx="24" cy="26" r="3" fill="#6b7d3e"/><circle cx="38" cy="40" r="3.5" fill="#6b7d3e"/><circle cx="36" cy="26" r="2" fill="#6b7d3e"/>
    <path d="M30 12 C30 8 34 8 34 12" stroke="#6b7d3e" stroke-width="3" fill="none" ${STROKE}/>`,
  brejo_reed: `<path d="M20 54 L24 14 M30 54 L32 10 M40 54 L38 16 M48 54 L44 22" stroke="#b8c4cc" stroke-width="3" ${STROKE}/>
    <path d="M24 12 L24 22 M32 8 L32 20 M38 14 L38 24" stroke="#7a5a3a" stroke-width="5" ${STROKE}/>
    <path d="M14 54 C22 48 42 48 50 54" stroke="#6b7d3e" stroke-width="3" fill="none" ${STROKE}/>`,
});
// Bosque Gris e Terras Mortas (tunnel#137): os materiais do Bando do Corvo, dos lobos, das aranhas e dos Devotos.
Object.assign(GLYPHS, {
  wolf_pelt: `<path d="M14 18 C22 10 42 10 50 18 L54 34 C50 36 48 40 50 48 L40 54 C38 50 26 50 24 54 L14 48 C16 40 14 36 10 34 Z" fill="#8b8f96"/>
    <path d="M22 22 L28 34 L24 46 M42 22 L36 34 L40 46" stroke="#5a5e66" stroke-width="2.5" fill="none" ${STROKE}/>`,
  wolf_fang: `<path d="M26 10 C40 12 44 30 36 54 C30 40 24 28 26 10 Z" fill="#efe6cf"/>
    <path d="M30 16 C36 24 36 36 34 44" stroke="#bfae86" stroke-width="2.5" fill="none" ${STROKE}/>`,
  moss_silk: `<path d="M32 8 V56 M12 20 L52 44 M52 20 L12 44" stroke="#c4d2c0" stroke-width="2" ${STROKE}/>
    <path d="M32 18 L44 24 V40 L32 46 L20 40 V24 Z" fill="none" stroke="#9fb79a" stroke-width="2.5"/>
    <path d="M32 27 L38 30 V36 L32 39 L26 36 V30 Z" fill="none" stroke="#7d8a6a" stroke-width="2"/>`,
  crow_insignia: `<path d="M12 40 C20 20 34 14 50 18 L44 26 L52 30 C40 32 38 44 22 50 Z" fill="#a2302a"/>
    <circle cx="42" cy="22" r="2" fill="#f5e6c8"/><path d="M50 18 L58 20 L50 24 Z" fill="#d6b46e"/>
    <path d="M18 44 L26 40" stroke="#d6b46e" stroke-width="2.5" ${STROKE}/>`,
  grave_ash: `<path d="M10 50 C12 36 24 30 32 18 C40 30 52 36 54 50 Z" fill="#8f8a80"/>
    <path d="M22 46 C26 38 38 38 42 46" stroke="#5b5750" stroke-width="2.5" fill="none" ${STROKE}/>
    <circle cx="32" cy="14" r="2.2" fill="#b9b3a6"/><circle cx="24" cy="24" r="1.6" fill="#b9b3a6"/>`,
  black_candle: `<rect x="26" y="26" width="12" height="28" rx="2" fill="#2b2a33"/>
    <path d="M32 8 C38 16 38 22 32 26 C26 22 26 16 32 8 Z" fill="#b8f0a0"/>
    <path d="M22 54 H42" stroke="#4a4858" stroke-width="3" ${STROKE}/>`,
  profane_tome: `<path d="M14 10 H46 V54 H14 Z" fill="#4b2f6a"/><path d="M14 10 H19 V54 H14 Z" fill="#2e1c44"/>
    <path d="M26 22 H38 M26 30 H38 M26 38 H34" stroke="#8a5cff" stroke-width="2.5" ${STROKE}/><circle cx="32" cy="47" r="3" fill="#b8f0a0"/>`,
  seal_fragment: `<path d="M12 36 L22 14 L44 10 L54 30 L46 52 L22 54 Z" fill="#6f6a62"/>
    <path d="M26 22 L36 20 M30 30 L42 34 M24 40 L34 44" stroke="#8a5cff" stroke-width="2.5" ${STROKE}/>`,
  bastion_medal: `<circle cx="32" cy="38" r="15" fill="#c9a24a"/><circle cx="32" cy="38" r="10" fill="none" stroke="#7a5d1c" stroke-width="2.5"/>
    <path d="M22 8 L32 24 L42 8 Z" fill="#8a1f1f"/><path d="M28 36 L32 30 L36 36 L32 44 Z" fill="#f5e6c8"/>`,
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

const WEAPON_LABEL: Record<string, string> = {
  sword: "Espada",
  axe: "Machado",
  axe2h: "Machado de Duas Mãos",
  staff: "Cajado",
  wand: "Varinha",
  daggers: "Adagas",
  bow: "Arco",
  crossbow: "Besta",
  crossbow2h: "Besta Pesada",
};

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
  /** +N de encantamento do item. */
  enchant?: number;
}

/** Conteúdo da dica de um item (usado pela dica flutuante e pelas janelas novas). */
export function buildItemTip(info: ItemInfo, ctx: TipContext): HTMLElement {
    const t = el("div", "item-tip-body");
    const head = el("div", "tip-head");
    const icon = el("div", "tip-icon");
    icon.innerHTML = itemIcon(info);
    const title = el("div", "tip-title");
    const ench = ctx.enchant ?? 0;
    const name = el("strong", "", enchantedName(info, ench));
    name.style.color = GRADE_COLOR[info.grade] ?? GRADE_COLOR[1];
    const kind = [(info.kind === ItemKind.WEAPON && WEAPON_LABEL[info.weaponType]) || KIND_LABEL[info.kind] || "Item"];
    if (info.armorWeight) kind.push(WEIGHT_NAME[info.armorWeight] ?? "");
    if (info.slot >= 0) kind.push(GRADE_NAME[info.grade] ?? "");
    if (tierLetter(info)) kind.push(`Grau ${tierLetter(info)}`);
    if (info.slots.length > 1) kind.push("2 encaixes");
    const nameLine = el("span", "tip-name");
    nameLine.append(name);
    const badge = tierBadge(info);
    if (badge) nameLine.append(badge);
    title.append(nameLine, el("span", "tip-kind", kind.filter(Boolean).join(" · ")));
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
    if (ench > 0) {
      const e = enchantBonus(info, ench);
      const parts = [
        e.pAtk && `Ataque físico +${e.pAtk}`,
        e.mAtk && `Ataque mágico +${e.mAtk}`,
        e.pDef && `Defesa física +${e.pDef}`,
        e.mDef && `Defesa mágica +${e.mDef}`,
      ].filter(Boolean);
      stat(`Encantamento +${ench}`, parts.join(" · "), "ench");
    }
    if (info.scroll === ScrollKind.ENCHANT_WEAPON || info.scroll === ScrollKind.ENCHANT_ARMOR) {
      const t = info.scrollTier;
      stat("Grau", t === 1 ? "D (e peças sem grau)" : TIER_LETTER[t] || "—");
      stat("Seguro até", `+${enchantSafe()}`);
      stat(`Chance de +${enchantSafe() + 1}`, `${Math.round(enchantChance(enchantSafe() + 1, t) * 100)}%`);
      stat("Taxa", `${formatGold(enchantFee(1, t))} moedas por nível de destino`, "gold");
    }
    if (info.crystalCount > 0) {
      const n = crystalYield(info, ench);
      stat("Cristaliza em", `${n}× Cristal ${tierLetter(info)}`);
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

    const set = info.set ? itemSets.get(info.set.id) : undefined;
    if (set) t.append(setBlock(info, set));
    if (info.description) t.append(el("p", "tip-desc", info.description));
    if (info.slot >= 0) {
      const req: string[] = [];
      if (info.classes.length) req.push(info.classes.map((c) => classInfo(c).label).join(", "));
      // O grau tem um nível mínimo (abaixo dele não se veste); do mínimo até o nível da
      // peça, ela vale com a penalidade de nível.
      if (info.tierLevel > 1 && info.tierLevel < info.reqLevel) req.push(`nível ${info.reqLevel} (veste do ${info.tierLevel}, com penalidade)`);
      else if (info.reqLevel > 1) req.push(`nível ${info.reqLevel}`);
      if (req.length) {
        const ok = (!info.classes.length || info.classes.includes(ctx.myClass)) && ctx.myLevel >= info.reqLevel;
        t.append(el("p", `tip-req${ok ? "" : " bad"}`, `Requer: ${req.join(" · ")}`));
      }
      const pen = bonusParts(info.underLevel);
      if (pen.length && ctx.myLevel < info.reqLevel) {
        const wearable = ctx.myLevel >= (info.tierLevel || 1);
        t.append(
          el(
            "p",
            "tip-penalty",
            wearable
              ? `Abaixo do nível ${info.reqLevel}, vestir esta peça dá ${pen.join(", ")} (uma vez, com uma ou mais peças).`
              : `Só se veste a partir do nível ${info.tierLevel}. Antes do nível ${info.reqLevel}: ${pen.join(", ")}.`,
          ),
        );
      }
    }
    if (ctx.hint) t.append(el("p", "tip-key", ctx.hint));
    return t;
}

/** O conjunto na dica: nome com as peças vestidas, a escolha do conjunto, as peças (as
 *  vestidas acesas) e os bônus de 2 e de 4 peças (os que valem, acesos). */
function setBlock(info: ItemInfo, set: ItemSetInfo): HTMLElement {
  const worn = activeStore?.setPiecesWorn().get(set.id) ?? 0;
  const wornKinds = new Set<ItemKind>();
  for (const s of activeStore?.equip ?? []) if (s?.info.set?.id === set.id) wornKinds.add(s.info.kind);
  const box = el("div", "tip-set");
  box.append(el("div", "tip-set-name", `${set.name} (${worn}/${set.pieces.length})`));
  if (set.theme) box.append(el("div", "tip-set-theme", set.theme));
  const pieces = el("ul", "tip-set-pieces");
  for (const id of set.pieces) {
    const p = activeStore?.get(id);
    if (!p) continue;
    pieces.append(el("li", wornKinds.has(p.kind) ? "on" : "", p.kind === info.kind ? `${p.name} ◂` : p.name));
  }
  box.append(pieces);
  for (const b of set.bonuses) {
    box.append(el("div", `tip-set-bonus${worn >= b.pieces ? " on" : ""}`, `${b.pieces} peças: ${bonusParts(b.bonus).join(", ")}`));
  }
  box.append(el("div", "tip-set-note", "A versão Exaltada de cada peça conta como ela."));
  return box;
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
