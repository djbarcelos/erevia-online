/**
 * Navegação de missões: a missão **rastreada** (uma por vez, sempre entre as fixadas no
 * painel; clique no painel ou "Rastrear" no Diário) guia o minimapa, o mapa do mundo e uma
 * seta na borda da tela.
 * O alvo é o primeiro passo pendente com lugar (o servidor manda x, z, raio e tipo no
 * QuestLog); quando o passo termina, o QuestLog novo já traz o próximo e o alvo avança.
 *
 * FPS em primeiro lugar: nada de malha 3D. O desenho entra nos canvas 2D que já existem
 * (minimapa ~15×/s, mapa só aberto) e a seta é um elemento DOM só, mexido no máximo 5×/s.
 */

import { QuestPlaceKind, type QuestDone, type QuestInfo, type QuestLog } from "../gen/erevia/v1/game_pb";
import { el, settings } from "./kit";
import { QuestPins, type PinStorage } from "./quest-pins";

/** Para onde a missão rastreada aponta agora. */
export interface QuestTarget {
  questId: string;
  kind: QuestPlaceKind;
  x: number;
  z: number;
  /** Metros: dentro dele o jogador chegou (a seta some). */
  radius: number;
  name: string;
  /** Índice do passo (o "Ir" percebe quando ele termina). */
  step: number;
  /** Matar: nomes dos monstros que contam (vazio = qualquer). */
  monsters: string[];
}

/** O primeiro passo pendente com lugar desta missão (null = nenhum: usar skill, poção...). */
export function questStepTarget(q: QuestInfo): QuestTarget | null {
  const i = q.steps.findIndex((s) => s.count < s.need && s.place !== QuestPlaceKind.NONE);
  if (i < 0) return null;
  const s = q.steps[i];
  return { questId: q.id, kind: s.place, x: s.x, z: s.z, radius: Math.max(1, s.radius), name: s.placeName, step: i, monsters: s.monsters };
}

/** Rastreada do esquema antigo (uma chave só para todos os personagens). */
const LEGACY_TRACK_KEY = "erevia.questTrack";
/** Fixadas e rastreada, por personagem (quest-pins.ts). */
const PINS_KEY = "erevia.questPins.";

function readLegacyTrack(): string | null {
  try {
    return localStorage.getItem(LEGACY_TRACK_KEY);
  } catch {
    return null; // armazenamento bloqueado
  }
}

function pinStorage(characterId: number): PinStorage {
  const key = PINS_KEY + characterId;
  return {
    load: () => localStorage.getItem(key),
    save: (raw) => localStorage.setItem(key, raw),
  };
}

/**
 * As missões ativas, as concluídas, as fixadas no painel e a rastreada (que guia o minimapa,
 * o mapa e a seta). Recalculado só com QuestLog, clique ou o fim do tempo da recém-aceita.
 */
export class QuestTracker {
  private quests: QuestInfo[] = [];
  private done: QuestDone[] = [];
  private pins: QuestPins;
  private current: QuestTarget | null = null;
  private subs = new Set<() => void>();
  private panelSubs = new Set<() => void>();
  private offSettings = settings.on(() => this.refresh());
  private expiry = 0;

  constructor(characterId: number) {
    this.pins = new QuestPins(pinStorage(characterId), () => Date.now(), readLegacyTrack());
  }

  dispose(): void {
    this.offSettings();
    clearTimeout(this.expiry);
    this.subs.clear();
    this.panelSubs.clear();
  }

  /** Novo QuestLog. A missão concluída sai do painel; se era a rastreada, segue a nova que
   *  chegou junto (a próxima da sequência). */
  setLog(log: QuestLog): void {
    this.quests = log.quests;
    if (log.doneFull) this.done = log.done;
    this.pins.update(this.quests.map((q) => q.id));
    this.panelChanged();
  }

  /** Ativas, na ordem do servidor. */
  list(): readonly QuestInfo[] {
    return this.quests;
  }

  /** Concluídas (só id, título e categoria). */
  doneList(): readonly QuestDone[] {
    return this.done;
  }

