/**
 * Personagem (tecla C): modelo 3D com o equipamento, nível e XP, e os atributos de
 * combate que o servidor manda (SelfStats). A distribuição de pontos chega na Fase B.
 */

import type { ClassId } from "../../api";
import { classInfo } from "../../classes";
import type { SelfStats } from "../../gen/erevia/v1/game_pb";
import type { ItemStore } from "../items";
import { bar, el, fmt } from "../kit";
import { createPreview } from "../preview3d";

export interface CharacterDeps {
  store: ItemStore;
  me(): { myClass: string; myLevel: number; name: string; emblem: string };
  stats(): SelfStats | null;
  onStats(fn: () => void): () => void;
}

const ROWS: { group: string; rows: [string, (s: SelfStats) => string][] }[] = [
  {
    group: "Recursos",
    rows: [
      ["Vida máxima", (s) => fmt.format(s.maxHp)],
      ["Mana máxima", (s) => fmt.format(s.maxMp)],
    ],
  },
  {
    group: "Ataque",
    rows: [
      ["Ataque físico", (s) => fmt.format(Math.round(s.pAtk))],
      ["Ataque mágico", (s) => fmt.format(Math.round(s.mAtk))],
      ["Chance de crítico", (s) => `${(s.crit * 100).toLocaleString("pt-BR", { maximumFractionDigits: 1 })}%`],
    ],
  },
  {
    group: "Defesa",
    rows: [
      ["Defesa física", (s) => fmt.format(Math.round(s.pDef))],
      ["Defesa mágica", (s) => fmt.format(Math.round(s.mDef))],
    ],
  },
  { group: "Utilidade", rows: [["Velocidade de movimento", (s) => `${s.speed.toLocaleString("pt-BR", { maximumFractionDigits: 1 })} m/s`]] },
];

export function buildCharacter(d: CharacterDeps): HTMLElement {
  const me = d.me();
  const root = el("div", "attr-win char-win");
  const canvas = el("canvas", "attr-canvas");
  const xp = bar("xp");
  const name = el("h3", "title ap-name", me.name);
  const sub = el("div", "ap-sub");
  const portrait = el("section", "attr-portrait", el("div", "ap-stage", canvas), el("div", "ap-info", name, sub, xp.root));
  portrait.style.setProperty("--emblem", me.emblem);
  const pv = createPreview(canvas, { radius: 5.2 });

  const derived = el("section", "attr-derived");
  const vals = new Map<string, HTMLElement>();
  for (const g of ROWS) {
    derived.append(el("h4", "d-group", g.group));
    for (const [label] of g.rows) {
      const v = el("b", "d-val", "–");
      vals.set(label, v);
      derived.append(el("div", "d-row", el("span", "d-name", label), el("span", "d-diff"), v));
    }
  }
  const soon = el(
    "section",
    "attr-base",
    el("div", "attr-head", el("h3", "title", "Atributos")),
    el("p", "hint-box small", "Em breve: a cada nível você ganha pontos para distribuir em Força, Destreza, Constituição, Inteligência, Sabedoria e Espírito."),
  );
  root.append(portrait, soon, el("div", "attr-right", el("h3", "title", "Combate"), derived));

  let lastLook = "";
  const render = () => {
    const m = d.me();
    sub.textContent = `${classInfo(m.myClass).label} · Nível ${m.myLevel}`;
    const s = d.stats();
    if (s) {
      for (const g of ROWS) for (const [label, f] of g.rows) vals.get(label)!.textContent = f(s);
      if (s.xpNextLevel > 0n) {
        const into = Number(s.xp - s.xpLevelStart);
        const need = Number(s.xpNextLevel - s.xpLevelStart);
        xp.set(into, need, `${fmt.format(into)} / ${fmt.format(need)} XP`);
      } else xp.set(1, 1, "Nível máximo");
    }
    const w = d.store.equip[0]?.info;
    const o = d.store.equip[1]?.info;
    const look = { right: w?.model || null, left: o?.model || w?.offModel || null, emblem: m.emblem };
    const key = JSON.stringify(look);
    if (key !== lastLook) {
      lastLook = key;
      void pv.show(m.myClass as ClassId, look);
    }
  };
  render();
  const offStats = d.onStats(render);
  const offStore = d.store.onChange(render);
  root.addEventListener("win-close", () => {
    offStats();
    offStore();
    pv.dispose();
  });
  return root;
}
