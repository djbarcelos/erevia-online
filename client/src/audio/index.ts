/**
 * Som do jogo: golpes, skills, mortes, passos, interface, ambiente e música por região.
 *
 * - Efeitos: um arquivo só (public/audio/sfx.mp3, ~330 KB) carregado e decodificado uma vez;
 *   cada som é um trecho dele (sprite.json). No máximo MAX_VOICES tocam juntos, e quando
 *   falta voz ganha o mais alto/mais perto (os do jogador local valem mais).
 * - Outros jogadores e monstros só são ouvidos até HEAR_DISTANCE, com volume pela distância
 *   e pan esquerda/direita pela câmera (2D calculado à mão, mais barato que áudio 3D).
 * - Música e ambiente carregam sob demanda (só a região e a hora atuais) e trocam com
 *   crossfade. Volumes e mudo seguem as Configurações → Áudio na hora.
 * - Sem WebAudio, ou antes do primeiro toque/clique (política de autoplay), tudo é ignorado.
 */

import type { Scene } from "@babylonjs/core";
import { settings, type Settings } from "../ui/kit";
import { RAID_LAIR, regionAt, type Region } from "../ui/regions";
import { isNight } from "../world/daynight";
import { AudioEngine, LoopLayer, type Bus } from "./engine";
import SPRITE from "./sprite.json";

/** Até onde se ouvem golpes, skills e mortes dos outros (m). */
export const HEAR_DISTANCE = 25;
/** Até esta distância o som não perde volume (m). */
const NEAR = 2;
/** Mesmo som de novo antes disto (ms) é ignorado (skills em área acertam vários juntos). */
const REPEAT_MS = 40;
/** Distância entre passos (m) e velocidade mínima para contar como andando (m/s). */
const STRIDE = 1.7;
const WALK_SPEED = 1;
const BASE = "/audio/";

type Clip = keyof typeof SPRITE;

interface SoundDef {
  clips: Clip[];
  gain: number;
  /** Velocidade de reprodução (tom) e quanto ela varia ao acaso (±). */
  rate?: number;
  jitter?: number;
}

const SOUNDS = {
  // Golpes
  hit: { clips: ["hit1", "hit2", "hit3"], gain: 0.62, jitter: 0.08 },
  crit: { clips: ["crit1", "crit2"], gain: 0.85, jitter: 0.05 },
  miss: { clips: ["miss1", "miss2", "miss3"], gain: 0.5, jitter: 0.1 },
  arcane: { clips: ["arcane"], gain: 0.38, jitter: 0.12 },
  slice: { clips: ["slice1", "slice2"], gain: 0.6, jitter: 0.06 },
  heavy: { clips: ["heavy", "chop"], gain: 0.85, rate: 0.92, jitter: 0.04 },
  bash: { clips: ["bash"], gain: 0.8, jitter: 0.05 },
  // Skills
  spell: { clips: ["spell"], gain: 0.5, jitter: 0.06 },
  bolt: { clips: ["bolt"], gain: 0.42, jitter: 0.08 },
  curse: { clips: ["bolt"], gain: 0.45, rate: 0.75 },
  fireCast: { clips: ["fireCast"], gain: 0.6, jitter: 0.05 },
  fire: { clips: ["fireHit", "fireHit2"], gain: 0.55, jitter: 0.05 },
  boom: { clips: ["boom"], gain: 0.8, jitter: 0.05 },
  ice: { clips: ["ice", "ice2"], gain: 0.5, jitter: 0.05 },
  iceCast: { clips: ["ice2"], gain: 0.45, rate: 1.1 },
  earth: { clips: ["earth"], gain: 0.7, jitter: 0.05 },
  buff: { clips: ["buff"], gain: 0.4, jitter: 0.04 },
  rage: { clips: ["roar"], gain: 0.55, rate: 1.15, jitter: 0.04 },
  roar: { clips: ["roar"], gain: 0.7, rate: 0.85 },
  warcry: { clips: ["warcry"], gain: 0.55, jitter: 0.05 },
  whirl: { clips: ["miss1", "miss3"], gain: 0.6, rate: 0.75 },
  shadow: { clips: ["shadow"], gain: 0.45, jitter: 0.05 },
  smoke: { clips: ["shadow"], gain: 0.45, rate: 0.75 },
  poison: { clips: ["poison"], gain: 0.45, jitter: 0.06 },
  throw: { clips: ["throw"], gain: 0.5, jitter: 0.06 },
  // Mortes e nível
  skeleton: { clips: ["skel1", "skel2"], gain: 0.6, jitter: 0.06 },
  bones: { clips: ["bones1", "bones2"], gain: 0.5, jitter: 0.1 },
  boss: { clips: ["skel1"], gain: 0.9, rate: 0.75 },
  rubble: { clips: ["rubble"], gain: 0.75, rate: 0.85 },
  fall: { clips: ["fall"], gain: 0.65 },
  levelup: { clips: ["levelup"], gain: 0.7 },
  // Passos
  step: { clips: ["step1", "step2", "step3", "step4"], gain: 0.22, jitter: 0.06 },
  // Interface
  click: { clips: ["click"], gain: 0.3 },
  open: { clips: ["open"], gain: 0.55 },
  close: { clips: ["close"], gain: 0.45 },
  coins: { clips: ["coins"], gain: 0.55, jitter: 0.04 },
  buy: { clips: ["buy"], gain: 0.5 },
  pickup: { clips: ["pickup"], gain: 0.6, jitter: 0.05 },
  potion: { clips: ["drink"], gain: 0.5, jitter: 0.05 },
  enchant: { clips: ["enchant"], gain: 0.65 },
  fail: { clips: ["fail"], gain: 0.5 },
  anvil: { clips: ["anvil"], gain: 0.16, jitter: 0.05 },
} satisfies Record<string, SoundDef>;

