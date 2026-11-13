/**
 * Kit de interface do jogo (e do protótipo): criação de elementos, janelas, abas, dicas, menu de
 * contexto, avisos, painel inferior (celular) e controles de formulário.
 */

type Child = Node | string | null | undefined | false;

/** Cria um elemento. `cls` aceita várias classes separadas por espaço. */
export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  cls?: string | null,
  ...children: Child[]
): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  for (const c of children) if (c !== null && c !== undefined && c !== false) e.append(c);
  return e;
}

/** Elemento com HTML interno (para SVG já pronto). */
export function art(svg: string, cls = "art"): HTMLSpanElement {
  const s = document.createElement("span");
  s.className = cls;
  s.innerHTML = svg;
  return s;
}

export function button(text: string, cls = "btn", onClick?: (ev: MouseEvent) => void): HTMLButtonElement {
  const b = el("button", cls, text);
  b.type = "button";
  if (onClick) b.addEventListener("click", onClick);
  return b;
}

export const fmt = new Intl.NumberFormat("pt-BR");
export const fmt1 = new Intl.NumberFormat("pt-BR", { maximumFractionDigits: 1 });

export function clamp(v: number, a: number, b: number): number {
  return Math.max(a, Math.min(b, v));
}

// ---------------------------------------------------------------------------------------
// Ambiente: toque e tamanho de tela
// ---------------------------------------------------------------------------------------

const coarse = matchMedia("(pointer: coarse)");

/** Controles de toque ligados (aparelho de toque ou forçado nas configurações / ?touch=1). */
export function isTouch(): boolean {
  const s = settings.get();
  if (s.touchMode === "on") return true;
  if (s.touchMode === "off") return false;
  return coarse.matches || new URLSearchParams(location.search).get("touch") === "1";
}

/** Layout compacto (celular): janelas ocupam a tela toda. */
export function isCompact(): boolean {
  return innerWidth < 768 || innerHeight <= 520;
}

export function applyEnvClasses(): void {
  const root = document.documentElement;
  root.classList.toggle("is-touch", isTouch());
  const s = settings.get();
  root.style.setProperty("--ui-scale", String(s.uiScale / 100));
  root.classList.toggle("lefty", s.lefty);
}

// ---------------------------------------------------------------------------------------
// Configurações (salvas no navegador)
// ---------------------------------------------------------------------------------------

export interface Settings {
  /** Preset de qualidade do render (world/render.ts). */
  quality: Quality;
  /** O jogador escolheu o preset; senão ele segue o aparelho (detectQuality). */
  qualityPicked: boolean;
  shadows: boolean;
  viewDistance: number;
  fpsLimit: "30" | "60" | "120" | "0";
  uiScale: number;
  vMaster: number;
  vMusic: number;
  vSfx: number;
  vAmbient: number;
  /** Som desligado (os volumes ficam guardados). */
  muted: boolean;
  touchMode: "auto" | "on" | "off";
  joySize: number;
  joyOpacity: number;
  lefty: boolean;
  showNames: boolean;
  showDamage: boolean;
  /** Tremor leve da câmera nos seus críticos e nos golpes do chefe em você (desligável por conforto). */
  cameraShake: boolean;
  showHelm: boolean;
  /** Missão rastreada no minimapa, no mapa e na seta da borda da tela. */
  questMarkers: boolean;
  /** "Ir" das missões: seguir andando mesmo levando dano (desligado: para ao ser atacado). */
  goKeepIfHit: boolean;
  language: "pt-BR" | "en" | "es";
  autoRadius: number;
  autoSkills: boolean;
  autoPotion: number;
  autoReturn: boolean;
  autoLoot: boolean;
  keys: Record<string, string>;
}

export const DEFAULT_KEYS: Record<string, string> = {
  inventory: "KeyI",
  skills: "KeyK",
  party: "KeyP",
  clan: "KeyN",
  social: "KeyO",
  glory: "KeyH",
  character: "KeyC",
  map: "KeyM",
  settings: "Escape",
  auto: "KeyF",
  target: "Tab",
  potion: "KeyQ",
};

export type Quality = "baixa" | "media" | "alta";
export const QUALITIES: readonly Quality[] = ["baixa", "media", "alta"];

/**
 * Preset da primeira vez, pelo aparelho: computador → alta; celular e tablet → média só
 * nos aparelhos com bastante memória e núcleos (o navegador informa), senão baixa. A meta
 * é 30 FPS estáveis num celular médio; o jogador pode subir em Configurações → Gráficos.
 */
