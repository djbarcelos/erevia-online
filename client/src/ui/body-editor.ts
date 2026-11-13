/**
 * Editor do visual do corpo: rosto, pele, cabelo, roupa e chapéu.
 * Usado na criação de personagem e na janela do barbeiro.
 */

import { normalLook, ORIGINAL_LOOK, type BodyLook, type BodyPalettes, type Shade } from "../world/body-look";
import { button, el } from "./kit";

export interface BodyEditor {
  root: HTMLElement;
  /** Troca o valor mostrado (sem chamar onChange). */
  set(look: BodyLook, cls?: string): void;
}

/** Linha "‹ valor ›" que percorre uma lista. */
function stepper(label: string, list: { id: string; name: string }[], get: () => string, onPick: (id: string) => void) {
  const text = el("span", "opt-value");
  const idx = () => Math.max(0, list.findIndex((o) => o.id === get()));
  const render = () => (text.textContent = list[idx()]?.name ?? "");
  const step = (dir: number) => {
    const i = (idx() + dir + list.length) % list.length;
    onPick(list[i].id);
    render();
  };
  const prev = button("‹", "btn btn-icon btn-sm opt-step", () => step(-1));
  const next = button("›", "btn btn-icon btn-sm opt-step", () => step(1));
  prev.setAttribute("aria-label", `${label}: anterior`);
  next.setAttribute("aria-label", `${label}: próximo`);
  render();
  return { root: el("div", "opt-row", el("span", "label", label), el("div", "opt-ctrl", prev, text, next)), render };
}

/** Fileira de cores: "Original" (0) e as da paleta (1..n). */
function shadeRow(label: string, shades: Shade[], get: () => number, onPick: (i: number) => void) {
  const row = el("div", "swatches body-swatches");
  row.setAttribute("role", "radiogroup");
  row.setAttribute("aria-label", label);
  const buttons: HTMLButtonElement[] = [];
  const add = (i: number, name: string, shade?: Shade) => {
    const b = el("button", shade ? "swatch shade" : "swatch original");
    b.type = "button";
    if (shade) {
      b.style.setProperty("--t", shade.top);
      b.style.setProperty("--b", shade.bottom);
      b.style.setProperty("--c", shade.top);
    }
    b.setAttribute("role", "radio");
    b.setAttribute("aria-label", `${label}: ${name}`);
    b.title = name;
    b.onclick = () => {
      onPick(i);
      render();
    };
    buttons.push(b);
    row.append(b);
  };
  add(0, "Original");
  shades.forEach((s, i) => add(i + 1, s.name, s));
  const name = el("span", "body-shade-name muted small");
  const render = () => {
    const v = get();
    buttons.forEach((b, i) => b.setAttribute("aria-checked", String(i === v)));
    name.textContent = v === 0 ? "Original" : (shades[v - 1]?.name ?? "");
  };
  render();
  return { root: el("div", "opt-block", el("span", "label body-label", label, name), row), render };
}

export function bodyEditor(pal: BodyPalettes, cls: string, value: BodyLook, onChange: (l: BodyLook) => void): BodyEditor {
  let model = cls;
  let look = normalLook(model, value, pal);
  const root = el("div", "body-editor");
  const change = (patch: Partial<BodyLook>) => {
    look = normalLook(model, { ...look, ...patch }, pal);
    onChange({ ...look });
  };

  const heads = pal.heads.map((h) => ({ id: h.id, name: h.name }));
  const face = stepper("Rosto e cabelo", heads, () => look.head || model, (id) => change({ head: id }));
  const skin = shadeRow("Pele", pal.skins, () => look.skin, (i) => change({ skin: i }));
  const hair = shadeRow("Cabelo", pal.hairs, () => look.hair, (i) => change({ hair: i }));
  const outfit = shadeRow("Roupa", pal.outfits, () => look.outfit, (i) => change({ outfit: i }));
  const hatOpts = [
    { id: "show", name: "Mostrar" },
    { id: "hide", name: "Esconder" },
  ];
  const hat = stepper("Chapéu", hatOpts, () => (look.hideHat ? "hide" : "show"), (id) => change({ hideHat: id === "hide" }));

  const rand = (n: number) => Math.floor(Math.random() * (n + 1));
  const random = button("Aleatório", "btn btn-sm", () => {
    change({
      skin: rand(pal.skins.length),
      hair: rand(pal.hairs.length),
      outfit: rand(pal.outfits.length),
      head: heads[Math.floor(Math.random() * heads.length)].id,
      hideHat: Math.random() < 0.35,
    });
    renderAll();
  });
  const reset = button("Original", "btn btn-sm btn-ghost", () => {
    change({ ...ORIGINAL_LOOK });
    renderAll();
  });

  const renderAll = () => {
    face.render();
    skin.render();
    hair.render();
    outfit.render();
    hat.render();
    hat.root.hidden = !pal.hatClasses.includes(model);
  };
  root.append(face.root, hat.root, skin.root, hair.root, outfit.root, el("div", "body-actions", random, reset));
  renderAll();

  return {
    root,
    set(l, c) {
      if (c) model = c;
      look = normalLook(model, l, pal);
      renderAll();
    },
  };
}
