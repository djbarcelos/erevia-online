/**
 * HUD do jogo (HTML sobre o canvas), no visual novo:
 * - topo esquerdo: personagem (vida, mana, nível, buffs);
 * - topo direito: minimapa (clique abre o mapa), zoom, X/Y, FPS e ping, e os eventos
 *   (XP em dobro, invasão da cidade) num selo discreto embaixo;
 * - embaixo: barra de 10 atalhos (skills e itens), AUTO e menu das janelas; XP na borda;
 * - alvo no topo, chat embaixo à esquerda, avisos de saque e de nível, tela de morte;
 * - no toque: joystick e botões de ação (ataque, skills em arco, poção, alvo, AUTO).
 */

import { worldBounds } from "../world/world-data";
import "../styles/hud.css";
import "../styles/windows.css";
import { classInfo } from "../classes";
import {
  Effect,
  EntityKind,
  InvasionPhase,
  PvpState,
  SkillTarget,
  type EventStatus,
  type MasteryState,
  type ItemInfo,
  type PartyState,
  type QuestLog,
  type SelfStats,
  type SkillInfo,
} from "../gen/erevia/v1/game_pb";
import { hourLabel, isNight } from "../world/daynight";
import type { ActiveEffect, EntityInfo } from "../world/entities";
import { ui, uniqueIds } from "./art";
import { effectIcon, effectName, effectText, formatSeconds, isDebuff, skillIcon } from "./icons";
import { buildItemTip, itemIcon, type ItemStore } from "./items";
import { art, bar, button, closeSheet, el, isTouch, keyLabel, openSheet, settings, toast, tooltip } from "./kit";
import { drawBossMarker, drawDot, drawPlayerArrow, MARKER_COLORS, type MapSource } from "./minimap";
import { drawQuestTarget, QuestArrow, questStepTarget, QuestTracker, type Project } from "./quest-nav";
import { levelText, regionAt, type Region } from "./regions";
import { createJoystick } from "./touch-controls";
import { specInfo } from "./specs";
import { masteryTip } from "./windows/mastery";

/** Quanto tempo uma skill de alvo pode ficar "na fila" sem resposta do servidor. */
const QUEUE_TIMEOUT_MS = 8000;

export const HOTBAR_SIZE = 10;
export const HOTBAR_KEYS = ["Digit1", "Digit2", "Digit3", "Digit4", "Digit5", "Digit6", "Digit7", "Digit8", "Digit9", "Digit0"];

/** Um atalho: "skill:<id>" ou "item:<id do item>". */
export type HotbarEntry = string | null;

export type WindowId = "character" | "inventory" | "skills" | "quests" | "party" | "clan" | "social" | "glory" | "map" | "settings";

/** Ações sobre um jogador selecionado (quadro do alvo). */
export type TargetAction = "attack" | "party" | "clan" | "whisper" | "trade" | "friend";

/** O que o quadro do alvo precisa saber de um jogador além do EntityInfo. */
export interface TargetExtras {
  ally: boolean;
  /** Mostrar "Convidar ao grupo" (sem grupo ou líder, e ele fora do grupo). */
  canParty: boolean;
  /** Mostrar "Convidar ao clã" (você convida e ele não tem clã). */
  canClan: boolean;
  /** Mostrar "Amigo +" (ele ainda não está na sua lista). */
  canFriend: boolean;
}

export interface HudCallbacks {
  onSkill(s: SkillInfo): void;
  /** Usar um item da bolsa pelo atalho. */
  onItem(itemId: string): void;
  onRespawn(): void;
  /** A barra mudou (salvar). */
  onHotbarChange(entries: HotbarEntry[]): void;
  onAuto(on: boolean): void;
  openWindow(id: WindowId): void;
  /** Barra de Maestria (nível 30): abre a aba Maestria da janela de skills. */
  openMastery(): void;
  /** Botão de ataque do toque: ataca o alvo ou o inimigo mais próximo. */
  onAttack(): void;
  /** Botões do quadro do alvo quando ele é um jogador. */
  onTargetAction(action: TargetAction): void;
  /** Clique num colega no painel do grupo (seleciona como alvo). */
  onPartyMember(id: number): void;
  /** Botão de cancelar a canalização (teleporte, pergaminho). */
  onCancelChannel(): void;
  onCycleTarget(): void;
  /** Botão "Ir" de uma missão do painel (o personagem anda até o lugar do passo). */
  onQuestGo(questId: string): void;
  /** Joystick (direção relativa à câmera) ou null ao soltar. */
  onStick(v: { x: number; y: number } | null): void;
  me(): { myClass: string; myLevel: number };
  /** Personagem (as missões fixadas ficam guardadas por personagem). */
  characterId: number;
  /** Distância do jogador até o alvo atual (null = sem alvo inimigo). */
  targetDistance(): number | null;
  map: MapSource;
  /** Ponto do chão na tela (seta da missão rastreada na borda). */
  project: Project;
  regions: Region[];
  stats(): { fps: number; ping: number };
  clock(): number;
}

interface SkillState {
  info: SkillInfo;
  readyAt: number;
  span: number;
}

interface SlotEl {
  root: HTMLButtonElement;
  sweep: HTMLElement;
  time: HTMLElement;
  qty: HTMLElement;
  entry: HotbarEntry | undefined;
}

const MENU: { id: WindowId; icon: string; label: string; key: string }[] = [
  { id: "character", icon: "user", label: "Personagem", key: "character" },
  { id: "inventory", icon: "bag", label: "Inventário", key: "inventory" },
  { id: "skills", icon: "book", label: "Skills", key: "skills" },
  { id: "quests", icon: "scroll", label: "Diário de missões", key: "quests" },
  { id: "party", icon: "users", label: "Grupo", key: "party" },
  { id: "clan", icon: "clan", label: "Clã", key: "clan" },
  { id: "social", icon: "mail", label: "Social", key: "social" },
  { id: "glory", icon: "trophy", label: "Glória", key: "glory" },
  { id: "map", icon: "map", label: "Mapa", key: "map" },
  { id: "settings", icon: "gear", label: "Configurações", key: "settings" },
];

function kindLabel(s: SkillInfo): string {
  switch (s.target) {
    case SkillTarget.SELF:
      return "Em si mesmo";
    case SkillTarget.AREA:
      return s.magic ? "Magia em área" : "Golpe em área";
    default:
      return s.magic ? "Magia à distância" : "Golpe corpo a corpo";
  }
}

/** HTML da dica de uma skill (mesmo formato das janelas). */
export function skillTipHtml(s: SkillInfo, keyHint = ""): string {
  const meta = [`<span>Mana <b>${s.mpCost}</b></span>`, `<span>Recarga <b>${formatSeconds(s.cooldownMs)} s</b></span>`];
  if (s.target === SkillTarget.ENEMY) meta.push(`<span>Alcance <b>${formatSeconds(s.range * 1000)} m</b></span>`);
  if (s.target === SkillTarget.AREA) meta.push(`<span>Raio <b>${formatSeconds(s.range * 1000)} m</b></span>`);
  let html = `<h4>${s.name}</h4><div class="sub">${kindLabel(s)}</div><div class="meta">${meta.join("")}</div>`;
  if (s.power > 0) html += `<div>Dano: ${Math.round(s.power * 100)}% do ataque ${s.magic ? "mágico" : "físico"}</div>`;
  if (s.extraCrit > 0) html += `<div>+${Math.round(s.extraCrit * 100)}% de chance de crítico</div>`;
  if (s.description) html += `<div class="desc">${s.description}</div>`;
  if (s.effect !== Effect.UNSPECIFIED) html += `<div class="${isDebuff(s.effect) ? "bad" : "ok"}">${s.effectOnSelf ? "Em você: " : "No alvo: "}${effectText(s.effect, s.effectValue, s.effectMs)}</div>`;
  if (keyHint) html += `<div class="cmp muted">${keyHint}</div>`;
  return html;
}

