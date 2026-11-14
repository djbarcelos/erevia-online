/**
 * Personagem (tecla C): modelo 3D com o equipamento, nível e XP, os 6 atributos com
 * pontos livres e os atributos de combate. Antes de confirmar, mostra a diferença que os
 * pontos vão fazer: o servidor manda o efeito de +1 ponto em cada atributo
 * (Progress.per_point) e as fórmulas são lineares, então a prévia é exata.
 */

import type { ClassId } from "../../api";
import { classInfo } from "../../classes";
import type { Progress, SelfStats, StatBlock } from "../../gen/erevia/v1/game_pb";
import type { ItemStore } from "../items";
import { bar, button, el, fmt, toast, tooltip } from "../kit";
import { createPreview } from "../preview3d";
import { specInfo } from "../specs";
import type { BodyLook } from "../../world/body-look";

export interface CharacterDeps {
  store: ItemStore;
  /** myClass: a classe ativa; bodyClass: a principal (o modelo da prévia). */
  me(): { myClass: string; myLevel: number; name: string; emblem: string; body?: BodyLook; bodyClass?: string };
  stats(): SelfStats | null;
  progress(): Progress | null;
  onStats(fn: () => void): () => void;
  onProgress(fn: () => void): () => void;
  allocate(add: Attrs): void;
  /** PvP: karma, PKs, vitórias e segundos de flag restantes. */
  pvp(): { karma: number; pks: number; wins: number; flag: number };
  onPvp(fn: () => void): () => void;
}

type AttrId = "str" | "dex" | "con" | "int" | "wis" | "spi";
type Attrs = Record<AttrId, number>;

const ATTRS: { id: AttrId; name: string; short: string; color: string; desc: string; affects: string }[] = [
  { id: "str", name: "Força", short: "FOR", color: "#e2674a", desc: "Músculo e peso do golpe.", affects: "+2% de ataque físico por ponto." },
  { id: "dex", name: "Destreza", short: "DES", color: "#e9c05a", desc: "Reflexo e precisão.", affects: "+0,4% de crítico e +1% de velocidade de ataque por ponto; +1,5 de precisão e +1,2 de esquiva." },
  { id: "con", name: "Constituição", short: "CON", color: "#6fc46a", desc: "Vigor e resistência do corpo.", affects: "+2,5% de vida, +1% de defesa física e +0,4 de vida por segundo por ponto." },
  { id: "int", name: "Inteligência", short: "INT", color: "#6c8cff", desc: "Domínio da magia.", affects: "+2,5% de ataque mágico por ponto." },
  { id: "wis", name: "Sabedoria", short: "SAB", color: "#58c7d6", desc: "Foco e calma ao conjurar.", affects: "+1,5% de velocidade de conjuração (recarga menor) e +0,3 de mana por segundo por ponto." },
  { id: "spi", name: "Espírito", short: "ESP", color: "#b67de8", desc: "Força de vontade.", affects: "+3% de mana e +1,5% de defesa mágica por ponto." },
];

/** Sugestão de distribuição por classe (pesos). */
const SUGGEST: Record<ClassId, Partial<Attrs>> = {
  knight: { con: 2, str: 1 },
  barbarian: { str: 2, con: 1 },
  mage: { int: 2, spi: 1 },
  rogue: { dex: 2, str: 1 },
};

type StatKey = keyof Omit<StatBlock, "$typeName" | "$unknown">;

const num = (v: number) => fmt.format(Math.round(v));
const dec = (d: number, suf = "") => (v: number) => `${v.toLocaleString("pt-BR", { minimumFractionDigits: d, maximumFractionDigits: d })}${suf}`;
const pct = (v: number) => `${(v * 100).toLocaleString("pt-BR", { maximumFractionDigits: 1 })}%`;