export function detectQuality(): Quality {
  const nav = navigator as Navigator & { deviceMemory?: number; userAgentData?: { mobile?: boolean } };
  const mobile =
    nav.userAgentData?.mobile === true ||
    /Android|iPhone|iPad|iPod|Mobile/i.test(nav.userAgent) ||
    (coarse.matches && nav.maxTouchPoints > 0);
  if (!mobile) return "alta";
  const mem = nav.deviceMemory ?? 4;
  const cores = nav.hardwareConcurrency ?? 4;
  return mem >= 6 && cores >= 8 ? "media" : "baixa";
}

const DEFAULT_SETTINGS: Settings = {
  quality: detectQuality(),
  qualityPicked: false,
  shadows: true,
  viewDistance: 70,
  fpsLimit: "60",
  uiScale: 100,
  vMaster: 80,
  vMusic: 55,
  vSfx: 75,
  vAmbient: 60,
  muted: false,
  touchMode: "auto",
  joySize: 100,
  joyOpacity: 80,
  lefty: false,
  showNames: true,
  showDamage: true,
  cameraShake: true,
  showHelm: true,
  questMarkers: true,
  goKeepIfHit: false,
  language: "pt-BR",
  autoRadius: 25,
  autoSkills: true,
  autoPotion: 45,
  autoReturn: true,
  autoLoot: true,
  keys: { ...DEFAULT_KEYS },
};

const SETTINGS_KEY = "erevia.settings";

function loadSettings(): Settings {
  try {
    const raw = localStorage.getItem(SETTINGS_KEY);
    if (raw) {
      const s = JSON.parse(raw) as Partial<Settings>;
      const v: Settings = { ...DEFAULT_SETTINGS, ...s, keys: { ...DEFAULT_KEYS, ...(s.keys ?? {}) } };
      // Antes do render novo a qualidade não fazia nada (e existia "ultra"): quem nunca
      // escolheu segue o aparelho; um preset que não existe mais vira o mais alto.
      if (!v.qualityPicked) v.quality = DEFAULT_SETTINGS.quality;
      else if (!QUALITIES.includes(v.quality)) v.quality = "alta";
      return v;
    }
  } catch {
    // armazenamento bloqueado ou corrompido: usa o padrão
  }
  return structuredClone(DEFAULT_SETTINGS);
}

class SettingsStore {
  private value = loadSettings();
  private subs = new Set<(s: Settings) => void>();

  get(): Settings {
    return this.value;
  }

  set<K extends keyof Settings>(key: K, v: Settings[K]): void {
    this.value = { ...this.value, [key]: v };
    try {
      localStorage.setItem(SETTINGS_KEY, JSON.stringify(this.value));
    } catch {
      // ignora
    }
    for (const fn of this.subs) fn(this.value);
  }

  reset(): void {
    this.value = structuredClone(DEFAULT_SETTINGS);
    try {
      localStorage.removeItem(SETTINGS_KEY);
    } catch {
      // ignora
    }
    for (const fn of this.subs) fn(this.value);
  }

  on(fn: (s: Settings) => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }
}

export const settings = new SettingsStore();

/** Nome legível de uma tecla ("KeyI" → "I"). */
export function keyLabel(code: string): string {
  if (code.startsWith("Key")) return code.slice(3);
  if (code.startsWith("Digit")) return code.slice(5);
  const names: Record<string, string> = {
    Escape: "Esc",
    Tab: "Tab",
    Space: "Espaço",
    ShiftLeft: "Shift",
    ControlLeft: "Ctrl",
    Enter: "Enter",
    Backquote: "`",
  };
  return names[code] ?? code;
}

// ---------------------------------------------------------------------------------------
// Abas
// ---------------------------------------------------------------------------------------

export interface TabDef {
  id: string;
  label: string;
  icon?: string;
  count?: string;
}

/** Barra de abas. Devolve o elemento e uma função para trocar a aba ativa. */
export function tabs(defs: TabDef[], active: string, onChange: (id: string) => void, cls = "tabs") {
  const root = el("div", cls);
  root.setAttribute("role", "tablist");
  const btns = new Map<string, HTMLButtonElement>();
  const select = (id: string, notify = true) => {
    for (const [k, b] of btns) b.setAttribute("aria-selected", String(k === id));
    btns.get(id)?.scrollIntoView({ block: "nearest", inline: "nearest" });
    if (notify) onChange(id);
  };
  for (const d of defs) {
    const b = el("button", "tab");
    b.type = "button";
    b.setAttribute("role", "tab");
    if (d.icon) b.append(art(d.icon, "tab-icon"));
    b.append(d.label);
    if (d.count) b.append(el("span", "count", d.count));
    b.onclick = () => select(d.id);
    btns.set(d.id, b);
    root.append(b);
  }
  select(active, false);
  return { root, select };
}

