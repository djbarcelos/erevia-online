/**
 * "Ir" das missões: o personagem anda sozinho até o lugar do passo (o mesmo comando do
 * clique no mapa do mundo; o servidor calcula o caminho e valida tudo) e, numa área de
 * caça, oferece "Caçar aqui": a auto-caça só com os monstros do passo, até ele terminar.
 *
 * Para sozinho quando o jogador mexe no movimento ou ataca, quando chega, quando leva
 * dano (a não ser que "Continuar andando se atacado" esteja ligado), quando a janela sai
 * de foco e depois de 5 minutos sem nenhuma interação real (contra bot).
 *
 * FPS em primeiro lugar: nada por quadro. Um intervalo de 250 ms roda só enquanto o "Ir"
 * ou o "Caçar aqui" estão ativos. Este arquivo não importa nada em tempo de execução (só
 * tipos), para os testes rodarem no Node (client/tests/questgo.test.ts).
 */

/** Para onde o passo da missão manda. */
export interface GoTarget {
  questId: string;
  /** Índice do passo (mudou = o passo terminou). */
  step: number;
  x: number;
  z: number;
  radius: number;
  name: string;
  /** Área de caça: chegar oferece "Caçar aqui". */
  hunt: boolean;
  /** Nomes dos monstros que contam (vazio = qualquer). */
  monsters: string[];
}

/** Uma Pedra de Teleporte e os destinos dela (preço = o de chegar lá, como no servidor). */
export interface StoneInfo {
  npcId: number;
  name: string;
  x: number;
  z: number;
  dests: { id: number; name: string; x: number; z: number; price: number }[];
}

export interface StoneOffer {
  npcId: number;
  from: string;
  destId: number;
  to: string;
  /** Pedra de partida e ponto de chegada (a pedra de destino). */
  fromX: number;
  fromZ: number;
  x: number;
  z: number;
  price: number;
  afford: boolean;
  /** Metros a pé pela pedra (até ela + do destino ao lugar) e direto. */
  walk: number;
  direct: number;
}

export type GoPhase = "idle" | "walking" | "toStone" | "arrived" | "hunting";

export interface QuestGoDeps {
  now(): number;
  position(): { x: number; z: number } | null;
  dead(): boolean;
  /** Janela do jogo em foco e visível. */
  focused(): boolean;
  keepIfHit(): boolean;
  gold(): number;
  /** Anda até (x, z): o mesmo comando do clique no mapa do mundo. */
  walk(x: number, z: number): void;
  /** Para de andar (como soltar o WASD). */
  halt(): void;
  /** Vai até a pedra e abre a janela dela com o destino sugerido (não viaja sozinho). */
  openStone(npcId: number, destId: number): void;
  startHunt(target: GoTarget): void;
  stopHunt(): void;
  /** A auto-caça do "Caçar aqui" ainda está ligada. */
  hunting(): boolean;
  notify(text: string, warn: boolean): void;
  /** Estado mudou (a pílula do HUD redesenha só o que mudou). */
  changed(): void;
  /** Repete fn a cada ms; devolve o cancelamento. */
  every(fn: () => void, ms: number): () => void;
}

/** Acima disso (em linha reta) uma pedra que encurte o caminho é oferecida. */
export const STONE_OFFER_DIST = 150;
/** A pedra precisa poupar pelo menos esta fração do caminho a pé... */
export const STONE_MIN_SAVE = 0.25;
/** ...e pelo menos estes metros. */
export const STONE_MIN_SAVE_M = 60;
/** Sem nenhuma interação real do jogador por este tempo, para. */
export const IDLE_LIMIT_MS = 5 * 60_000;
/** Sem sair do lugar por este tempo, manda o comando de andar de novo. */
export const STALL_MS = 3000;
/** Tentativas sem sair do lugar antes de desistir. */
export const MAX_RETRIES = 4;
/** Perto assim da pedra de destino = viajou. */
const ARRIVED_STONE_R = 8;
/** Perto assim da pedra de partida = chegou nela (a janela abre a 4,5 m). */
const AT_STONE_R = 5;
const TICK_MS = 250;

const dist = (a: { x: number; z: number }, b: { x: number; z: number }) => Math.hypot(a.x - b.x, a.z - b.z);

/** Raio de chegada: a área de caça inteira; NPC, loja, pedra e portão com uma folga. */
export function arriveRadius(t: GoTarget): number {
  return t.hunt ? Math.max(1, t.radius) : Math.max(t.radius, 3.5);
}

/**
 * Pedra de Teleporte que encurta o caminho: a combinação (pedra de partida, destino) que
 * deixa menos chão a pé. Só para lugares a mais de STONE_OFFER_DIST e se poupar o bastante.
 */
