/**
 * Motor de áudio em WebAudio puro (sem o áudio do Babylon): um AudioContext criado no
 * primeiro toque/clique, três canais (música, efeitos e ambiente) sob um volume geral, um
 * conjunto fixo de vozes para os efeitos e camadas em laço para música e ambiente.
 *
 * Tudo é tolerante a falhas: sem WebAudio (ou se o navegador recusar), as chamadas viram
 * nada e o jogo segue mudo.
 */

export type Bus = "music" | "sfx" | "ambient";

export interface Volumes {
  master: number;
  music: number;
  sfx: number;
  ambient: number;
  muted: boolean;
}

/** Efeitos tocando ao mesmo tempo, no máximo. */
export const MAX_VOICES = 8;

interface Voice {
  gain: GainNode;
  pan: StereoPannerNode | null;
  src: AudioBufferSourceNode | null;
  prio: number;
  /** ctx.currentTime em que começou e em que acaba. */
  from: number;
  until: number;
}

type Ctor = typeof AudioContext;
type OfflineCtor = typeof OfflineAudioContext;

const w = window as unknown as { webkitAudioContext?: Ctor; webkitOfflineAudioContext?: OfflineCtor };
const AudioCtor: Ctor | undefined = window.AudioContext ?? w.webkitAudioContext;
const OfflineCtorRef: OfflineCtor | undefined = window.OfflineAudioContext ?? w.webkitOfflineAudioContext;

/** Volume do controle (0–100) para ganho: curva quadrática, mais natural ao ouvido. */
const curve = (v: number) => Math.max(0, Math.min(1, v / 100)) ** 2;

export class AudioEngine {
  ctx: AudioContext | null = null;
  /** false se o navegador não tem WebAudio ou ele falhou: tudo vira nada. */
  available = !!AudioCtor;
  private master: GainNode | null = null;
  private buses: Partial<Record<Bus, GainNode>> = {};
  private voices: Voice[] = [];
  private vol: Volumes = { master: 80, music: 55, sfx: 75, ambient: 60, muted: false };
  private readyFns: (() => void)[] = [];
  private armed = false;

  /** Começa a esperar o primeiro gesto (política de autoplay) e pausa com a aba escondida. */
  arm(): void {
    if (this.armed || !this.available) return;
    this.armed = true;
    // Fica escutando sempre: no iPhone o áudio pode ser interrompido (ligação, outra aba)
    // e só volta dentro de um gesto. Quando já está tocando, o custo é ler um campo.
    const onGesture = (ev: Event) => {
      // Só eventos que o navegador aceita como gesto: no toque, o pointerdown não vale
      // (o Chrome reclamaria no console); o pointerup/touchend do mesmo toque liberam.
      if (ev.type === "pointerdown" && (ev as PointerEvent).pointerType !== "mouse") return;
      if (ev.type === "keydown" && (ev as KeyboardEvent).key === "Escape") return;
      this.unlock();
    };
    for (const ev of ["pointerdown", "pointerup", "touchend", "keydown", "click"]) {
      addEventListener(ev, onGesture, { capture: true, passive: true });
    }
    document.addEventListener("visibilitychange", () => {
      const ctx = this.ctx;
      if (!ctx) return;
      if (document.hidden) void ctx.suspend().catch(() => undefined);
      else void ctx.resume().catch(() => undefined);
    });
  }

  /** Já pode tocar. */
  get running(): boolean {
    return this.ctx?.state === "running";
  }

  /** Roda fn quando o áudio estiver liberado (na hora, se já estiver). */
  whenReady(fn: () => void): void {
    if (this.running) fn();
    else this.readyFns.push(fn);
  }

  private unlock(): void {
    if (!this.available || document.hidden) return;
    if (this.ctx?.state === "running") return;
    try {
      if (!this.ctx) this.create();
      const ctx = this.ctx!;
      void ctx
        .resume()
        .then(() => this.flushReady())
        .catch(() => undefined);
      // iOS antigo: tocar um buffer vazio dentro do gesto destrava a saída de som.
      const b = ctx.createBufferSource();
      b.buffer = ctx.createBuffer(1, 1, 22050);
      b.connect(ctx.destination);
      b.start(0);
    } catch (err) {
      console.warn("áudio indisponível", err);
      this.available = false;
    }
  }

  private flushReady(): void {
    if (!this.running) return;
    for (const fn of this.readyFns.splice(0)) {
      try {
        fn();
      } catch (err) {
        console.warn("áudio:", err);
      }
    }
  }

