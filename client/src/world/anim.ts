import type { AnimationGroup, Node } from "@babylonjs/core";
import { stepRatios } from "./motion";
import type { AnimRole } from "./models";

/**
 * Mapa papel -> clipe que avisa quando é trocado de conjunto (o bárbaro com arma de duas mãos passa a usar
 * parado, andar e correr com as duas mãos juntas): o AnimController recomeça com os clipes novos.
 */
export class RoleMap extends Map<AnimRole, AnimationGroup> {
  onChange: (() => void) | null = null;
  /**
   * Velocidade natural dos clipes de andar e correr (m/s do mundo, com a escala do modelo): a que o pé de apoio
   * percorre para trás em relação ao quadril. O clipe toca a `velocidade real / natural` para o pé não deslizar.
   * Sem isto (golem de código), vale a regra antiga (RUN_ANIM_SPEED).
   */
  footSpeeds: { walk?: number; run: number } | undefined;
  /**
   * Segundos de golpe mostrados antes do impacto (no lugar do SWING_LEAD do combate). Os heróis v4 têm a
   * preparação animada inteira (0,4–0,5 s até o impacto): com o padrão de 0,18 s ela era pulada.
   */
  swingLead?: number;
}

/** Variantes de cada papel (o que existir no conjunto): o golpe alterna em sequência, dano e morte sorteiam. */
const VARIANTS: Partial<Record<AnimRole, AnimRole[]>> = {
  attack: ["attack", "attack2", "attack3"],
  hit: ["hit", "hit2"],
  death: ["death", "death2"],
};

/** Velocidade das transições de peso da locomoção (1/s). */
const BLEND_RATE = 14;
/** Entrada de uma ação (1/s): ~95% em 0,15 s. */
const ACTION_IN_RATE = 20;
/** Saída de uma ação que terminou ou foi trocada (s), em curva suave (começa e termina devagar). */
const ACTION_OUT_S = 0.3;
/** Andando há este tanto (s) no meio de um golpe ou tiro: a ação cede para a corrida (senão o corpo desliza parado). */
const MOVE_BREAKS_ACTION_S = 0.12;

/** Como tocar uma ação. */
export interface PlayOpts {
  /** Mantém a pose final (morte). */
  hold?: boolean;
  /** Velocidade do clipe (1 = normal). */
  speed?: number;
  /**
   * Segundos de preparação antes do impacto: o clipe começa já no meio do movimento, para o
   * golpe encostar `lead` segundos depois (o servidor já decidiu o dano quando o pacote chega).
   */
  lead?: number;
  /** Repete até outra ação (ou endEmote) tomar o lugar: dançar e sentar. */
  loop?: boolean;
}

/**
 * Controla as animações de um personagem em duas camadas:
 * - locomoção: parado ↔ correndo, sempre tocando, misturadas por peso;
 * - ação: ataque/skill/apanhar/morrer, toca uma vez por cima e devolve o corpo aos poucos
 *   (os emotes de dançar e sentar repetem até endEmote ou outra ação).
 * A morte segura a pose final até o personagem reviver.
 */