export function stoneOffer(me: { x: number; z: number }, t: { x: number; z: number }, stones: StoneInfo[], gold: number): StoneOffer | null {
  const direct = dist(me, t);
  if (direct <= STONE_OFFER_DIST) return null;
  let best: StoneOffer | null = null;
  for (const s of stones) {
    const toStone = dist(me, s);
    for (const d of s.dests) {
      const walk = toStone + dist(d, t);
      if (!best || walk < best.walk) {
        best = { npcId: s.npcId, from: s.name, fromX: s.x, fromZ: s.z, destId: d.id, to: d.name, x: d.x, z: d.z, price: d.price, afford: gold >= d.price, walk, direct };
      }
    }
  }
  if (!best || best.walk > direct * (1 - STONE_MIN_SAVE) || direct - best.walk < STONE_MIN_SAVE_M) return null;
  return best;
}

/** O monstro de nome `name` serve de alvo para o filtro (vazio/null = qualquer um). */
export function huntAllows(names: readonly string[] | null, name: string | undefined): boolean {
  return !names || names.length === 0 || (name !== undefined && names.includes(name));
}

/** O mais perto da lista que passa no filtro. */
export function nearestAllowed<T extends { name: string; dist: number }>(list: Iterable<T>, names: readonly string[] | null): T | null {
  let best: T | null = null;
  for (const m of list) if (huntAllows(names, m.name) && (!best || m.dist < best.dist)) best = m;
  return best;
}

export class QuestGo {
  phase: GoPhase = "idle";
  target: GoTarget | null = null;
  offer: StoneOffer | null = null;
  /** Distância até o lugar no último tick (para a pílula). */
  distance = 0;
  private d: QuestGoDeps;
  private lastInput: number;
  private lastProgress = 0;
  private lastSend = 0;
  private lastPos: { x: number; z: number } | null = null;
  private retries = 0;
  private cancelTimer: (() => void) | null = null;

  constructor(deps: QuestGoDeps) {
    this.d = deps;
    this.lastInput = deps.now();
  }

  get active(): boolean {
    return this.phase !== "idle";
  }

  /** Interação real do jogador (tecla, clique, toque, roda do mouse). */
  input(): void {
    this.lastInput = this.d.now();
  }

  /** Botão "Ir". */
  go(t: GoTarget, stones: StoneInfo[]): void {
    const d = this.d;
    this.input();
    if (!d.focused()) return d.notify("O Ir só funciona com a janela do jogo em foco.", true);
    if (d.dead()) return d.notify("Você está morto.", true);
    const me = d.position();
    if (!me) return;
    if (d.hunting()) d.stopHunt();
    this.target = t;
    this.distance = dist(me, t);
    if (this.distance <= arriveRadius(t)) return this.arrive();
    this.offer = stoneOffer(me, t, stones, d.gold());
    this.phase = "walking";
    this.startTimer();
    this.resetProgress();
    this.send();
    d.changed();
  }

  /** Botão "Viajar pela Pedra": anda até a pedra e abre a janela (quem escolhe é o jogador). */
  takeStone(): void {
    const o = this.offer;
    if (this.phase !== "walking" || !o) return;
    this.input();
    this.phase = "toStone";
    this.resetProgress();
    this.d.openStone(o.npcId, o.destId);
    this.d.changed();
  }

  /** Botão "Caçar aqui". */
  huntHere(): void {
    const t = this.target;
    if (this.phase !== "arrived" || !t) return;
    this.input();
    if (!this.d.focused()) return this.d.notify("O Caçar aqui só funciona com a janela do jogo em foco.", true);
    this.phase = "hunting";
    this.d.startHunt(t);
    this.startTimer();
    this.d.changed();
  }

  /** Para tudo (reason vazio = sem aviso). */
  stop(reason = ""): void {
    if (this.phase === "idle") return;
    const was = this.phase;
    this.phase = "idle";
    this.offer = null;
    this.cancelTimer?.();
    this.cancelTimer = null;
    if (was === "walking" || was === "toStone") this.d.halt();
    if (was === "hunting" && this.d.hunting()) this.d.stopHunt();
    if (reason) this.d.notify(reason, true);
    this.d.changed();
  }

  /** O jogador andou (WASD, joystick, clique no chão ou no mapa) ou atacou: ele assumiu. */
  onPlayerControl(): void {
    this.input();
    if (this.phase === "walking" || this.phase === "toStone") {
      // Não manda parar: o comando do jogador já está a caminho do servidor.
      this.phase = "idle";
      this.offer = null;
      this.cancelTimer?.();
      this.cancelTimer = null;
      this.d.changed();
    }
  }

