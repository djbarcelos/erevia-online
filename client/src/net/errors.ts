/**
 * Relatório de erros do cliente. Todo erro que escaparia em silêncio (no celular de um
 * jogador ninguém abre o console) vai ao servidor, em POST /api/client-errors, com o
 * contexto para reproduzir: versão do build, região, posição, FPS e ping.
 *
 * - Captura sozinho window.onerror e promessas rejeitadas sem tratamento; o resto do código
 *   relata com reportClientError (perda do contexto WebGL, mensagem do servidor que não
 *   decodifica, modelo que não carrega, invariantes do cliente…).
 * - Agrupa pela mensagem + a primeira linha da stack (o servidor faz o hash com a mesma
 *   regra) e manda em lote a cada 10 s, ou com sendBeacon ao esconder ou fechar a aba.
 * - No máximo 20 erros distintos por sessão (repetições dos já vistos só somam a contagem).
 * - Guarda em memória os últimos 20 erros e avisos da sessão (recentClientErrors), que o
 *   /bug completo anexa.
 *
 * Este módulo é o primeiro import do main.ts e se instala ao ser carregado, antes de
 * qualquer outro código do jogo rodar.
 */
import type { AbstractEngine } from "@babylonjs/core";

/** Commit do build, injetado pelo Vite (vite.config.ts). */
declare const __APP_VERSION__: string;
export const CLIENT_VERSION: string = typeof __APP_VERSION__ === "string" ? __APP_VERSION__ : "dev";

const ENDPOINT = "/api/client-errors";
const FLUSH_MS = 10_000;
/** Erros distintos por sessão (os mesmos limites o servidor confere). */
const MAX_DISTINCT = 20;
const MAX_RECENT = 20;
const MAX_MESSAGE = 500;
const MAX_STACK = 4096;
const MAX_EXTRA = 2048;
/** fetch com keepalive e sendBeacon recusam corpos acima de 64 KB. */
const KEEPALIVE_MAX = 60_000;

/** Onde o jogo está (só com o jogo rodando). */
export interface ErrorContext {
  characterId?: number;
  region?: string;
  x?: number;
  z?: number;
  fps?: number;
  ping?: number;
}

/** Um erro ou aviso guardado em memória (recentClientErrors). */
export interface ClientErrorRecord {
  level: "error" | "warn";
  /** error, rejection, webgl, decode, model, invariant… */
  kind: string;
  message: string;
  stack: string;
  /** Hora da última vez (Date.now()). */
  at: number;
  /** Quantas vezes seguidas aconteceu. */
  count: number;
  extra?: Record<string, unknown>;
  ctx?: ErrorContext;
}

interface Pending {
  kind: string;
  message: string;
  stack: string;
  count: number;
  ctx?: ErrorContext;
  extra?: Record<string, unknown>;
}

/** Erros do próximo lote, pela chave (mensagem + primeira linha da stack). */
const pending = new Map<string, Pending>();
/** Chaves já vistas nesta sessão (até MAX_DISTINCT). */
const distinct = new Set<string>();
const recent: (ClientErrorRecord & { key: string })[] = [];
let contextOf: (() => ErrorContext) | null = null;
let installed = false;
/** Um erro dentro do próprio relatório não pode virar outro relatório. */
let busy = false;

/**
 * Relata um erro: vai ao servidor no próximo lote e fica em recentClientErrors.
 * `error` (opcional) dá a stack; sem ele, vale a stack de quem chamou.
 */
export function reportClientError(kind: string, message: string, extra?: Record<string, unknown>, error?: unknown): void {
  const stack = stackOf(error) ?? dropFrames(new Error().stack ?? "", 1);
  record("error", kind, message, stack, extra);
}

/** Aviso: só fica em memória (recentClientErrors), não vai ao servidor sozinho. */
export function reportClientWarning(kind: string, message: string, extra?: Record<string, unknown>): void {
  record("warn", kind, message, dropFrames(new Error().stack ?? "", 1), extra);
}

/** Os últimos 20 erros e avisos da sessão, do mais velho ao mais novo (cópias). */
export function recentClientErrors(): ClientErrorRecord[] {
  return recent.map(({ key: _key, ...r }) => ({ ...r, ctx: r.ctx && { ...r.ctx } }));
}

/** O jogo informa onde está (região, posição, FPS, ping); null ao sair do mundo. */
export function setErrorContext(fn: (() => ErrorContext) | null): void {
  contextOf = fn;
}

/** Relata a perda do contexto WebGL/WebGPU deste engine. Devolve quem desliga. */
export function watchEngine(engine: AbstractEngine, label: string): () => void {
  const lost = engine.onContextLostObservable.add(() =>
    reportClientError("webgl", `contexto WebGL perdido (${label})`, { gpu: gpuInfo(engine) }),
  );
  const restored = engine.onContextRestoredObservable.add(() => reportClientWarning("webgl", `contexto WebGL recuperado (${label})`));
  return () => {
    engine.onContextLostObservable.remove(lost);
    engine.onContextRestoredObservable.remove(restored);
  };
}

/** Manda o que estiver pendente (beacon: a aba está sumindo). */
export function flushClientErrors(beacon = false): void {
  if (!pending.size) return;
  const body = JSON.stringify({ version: CLIENT_VERSION, errors: [...pending.values()] });
  pending.clear();
  try {
    // sendBeacon com texto vai como text/plain (o servidor aceita); é o único envio que
    // sobrevive com certeza ao fechar a aba.
    if (beacon && body.length <= KEEPALIVE_MAX && navigator.sendBeacon?.(ENDPOINT, body)) return;
    void fetch(ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
      credentials: "same-origin",
      keepalive: body.length <= KEEPALIVE_MAX,
    }).catch(() => {});
  } catch {
    // Sem rede ou sem fetch: o relatório se perde, o jogo segue.
  }
}