/** Texto para dentro de HTML de dica (os textos das missões vêm do servidor). */
function escapeHtml(s: string): string {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!);
}

export class Hud {
  // "hud-screen": as regras de toque e de tela pequena em styles/hud.css partem desta classe.
  private root = el("div", "hud hud-screen");
  private skills = new Map<number, SkillState>();
  private hotbar: HotbarEntry[] = Array(HOTBAR_SIZE).fill(null);
  private slotEls: SlotEl[] = [];
  private touchSlots: SlotEl[] = [];
  private touchPage = 0;
  private groupReady = new Map<number, { readyAt: number; span: number }>();
  private mpNow = 0;
  private queued = 0;
  private queuedAt = 0;
  private raf = 0;
  private auto = false;

  // Elementos
  private hpBar = bar("hp");
  private mpBar = bar("mp");
  private xpBar = bar("xp", "hud-xp");
  private self: SelfStats | null = null;
  private mastery: MasteryState | null = null;
  private xpIsMastery = false;
  private meEl = el("div", "hud-me frame glass");
  private meLevel = el("span", "me-level", "1");
  private meName = el("div", "me-name");
  private myEffects = el("div", "effects");
  private target = el("div", "hud-target frame glass");
  private tName = el("div", "t-name");
  private tHp = bar("hp");
  private tEffects = el("div", "effects");
  private tClan = el("div", "t-clan");
  private tActions = el("div", "t-actions");
  private tActionBtns = new Map<TargetAction, HTMLButtonElement>();
  private deathSub = el("p", "muted", "Você perdeu um pouco de experiência.");
  private channelEl = el("div", "channel-bar frame glass");
  private channelLabel = el("span", "ch-label");
  private channelFill = el("i");
  private partyEl = el("section", "party");
  private partyList = el("div", "party-list");
  private questsEl = el("section", "quests");
  private questList = el("div", "quest-list");
  private questBoxes = new Map<string, HTMLElement>();
  /** Missões fixadas e a rastreada (guia o minimapa, o mapa do mundo e a seta da borda). */
  readonly quests: QuestTracker;
  private questArrow = new QuestArrow();
  private lastArrow = 0;
  /** Última posição do jogador vista pelo minimapa (a seta reaproveita). */
  private mePos: { x: number; z: number; at: number } | null = null;
  private partyRows = new Map<number, { root: HTMLElement; name: HTMLElement; hp: ReturnType<typeof bar>; mp: ReturnType<typeof bar> }>();
  private miniCanvas = el("canvas", "mm-canvas");
  private miniCtx = this.miniCanvas.getContext("2d")!;
  private miniRadius = 45;
  private zoneName = el("div", "mm-zone");
  private clock = el("span", "mm-clock");
  private coordX = el("b", "mm-cv", "0");
  private coordY = el("b", "mm-cv", "0");
  private fpsEl = el("span");
  private pingEl = el("span");
  /** Eventos do servidor: só texto, atualizado com o resto do HUD lento (2×/s). */
  private eventsEl = el("div", "hud-events");
  private xp2El = el("span", "ev-xp2", "XP ×2");
  private invEl = el("span", "ev-inv");
  private events: EventStatus | null = null;
  private serverNow: () => number = () => Date.now();
  private menuBtns = new Map<WindowId, HTMLButtonElement>();
  private autoBtn = el("button", "auto-btn");
  private autoTouch = el("button", "act-auto");
  private autoPill = el("div", "auto-pill");
  private autoText = el("span");
  private loot = el("div", "loot-feed");
  private death = el("div", "death-screen");
  private bannerEl = el("div", "level-banner");
  private attackBtn = el("button", "act-attack");
  private potionBtn = el("button", "act-small act-potion");
  private potionQty = el("span", "qty");
  private potionSweep = el("span", "cd-sweep");
  /** Chat (o componente Chat usa os ids de dentro). */
  readonly chatRoot = el("div", "hud-chat");
  private joyDispose: () => void;
  private lastMini = 0;
  private lastSlow = 0;
  private offSettings: () => void;