  private create(): void {
    let ctx: AudioContext;
    try {
      ctx = new AudioCtor!({ latencyHint: "interactive" });
    } catch {
      ctx = new AudioCtor!(); // Safari antigo não aceita opções
    }
    this.ctx = ctx;
    ctx.onstatechange = () => this.flushReady();
    this.master = ctx.createGain();
    this.master.connect(ctx.destination);
    for (const b of ["music", "sfx", "ambient"] as Bus[]) {
      const g = ctx.createGain();
      g.connect(this.master);
      this.buses[b] = g;
    }
    // Vozes fixas dos efeitos: ganho → pan → canal. Cada golpe só cria a fonte (o nó de
    // leitura do buffer, que no WebAudio é de uso único e muito barato).
    const canPan = typeof ctx.createStereoPanner === "function";
    for (let i = 0; i < MAX_VOICES; i++) {
      const gain = ctx.createGain();
      const pan = canPan ? ctx.createStereoPanner() : null;
      if (pan) gain.connect(pan).connect(this.buses.sfx!);
      else gain.connect(this.buses.sfx!);
      this.voices.push({ gain, pan, src: null, prio: 0, from: 0, until: 0 });
    }
    this.applyVolumes();
  }

  setVolumes(v: Volumes): void {
    this.vol = v;
    this.applyVolumes();
  }

  private applyVolumes(): void {
    const ctx = this.ctx;
    if (!ctx || !this.master) return;
    const t = ctx.currentTime;
    this.master.gain.setTargetAtTime(this.vol.muted ? 0 : curve(this.vol.master), t, 0.04);
    this.buses.music!.gain.setTargetAtTime(curve(this.vol.music), t, 0.04);
    this.buses.sfx!.gain.setTargetAtTime(curve(this.vol.sfx), t, 0.04);
    this.buses.ambient!.gain.setTargetAtTime(curve(this.vol.ambient), t, 0.04);
  }

  /** Volume final de um canal (0 = não vale a pena tocar nem carregar nada). */
  level(bus: Bus): number {
    if (this.vol.muted || !this.available) return 0;
    return curve(this.vol.master) * curve(this.vol[bus]);
  }

  /** Vozes tocando agora (testes). */
  busyVoices(): number {
    const now = this.ctx?.currentTime ?? 0;
    return this.voices.filter((v) => v.until > now).length;
  }

  bus(b: Bus): GainNode | null {
    return this.buses[b] ?? null;
  }

  /**
   * Decodifica um arquivo. Antes do primeiro gesto usa um OfflineAudioContext (que não
   * depende da política de autoplay), na taxa pedida; o buffer serve para qualquer contexto.
   */
  async decode(bytes: ArrayBuffer, rate: number): Promise<AudioBuffer> {
    if (OfflineCtorRef) {
      try {
        const off = new OfflineCtorRef(1, 1, rate);
        return await decodeWith(off, bytes.slice(0));
      } catch {
        // cai para o contexto principal
      }
    }
    const ctx = await new Promise<AudioContext>((res) => this.whenReady(() => res(this.ctx!)));
    return decodeWith(ctx, bytes);
  }

  /**
   * Toca um trecho de um buffer numa das vozes. Sem voz livre, toma a de menor prioridade
   * (pesando o quanto falta para ela acabar); se todas valem mais, o som é descartado.
   */
  play(buf: AudioBuffer, offset: number, dur: number, gain: number, pan: number, rate: number, prio: number): void {
    const ctx = this.ctx;
    if (!ctx || ctx.state !== "running" || gain <= 0.001) return;
    const now = ctx.currentTime;
    let pick: Voice | null = null;
    let worst = Infinity;
    for (const v of this.voices) {
      if (v.until <= now) {
        pick = v;
        break;
      }
      const left = (v.until - now) / Math.max(0.01, v.until - v.from);
      const score = v.prio * (0.3 + 0.7 * left);
      if (score < worst) {
        worst = score;
        pick = v;
      }
    }
    if (!pick || (pick.until > now && worst >= prio)) return;
    const v = pick;
    let at = now;
    const g = v.gain.gain;
    g.cancelScheduledValues(now);
    if (v.src) {
      // Tomando uma voz que ainda toca: some em 12 ms (sem estalo) e o novo entra depois.
      const old = v.src;
      old.onended = null;
      g.setValueAtTime(g.value, now);
      g.linearRampToValueAtTime(0, now + 0.012);
      try {
        old.stop(now + 0.013);
      } catch {
        // já tinha parado
      }
      at = now + 0.015;
    }
    g.setValueAtTime(gain, at);
    if (v.pan) v.pan.pan.setValueAtTime(Math.max(-1, Math.min(1, pan)), at);
    const src = ctx.createBufferSource();
    src.buffer = buf;
    src.playbackRate.value = rate;
    src.connect(v.gain);
    src.onended = () => {
      src.disconnect();
      if (v.src === src) {
        v.src = null;
        v.until = 0;
      }
    };
    src.start(at, offset, dur);
    v.src = src;
    v.prio = prio;
    v.from = at;
    v.until = at + dur / rate;
  }