  /** O que aparece no painel do HUD: as fixadas e a recém-aceita esperando o "Fixar". */
  panel(): QuestInfo[] {
    return this.quests.filter((q) => this.pins.visible(q.id));
  }

  /** A missão rastreada (sempre uma das fixadas; nenhuma se nada estiver fixado). */
  tracked(): QuestInfo | undefined {
    const id = this.pins.tracked();
    return id ? this.quests.find((q) => q.id === id) : undefined;
  }

  isPinned(id: string): boolean {
    return this.pins.isPinned(id);
  }

  isTemp(id: string): boolean {
    return this.pins.isTemp(id);
  }

  /** Quantos ms faltam para a recém-aceita sumir do painel. */
  tempLeft(id: string): number {
    return this.pins.tempLeft(id);
  }

  pinnedCount(): number {
    return this.pins.pinned().length;
  }

  /** Título da fixada que sairia para dar lugar a mais uma (null = há lugar). */
  wouldEvict(): string | null {
    const id = this.pins.wouldEvict();
    return id ? (this.quests.find((q) => q.id === id)?.title ?? null) : null;
  }

  /** Rastreia (e fixa). Devolve o título da que saiu do painel para dar lugar. */
  track(id: string): string | null {
    const before = this.snapshot();
    return this.after(this.pins.track(id), before);
  }

  /** Fixa no painel. Devolve o título da que saiu para dar lugar. */
  pin(id: string): string | null {
    const before = this.snapshot();
    return this.after(this.pins.pin(id), before);
  }

  unpin(id: string): void {
    const before = this.snapshot();
    this.pins.unpin(id);
    this.after(null, before);
  }

  /** Alvo de agora de uma missão qualquer (o "Ir"; vale com os marcadores desligados). */
  targetOf(questId: string): QuestTarget | null {
    const q = this.quests.find((q) => q.id === questId);
    return q ? questStepTarget(q) : null;
  }

  /** Alvo atual (null: sem missão com lugar ou marcadores desligados). */
  target(): QuestTarget | null {
    return this.current;
  }

  /** O alvo mudou (minimapa, mapa, seta). */
  onChange(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }

  /** Chegou QuestLog ou mudou a fixação (painel do HUD e Diário redesenham). */
  onPanel(fn: () => void): () => void {
    this.panelSubs.add(fn);
    return () => this.panelSubs.delete(fn);
  }

  /** Clique de fixar/rastrear: redesenha só se algo mudou de fato. */
  private after(evicted: string | null, before: string): string | null {
    const title = evicted ? (this.quests.find((q) => q.id === evicted)?.title ?? null) : null;
    if (this.snapshot() !== before) this.panelChanged();
    return title;
  }

  private snapshot(): string {
    return this.pins.key();
  }

  private panelChanged(): void {
    // Um timeout só, para a próxima recém-aceita que some (nada por quadro).
    clearTimeout(this.expiry);
    const ms = this.pins.nextExpiry();
    if (ms !== null)
      this.expiry = window.setTimeout(() => {
        if (this.pins.expire()) this.panelChanged();
      }, ms + 50);
    for (const fn of this.panelSubs) fn();
    this.refresh();
  }

  private refresh(): void {
    const q = this.tracked();
    this.current = q && settings.get().questMarkers ? questStepTarget(q) : null;
    for (const fn of this.subs) fn();
  }
}

export const QUEST_COLOR = "#ffd36b";

/**
 * Desenha o alvo num canvas de mapa: área de caça como círculo destacado, NPC, loja, pedra
 * e portão com ícone, e a linha tracejada do jogador até lá. sx/sy levam metros para pixels
 * e `scale` é pixels por metro. `clampR` (minimapa redondo): o ícone de quem está fora fica
 * preso na borda, na direção do alvo.
 */