export type SoundName = keyof typeof SOUNDS;
export type UiSound = "click" | "open" | "close" | "coins" | "buy" | "pickup" | "potion" | "enchant" | "fail" | "levelup";

/** Impacto de cada skill no alvo (o resto usa o golpe normal). */
const IMPACT: Record<string, SoundName> = {
  fireball: "fire",
  m_ember: "fire",
  m_rain: "fire",
  m_meteor: "boom",
  ice_nova: "ice",
  m_lance: "ice",
  m_prison: "ice",
  shield_bash: "bash",
  k_charge: "bash",
  brutal_strike: "heavy",
  b_heavy: "heavy",
  k_judg: "heavy",
  b_exec: "heavy",
  backstab: "slice",
  r_twin: "slice",
  r_para: "slice",
  r_poison: "slice",
  r_weak: "slice",
  r_flask: "poison",
  r_plague: "poison",
  r_mark: "curse",
  b_quake: "earth",
  b_stomp: "earth",
  raid: "boom",
};

/** Som ao lançar (skills em si/área e projéteis). */
const CAST: Record<string, SoundName> = {
  fireball: "fireCast",
  m_ember: "fireCast",
  m_rain: "fireCast",
  m_meteor: "fireCast",
  ice_nova: "ice",
  m_armor: "iceCast",
  m_lance: "iceCast",
  m_prison: "iceCast",
  fortress: "buff",
  m_shield: "buff",
  fury: "rage",
  b_frenzy: "rage",
  b_undying: "rage",
  b_roar: "roar",
  k_taunt: "warcry",
  k_cry: "warcry",
  k_banner: "warcry",
  k_spin: "whirl",
  b_quake: "earth",
  b_stomp: "earth",
  shadow_step: "shadow",
  r_vanish: "shadow",
  m_blink: "shadow",
  r_smoke: "smoke",
  r_flask: "throw",
  r_plague: "throw",
  r_mark: "curse",
};

/**
 * Região para o som: como regionAt, mas o espaço entre dois anéis (ex.: logo fora da
 * muralha, antes da primeira zona de caça) conta como o anel de fora, e não como o último
 * anel ou a área mais próxima (a Cripta tocaria no vão entre os Campos e os Ermos).
 */
function soundRegion(regions: Region[], x: number, z: number): Region {
  const r = regionAt(regions, x, z);
  if (r.area && Math.hypot(x - r.area.x, z - r.area.z) < r.area.r * 1.6) return r; // 1,6 = AREA_FRINGE
  const d = Math.hypot(x, z);
  return regions.find((g) => !g.area && d < g.rMax) ?? r;
}

/** Música de cada região: cidade, ermos perto, ermos longe e a cripta do raid boss. */
function musicFor(r: Region): string {
  if (r.safe) return "town";
  if (r.name === RAID_LAIR.name) return "crypt";
  return r.levelMax <= 12 ? "wild" : "dark";
}