// ---------------------------------------------------------------------------------------
// Controles de formulário
// ---------------------------------------------------------------------------------------

export function slider(
  value: number,
  min: number,
  max: number,
  step: number,
  onInput: (v: number) => void,
  format: (v: number) => string = (v) => String(v),
): HTMLDivElement {
  const wrap = el("div", "range");
  const input = el("input");
  input.type = "range";
  input.min = String(min);
  input.max = String(max);
  input.step = String(step);
  input.value = String(value);
  const out = el("output", null, format(value));
  const paint = () => input.style.setProperty("--p", `${((Number(input.value) - min) / (max - min)) * 100}%`);
  paint();
  input.oninput = () => {
    paint();
    out.textContent = format(Number(input.value));
    onInput(Number(input.value));
  };
  wrap.append(input, out);
  return wrap;
}

export function toggle(checked: boolean, onChange: (v: boolean) => void, label = ""): HTMLLabelElement {
  const wrap = el("label", "toggle");
  const input = el("input");
  input.type = "checkbox";
  input.checked = checked;
  if (label) input.setAttribute("aria-label", label);
  input.onchange = () => onChange(input.checked);
  wrap.append(input, el("span"));
  return wrap;
}

export function segmented<T extends string>(
  options: { id: T; label: string }[],
  value: T,
  onChange: (v: T) => void,
): HTMLDivElement {
  const root = el("div", "seg");
  const btns: HTMLButtonElement[] = [];
  for (const o of options) {
    const b = button(o.label, "");
    b.setAttribute("aria-pressed", String(o.id === value));
    b.onclick = () => {
      for (const x of btns) x.setAttribute("aria-pressed", String(x === b));
      onChange(o.id);
    };
    btns.push(b);
    root.append(b);
  }
  return root;
}

export function select<T extends string>(
  options: { id: T; label: string }[],
  value: T,
  onChange: (v: T) => void,
): HTMLSelectElement {
  const s = el("select", "input");
  for (const o of options) {
    const opt = el("option", null, o.label);
    opt.value = o.id;
    opt.selected = o.id === value;
    s.append(opt);
  }
  s.onchange = () => onChange(s.value as T);
  return s;
}

export function bar(kind: "hp" | "mp" | "xp", cls = ""): { root: HTMLDivElement; set(v: number, max: number, label?: string): void } {
  const root = el("div", `pbar ${kind} ${cls}`);
  const fill = el("i");
  const text = el("b");
  root.append(fill, text);
  return {
    root,
    set(v, max, label) {
      root.style.setProperty("--v", `${(clamp(max > 0 ? v / max : 0, 0, 1) * 100).toFixed(2)}%`);
      text.textContent = label ?? `${fmt.format(Math.round(v))} / ${fmt.format(Math.round(max))}`;
    },
  };
}

// ---------------------------------------------------------------------------------------
// Dica (tooltip): hover no PC, toque longo no celular
// ---------------------------------------------------------------------------------------

let tipEl: HTMLDivElement | null = null;

export function showTip(content: Node | string, x: number, y: number): void {
  hideTip();
  tipEl = el("div", "tip frame");
  if (typeof content === "string") tipEl.innerHTML = content;
  else tipEl.append(content);
  document.body.append(tipEl);
  placeNear(tipEl, x, y);
}

export function hideTip(): void {
  tipEl?.remove();
  tipEl = null;
}

function placeNear(e: HTMLElement, x: number, y: number): void {
  const r = e.getBoundingClientRect();
  let left = x + 16;
  let top = y + 16;
  if (left + r.width > innerWidth - 8) left = x - r.width - 16;
  if (top + r.height > innerHeight - 8) top = innerHeight - r.height - 8;
  e.style.left = `${Math.max(8, left)}px`;
  e.style.top = `${Math.max(8, top)}px`;
}

/**
 * Liga uma dica a um elemento. No toque, a dica abre com toque longo (e some ao soltar);
 * `onTap` recebe o toque curto.
 */
