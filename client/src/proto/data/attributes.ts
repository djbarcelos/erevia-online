/**
 * Atributos com pontos livres — PROPOSTA para implementar no servidor depois.
 *
 * Ideia: o servidor continua calculando os atributos de combate da classe por nível
 * (`game.ClassStats` em server/internal/game/classes.go). Os 6 atributos base viram
 * multiplicadores em cima desses valores, mais os bônus dos equipamentos.
 *
 * - Cada classe começa com uma distribuição própria (soma 60).
 * - Cada nível acima do 1 dá 3 pontos para distribuir (nível 20 = 57 pontos).
 * - "Neutro" é 10: abaixo disso o atributo reduz, acima aumenta.
 */

import type { ClassId } from "../../api";

export type AttrId = "str" | "dex" | "con" | "int" | "wis" | "spi";

export interface AttrDef {
  id: AttrId;
  name: string;
  short: string;
  color: string;
  desc: string;
  affects: string;
}

export const ATTRS: AttrDef[] = [
  { id: "str", name: "Força", short: "FOR", color: "#e2674a", desc: "Músculo e peso do golpe.", affects: "Atq. físico (+2% por ponto) e capacidade de carga." },
  { id: "dex", name: "Destreza", short: "DES", color: "#e9c05a", desc: "Reflexo e precisão.", affects: "Crítico (+0,4%), precisão, esquiva e velocidade de ataque (+1%)." },
  { id: "con", name: "Constituição", short: "CON", color: "#6fc46a", desc: "Vigor e resistência do corpo.", affects: "HP máximo (+2,5%), def. física (+1%) e regeneração de HP." },
  { id: "int", name: "Inteligência", short: "INT", color: "#6c8cff", desc: "Domínio da magia.", affects: "Atq. mágico (+2,5% por ponto)." },
  { id: "wis", name: "Sabedoria", short: "SAB", color: "#58c7d6", desc: "Foco e calma ao conjurar.", affects: "Velocidade de conjuração (+1,5%), regeneração de MP e resistência a atordoar." },
  { id: "spi", name: "Espírito", short: "ESP", color: "#b67de8", desc: "Força de vontade.", affects: "MP máximo (+3%) e def. mágica (+1,5%)." },
];

export type Attrs = Record<AttrId, number>;

export const POINTS_PER_LEVEL = 3;

export const CLASS_BASE_ATTRS: Record<ClassId, Attrs> = {
  knight: { str: 12, dex: 8, con: 15, int: 6, wis: 9, spi: 10 },
  barbarian: { str: 16, dex: 10, con: 13, int: 5, wis: 7, spi: 9 },
  mage: { str: 5, dex: 9, con: 8, int: 16, wis: 12, spi: 10 },
  rogue: { str: 11, dex: 16, con: 10, int: 6, wis: 8, spi: 9 },
};

/** Sugestão de distribuição por classe (pesos). */
export const CLASS_SUGGEST: Record<ClassId, Partial<Attrs>> = {
  knight: { con: 2, str: 1 },
  barbarian: { str: 2, con: 1 },
  mage: { int: 2, spi: 1 },
  rogue: { dex: 2, str: 1 },
};

/** Valores de classe por nível: cópia de server/internal/game/classes.go. */
interface ClassStats {
  maxHp: number;
  maxMp: number;
  pAtk: number;
  mAtk: number;
  pDef: number;
  mDef: number;
  atkInterval: number;
  crit: number;
  speed: number;
}

const BASE: Record<ClassId, { base: ClassStats; per: Partial<ClassStats> }> = {
  knight: {
    base: { maxHp: 160, maxMp: 40, pAtk: 14, mAtk: 4, pDef: 30, mDef: 15, atkInterval: 1.2, crit: 0.05, speed: 5 },
    per: { maxHp: 22, maxMp: 4, pAtk: 2.2, mAtk: 0.5, pDef: 3.5, mDef: 2 },
  },
  barbarian: {
    base: { maxHp: 150, maxMp: 35, pAtk: 18, mAtk: 3, pDef: 20, mDef: 12, atkInterval: 1.4, crit: 0.1, speed: 5 },
    per: { maxHp: 20, maxMp: 3, pAtk: 3, mAtk: 0.4, pDef: 2.5, mDef: 1.5 },
  },
  mage: {
    base: { maxHp: 95, maxMp: 120, pAtk: 6, mAtk: 20, pDef: 12, mDef: 25, atkInterval: 1.6, crit: 0.05, speed: 5 },
    per: { maxHp: 11, maxMp: 14, pAtk: 0.6, mAtk: 3.2, pDef: 1.5, mDef: 3 },
  },
  rogue: {
    base: { maxHp: 120, maxMp: 60, pAtk: 12, mAtk: 4, pDef: 16, mDef: 12, atkInterval: 0.8, crit: 0.25, speed: 5.5 },
    per: { maxHp: 15, maxMp: 6, pAtk: 2, mAtk: 0.5, pDef: 2, mDef: 1.5 },
  },
};

