/**
 * Dessincronia cliente × servidor (tunnel#122): quanto o jogo "dá tranco" e onde.
 *
 * Contadores da janela atual, zerados a cada envio (junto com a amostra de FPS, a cada ~60 s,
 * em perf-sampler.ts):
 *  - correções da posição prevista do jogador local acima de 0,5 m e acima de 2 m
 *    (entities.ts, reconcileLocal);
 *  - retomadas: a previsão errou demais (LOCAL_SNAP) e o servidor assumiu o movimento;
 *  - piscadas: uma entidade some (Despawn) e volta (Spawn) em menos de 1 s;
 *  - ping: cada medida de ida e volta (connection.ts), para a média e o p95.
 * O servidor grava como game_events.kind = 'client_sync', e a view v_client_sync junta por
 * região e faixa de ping.
 */

export const CORRECTION_MIN = 0.5;
export const CORRECTION_BIG = 2;
const FLICKER_MS = 1000;

export interface SyncSample {
  corrections: number;
  correctionsBig: number;
  resyncs: number;
  flickers: number;
  pingAvg: number;
  pingP95: number;
}

let corrections = 0;
let correctionsBig = 0;
let resyncs = 0;
let flickers = 0;
let pings: number[] = [];
const gone = new Map<number, number>(); // id → quando sumiu (ms)

/** Erro (m) entre a posição prevista do jogador local e a do servidor, numa correção. */
export function noteCorrection(err: number): void {
  if (!(err > CORRECTION_MIN)) return;
  corrections++;
  if (err > CORRECTION_BIG) correctionsBig++;
}

/** O servidor assumiu o movimento do jogador local (a previsão errou demais). */
export function noteResync(): void {
  resyncs++;
}

/** Uma entidade saiu de cena (Despawn). */
export function noteDespawn(id: number, now = performance.now()): void {
  gone.set(id, now);
  if (gone.size > 2000) {
    for (const [k, t] of gone) if (now - t > FLICKER_MS) gone.delete(k);
  }
}

/** Uma entidade entrou em cena (Spawn): conta se tinha saído há menos de 1 s. */
export function noteSpawn(id: number, now = performance.now()): void {
  const t = gone.get(id);
  if (t === undefined) return;
  gone.delete(id);
  if (now - t < FLICKER_MS) flickers++;
}

/** Uma medida de ping (ms). */
export function notePing(ms: number): void {
  if (Number.isFinite(ms) && ms >= 0 && pings.length < 1000) pings.push(ms);
}

/** Os números da janela, e zera para a próxima. */
export function takeSyncSample(): SyncSample {
  const sorted = [...pings].sort((a, b) => a - b);
  const avg = sorted.length ? sorted.reduce((s, v) => s + v, 0) / sorted.length : 0;
  const p95 = sorted.length ? sorted[Math.min(sorted.length - 1, Math.floor(sorted.length * 0.95))] : 0;
  const out = { corrections, correctionsBig, resyncs, flickers, pingAvg: avg, pingP95: p95 };
  corrections = correctionsBig = resyncs = flickers = 0;
  pings = [];
  return out;
}