  /** O jogador levou dano. */
  onHit(damage: number): void {
    if (damage <= 0 || (this.phase !== "walking" && this.phase !== "toStone")) return;
    if (this.d.keepIfHit()) return;
    this.stop("Ir parou: você foi atacado.");
  }

  /** QuestLog novo: o alvo de agora da mesma missão (null = a missão acabou ou ficou sem lugar). */
  questUpdate(next: GoTarget | null): void {
    const t = this.target;
    if (this.phase === "idle" || !t) return;
    if (!next || next.step !== t.step) {
      const msg = `Passo concluído: ${t.name}.`;
      return this.stop(this.phase === "hunting" ? `${msg} O Caçar aqui parou.` : msg);
    }
    // Mesmo passo, lugar novo (a área de caça muda com o nível): segue para o novo.
    const moved = dist(next, t) > 1;
    this.target = next;
    if (moved && this.phase === "walking") {
      this.resetProgress();
      this.send();
    }
  }

  /** Um passo de 250 ms (o intervalo só existe enquanto ativo; os testes chamam direto). */
  tick(): void {
    const d = this.d;
    const t = this.target;
    if (this.phase === "idle" || !t) return;
    if (d.dead()) return this.stop("Ir parou: você morreu.");
    if (!d.focused()) return this.stop("Ir parou: a janela do jogo saiu de foco.");
    if (d.now() - this.lastInput > IDLE_LIMIT_MS) return this.stop("Ir parou: 5 minutos sem você mexer no jogo.");
    const me = d.position();
    if (!me) return;
    this.distance = dist(me, t);
    switch (this.phase) {
      case "hunting":
        if (!d.hunting()) {
          // A auto-caça parou por conta própria (F, morte, sem poção): já avisou.
          this.phase = "idle";
          this.cancelTimer?.();
          this.cancelTimer = null;
        }
        break;
      case "arrived":
        if (this.distance > arriveRadius(t) + 8) {
          this.phase = "idle"; // saiu da área: a oferta some
          this.cancelTimer?.();
          this.cancelTimer = null;
        }
        break;
      case "walking":
        if (this.distance <= arriveRadius(t)) {
          d.halt();
          this.arrive();
          break;
        }
        this.watchProgress(me, () => this.send());
        break;
      case "toStone": {
        const o = this.offer;
        if (!o) break;
        if (dist(me, o) <= ARRIVED_STONE_R) {
          // Viajou: segue a pé do destino até o lugar.
          this.offer = null;
          this.phase = "walking";
          this.resetProgress();
          this.send();
          break;
        }
        if (dist(me, { x: o.fromX, z: o.fromZ }) <= AT_STONE_R) {
          this.resetProgress(); // esperando o jogador escolher na janela da pedra
          break;
        }
        this.watchProgress(me, () => {
          this.lastSend = d.now();
          d.openStone(o.npcId, o.destId);
        });
        break;
      }
    }
    d.changed();
  }

  private arrive(): void {
    const t = this.target!;
    this.offer = null;
    if (t.hunt) {
      this.phase = "arrived";
      this.startTimer();
      this.d.notify(`Você chegou: ${t.name}. Use "Caçar aqui" para caçar só os monstros da missão.`, false);
    } else {
      this.phase = "idle";
      this.cancelTimer?.();
      this.cancelTimer = null;
      this.d.notify(`Você chegou: ${t.name}.`, false);
    }
    this.d.changed();
  }

  private send(): void {
    const t = this.target!;
    this.lastSend = this.d.now();
    this.d.walk(t.x, t.z);
  }

  private resetProgress(): void {
    this.lastProgress = this.d.now();
    this.lastPos = this.d.position();
    this.retries = 0;
  }

  /** Andou desde a última olhada? Parado há STALL_MS: reenvia; depois de MAX_RETRIES, desiste. */
  private watchProgress(me: { x: number; z: number }, resend: () => void): void {
    const now = this.d.now();
    if (!this.lastPos || dist(me, this.lastPos) > 0.5) {
      this.lastPos = { x: me.x, z: me.z };
      this.lastProgress = now;
      this.retries = 0;
      return;
    }
    if (now - this.lastProgress < STALL_MS || now - this.lastSend < STALL_MS) return;
    if (++this.retries > MAX_RETRIES) return this.stop("Ir parou: não encontrei caminho até lá.");
    this.lastProgress = now;
    resend();
  }

  private startTimer(): void {
    if (this.cancelTimer) return;
    this.resetProgress();
    this.cancelTimer = this.d.every(() => this.tick(), TICK_MS);
  }
}