const DERIVED: { group: string; rows: { key: StatKey; self: (s: SelfStats) => number; name: string; fmt: (v: number) => string; eps: number }[] }[] = [
  {
    group: "Recursos",
    rows: [
      { key: "maxHp", self: (s) => s.maxHp, name: "Vida máxima", fmt: num, eps: 0.5 },
      { key: "maxMp", self: (s) => s.maxMp, name: "Mana máxima", fmt: num, eps: 0.5 },
      { key: "hpRegen", self: (s) => s.hpRegen, name: "Vida por segundo", fmt: dec(1), eps: 0.05 },
      { key: "mpRegen", self: (s) => s.mpRegen, name: "Mana por segundo", fmt: dec(1), eps: 0.05 },
    ],
  },
  {
    group: "Ataque",
    rows: [
      { key: "pAtk", self: (s) => s.pAtk, name: "Ataque físico", fmt: num, eps: 0.5 },
      { key: "mAtk", self: (s) => s.mAtk, name: "Ataque mágico", fmt: num, eps: 0.5 },
      { key: "crit", self: (s) => s.crit, name: "Chance de crítico", fmt: pct, eps: 0.0005 },
      { key: "critDamage", self: (s) => s.critDamage, name: "Dano crítico", fmt: pct, eps: 0.0005 },
      { key: "accuracy", self: (s) => s.accuracy, name: "Precisão", fmt: num, eps: 0.5 },
      { key: "atkSpeed", self: (s) => s.atkSpeed, name: "Ataques por segundo", fmt: dec(2), eps: 0.005 },
      { key: "castSpeed", self: (s) => s.castSpeed, name: "Velocidade de conjuração", fmt: pct, eps: 0.0005 },
    ],
  },
  {
    group: "Defesa",
    rows: [
      { key: "pDef", self: (s) => s.pDef, name: "Defesa física", fmt: num, eps: 0.5 },
      { key: "mDef", self: (s) => s.mDef, name: "Defesa mágica", fmt: num, eps: 0.5 },
      { key: "evasion", self: (s) => s.evasion, name: "Esquiva", fmt: num, eps: 0.5 },
    ],
  },
  { group: "Utilidade", rows: [{ key: "speed", self: (s) => s.speed, name: "Velocidade de movimento", fmt: dec(1, " m/s"), eps: 0.05 }] },
];

const zero = (): Attrs => ({ str: 0, dex: 0, con: 0, int: 0, wis: 0, spi: 0 });
const sum = (a: Attrs) => Object.values(a).reduce((x, y) => x + y, 0);

