/** Itens falsos do protótipo: catálogo, equipamento e mochila. */

import { itemArt, type ItemArt } from "../art";
import type { Bonus, DerivedId } from "./attributes";

export type Rarity = "common" | "uncommon" | "rare" | "epic" | "legend";

export const RARITY_NAME: Record<Rarity, string> = {
  common: "Comum",
  uncommon: "Incomum",
  rare: "Raro",
  epic: "Épico",
  legend: "Lendário",
};

export type EquipSlot =
  | "head"
  | "chest"
  | "legs"
  | "hands"
  | "feet"
  | "cape"
  | "weapon"
  | "offhand"
  | "neck"
  | "ear1"
  | "ear2"
  | "ring1"
  | "ring2";

export const SLOT_NAME: Record<EquipSlot, string> = {
  head: "Cabeça",
  chest: "Peito",
  legs: "Pernas",
  hands: "Mãos",
  feet: "Pés",
  cape: "Capa",
  weapon: "Arma",
  offhand: "Mão esquerda",
  neck: "Colar",
  ear1: "Brinco",
  ear2: "Brinco",
  ring1: "Anel",
  ring2: "Anel",
};

/** Tipo de encaixe do item (brincos e anéis cabem em dois slots). */
export type Fit = "head" | "chest" | "legs" | "hands" | "feet" | "cape" | "weapon" | "offhand" | "neck" | "ear" | "ring";

export function fits(fit: Fit | undefined, slot: EquipSlot): boolean {
  if (!fit) return false;
  if (fit === "ear") return slot === "ear1" || slot === "ear2";
  if (fit === "ring") return slot === "ring1" || slot === "ring2";
  return fit === slot;
}

export type Category = "equip" | "consumable" | "material" | "quest";

export interface ItemDef {
  id: string;
  name: string;
  rarity: Rarity;
  category: Category;
  art: ItemArt;
  color: string;
  fit?: Fit;
  /** Texto do tipo ("Espada de uma mão", "Poção"...) */
  type: string;
  level?: number;
  stats?: Bonus;
  desc?: string;
  /** Peso por unidade. */
  weight: number;
  stack?: number;
  price: number;
  /** Ao usar (consumíveis). */
  use?: { hp?: number; mp?: number; text: string };
}

const D = (d: ItemDef) => d;