  constructor(
    skills: SkillInfo[],
    private store: ItemStore,
    private cb: HudCallbacks,
  ) {
    for (const s of skills) this.skills.set(s.id, { info: s, readyAt: 0, span: 1 });
    this.quests = new QuestTracker(cb.characterId);

    // ---------- Personagem (topo esquerdo) ----------
    const crest = el("button", "me-crest", el("span", "me-initial"), this.meLevel);
    crest.type = "button";
    crest.setAttribute("aria-label", "Abrir personagem");
    crest.onclick = () => cb.openWindow("character");
    tooltip(crest, () => `Personagem <span class="muted">(${keyLabel(settings.get().keys.character)})</span>`);
    this.meEl.append(crest, el("div", "me-body", this.meName, this.hpBar.root, this.mpBar.root), this.myEffects);
    // ---------- Grupo (abaixo do personagem; some sem grupo) ----------
    const pHead = el("button", "side-head");
    pHead.type = "button";
    pHead.innerHTML = `${ui("users", 16)}<span>Grupo</span><i>▾</i>`;
    pHead.onclick = () => this.partyEl.classList.toggle("collapsed");
    this.partyEl.append(pHead, this.partyList);
    this.partyEl.hidden = true;
    // ---------- Missões (abaixo do grupo; só as fixadas; some sem missão ativa) ----------
    const qHead = el("button", "side-head");
    qHead.type = "button";
    qHead.innerHTML = `${ui("scroll", 16)}<span>Missões</span><i>▾</i>`;
    qHead.onclick = () => this.questsEl.classList.toggle("collapsed");
    const journal = button("Diário", "q-journal");
    journal.onclick = () => cb.openWindow("quests");
    tooltip(journal, () => `<b>Diário de missões</b> <span class="muted">(${keyLabel(settings.get().keys.quests)})</span><br>Todas as missões; fixe até 3 aqui no painel.`);
    this.questsEl.append(el("div", "q-panel-head", qHead, journal), this.questList);
    this.questsEl.hidden = true;
    const left = el("div", "hud-left", this.meEl, el("div", "hud-side", this.partyEl, this.questsEl));

    // ---------- Alvo ----------
    const tClose = button("✕", "t-close");
    tClose.setAttribute("aria-label", "Fechar quadro do alvo");
    tClose.onclick = () => (this.target.hidden = true);
    // Jogador selecionado: clã e botões (atacar, convidar, sussurrar).
    for (const [action, label, title] of [
      ["attack", "Atacar", "Atacar este jogador (no PC: Ctrl + clique)"],
      ["party", "Grupo +", "Convidar para o seu grupo"],
      ["clan", "Clã +", "Convidar para o seu clã"],
      ["whisper", "Sussurrar", "Mandar uma mensagem particular"],
      ["trade", "Trocar", "Propor uma troca de itens e moedas (chegue perto)"],
      ["friend", "Amigo +", "Adicionar à sua lista de amigos (você vê quando ele está online)"],
    ] as const) {
      const b = button(label, "btn btn-sm t-act", () => cb.onTargetAction(action));
      b.title = title;
      if (action === "attack") b.classList.add("danger");
      this.tActionBtns.set(action, b);
      this.tActions.append(b);
    }
    this.target.append(el("div", "t-head", this.tName, tClose), this.tClan, this.tHp.root, this.tEffects, this.tActions);
    this.target.hidden = true;

    // ---------- Minimapa ----------
    const openBadge = el("span", "mm-open");
    openBadge.innerHTML = ui("map", 14);
    const frame = el("button", "mm-frame", this.miniCanvas, el("span", "mm-n", "N"), this.clock, openBadge);
    frame.type = "button";
    frame.setAttribute("aria-label", "Abrir o mapa do mundo");
    frame.onclick = () => cb.openWindow("map");
    tooltip(frame, () => `<b>Abrir mapa</b> <span class="muted">(${keyLabel(settings.get().keys.map)})</span>`);
    const zoomOut = button("−", "mm-btn", () => (this.miniRadius = Math.min(110, this.miniRadius + 15)));
    const zoomIn = button("+", "mm-btn", () => (this.miniRadius = Math.max(20, this.miniRadius - 15)));
    zoomOut.setAttribute("aria-label", "Afastar o minimapa");
    zoomIn.setAttribute("aria-label", "Aproximar o minimapa");
    const coords = el("span", "mm-coords", el("i", "mm-ax", "X"), this.coordX, el("i", "mm-ax", "Y"), this.coordY);
    coords.title = "Sua posição (X = leste/oeste, Y = norte/sul)";
    const statsEl = el("div", "hud-stats", this.fpsEl, this.pingEl);
    this.eventsEl.append(this.xp2El, this.invEl);
    this.eventsEl.hidden = true;
    tooltip(this.xp2El, () => this.xp2Tip());
    tooltip(this.xpBar.root, () => this.xpTip());
    this.xpBar.root.onclick = () => this.mastery?.active && cb.openMastery();
    tooltip(this.invEl, () => "<h4>Invasão da cidade</h4>Esqueletos entram pelos 4 portões. Enquanto durar, as estradas e a praça deixam de ser zona segura (o PvP continua proibido). A recompensa sai no fim, pelo dano e pelos abates.");
    const mini = el("div", "hud-minimap", this.zoneName, frame, el("div", "mm-zoom", zoomOut, zoomIn), coords, statsEl, this.eventsEl);

    // ---------- Menu ☰ (toque) ----------
    const burger = el("button", "menu-burger");
    burger.type = "button";
    burger.innerHTML = ui("menu", 22);
    burger.setAttribute("aria-label", "Menu");
    burger.onclick = () => this.openMobileMenu();
    const topRight = el("div", "hud-tr", burger);

    // ---------- Barra de atalhos + AUTO + menu ----------
    const hotbarEl = el("div", "hotbar");
    for (let i = 0; i < HOTBAR_SIZE; i++) {
      const s = this.makeSlot(i, "hb-slot slot", keyLabel(HOTBAR_KEYS[i]));
      hotbarEl.append(s.root);
      this.slotEls.push(s);
    }
    this.autoBtn.type = "button";
    this.autoBtn.innerHTML = `${ui("auto", 20)}<span>AUTO</span><kbd>${keyLabel(settings.get().keys.auto)}</kbd>`;
    this.autoBtn.setAttribute("aria-pressed", "false");
    this.autoBtn.onclick = () => this.setAuto(!this.auto, true);
    tooltip(this.autoBtn, () => `<h4>Auto-caça</h4>Procura monstros num raio de ${settings.get().autoRadius} m, ataca, usa as skills da barra e toma poção abaixo de ${settings.get().autoPotion}% de vida.<div class="cmp muted">Ajuste em Configurações → Jogo</div>`);
    const menu = el("div", "hud-menu");
    for (const m of MENU) {
      const b = el("button", "menu-btn");
      b.type = "button";
      b.innerHTML = `${ui(m.icon, 20)}<kbd>${keyLabel(settings.get().keys[m.key])}</kbd>`;
      b.setAttribute("aria-label", m.label);
      tooltip(b, () => `<b>${m.label}</b> <span class="muted">(${keyLabel(settings.get().keys[m.key])})</span>`);
      b.onclick = () => cb.openWindow(m.id);
      this.menuBtns.set(m.id, b);
      menu.append(b);
    }
    const bottom = el("div", "hud-bottom", el("div", "hb-row", hotbarEl, this.autoBtn, menu));

    // ---------- Auto-caça ----------
    this.autoPill.hidden = true;
    this.autoPill.append(el("span", "auto-dot"), this.autoText, button("Parar", "btn btn-sm btn-ghost", () => this.setAuto(false, true)));

    // ---------- Chat (DOM; a lógica fica em ui/chat.ts) ----------
    this.buildChat();

    // ---------- Morte e nível ----------
    this.death.hidden = true;
    this.death.append(
      el(
        "div",
        "death-box",
        el("h2", "death-title", "Você morreu"),
        this.deathSub,
        el("div", "death-actions", button("Voltar à cidade", "btn btn-gold", () => cb.onRespawn())),
      ),
    );
    this.bannerEl.hidden = true;
    const chCancel = button("✕", "ch-cancel", () => cb.onCancelChannel());
    chCancel.title = "Cancelar (Esc)";
    this.channelEl.append(el("div", "ch-row", this.channelLabel, chCancel), el("div", "ch-track", this.channelFill));
    this.channelEl.hidden = true;

    // ---------- Toque ----------
    const joy = createJoystick((v) => cb.onStick(v));
    this.joyDispose = joy.dispose;
    const cluster = el("div", "action-cluster");
    this.attackBtn.type = "button";
    this.attackBtn.innerHTML = ui("sword", 34);
    this.attackBtn.setAttribute("aria-label", "Atacar");
    this.attackBtn.onclick = () => cb.onAttack();
    for (let k = 0; k < 4; k++) {
      const s = this.makeSlot(-1, `act-skill s${k}`, "");
      s.root.onclick = () => this.useSlot(this.touchPage * 4 + k);
      s.root.oncontextmenu = (e) => e.preventDefault();
      this.touchSlots.push(s);
      cluster.append(s.root);
    }
    const pageBtn = button("1/2", "act-small act-page", () => {
      this.touchPage = 1 - this.touchPage;
      pageBtn.textContent = this.touchPage ? "2/2" : "1/2";
      this.renderSlots();
    });
    pageBtn.setAttribute("aria-label", "Trocar página de skills");
    const targetBtn = el("button", "act-small act-target");
    targetBtn.type = "button";
    targetBtn.innerHTML = ui("target", 20);
    targetBtn.setAttribute("aria-label", "Trocar alvo");
    targetBtn.onclick = () => cb.onCycleTarget();
    this.potionBtn.type = "button";
    this.potionBtn.setAttribute("aria-label", "Usar poção de vida");
    this.potionBtn.onclick = () => this.usePotion();
    this.autoTouch.type = "button";
    this.autoTouch.innerHTML = `${ui("auto", 22)}<span>AUTO</span>`;
    this.autoTouch.setAttribute("aria-pressed", "false");
    this.autoTouch.onclick = () => this.setAuto(!this.auto, true);
    cluster.append(this.attackBtn, pageBtn, targetBtn, this.potionBtn, this.autoTouch);
    const touch = el("div", "touch-ui", joy.root, cluster);

    this.root.append(left, mini, this.target, topRight, this.chatRoot, bottom, this.xpBar.root, this.autoPill, this.loot, touch, this.questArrow.el, this.bannerEl, this.channelEl, this.death);
    this.quests.onChange(() => this.markTracked());
    this.quests.onPanel(() => this.renderQuests());
    document.body.append(this.root);

    const applyEnv = () => {
      this.root.classList.toggle("touch", isTouch());
      for (const m of MENU) this.menuBtns.get(m.id)!.querySelector("kbd")!.textContent = keyLabel(settings.get().keys[m.key]);
      this.autoBtn.querySelector("kbd")!.textContent = keyLabel(settings.get().keys.auto);
    };
    applyEnv();
    this.offSettings = settings.on(applyEnv);
    addEventListener("resize", applyEnv);

    this.renderSlots();
    store.onChange(() => this.renderSlots());
    const loop = () => {
      this.tick();
      this.raf = requestAnimationFrame(loop);
    };
    this.raf = requestAnimationFrame(loop);
  }

