/**
 * /bug completo: o relato vai com o contexto do cliente e um print do canvas.
 *
 * 1. POST /api/bug-reports {text, characterId, context} → {id}. O servidor confere o
 *    personagem online da conta, aplica o intervalo do /bug (o mesmo do chat) e grava.
 * 2. POST /api/bug-reports/{id}/shot com o JPEG (só o dono, uma vez, até 10 min depois).
 *
 * Servidor sem a rota ou fora do ar: cai no /bug do chat (só o texto, como antes).
 * Os dois caminhos: /bug texto no chat e o botão Configurações → Sobre → Relatar problema.
 *
 * O contexto (formato no README, "Moderação") leva os erros recentes, FPS do último minuto,
 * ping, região, hora do jogo e os tipos e ids das últimas mensagens do servidor, nunca o
 * texto do chat.
 */
import type { Scene } from "@babylonjs/core";
import type { ServerMessage } from "../gen/erevia/v1/game_pb";
import { CLIENT_VERSION, recentClientErrors, type ClientErrorRecord } from "../net/errors";
import { brokenInvariants } from "../net/invariants";
import { hourLabel, isNight } from "../world/daynight";
import { button, el, isCompact, isTouch, settings, toast } from "./kit";
import "../styles/bug-report.css";

/** Formato do contexto (sobe quando um campo muda de sentido). */
export const BUG_CONTEXT_VERSION = 1;
/** O servidor aceita até 32 KB; o cliente para antes. */
const CONTEXT_MAX = 30_000;
const TEXT_MAX = 1000;
const LOG_MAX = 30;
/** Print: lado maior até 640 px, JPEG 0,6; acima de 80 KB tenta de novo menor. */
const SHOT_SIDE = 640;
const SHOT_QUALITY = 0.6;
const SHOT_MAX_BYTES = 80_000;

// ---------- Últimas mensagens do servidor (tipos e ids) ----------

interface LoggedMessage {
  at: number;
  type: string;
  id?: number;
  /** Quantas seguidas iguais (mesmo tipo e id). */
  n: number;
}

const messages: LoggedMessage[] = [];

/**
 * Anota uma mensagem que chegou do servidor (chamado para todas, então é barato: sem
 * cópia, só o tipo e um id). O snapshot (20×/s) só entra quando traz mortes, revividas,
 * subidas de nível ou avisos de chefe. Do chat, só o tipo: nunca o texto nem quem falou.
 */
export function noteServerMessage(msg: ServerMessage): void {
  const m = msg.msg;
  if (!m.case) return;
  if (m.case === "snapshot") {
    const s = m.value;
    if (s.deaths.length) logMessage("snapshot.deaths", s.deaths[0], s.deaths.length);
    if (s.revives.length) logMessage("snapshot.revives", s.revives[0].id, s.revives.length);
    if (s.levelUps.length) logMessage("snapshot.levelUps", s.levelUps[0].id, s.levelUps.length);
    if (s.telegraphs.length) logMessage("snapshot.telegraphs", undefined, s.telegraphs.length);
    return;
  }
  const v = m.value as { id?: unknown; entity?: { id?: unknown } };
  const id = typeof v.id === "number" ? v.id : typeof v.entity?.id === "number" ? v.entity.id : undefined;
  logMessage(m.case, id, 1);
}

function logMessage(type: string, id: number | undefined, n: number): void {
  const now = performance.now();
  const last = messages[messages.length - 1];
  if (last && last.type === type && last.id === id) {
    last.n += n;
    last.at = now;
    return;
  }
  messages.push({ at: now, type, id, n });
  if (messages.length > LOG_MAX) messages.shift();
}

// ---------- Contexto ----------

export interface BugDeps {
  characterId: number;
  scene: Scene;
  ping(): number;
  position(): { x: number; z: number } | undefined;
  region(): string | undefined;
  /** Fase do dia (0–1, WorldClock). */
  phase(): number;
  fps(): { avg: number; min: number; samples: number };
  /** Jogadores na tela. */
  players(): number;
  /** /bug pelo chat (o caminho antigo, sem contexto). */
  viaChat(text: string): void;
}

const round = (v: number, k = 10) => Math.round(v * k) / k;