/** Fundo ambiente: grilos à noite, cidade, vento nas terras mortas e na cripta, floresta. */
function ambientFor(r: Region, night: boolean): string {
  if (r.name === RAID_LAIR.name) return "wind";
  if (night) return "night";
  if (r.safe) return "town";
  return r.levelMax > 20 ? "wind" : "forest";
}

/** Volume de cada faixa (equilibra os arquivos entre si). */
const TRACK_GAIN: Record<string, number> = {
  town: 0.8,
  wild: 0.9,
  dark: 0.8,
  crypt: 0.75,
  forest: 1,
  night: 1,
  wind: 0.45,
};

/** Posição no chão (Vector3 do Babylon ou qualquer {x, z}). */
interface Pos {
  x: number;
  z: number;
}

export interface WorldAudioOpts {
  scene: Scene;
  regions: Region[];
  /** Fase do dia (0–1). */
  phase(): number;
  /** Posição do jogador local (ouvinte). */
  listener(): Pos | undefined;
  dead(): boolean;
  camera: { alpha: number };
  /** Moedas na bolsa e aviso de mudança do inventário (comprar, vender, pegar moedas). */
  gold(): number;
  onInventory(fn: () => void): () => void;
}

/**
 * Camada de música ou ambiente: guarda a faixa pedida, carrega quando precisa (no máximo
 * `keep` faixas decodificadas na memória) e troca com crossfade.
 */
class Channel {
  private desired = "";
  private want = "";
  private layer: LoopLayer | null = null;
  private layerKey = "";
  private cache = new Map<string, Promise<AudioBuffer | null>>();

  constructor(
    private eng: AudioEngine,
    private bus: Bus,
    private dir: string,
    private seam: number,
    private fade: number,
    private rate: number,
    private keep: number,
  ) {}

  set(key: string): void {
    this.desired = key;
    this.sync();
  }

  /** Reaplica depois de mudar volume/mudo (volume zero = nem carrega). */
  sync(): void {
    const key = this.eng.level(this.bus) > 0 ? this.desired : "";
    if (key === this.want) return;
    this.want = key;
    if (!key) {
      this.layer?.stop(this.fade);
      this.layer = null;
      this.layerKey = "";
      return;
    }
    void this.load(key).then((buf) => {
      if (!buf) return;
      this.eng.whenReady(() => {
        const ctx = this.eng.ctx;
        const out = this.eng.bus(this.bus);
        if (this.want !== key || this.layerKey === key || !ctx || !out) return;
        this.layer?.stop(this.fade);
        this.layer = new LoopLayer(ctx, out, buf, this.seam);
        this.layer.start(TRACK_GAIN[key] ?? 1, this.fade);
        this.layerKey = key;
      });
    });
  }

  private load(key: string): Promise<AudioBuffer | null> {
    let p = this.cache.get(key);
    if (!p) {
      p = fetch(`${BASE}${this.dir}/${key}.mp3`)
        .then((r) => (r.ok ? r.arrayBuffer() : Promise.reject(new Error(`HTTP ${r.status}`))))
        .then((b) => this.eng.decode(b, this.rate))
        .catch((err) => {
          console.warn(`áudio: ${this.dir}/${key} indisponível`, err);
          this.cache.delete(key);
          return null;
        });
      this.cache.set(key, p);
    } else {
      this.cache.delete(key); // vai para o fim (mais recente)
      this.cache.set(key, p);
    }
    // Esquece as faixas mais antigas (a memória do celular é curta).
    for (const k of this.cache.keys()) {
      if (this.cache.size <= this.keep) break;
      if (k !== key && k !== this.layerKey) this.cache.delete(k);
    }
    return p;
  }

  stop(): void {
    this.desired = "";
    this.sync();
  }

  /** Estado para os testes no navegador. */
  debug(): { want: string; playing: string; cached: string[] } {
    return { want: this.want, playing: this.layerKey, cached: [...this.cache.keys()] };
  }
}

class GameAudio {
  private eng = new AudioEngine();
  private sprite: AudioBuffer | null = null;
  private last = new Map<SoundName, number>();
  private started = false;
  private music = new Channel(this.eng, "music", "music", 2.5, 3, 32000, 2);
  private ambient = new Channel(this.eng, "ambient", "amb", 2, 2.5, 24000, 3);
  /** Ouvinte (jogador local) e vetor "direita" da câmera, no chão. */
  private lx = 0;
  private lz = 0;
  private rx = 1;
  private rz = 0;
  private hasListener = false;