export class AnimController {
  private runWeight = 0;
  /** Há quanto tempo (s) o corpo está andando sem parar. */
  private movingFor = 0;
  /** 0 = correr, 1 = andar (só modelos com clipe "walk": o remaster). */
  private walkMix = 0;
  private walkRatio = 1;
  private actionWeight = 0;
  private action: AnimationGroup | null = null;
  private actionDone = true;
  private holding = false;
  /** A morte terminou e está congelada no último quadro. */
  private frozen = false;
  /** Ações saindo de cena: paradas no quadro em que estavam, com o peso caindo. */
  private fading: { g: AnimationGroup; w: number; w0: number; t: number }[] = [];
  /**
   * Voltar à pose de descanso antes das animações do próximo quadro. O update() roda depois
   * que o Babylon já animou o quadro atual: zerar os ossos ali mostraria a pose de descanso
   * (o boneco "pulando") por um quadro.
   */
  private resetDue = false;
  /** Velocidade da ação atual (para saber quando ela chega ao fim). */
  private actionSpeed = 1;
  /** Sem cena para esperar o próximo quadro (não deveria acontecer): volta à pose na hora. */
  private deferReset = false;
  /** A ação atual repete (emote de dançar ou sentar). */
  private looping = false;
  /** Pausa de impacto: segundos que ainda faltam com a pose congelada (só este personagem). */
  private stopLeft = 0;
  /** Velocidade da corrida no último quadro (para devolver depois da pausa). */
  private runRatio = 1;
  /**
   * LOD de animação: false = fora da tela ou longe. Os grupos ficam pausados (pose
   * congelada) e as ações não tocam: nada de stop()/start(), que varrem todas as animações
   * ativas da cena no Babylon.
   */
  private active = true;
  /** Grupos distintos do personagem (ataque e skill podem ser o mesmo clipe). */
  private groups!: AnimationGroup[];
  /** Grupos pausados ao sair de cena, retomados ao voltar. */
  private paused: AnimationGroup[] = [];
  /** Morte ou emote em loop pedidos fora de cena: tocam ao voltar (a morte já no fim). */
  private deferred: { role: AnimRole; opts: PlayOpts } | null = null;
  /** Uma ação foi cortada fora de cena: volta à pose de descanso antes de retomar. */
  private resetOnWake = false;
  /** Pausas do parado ("idle breaks": olhar em volta, alongar, gesto): tocam de vez em quando, com tempo e fase próprios. */
  private breaks!: AnimationGroup[];
  private resetHooked = false;
  /** Tempo (s) que os clipes ficaram pausados desde a última vez que animaram. */
  private asleepFor = 0;
  /** Escala do corpo (monstros grandes dão passos maiores): multiplica as velocidades naturais dos clipes. */
  private bodyScale = 1;
  /** Só para medir o "antes" (?legacymove=1): velocidade do clipe pela constante antiga e mistura mais lenta. */
  legacy = false;
  private breakGroup: AnimationGroup | null = null;
  /** Próxima variante do golpe (índice em VARIANTS.attack, entre as que existem). */
  private comboStep = 0;
  private breakW = 0;
  private breakTimer = 0;
  /** Segundos que ainda faltam "em combate" (golpe, skill, dano ou bloqueio recentes): o parado vira a guarda. */
  private combatLeft = 0;
  /** Mistura do parado para a guarda (0 = parado, 1 = guarda). */
  private stanceW = 0;

  /**
   * resetPose: devolve os ossos à pose de descanso quando uma ação sai.
   * impacts: instante (s) do golpe/soltura de cada ação, no próprio clipe.
   */
  constructor(
    private anims: Map<AnimRole, AnimationGroup>,
    private resetPose: () => void = () => {},
    private impacts: Partial<Record<AnimRole, number>> = {},
  ) {
    this.start();
    if (anims instanceof RoleMap) anims.onChange = () => this.rebind();
  }

  /** O conjunto de clipes mudou: para tudo o que tocava e recomeça com os clipes novos (parado, andar e correr). */
  setBodyScale(s: number): void {
    this.bodyScale = s > 0 ? s : 1;
  }

  /** Pesos e velocidades dos clipes de locomoção (para medir o deslize nos testes no navegador). */
  debug(): { idle: number; run: number; walk: number; runRatio: number; walkRatio: number; action: number; runNat: number } {
    return {
      idle: this.anims.get("idle")?.weight ?? 0,
      run: this.anims.get("run")?.weight ?? 0,
      walk: this.anims.get("walk")?.weight ?? 0,
      runRatio: this.anims.get("run")?.speedRatio ?? 0,
      walkRatio: this.anims.get("walk")?.speedRatio ?? 0,
      action: this.actionWeight,
      runNat: this.anims instanceof RoleMap && this.anims.footSpeeds ? this.anims.footSpeeds.run * this.bodyScale : 0,
    };
  }