  /** Toca um buffer inteiro num canal, fora das vozes (sons raros do ambiente). */
  playLoose(buf: AudioBuffer, offset: number, dur: number, bus: Bus, gain: number, pan: number): void {
    const ctx = this.ctx;
    const out = this.buses[bus];
    if (!ctx || !out || ctx.state !== "running") return;
    const src = ctx.createBufferSource();
    src.buffer = buf;
    const g = ctx.createGain();
    g.gain.value = gain;
    let tail: AudioNode = g;
    if (typeof ctx.createStereoPanner === "function") {
      const p = ctx.createStereoPanner();
      p.pan.value = pan;
      g.connect(p);
      tail = p;
    }
    tail.connect(out);
    src.connect(g);
    src.onended = () => {
      src.disconnect();
      g.disconnect();
      tail.disconnect();
    };
    src.start(ctx.currentTime, offset, dur);
  }
}

function decodeWith(ctx: BaseAudioContext, bytes: ArrayBuffer): Promise<AudioBuffer> {
  // Safari antigo só tem a versão com callbacks.
  return new Promise((res, rej) => {
    const p = ctx.decodeAudioData(bytes, res, rej) as Promise<AudioBuffer> | undefined;
    p?.then(res, rej);
  });
}

/**
 * Um som em laço (música ou ambiente) com emenda suave: cada volta começa antes da
 * anterior acabar, uma sumindo e a outra entrando (funciona com qualquer arquivo, mesmo
 * os MP3 com silêncio de codificação nas pontas). O ganho próprio faz a troca entre
 * camadas (crossfade de região).
 */
export class LoopLayer {
  readonly out: GainNode;
  private next = 0;
  private timer = 0;
  private playing: { src: AudioBufferSourceNode; g: GainNode }[] = [];
  private stopped = false;

  constructor(
    private ctx: AudioContext,
    dest: AudioNode,
    private buf: AudioBuffer,
    private seam: number,
  ) {
    this.out = ctx.createGain();
    this.out.gain.value = 0;
    this.out.connect(dest);
  }

  /** Entra em `fade` segundos até o volume `gain`. */
  start(gain: number, fade: number): void {
    const t = this.ctx.currentTime;
    this.out.gain.setValueAtTime(0, t);
    this.out.gain.linearRampToValueAtTime(gain, t + fade);
    this.next = t + 0.02;
    this.schedule();
  }

  setGain(gain: number): void {
    this.out.gain.setTargetAtTime(gain, this.ctx.currentTime, 0.3);
  }

  /** Some em `fade` segundos e libera tudo. */
  stop(fade: number): void {
    if (this.stopped) return;
    this.stopped = true;
    clearTimeout(this.timer);
    const t = this.ctx.currentTime;
    const g = this.out.gain;
    g.cancelScheduledValues(t);
    g.setValueAtTime(g.value, t);
    g.linearRampToValueAtTime(0, t + fade);
    for (const p of this.playing) {
      try {
        p.src.stop(t + fade + 0.05);
      } catch {
        // já parou
      }
    }
    setTimeout(() => this.out.disconnect(), (fade + 0.2) * 1000);
  }

  private schedule(): void {
    if (this.stopped) return;
    const ctx = this.ctx;
    const d = this.buf.duration;
    const f = Math.min(this.seam, d / 4);
    // Ficou para trás (aba pausada, travada): recomeça agora, entrando suave.
    const t = Math.max(this.next, ctx.currentTime + 0.02);
    const src = ctx.createBufferSource();
    src.buffer = this.buf;
    const g = ctx.createGain();
    g.gain.setValueAtTime(0, t);
    g.gain.linearRampToValueAtTime(1, t + f);
    g.gain.setValueAtTime(1, t + d - f);
    g.gain.linearRampToValueAtTime(0, t + d);
    src.connect(g).connect(this.out);
    const entry = { src, g };
    this.playing.push(entry);
    src.onended = () => {
      src.disconnect();
      g.disconnect();
      this.playing = this.playing.filter((p) => p !== entry);
    };
    src.start(t);
    src.stop(t + d + 0.05);
    this.next = t + d - f;
    // Agenda a próxima volta um pouco antes da emenda (o horário exato é do áudio).
    this.timer = window.setTimeout(() => this.schedule(), Math.max(0.2, this.next - ctx.currentTime - 1.5) * 1000);
  }
}