export function tooltip(target: HTMLElement, content: () => Node | string | null): void {
  target.addEventListener("pointerenter", (ev) => {
    if (ev.pointerType !== "mouse") return;
    const c = content();
    if (c) showTip(c, ev.clientX, ev.clientY);
  });
  target.addEventListener("pointermove", (ev) => {
    if (ev.pointerType === "mouse" && tipEl) placeNear(tipEl, ev.clientX, ev.clientY);
  });
  target.addEventListener("pointerleave", hideTip);
  target.addEventListener("pointerdown", (ev) => {
    if (ev.pointerType === "mouse") return;
    const t = window.setTimeout(() => {
      const c = content();
      if (c) showTip(c, ev.clientX, ev.clientY - 60);
    }, 420);
    const end = () => {
      clearTimeout(t);
      window.setTimeout(hideTip, 1200);
      target.removeEventListener("pointerup", end);
      target.removeEventListener("pointercancel", end);
    };
    target.addEventListener("pointerup", end);
    target.addEventListener("pointercancel", end);
  });
}

// ---------------------------------------------------------------------------------------
// Menu de contexto
// ---------------------------------------------------------------------------------------

export interface MenuItem {
  label: string;
  danger?: boolean;
  disabled?: boolean;
  run(): void;
}

export function contextMenu(items: MenuItem[], x: number, y: number): void {
  closeMenus();
  const m = el("div", "ctx frame");
  m.dataset.menu = "1";
  for (const it of items) {
    const b = button(it.label, it.danger ? "danger" : "");
    b.disabled = !!it.disabled;
    b.onclick = () => {
      closeMenus();
      it.run();
    };
    m.append(b);
  }
  document.body.append(m);
  placeNear(m, x - 16, y - 16);
  setTimeout(() => document.addEventListener("pointerdown", onOutside, true));
}

function onOutside(ev: PointerEvent): void {
  if ((ev.target as HTMLElement).closest("[data-menu]")) return;
  closeMenus();
}

export function closeMenus(): void {
  document.querySelectorAll("[data-menu]").forEach((m) => m.remove());
  document.removeEventListener("pointerdown", onOutside, true);
}

// ---------------------------------------------------------------------------------------
// Painel inferior (detalhes no celular)
// ---------------------------------------------------------------------------------------

export function openSheet(content: Node): () => void {
  closeSheet();
  const scrim = el("div", "sheet-scrim");
  scrim.dataset.sheet = "1";
  const sheet = el("div", "sheet frame scroll", el("div", "grab"), content);
  sheet.dataset.sheet = "1";
  scrim.onclick = closeSheet;
  document.body.append(scrim, sheet);
  return closeSheet;
}

export function closeSheet(): void {
  document.querySelectorAll("[data-sheet]").forEach((m) => m.remove());
}

// ---------------------------------------------------------------------------------------
// Avisos
// ---------------------------------------------------------------------------------------

let toastBox: HTMLDivElement | null = null;

export function toast(text: string, kind: "" | "warn" | "bad" = ""): void {
  if (!toastBox || !toastBox.isConnected) {
    toastBox = el("div", "toasts");
    document.body.append(toastBox);
  }
  const t = el("div", `toast ${kind}`, text);
  toastBox.append(t);
  while (toastBox.children.length > 3) toastBox.firstElementChild?.remove();
  setTimeout(() => t.remove(), 2900);
}

// ---------------------------------------------------------------------------------------
// Janelas do jogo
// ---------------------------------------------------------------------------------------

export interface WindowDef {
  id: string;
  title: string;
  /** Tecla (código) mostrada no título. */
  hotkey?: string;
  width: number;
  height?: number;
  build(close: () => void): HTMLElement | Promise<HTMLElement>;
  onClose?(): void;
}

interface OpenWin {
  def: WindowDef;
  root: HTMLDivElement;
}

/** Abre/fecha janelas por cima do jogo. Arrastáveis no PC; tela cheia no celular. */
export class WindowManager {
  readonly layer = el("div", "gwin-layer");
  private scrim = el("div", "gwin-scrim");
  private open = new Map<string, OpenWin>();
  private z = 1;
  private cascade = 0;
  onChange: (() => void) | null = null;

  constructor(private defs: Map<string, WindowDef>) {
    this.layer.append(this.scrim);
    this.scrim.onclick = () => this.closeTop();
    document.body.append(this.layer);
  }

  isOpen(id: string): boolean {
    return this.open.has(id);
  }

