/**
 * Protótipo visual das telas do Erevia (dados falsos, sem servidor).
 * Rotas por hash: #login, #personagem, #hud, #config, #inventario, #skills, #atributos.
 * As quatro últimas abrem a janela por cima do HUD, como no jogo.
 */

import "../styles/tokens.css";
import "../styles/kit.css";
import "./styles/proto.css";
import { applyEnvClasses, el, settings } from "../ui/kit";
import { ui } from "../ui/art";

export interface Screen {
  dispose(): void;
  /** Só o HUD: abre uma janela (config, inventario, skills, atributos). */
  openWindow?(id: string): void;
}

export interface Ctx {
  go(route: string): void;
}

const ROUTES = [
  { id: "login", label: "Login", icon: "lock" },
  { id: "personagem", label: "Personagem", icon: "user" },
  { id: "hud", label: "HUD", icon: "sword" },
  { id: "config", label: "Configurações", icon: "gear" },
  { id: "inventario", label: "Inventário", icon: "bag" },
  { id: "skills", label: "Skills", icon: "book" },
  { id: "atributos", label: "Atributos", icon: "sparkle" },
] as const;

const WINDOW_ROUTES = new Set(["config", "inventario", "skills", "atributos"]);

const params = new URLSearchParams(location.search);
const embedded = params.has("embed");
const app = document.getElementById("app")!;

let current: { route: string; screen: Screen } | null = null;
let loading = 0;

const ctx: Ctx = {
  go(route) {
    if (location.hash.slice(1) === route) void render();
    else location.hash = route;
  },
};

function routeFromHash(): string {
  const r = location.hash.slice(1);
  return ROUTES.some((x) => x.id === r) ? r : "login";
}

async function render(): Promise<void> {
  const route = routeFromHash();
  syncBar(route);
  const base = WINDOW_ROUTES.has(route) ? "hud" : route;
  const token = ++loading;
  // No HUD a barra vira uma aba na borda esquerda, para não tapar os atalhos.
  if (bar && !barTouched) bar.classList.toggle("collapsed", base === "hud");

  if (current?.route === base && base === "hud") {
    if (route !== "hud") current.screen.openWindow?.(route);
    return;
  }

  current?.screen.dispose();
  current = null;
  app.replaceChildren();
  document.querySelectorAll(".gwin-layer, .toasts, .tip, [data-menu], [data-sheet]").forEach((n) => n.remove());

  let mod: { mount(root: HTMLElement, ctx: Ctx): Promise<Screen> | Screen };
  switch (base) {
    case "personagem":
      mod = await import("./screens/character");
      break;
    case "hud":
      mod = await import("./screens/hud");
      break;
    default:
      mod = await import("./screens/login");
  }
  if (token !== loading) return;
  const root = el("div", "screen");
  app.append(root);
  const screen = await mod.mount(root, ctx);
  if (token !== loading) {
    screen.dispose();
    return;
  }
  current = { route: base, screen };
  if (route !== base) screen.openWindow?.(route);
}

// ---------------------------------------------------------------------------------------
// Barra do protótipo + "Ver como" (aparelho simulado num iframe)
// ---------------------------------------------------------------------------------------

type Device = "full" | "phone" | "phone-land" | "tablet";
const DEVICES: { id: Device; label: string; icon: string; w: number; h: number }[] = [
  { id: "full", label: "Tela atual", icon: "desktop", w: 0, h: 0 },
  { id: "phone", label: "Celular em pé", icon: "phone", w: 390, h: 844 },
  { id: "phone-land", label: "Celular deitado", icon: "phone", w: 844, h: 390 },
  { id: "tablet", label: "Tablet", icon: "tablet", w: 1024, h: 768 },
];

let bar: HTMLElement | null = null;
/** O usuário abriu/fechou a barra na mão (aí não mexemos mais sozinhos). */
let barTouched = false;
const links = new Map<string, HTMLAnchorElement>();
let stage: { root: HTMLElement; frame: HTMLIFrameElement; device: Device } | null = null;