  /** Raiz do HUD (janelas do jogo entram aqui e somem junto ao desconectar). */
  get element(): HTMLElement {
    return this.root;
  }

  dispose(): void {
    cancelAnimationFrame(this.raf);
    this.joyDispose();
    this.offSettings();
    this.quests.dispose();
    this.root.remove();
  }

  // ------------------------------------------------------------------------------------
  // Atalhos
  // ------------------------------------------------------------------------------------

  setHotbar(entries: HotbarEntry[]): void {
    this.hotbar = Array.from({ length: HOTBAR_SIZE }, (_, i) => entries[i] ?? null);
    this.renderSlots();
  }

  getHotbar(): HotbarEntry[] {
    return [...this.hotbar];
  }

  /** Coloca algo num atalho (tira de onde estava antes). */
  assign(index: number, entry: HotbarEntry): void {
    if (index < 0 || index >= HOTBAR_SIZE) return;
    if (entry) this.hotbar = this.hotbar.map((e) => (e === entry ? null : e));
    this.hotbar[index] = entry;
    this.renderSlots();
    this.cb.onHotbarChange(this.getHotbar());
  }

  /** Usa o atalho i (tecla ou clique). */
  useSlot(i: number): void {
    const e = this.hotbar[i];
    if (!e) return;
    if (e.startsWith("skill:")) {
      const s = this.skills.get(Number(e.slice(6)));
      if (s) this.cb.onSkill(s.info);
    } else if (e.startsWith("item:")) this.cb.onItem(e.slice(5));
  }

  private usePotion(): void {
    const id = this.hotbar.find((e) => e?.startsWith("item:") && this.store.get(e.slice(5))?.healHp)?.slice(5) ?? "potion_hp_s";
    this.cb.onItem(id);
  }

  private makeSlot(index: number, cls: string, key: string): SlotEl {
    const root = el("button", cls);
    root.type = "button";
    if (index >= 0) root.dataset.hotbar = String(index);
    const sweep = el("span", "cd-sweep");
    const time = el("span", "cd-time");
    const qty = el("span", "qty");
    root.append(sweep, time, qty);
    if (key) root.append(el("kbd", "hb-key", key));
    if (index >= 0) {
      root.onclick = () => this.useSlot(index);
      root.oncontextmenu = (e) => {
        e.preventDefault();
        this.assign(index, null);
      };
      tooltip(root, () => this.slotTip(index));
    }
    return { root, sweep, time, qty, entry: undefined };
  }

  private slotTip(i: number): string | HTMLElement {
    const e = this.hotbar[i];
    const key = keyLabel(HOTBAR_KEYS[i]);
    if (!e) return `<span class="muted">Arraste uma skill (K) ou poção (I) para cá · tecla ${key}</span>`;
    if (e.startsWith("skill:")) {
      const s = this.skills.get(Number(e.slice(6)));
      return s ? skillTipHtml(s.info, `Tecla ${key} · botão direito tira da barra`) : "";
    }
    const info = this.store.get(e.slice(5));
    if (!info) return "";
    return buildItemTip(info, { ...this.cb.me(), hint: `Tecla ${key} · você tem ${this.store.countOf(info.id)} · botão direito tira da barra` });
  }

  private iconFor(e: HotbarEntry): string {
    if (!e) return "";
    if (e.startsWith("skill:")) {
      const s = this.skills.get(Number(e.slice(6)));
      return s ? uniqueIds(skillIcon(s.info.fx)) : "";
    }
    const info = this.store.get(e.slice(5));
    return info ? uniqueIds(itemIcon(info)) : "";
  }

  private renderSlots(): void {
    const paint = (s: SlotEl, e: HotbarEntry) => {
      if (s.entry !== e) {
        s.entry = e;
        s.root.querySelector(".art")?.remove();
        const svg = this.iconFor(e);
        if (svg) s.root.prepend(art(svg));
        s.root.classList.toggle("empty", !svg);
      }
      const item = e?.startsWith("item:") ? e.slice(5) : null;
      const n = item ? this.store.countOf(item) : 0;
      s.qty.textContent = item ? String(n) : "";
      s.root.classList.toggle("none-left", !!item && n === 0);
    };
    this.slotEls.forEach((s, i) => paint(s, this.hotbar[i]));
    this.touchSlots.forEach((s, k) => paint(s, this.hotbar[this.touchPage * 4 + k]));
    const potion = this.hotbar.find((e) => e?.startsWith("item:") && this.store.get(e.slice(5))?.healHp)?.slice(5) ?? "potion_hp_s";
    const pInfo = this.store.get(potion);
    this.potionBtn.innerHTML = pInfo ? uniqueIds(itemIcon(pInfo)) : "";
    this.potionQty.textContent = String(this.store.countOf(potion));
    this.potionBtn.append(this.potionSweep, this.potionQty);
  }

  // ------------------------------------------------------------------------------------
  // Estado vindo do servidor
  // ------------------------------------------------------------------------------------

  setStats(s: SelfStats): void {
    this.hpBar.set(s.hp, s.maxHp);
    this.mpBar.set(s.mp, s.maxMp);
    this.mpNow = s.mp;
    this.meLevel.textContent = String(s.level);
    this.meEl.classList.toggle("low", s.maxHp > 0 && s.hp / s.maxHp < 0.3);
    this.self = s;
    this.renderXp();
    const wasDead = !this.death.hidden;
    this.death.hidden = !s.dead;
    if (s.dead) this.deathSub.textContent = s.killedBy ? `Morto por ${s.killedBy}.` : "Você perdeu um pouco de experiência.";
    if (s.dead && !wasDead && this.auto) this.setAuto(false, false);
  }

  /** Maestria (nível 30): a barra de XP do HUD vira a barra de Maestria. */
  setMastery(m: MasteryState): void {
    this.mastery = m;
    this.renderXp();
  }

