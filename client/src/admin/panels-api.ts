/** API dos painéis de leitura (Relatos, Métricas, Economia): /api/admin/bugs, metrics, economy. */

import { AdminError } from "./api";

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  // Mesmo cabeçalho do resto do portal: o servidor exige nos pedidos de escrita.
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
    const msg = res.status === 404 && !data.error ? "Acesso encerrado. Entre de novo pelo jogo." : (data.error ?? `Erro ${res.status}`);
    throw new AdminError(res.status, msg);
  }
  return data as T;
}

function qs(params: Record<string, string | number | undefined>): string {
  const u = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "" && v !== 0) u.set(k, String(v));
  const s = u.toString();
  return s ? `?${s}` : "";
}

// ---------- Relatos ----------

export interface Bug {
  id: number;
  at: string;
  accountId: number;
  characterId: number | null;
  characterName: string;
  level: number;
  x: number;
  z: number;
  region: string;
  version: string;
  userAgent: string;
  text: string;
  resolved: boolean;
  resolvedAt: string | null;
  resolvedBy: number | null;
  resolvedByName: string;
  resolvedNote: string;
}

export interface Facet {
  value: string;
  open: number;
  total: number;
}

export interface BugFacets {
  open: number;
  resolved: number;
  versions: Facet[];
  regions: Facet[];
}

export interface BugFilter {
  status?: string;
  version?: string;
  region?: string;
  q?: string;
  before?: number;
  limit?: number;
}

export interface ClientErrorRow {
  id: number;
  hash: string;
  kind: string;
  message: string;
  stack: string;
  count: number;
  versions: string[];
  firstSeen: string;
  lastSeen: string;
  userAgent: string;
  region: string;
}

export interface ClientErrorVersion {
  version: string;
  distinct: number;
  total: number;
  lastSeen: string;
}

// ---------- Métricas ----------

export interface TickSnapshot {
  since: string;
  until: string;
  ticks: number;
  avgMs: number;
  worstMs: number;
  budgetMs: number;
  slowTicks: number;
  eventsPerSec: number;
}

export interface LiveStats {
  online: number;
  byRegion: Record<string, number>;
  monsters: number;
  monstersDead: number;
  ground: number;
  lastTick: TickSnapshot | null;
  currentTick: TickSnapshot;
}

export interface Live {
  version: string;
  startedAt: string;
  uptimeSec: number;
  serverTime: string;
  regions: string[];
  zone: LiveStats | null;
}

export interface Metrics {
  days: number;
  since: string;
  gold: { day: string; source: string; in: number; out: number }[] | null;
  levels: { level: number; characters: number; avgMinutes: number }[] | null;
  deaths: { region: string; cause: string; deaths: number; avgLevel: number }[] | null;
  quit: { level: number; characters: number }[] | null;
  fps: { region: string; device: string; samples: number; avgFps: number; avgLow: number }[] | null;
  itemsDaily: { day: string; how: string; in: number; out: number }[] | null;
  itemsTop: { item: string; in: number; out: number }[] | null;
  serverDays: { day: string; kind: string; count: number }[] | null;
  serverTop: { kind: string; ref: string; count: number; last: string; where: string; message: string; version: string }[] | null;
  players: { day: string; characters: number; sessions: number; minutes: number }[] | null;
  timingsMs: Record<string, number>;
  slowestMs: Record<string, number>;
  queries: Record<string, number>;
  generatedAt: string;
}

// ---------- Economia ----------

export interface GoldSupply {
  bags: number;
  storage: number;
  total: number;
  characters: number;
  expected: number;
  surplus: number;
}

export interface ItemBalance {
  item: string;
  name: string;
  bags: number;
  storage: number;
  existing: number;
  in: number;
  out: number;
  kit: number;
  expected: number;
  surplus: number;
}

export interface Spike {
  characterId: number;
  name: string;
  day: string;
  level: number;
  what: "gold" | "items";
  amount: number;
  baseline: number;
  ratio: number;
}

export interface Economy {
  days: number;
  since: string;
  supply: GoldSupply;
  goldDaily: { day: string; in: number; out: number; net: number; supply: number }[] | null;
  items: ItemBalance[] | null;
  suspicious: number;
  spikes: Spike[] | null;
  timingsMs: Record<string, number>;
  slowestMs: Record<string, number>;
  queries: Record<string, number>;
  generatedAt: string;
}

export interface CharacterEconomy {
  days: number;
  since: string;
  character: { id: number; accountId: number; name: string; class: string; level: number; gold: number };
  history: { day: string; level: number; goldIn: number; goldOut: number; itemsIn: number; itemsOut: number }[];
  flows: { kind: string; ref: string; how: string; amount: number; times: number }[];
  spikes: Spike[];
  timingsMs: Record<string, number>;
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

export interface GMLogFilter {
  gm?: string;
  command?: string;
  target?: string;
  q?: string;
  days?: number;
  before?: number;
}

export const panelsApi = {
  bugs: (f: BugFilter) => call<{ bugs: Bug[]; facets?: BugFacets }>("GET", `/api/admin/bugs${qs({ ...f })}`),
  bug: (id: number) => call<{ bug: Bug; extra: Record<string, unknown> }>("GET", `/api/admin/bugs/${id}`),
  resolveBug: (id: number, resolved: boolean, note: string) =>
    call<{ bug: Bug }>("POST", `/api/admin/bugs/${id}/resolve`, { resolved, note }),
  clientErrors: (days: number, version: string) =>
    call<{ days: number; errors: ClientErrorRow[]; versions: ClientErrorVersion[] }>(
      "GET",
      `/api/admin/client-errors${qs({ days, version })}`,
    ),
  live: () => call<Live>("GET", "/api/admin/metrics/live"),
  metrics: (days: number) => call<Metrics>("GET", `/api/admin/metrics${qs({ days })}`),
  economy: (days: number) => call<Economy>("GET", `/api/admin/economy${qs({ days })}`),
  character: (name: string, days: number) => call<CharacterEconomy>("GET", `/api/admin/economy/character${qs({ name, days })}`),
  gmLog: (f: GMLogFilter) => call<{ entries: GMLogEntry[]; commands?: Facet[] }>("GET", `/api/admin/economy/gm-log${qs({ ...f })}`),
};