  private rebind(): void {
    for (const g of this.groups) g.stop();
    for (const f of this.fading) f.g.stop();
    this.fading.length = 0;
    this.action = null;
    this.actionDone = true;
    this.actionWeight = 0;
    this.looping = false;
    this.holding = false;
    this.frozen = false;
    this.breakGroup = null;
    this.breakW = 0;
    this.paused.length = 0;
    this.queueReset();
    this.start();
  }

  private start(): void {
    const anims = this.anims;
    this.groups = [...new Set(anims.values())];
    this.breaks = (["break1", "break2", "break3"] as const).map((r) => anims.get(r)).filter((g): g is AnimationGroup => !!g);
    // Cada personagem começa numa fase e num tempo próprios (ninguém respira ou olha em volta junto com o vizinho).
    this.breakTimer = BREAK_MIN_S + Math.random() * (BREAK_MAX_S - BREAK_MIN_S);
    const idle = anims.get("idle");
    const run = anims.get("run");
    const walk = anims.get("walk");
    if (walk) {
      walk.weight = 0;
      walk.start(true);
    }
    if (idle) {
      idle.weight = 1;
      idle.start(true);
      idle.goToFrame(idle.from + Math.random() * (idle.to - idle.from));
    }
    if (run) {
      run.weight = 0;
      run.start(true);
    }
    const target = [...anims.values()][0]?.targetedAnimations[0]?.target as Node | undefined;
    const scene = target?.getScene?.();
    if (scene && target && !this.resetHooked) {
      this.resetHooked = true;
      this.deferReset = true;
      const obs = scene.onBeforeAnimationsObservable.add(() => {
        if (target.isDisposed()) {
          scene.onBeforeAnimationsObservable.remove(obs);
          return;
        }
        if (this.resetDue) {
          this.resetDue = false;
          this.resetPose();
        }
      });
    }
  }

  private queueReset(): void {
    if (this.deferReset) this.resetDue = true;
    else this.resetPose();
  }

  /** Duração do clipe (s) na velocidade normal; 0 se o modelo não tem essa ação. */
  length(role: AnimRole): number {
    const g = this.anims.get(role);
    return g ? (g.to - g.from) / fps(g) : 0;
  }

  /**
   * Quanto do clipe ainda toca depois de começar com esse `lead` (s, velocidade normal).
   * Sem instante de impacto conhecido, o clipe toca inteiro.
   */
  remaining(role: AnimRole, lead?: number): number {
    const r = this.variant(role, false);
    return this.length(r) - this.startAt(r, lead);
  }

  /**
   * O papel que toca de fato: o golpe segue a sequência (combo) e só avança quando toca (`advance`); dano e
   * morte sorteiam entre as variantes. Sem variantes no conjunto, o próprio papel.
   */
  private variant(role: AnimRole, advance: boolean): AnimRole {
    const all = VARIANTS[role]?.filter((r) => this.anims.has(r));
    if (!all || all.length < 2) return role;
    if (role === "attack") {
      const r = all[this.comboStep % all.length];
      if (advance) this.comboStep = (this.comboStep + 1) % all.length;
      return r;
    }
    return advance ? all[Math.floor(Math.random() * all.length)] : role;
  }

  private startAt(role: AnimRole, lead?: number): number {
    const impact = this.impacts[role];
    if (lead === undefined || impact === undefined) return 0;
    return Math.max(0, impact - lead);
  }

  /**
   * Toca uma ação uma vez. Devolve em quantos segundos o golpe encosta (com `lead`), ou 0
   * se a ação não tocou.
   */
  /** Preparação do golpe própria do modelo (RoleMap.swingLead), se houver. */
  get swingLead(): number | undefined {
    return this.anims instanceof RoleMap ? this.anims.swingLead : undefined;
  }

  /** O conjunto tem esse papel (ex.: "block" só nos heróis com escudo animado). */
  has(role: AnimRole): boolean {
    return this.anims.has(role);
  }