/** Liga a captura (uma vez; já chamado ao carregar o módulo). */
export function installErrorReporting(): void {
  if (installed || typeof window === "undefined") return;
  installed = true;
  window.addEventListener("error", (ev) => {
    // Erros de carga de <img>/<script> não sobem até a window (só na captura): aqui é JS.
    if (!(ev instanceof ErrorEvent)) return;
    const err: unknown = ev.error;
    const message = err instanceof Error ? errorText(err) : ev.message || "erro sem mensagem";
    const stack = stackOf(err) ?? (ev.filename ? `at ${ev.filename}:${ev.lineno}:${ev.colno}` : "");
    record("error", "error", message, stack);
  });
  window.addEventListener("unhandledrejection", (ev) => {
    const r: unknown = ev.reason;
    record("error", "rejection", r instanceof Error ? errorText(r) : describe(r), stackOf(r) ?? "");
  });
  window.setInterval(() => flushClientErrors(), FLUSH_MS);
  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "hidden") flushClientErrors(true);
  });
  window.addEventListener("pagehide", () => flushClientErrors(true));
}

installErrorReporting();

// ---------- Por dentro ----------

function record(level: ClientErrorRecord["level"], kind: string, message: string, stack: string, extra?: Record<string, unknown>): void {
  if (busy) return;
  busy = true;
  try {
    const msg = clip(String(message), MAX_MESSAGE) || "(sem mensagem)";
    const st = clip(stack, MAX_STACK);
    const ex = extra && safeExtra(extra);
    const ctx = currentContext();
    const key = msg + "\n" + firstStackFrame(st);
    remember({ level, kind, message: msg, stack: st, at: Date.now(), count: 1, extra: ex, ctx, key });
    if (level !== "error") return;
    const p = pending.get(key);
    if (p) {
      p.count++;
      p.kind = kind;
      p.ctx = ctx;
      if (ex) p.extra = ex;
      return;
    }
    if (!distinct.has(key)) {
      if (distinct.size >= MAX_DISTINCT) return;
      distinct.add(key);
    }
    pending.set(key, { kind, message: msg, stack: st, count: 1, ctx, extra: ex });
  } catch {
    // Nunca deixa o relatório quebrar o jogo.
  } finally {
    busy = false;
  }
}

/** Guarda nos últimos 20; o mesmo erro seguido só soma (não empurra os outros para fora). */
function remember(r: ClientErrorRecord & { key: string }): void {
  const last = recent[recent.length - 1];
  if (last && last.key === r.key && last.level === r.level) {
    last.count++;
    last.at = r.at;
    last.ctx = r.ctx;
    return;
  }
  recent.push(r);
  if (recent.length > MAX_RECENT) recent.shift();
}

/**
 * A primeira linha da stack que é um quadro de chamada (V8: "at f (url:l:c)";
 * Firefox/Safari: "f@url:l:c"), sem o cabeçalho "Error: mensagem". Mesma regra do servidor
 * (store.FirstStackFrame).
 */
export function firstStackFrame(stack: string): string {
  let first = "";
  for (const raw of stack.split("\n")) {
    const line = raw.trim();
    if (!line) continue;
    if (isFrame(line)) return line;
    if (!first) first = line;
  }
  return first;
}

function isFrame(line: string): boolean {
  return line.startsWith("at ") || line.includes("@");
}

/** Só os quadros da stack, sem os n primeiros (os deste módulo). */
function dropFrames(stack: string, n: number): string {
  const frames = stack.split("\n").filter((l) => isFrame(l.trim()));
  return frames.slice(n).join("\n");
}

function stackOf(err: unknown): string | undefined {
  return err instanceof Error && typeof err.stack === "string" && err.stack ? err.stack : undefined;
}

function errorText(err: Error): string {
  try {
    return err.message ? `${err.name || "Error"}: ${err.message}` : err.name || "Error";
  } catch {
    return "Error";
  }
}

function describe(v: unknown): string {
  if (typeof v === "string") return v;
  try {
    return JSON.stringify(v) ?? String(v);
  } catch {
    return Object.prototype.toString.call(v);
  }
}

function clip(s: string, n: number): string {
  return s.length > n ? s.slice(0, n) : s;
}

/** Cópia que vira JSON com certeza (sem ciclos nem BigInt) e cabe em 2 KB. */
function safeExtra(extra: Record<string, unknown>): Record<string, unknown> {
  try {
    const json = JSON.stringify(extra, (_k, v: unknown) => (typeof v === "bigint" ? v.toString() : v));
    if (!json) return {};
    return json.length > MAX_EXTRA ? { truncated: json.slice(0, MAX_EXTRA - 100) } : (JSON.parse(json) as Record<string, unknown>);
  } catch {
    return { unserializable: true };
  }
}

function currentContext(): ErrorContext | undefined {
  if (!contextOf) return undefined;
  try {
    const c = contextOf();
    const round = (v: number | undefined, k: number) => (v === undefined || !Number.isFinite(v) ? undefined : Math.round(v * k) / k);
    return {
      characterId: c.characterId,
      region: c.region,
      x: round(c.x, 10),
      z: round(c.z, 10),
      fps: round(c.fps, 10),
      ping: round(c.ping, 1),
    };
  } catch {
    return undefined;
  }
}

function gpuInfo(engine: AbstractEngine): string {
  try {
    // Só o engine WebGL tem getGlInfo (o WebGPU não).
    const info = (engine as { getGlInfo?: () => { vendor: string; renderer: string } }).getGlInfo?.();
    return info ? `${info.vendor} · ${info.renderer}` : engine.description;
  } catch {
    return "";
  }
}