/** O contexto que vai em bug_reports.context (cabe em 30 KB). */
export function buildBugContext(d: BugDeps): Record<string, unknown> {
  const now = performance.now();
  const p = d.position();
  const phase = d.phase();
  const s = settings.get();
  const ctx: Record<string, unknown> = {
    v: BUG_CONTEXT_VERSION,
    client: CLIENT_VERSION,
    at: new Date().toISOString(),
    fps: d.fps(),
    ping: d.ping(),
    region: d.region() ?? null,
    x: p ? round(p.x) : null,
    z: p ? round(p.z) : null,
    gameTime: { phase: round(phase, 1000), hour: hourLabel(phase), night: isNight(phase) },
    screen: {
      w: innerWidth,
      h: innerHeight,
      dpr: round(devicePixelRatio || 1, 100),
      touch: isTouch(),
      quality: s.quality,
      shadows: s.shadows,
      viewDistance: s.viewDistance,
    },
    players: d.players(),
    invariants: brokenInvariants(),
    errors: recentClientErrors().map((e) => slimError(e, true, true)),
    messages: messages.map((m) => ({ t: round((m.at - now) / 1000), type: m.type, ...(m.id !== undefined && { id: m.id }), ...(m.n > 1 && { n: m.n }) })),
  };
  // Grande demais (stacks e extras longos): tira primeiro as stacks, depois os extras,
  // depois os erros mais antigos.
  // O limite do servidor é em bytes (UTF-8), não em caracteres.
  const enc = new TextEncoder();
  const tooBig = () => enc.encode(JSON.stringify(ctx)).length > CONTEXT_MAX;
  const errs = recentClientErrors();
  if (tooBig()) ctx.errors = errs.map((e) => slimError(e, false, true));
  if (tooBig()) ctx.errors = errs.map((e) => slimError(e, false, false));
  while (tooBig() && (ctx.errors as unknown[]).length) (ctx.errors as unknown[]).shift();
  return ctx;
}

function slimError(e: ClientErrorRecord, stack: boolean, extra: boolean): Record<string, unknown> {
  const out: Record<string, unknown> = { level: e.level, kind: e.kind, message: e.message.slice(0, 300), at: new Date(e.at).toISOString(), count: e.count };
  if (stack && e.stack) out.stack = e.stack.split("\n").slice(0, 3).join("\n").slice(0, 600);
  if (extra && e.extra) {
    const j = JSON.stringify(e.extra);
    out.extra = j.length <= 600 ? e.extra : j.slice(0, 600);
  }
  return out;
}

// ---------- Print do canvas ----------

/**
 * Print do que o canvas 3D mostra (sem a interface HTML), em JPEG pequeno. O engine usa
 * preserveDrawingBuffer: false, então a cópia é feita no fim de um render, no mesmo
 * tique (onAfterRenderObservable), antes de o navegador descartar o buffer.
 */
export function captureShot(scene: Scene): Promise<Blob | null> {
  return new Promise((resolve) => {
    let settled = false;
    const done = (b: Blob | null) => {
      if (settled) return;
      settled = true;
      resolve(b);
    };
    const timeout = window.setTimeout(() => {
      scene.onAfterRenderObservable.remove(obs);
      done(null); // nada renderizou (aba escondida?): o relato vai sem print
    }, 2000);
    const obs = scene.onAfterRenderObservable.addOnce(() => {
      window.clearTimeout(timeout);
      try {
        const src = scene.getEngine().getRenderingCanvas();
        if (!src || !src.width || !src.height) return done(null);
        const k = Math.min(1, SHOT_SIDE / Math.max(src.width, src.height));
        const copy = document.createElement("canvas");
        copy.width = Math.max(1, Math.round(src.width * k));
        copy.height = Math.max(1, Math.round(src.height * k));
        const g = copy.getContext("2d");
        if (!g) return done(null);
        g.drawImage(src, 0, 0, copy.width, copy.height);
        void encodeSmall(copy).then(done, () => done(null));
      } catch {
        done(null);
      }
    });
  });
}

/** JPEG abaixo de SHOT_MAX_BYTES: tenta com qualidade menor e depois menor ainda. */
async function encodeSmall(c: HTMLCanvasElement): Promise<Blob | null> {
  for (const q of [SHOT_QUALITY, 0.45, 0.3]) {
    const b = await new Promise<Blob | null>((res) => c.toBlob(res, "image/jpeg", q));
    if (!b) return null;
    if (b.size <= SHOT_MAX_BYTES) return b;
  }
  // Ainda grande: metade do tamanho.
  const half = document.createElement("canvas");
  half.width = Math.max(1, c.width >> 1);
  half.height = Math.max(1, c.height >> 1);
  half.getContext("2d")?.drawImage(c, 0, 0, half.width, half.height);
  const b = await new Promise<Blob | null>((res) => half.toBlob(res, "image/jpeg", 0.5));
  return b && b.size <= SHOT_MAX_BYTES ? b : null;
}

// ---------- Envio ----------

export type BugResult =
  | { ok: true; id: number; shot: boolean }
  /** Recusado pelo servidor (curto, intervalo, fora do jogo): a mensagem dele. */
  | { ok: false; message: string }
  /** Sem a rota ou sem servidor: foi pelo chat, sem contexto (a resposta vem no chat). */
  | { ok: false; viaChat: true };

let sending = false;

