/**
 * HUD do jogo (protótipo): mundo 3D real com simulação local, barras, alvo, atalhos,
 * minimapa, chat, missões, grupo, saque, auto-caça e controles de toque.
 * As janelas (inventário, skills, atributos, configurações, mapa) abrem por cima.
 */

import "../styles/hud.css";
import "../styles/windows.css";
import { Effect } from "../../gen/erevia/v1/game_pb";
import { classInfo } from "../../classes";
import { effectIcon } from "../../ui/icons";
import { drawDot, drawPlayerArrow, drawBossMarker, MARKER_COLORS } from "../../ui/minimap";
import { hourLabel, isNight } from "../../world/daynight";
import { renderMapCanvas } from "../../world/map-painter";
import { ui, uniqueIds } from "../art";
import { ITEMS, itemIcon } from "../data/items";
import { CHAT_SEED, game, HOTBAR_SIZE, PARTY, QUESTS, xpNeeded } from "../data/mock";
import { nodeIcon, type SkillNode } from "../data/skilltree";
import {
  art,
  bar,
  button,
  el,
  fmt,
  isCompact,
  isTouch,
  keyLabel,
  openSheet,
  closeSheet,
  settings,
  toast,
  tooltip,
  WindowManager,
  type WindowDef,
} from "../kit";
import type { Ctx, Screen } from "../main";
import { Sim } from "../hud/sim";
import { createJoystick } from "../hud/touch-controls";

const HOTKEYS = ["Digit1", "Digit2", "Digit3", "Digit4", "Digit5", "Digit6", "Digit7", "Digit8", "Digit9", "Digit0"];

const WIN_ROUTE: Record<string, string> = { config: "settings", inventario: "inventory", skills: "skills", atributos: "character" };

const EFFECT_OF: Record<string, Effect> = {
  def: Effect.BUFF_DEF,
  atk: Effect.BUFF_ATK,
  speed: Effect.BUFF_SPEED,
  stun: Effect.STUN,
  slow: Effect.SLOW,
};

/** Ícone, nome e dica de um atalho (skill da árvore ou item). */
export function slotInfo(id: string | null): { icon: string; name: string; node?: SkillNode; item?: string } | null {
  if (!id) return null;
  if (id.startsWith("item:")) {
    const it = id.slice(5);
    const d = ITEMS[it];
    return d ? { icon: itemIcon(it), name: d.name, item: it } : null;
  }
  const n = game.node(id);
  return n ? { icon: nodeIcon(n), name: n.name, node: n } : null;
}

export function skillTip(n: SkillNode, rank: number): string {
  const meta = n.kind === "active"
    ? `<div class="meta">${n.mp ? `<span>MP <b>${n.mp}</b></span>` : ""}${n.cd ? `<span>Recarga <b>${n.cd} s</b></span>` : ""}${n.range ? `<span>Alcance <b>${n.range} m</b></span>` : ""}</div>`
    : `<div class="meta"><span><b>Passiva</b></span></div>`;
  return `<h4>${n.name}</h4><div class="sub">${n.kind === "active" ? "Ativa" : "Passiva"} · Rank ${rank}/${n.maxRank}</div>${meta}<div>${n.effect(Math.max(1, rank))}</div>`;
}

