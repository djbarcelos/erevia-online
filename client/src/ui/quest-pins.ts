/**
 * Missões fixadas no painel do HUD (até 3) e a rastreada (uma só, sempre entre as fixadas).
 *
 * - Missão nova: entra sozinha se houver menos de 3 fixadas; senão aparece no painel por
 *   NEW_QUEST_MS com o botão "Fixar" e some se não for fixada.
 * - Missão concluída sai do painel; se era a rastreada, a nova que chegou junto (a próxima
 *   da sequência) vira a rastreada.
 * - Fixar com o painel cheio tira a fixada mais antiga que não é a rastreada.
 * - A escolha fica no navegador por personagem e sobrevive a relogar. "seen" guarda as
 *   ativas da última vez: a missão que o jogador desafixou não volta sozinha, e a que chegou
 *   com ele fora (nível novo ao entrar) conta como nova.
 *
 * Só lógica, sem DOM e sem importar nada em tempo de execução: os testes rodam no Node
 * (client/tests/quest-pins.test.ts). Nada aqui roda por quadro.
 */

export const MAX_PINS = 3;
/** Quanto tempo a missão recém-aceita fica no painel esperando o "Fixar". */
export const NEW_QUEST_MS = 8000;

/** Onde a escolha fica guardada (o texto JSON; qualquer chamada pode falhar). */
export interface PinStorage {
  load(): string | null;
  save(raw: string): void;
}

export interface SavedPins {
  pins: string[];
  track: string | null;
  seen: string[];
}

const MAX_ID = 64;
const MAX_SEEN = 200;

const ids = (v: unknown): string[] =>
  Array.isArray(v) ? [...new Set(v.filter((s): s is string => typeof s === "string" && s.length > 0 && s.length <= MAX_ID))] : [];

/** Lê o que foi guardado; qualquer coisa estragada vira null (começa do zero, sem erro). */
export function parsePins(raw: string | null): SavedPins | null {
  if (!raw) return null;
  let v: unknown;
  try {
    v = JSON.parse(raw);
  } catch {
    return null;
  }
  if (!v || typeof v !== "object" || Array.isArray(v)) return null;
  const o = v as Record<string, unknown>;
  const track = typeof o.track === "string" && o.track.length <= MAX_ID ? o.track : null;
  return { pins: ids(o.pins).slice(0, MAX_PINS), track, seen: ids(o.seen).slice(0, MAX_SEEN) };
}

export class QuestPins {
  private pins: string[] = [];
  private trackId: string | null = null;
  /** Recém-aceitas esperando o "Fixar": id → até quando aparecem. */
  private temps = new Map<string, number>();
  private active: string[] = [];
  private started = false;
  private lastSaved = "";
  private store: PinStorage | null;
  private now: () => number;
  private legacyTrack: string | null;

  /** legacyTrack: rastreada do esquema antigo (uma chave só, sem personagem); vale se não
   *  houver nada salvo deste personagem. */
  constructor(store: PinStorage | null, now: () => number, legacyTrack: string | null = null) {
    this.store = store;
    this.now = now;
    this.legacyTrack = legacyTrack;
  }

  /** Missões ativas de um QuestLog novo, na ordem do servidor. */
  update(list: readonly string[]): void {
    const now = new Set(list);
    let known: Set<string>;
    let temps = true;
    if (!this.started) {
      this.started = true;
      let raw: string | null = null;
      try {
        raw = this.store?.load() ?? null;
      } catch {
        raw = null; // armazenamento bloqueado: começa do zero
      }
      const saved = parsePins(raw);
      known = new Set(saved?.seen ?? []);
      if (saved) {
        this.pins = saved.pins;
        this.trackId = saved.track;
        if (this.trackId && now.has(this.trackId) && !this.pins.includes(this.trackId)) this.pins = [this.trackId, ...this.pins].slice(0, MAX_PINS);
      } else {
        // Primeira vez neste navegador: sem recém-aceitas piscando, só enche o painel.
        temps = false;
        const legacy = this.legacyTrack;
        if (legacy && now.has(legacy)) {
          this.pins = [legacy];
          this.trackId = legacy;
        }
      }
    } else known = new Set(this.active);

    // Saíram (concluídas): deixam o painel.
    const trackGone = this.trackId !== null && !now.has(this.trackId);
    this.pins = this.pins.filter((id) => now.has(id));
    for (const id of [...this.temps.keys()]) if (!now.has(id)) this.temps.delete(id);

    // Chegaram: entram sozinhas enquanto houver lugar; senão ficam um pouco com o "Fixar".
    const fresh = list.filter((id) => !known.has(id));
    for (const id of fresh) {
      if (this.pins.includes(id)) continue;
      if (this.pins.length < MAX_PINS) this.pins.push(id);
      else if (temps) this.temps.set(id, this.now() + NEW_QUEST_MS);
    }

    if (trackGone || (this.trackId !== null && !this.pins.includes(this.trackId))) {
      this.trackId = fresh.find((id) => this.pins.includes(id)) ?? null;
    }
    this.trackId ??= this.pins[0] ?? null;
    this.active = [...list];
    this.save();
  }