export function drawQuestTarget(
  ctx: CanvasRenderingContext2D,
  t: QuestTarget,
  me: { x: number; z: number },
  sx: (x: number) => number,
  sy: (z: number) => number,
  scale: number,
  icon: number,
  clamp?: { cx: number; cy: number; r: number },
): void {
  const px = sx(me.x);
  const py = sy(me.z);
  let tx = sx(t.x);
  let ty = sy(t.z);
  const rr = t.radius * scale;
  ctx.save();
  // Linha: do jogador até a borda da área (ou até o ícone).
  const d = Math.hypot(tx - px, ty - py);
  const stop = t.kind === QuestPlaceKind.HUNT ? rr : icon;
  if (d > stop + 2) {
    const k = (d - stop) / d;
    ctx.setLineDash([icon * 0.5, icon * 0.45]);
    ctx.lineWidth = Math.max(1.5, icon * 0.18);
    ctx.strokeStyle = "rgba(255,211,107,0.9)";
    ctx.lineCap = "round";
    ctx.beginPath();
    ctx.moveTo(px, py);
    ctx.lineTo(px + (tx - px) * k, py + (ty - py) * k);
    ctx.stroke();
    ctx.setLineDash([]);
  }
  if (t.kind === QuestPlaceKind.HUNT) {
    ctx.beginPath();
    ctx.arc(tx, ty, rr, 0, Math.PI * 2);
    ctx.fillStyle = "rgba(255,211,107,0.16)";
    ctx.fill();
    ctx.setLineDash([icon * 0.55, icon * 0.35]);
    ctx.lineWidth = Math.max(1.5, icon * 0.2);
    ctx.strokeStyle = "rgba(255,211,107,0.95)";
    ctx.stroke();
    ctx.setLineDash([]);
  }
  if (clamp) {
    const dx = tx - clamp.cx;
    const dy = ty - clamp.cy;
    const dd = Math.hypot(dx, dy);
    const lim = clamp.r - icon * 1.1;
    if (dd > lim && (t.kind !== QuestPlaceKind.HUNT || dd - rr > lim)) {
      tx = clamp.cx + (dx / dd) * lim;
      ty = clamp.cy + (dy / dd) * lim;
    } else if (t.kind === QuestPlaceKind.HUNT) {
      ctx.restore();
      return; // a área já aparece inteira no minimapa
    }
  }
  drawQuestIcon(ctx, t.kind, tx, ty, icon);
  ctx.restore();
}

/** Ícone do tipo de lugar: medalhão dourado com o desenho dentro. */
export function drawQuestIcon(ctx: CanvasRenderingContext2D, kind: QuestPlaceKind, x: number, y: number, r: number): void {
  ctx.beginPath();
  ctx.arc(x, y, r, 0, Math.PI * 2);
  ctx.fillStyle = kind === QuestPlaceKind.STONE ? "#1d2a4a" : "#2a1d07";
  ctx.fill();
  ctx.lineWidth = Math.max(1.2, r * 0.22);
  ctx.strokeStyle = kind === QuestPlaceKind.STONE ? "#9cc4ff" : QUEST_COLOR;
  ctx.stroke();
  ctx.fillStyle = kind === QuestPlaceKind.STONE ? "#dfeeff" : QUEST_COLOR;
  ctx.strokeStyle = ctx.fillStyle;
  const u = r * 0.55;
  ctx.beginPath();
  switch (kind) {
    case QuestPlaceKind.NPC: // "!" de quem tem algo a dizer
      ctx.fillRect(x - u * 0.18, y - u, u * 0.36, u * 1.25);
      ctx.arc(x, y + u * 0.72, u * 0.2, 0, Math.PI * 2);
      ctx.fill();
      break;
    case QuestPlaceKind.SHOP: // saco de moedas
      ctx.arc(x, y + u * 0.25, u * 0.68, 0, Math.PI * 2);
      ctx.moveTo(x - u * 0.38, y - u * 0.75);
      ctx.lineTo(x + u * 0.38, y - u * 0.75);
      ctx.lineTo(x + u * 0.16, y - u * 0.3);
      ctx.lineTo(x - u * 0.16, y - u * 0.3);
      ctx.fill();
      break;
    case QuestPlaceKind.STONE: // cristal de quatro pontas
      ctx.moveTo(x, y - u);
      ctx.lineTo(x + u * 0.3, y - u * 0.3);
      ctx.lineTo(x + u, y);
      ctx.lineTo(x + u * 0.3, y + u * 0.3);
      ctx.lineTo(x, y + u);
      ctx.lineTo(x - u * 0.3, y + u * 0.3);
      ctx.lineTo(x - u, y);
      ctx.lineTo(x - u * 0.3, y - u * 0.3);
      ctx.fill();
      break;
    case QuestPlaceKind.GATE: // arco do portão
      ctx.lineWidth = Math.max(1.2, u * 0.3);
      ctx.moveTo(x - u * 0.75, y + u * 0.8);
      ctx.lineTo(x - u * 0.75, y - u * 0.05);
      ctx.arc(x, y - u * 0.05, u * 0.75, Math.PI, 0);
      ctx.lineTo(x + u * 0.75, y + u * 0.8);
      ctx.stroke();
      break;
    default: // área de caça: espadas cruzadas viram um "x" simples
      ctx.lineWidth = Math.max(1.2, u * 0.3);
      ctx.moveTo(x - u * 0.7, y - u * 0.7);
      ctx.lineTo(x + u * 0.7, y + u * 0.7);
      ctx.moveTo(x + u * 0.7, y - u * 0.7);
      ctx.lineTo(x - u * 0.7, y + u * 0.7);
      ctx.stroke();
  }
}

