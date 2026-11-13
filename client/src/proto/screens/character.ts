/**
 * Seleção e criação de personagem: modelo 3D no pedestal, classes, personalização
 * (armas das mãos, cor do emblema, poses) e nome com validação ao vivo.
 */

import "../styles/screens.css";
import type { ClassId } from "../../api";
import { CLASSES, classInfo } from "../../classes";
import { skillIcon } from "../../ui/icons";
import { ui, uniqueIds } from "../art";
import { DEFAULT_LOOK, game, type Appearance } from "../data/mock";
import { TREES } from "../data/skilltree";
import { art, button, el, isCompact, tabs, toast, tooltip } from "../kit";
import type { Ctx, Screen } from "../main";
import { createPreview } from "../preview3d";
import { animatedBackdrop } from "./backdrop";

const CLASS_EXTRA: Record<ClassId, { role: string; diff: number; bars: [number, number, number, number] }> = {
  knight: { role: "Tanque · corpo a corpo", diff: 1, bars: [55, 95, 15, 45] },
  barbarian: { role: "Dano · corpo a corpo", diff: 2, bars: [90, 60, 10, 55] },
  mage: { role: "Dano · à distância", diff: 3, bars: [20, 30, 100, 45] },
  rogue: { role: "Dano · crítico e mobilidade", diff: 2, bars: [75, 40, 15, 95] },
};
const BAR_NAMES = ["Ataque", "Defesa", "Magia", "Mobilidade"];

const RIGHT: { id: string | null; name: string }[] = [
  { id: "sword_1handed", name: "Espada" },
  { id: "axe_1handed", name: "Machado" },
  { id: "dagger", name: "Adaga" },
  { id: "staff", name: "Cajado" },
  { id: "skeleton_blade", name: "Lâmina curva" },
  { id: "skeleton_axe", name: "Machado serrilhado" },
  { id: "skeleton_staff", name: "Cajado sombrio" },
];
const LEFT: { id: string | null; name: string }[] = [
  { id: null, name: "Nada" },
  { id: "shield_badge_color", name: "Escudo brasonado" },
  { id: "shield_round_barbarian", name: "Broquel" },
  { id: "skeleton_shield_large_a", name: "Escudo pesado" },
  { id: "dagger", name: "Adaga" },
];
const EMBLEMS = ["#d6b46e", "#e2674a", "#d04050", "#8c6fd6", "#4f9dff", "#58c7d6", "#5fae7d", "#e8e4dc"];

const POSES: { role: "idle" | "attack" | "skill" | "buff"; label: string }[] = [
  { role: "idle", label: "Parado" },
  { role: "attack", label: "Ataque" },
  { role: "skill", label: "Skill" },
  { role: "buff", label: "Vitória" },
];

/** Personagens já criados (falsos) para a aba de seleção. */
function roster() {
  const h = game.hero;
  return [
    { name: h.name, cls: h.cls, level: h.level, place: "Campos ao norte", look: game.look(), last: "agora" },
    { name: "Selene", cls: "mage" as ClassId, level: 9, place: "Cidade de Vel'Aran", look: { ...DEFAULT_LOOK.mage, emblem: "#58c7d6" }, last: "ontem" },
  ];
}

