/**
 * Estado falso do personagem no protótipo (nada vai ao servidor). As telas leem e
 * alteram este estado; quem precisa reagir (HUD, janelas) assina `onChange`.
 */

import type { ClassId } from "../../api";
import { CLASS_BASE_ATTRS, derive, attrPointsTotal, type Attrs, type Bonus, type DerivedId } from "./attributes";
import { initialInventory, ITEMS, itemStats, stack, type EquipSlot, type ItemStack } from "./items";
import { initialRanks, skillPointsTotal, TREES, type SkillNode } from "./skilltree";

export interface Appearance {
  /** Arma da mão direita e item da esquerda (arquivos em /models/weapons; null = vazio). */
  right: string | null;
  left: string | null;
  /** Cor do emblema (aura no chão e nome). */
  emblem: string;
}

export const DEFAULT_LOOK: Record<ClassId, Appearance> = {
  knight: { right: "sword_1handed", left: "shield_badge_color", emblem: "#d6b46e" },
  barbarian: { right: "axe_1handed", left: "shield_round_barbarian", emblem: "#e2674a" },
  mage: { right: "staff", left: null, emblem: "#8c6fd6" },
  rogue: { right: "dagger", left: "dagger", emblem: "#5fae7d" },
};

export interface Hero {
  name: string;
  cls: ClassId;
  level: number;
  /** XP no nível atual (0..xpNeeded). */
  xp: number;
  look: Appearance;
  /** Pontos gastos em cada atributo (somados à base da classe). */
  spent: Attrs;
  ranks: Record<string, number>;
  bag: (ItemStack | null)[];
  equip: Partial<Record<EquipSlot, ItemStack>>;
  gold: number;
  /** Barra de atalhos: ids de skills (nós da árvore) ou de itens ("item:pt_hp"). */
  hotbar: (string | null)[];
}

export const HOTBAR_SIZE = 10;

const zeroAttrs = (): Attrs => ({ str: 0, dex: 0, con: 0, int: 0, wis: 0, spi: 0 });

/** Distribuição já feita no personagem de exemplo (deixa 5 pontos livres). */
function exampleSpent(cls: ClassId, level: number): Attrs {
  const s = zeroAttrs();
  const total = attrPointsTotal(level) - 5;
  const order: Record<ClassId, (keyof Attrs)[]> = {
    knight: ["con", "str", "con", "spi"],
    barbarian: ["str", "con", "str", "dex"],
    mage: ["int", "spi", "int", "wis"],
    rogue: ["dex", "str", "dex", "con"],
  };
  for (let i = 0; i < total; i++) s[order[cls][i % 4]]++;
  return s;
}

export function xpNeeded(level: number): number {
  return Math.round(80 * Math.pow(level, 2.2));
}

function defaultHotbar(cls: ClassId, ranks: Record<string, number>): (string | null)[] {
  const actives = TREES[cls].nodes.filter((n) => n.kind === "active" && (ranks[n.id] ?? 0) > 0).map((n) => n.id);
  const bar: (string | null)[] = Array(HOTBAR_SIZE).fill(null);
  actives.slice(0, 6).forEach((id, i) => (bar[i] = id));
  bar[8] = "item:pt_hp";
  bar[9] = "item:pt_mp";
  return bar;
}

/** Modelo 3D de arma/escudo usado por cada desenho de item (em /models/weapons). */
const HAND_MODEL: Record<string, string> = {
  sword: "sword_1handed",
  axe: "axe_1handed",
  dagger: "dagger",
  staff: "staff",
  shield: "shield_badge_color",
  buckler: "shield_round_barbarian",
};

/** Ajusta a arma inicial equipada à classe (a antiga vai para a mochila). */
function classGear(cls: ClassId, inv: ReturnType<typeof initialInventory>): void {
  const weapon: Record<ClassId, string> = { knight: "sw_vigil", barbarian: "ax_war", mage: "st_ash", rogue: "dg_curve" };
  const want = weapon[cls];
  if (inv.equip.weapon?.id === want) return;
  const i = inv.bag.findIndex((s) => s?.id === want);
  const old = inv.equip.weapon;
  inv.equip.weapon = i >= 0 ? inv.bag[i]! : stack(want, 1);
  if (i >= 0) inv.bag[i] = old ?? null;
  if (cls === "mage" && inv.equip.offhand) {
    const free = inv.bag.findIndex((s) => !s);
    if (free >= 0) inv.bag[free] = inv.equip.offhand;
    delete inv.equip.offhand;
  }
}