export function buildCharacter(d: CharacterDeps): HTMLElement {
  const me = d.me();
  let draft = zero();
  const root = el("div", "attr-win");

  // ---------- Retrato 3D ----------
  const canvas = el("canvas", "attr-canvas");
  const xp = bar("xp");
  const sub = el("div", "ap-sub");
  // Especialização (segunda classe): nome na cor do caminho e o bônus de atributos.
  const specLine = el("div", "ap-spec small");
  const portrait = el("section", "attr-portrait", el("div", "ap-stage", canvas), el("div", "ap-info", el("h3", "title ap-name", me.name), sub, specLine, xp.root));
  portrait.style.setProperty("--emblem", me.emblem);
  const pv = createPreview(canvas, { radius: 5.2 });

  // ---------- Atributos ----------
  const pointsEl = el("div", "attr-points");
  const rows = new Map<AttrId, { val: HTMLElement; add: HTMLElement; plus: HTMLButtonElement; minus: HTMLButtonElement; fill: HTMLElement }>();
  const list = el("div", "attr-list");
  for (const a of ATTRS) {
    const val = el("span", "a-val");
    const add = el("span", "a-add");
    const fill = el("i");
    const minus = button("−", "a-btn", () => change(a.id, -1));
    const plus = button("+", "a-btn plus", () => change(a.id, 1));
    minus.setAttribute("aria-label", `Tirar ponto de ${a.name}`);
    plus.setAttribute("aria-label", `Pôr ponto em ${a.name}`);
    // Segurar o + adiciona vários.
    let hold = 0;
    plus.addEventListener("pointerdown", () => {
      hold = window.setTimeout(function rep() {
        change(a.id, 1);
        hold = window.setTimeout(rep, 90);
      }, 380);
    });
    for (const ev of ["pointerup", "pointerleave", "pointercancel"]) plus.addEventListener(ev, () => clearTimeout(hold));
    const name = el("div", "a-name", el("span", "a-short", a.short), el("span", null, a.name));
    tooltip(name, () => `<h4 style="color:${a.color}">${a.name}</h4><div class="desc">${a.desc}</div><div class="cmp">${a.affects}</div>`);
    const row = el("div", "a-row", name, el("div", "a-meter", fill), minus, el("span", "a-num", val, add), plus);
    row.style.setProperty("--ac", a.color);
    rows.set(a.id, { val, add, plus, minus, fill });
    list.append(row);
  }
  const free = () => (d.progress()?.freeAttrPoints ?? 0) - sum(draft);
  const undo = button("Desfazer", "btn btn-sm", () => {
    draft = zero();
    render();
  });
  const suggest = button("Sugestão da classe", "btn btn-sm", () => {
    let left = free();
    const order = (Object.entries(SUGGEST[me.myClass as ClassId] ?? { con: 1 }) as [AttrId, number][]).flatMap(([k, n]) => Array(n).fill(k) as AttrId[]);
    let i = 0;
    while (left-- > 0) draft[order[i++ % order.length]]++;
    render();
  });
  const confirm = button("Confirmar", "btn btn-gold btn-sm", () => {
    if (sum(draft) <= 0) return;
    d.allocate(draft);
    toast("Atributos distribuídos!");
  });
  const attrs = el(
    "section",
    "attr-base",
    el("div", "attr-head", el("h3", "title", "Atributos"), pointsEl),
    list,
    el("div", "attr-actions", undo, suggest, confirm),
    el("p", "small muted attr-note", "Cada nível dá 3 pontos. A base vem da classe (e do caminho, a partir do nível 20); os pontos somam em cima."),
  );

  // ---------- Combate ----------
  const derived = el("section", "attr-derived");
  const dRows = new Map<StatKey, { val: HTMLElement; diff: HTMLElement }>();
  for (const g of DERIVED) {
    derived.append(el("h4", "d-group", g.group));
    for (const r of g.rows) {
      const val = el("b", "d-val", "–");
      const diff = el("span", "d-diff");
      derived.append(el("div", "d-row", el("span", "d-name", r.name), diff, val));
      dRows.set(r.key, { val, diff });
    }
  }
  // ---------- PvP ----------
  const pvpRows = {
    karma: el("b", "d-val"),
    pks: el("b", "d-val"),
    wins: el("b", "d-val"),
    flag: el("b", "d-val"),
  };
  derived.append(
    el("h4", "d-group", "PvP"),
    el("div", "d-row", el("span", "d-name", "Karma"), el("span", "d-diff"), pvpRows.karma),
    el("div", "d-row", el("span", "d-name", "Jogadores mortos sem luta (PK)"), el("span", "d-diff"), pvpRows.pks),
    el("div", "d-row", el("span", "d-name", "Vitórias em combate"), el("span", "d-diff"), pvpRows.wins),
    el("div", "d-row", el("span", "d-name", "Em combate (nome roxo)"), el("span", "d-diff"), pvpRows.flag),
  );
  tooltip(
    pvpRows.karma.parentElement!,
    () =>
      `<h4>Karma</h4><div class="desc">Matar quem não estava lutando dá karma e deixa seu nome vermelho.</div><div class="cmp">Com karma você perde o dobro de XP e pode deixar itens caírem ao morrer, pode ser atacado dentro da cidade e não usa teleporte. Caçar monstros reduz o karma.</div>`,
  );
  const renderPvp = () => {
    const v = d.pvp();
    pvpRows.karma.textContent = fmt.format(v.karma);
    pvpRows.karma.style.color = v.karma > 0 ? "#ff5a4a" : "";
    pvpRows.pks.textContent = String(v.pks);
    pvpRows.wins.textContent = String(v.wins);
    pvpRows.flag.textContent = v.flag > 0 ? `${Math.ceil(v.flag)} s` : "Não";
    pvpRows.flag.style.color = v.flag > 0 ? "#c77dff" : "";
  };
  renderPvp();
  const pvpTimer = window.setInterval(renderPvp, 1000);
  const offPvp = d.onPvp(renderPvp);

  root.append(portrait, attrs, el("div", "attr-right", el("h3", "title", "Combate"), derived));

  function change(id: AttrId, delta: number) {
    if (delta > 0 && free() <= 0) return;
    if (delta < 0 && draft[id] <= 0) return;
    draft[id] += delta;
    render();
  }

  let lastLook = "";
  function render() {
    const m = d.me();
    const p = d.progress();
    const s = d.stats();
    const spec = specInfo(p?.spec);
    sub.textContent = `${classInfo(m.myClass).label}${spec ? ` · ${spec.name}` : ""} · Nível ${m.myLevel}`;
    specLine.hidden = !spec;
    if (spec) {
      specLine.textContent = `Caminho: ${spec.name} (${spec.bonusText})`;
      specLine.style.color = spec.color;
    }
    if (s) {
      if (s.xpNextLevel > 0n) {
        const into = Number(s.xp - s.xpLevelStart);
        const need = Number(s.xpNextLevel - s.xpLevelStart);
        xp.set(into, need, `${fmt.format(into)} / ${fmt.format(need)} XP`);
      } else xp.set(1, 1, "Nível máximo");
    }
    // Atributos: base da classe + pontos + rascunho
    const f = free();
    pointsEl.replaceChildren(el("span", "label", "Pontos livres"), el("b", f > 0 ? "gold pulse" : "", String(Math.max(0, f))));
    // Base da classe + bônus da especialização + pontos + rascunho.
    const values = ATTRS.map((a) => (p?.base?.[a.id] ?? 0) + (spec?.bonus?.[a.id] ?? 0) + (p?.spent?.[a.id] ?? 0) + draft[a.id]);
    const top = Math.max(30, ...values);
    ATTRS.forEach((a, i) => {
      const r = rows.get(a.id)!;
      r.val.textContent = String(values[i]);
      r.add.textContent = draft[a.id] ? ` +${draft[a.id]}` : "";
      r.plus.disabled = f <= 0;
      r.minus.disabled = draft[a.id] <= 0;
      r.fill.style.width = `${(values[i] / top) * 100}%`;
      r.fill.style.setProperty("--base", `${((p?.base?.[a.id] ?? 0) / Math.max(1, values[i])) * 100}%`);
    });
    // Combate: valor atual + prévia (soma dos +1 ponto de cada atributo do rascunho)
    for (const g of DERIVED) {
      for (const row of g.rows) {
        const r = dRows.get(row.key)!;
        const now = s ? row.self(s) : 0;
        let diff = 0;
        ATTRS.forEach((a, i) => {
          const per = p?.perPoint[i];
          if (per && draft[a.id]) diff += (per[row.key] as number) * draft[a.id];
        });
        r.val.textContent = s ? row.fmt(now + diff) : "–";
        r.val.classList.toggle("changed", Math.abs(diff) > row.eps);
        r.diff.textContent = Math.abs(diff) > row.eps ? `${diff > 0 ? "▲" : "▼"} ${row.fmt(Math.abs(diff))}` : "";
        r.diff.className = `d-diff ${diff > 0 ? "ok" : "bad"}`;
      }
    }
    const dirty = sum(draft) > 0;
    undo.disabled = !dirty;
    confirm.disabled = !dirty;
    suggest.disabled = f <= 0;
    // Modelo com o equipamento atual
    const w = d.store.equip[0]?.info;
    const o = d.store.equip[1]?.info;
    const look = { right: w?.model || null, left: o?.model || w?.offModel || null, emblem: m.emblem, body: m.body };
    const key = JSON.stringify(look);
    if (key !== lastLook) {
      lastLook = key;
      void pv.show((m.bodyClass ?? m.myClass) as ClassId, look);
    }
  }

  render();
  const offStats = d.onStats(render);
  const offStore = d.store.onChange(render);
  // Resposta do servidor: o rascunho já foi aplicado (ou recusado); recomeça do zero.
  const offProgress = d.onProgress(() => {
    draft = zero();
    render();
  });
  root.addEventListener("win-close", () => {
    offStats();
    offStore();
    offProgress();
    offPvp();
    window.clearInterval(pvpTimer);
    pv.dispose();
  });
  return root;
}