export async function mount(root: HTMLElement, ctx: Ctx): Promise<Screen> {
  root.classList.add("hud-screen");
  const canvas = el("canvas", "game-canvas");
  canvas.tabIndex = 0;
  const hud = el("div", "hud");
  const loading = el("div", "hud-loading", el("div", "logo-word small-logo", "EREVIA"), el("div", "spinner"), el("p", "muted"));
  root.append(canvas, hud, loading);

  const cleanups: (() => void)[] = [];
  let raf = 0;

  // ---------- Simulação ----------
  const sim = new Sim({
    onLog: (text, kind) => {
      if (kind === "loot") lootFeed(text);
      else if (kind === "xp") lootFeed(text, "xp");
      chat.add("system", undefined, text);
    },
    onDeath: () => showDeath(),
    onLevel: () => {
      banner(`Nível ${game.hero.level}!`, "Você ganhou 3 pontos de atributo e 1 ponto de skill.");
      chat.add("system", undefined, `Você subiu para o nível ${game.hero.level}!`);
    },
  });

  // ---------- Janelas ----------
  const defs = new Map<string, WindowDef>();
  const reg = (d: WindowDef) => defs.set(d.id, d);
  reg({ id: "inventory", title: "Inventário", hotkey: settings.get().keys.inventory, width: 860, height: 640, build: async () => (await import("./windows/inventory")).build() });
  reg({ id: "skills", title: "Árvore de skills", hotkey: settings.get().keys.skills, width: 980, height: 700, build: async () => (await import("./windows/skilltree")).build() });
  reg({ id: "character", title: "Personagem", hotkey: settings.get().keys.character, width: 940, height: 660, build: async () => (await import("./windows/attributes")).build() });
  reg({
    id: "settings",
    title: "Configurações",
    hotkey: settings.get().keys.settings,
    width: 820,
    height: 640,
    build: async (close) => (await import("./windows/settings")).build({ close, exit: () => ctx.go("personagem"), logout: () => ctx.go("login") }),
  });
  reg({ id: "map", title: "Mapa do mundo", hotkey: settings.get().keys.map, width: 720, height: 720, build: () => buildMap() });
  const wm = new WindowManager(defs);
  wm.onChange = () => {
    menuBtns.forEach((b, id) => b.classList.toggle("on", wm.isOpen(id)));
    // Volta a rota para #hud quando nenhuma janela está aberta (para os links funcionarem de novo).
    if (!wm.anyOpen() && location.hash !== "#hud" && WIN_ROUTE[location.hash.slice(1)]) history.replaceState(null, "", "#hud");
  };
  cleanups.push(() => {
    wm.closeAll();
    wm.layer.remove();
  });

  // =================================================================================
  // Elementos do HUD
  // =================================================================================

  // ---------- Jogador ----------
  const me = el("div", "hud-me frame glass");
  const crest = el("div", "me-crest", el("span", "me-initial"), el("span", "me-level"));
  const hpBar = bar("hp");
  const mpBar = bar("mp");
  const meName = el("div", "me-name");
  const myEffects = el("div", "effects");
  me.append(crest, el("div", "me-body", meName, hpBar.root, mpBar.root), myEffects);
  tooltip(crest, () => "Abrir personagem (C)");
  crest.onclick = () => wm.toggle("character");

  // ---------- Alvo ----------
  const target = el("div", "hud-target frame glass");
  const tName = el("div", "t-name");
  const tHp = bar("hp");
  const tEffects = el("div", "effects");
  const tClose = button("✕", "t-close");
  tClose.setAttribute("aria-label", "Limpar alvo");
  tClose.onclick = () => sim.setTarget(0);
  target.append(el("div", "t-head", tName, tClose), tHp.root, tEffects);

  // ---------- Minimapa ----------
  const mini = el("div", "hud-minimap");
  const miniCanvas = el("canvas", "mm-canvas");
  const miniCtx = miniCanvas.getContext("2d")!;
  const clock = el("span", "mm-clock");
  const coords = el("span", "mm-coords");
  const zoneName = el("div", "mm-zone", "Campos ao Norte");
  let miniRadius = 40;
  const zoomIn = button("+", "mm-btn", () => (miniRadius = Math.max(20, miniRadius - 10)));
  const zoomOut = button("−", "mm-btn", () => (miniRadius = Math.min(90, miniRadius + 10)));
  const mapBtn = button("", "mm-btn", () => wm.toggle("map"));
  mapBtn.innerHTML = ui("map", 14);
  mapBtn.title = "Mapa (M)";
  mini.append(zoneName, el("div", "mm-frame", miniCanvas, el("span", "mm-n", "N"), clock), el("div", "mm-row", coords, zoomOut, zoomIn, mapBtn));
  let mapCanvas: HTMLCanvasElement | null = null;

  // ---------- Menu (canto superior direito) ----------
  const menu = el("div", "hud-menu");
  const menuBtns = new Map<string, HTMLButtonElement>();
  const MENU = [
    { id: "character", icon: "user", label: "Personagem", key: "character" },
    { id: "inventory", icon: "bag", label: "Inventário", key: "inventory" },
    { id: "skills", icon: "book", label: "Skills", key: "skills" },
    { id: "map", icon: "map", label: "Mapa", key: "map" },
    { id: "settings", icon: "gear", label: "Configurações", key: "settings" },
  ];
  for (const m of MENU) {
    const b = el("button", "menu-btn");
    b.type = "button";
    b.innerHTML = `${ui(m.icon, 20)}<kbd>${keyLabel(settings.get().keys[m.key])}</kbd>`;
    b.setAttribute("aria-label", m.label);
    tooltip(b, () => `<b>${m.label}</b> <span class="muted">(${keyLabel(settings.get().keys[m.key])})</span>`);
    b.onclick = () => wm.toggle(m.id);
    menuBtns.set(m.id, b);
    menu.append(b);
  }
  const hamburger = el("button", "menu-burger");
  hamburger.type = "button";
  hamburger.innerHTML = ui("menu", 22);
  hamburger.setAttribute("aria-label", "Menu");
  hamburger.onclick = () => openMobileMenu();
  const stats = el("div", "hud-stats");
  const fpsEl = el("span");
  const pingEl = el("span");
  stats.append(fpsEl, pingEl);
  const topRight = el("div", "hud-tr", menu, hamburger, stats);

  // ---------- Missões e grupo ----------
  const side = el("div", "hud-side");
  const quests = el("section", "quests");
  const qHead = button("", "side-head");
  qHead.innerHTML = `${ui("scroll", 16)}<span>Missões</span><i>▾</i>`;
  qHead.onclick = () => quests.classList.toggle("collapsed");
  quests.append(qHead);
  const killStep = el("span", "q-count");
  for (const [i, q] of QUESTS.entries()) {
    const s = q.steps[0];
    const count = i === 0 ? killStep : el("span", "q-count", `${s.n}/${s.of}`);
    if (i === 0) killStep.textContent = `${s.n}/${s.of}`;
    quests.append(el("div", "quest", el("b", null, q.title), el("div", "q-step", el("span", null, s.text), count), el("small", null, q.zone)));
  }
  const party = el("section", "party");
  const pHead = button("", "side-head");
  pHead.innerHTML = `${ui("users", 16)}<span>Grupo</span><i>▾</i>`;
  pHead.onclick = () => party.classList.toggle("collapsed");
  party.append(pHead);
  for (const p of PARTY) {
    const info = classInfo(p.cls);
    const h = bar("hp", "thin");
    h.set(p.hp, 1, "");
    const m = bar("mp", "thin");
    m.set(p.mp, 1, "");
    const row = el("div", "member", el("span", "m-crest", info.label[0]), el("div", "m-body", el("div", "m-name", p.name, el("small", null, ` Nv ${p.level}`)), h.root, m.root));
    row.style.setProperty("--cc", info.color);
    party.append(row);
  }
  side.append(quests, party);

  // ---------- Chat ----------
  const chat = createChat();

  // ---------- Barra de atalhos ----------
  const hotbar = el("div", "hotbar");
  const slots: { root: HTMLButtonElement; sweep: HTMLElement; time: HTMLElement; qty: HTMLElement; id: string | null }[] = [];
  for (let i = 0; i < HOTBAR_SIZE; i++) {
    const b = el("button", "hb-slot slot");
    b.type = "button";
    b.dataset.hotbar = String(i);
    const sweep = el("span", "cd-sweep");
    const time = el("span", "cd-time");
    const qty = el("span", "qty");
    const key = el("kbd", "hb-key", keyLabel(HOTKEYS[i]));
    b.append(sweep, time, qty, key);
    b.onclick = () => useSlot(i);
    b.oncontextmenu = (e) => {
      e.preventDefault();
      game.hero.hotbar[i] = null;
      game.emit("hotbar");
    };
    tooltip(b, () => {
      const s = slotInfo(game.hero.hotbar[i]);
      if (!s) return `<span class="muted">Arraste uma skill ou poção para cá</span>`;
      if (s.node) return skillTip(s.node, game.hero.ranks[s.node.id] ?? 0) + `<div class="cmp muted">Botão direito: remover</div>`;
      const d = ITEMS[s.item!];
      return `<h4 class="rar-${d.rarity}">${d.name}</h4><div>${d.use?.text ?? ""}</div><div class="cmp muted">Você tem ${game.countItem(s.item!)}</div>`;
    });
    slots.push({ root: b, sweep, time, qty, id: null });
    hotbar.append(b);
  }
  const autoBtn = el("button", "auto-btn");
  autoBtn.type = "button";
  autoBtn.innerHTML = `${ui("auto", 20)}<span>AUTO</span><kbd>${keyLabel(settings.get().keys.auto)}</kbd>`;
  autoBtn.setAttribute("aria-pressed", "false");
  autoBtn.onclick = () => toggleAuto();
  tooltip(autoBtn, () => `<h4>Auto-caça</h4>Procura monstros num raio de ${settings.get().autoRadius} m, ataca, usa as skills da barra e toma poção abaixo de ${settings.get().autoPotion}% de HP.<div class="cmp muted">Ajuste em Configurações → Jogo</div>`);
  const xpBar = bar("xp", "hud-xp");
  const bottom = el("div", "hud-bottom", me, el("div", "hb-row", hotbar, autoBtn));

  // ---------- Status da auto-caça ----------
  const autoPill = el("div", "auto-pill");
  autoPill.hidden = true;
  const autoText = el("span");
  const autoStop = button("Parar", "btn btn-sm btn-ghost", () => toggleAuto(false));
  autoPill.append(el("span", "auto-dot"), autoText, autoStop);

  // ---------- Saque ----------
  const loot = el("div", "loot-feed");
  function lootFeed(text: string, kind: "loot" | "xp" = "loot") {
    const line = el("div", `loot-line ${kind}`, text);
    loot.prepend(line);
    while (loot.children.length > 5) loot.lastElementChild?.remove();
    setTimeout(() => line.classList.add("out"), 4200);
    setTimeout(() => line.remove(), 5000);
  }

  // ---------- Controles de toque ----------
  const joy = createJoystick((v) => (sim.stick = v));
  cleanups.push(joy.dispose);
  const touch = el("div", "touch-ui");
  const cluster = el("div", "action-cluster");
  const attackBtn = el("button", "act-attack");
  attackBtn.type = "button";
  attackBtn.innerHTML = ui("sword", 34);
  attackBtn.setAttribute("aria-label", "Atacar");
  attackBtn.onclick = () => sim.attackTarget();
  const tSkills: { root: HTMLButtonElement; sweep: HTMLElement; time: HTMLElement; idx: number }[] = [];
  let page = 0;
  for (let k = 0; k < 4; k++) {
    const b = el("button", `act-skill s${k}`);
    b.type = "button";
    const sweep = el("span", "cd-sweep");
    const time = el("span", "cd-time");
    b.append(sweep, time);
    b.onclick = () => useSlot(page * 4 + k);
    tooltip(b, () => {
      const s = slotInfo(game.hero.hotbar[page * 4 + k]);
      return s?.node ? skillTip(s.node, game.hero.ranks[s.node.id] ?? 0) : null;
    });
    tSkills.push({ root: b, sweep, time, idx: k });
    cluster.append(b);
  }
  const pageBtn = button("", "act-small act-page", () => {
    page = 1 - page;
    pageBtn.textContent = page ? "2/2" : "1/2";
  });
  pageBtn.textContent = "1/2";
  pageBtn.setAttribute("aria-label", "Trocar página de skills");
  const targetBtn = el("button", "act-small act-target");
  targetBtn.type = "button";
  targetBtn.innerHTML = ui("target", 20);
  targetBtn.setAttribute("aria-label", "Trocar alvo");
  targetBtn.onclick = () => sim.cycleTarget();
  const potionBtn = el("button", "act-small act-potion");
  potionBtn.type = "button";
  const potionQty = el("span", "qty");
  const potionSweep = el("span", "cd-sweep");
  potionBtn.innerHTML = itemIcon("pt_hp");
  potionBtn.append(potionSweep, potionQty);
  potionBtn.setAttribute("aria-label", "Usar poção de vida");
  potionBtn.onclick = () => {
    const err = sim.usePotion("hp");
    if (err) toast(err, "warn");
  };
  const autoTouch = el("button", "act-auto");
  autoTouch.type = "button";
  autoTouch.innerHTML = `${ui("auto", 22)}<span>AUTO</span>`;
  autoTouch.setAttribute("aria-pressed", "false");
  autoTouch.onclick = () => toggleAuto();
  cluster.append(attackBtn, pageBtn, targetBtn, potionBtn, autoTouch);
  touch.append(joy.root, cluster);

  // ---------- Morte / avisos ----------
  const death = el("div", "death-screen");
  death.hidden = true;
  const bannerEl = el("div", "level-banner");
  bannerEl.hidden = true;

  hud.append(mini, target, topRight, side, chat.root, bottom, xpBar.root, autoPill, loot, touch, bannerEl, death);

  // =================================================================================
  // Comportamento
  // =================================================================================

  function useSlot(i: number) {
    const id = game.hero.hotbar[i];
    const s = slotInfo(id);
    if (!s) return;
    if (s.item) {
      const d = ITEMS[s.item];
      if (d.use?.hp || d.use?.mp) {
        const err = sim.usePotion(d.use.hp ? "hp" : "mp");
        if (err) toast(err, "warn");
      } else toast(`${d.name}: ${d.use?.text ?? "não pode ser usado aqui"}`);
      return;
    }
    const n = s.node!;
    if ((game.hero.ranks[n.id] ?? 0) < 1) {
      toast("Você ainda não aprendeu essa skill.", "warn");
      return;
    }
    const err = sim.useSkill(n);
    if (err) toast(err, "warn");
  }

  function toggleAuto(on = !sim.auto) {
    sim.setAuto(on);
    autoBtn.setAttribute("aria-pressed", String(on));
    autoTouch.setAttribute("aria-pressed", String(on));
    autoPill.hidden = !on;
    toast(on ? "Auto-caça ligada" : "Auto-caça desligada");
  }

  function showDeath() {
    if (sim.auto) toggleAuto(false);
    death.hidden = false;
    death.replaceChildren(
      el(
        "div",
        "death-box",
        el("h2", "death-title", "Você morreu"),
        el("p", "muted", "Você perdeu 4% da experiência deste nível."),
        el(
          "div",
          "death-actions",
          button("Renascer na cidade", "btn btn-gold", () => {
            death.hidden = true;
            sim.respawn();
          }),
          button("Pergaminho de ressurreição (0)", "btn", () => toast("Você não tem pergaminhos.", "warn")),
        ),
      ),
    );
  }

  function banner(title: string, sub: string) {
    bannerEl.hidden = false;
    bannerEl.replaceChildren(el("div", "lb-title", title), el("div", "lb-sub", sub));
    bannerEl.classList.remove("show");
    void bannerEl.offsetWidth;
    bannerEl.classList.add("show");
    setTimeout(() => (bannerEl.hidden = true), 3600);
  }

  function openMobileMenu() {
    const grid = el("div", "mobile-menu");
    for (const m of MENU) {
      const b = el("button", "mm-item");
      b.type = "button";
      b.innerHTML = `${ui(m.icon, 26)}<span>${m.label}</span>`;
      b.onclick = () => {
        closeSheet();
        wm.toggle(m.id);
      };
      grid.append(b);
    }
    const extra = [
      { icon: "chat", label: "Chat", run: () => chat.open() },
      { icon: "scroll", label: "Missões", run: () => quests.classList.toggle("collapsed") },
      { icon: "trophy", label: "Morte (prévia)", run: () => sim.killMe() },
      { icon: "logout", label: "Trocar personagem", run: () => ctx.go("personagem") },
    ];
    for (const x of extra) {
      const b = el("button", "mm-item");
      b.type = "button";
      b.innerHTML = `${ui(x.icon, 26)}<span>${x.label}</span>`;
      b.onclick = () => {
        closeSheet();
        x.run();
      };
      grid.append(b);
    }
    openSheet(el("div", null, el("h3", "title sheet-title", "Menu"), grid));
  }

  function buildMap(): HTMLElement {
    const wrap = el("div", "world-map");
    const c = el("canvas", "wm-canvas");
    const src = mapCanvas ?? (mapCanvas = renderMapCanvas(sim.data));
    c.width = src.width;
    c.height = src.height;
    const cx = c.getContext("2d")!;
    const draw = () => {
      if (!c.isConnected) return;
      cx.drawImage(src, 0, 0);
      const k = src.width / sim.data.size;
      const p = (x: number, z: number) => [(x + sim.data.size / 2) * k, (sim.data.size / 2 - z) * k] as const;
      for (const u of sim.units) {
        if (u.dead) continue;
        const [x, y] = p(u.x, u.z);
        if (u.boss) drawBossMarker(cx, x, y, 9);
        else drawDot(cx, x, y, 5, MARKER_COLORS.monster);
      }
      const [x, y] = p(sim.me.x, sim.me.z);
      cx.save();
      cx.translate(x, y);
      cx.rotate(sim.me.heading);
      cx.scale(1.6, 1.6);
      drawPlayerArrow(cx, 14);
      cx.restore();
      requestAnimationFrame(draw);
    };
    draw();
    wrap.append(c, el("p", "muted small wm-legend", "● Monstros   ◆ Chefe   ▲ Você — role para aproximar (no jogo real)"));
    return wrap;
  }

  // ---------- Chat ----------
  function createChat() {
    const rootEl = el("div", "hud-chat");
    type Ch = "general" | "party" | "whisper" | "system";
    let filter: "all" | Ch = "all";
    const log = el("ol", "chat-log scroll");
    const tabsRow = el("div", "chat-tabs");
    const TABS: { id: "all" | Ch; label: string }[] = [
      { id: "all", label: "Todos" },
      { id: "general", label: "Geral" },
      { id: "party", label: "Grupo" },
      { id: "whisper", label: "Sussurro" },
      { id: "system", label: "Sistema" },
    ];
    for (const t of TABS) {
      const b = button(t.label, "chat-tab");
      b.setAttribute("aria-pressed", String(t.id === filter));
      b.onclick = () => {
        filter = t.id;
        tabsRow.querySelectorAll(".chat-tab").forEach((x) => x.setAttribute("aria-pressed", String(x === b)));
        log.querySelectorAll<HTMLElement>("li").forEach((li) => (li.hidden = filter !== "all" && li.dataset.ch !== filter));
        log.scrollTop = log.scrollHeight;
      };
      tabsRow.append(b);
    }
    const closeBtn = button("✕", "chat-close", () => rootEl.classList.remove("open"));
    closeBtn.setAttribute("aria-label", "Fechar chat");
    tabsRow.append(closeBtn);
    const form = el("form", "chat-form");
    const input = el("input", "chat-input");
    input.placeholder = "Enter para falar · /g grupo · /w Nome";
    input.maxLength = 200;
    input.setAttribute("aria-label", "Mensagem");
    form.append(input);
    form.onsubmit = (e) => {
      e.preventDefault();
      const text = input.value.trim();
      if (!text) return input.blur();
      if (text.startsWith("/g ")) add("party", game.hero.name, text.slice(3));
      else if (text.startsWith("/w ")) {
        const [, to, ...rest] = text.split(" ");
        add("whisper", `para ${to}`, rest.join(" "));
      } else add("general", game.hero.name, text);
      input.value = "";
      input.blur();
    };
    const bubble = el("button", "chat-bubble");
    bubble.type = "button";
    bubble.setAttribute("aria-label", "Abrir chat");
    bubble.onclick = () => open();
    rootEl.append(tabsRow, log, form, bubble);

    function add(ch: Ch, from: string | undefined, text: string) {
      const li = el("li", `ch-${ch}`);
      li.dataset.ch = ch;
      if (from) li.append(el("b", null, `${ch === "party" ? "[Grupo] " : ch === "whisper" ? "[Sussurro] " : ""}${from}: `));
      li.append(text);
      li.hidden = filter !== "all" && filter !== ch;
      log.append(li);
      while (log.children.length > 80) log.firstElementChild?.remove();
      log.scrollTop = log.scrollHeight;
      bubble.replaceChildren(art(ui("chat", 16), "cb-icon"), el("span", `cb-text ch-${ch}`, from ? `${from}: ${text}` : text));
    }
    function open() {
      rootEl.classList.add("open");
      if (!isTouch()) input.focus();
    }
    for (const m of CHAT_SEED) add(m.ch, m.from, m.text);
    return { root: rootEl, add, open, input };
  }

  const FAKE_CHAT = [
    { ch: "general" as const, from: "Orin", text: "LF grupo pro Capitão Ossudo" },
    { ch: "general" as const, from: "Isadora", text: "comprando Fragmento de Mithril" },
    { ch: "party" as const, from: "Brann", text: "vou puxar os da esquerda" },
    { ch: "general" as const, from: "Theron", text: "a noite tá dando muito XP mesmo" },
    { ch: "party" as const, from: "Lyra", text: "buff em 5s" },
  ];
  let fakeIdx = 0;
  const chatTimer = window.setInterval(() => {
    const m = FAKE_CHAT[fakeIdx++ % FAKE_CHAT.length];
    chat.add(m.ch, m.from, m.text);
  }, 14000);
  cleanups.push(() => clearInterval(chatTimer));

  // ---------- Entrada: mouse e toque no mundo ----------
  let downAt: { x: number; y: number; t: number; id: number; type: string } | null = null;
  let holding = false;
  const touches = new Map<number, { x: number; y: number }>();
  let pinch = 0;

  canvas.addEventListener("pointerdown", (e) => {
    canvas.focus({ preventScroll: true });
    if (e.pointerType === "mouse") {
      if (e.button !== 0) return;
      const u = sim.pickAt(e.offsetX, e.offsetY);
      if (u) {
        sim.attackTarget(u.id);
        return;
      }
      holding = true;
      const g = sim.groundAt(e.offsetX, e.offsetY);
      if (g) sim.moveTo(g.x, g.z);
      return;
    }
    touches.set(e.pointerId, { x: e.clientX, y: e.clientY });
    canvas.setPointerCapture(e.pointerId);
    if (touches.size === 1) downAt = { x: e.clientX, y: e.clientY, t: performance.now(), id: e.pointerId, type: e.pointerType };
    else {
      downAt = null;
      const [a, b] = [...touches.values()];
      pinch = Math.hypot(a.x - b.x, a.y - b.y);
    }
  });
  canvas.addEventListener("pointermove", (e) => {
    if (e.pointerType === "mouse") {
      if (holding && e.buttons & 1) {
        const g = sim.groundAt(e.offsetX, e.offsetY);
        if (g) sim.moveTo(g.x, g.z);
      }
      return;
    }
    const prev = touches.get(e.pointerId);
    if (!prev) return;
    const cam = sim.world.camera;
    if (touches.size === 1) {
      cam.alpha -= (e.clientX - prev.x) * 0.006;
      cam.beta = Math.max(cam.lowerBetaLimit ?? 0.35, Math.min(cam.upperBetaLimit ?? 1.4, cam.beta - (e.clientY - prev.y) * 0.004));
    }
    touches.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (touches.size === 2) {
      const [a, b] = [...touches.values()];
      const d = Math.hypot(a.x - b.x, a.y - b.y);
      cam.radius = Math.max(cam.lowerRadiusLimit ?? 5, Math.min(cam.upperRadiusLimit ?? 45, cam.radius * (pinch / d)));
      pinch = d;
    }
  });
  const touchEnd = (e: PointerEvent) => {
    if (e.pointerType === "mouse") {
      holding = false;
      return;
    }
    touches.delete(e.pointerId);
    if (downAt && downAt.id === e.pointerId && performance.now() - downAt.t < 350 && Math.hypot(e.clientX - downAt.x, e.clientY - downAt.y) < 12) {
      const r = canvas.getBoundingClientRect();
      const u = sim.pickAt(e.clientX - r.left, e.clientY - r.top);
      if (u) sim.attackTarget(u.id);
      else {
        const g = sim.groundAt(e.clientX - r.left, e.clientY - r.top);
        if (g) sim.moveTo(g.x, g.z);
      }
    }
    downAt = null;
  };
  canvas.addEventListener("pointerup", touchEnd);
  canvas.addEventListener("pointercancel", touchEnd);

  // ---------- Teclado ----------
  const held = new Set<string>();
  const updateKeysStick = () => {
    if (isTouch() && !held.size) return;
    const x = (held.has("KeyD") ? 1 : 0) - (held.has("KeyA") ? 1 : 0);
    const y = (held.has("KeyS") ? 1 : 0) - (held.has("KeyW") ? 1 : 0);
    sim.stick = x || y ? { x, y } : null;
  };
  const onKey = (e: KeyboardEvent) => {
    const typing = (e.target as HTMLElement).closest("input, textarea, select");
    if (typing) {
      if (e.code === "Escape") (e.target as HTMLElement).blur();
      return;
    }
    if (document.querySelector(".rebinding")) return;
    const k = settings.get().keys;
    if (e.code === "Enter") {
      chat.open();
      chat.input.focus();
      e.preventDefault();
      return;
    }
    if (["KeyW", "KeyA", "KeyS", "KeyD"].includes(e.code)) {
      held.add(e.code);
      updateKeysStick();
      return;
    }
    const hk = HOTKEYS.indexOf(e.code);
    if (hk >= 0) return useSlot(hk);
    if (e.code === k.settings) {
      e.preventDefault();
      if (!wm.closeTop()) wm.toggle("settings");
      return;
    }
    const map: Record<string, () => void> = {
      [k.inventory]: () => wm.toggle("inventory"),
      [k.skills]: () => wm.toggle("skills"),
      [k.character]: () => wm.toggle("character"),
      [k.map]: () => wm.toggle("map"),
      [k.auto]: () => toggleAuto(),
      [k.target]: () => sim.cycleTarget(),
      [k.potion]: () => {
        const err = sim.usePotion("hp");
        if (err) toast(err, "warn");
      },
    };
    const fn = map[e.code];
    if (fn) {
      e.preventDefault();
      fn();
    }
  };
  const onKeyUp = (e: KeyboardEvent) => {
    if (held.delete(e.code)) updateKeysStick();
  };
  addEventListener("keydown", onKey);
  addEventListener("keyup", onKeyUp);
  cleanups.push(() => {
    removeEventListener("keydown", onKey);
    removeEventListener("keyup", onKeyUp);
  });

  // =================================================================================
  // Atualização por quadro
  // =================================================================================

  const effectsHtml = (box: HTMLElement, list: Map<string, { until: number; total: number; name: string; debuff: boolean }>) => {
    const now = performance.now();
    const keys = [...list.keys()].join(",");
    if (box.dataset.keys !== keys) {
      box.dataset.keys = keys;
      box.replaceChildren();
      for (const [k, e] of list) {
        const d = el("div", `effect${e.debuff ? " debuff" : ""}`);
        d.innerHTML = uniqueIds(effectIcon(EFFECT_OF[k] ?? Effect.BUFF_ATK));
        d.append(el("span", "cd-sweep"), el("span", "effect-time"));
        d.title = e.name;
        d.dataset.k = k;
        box.append(d);
      }
    }
    box.querySelectorAll<HTMLElement>(".effect").forEach((d) => {
      const e = list.get(d.dataset.k!);
      if (!e) return;
      const left = Math.max(0, e.until - now);
      d.querySelector<HTMLElement>(".cd-sweep")!.style.setProperty("--p", `${(1 - left / e.total) * 360}deg`);
      d.querySelector(".effect-time")!.textContent = String(Math.ceil(left / 1000));
      d.classList.toggle("ending", left < 2000);
    });
  };

  const sweep = (s: HTMLElement, t: HTMLElement, cd: { at: number; total: number } | undefined, now: number) => {
    const left = cd ? cd.at - now : 0;
    if (left > 0) {
      s.style.setProperty("--p", `${(1 - left / cd!.total) * 360}deg`);
      s.hidden = false;
      t.textContent = left > 10000 ? String(Math.ceil(left / 1000)) : (left / 1000).toFixed(1).replace(".", ",");
    } else {
      s.hidden = true;
      t.textContent = "";
    }
  };

  let lastFps = 0;
  const drawHotbar = () => {
    for (const [i, s] of slots.entries()) {
      const id = game.hero.hotbar[i];
      if (s.id !== id) {
        s.id = id;
        s.root.querySelector(".art")?.remove();
        const info = slotInfo(id);
        if (info) s.root.prepend(art(info.icon));
        s.root.classList.toggle("empty", !info);
      }
    }
    for (const t of tSkills) {
      const id = game.hero.hotbar[page * 4 + t.idx];
      if (t.root.dataset.id !== (id ?? "")) {
        t.root.dataset.id = id ?? "";
        t.root.querySelector(".art")?.remove();
        const info = slotInfo(id);
        if (info) t.root.prepend(art(info.icon));
        t.root.classList.toggle("empty", !info);
      }
    }
  };
  const offState = game.onChange((what) => {
    if (what === "hotbar" || what === "hero" || what === "ranks") drawHotbar();
  });
  cleanups.push(offState);

  const frame = () => {
    raf = requestAnimationFrame(frame);
    if (!sim.me) return;
    const now = performance.now();
    const h = game.hero;
    const st = game.stats();
    const info = classInfo(h.cls);

    // Jogador
    me.style.setProperty("--emblem", h.look.emblem);
    crest.querySelector(".me-initial")!.textContent = info.label[0];
    crest.querySelector(".me-level")!.textContent = String(h.level);
    meName.textContent = h.name;
    hpBar.set(sim.me.hp, st.maxHp);
    mpBar.set(sim.mp, st.maxMp);
    me.classList.toggle("low", sim.me.hp / st.maxHp < 0.3);
    effectsHtml(myEffects, sim.me.effects);
    xpBar.set(h.xp, xpNeeded(h.level), `Nv ${h.level} · ${((h.xp / xpNeeded(h.level)) * 100).toFixed(2).replace(".", ",")}%`);

    // Alvo
    const t = sim.units.find((u) => u.id === sim.targetId);
    target.hidden = !t;
    if (t) {
      tName.replaceChildren(el("b", null, t.name), el("span", null, ` Nv ${t.level}`), t.boss ? el("em", null, "CHEFE") : "");
      tHp.set(t.hp, t.maxHp, t.dead ? "Morto" : `${fmt.format(Math.round(t.hp))} / ${fmt.format(t.maxHp)}`);
      target.classList.toggle("boss", t.boss);
      target.classList.toggle("dead", t.dead);
      effectsHtml(tEffects, t.effects);
    }

    // Atalhos (recarga, mana, quantidade)
    for (const [i, s] of slots.entries()) {
      const id = game.hero.hotbar[i];
      const inf = slotInfo(id);
      sweep(s.sweep, s.time, id ? sim.cooldowns.get(id) : undefined, now);
      s.qty.textContent = inf?.item ? String(game.countItem(inf.item)) : "";
      s.root.classList.toggle("nomana", !!inf?.node && sim.mp < (inf.node.mp ?? 0));
      s.root.classList.toggle("locked", !!inf?.node && (h.ranks[inf.node.id] ?? 0) < 1);
      s.root.classList.toggle("queued", !!inf?.node && sim.queued?.id === inf.node.id);
    }
    for (const ts of tSkills) {
      const id = game.hero.hotbar[page * 4 + ts.idx];
      const inf = slotInfo(id);
      sweep(ts.sweep, ts.time, id ? sim.cooldowns.get(id) : undefined, now);
      ts.root.classList.toggle("nomana", !!inf?.node && sim.mp < (inf.node.mp ?? 0));
      ts.root.classList.toggle("queued", !!inf?.node && sim.queued?.id === inf.node.id);
    }
    potionQty.textContent = String(game.countItem("pt_hp"));
    const pcd = sim.cooldowns.get("item:pt_hp");
    potionSweep.hidden = !pcd || pcd.at < now;
    if (pcd) potionSweep.style.setProperty("--p", `${(1 - (pcd.at - now) / pcd.total) * 360}deg`);
    attackBtn.classList.toggle("on", sim.attacking);

    // Auto-caça
    if (sim.auto) {
      const secs = Math.floor((now - sim.autoStats.since) / 1000);
      autoText.textContent = `Auto-caça · ${sim.autoStats.kills} abates · ${Math.floor(secs / 60)}:${String(secs % 60).padStart(2, "0")} · +${fmt.format(sim.autoStats.xp)} XP`;
    }
    killStep.textContent = `${Math.min(12, 7 + sim.autoStats.kills)}/12`;

    // Minimapa (~15 Hz) e estatísticas (~2 Hz)
    drawMinimap();
    if (now - lastFps > 500) {
      lastFps = now;
      fpsEl.textContent = `${Math.round(sim.world.engine.getFps())} FPS`;
      pingEl.textContent = `${sim.ping} ms`;
      pingEl.className = sim.ping < 60 ? "ok" : "warn";
      clock.innerHTML = `${ui(isNight(sim.phase) ? "moon" : "sun", 12)}${hourLabel(sim.phase)}`;
      coords.textContent = `${Math.round(sim.me.x)}, ${Math.round(sim.me.z)}`;
    }
  };

  let lastMini = 0;
  function drawMinimap() {
    const now = performance.now();
    if (now - lastMini < 66) return;
    lastMini = now;
    if (!mapCanvas) mapCanvas = renderMapCanvas(sim.data);
    const size = miniCanvas.clientWidth;
    if (!size) return;
    const dpr = Math.min(2, devicePixelRatio || 1);
    if (miniCanvas.width !== Math.round(size * dpr)) {
      miniCanvas.width = miniCanvas.height = Math.round(size * dpr);
    }
    const c = miniCtx;
    const W = miniCanvas.width;
    const k = mapCanvas.width / sim.data.size; // px do mapa por metro
    const scale = W / (miniRadius * 2); // px do minimapa por metro
    c.setTransform(1, 0, 0, 1, 0, 0);
    c.clearRect(0, 0, W, W);
    c.save();
    c.beginPath();
    c.arc(W / 2, W / 2, W / 2, 0, Math.PI * 2);
    c.clip();
    c.fillStyle = "#1a2214";
    c.fillRect(0, 0, W, W);
    const mx = (sim.me.x + sim.data.size / 2) * k;
    const my = (sim.data.size / 2 - sim.me.z) * k;
    const srcR = miniRadius * k;
    c.drawImage(mapCanvas, mx - srcR, my - srcR, srcR * 2, srcR * 2, 0, 0, W, W);
    for (const u of sim.units) {
      if (u.dead) continue;
      const x = W / 2 + (u.x - sim.me.x) * scale;
      const y = W / 2 - (u.z - sim.me.z) * scale;
      if (u.boss) drawBossMarker(c, x, y, 6 * dpr);
      else drawDot(c, x, y, (u.id === sim.targetId ? 4.5 : 3.2) * dpr, u.id === sim.targetId ? "#ffd36b" : MARKER_COLORS.monster);
    }
    c.translate(W / 2, W / 2);
    c.rotate(sim.me.heading);
    c.scale(dpr, dpr);
    drawPlayerArrow(c, 12);
    c.restore();
    // Cone de visão da câmera
    c.save();
    c.translate(W / 2, W / 2);
    c.rotate(sim.cameraHeading());
    const g = c.createRadialGradient(0, 0, 0, 0, 0, W / 2);
    g.addColorStop(0, "rgba(255,240,200,0.16)");
    g.addColorStop(1, "rgba(255,240,200,0)");
    c.fillStyle = g;
    c.beginPath();
    c.moveTo(0, 0);
    c.arc(0, 0, W / 2, -Math.PI / 2 - 0.6, -Math.PI / 2 + 0.6);
    c.fill();
    c.restore();
  }

  const applyEnv = () => {
    root.classList.toggle("touch", isTouch());
    root.classList.toggle("compact", isCompact());
  };
  applyEnv();
  addEventListener("resize", applyEnv);
  const offSettings = settings.on(applyEnv);
  cleanups.push(() => removeEventListener("resize", applyEnv), offSettings);

  // ---------- Iniciar ----------
  const setProgress = (text: string) => (loading.querySelector("p")!.textContent = text);
  try {
    await sim.init(canvas, setProgress);
  } catch (err) {
    console.error(err);
    setProgress("Não foi possível iniciar o 3D neste navegador.");
  }
  loading.classList.add("done");
  setTimeout(() => loading.remove(), 600);
  drawHotbar();
  raf = requestAnimationFrame(frame);
  if (isTouch()) toast("Joystick à esquerda · toque num monstro para atacar · AUTO caça sozinho");
  else toast("Clique para andar · clique num monstro para atacar · F liga a auto-caça");

  return {
    openWindow(route) {
      const id = WIN_ROUTE[route];
      if (id) void wm.show(id);
    },
    dispose() {
      cancelAnimationFrame(raf);
      for (const c of cleanups) c();
      sim.dispose();
    },
  };
}