  play(asked: AnimRole, opts: PlayOpts = {}): number {
    if (COMBAT_ROLES.has(asked)) this.combatLeft = COMBAT_STANCE_S;
    const role = this.variant(asked, true);
    const g = this.anims.get(role);
    if (!g) return 0;
    if (this.holding && !opts.hold) return 0; // morto não reage
    // Apanhar não interrompe um ataque em andamento.
    // (Um "apanhar" em andamento pode recomeçar, mesmo que a variante sorteada agora seja outra.)
    const hitting = this.action === this.anims.get("hit") || this.action === this.anims.get("hit2");
    if ((asked === "hit" || asked === "block") && this.action && !this.actionDone && !hitting) return 0;

    const speed = opts.speed ?? 1;
    const start = this.startAt(role, opts.lead);
    const impact = this.impacts[role];
    const lands = impact !== undefined && opts.lead !== undefined ? (impact - start) / speed : 0;
    if (!this.active) {
      // Fora de cena: golpe, apanhar e acenar não aparecem; a morte e o emote em loop ficam
      // para quando voltar. O instante do impacto é o mesmo (o combate agenda por ele).
      // Um emote que estava pausado sai: a ação nova toma o lugar dele, como em cena.
      if (this.action && !this.holding) {
        this.action.stop();
        this.action = null;
        this.actionWeight = 0;
        this.actionDone = true;
        this.resetOnWake = true;
      }
      this.deferred = opts.hold || opts.loop ? { role, opts } : null;
      this.holding = !!opts.hold;
      this.looping = !!opts.loop;
      return lands;
    }
    // A ação anterior sai aos poucos (sem estalo de pose). Se é o mesmo clipe, reinicia
    // com o peso que já tinha.
    let weight = 0;
    if (this.action === g) weight = this.actionWeight;
    else if (this.action) this.fadeOut(this.action, this.actionWeight);
    const i = this.fading.findIndex((f) => f.g === g);
    if (i >= 0) {
      weight = this.fading[i].w;
      this.fading.splice(i, 1);
    }

    g.stop(); // start() não faz nada num grupo já iniciado (mesmo pausado)
    this.action = g;
    this.actionWeight = weight;
    this.actionDone = false;
    this.frozen = false;
    this.holding = !!opts.hold;
    this.looping = !!opts.loop;
    this.actionSpeed = speed;
    g.speedRatio = speed;
    // Toca em loop para o grupo não ser removido ao terminar; na primeira volta, para no
    // último quadro. A morte fica assim; as outras ações devolvem o corpo à locomoção.
    g.start(true, speed, g.from + start * fps(g), g.to);
    // Dentro de uma pausa de impacto, a ação nova espera a pausa acabar (o "apanhar" do alvo).
    if (this.stopLeft > 0) g.speedRatio = 0;
    g.onAnimationGroupLoopObservable.addOnce(() => {
      if (this.action !== g || this.looping) return;
      if (this.holding) {
        g.goToFrame(g.to);
        g.pause();
        this.frozen = true;
      } else {
        // Normalmente o update() já parou o clipe antes do fim; isto só pega um quadro longo.
        g.speedRatio = 0;
        this.actionDone = true;
      }
    });
    return lands;
  }

  /** Um emote em loop (dançar, sentar) está tocando. */
  get emoting(): boolean {
    return this.looping && (this.action !== null || this.deferred !== null);
  }

  /** Acaba o emote em loop: o corpo volta à locomoção aos poucos. */
  endEmote(): void {
    if (!this.looping) return;
    this.looping = false;
    this.deferred = null;
    if (this.action) this.fadeOut(this.action, this.actionWeight);
    this.action = null;
    this.actionWeight = 0;
  }

  /**
   * Pausa de impacto (hit stop): congela as animações deste personagem por `seconds` (40–80
   * ms). Só a pose para: o personagem continua andando, o jogo e a rede seguem normais.
   */
  hitStop(seconds: number): void {
    if (this.holding || seconds <= 0) return;
    this.stopLeft = Math.max(this.stopLeft, seconds);
    for (const g of this.anims.values()) g.speedRatio = 0;
  }