  anyOpen(): boolean {
    return this.open.size > 0;
  }

  toggle(id: string): void {
    if (this.open.has(id)) this.close(id);
    else void this.show(id);
  }

  /** Largura que a janela terá nesta tela (para posicionar outras ao lado). */
  widthOf(id: string): number {
    const def = this.defs.get(id);
    return def ? Math.min(def.width, innerWidth - 24) : 0;
  }

  /** Abre a janela (ou traz para a frente). `at` fixa a posição em vez de centralizar. */
  async show(id: string, at?: { left: number; top: number }): Promise<void> {
    const existing = this.open.get(id);
    if (existing) {
      this.focus(existing.root);
      return;
    }
    const def = this.defs.get(id);
    if (!def) return;
    // No celular só cabe uma janela por vez.
    if (isCompact()) for (const k of [...this.open.keys()]) this.close(k);

    const root = el("div", "gwin frame");
    root.dataset.win = id;
    root.setAttribute("role", "dialog");
    root.setAttribute("aria-label", def.title);
    const head = el("div", "gwin-head", el("h2", "title", def.title));
    if (def.hotkey) head.append(el("span", "key", keyLabel(def.hotkey)));
    const x = el("button", "x-btn", "✕");
    x.type = "button";
    x.setAttribute("aria-label", "Fechar");
    x.onclick = () => this.close(id);
    head.append(x);
    const body = el("div", "gwin-body scroll");
    root.append(head, body);

    const w = Math.min(def.width, innerWidth - 24);
    const h = def.height ? Math.min(def.height, innerHeight - 24) : undefined;
    root.style.width = `${w}px`;
    if (h) root.style.height = `${h}px`;
    if (at) {
      root.style.left = `${clamp(at.left, 12, Math.max(12, innerWidth - w - 12))}px`;
      root.style.top = `${clamp(at.top, 12, Math.max(12, innerHeight - (h ?? 560) - 12))}px`;
    } else {
      const off = (this.cascade++ % 4) * 28;
      root.style.left = `${Math.max(12, (innerWidth - w) / 2 + off - 42)}px`;
      root.style.top = `${Math.max(12, (innerHeight - (h ?? 560)) / 2 + off - 42)}px`;
    }
    root.addEventListener("pointerdown", () => this.focus(root));
    this.drag(head, root);

    this.layer.append(root);
    this.open.set(id, { def, root });
    this.focus(root);
    this.sync();
    body.append(await def.build(() => this.close(id)));
  }

  close(id: string): void {
    const w = this.open.get(id);
    if (!w) return;
    // Avisa o conteúdo (para liberar 3D, ouvintes etc.).
    w.root.querySelector(".gwin-body > *")?.dispatchEvent(new Event("win-close"));
    w.root.remove();
    this.open.delete(id);
    hideTip();
    closeMenus();
    closeSheet();
    w.def.onClose?.();
    this.sync();
  }

  closeTop(): boolean {
    let top: [string, OpenWin] | null = null;
    for (const e of this.open) if (!top || Number(e[1].root.style.zIndex) > Number(top[1].root.style.zIndex)) top = e;
    if (!top) return false;
    this.close(top[0]);
    return true;
  }

  closeAll(): void {
    for (const k of [...this.open.keys()]) this.close(k);
  }

  private sync(): void {
    this.scrim.style.display = isCompact() && this.open.size ? "block" : "";
    this.onChange?.();
  }

  private focus(root: HTMLElement): void {
    root.style.zIndex = String(++this.z);
  }

  private drag(handle: HTMLElement, root: HTMLElement): void {
    handle.addEventListener("pointerdown", (ev) => {
      if (isCompact() || (ev.target as HTMLElement).closest("button")) return;
      const sx = ev.clientX - root.offsetLeft;
      const sy = ev.clientY - root.offsetTop;
      handle.setPointerCapture(ev.pointerId);
      handle.style.cursor = "grabbing";
      const move = (e: PointerEvent) => {
        const x = clamp(e.clientX - sx, -root.offsetWidth + 80, innerWidth - 80);
        const y = clamp(e.clientY - sy, 0, innerHeight - 48);
        root.style.left = `${x}px`;
        root.style.top = `${y}px`;
      };
      const up = () => {
        handle.style.cursor = "";
        handle.removeEventListener("pointermove", move);
        handle.removeEventListener("pointerup", up);
      };
      handle.addEventListener("pointermove", move);
      handle.addEventListener("pointerup", up);
    });
  }
}