  /** Liga o áudio: espera o primeiro gesto, segue as configurações e carrega os efeitos. */
  init(): void {
    if (this.started) return;
    this.started = true;
    if (!this.eng.available) return;
    this.eng.arm();
    const apply = (s: Settings) => {
      this.eng.setVolumes({ master: s.vMaster, music: s.vMusic, sfx: s.vSfx, ambient: s.vAmbient, muted: s.muted });
      this.music.sync();
      this.ambient.sync();
    };
    apply(settings.get());
    settings.on(apply);
    fetch(`${BASE}sfx.mp3`)
      .then((r) => (r.ok ? r.arrayBuffer() : Promise.reject(new Error(`HTTP ${r.status}`))))
      .then((b) => this.eng.decode(b, 44100))
      .then((buf) => (this.sprite = buf))
      .catch((err) => console.warn("áudio: efeitos indisponíveis", err));
    // Clique nos botões da interface (a barra de atalhos e os controles de toque não fazem
    // barulho: são usados o tempo todo).
    document.addEventListener(
      "click",
      (ev) => {
        const t = ev.target as HTMLElement | null;
        const b = t?.closest?.("button, [role='tab'], select, label.toggle, .seg button");
        if (b && !b.closest(".hotbar, .touch-ui")) this.ui("click");
      },
      { capture: true, passive: true },
    );
  }

  /** Efeito sem posição (interface, nível do jogador local). */
  ui(name: UiSound): void {
    this.play(name, null, true);
  }

  private windows = 0;
  /** Quantas janelas estão abertas: abrir e fechar têm som. */
  windowCount(n: number): void {
    if (n > this.windows) this.ui("open");
    else if (n < this.windows) this.ui("close");
    this.windows = n;
  }

  /** Golpe que encostou (fx da skill, ou nada no ataque básico). */
  hit(fx: string | undefined, crit: boolean, magic: boolean, at: Pos, local: boolean): void {
    const name: SoundName = (fx && IMPACT[fx]) || (crit ? "crit" : magic ? "arcane" : "hit");
    this.play(name, at, local);
    if (crit && name !== "crit" && fx) this.play("crit", at, local, 0.6);
  }

  miss(at: Pos, local: boolean): void {
    this.play("miss", at, local);
  }

  /** Projétil saindo da mão (Bola de Fogo, setas mágicas, magia básica). */
  launch(fx: string | undefined, at: Pos, local: boolean): void {
    if (!fx) return; // a magia básica só faz barulho ao chegar
    this.play(CAST[fx] ?? "bolt", at, local);
  }

  /** Skill em si mesmo ou em área (no instante em que o efeito sai). */
  cast(fx: string, at: Pos, local: boolean): void {
    this.play(CAST[fx] ?? "spell", at, local);
  }

  /** Morte (quando a queda aparece na tela). */
  death(model: string, monster: boolean, boss: boolean, at: Pos, local: boolean): void {
    if (!monster) return this.play("fall", at, local);
    if (model === "stone_golem") return this.play("rubble", at, local);
    this.play(boss ? "boss" : "skeleton", at, local);
    this.play("bones", at, local, 0.8);
  }

  levelUp(at: Pos, local: boolean): void {
    this.play("levelup", local ? null : at, local, local ? 1 : 0.6);
  }