  /** Barra da borda: XP do nível ou, no 30, a de Maestria (outra cor, com o bônus diário). */
  private renderXp(): void {
    const m = this.mastery;
    const s = this.self;
    const mastery = !!m?.active;
    if (mastery !== this.xpIsMastery) {
      this.xpIsMastery = mastery;
      this.xpBar.root.classList.toggle("mastery", mastery);
    }
    if (m && mastery) {
      const into = Number(m.xp);
      const need = Number(m.need);
      const rested = m.rested > 0n ? ` · bônus diário ${m.restedMult}×` : "";
      if (m.level >= m.maxLevel) this.xpBar.set(1, 1, `Maestria ${m.level} · teto`);
      else this.xpBar.set(into, need, `Maestria ${m.level} · ${((into / need) * 100).toFixed(1).replace(".", ",")}%${rested}`);
      return;
    }
    if (!s) return;
    if (s.xpNextLevel > 0n) {
      const into = Number(s.xp - s.xpLevelStart);
      const need = Number(s.xpNextLevel - s.xpLevelStart);
      this.xpBar.set(into, need, `Nível ${s.level} · ${((into / need) * 100).toFixed(1).replace(".", ",")}%`);
    } else this.xpBar.set(1, 1, `Nível ${s.level} · máximo`);
  }

  private xpTip(): string {
    const m = this.mastery;
    if (m?.active) return masteryTip(m);
    const s = this.self;
    if (!s) return "";
    if (s.xpNextLevel <= 0n) return `<h4>Nível ${s.level}</h4>Nível máximo.`;
    const into = Number(s.xp - s.xpLevelStart);
    const need = Number(s.xpNextLevel - s.xpLevelStart);
    return `<h4>Nível ${s.level}</h4><div class="sub">${into.toLocaleString("pt-BR")} / ${need.toLocaleString("pt-BR")} XP</div>`;
  }

  /** Nome e cor do emblema no quadro do personagem. */
  setIdentity(name: string, initial: string, emblem: string): void {
    this.meName.textContent = name;
    this.meEl.querySelector(".me-initial")!.textContent = initial;
    this.meEl.style.setProperty("--emblem", emblem);
  }

  itemCooldown(group: number, readyInMs: number, totalMs: number): void {
    this.groupReady.set(group, { readyAt: performance.now() + readyInMs, span: Math.max(readyInMs, totalMs) });
  }

  itemReady(group: number): boolean {
    return performance.now() >= (this.groupReady.get(group)?.readyAt ?? 0);
  }

  cooldown(skillId: number, readyInMs: number): void {
    const s = this.skills.get(skillId);
    if (!s) return;
    s.readyAt = performance.now() + readyInMs;
    s.span = Math.max(readyInMs, s.info.cooldownMs);
    if (this.queued === skillId) this.setQueued(0);
  }

  startLocalCooldown(skillId: number): void {
    const s = this.skills.get(skillId);
    if (!s) return;
    s.readyAt = performance.now() + s.info.cooldownMs;
    s.span = s.info.cooldownMs;
  }

  setQueued(skillId: number): void {
    this.queued = skillId;
    this.queuedAt = performance.now();
  }

  isReady(skillId: number): boolean {
    const s = this.skills.get(skillId);
    return !!s && performance.now() >= s.readyAt;
  }

  hasMana(skill: SkillInfo): boolean {
    return this.mpNow >= skill.mpCost;
  }

  skillList(): SkillInfo[] {
    return [...this.skills.values()].map((s) => s.info);
  }

  /** Troca os dados das skills (rank novo, velocidade de conjuração) sem perder as recargas. */
  setSkills(list: SkillInfo[]): void {
    for (const info of list) {
      const st = this.skills.get(info.id);
      if (st) st.info = info;
      else this.skills.set(info.id, { info, readyAt: 0, span: 1 });
    }
    // O ícone não muda; força o redesenho das dicas e estados.
    this.slotEls.forEach((s) => (s.entry = undefined));
    this.touchSlots.forEach((s) => (s.entry = undefined));
    this.renderSlots();
  }

  /** Bolinha com pontos livres nos botões de Personagem e Skills. */
  setBadges(badges: Partial<Record<WindowId, number>>): void {
    for (const [id, n] of Object.entries(badges) as [WindowId, number][]) {
      const b = this.menuBtns.get(id);
      if (!b) continue;
      let dot = b.querySelector<HTMLElement>(".menu-badge");
      if (!n) {
        dot?.remove();
        continue;
      }
      if (!dot) {
        dot = el("span", "menu-badge");
        b.append(dot);
      }
      dot.textContent = String(n);
    }
    const total = Object.values(badges).reduce((a, b) => a + (b ?? 0), 0);
    this.root.querySelector(".menu-burger")?.classList.toggle("has-badge", total > 0);
  }

  /** Barra de canalização (teleporte/pergaminho); totalMs 0 = esconde. */
  setChannel(label: string, totalMs: number): void {
    this.channelEl.hidden = totalMs <= 0;
    if (totalMs <= 0) return;
    this.channelLabel.textContent = label;
    this.channelFill.style.transition = "none";
    this.channelFill.style.width = "0%";
    void this.channelFill.offsetWidth; // reinicia a animação
    this.channelFill.style.transition = `width ${totalMs}ms linear`;
    this.channelFill.style.width = "100%";
  }

  get channeling(): boolean {
    return !this.channelEl.hidden;
  }

  /** Estado dos eventos do servidor (EventStatus). */
  setEvents(s: EventStatus, serverNow: () => number): void {
    this.events = s;
    this.serverNow = serverNow;
    this.renderEvents();
  }

  private renderEvents(): void {
    const s = this.events!;
    const inv = s.invasion !== InvasionPhase.NONE;
    this.xp2El.hidden = !s.xp2;
    this.invEl.hidden = !inv;
    this.eventsEl.hidden = !s.xp2 && !inv;
    if (!inv) return;
    const left = Math.max(0, Math.ceil((Number(s.invasionAtMs) - this.serverNow()) / 1000));
    const clock = `${Math.floor(left / 60)}:${String(left % 60).padStart(2, "0")}`;
    this.invEl.classList.toggle("active", s.invasion === InvasionPhase.ACTIVE);
    this.invEl.textContent =
      s.invasion === InvasionPhase.WARNING ? `Invasão em ${clock}` : `Invasão · onda ${s.wave}/${s.waves} · ${s.alive} ☠ · ${clock}`;
  }

  private xp2Tip(): string {
    const until = Number(this.events?.xp2UntilMs ?? 0);
    const when = until > 0 ? `até ${new Date(until).toLocaleString("pt-BR", { weekday: "long", hour: "2-digit", minute: "2-digit" })}` : "por tempo indeterminado";
    return `<h4>XP em dobro</h4>Monstros dão o dobro de experiência ${when}.`;
  }

  /** QuestLog novo: o rastreador avisa o painel (e o Diário, se aberto). */
  setQuests(log: QuestLog): void {
    this.quests.setLog(log);
  }