function buildBar(): void {
  bar = el("nav", "proto-bar");
  bar.setAttribute("aria-label", "Navegação do protótipo");
  const brand = el("button", "proto-brand");
  brand.type = "button";
  brand.innerHTML = `<span class="dot"></span>Protótipo`;
  brand.title = "Recolher/abrir a barra";
  brand.onclick = () => {
    barTouched = true;
    bar!.classList.toggle("collapsed");
  };

  const nav = el("div", "proto-links");
  for (const r of ROUTES) {
    const a = el("a", "proto-link");
    a.href = `#${r.id}`;
    a.innerHTML = `${ui(r.icon, 16)}<span>${r.label}</span>`;
    a.onclick = (ev) => {
      if (stage) {
        ev.preventDefault();
        stage.frame.contentWindow!.location.hash = r.id;
        setActive(r.id);
      }
    };
    links.set(r.id, a);
    nav.append(a);
  }

  const dev = el("div", "proto-devices");
  for (const d of DEVICES) {
    const b = el("button", "proto-dev");
    b.type = "button";
    b.dataset.dev = d.id;
    b.title = d.label;
    b.innerHTML = `${ui(d.icon, 16)}<span>${d.label}</span>`;
    if (d.id === "phone-land") b.querySelector("svg")!.style.transform = "rotate(90deg)";
    b.onclick = () => setDevice(d.id);
    dev.append(b);
  }
  bar.append(brand, nav, dev);
  document.body.append(bar);
  document.documentElement.classList.add("proto-bar-on");
  setDeviceButtons("full");
}

function setActive(route: string): void {
  for (const [id, a] of links) a.classList.toggle("active", id === route);
}

function syncBar(route: string): void {
  if (!stage) setActive(route);
}

function setDeviceButtons(d: Device): void {
  bar?.querySelectorAll<HTMLButtonElement>(".proto-dev").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.dev === d)));
}

function setDevice(d: Device): void {
  setDeviceButtons(d);
  const route = stage ? (stage.frame.contentWindow?.location.hash.slice(1) ?? routeFromHash()) : routeFromHash();
  if (d === "full") {
    stage?.root.remove();
    stage = null;
    app.hidden = false;
    if (location.hash.slice(1) !== route) location.hash = route;
    else void render();
    return;
  }
  // Desliga a tela principal (libera a GPU) e mostra o aparelho.
  current?.screen.dispose();
  current = null;
  app.replaceChildren();
  app.hidden = true;
  document.querySelectorAll(".gwin-layer, .toasts").forEach((n) => n.remove());

  const spec = DEVICES.find((x) => x.id === d)!;
  if (!stage) {
    const root = el("div", "device-stage");
    const frame = el("iframe", "device-frame");
    frame.title = "Prévia no aparelho";
    const shell = el("div", "device-shell", el("div", "device-notch"), frame);
    const caption = el("p", "device-caption");
    root.append(shell, caption);
    document.body.append(root);
    stage = { root, frame, device: d };
    frame.src = `${location.pathname}?embed=1&touch=1#${route}`;
    frame.addEventListener("load", () => {
      frame.contentWindow?.addEventListener("hashchange", () => setActive(frame.contentWindow!.location.hash.slice(1)));
    });
  }
  stage.device = d;
  const shell = stage.root.querySelector<HTMLElement>(".device-shell")!;
  shell.dataset.dev = d;
  shell.style.setProperty("--dw", `${spec.w}px`);
  shell.style.setProperty("--dh", `${spec.h}px`);
  stage.root.querySelector(".device-caption")!.textContent = `${spec.label} · ${spec.w}×${spec.h} · controles de toque ligados`;
  fitStage();
  setActive(route);
}

function fitStage(): void {
  if (!stage) return;
  const shell = stage.root.querySelector<HTMLElement>(".device-shell")!;
  const spec = DEVICES.find((x) => x.id === stage!.device)!;
  const pad = 28;
  const availW = innerWidth - 40;
  const availH = innerHeight - 130;
  const k = Math.min(1, availW / (spec.w + pad), availH / (spec.h + pad));
  shell.style.setProperty("--k", String(k));
}

// ---------------------------------------------------------------------------------------

applyEnvClasses();
settings.on(applyEnvClasses);
document.documentElement.classList.toggle("embedded", embedded);
if (!embedded) buildBar();
addEventListener("hashchange", () => void render());
addEventListener("resize", () => {
  applyEnvClasses();
  fitStage();
});
void render();