export const ITEMS: Record<string, ItemDef> = Object.fromEntries(
  [
    D({ id: "sw_vigil", name: "Espada do Vigia", rarity: "rare", category: "equip", art: "sword", color: "#4f9dff", fit: "weapon", type: "Espada de uma mão", level: 12, stats: { pAtk: 38, crit: 0.02 }, weight: 18, price: 4200, desc: "Forjada para a guarda das muralhas de Vel'Aran." }),
    D({ id: "sw_rust", name: "Espada Enferrujada", rarity: "common", category: "equip", art: "sword", color: "#8a7a6a", fit: "weapon", type: "Espada de uma mão", level: 1, stats: { pAtk: 9 }, weight: 16, price: 40 }),
    D({ id: "ax_war", name: "Machado de Guerra", rarity: "rare", category: "equip", art: "axe", color: "#c8734f", fit: "weapon", type: "Machado de uma mão", level: 14, stats: { pAtk: 46, atkSpeed: -0.05 }, weight: 24, price: 5100 }),
    D({ id: "dg_curve", name: "Adaga Curva", rarity: "uncommon", category: "equip", art: "dagger", color: "#5fd17a", fit: "weapon", type: "Adaga", level: 10, stats: { pAtk: 24, crit: 0.05 }, weight: 8, price: 1800 }),
    D({ id: "st_ash", name: "Cajado de Freixo", rarity: "common", category: "equip", art: "staff", color: "#7fb4ff", fit: "weapon", type: "Cajado", level: 6, stats: { mAtk: 22, maxMp: 30 }, weight: 14, price: 600 }),
    D({ id: "sh_oak", name: "Escudo de Carvalho", rarity: "uncommon", category: "equip", art: "buckler", color: "#d6b46e", fit: "offhand", type: "Escudo", level: 8, stats: { pDef: 22, evasion: -2 }, weight: 20, price: 900 }),
    D({ id: "sh_guard", name: "Égide do Guardião", rarity: "epic", category: "equip", art: "shield", color: "#3e5d9e", fit: "offhand", type: "Escudo", level: 15, stats: { pDef: 48, mDef: 18, maxHp: 120 }, weight: 30, price: 14800, desc: "Diz-se que nenhuma flecha atravessa esta égide duas vezes." }),
    D({ id: "hm_iron", name: "Elmo de Ferro", rarity: "common", category: "equip", art: "helmet", color: "#8a8f9c", fit: "head", type: "Elmo pesado", level: 8, stats: { pDef: 12 }, weight: 12, price: 420 }),
    D({ id: "hm_guard", name: "Elmo do Guardião", rarity: "epic", category: "equip", art: "helmet", color: "#b46cff", fit: "head", type: "Elmo pesado", level: 14, stats: { pDef: 26, mDef: 10, maxHp: 80 }, weight: 14, price: 11200 }),
    D({ id: "ch_mail", name: "Cota de Malha", rarity: "uncommon", category: "equip", art: "chest", color: "#6f7a8c", fit: "chest", type: "Armadura pesada", level: 10, stats: { pDef: 34, speed: -0.1 }, weight: 38, price: 1600 }),
    D({ id: "lg_mail", name: "Grevas de Malha", rarity: "uncommon", category: "equip", art: "legs", color: "#6f7a8c", fit: "legs", type: "Armadura pesada", level: 10, stats: { pDef: 22 }, weight: 26, price: 1100 }),
    D({ id: "gl_rune", name: "Manoplas Rúnicas", rarity: "legend", category: "equip", art: "gloves", color: "#c9763a", fit: "hands", type: "Luvas", level: 15, stats: { pAtk: 18, mAtk: 18, atkSpeed: 0.08, castSpeed: 0.06 }, weight: 6, price: 38000, desc: "As runas acendem quando o portador está em perigo." }),
    D({ id: "gl_leather", name: "Luvas de Couro", rarity: "common", category: "equip", art: "gloves", color: "#7a5434", fit: "hands", type: "Luvas", level: 4, stats: { pDef: 5 }, weight: 3, price: 90 }),
    D({ id: "bt_leather", name: "Botas de Viajante", rarity: "common", category: "equip", art: "boots", color: "#7a5434", fit: "feet", type: "Botas", level: 5, stats: { pDef: 6, speed: 0.2 }, weight: 6, price: 160 }),
    D({ id: "cp_dusk", name: "Capa do Crepúsculo", rarity: "rare", category: "equip", art: "cape", color: "#4b3a7a", fit: "cape", type: "Capa", level: 12, stats: { mDef: 14, evasion: 6 }, weight: 4, price: 3600 }),
    D({ id: "nk_wolf", name: "Colar do Lobo", rarity: "rare", category: "equip", art: "necklace", color: "#4f9dff", fit: "neck", type: "Colar", level: 12, stats: { mDef: 20, maxHp: 60 }, weight: 1, price: 4800 }),
    D({ id: "er_amber", name: "Brinco de Âmbar", rarity: "uncommon", category: "equip", art: "earring", color: "#e8a23a", fit: "ear", type: "Brinco", level: 8, stats: { mDef: 10, maxMp: 20 }, weight: 1, price: 1300 }),
    D({ id: "rg_onyx", name: "Anel de Ônix", rarity: "rare", category: "equip", art: "ring", color: "#35303f", fit: "ring", type: "Anel", level: 11, stats: { mDef: 9, crit: 0.01 }, weight: 1, price: 3900 }),
    D({ id: "rg_copper", name: "Anel de Cobre", rarity: "common", category: "equip", art: "ring", color: "#c87a4a", fit: "ring", type: "Anel", level: 2, stats: { mDef: 3 }, weight: 1, price: 50 }),
    D({ id: "pt_hp", name: "Poção de Vida", rarity: "common", category: "consumable", art: "potion", color: "#e2483a", type: "Poção", weight: 1, stack: 99, price: 25, use: { hp: 180, text: "Recupera 180 HP." } }),
    D({ id: "pt_mp", name: "Poção de Mana", rarity: "common", category: "consumable", art: "potion", color: "#4c9cff", type: "Poção", weight: 1, stack: 99, price: 35, use: { mp: 90, text: "Recupera 90 MP." } }),
    D({ id: "pt_big", name: "Elixir Ancestral", rarity: "epic", category: "consumable", art: "potion", color: "#b46cff", type: "Elixir", weight: 1, stack: 20, price: 900, use: { hp: 600, mp: 300, text: "Recupera 600 HP e 300 MP." } }),
    D({ id: "sc_return", name: "Pergaminho de Retorno", rarity: "uncommon", category: "consumable", art: "scroll", color: "#5fd17a", type: "Pergaminho", weight: 1, stack: 20, price: 120, use: { text: "Teleporta para a cidade mais próxima." } }),
    D({ id: "mt_bone", name: "Osso Antigo", rarity: "common", category: "material", art: "bone", color: "#e8dcc0", type: "Material", weight: 1, stack: 999, price: 4, desc: "Deixado pelos mortos-vivos. Os ferreiros pagam bem." }),
    D({ id: "mt_iron", name: "Minério de Ferro", rarity: "common", category: "material", art: "ore", color: "#c9ced8", type: "Material", weight: 2, stack: 999, price: 12 }),
    D({ id: "mt_gem", name: "Gema Sombria", rarity: "epic", category: "material", art: "gem", color: "#8e5bd8", type: "Material raro", weight: 1, stack: 99, price: 2400, desc: "Usada para encantar equipamentos." }),
    D({ id: "mt_mithril", name: "Fragmento de Mithril", rarity: "rare", category: "material", art: "ore", color: "#7fd0ff", type: "Material", weight: 1, stack: 999, price: 380 }),
    D({ id: "qs_letter", name: "Carta Selada", rarity: "uncommon", category: "quest", art: "letter", color: "#b8352e", type: "Item de missão", weight: 0, price: 0, desc: "Entregar ao capitão Hadrian, na muralha leste." }),
  ].map((d) => [d.id, d]),
);