  /** Fim da pausa: cada clipe volta à velocidade que tinha. */
  private resume(): void {
    this.stopLeft = 0;
    const idle = this.anims.get("idle");
    const run = this.anims.get("run");
    if (idle) idle.speedRatio = 1;
    const stance = this.anims.get("stance");
    if (stance) stance.speedRatio = 1;
    for (const g of this.breaks) g.speedRatio = 1;
    if (run) run.speedRatio = this.runRatio;
    const walk = this.anims.get("walk");
    if (walk) walk.speedRatio = this.walkRatio;
    // Ação parada no fim (ou saindo de cena) fica parada; a que ainda toca volta a andar.
    // (A morte que começou durante a pausa também volta; a já congelada no fim, não.)
    if (this.action && !this.actionDone && !this.frozen) this.action.speedRatio = this.actionSpeed;
  }

  /** Volta ao normal (ao reviver). */
  reset(): void {
    if (this.stopLeft > 0) this.resume();
    this.deferred = null;
    this.holding = false;
    this.looping = false;
    this.frozen = false;
    this.actionDone = true;
    this.action?.stop();
    this.action = null;
    this.actionWeight = 0;
    for (const f of this.fading) f.g.stop();
    this.fading.length = 0;
    this.queueReset();
  }

  /**
   * Para o clipe onde está, mas sem pausar: uma animação pausada não entra na mistura do
   * Babylon e, se ainda "ocupasse" peso, a soma ficaria < 1 e o Babylon completaria com o
   * valor original dos ossos — nos esqueletos isso enterra o corpo no chão.
   */
  private fadeOut(g: AnimationGroup, w: number): void {
    g.speedRatio = 0;
    if (w > 0.01) this.fading.push({ g, w, w0: w, t: 0 });
    else {
      g.stop();
      this.queueReset();
    }
  }

  /**
   * Saiu de cena: a ação de um instante acaba ali (sem voltar à pose, que ainda pode estar
   * na tela de longe) e os grupos que tocam são pausados. Morte e emote em loop só pausam.
   */
  private sleep(): void {
    const a = this.action;
    if (a && !this.holding && !this.looping) {
      a.stop();
      this.action = null;
      this.actionWeight = 0;
      this.actionDone = true;
      this.resetOnWake = true;
    }
    for (const f of this.fading) f.g.stop();
    if (this.fading.length) this.resetOnWake = true;
    this.fading.length = 0;
    for (const g of this.groups) {
      if (!g.isPlaying) continue;
      g.pause();
      this.paused.push(g);
    }
  }

  /** Voltou para a cena: retoma os grupos e toca o que ficou pendente. */
  /**
   * Quem acordou depois de `seconds` parado (fora da tela, ou longe e animado a 15 Hz) retoma o passo na fase certa:
   * os clipes de locomoção avançam o tempo que ficaram pausados, senão o pé tocaria devagar e deslizaria.
   */
  private catchUp(seconds: number): void {
    if (seconds <= 0) return;
    for (const g of [this.anims.get("idle"), this.anims.get("run"), this.anims.get("walk"), this.breakGroup]) {
      const a = g?.animatables[0];
      if (!g || !a || !g.isPlaying) continue;
      const span = g.to - g.from;
      if (span <= 0) continue;
      const moved = (a.masterFrame - g.from + seconds * fps(g) * g.speedRatio) % span;
      g.goToFrame(g.from + (moved < 0 ? moved + span : moved));
    }
  }

  private wake(): void {
    if (this.resetOnWake) {
      this.resetOnWake = false;
      this.queueReset();
    }
    for (const g of this.paused) g.restart(); // grupo parado nesse meio-tempo não volta
    this.paused.length = 0;
    this.catchUp(this.asleepFor);
    this.asleepFor = 0;
    const d = this.deferred;
    if (!d) return;
    this.deferred = null;
    this.holding = false;
    this.looping = false;
    this.play(d.role, d.opts);
    const g = this.action;
    if (d.opts.hold && g) {
      // Morreu fora de cena: aparece já caído (último quadro), sem a queda.
      this.actionWeight = 1;
      g.weight = 1;
      g.goToFrame(g.to);
      g.pause();
      this.frozen = true;
    }
  }