  /** Painel de missões: só as fixadas (até 3) e, por alguns segundos, a recém-aceita com o
   *  "Fixar". Redesenhado só quando chega o QuestLog ou muda a fixação (nada por quadro).
   *  Clicar numa missão a torna a rastreada. */
  private renderQuests(): void {
    const all = this.quests.list();
    this.questsEl.hidden = all.length === 0;
    this.questBoxes.clear();
    const shown = this.quests.panel();
    if (!shown.length) {
      this.questList.replaceChildren(el("p", "q-empty", `Nenhuma missão fixada. Abra o Diário (${keyLabel(settings.get().keys.quests)}) para fixar.`));
      return;
    }
    this.questList.replaceChildren(
      ...shown.map((q) => {
        const temp = this.quests.isTemp(q.id);
        // Caixa clicável (rastreia) com o botão "Ir" dentro: div com papel de botão, porque
        // botão dentro de botão não vale.
        const head = el("div", "q-head", el("b", null, q.title));
        const box = el("div", temp ? "quest temp" : "quest", head);
        if (temp) box.style.setProperty("--q-left", `${this.quests.tempLeft(q.id)}ms`);
        box.tabIndex = 0;
        box.setAttribute("role", "button");
        box.onclick = () => this.quests.track(q.id);
        box.onkeydown = (ev) => {
          if (ev.key === "Enter" || ev.key === " ") {
            ev.preventDefault();
            this.quests.track(q.id);
          }
        };
        if (temp) {
          // Recém-aceita com o painel cheio: some sozinha se não for fixada.
          const pin = button("Fixar", "btn btn-sm btn-gold q-go", (ev) => {
            ev.stopPropagation();
            const out = this.quests.pin(q.id);
            if (out) toast(`${out} saiu do painel (continua no Diário).`);
          });
          pin.title = `Fixar no painel (fica no lugar de ${this.quests.wouldEvict() ?? "outra"})`;
          head.append(pin);
        } else if (questStepTarget(q)) {
          const go = button("Ir", "btn btn-sm btn-gold q-go", (ev) => {
            ev.stopPropagation();
            this.quests.track(q.id);
            this.cb.onQuestGo(q.id);
          });
          go.title = "Andar até o objetivo (WASD, clique no chão ou ser atacado param)";
          head.append(go);
        }
        this.questBoxes.set(q.id, box);
        for (const s of q.steps) {
          const row = el("div", "q-step", el("span", null, s.text), el("span", "q-count", `${s.count}/${s.need}`));
          if (s.count >= s.need) row.classList.add("done");
          box.append(row);
        }
        box.append(el("small", null, temp ? "Nova missão: fixe para manter no painel." : q.text));
        tooltip(box, () => {
          const hint = !settings.get().questMarkers
            ? ""
            : box.classList.contains("tracked")
              ? `<br><span class="muted">Rastreada no minimapa e no mapa.</span>`
              : `<br><span class="muted">Clique para rastrear.</span>`;
          return `<b>${escapeHtml(q.title)}</b><br>${escapeHtml(q.text)}<br><span class="muted">Recompensa: ${escapeHtml(q.reward)}</span>${hint}`;
        });
        return box;
      }),
    );
    this.markTracked();
  }

  /** Destaque da missão rastreada no painel (só com os marcadores ligados). */
  private markTracked(): void {
    const id = settings.get().questMarkers ? this.quests.tracked()?.id : undefined;
    for (const [qid, box] of this.questBoxes) box.classList.toggle("tracked", qid === id);
  }

  /** Painel do grupo: um por colega (você não aparece), com vida, mana e o líder. */
  setParty(state: PartyState, localId: number, near: (id: number) => boolean): void {
    const others = state.members.filter((m) => m.id !== localId);
    this.partyEl.hidden = others.length === 0;
    const seen = new Set<number>();
    for (const m of others) {
      seen.add(m.id);
      let row = this.partyRows.get(m.id);
      if (!row) {
        const hp = bar("hp", "thin");
        const mp = bar("mp", "thin");
        const name = el("div", "m-name");
        const root = el("button", "member", el("span", "m-crest", classInfo(m.class).label[0]), el("div", "m-body", name, hp.root, mp.root));
        (root as HTMLButtonElement).type = "button";
        root.style.setProperty("--cc", classInfo(m.class).color);
        root.onclick = () => this.cb.onPartyMember(m.id);
        row = { root, name, hp, mp };
        this.partyRows.set(m.id, row);
      }
      const lead = m.id === state.leaderId ? el("span", "m-lead", "♛") : "";
      row.name.replaceChildren(lead, document.createTextNode(m.name), el("small", null, ` Nv ${m.level}`));
      row.hp.set(m.hp, m.maxHp, "");
      row.mp.set(m.mp, m.maxMp, "");
      row.root.classList.toggle("dead", m.dead);
      row.root.classList.toggle("far", !near(m.id));
      row.root.title = m.dead ? `${m.name} (morto)` : `Selecionar ${m.name}`;
    }
    for (const [id, row] of this.partyRows) {
      if (!seen.has(id)) {
        row.root.remove();
        this.partyRows.delete(id);
      }
    }
    // Ordem: líder primeiro, depois a ordem do servidor.
    const order = others.slice().sort((a, b) => Number(b.id === state.leaderId) - Number(a.id === state.leaderId));
    this.partyList.replaceChildren(...order.map((m) => this.partyRows.get(m.id)!.root));
  }

  setTarget(info: EntityInfo | undefined, extras?: TargetExtras): void {
    if (!info) {
      this.target.hidden = true;
      return;
    }
    this.target.hidden = false;
    const npc = info.kind === EntityKind.NPC;
    const pc = info.kind === EntityKind.PLAYER;
    this.target.classList.toggle("npc", npc);
    this.target.classList.toggle("pc", pc);
    this.target.classList.toggle("flagged", pc && info.pvpState === PvpState.FLAGGED);
    this.target.classList.toggle("chaotic", pc && info.pvpState === PvpState.CHAOTIC);
    this.target.classList.toggle("ally", pc && !!extras?.ally);
    this.tClan.textContent = pc && info.clanName ? `‹${info.clanName}›` : "";
    this.tClan.hidden = !this.tClan.textContent;
    // Os botões são os mesmos (recriá-los a cada snapshot perderia cliques): só mostra/esconde.
    this.tActions.hidden = !pc || !extras;
    if (pc && extras) {
      this.tActionBtns.get("attack")!.hidden = extras.ally || info.dead;
      this.tActionBtns.get("party")!.hidden = !extras.canParty;
      this.tActionBtns.get("clan")!.hidden = !extras.canClan;
      this.tActionBtns.get("friend")!.hidden = !extras.canFriend;
    }
    this.target.classList.toggle("dead", info.dead);
    this.target.classList.toggle("boss", !!info.boss);
    // Jogador com especialização: o nome do caminho, na cor dele, depois do nível.
    const spec = pc ? specInfo(info.spec) : undefined;
    const specEl = spec ? el("span", "t-spec", ` · ${spec.name}`) : "";
    if (spec && specEl) specEl.style.color = spec.color;
    this.tName.replaceChildren(el("b", null, info.name), el("span", null, npc ? ` · ${info.title}` : ` Nv ${info.level}`), specEl, info.boss ? el("em", null, "CHEFE") : "");
    this.tHp.root.hidden = npc;
    this.tHp.set(info.hp, info.maxHp, info.dead ? "Morto" : undefined);
  }

  setEffects(mine: ActiveEffect[], target: ActiveEffect[]): void {
    this.effectsInto(this.myEffects, mine);
    this.effectsInto(this.tEffects, target);
  }