/** Manda o relato com o contexto e, se der, o print. */
export async function submitBugReport(text: string, d: BugDeps, shot: Promise<Blob | null>): Promise<BugResult> {
  if (sending) return { ok: false, message: "Enviando o relato anterior…" };
  sending = true;
  try {
    const body = JSON.stringify({ text: text.slice(0, TEXT_MAX), characterId: d.characterId, context: buildBugContext(d) });
    let res: Response;
    try {
      res = await fetch("/api/bug-reports", { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/json" }, body });
    } catch {
      d.viaChat(text);
      return { ok: false, viaChat: true };
    }
    const data = (await res.json().catch(() => ({}))) as { id?: number; error?: string };
    if (res.status === 201 && typeof data.id === "number") {
      const blob = await shot;
      let sent = false;
      if (blob) {
        sent = await fetch(`/api/bug-reports/${data.id}/shot`, { method: "POST", credentials: "same-origin", headers: { "Content-Type": "image/jpeg" }, body: blob })
          .then((r) => r.status === 204)
          .catch(() => false);
      }
      return { ok: true, id: data.id, shot: sent };
    }
    if ([400, 409, 413, 429].includes(res.status) && data.error) return { ok: false, message: data.error };
    // 404/405 (servidor antigo), 5xx, 503 (zona ocupada): o caminho antigo ainda funciona.
    d.viaChat(text);
    return { ok: false, viaChat: true };
  } finally {
    sending = false;
  }
}

/** Frase para o chat/aviso com o resultado. */
export function bugResultText(r: BugResult): string | null {
  if (r.ok) return `Obrigado! Seu relato #${r.id} foi enviado${r.shot ? " com um print da tela" : ""}.`;
  if ("viaChat" in r) return null; // o servidor responde no chat
  return r.message;
}

/** /bug texto no chat: o mesmo envio, com o print do quadro atual. */
export function bugFromChat(text: string, d: BugDeps, say: (line: string) => void): void {
  const shot = captureShot(d.scene);
  void submitBugReport(text, d, shot).then((r) => {
    const line = bugResultText(r);
    if (line) say(line);
  });
}

// ---------- Caixinha "Relatar problema" (Configurações → Sobre) ----------

/** Abre a caixinha do relato. O print é tirado na hora em que ela abre (o jogo atrás). */
export function openBugDialog(d: BugDeps): void {
  document.querySelector(".bug-modal")?.remove();
  const shot = captureShot(d.scene);
  const scrim = el("div", "account-modal bug-modal");
  const onKey = (e: KeyboardEvent) => {
    if (e.key === "Escape") {
      e.stopPropagation();
      close();
    }
  };
  const close = () => {
    scrim.remove();
    document.removeEventListener("keydown", onKey, true);
  };
  document.addEventListener("keydown", onKey, true);
  scrim.onclick = (e) => {
    if (e.target === scrim) close();
  };

  const x = button("✕", "btn btn-icon btn-ghost btn-sm account-x", close);
  x.setAttribute("aria-label", "Fechar");
  const text = el("textarea", "input bug-text");
  text.id = "bug-text";
  text.maxLength = TEXT_MAX;
  text.rows = 5;
  text.placeholder = "Ex.: o monstro ficou parado no meio da ponte e não dava para atacar.";
  const label = el("label", "label", "O que aconteceu?");
  label.htmlFor = text.id;
  const count = el("span", "muted small bug-count", `0 / ${TEXT_MAX}`);
  const error = el("p", "account-error bad small");
  error.setAttribute("role", "alert");
  const send = button("Enviar relato", "btn btn-gold btn-block");
  send.disabled = true;
  const valid = () => text.value.trim().length >= 8;
  text.oninput = () => {
    count.textContent = `${text.value.length} / ${TEXT_MAX}`;
    send.disabled = !valid();
    error.textContent = "";
  };
  // As teclas do jogo (WASD, atalhos) não podem disparar enquanto escreve.
  text.addEventListener("keydown", (e) => e.stopPropagation());
  send.onclick = async () => {
    if (!valid() || send.classList.contains("loading")) return;
    send.disabled = true;
    text.disabled = true;
    send.classList.add("loading");
    const r = await submitBugReport(text.value.trim(), d, shot);
    send.classList.remove("loading");
    if (r.ok || "viaChat" in r) {
      close();
      toast(bugResultText(r) ?? "Relato enviado pelo chat (sem o print).");
      return;
    }
    error.textContent = r.message;
    text.disabled = false;
    send.disabled = !valid();
  };

  const box = el(
    "section",
    "account-dialog frame bug-dialog",
    x,
    el("h2", "title", "Relatar problema"),
    el("p", "muted small", "Conte o que aconteceu e onde. Junto vão um print da tela do jogo (sem o chat), o FPS, o ping, a região e os erros recentes, para a equipe reproduzir."),
    label,
    text,
    count,
    error,
    send,
  );
  box.setAttribute("role", "dialog");
  box.setAttribute("aria-modal", "true");
  box.setAttribute("aria-label", "Relatar problema");
  scrim.append(box);
  document.body.append(scrim);
  if (!isCompact()) text.focus({ preventScroll: true });
}