export function makeHero(name: string, cls: ClassId, look?: Appearance): Hero {
  const level = 14;
  const ranks = initialRanks(cls);
  const inv = initialInventory();
  classGear(cls, inv);
  return {
    name,
    cls,
    level,
    xp: Math.round(xpNeeded(level) * 0.62),
    look: look ?? { ...DEFAULT_LOOK[cls] },
    spent: exampleSpent(cls, level),
    ranks,
    bag: inv.bag,
    equip: inv.equip,
    gold: inv.gold,
    hotbar: defaultHotbar(cls, ranks),
  };
}

const HERO_KEY = "erevia.proto.hero";

interface SavedHero {
  name: string;
  cls: ClassId;
  look: Appearance;
}

function loadHero(): Hero {
  try {
    const raw = localStorage.getItem(HERO_KEY);
    if (raw) {
      const s = JSON.parse(raw) as SavedHero;
      if (s.cls in TREES) return makeHero(s.name, s.cls, s.look);
    }
  } catch {
    // ignora
  }
  return makeHero("Aldric", "knight");
}

class GameState {
  hero = loadHero();
  private subs = new Set<(what: string) => void>();

  onChange(fn: (what: string) => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  emit(what: string): void {
    for (const fn of this.subs) fn(what);
  }

  /** Troca o personagem (vindo da criação). */
  setHero(name: string, cls: ClassId, look: Appearance): void {
    this.hero = makeHero(name, cls, look);
    try {
      localStorage.setItem(HERO_KEY, JSON.stringify({ name, cls, look } satisfies SavedHero));
    } catch {
      // ignora
    }
    this.emit("hero");
  }

  /** Aparência no jogo: as mãos mostram o que está equipado; a cor vem da criação. */
  look(): Appearance {
    const eq = this.hero.equip;
    const model = (s?: ItemStack) => (s ? (HAND_MODEL[ITEMS[s.id]?.art] ?? null) : null);
    return { right: model(eq.weapon), left: model(eq.offhand), emblem: this.hero.look.emblem };
  }

  attrs(): Attrs {
    const b = CLASS_BASE_ATTRS[this.hero.cls];
    const s = this.hero.spent;
    return { str: b.str + s.str, dex: b.dex + s.dex, con: b.con + s.con, int: b.int + s.int, wis: b.wis + s.wis, spi: b.spi + s.spi };
  }

  attrPointsFree(): number {
    const used = Object.values(this.hero.spent).reduce((a, b) => a + b, 0);
    return attrPointsTotal(this.hero.level) - used;
  }

  gearBonus(): Bonus {
    const out: Bonus = {};
    for (const s of Object.values(this.hero.equip)) {
      if (!s) continue;
      for (const [k, v] of Object.entries(itemStats(s)) as [DerivedId, number][]) out[k] = (out[k] ?? 0) + v;
    }
    return out;
  }

  stats() {
    return derive(this.hero.cls, this.hero.level, this.attrs(), this.gearBonus());
  }

  skillPointsFree(): number {
    const used = Object.values(this.hero.ranks).reduce((a, b) => a + b, 0);
    return skillPointsTotal(this.hero.level) - used;
  }

  node(id: string): SkillNode | undefined {
    return TREES[this.hero.cls].nodes.find((n) => n.id === id);
  }

  countItem(id: string): number {
    return this.hero.bag.reduce((n, s) => n + (s?.id === id ? s.qty : 0), 0);
  }

  /** Gasta 1 unidade de um item da mochila. */
  consume(id: string): boolean {
    const i = this.hero.bag.findIndex((s) => s?.id === id);
    if (i < 0) return false;
    const s = this.hero.bag[i]!;
    s.qty--;
    if (s.qty <= 0) this.hero.bag[i] = null;
    this.emit("bag");
    return true;
  }

  /** Coloca um item na mochila (empilha se der). */
  addItem(id: string, qty = 1): void {
    const same = this.hero.bag.find((s) => s?.id === id);
    if (same) same.qty += qty;
    else {
      const i = this.hero.bag.findIndex((s) => !s);
      if (i >= 0) this.hero.bag[i] = { uid: Date.now() + Math.random(), id, qty };
    }
    this.emit("bag");
  }
}

export const game = new GameState();

/** Dados falsos do mundo para o HUD e a tela de login. */
export const NEWS = [
  { tag: "Evento", date: "12 nov", title: "Noite dos Ossos", text: "Durante a noite, esqueletos dão o dobro de XP até domingo." },
  { tag: "Atualização", date: "08 nov", title: "Ciclo de dia e noite", text: "Tochas acendem na cidade e a noite fica mais perigosa." },
  { tag: "Aviso", date: "02 nov", title: "Manutenção semanal", text: "Toda quarta, das 6h às 7h (horário de Brasília)." },
];

export const SERVERS = [
  { id: "aurora", name: "Aurora", region: "Brasil", pop: "Alta", popLevel: 3, ping: 18, tag: "Recomendado" },
  { id: "brasa", name: "Brasa", region: "Brasil", pop: "Média", popLevel: 2, ping: 22, tag: "PvP" },
  { id: "nevoa", name: "Névoa", region: "EUA Leste", pop: "Baixa", popLevel: 1, ping: 128, tag: "" },
];

export const QUESTS = [
  { title: "Ossos que não descansam", steps: [{ text: "Derrotar Esqueletos", n: 7, of: 12 }], zone: "Campos ao norte" },
  { title: "Carta para o capitão", steps: [{ text: "Entregar a Carta Selada a Hadrian", n: 0, of: 1 }], zone: "Muralha leste" },
];

export const PARTY = [
  { name: "Lyra", cls: "mage" as ClassId, level: 15, hp: 0.82, mp: 0.4 },
  { name: "Brann", cls: "barbarian" as ClassId, level: 13, hp: 0.55, mp: 0.9 },
];

export const CHAT_SEED: { ch: "general" | "party" | "whisper" | "system"; from?: string; text: string }[] = [
  { ch: "system", text: "Bem-vindo a Erevia! Clique num monstro para atacar." },
  { ch: "general", from: "Theron", text: "alguém pra caçar no cemitério?" },
  { ch: "party", from: "Lyra", text: "tô sem mana, segura um pouco" },
  { ch: "general", from: "Mirela", text: "vendo Gema Sombria, chama no privado" },
  { ch: "whisper", from: "Kael", text: "bora fazer a missão do capitão depois?" },
  { ch: "system", text: "Você recebeu 1.240 de XP." },
];

// ---------------------------------------------------------------------------------------
// Mapa: zonas de caça (anéis ao redor da cidade) e pontos de interesse
// ---------------------------------------------------------------------------------------

export interface Zone {
  id: string;
  name: string;
  rMin: number;
  rMax: number;
  levelMin: number;
  levelMax: number;
  safe?: boolean;
  /** "r,g,b" */
  rgb: string;
  desc: string;
}

export const ZONES: Zone[] = [
  { id: "city", name: "Vel'Aran", rMin: 0, rMax: 40, levelMin: 0, levelMax: 0, safe: true, rgb: "120,190,255", desc: "Cidade murada. Zona segura: aqui ninguém ataca." },
  { id: "outskirts", name: "Arredores", rMin: 40, rMax: 58, levelMin: 1, levelMax: 8, rgb: "120,200,110", desc: "Bichos fracos perto da muralha. Bom para começar." },
  { id: "bones", name: "Campos dos Ossos", rMin: 58, rMax: 105, levelMin: 10, levelMax: 16, rgb: "230,180,80", desc: "Esqueletos que levantam à noite. Chefe: Capitão Ossudo." },
  { id: "wastes", name: "Ermos Partidos", rMin: 105, rMax: 150, levelMin: 17, levelMax: 25, rgb: "230,90,70", desc: "Terra rachada e perigosa. Vá em grupo." },
];

export function zoneAt(x: number, z: number): Zone {
  const r = Math.hypot(x, z);
  return ZONES.find((zn) => r >= zn.rMin && r < zn.rMax) ?? ZONES[ZONES.length - 1];
}

export interface MapPoi {
  kind: "quest" | "quest-area" | "gate" | "portal";
  name: string;
  sub?: string;
  x: number;
  z: number;
  r?: number;
}

export const MAP_POIS: MapPoi[] = [
  { kind: "quest", name: "Capitão Hadrian", sub: "Carta para o capitão", x: 36, z: 6 },
  { kind: "quest-area", name: "Ossos que não descansam", sub: "Derrotar esqueletos", x: 0, z: 78, r: 20 },
  { kind: "portal", name: "Pedra de Teleporte", sub: "Viagem rápida", x: 8, z: -8 },
  { kind: "gate", name: "Portão Norte", x: 0, z: 39 },
  { kind: "gate", name: "Portão Sul", x: 0, z: -39 },
  { kind: "gate", name: "Portão Leste", x: 39, z: 0 },
  { kind: "gate", name: "Portão Oeste", x: -39, z: 0 },
];

/** Posições falsas dos colegas de grupo no mapa. */
export const PARTY_POS = [
  { name: "Lyra", x: 12, z: 70 },
  { name: "Brann", x: -15, z: 74 },
];
