/**
 * Navegação de missões: a missão **rastreada** (uma por vez, clique no painel Missões; a
 * primeira ativa é a padrão) guia o minimapa, o mapa do mundo e uma seta na borda da tela.
 * O alvo é o primeiro passo pendente com lugar (o servidor manda x, z, raio e tipo no
 * QuestLog); quando o passo termina, o QuestLog novo já traz o próximo e o alvo avança.
 *
 * FPS em primeiro lugar: nada de malha 3D. O desenho entra nos canvas 2D que já existem
 * (minimapa ~15×/s, mapa só aberto) e a seta é um elemento DOM só, mexido no máximo 5×/s.
 */

import { QuestPlaceKind, type QuestInfo, type QuestLog } from "../gen/erevia/v1/game_pb";
import { el, settings } from "./kit";

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

const TRACK_KEY = "erevia.questTrack";

function readTracked(): string | null {
  try {
    return localStorage.getItem(TRACK_KEY);
  } catch {
    return null; // armazenamento bloqueado: fica a padrão (a primeira)
  }
}

function saveTracked(id: string): void {
  try {
    localStorage.setItem(TRACK_KEY, id);
  } catch {
    // ignora
  }
}

/** Qual missão está rastreada e para onde ela aponta (recalculado só com QuestLog ou clique). */
export class QuestTracker {
  private quests: QuestInfo[] = [];
  private trackedId = readTracked();
  private current: QuestTarget | null = null;
  private subs = new Set<() => void>();
  private offSettings = settings.on(() => this.refresh());

  dispose(): void {
    this.offSettings();
    this.subs.clear();
  }

  /** Novo QuestLog. Se a rastreada acabou, segue a missão nova que chegou junto (a próxima
   *  da sequência); senão volta para a primeira. */
  setLog(log: QuestLog): void {
    const before = new Set(this.quests.map((q) => q.id));
    this.quests = log.quests;
    if (this.trackedId && !this.quests.some((q) => q.id === this.trackedId) && before.has(this.trackedId)) {
      const fresh = this.quests.find((q) => !before.has(q.id));
      this.trackedId = fresh?.id ?? null;
      if (fresh) saveTracked(fresh.id);
    }
    this.refresh();
  }

  /** A missão rastreada (a escolhida ou, sem escolha válida, a primeira). */
  tracked(): QuestInfo | undefined {
    return this.quests.find((q) => q.id === this.trackedId) ?? this.quests[0];
  }

  track(id: string): void {
    this.trackedId = id;
    saveTracked(id);
    this.refresh();
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

  onChange(fn: () => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
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