  /** Fixadas, da mais antiga para a mais nova. */
  pinned(): readonly string[] {
    return this.pins;
  }

  tracked(): string | null {
    return this.trackId;
  }

  isPinned(id: string): boolean {
    return this.pins.includes(id);
  }

  /** Recém-aceita ainda esperando o "Fixar". */
  isTemp(id: string): boolean {
    const t = this.temps.get(id);
    return t !== undefined && t > this.now();
  }

  /** Quantos ms faltam para a recém-aceita sumir (0 = não é recém-aceita). */
  tempLeft(id: string): number {
    const t = this.temps.get(id);
    return t === undefined ? 0 : Math.max(0, t - this.now());
  }

  /** Aparece no painel: fixada ou recém-aceita. */
  visible(id: string): boolean {
    return this.isPinned(id) || this.isTemp(id);
  }

  /** A fixada que sairia para dar lugar a uma nova (null = há lugar). */
  wouldEvict(): string | null {
    if (this.pins.length < MAX_PINS) return null;
    return this.pins.find((p) => p !== this.trackId) ?? null;
  }

  /** Fixa. Devolve a que saiu para dar lugar (null = nenhuma). */
  pin(id: string): string | null {
    if (!this.active.includes(id) || this.pins.includes(id)) return null;
    this.temps.delete(id);
    const out = this.wouldEvict();
    if (out) this.pins = this.pins.filter((p) => p !== out);
    this.pins.push(id);
    this.trackId ??= id;
    this.save();
    return out;
  }

  /** Desafixa. Se era a rastreada, a próxima fixada assume (ou nenhuma). */
  unpin(id: string): void {
    this.temps.delete(id);
    if (!this.pins.includes(id)) return;
    this.pins = this.pins.filter((p) => p !== id);
    if (this.trackId === id) this.trackId = this.pins[0] ?? null;
    this.save();
  }

  /** Rastreia (e fixa, se não estava). Devolve a que saiu do painel para dar lugar. */
  track(id: string): string | null {
    if (!this.active.includes(id)) return null;
    const out = this.pin(id);
    this.trackId = id;
    this.save();
    return out;
  }

  /** Resumo do estado (muda quando muda o que o painel mostra). */
  key(): string {
    return `${this.pins.join("|")}#${this.trackId ?? ""}#${[...this.temps.keys()].join("|")}`;
  }

  /** Daqui a quantos ms a próxima recém-aceita some (null = nenhuma esperando). */
  nextExpiry(): number | null {
    let min: number | null = null;
    const now = this.now();
    for (const t of this.temps.values()) min = min === null ? t - now : Math.min(min, t - now);
    return min === null ? null : Math.max(0, min);
  }

  /** Tira as recém-aceitas que passaram do tempo. Devolve se alguma saiu. */
  expire(): boolean {
    const now = this.now();
    let changed = false;
    for (const [id, t] of [...this.temps]) {
      if (t <= now) {
        this.temps.delete(id);
        changed = true;
      }
    }
    return changed;
  }

  private save(): void {
    const raw = JSON.stringify({ pins: this.pins, track: this.trackId, seen: this.active.slice(0, MAX_SEEN) } satisfies SavedPins);
    if (raw === this.lastSaved) return; // o QuestLog chega a cada monstro: só grava quando muda
    this.lastSaved = raw;
    try {
      this.store?.save(raw);
    } catch {
      // armazenamento cheio ou bloqueado: vale só nesta sessão
    }
  }
}