  private effectsInto(box: HTMLElement, list: ActiveEffect[]): void {
    const now = performance.now();
    const keys = list.map((e) => e.effect).join(",");
    if (box.dataset.keys !== keys) {
      box.dataset.keys = keys;
      box.replaceChildren();
      for (const ef of list) {
        const d = el("div", `effect${isDebuff(ef.effect) ? " debuff" : ""}`);
        d.innerHTML = uniqueIds(effectIcon(ef.effect));
        d.append(el("span", "cd-sweep"), el("span", "effect-time"));
        d.title = `${effectName(ef.effect)} — ${effectText(ef.effect, ef.value, ef.durationMs)}`;
        box.append(d);
      }
    }
    list.forEach((ef, i) => {
      const d = box.children[i] as HTMLElement | undefined;
      if (!d) return;
      const left = Math.max(0, ef.until - now);
      d.querySelector<HTMLElement>(".cd-sweep")!.style.setProperty("--p", `${(1 - (ef.durationMs > 0 ? left / ef.durationMs : 0)) * 360}deg`);
      d.querySelector(".effect-time")!.textContent = String(Math.ceil(left / 1000));
      d.classList.toggle("ending", left < 2000);
    });
  }

  // ------------------------------------------------------------------------------------
  // Avisos, auto-caça, menu
  // ------------------------------------------------------------------------------------

  lootLine(text: string, kind: "loot" | "xp" = "loot"): void {
    const line = el("div", `loot-line ${kind}`, text);
    this.loot.prepend(line);
    while (this.loot.children.length > 5) this.loot.lastElementChild?.remove();
    setTimeout(() => line.classList.add("out"), 4200);
    setTimeout(() => line.remove(), 5000);
  }

  /** Prende um elemento extra na camada do HUD (barra do raid boss, anúncios). */
  mount(...els: HTMLElement[]): void {
    this.root.append(...els);
  }

  levelBanner(level: number, title = `Nível ${level}!`, sub = "Você ficou mais forte."): void {
    this.bannerEl.hidden = false;
    this.bannerEl.replaceChildren(el("div", "lb-title", title), el("div", "lb-sub", sub));
    this.bannerEl.classList.remove("show");
    void this.bannerEl.offsetWidth;
    this.bannerEl.classList.add("show");
    setTimeout(() => (this.bannerEl.hidden = true), 3600);
  }

  /** Ponto de Maestria ganho (o mesmo banner do nível). */
  masteryBanner(level: number): void {
    this.levelBanner(level, `Maestria ${level}!`, "Um ponto novo para a árvore (aba Maestria das skills).");
  }

  /** Liga/desliga o visual da auto-caça; notify = avisar o jogo (clique do jogador). */
  setAuto(on: boolean, notify: boolean): void {
    if (this.auto === on) return;
    this.auto = on;
    this.autoBtn.setAttribute("aria-pressed", String(on));
    this.autoTouch.setAttribute("aria-pressed", String(on));
    this.autoPill.hidden = !on;
    if (notify) {
      this.cb.onAuto(on);
      toast(on ? "Auto-caça ligada" : "Auto-caça desligada");
    }
  }

  setAutoStatus(text: string): void {
    this.autoText.textContent = text;
  }

  get autoOn(): boolean {
    return this.auto;
  }

  setWindowOpen(id: string, open: boolean): void {
    this.menuBtns.get(id as WindowId)?.classList.toggle("on", open);
  }

  setAttacking(on: boolean): void {
    this.attackBtn.classList.toggle("on", on);
  }

  private openMobileMenu(): void {
    const grid = el("div", "mobile-menu");
    for (const m of MENU) {
      const b = el("button", "mm-item");
      b.type = "button";
      b.innerHTML = `${ui(m.icon, 26)}<span>${m.label}</span>`;
      b.onclick = () => {
        closeSheet();
        this.cb.openWindow(m.id);
      };
      grid.append(b);
    }
    const chatBtn = el("button", "mm-item");
    chatBtn.type = "button";
    chatBtn.innerHTML = `${ui("chat", 26)}<span>Chat</span>`;
    chatBtn.onclick = () => {
      closeSheet();
      this.openChat();
    };
    grid.append(chatBtn);
    openSheet(el("div", null, el("h3", "title sheet-title", "Menu"), grid));
  }

  // ------------------------------------------------------------------------------------
  // Chat
  // ------------------------------------------------------------------------------------

  private buildChat(): void {
    type Filter = "all" | "general" | "party" | "clan" | "whisper" | "system";
    const log = el("ol", "chat-log scroll");
    log.id = "chat-log";
    log.setAttribute("aria-live", "polite");
    log.setAttribute("aria-label", "Mensagens do chat");
    const tabsRow = el("div", "chat-tabs");
    const TABS: { id: Filter; label: string }[] = [
      { id: "all", label: "Todos" },
      { id: "general", label: "Geral" },
      { id: "party", label: "Grupo" },
      { id: "clan", label: "Clã" },
      { id: "whisper", label: "Sussurro" },
      { id: "system", label: "Sistema" },
    ];
    let filter: Filter = "all";
    for (const t of TABS) {
      const b = button(t.label, "chat-tab");
      b.setAttribute("aria-pressed", String(t.id === filter));
      b.onclick = () => {
        filter = t.id;
        tabsRow.querySelectorAll(".chat-tab").forEach((x) => x.setAttribute("aria-pressed", String(x === b)));
        log.dataset.filter = filter;
        log.scrollTop = log.scrollHeight;
      };
      tabsRow.append(b);
    }
    const close = button("✕", "chat-close", () => this.chatRoot.classList.remove("open"));
    close.setAttribute("aria-label", "Fechar chat");
    tabsRow.append(close);
    const form = el("form", "chat-form");
    form.id = "chat-form";
    form.autocomplete = "off";
    const input = el("input", "chat-input");
    input.id = "chat-input";
    input.maxLength = 220;
    input.placeholder = "Enter para falar · /g grupo · /c clã · /w Nome";
    input.setAttribute("aria-label", "Mensagem do chat");
    form.append(input);
    const bubble = el("button", "chat-bubble");
    bubble.type = "button";
    bubble.setAttribute("aria-label", "Abrir chat");
    bubble.onclick = () => this.openChat();
    this.chatRoot.append(tabsRow, log, form, bubble);
    // Última linha do chat vira a "bolha" no celular.
    new MutationObserver(() => {
      const last = log.lastElementChild as HTMLElement | null;
      if (!last) return;
      bubble.replaceChildren(art(ui("chat", 16), "cb-icon"), el("span", `cb-text ${last.className}`, last.textContent ?? ""));
    }).observe(log, { childList: true });
  }

  openChat(): void {
    this.chatRoot.classList.add("open");
    if (!isTouch()) this.chatRoot.querySelector<HTMLInputElement>("#chat-input")?.focus();
  }

  // ------------------------------------------------------------------------------------
  // Quadro a quadro
  // ------------------------------------------------------------------------------------