/** Ponto do mundo na tela (px do viewport); front = na frente da câmera. */
export type Project = (x: number, z: number) => { x: number; y: number; front: boolean } | null;

const ARROW_MARGIN = 56;

/**
 * Seta na borda da tela apontando para o alvo quando ele está fora de vista, com a
 * distância. Um elemento só; o chamador limita a ~5 atualizações por segundo, e o estilo
 * só é mexido quando algo mudou.
 */
export class QuestArrow {
  readonly el = el("div", "quest-arrow");
  private shown = false;
  private last = "";
  private lastText = "";

  constructor() {
    this.el.hidden = true;
    this.el.setAttribute("aria-hidden", "true");
  }

  update(t: QuestTarget | null, me: { x: number; z: number } | null, project: Project): void {
    const dist = t && me ? Math.hypot(t.x - me.x, t.z - me.z) : 0;
    const p = t && me && dist > t.radius ? project(t.x, t.z) : null;
    const W = innerWidth;
    const H = innerHeight;
    const onScreen = p && p.front && p.x > ARROW_MARGIN && p.x < W - ARROW_MARGIN && p.y > ARROW_MARGIN && p.y < H - ARROW_MARGIN;
    if (!t || !p || onScreen) {
      if (this.shown) this.el.hidden = true;
      this.shown = false;
      return;
    }
    // Atrás da câmera a projeção sai espelhada: inverte para apontar certo.
    let dx = p.x - W / 2;
    let dy = p.y - H / 2;
    if (!p.front) {
      dx = -dx;
      dy = -dy;
    }
    if (Math.abs(dx) < 1e-3 && Math.abs(dy) < 1e-3) dy = 1;
    const k = Math.min((W / 2 - ARROW_MARGIN) / Math.max(1e-3, Math.abs(dx)), (H / 2 - ARROW_MARGIN) / Math.max(1e-3, Math.abs(dy)));
    const x = Math.round(W / 2 + dx * k);
    const y = Math.round(H / 2 + dy * k);
    const a = Math.round((Math.atan2(dy, dx) * 180) / Math.PI);
    const key = `${x},${y},${a}`;
    if (key !== this.last) {
      this.last = key;
      this.el.style.transform = `translate(${x}px, ${y}px)`;
      this.el.style.setProperty("--a", `${a}deg`);
    }
    const text = `${Math.round(dist)} m`;
    if (text !== this.lastText) this.el.textContent = this.lastText = text;
    if (this.el.title !== t.name) this.el.title = t.name;
    if (!this.shown) this.el.hidden = false;
    this.shown = true;
  }
}