export interface ItemStack {
  uid: number;
  id: string;
  qty: number;
  ench?: number;
}

let nextUid = 1;
export const stack = (id: string, qty = 1, ench?: number): ItemStack => ({ uid: nextUid++, id, qty, ench });

export function itemIcon(id: string): string {
  const d = ITEMS[id];
  return d ? itemArt(d.art, d.color) : "";
}

export function itemName(s: ItemStack): string {
  const d = ITEMS[s.id];
  return `${s.ench ? `+${s.ench} ` : ""}${d?.name ?? s.id}`;
}

/** Bônus do item, contando o encantamento (+4% por nível nos atributos principais). */
export function itemStats(s: ItemStack): Bonus {
  const d = ITEMS[s.id];
  const out: Bonus = {};
  const mult = 1 + (s.ench ?? 0) * 0.04;
  for (const [k, v] of Object.entries(d?.stats ?? {}) as [DerivedId, number][]) {
    out[k] = k === "crit" || k === "atkSpeed" || k === "castSpeed" || k === "speed" ? v : Math.round(v * mult);
  }
  return out;
}

export const STAT_LABEL: Record<DerivedId, string> = {
  maxHp: "HP máximo",
  maxMp: "MP máximo",
  pAtk: "Atq. físico",
  mAtk: "Atq. mágico",
  pDef: "Def. física",
  mDef: "Def. mágica",
  crit: "Crítico",
  accuracy: "Precisão",
  evasion: "Esquiva",
  atkSpeed: "Vel. de ataque",
  castSpeed: "Vel. de conjuração",
  speed: "Vel. de movimento",
  hpRegen: "Regen. de HP",
  mpRegen: "Regen. de MP",
};

export function statText(k: DerivedId, v: number): string {
  const sign = v > 0 ? "+" : v < 0 ? "−" : "";
  const a = Math.abs(v);
  if (k === "crit" || k === "atkSpeed" || k === "castSpeed") return `${sign}${(a * 100).toLocaleString("pt-BR", { maximumFractionDigits: 1 })}%`;
  if (k === "speed") return `${sign}${a.toLocaleString("pt-BR", { maximumFractionDigits: 1 })} m/s`;
  return `${sign}${a.toLocaleString("pt-BR")}`;
}

export const INVENTORY_SIZE = 48;
export const MAX_WEIGHT = 1200;

/** Estado inicial da mochila e do equipamento. */
export function initialInventory(): { bag: (ItemStack | null)[]; equip: Partial<Record<EquipSlot, ItemStack>>; gold: number } {
  const bag: (ItemStack | null)[] = Array(INVENTORY_SIZE).fill(null);
  const list = [
    stack("pt_hp", 24),
    stack("pt_mp", 12),
    stack("pt_big", 2),
    stack("sc_return", 3),
    stack("hm_guard", 1, 2),
    stack("gl_rune", 1),
    stack("ax_war", 1, 1),
    stack("dg_curve", 1),
    stack("st_ash", 1),
    stack("sh_guard", 1),
    stack("rg_copper", 1),
    stack("mt_bone", 37),
    stack("mt_iron", 12),
    stack("mt_mithril", 5),
    stack("mt_gem", 2),
    stack("qs_letter", 1),
    stack("sw_rust", 1),
  ];
  list.forEach((s, i) => (bag[i] = s));
  return {
    bag,
    gold: 48230,
    equip: {
      weapon: stack("sw_vigil", 1, 3),
      offhand: stack("sh_oak", 1),
      head: stack("hm_iron", 1),
      chest: stack("ch_mail", 1, 1),
      legs: stack("lg_mail", 1),
      hands: stack("gl_leather", 1),
      feet: stack("bt_leather", 1),
      cape: stack("cp_dusk", 1),
      neck: stack("nk_wolf", 1),
      ear1: stack("er_amber", 1),
      ring1: stack("rg_onyx", 1),
    },
  };
}