  /**
   * active = false (fora da tela ou longe): os grupos pausam e o esqueleto fica congelado
   * na última pose até voltar (ver sleep/wake).
   */
  update(dt: number, moving: boolean, runSpeedRatio: number, active = true, speed = Infinity): void {
    const fs = this.anims instanceof RoleMap && !this.legacy ? this.anims.footSpeeds : undefined;
    let walkTarget = speed < WALK_BELOW ? 1 : 0;
    if (fs) {
      // Pé no chão: o clipe toca na velocidade em que o personagem anda de verdade (speed = velocidade desenhada).
      const runNat = fs.run * this.bodyScale;
      const walkNat = (fs.walk ?? 0) * this.bodyScale;
      const r = stepRatios(speed, runNat, walkNat);
      this.runRatio = r.run;
      this.walkRatio = r.walkRatio;
      walkTarget = r.walk ? 1 : 0;
    } else {
      this.runRatio = runSpeedRatio;
      this.walkRatio = Math.min(1.5, Math.max(0.5, speed / WALK_ANIM_SPEED));
    }
    if (active !== this.active) {
      this.active = active;
      if (active) this.wake();
      else this.sleep();
    }
    if (this.stopLeft > 0) {
      // Pausa de impacto: pose e pesos ficam como estão até ela acabar.
      this.stopLeft -= dt;
      if (this.stopLeft > 0) return;
      this.resume();
    }
    if (!active) {
      this.asleepFor += dt;
      return;
    }
    const k = 1 - Math.exp(-(this.legacy ? 10 : BLEND_RATE) * dt);
    this.runWeight += ((moving && !this.holding ? 1 : 0) - this.runWeight) * k;
    // Devagar (monstros, lentidão): clipe de andar, senão o personagem "corre" em câmera lenta.
    this.walkMix += (walkTarget - this.walkMix) * k;
    // A ação chega ao fim no próximo quadro: para ali (velocidade 0, ainda na mistura). Em
    // loop o Babylon voltaria ao primeiro quadro, e o corpo pularia para o começo do golpe.
    // Saiu andando no meio da ação (o alvo morreu, a auto-caça foi para o próximo): a ação sai aos poucos e a
    // corrida assume, em vez de o golpe seguir tocando com o corpo deslizando pelo chão.
    this.movingFor = moving ? this.movingFor + dt : 0;
    if (this.action && !this.actionDone && !this.holding && !this.looping && this.movingFor >= MOVE_BREAKS_ACTION_S) this.actionDone = true;
    const a = this.action;
    if (a && !this.actionDone && !this.holding && !this.looping) {
      const frame = a.animatables[0]?.masterFrame ?? a.from;
      if (frame + this.actionSpeed * fps(a) * dt * 1.5 >= a.to) {
        a.speedRatio = 0;
        this.actionDone = true;
      }
    }
    // A ação terminou: fica parada no último quadro e sai aos poucos.
    if (this.action && this.actionDone) {
      this.fadeOut(this.action, this.actionWeight);
      this.action = null;
      this.actionWeight = 0;
    }
    if (this.action) this.actionWeight += (1 - this.actionWeight) * (1 - Math.exp(-ACTION_IN_RATE * dt));

    let fadeSum = 0;
    for (let i = this.fading.length - 1; i >= 0; i--) {
      const f = this.fading[i];
      f.t += dt;
      const x = Math.min(1, f.t / ACTION_OUT_S);
      f.w = Math.min(f.w, f.w0 * (1 - x * x * (3 - 2 * x)));
      if (f.w < 0.01) {
        f.g.stop();
        this.fading.splice(i, 1);
        this.queueReset();
      } else {
        fadeSum += f.w;
      }
    }
    // A soma dos pesos nunca passa de 1: o que sai cede espaço para o que entra.
    const room = 1 - this.actionWeight;
    if (fadeSum > room) {
      const s = room / fadeSum;
      for (const f of this.fading) f.w *= s;
      fadeSum = room;
    }

    // Morte congelada: só ela segura a pose (locomoção com peso 0 não é calculada).
    const loco = this.frozen ? 0 : Math.max(0, 1 - this.actionWeight - fadeSum);
    const idle = this.anims.get("idle");
    const run = this.anims.get("run");
    // Peso exatamente 0 faz o Babylon pular a animação (não calcula os ossos). As misturas
    // convergem exponencialmente e nunca chegam a 0 sozinhas, então zeramos as sobras.
    // Pausas do parado: só com o corpo parado, sem ação; entram e saem suaves, e o clipe acaba antes do fim.
    // Guarda: em combate o parado mistura para a postura de combate (se o conjunto tiver), sem pausas.
    this.combatLeft = Math.max(0, this.combatLeft - dt);
    const stance = this.anims.get("stance");
    const stanceTarget = stance && this.combatLeft > 0 ? 1 : 0;
    this.stanceW += (stanceTarget - this.stanceW) * (1 - Math.exp(-5 * dt));
    if (stance && this.stanceW > 0.001 && !stance.isPlaying) stance.start(true);
    if (stance && stanceTarget === 0 && this.stanceW < 0.01) {
      this.stanceW = 0;
      if (stance.isPlaying) stance.stop();
    }
    const standing = loco > 0.9 && this.runWeight < 0.05 && this.stanceW < 0.05;
    let breakTarget = 0;
    if (this.breakGroup) {
      const frame = this.breakGroup.animatables[0]?.masterFrame ?? this.breakGroup.from;
      const left = (this.breakGroup.to - frame) / fps(this.breakGroup);
      breakTarget = standing && left > 0.5 && this.breakGroup.isPlaying ? 1 : 0;
      this.breakW += (breakTarget - this.breakW) * (1 - Math.exp(-6 * dt));
      if (breakTarget === 0 && this.breakW < 0.02) {
        this.breakGroup.stop();
        this.breakGroup = null;
        this.breakW = 0;
        this.breakTimer = BREAK_MIN_S + Math.random() * (BREAK_MAX_S - BREAK_MIN_S);
      }
    } else if (this.breaks.length && standing) {
      this.breakTimer -= dt;
      if (this.breakTimer <= 0) {
        this.breakGroup = this.breaks[Math.floor(Math.random() * this.breaks.length)];
        this.breakGroup.weight = 0.001;
        this.breakGroup.start(false);
        this.breakW = 0;
      }
    }
    const bw = this.breakGroup ? this.breakW : 0;
    if (this.breakGroup) this.breakGroup.weight = Math.max(loco * (1 - this.runWeight) * bw, 0.001);
    const still = loco * (1 - this.runWeight) * (1 - bw);
    if (idle) idle.weight = snap(still * (1 - this.stanceW));
    if (stance && this.stanceW > 0) stance.weight = snap(still * this.stanceW);
    const walk = this.anims.get("walk");
    const walkW = walk ? this.walkMix : 0;
    if (run) {
      run.weight = snap(loco * this.runWeight * (1 - walkW));
      run.speedRatio = this.runRatio;
    }
    if (walk) {
      walk.weight = snap(loco * this.runWeight * walkW);
      walk.speedRatio = this.walkRatio;
    }
    if (this.action) this.action.weight = Math.max(this.actionWeight, 0.001);
    for (const f of this.fading) f.g.weight = f.w;
  }
}

/** Velocidade (m/s) abaixo da qual o personagem anda em vez de correr, e a que o clipe de andar casa com o chão. */
const WALK_BELOW = 3.4;
const WALK_ANIM_SPEED = 1.9;

/** Papéis que põem o personagem em combate, e por quanto tempo (s) a guarda fica depois do último. */
const COMBAT_ROLES = new Set<AnimRole>(["attack", "attack2", "attack3", "skill", "hit", "hit2", "block"]);
const COMBAT_STANCE_S = 6;

/** Intervalo (s) entre as pausas do parado. */
const BREAK_MIN_S = 8;
const BREAK_MAX_S = 20;

function snap(w: number): number {
  return w < 0.01 ? 0 : w > 0.99 ? 1 : w;
}

/** Quadros por segundo do clipe (os glTF vêm em 60, o golem em 30). */
function fps(g: AnimationGroup): number {
  return g.targetedAnimations[0]?.animation.framePerSecond || 60;
}