export function mount(root: HTMLElement, ctx: Ctx): Screen {
  root.classList.add("char-screen");
  const stopBg = animatedBackdrop(root, { castle: false, tint: "#3a2a48" });

  const stage = el("div", "char-stage");
  const canvas = el("canvas", "char-canvas");
  canvas.setAttribute("aria-label", "Prévia 3D do personagem — arraste para girar");
  const loading = el("div", "stage-loading", el("span", "spinner"), "Carregando modelo…");
  const nameplate = el("div", "nameplate", el("span", "np-name"), el("span", "np-sub"));
  const spinL = button("◀", "btn btn-icon btn-ghost spin-btn l");
  const spinR = button("▶", "btn btn-icon btn-ghost spin-btn r");
  spinL.setAttribute("aria-label", "Girar para a esquerda");
  spinR.setAttribute("aria-label", "Girar para a direita");
  const hint = el("p", "stage-hint", "Arraste para girar · role ou pince para aproximar");
  stage.append(canvas, loading, nameplate, spinL, spinR, hint);

  const preview = createPreview(canvas);
  preview.ready.then(() => loading.remove()).catch(() => (loading.textContent = "Modelos 3D indisponíveis"));
  spinL.onclick = () => preview.spin(-0.6);
  spinR.onclick = () => preview.spin(0.6);

  // ---------- Topo ----------
  let mode: "select" | "create" = "select";
  const top = el("header", "char-top");
  const back = button("", "btn btn-ghost btn-sm", () => ctx.go("login"));
  back.innerHTML = `${ui("logout", 16)}<span>Sair</span>`;
  const modeTabs = tabs(
    [
      { id: "select", label: "Personagens" },
      { id: "create", label: "Criar novo" },
    ],
    mode,
    (id) => setMode(id as typeof mode),
    "tabs mode-tabs",
  );
  top.append(back, modeTabs.root, el("span", "account small muted", "Servidor Aurora · conta Google"));

  // ---------- Seleção ----------
  const selectPanel = el("aside", "char-panel left frame glass select-panel");
  const selectFoot = el("div", "char-foot select-foot");
  let picked = 0;

  const renderSelect = () => {
    const list = roster();
    selectPanel.replaceChildren(el("h2", "title panel-title", "Personagens", el("span", "count", ` ${list.length}/3`)));
    list.forEach((c, i) => {
      const info = classInfo(c.cls);
      const b = el("button", "roster-slot");
      b.type = "button";
      b.setAttribute("aria-pressed", String(i === picked));
      b.style.setProperty("--cc", info.color);
      b.append(
        el("span", "roster-crest", info.label[0]),
        el("span", "roster-info", el("b", null, c.name), el("span", null, `${info.label} · Nv ${c.level}`), el("small", null, c.place)),
      );
      b.onclick = () => {
        picked = i;
        renderSelect();
      };
      selectPanel.append(b);
    });
    const empty = el("button", "roster-slot empty");
    empty.type = "button";
    empty.innerHTML = `${ui("plus", 22)}<span>Criar novo personagem</span>`;
    empty.onclick = () => {
      modeTabs.select("create");
    };
    selectPanel.append(empty);

    const c = list[picked];
    void preview.show(c.cls, c.look);
    setPlate(c.name, `${classInfo(c.cls).label} · Nível ${c.level}`, c.look.emblem);
    const play = button("Entrar no mundo", "btn btn-gold btn-play", () => {
      if (picked !== 0) game.setHero(c.name, c.cls, c.look);
      ctx.go("hud");
    });
    const del = button("", "btn btn-ghost btn-sm", () => toast("No protótipo não dá para excluir.", "warn"));
    del.innerHTML = `${ui("trash", 16)}<span>Excluir</span>`;
    selectFoot.replaceChildren(el("div", "foot-info", el("b", null, c.name), el("span", "muted small", `Última vez: ${c.last} · ${c.place}`)), play, del);
  };

  // ---------- Criação ----------
  let cls: ClassId = "knight";
  let look: Appearance = { ...DEFAULT_LOOK.knight };
  let name = "";

  const classPanel = el("aside", "char-panel left frame glass create-panel");
  const lookPanel = el("aside", "char-panel right frame glass create-panel");
  const createFoot = el("div", "char-foot create-foot");

  const renderClasses = () => {
    classPanel.replaceChildren(el("h2", "title panel-title", "Classe"));
    const grid = el("div", "class-grid");
    for (const c of CLASSES) {
      const x = CLASS_EXTRA[c.id];
      const b = el("button", "class-card2");
      b.type = "button";
      b.setAttribute("aria-pressed", String(c.id === cls));
      b.style.setProperty("--cc", c.color);
      b.append(
        el("span", "cc-crest", c.label[0]),
        el("span", "cc-text", el("b", null, c.label), el("small", null, x.role)),
        el("span", "cc-diff", ...[1, 2, 3].map((n) => el("i", n <= x.diff ? "on" : ""))),
      );
      b.title = `Dificuldade ${["", "fácil", "média", "difícil"][x.diff]}`;
      b.onclick = () => {
        cls = c.id;
        look = { ...DEFAULT_LOOK[cls] };
        renderClasses();
        renderLook();
        refreshPreview();
      };
      grid.append(b);
    }
    const info = classInfo(cls);
    const x = CLASS_EXTRA[cls];
    const detail = el("div", "class-detail");
    detail.append(el("p", "class-desc", info.desc));
    const bars = el("div", "class-bars");
    x.bars.forEach((v, i) => bars.append(el("span", "cb-name", BAR_NAMES[i]), el("span", "cb-bar", el("i", null)), el("span", "cb-val", String(v))));
    bars.querySelectorAll<HTMLElement>(".cb-bar i").forEach((b, i) => b.style.setProperty("--v", `${x.bars[i]}%`));
    detail.append(bars);
    const skills = el("div", "class-skills");
    const roots = TREES[cls].nodes.filter((n) => n.fx);
    for (const n of roots) {
      const s = el("div", "cs-skill", art(uniqueIds(skillIcon(n.fx!)), "cs-icon"), el("span", null, el("b", null, n.name), el("small", null, n.kind === "active" ? `${n.mp} MP · ${n.cd} s` : "Passiva")));
      tooltip(s, () => `<h4>${n.name}</h4><div class="sub">Skill inicial</div>${n.effect(1)}`);
      skills.append(s);
    }
    detail.append(el("div", "label", "Skills iniciais"), skills);
    classPanel.append(grid, detail);
  };

  const optionRow = (label: string, opts: { id: string | null; name: string }[], value: string | null, onPick: (id: string | null) => void) => {
    const row = el("div", "opt-row");
    const i = Math.max(0, opts.findIndex((o) => o.id === value));
    const text = el("span", "opt-value", opts[i].name);
    let idx = i;
    const step = (d: number) => {
      idx = (idx + d + opts.length) % opts.length;
      text.textContent = opts[idx].name;
      onPick(opts[idx].id);
    };
    const prev = button("‹", "btn btn-icon btn-sm opt-step", () => step(-1));
    const next = button("›", "btn btn-icon btn-sm opt-step", () => step(1));
    prev.setAttribute("aria-label", `${label}: anterior`);
    next.setAttribute("aria-label", `${label}: próximo`);
    row.append(el("span", "label", label), el("div", "opt-ctrl", prev, text, next));
    return row;
  };

  const renderLook = () => {
    lookPanel.replaceChildren(el("h2", "title panel-title", "Aparência"));
    lookPanel.append(
      optionRow("Mão direita", RIGHT, look.right, (id) => {
        look.right = id;
        refreshPreview();
      }),
      optionRow("Mão esquerda", LEFT, look.left, (id) => {
        look.left = id;
        refreshPreview();
      }),
    );
    const sw = el("div", "swatches");
    sw.setAttribute("role", "radiogroup");
    sw.setAttribute("aria-label", "Cor do emblema");
    for (const c of EMBLEMS) {
      const b = el("button", "swatch");
      b.type = "button";
      b.style.setProperty("--c", c);
      b.setAttribute("role", "radio");
      b.setAttribute("aria-checked", String(c === look.emblem));
      b.setAttribute("aria-label", `Cor ${c}`);
      b.onclick = () => {
        look.emblem = c;
        sw.querySelectorAll(".swatch").forEach((x) => x.setAttribute("aria-checked", String(x === b)));
        preview.setEmblem(c);
        setPlate(name || "Seu nome", classInfo(cls).label, c);
      };
      sw.append(b);
    }
    lookPanel.append(el("div", "opt-block", el("span", "label", "Cor do emblema"), sw));

    const poses = el("div", "pose-row");
    for (const p of POSES) poses.append(button(p.label, "btn btn-sm", () => preview.play(p.role)));
    lookPanel.append(el("div", "opt-block", el("span", "label", "Ver pose"), poses));
    lookPanel.append(
      el(
        "p",
        "hint-box small",
        "Rosto, cabelo e cores do traje entram quando trocarmos os modelos atuais por modelos com peças separadas.",
      ),
    );
  };

  const nameInput = el("input", "input name-input");
  nameInput.placeholder = "Nome do personagem";
  nameInput.maxLength = 16;
  nameInput.autocomplete = "off";
  nameInput.setAttribute("aria-label", "Nome do personagem");
  const nameMsg = el("span", "name-msg small");
  const createBtn = button("Criar personagem", "btn btn-gold btn-play");
  const TAKEN = new Set(["aldric", "selene", "admin", "erevia"]);
  const validate = () => {
    name = nameInput.value.trim();
    let msg = "";
    let ok = false;
    if (!name) msg = "3 a 16 letras ou números.";
    else if (!/^[A-Za-z0-9]+$/.test(name)) msg = "Só letras e números, sem espaço ou acento.";
    else if (name.length < 3) msg = "Mínimo de 3 caracteres.";
    else if (TAKEN.has(name.toLowerCase())) msg = "Esse nome já está em uso.";
    else {
      msg = "✓ Nome disponível";
      ok = true;
    }
    nameMsg.textContent = msg;
    nameMsg.className = `name-msg small ${ok ? "ok" : name ? "bad" : "muted"}`;
    nameInput.classList.toggle("invalid", !!name && !ok);
    createBtn.disabled = !ok;
    setPlate(name || "Seu nome", classInfo(cls).label, look.emblem);
    return ok;
  };
  nameInput.oninput = validate;
  createBtn.onclick = () => {
    if (!validate()) return;
    game.setHero(name, cls, { ...look });
    toast(`${name} foi criado!`);
    picked = 0;
    modeTabs.select("select");
  };
  createFoot.append(el("div", "name-field", nameInput, nameMsg), createBtn);

  // Abas do celular (Classe / Aparência / Nome)
  const mobileTabs = tabs(
    [
      { id: "class", label: "Classe" },
      { id: "look", label: "Aparência" },
    ],
    "class",
    (id) => {
      root.dataset.mtab = id;
    },
    "tabs mobile-tabs create-panel",
  );
  root.dataset.mtab = "class";

  function refreshPreview() {
    void preview.show(cls, look);
    setPlate(name || "Seu nome", classInfo(cls).label, look.emblem);
  }

  function setPlate(n: string, sub: string, color: string) {
    nameplate.querySelector(".np-name")!.textContent = n;
    nameplate.querySelector(".np-sub")!.textContent = sub;
    nameplate.style.setProperty("--emblem", color);
  }

  function setMode(m: typeof mode) {
    mode = m;
    root.dataset.mode = m;
    if (m === "select") renderSelect();
    else {
      renderClasses();
      renderLook();
      validate();
      refreshPreview();
      if (!isCompact()) nameInput.focus({ preventScroll: true });
    }
  }

  root.append(stage, top, selectPanel, classPanel, mobileTabs.root, lookPanel, selectFoot, createFoot);
  setMode("select");

  return {
    dispose() {
      preview.dispose();
      stopBg();
    },
  };
}