  private tick(): void {
    const now = performance.now();
    if (this.queued && now - this.queuedAt > QUEUE_TIMEOUT_MS) this.setQueued(0);
    const dist = this.cb.targetDistance();

    const sweep = (s: SlotEl, e: HotbarEntry) => {
      let left = 0;
      let span = 1;
      let noMana = false;
      let far = false;
      let queued = false;
      if (e?.startsWith("skill:")) {
        const st = this.skills.get(Number(e.slice(6)));
        if (st) {
          left = st.readyAt - now;
          span = st.span;
          noMana = this.mpNow < st.info.mpCost;
          far = st.info.target === SkillTarget.ENEMY && dist !== null && dist > st.info.range;
          queued = this.queued === st.info.id;
        }
      } else if (e?.startsWith("item:")) {
        const g = this.store.get(e.slice(5))?.cooldownGroup;
        const c = g ? this.groupReady.get(g) : undefined;
        if (c) {
          left = c.readyAt - now;
          span = c.span;
        }
      }
      if (left > 0) {
        s.sweep.hidden = false;
        s.sweep.style.setProperty("--p", `${(1 - Math.min(1, left / Math.max(1, span))) * 360}deg`);
        s.time.textContent = left > 10000 ? String(Math.ceil(left / 1000)) : (left / 1000).toFixed(1).replace(".", ",");
        s.root.classList.add("cooling");
      } else {
        s.sweep.hidden = true;
        s.time.textContent = "";
        if (s.root.classList.contains("cooling")) {
          s.root.classList.remove("cooling", "ready-flash");
          void s.root.offsetWidth;
          s.root.classList.add("ready-flash");
        }
      }
      s.root.classList.toggle("nomana", noMana);
      s.root.classList.toggle("far", far);
      s.root.classList.toggle("queued", queued);
    };
    this.slotEls.forEach((s, i) => sweep(s, this.hotbar[i]));
    this.touchSlots.forEach((s, k) => sweep(s, this.hotbar[this.touchPage * 4 + k]));
    const pg = this.groupReady.get(1);
    const pLeft = pg ? pg.readyAt - now : 0;
    this.potionSweep.hidden = pLeft <= 0;
    if (pg && pLeft > 0) this.potionSweep.style.setProperty("--p", `${(1 - pLeft / pg.span) * 360}deg`);

    if (now - this.lastMini > 66) {
      this.lastMini = now;
      this.drawMinimap();
    }
    if (now - this.lastArrow > 200) {
      this.lastArrow = now;
      this.updateQuestArrow(now);
    }
    if (now - this.lastSlow > 500) {
      this.lastSlow = now;
      const st = this.cb.stats();
      this.fpsEl.textContent = `${Math.round(st.fps)} FPS`;
      this.pingEl.textContent = `${st.ping} ms`;
      this.pingEl.className = st.ping < 80 ? "ok" : "warn";
      const phase = this.cb.clock();
      this.clock.innerHTML = `${ui(isNight(phase) ? "moon" : "sun", 12)}${hourLabel(phase)}`;
      if (this.events) this.renderEvents();
    }
  }

  /** Seta na borda da tela para a missão rastreada (até 5×/s). */
  private updateQuestArrow(now: number): void {
    const t = this.quests.target();
    let me: { x: number; z: number } | null = null;
    if (t) {
      if (this.mePos && now - this.mePos.at < 400) me = this.mePos;
      else
        this.cb.map.forEachMarker((m) => {
          if (m.local) me = { x: m.x, z: m.z };
        });
    }
    this.questArrow.update(t, me, this.cb.project);
  }

  private drawMinimap(): void {
    const src = this.cb.map;
    const size = this.miniCanvas.clientWidth;
    if (!size) return;
    let me: { x: number; z: number; heading: number } | null = null;
    src.forEachMarker((m) => {
      if (m.local) me = m;
    });
    if (!me) return;
    const pos = me as { x: number; z: number; heading: number };
    this.mePos = { x: pos.x, z: pos.z, at: performance.now() };
    this.coordX.textContent = String(Math.round(pos.x));
    this.coordY.textContent = String(Math.round(pos.z));
    this.zoneName.textContent = regionAt(this.cb.regions, pos.x, pos.z).name;

    const dpr = Math.min(2, devicePixelRatio || 1);
    const W = Math.round(size * dpr);
    if (this.miniCanvas.width !== W) this.miniCanvas.width = this.miniCanvas.height = W;
    const c = this.miniCtx;
    const world = src.world;
    const b = worldBounds(world);
    const k = src.mapCanvas.width / (b.maxX - b.minX);
    const scale = W / (this.miniRadius * 2);
    c.setTransform(1, 0, 0, 1, 0, 0);
    c.clearRect(0, 0, W, W);
    c.save();
    c.beginPath();
    c.arc(W / 2, W / 2, W / 2, 0, Math.PI * 2);
    c.clip();
    c.fillStyle = "#1a2214";
    c.fillRect(0, 0, W, W);
    const mx = (pos.x - b.minX) * k;
    const my = (b.maxZ - pos.z) * k;
    const sr = this.miniRadius * k;
    c.drawImage(src.mapCanvas, mx - sr, my - sr, sr * 2, sr * 2, 0, 0, W, W);
    src.forEachMarker((m) => {
      if (m.local) return;
      const x = W / 2 + (m.x - pos.x) * scale;
      const y = W / 2 - (m.z - pos.z) * scale;
      if (m.boss) drawBossMarker(c, x, y, 6 * dpr);
      else if (m.kind === EntityKind.MONSTER) drawDot(c, x, y, (m.target ? 4.5 : 3.2) * dpr, m.target ? "#ffd36b" : MARKER_COLORS.monster);
      else if (m.kind === EntityKind.NPC) drawDot(c, x, y, 3.5 * dpr, MARKER_COLORS.npc);
      else if (m.party) drawDot(c, x, y, 4.2 * dpr, MARKER_COLORS.party);
      else drawDot(c, x, y, 3.5 * dpr, MARKER_COLORS.player);
    });
    // Missão rastreada: área, ícone (preso na borda se estiver longe) e linha tracejada.
    const qt = this.quests.target();
    if (qt) {
      drawQuestTarget(c, qt, pos, (x) => W / 2 + (x - pos.x) * scale, (z) => W / 2 - (z - pos.z) * scale, scale, 6.5 * dpr, { cx: W / 2, cy: W / 2, r: W / 2 });
    }
    // Cone da câmera e seta do jogador
    c.translate(W / 2, W / 2);
    c.save();
    c.rotate(src.cameraHeading());
    const g = c.createRadialGradient(0, 0, 0, 0, 0, W / 2);
    g.addColorStop(0, "rgba(255,240,200,0.16)");
    g.addColorStop(1, "rgba(255,240,200,0)");
    c.fillStyle = g;
    c.beginPath();
    c.moveTo(0, 0);
    c.arc(0, 0, W / 2, -Math.PI / 2 - 0.6, -Math.PI / 2 + 0.6);
    c.fill();
    c.restore();
    c.rotate(pos.heading);
    c.scale(dpr, dpr);
    drawPlayerArrow(c, 12);
    c.restore();
  }
}

/** Região atual em texto (para o mapa e testes). */
export function regionLabel(regions: Region[], x: number, z: number): string {
  const r = regionAt(regions, x, z);
  return `${r.name} · ${levelText(r)}`;
}

/** Ícone de item para usar fora do HUD. */
export function itemArt(info: ItemInfo): string {
  return uniqueIds(itemIcon(info));
}
