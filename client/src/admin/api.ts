/** Cliente da API do portal (/api/admin/*). Mesma origem; a sessão vai no cookie HttpOnly. */

export interface Summary {
  accounts: number;
  accounts24h: number;
  characters: number;
  characters24h: number;
  bugs24h: number;
  bugsOpen: number;
}

export interface Overview {
  /** null: a zona não respondeu no prazo. */
  online: number | null;
  version: string;
  startedAt: string;
  uptimeSec: number;
  serverTime: string;
  summary: Summary;
}

export interface GMLogEntry {
  id: number;
  at: string;
  gmAccountId: number;
  gmName: string;
  command: string;
  target: string;
  detail: string;
}

export class AdminError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

/**
 * Todo pedido leva o cabeçalho X-Erevia-Admin: o servidor exige nos de escrita (contra
 * CSRF: outra origem não consegue mandá-lo sem preflight) e ignora nos de leitura.
 */
async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { "X-Erevia-Admin": "1" };
  if (body !== undefined) headers["Content-Type"] = "application/json";
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers,
      credentials: "same-origin",
      cache: "no-store",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
  } catch {
    throw new AdminError(0, "Sem conexão com o servidor.");
  }
  const data = (await res.json().catch(() => ({}))) as { error?: string };
  if (!res.ok) {
    // 404 aqui = a sessão caiu ou a conta deixou de ser GM (o servidor não diz qual).
    const msg = res.status === 404 ? "Acesso encerrado. Entre de novo pelo jogo." : (data.error ?? `Erro ${res.status}`);
    throw new AdminError(res.status, msg);
  }
  return data as T;
}

export const adminApi = {
  me: () => call<{ accountId: number; displayName: string }>("GET", "/api/admin/me"),
  overview: () => call<Overview>("GET", "/api/admin/overview"),
  gmLog: (limit = 10) => call<{ entries: GMLogEntry[] }>("GET", `/api/admin/gm-log?limit=${limit}`),
};