export function classStats(cls: ClassId, level: number): ClassStats {
  const c = BASE[cls];
  const l = level - 1;
  const s = { ...c.base };
  for (const [k, v] of Object.entries(c.per) as [keyof ClassStats, number][]) s[k] += v * l;
  return s;
}

/** Bônus planos vindos dos equipamentos. */
export type Bonus = Partial<Record<DerivedId, number>>;

export type DerivedId =
  | "maxHp"
  | "maxMp"
  | "pAtk"
  | "mAtk"
  | "pDef"
  | "mDef"
  | "crit"
  | "accuracy"
  | "evasion"
  | "atkSpeed"
  | "castSpeed"
  | "speed"
  | "hpRegen"
  | "mpRegen";

export interface DerivedDef {
  id: DerivedId;
  name: string;
  group: "Recursos" | "Ataque" | "Defesa" | "Utilidade";
  /** Formata o valor para a tela. */
  fmt(v: number): string;
  /** Diferença mínima para mostrar a prévia. */
  eps: number;
}

const int = (v: number) => Math.round(v).toLocaleString("pt-BR");
const pct = (v: number) => `${(v * 100).toLocaleString("pt-BR", { maximumFractionDigits: 1 })}%`;
const dec = (d: number, suf = "") => (v: number) => `${v.toLocaleString("pt-BR", { minimumFractionDigits: d, maximumFractionDigits: d })}${suf}`;

export const DERIVED: DerivedDef[] = [
  { id: "maxHp", name: "HP máximo", group: "Recursos", fmt: int, eps: 0.5 },
  { id: "maxMp", name: "MP máximo", group: "Recursos", fmt: int, eps: 0.5 },
  { id: "hpRegen", name: "Regeneração de HP", group: "Recursos", fmt: dec(1, "/s"), eps: 0.05 },
  { id: "mpRegen", name: "Regeneração de MP", group: "Recursos", fmt: dec(1, "/s"), eps: 0.05 },
  { id: "pAtk", name: "Ataque físico", group: "Ataque", fmt: int, eps: 0.5 },
  { id: "mAtk", name: "Ataque mágico", group: "Ataque", fmt: int, eps: 0.5 },
  { id: "crit", name: "Chance de crítico", group: "Ataque", fmt: pct, eps: 0.0005 },
  { id: "accuracy", name: "Precisão", group: "Ataque", fmt: int, eps: 0.5 },
  { id: "atkSpeed", name: "Velocidade de ataque", group: "Ataque", fmt: dec(2, "/s"), eps: 0.005 },
  { id: "castSpeed", name: "Velocidade de conjuração", group: "Ataque", fmt: pct, eps: 0.0005 },
  { id: "pDef", name: "Defesa física", group: "Defesa", fmt: int, eps: 0.5 },
  { id: "mDef", name: "Defesa mágica", group: "Defesa", fmt: int, eps: 0.5 },
  { id: "evasion", name: "Esquiva", group: "Defesa", fmt: int, eps: 0.5 },
  { id: "speed", name: "Velocidade de movimento", group: "Utilidade", fmt: dec(1, " m/s"), eps: 0.05 },
];

export type Derived = Record<DerivedId, number>;

/** Atributos de combate finais (classe × atributos + equipamento). */
export function derive(cls: ClassId, level: number, a: Attrs, gear: Bonus = {}): Derived {
  const c = classStats(cls, level);
  const k = (v: number) => v - 10;
  const g = (id: DerivedId) => gear[id] ?? 0;
  const maxHp = c.maxHp * (1 + k(a.con) * 0.025) + g("maxHp");
  const maxMp = c.maxMp * (1 + k(a.spi) * 0.03) + g("maxMp");
  return {
    maxHp,
    maxMp,
    pAtk: c.pAtk * (1 + k(a.str) * 0.02) + g("pAtk"),
    mAtk: c.mAtk * (1 + k(a.int) * 0.025) + g("mAtk"),
    pDef: c.pDef * (1 + k(a.con) * 0.01) + g("pDef"),
    mDef: c.mDef * (1 + k(a.spi) * 0.015) + g("mDef"),
    crit: Math.max(0, c.crit + k(a.dex) * 0.004 + g("crit")),
    accuracy: 70 + a.dex * 1.5 + level + g("accuracy"),
    evasion: 20 + a.dex * 1.2 + level * 0.5 + g("evasion"),
    atkSpeed: (1 / c.atkInterval) * (1 + k(a.dex) * 0.01) + g("atkSpeed"),
    castSpeed: 1 + k(a.wis) * 0.015 + g("castSpeed"),
    speed: c.speed + g("speed"),
    hpRegen: maxHp * 0.004 + a.con * 0.08 + g("hpRegen"),
    mpRegen: maxMp * 0.008 + a.wis * 0.12 + g("mpRegen"),
  };
}

export function attrPointsTotal(level: number): number {
  return (level - 1) * POINTS_PER_LEVEL;
}