  /** Abre o som do mundo (música, ambiente, passos e moedas) até a função devolvida. */
  attachWorld(o: WorldAudioOpts): () => void {
    let lastX = NaN;
    let lastZ = NaN;
    let walked = 0;
    let zoneAt = 0;
    let anvilAt = performance.now() + 4000;
    let ambientKey = "";
    const zone = () => {
      const p = o.listener();
      if (!p) return;
      const r = soundRegion(o.regions, p.x, p.z);
      this.music.set(musicFor(r));
      ambientKey = ambientFor(r, isNight(o.phase()));
      this.ambient.set(ambientKey);
    };
    const obs = o.scene.onBeforeRenderObservable.add(() => {
      const p = o.listener();
      if (!p) return;
      const now = performance.now();
      const dt = Math.min(o.scene.getEngine().getDeltaTime() / 1000, 0.1);
      this.lx = p.x;
      this.lz = p.z;
      this.hasListener = true;
      // A câmera olha para -(cos α, sin α); a direita no chão é (-sen α, cos α).
      this.rx = -Math.sin(o.camera.alpha);
      this.rz = Math.cos(o.camera.alpha);
      // Passos do jogador local: um a cada STRIDE andados (teleporte não conta).
      const d = Number.isNaN(lastX) ? 0 : Math.hypot(p.x - lastX, p.z - lastZ);
      lastX = p.x;
      lastZ = p.z;
      if (o.dead() || d > 3) walked = 0;
      else if (dt > 0 && d / dt > WALK_SPEED) {
        walked += d;
        if (walked >= STRIDE) {
          walked -= STRIDE;
          this.play("step", null, true);
        }
      } else walked = Math.min(walked, STRIDE * 0.6); // ao voltar a andar, o 1º passo sai logo
      if (now - zoneAt > 500) {
        zoneAt = now;
        zone();
      }
      // Ferreiro de vez em quando na cidade, de dia.
      if (ambientKey === "town" && now > anvilAt) {
        anvilAt = now + 7000 + Math.random() * 9000;
        if (this.sprite && this.eng.level("ambient") > 0) {
          const [off, dur] = SPRITE.anvil;
          this.eng.playLoose(this.sprite, off, dur, "ambient", SOUNDS.anvil.gain, Math.random() * 1.2 - 0.6);
        }
      }
    });
    let lastGold = o.gold();
    const offInv = o.onInventory(() => {
      const g = o.gold();
      if (g > lastGold) this.ui("coins");
      else if (g < lastGold) this.ui("buy");
      lastGold = g;
    });
    zone();
    return () => {
      o.scene.onBeforeRenderObservable.remove(obs);
      offInv();
      this.music.stop();
      this.ambient.stop();
      this.hasListener = false;
    };
  }

  /**
   * Toca um som. `at` = posição no mundo (null = na cabeça do jogador); `local` = envolve o
   * jogador local (sempre ouvido e com prioridade).
   */
  private play(name: SoundName, at: Pos | null, local: boolean, gainMul = 1): void {
    const buf = this.sprite;
    if (!buf || !this.eng.running) return;
    const level = this.eng.level("sfx");
    if (level <= 0) return;
    let gain: number = SOUNDS[name].gain * gainMul;
    let pan = 0;
    if (at && this.hasListener) {
      const dx = at.x - this.lx;
      const dz = at.z - this.lz;
      const d = Math.hypot(dx, dz);
      if (!local && d >= HEAR_DISTANCE) return;
      if (d > NEAR) {
        const k = 1 - Math.min(1, (d - NEAR) / (HEAR_DISTANCE - NEAR));
        gain *= local ? Math.max(0.5, k * k) : k * k;
        pan = ((dx * this.rx + dz * this.rz) / d) * 0.75;
      }
    }
    if (gain < 0.01) return;
    const now = performance.now();
    if (now - (this.last.get(name) ?? -Infinity) < REPEAT_MS) return;
    this.last.set(name, now);
    const def: SoundDef = SOUNDS[name];
    const clip = def.clips[(Math.random() * def.clips.length) | 0];
    const [off, dur] = SPRITE[clip];
    const rate = (def.rate ?? 1) * (1 + (Math.random() * 2 - 1) * (def.jitter ?? 0));
    this.eng.play(buf, off, dur, gain, pan, rate, gain * (local ? 4 : 1));
    this.played.set(name, (this.played.get(name) ?? 0) + 1);
  }

  private played = new Map<SoundName, number>();
  /** Estado para os testes no navegador (só em desenvolvimento). */
  debug() {
    return {
      state: this.eng.ctx?.state ?? "sem contexto",
      available: this.eng.available,
      sprite: this.sprite ? Math.round(this.sprite.duration * 10) / 10 : 0,
      busy: this.eng.busyVoices(),
      levels: { music: this.eng.level("music"), sfx: this.eng.level("sfx"), ambient: this.eng.level("ambient") },
      music: this.music.debug(),
      ambient: this.ambient.debug(),
      played: Object.fromEntries(this.played),
    };
  }
}

export const gameAudio = new GameAudio();
if (import.meta.env.DEV) (window as unknown as Record<string, unknown>).__ereviaAudio = gameAudio;
