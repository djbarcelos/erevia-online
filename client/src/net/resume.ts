/**
 * Reconexão automática. Quando a conexão cai (sem expulsão), esperamos o servidor
 * responder e recarregamos a página, que entra direto no mesmo personagem: é o mesmo
 * caminho da entrada normal. Recarregar sem rede mostraria a tela de erro do navegador
 * (e o jogo sumiria), por isso só recarregamos depois de o servidor responder.
 *
 * O personagem a retomar fica no sessionStorage (só nesta aba).
 */

const KEY = "erevia:resume";
/** Servidor fora do ar por mais que isso: desiste (o jogador ainda pode tentar de novo). */
const GIVE_UP_MS = 2 * 60_000;
/** Recargas seguidas sem conseguir entrar: desiste (ex.: o WebSocket é bloqueado na rede). */
const MAX_RELOADS = 5;
const MAX_DELAY_MS = 8000;
/** A retomada só vale logo depois da recarga. */
const FRESH_MS = 60_000;

export interface Resume {
  /** Personagem a retomar. */
  id: number;
  /** Quando a conexão caiu (Date.now()). */
  since: number;
  /** Recargas já feitas desde a queda. */
  reloads: number;
}

export function readResume(): Resume | null {
  try {
    const raw = sessionStorage.getItem(KEY);
    if (raw) {
      const r = JSON.parse(raw) as Partial<Resume> & { at?: number };
      if (typeof r.id === "number" && typeof r.since === "number" && typeof r.reloads === "number" && Date.now() - (r.at ?? 0) < FRESH_MS) {
        return { id: r.id, since: r.since, reloads: r.reloads };
      }
    }
  } catch {
    // storage bloqueado ou valor inválido: sem retomada
  }
  clearResume();
  return null;
}

export function clearResume(): void {
  try {
    sessionStorage.removeItem(KEY);
  } catch {
    // nada a limpar
  }
}

function saveResume(r: Resume): boolean {
  try {
    sessionStorage.setItem(KEY, JSON.stringify({ ...r, at: Date.now() }));
    return true;
  } catch {
    return false;
  }
}

const sleep = (ms: number) => new Promise<void>((res) => window.setTimeout(res, ms));

function waitOnline(): Promise<void> {
  if (navigator.onLine) return Promise.resolve();
  return new Promise((res) => addEventListener("online", () => res(), { once: true }));
}

/** O servidor do jogo responde? (401 também vale: a página recarrega e mostra o login.) */
async function serverUp(): Promise<boolean> {
  try {
    const res = await fetch("/api/me", { cache: "no-store", credentials: "same-origin", signal: AbortSignal.timeout(4000) });
    return res.status < 500;
  } catch {
    return false;
  }
}

/**
 * Tenta voltar: espera (1 s, 2 s, 4 s… até 8 s) e a rede voltar, e recarrega quando o
 * servidor responde. Desiste (onGiveUp) com o servidor fora do ar por muito tempo ou
 * depois de várias recargas sem conseguir entrar.
 */
export async function reconnect(r: Resume, onGiveUp: () => void): Promise<void> {
  if (r.reloads < MAX_RELOADS) {
    for (let i = r.reloads; ; i++) {
      await sleep(Math.min(1000 * 2 ** i, MAX_DELAY_MS));
      await waitOnline();
      if (await serverUp()) {
        if (!saveResume({ ...r, reloads: r.reloads + 1 })) break; // sem storage, não dá para retomar
        location.reload();
        return;
      }
      if (Date.now() - r.since > GIVE_UP_MS) break;
    }
  }
  clearResume();
  onGiveUp();
}
