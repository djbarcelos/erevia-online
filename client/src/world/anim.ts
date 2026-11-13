import type { AnimationGroup, Node } from "@babylonjs/core";
import type { AnimRole } from "./models";

/** Velocidade das transições de peso da locomoção (1/s). */
const BLEND_RATE = 10;
/** Entrada de uma ação (1/s): ~95% em 0,15 s. */
const ACTION_IN_RATE = 20;
/** Saída de uma ação que terminou ou foi trocada (s), em curva suave (começa e termina devagar). */
const ACTION_OUT_S = 0.3;

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
   * resetPose: devolve os ossos à pose de descanso quando uma ação sai.
   * impacts: instante (s) do golpe/soltura de cada ação, no próprio clipe.
   */
  constructor(
    private anims: Map<AnimRole, AnimationGroup>,
    private resetPose: () => void = () => {},
    private impacts: Partial<Record<AnimRole, number>> = {},
  ) {
    const idle = anims.get("idle");
    const run = anims.get("run");
    if (idle) {
      idle.weight = 1;
      idle.start(true);
    }
    if (run) {
      run.weight = 0;
      run.start(true);
    }
    const target = [...anims.values()][0]?.targetedAnimations[0]?.target as Node | undefined;
    const scene = target?.getScene?.();
    if (scene && target) {
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
    return this.length(role) - this.startAt(role, lead);
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
  play(role: AnimRole, opts: PlayOpts = {}): number {
    const g = this.anims.get(role);
    if (!g) return 0;
    if (this.holding && !opts.hold) return 0; // morto não reage
    // Apanhar não interrompe um ataque em andamento.
    if (role === "hit" && this.action && !this.actionDone && this.action !== g) return 0;

    const speed = opts.speed ?? 1;
    const start = this.startAt(role, opts.lead);
    const impact = this.impacts[role];
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
    return impact !== undefined && opts.lead !== undefined ? (impact - start) / speed : 0;
  }

  /** Um emote em loop (dançar, sentar) está tocando. */
  get emoting(): boolean {
    return this.looping && this.action !== null;
  }

  /** Acaba o emote em loop: o corpo volta à locomoção aos poucos. */
  endEmote(): void {
    if (!this.looping) return;
    this.looping = false;
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
    if (run) run.speedRatio = this.runRatio;
    // Ação parada no fim (ou saindo de cena) fica parada; a que ainda toca volta a andar.
    // (A morte que começou durante a pausa também volta; a já congelada no fim, não.)
    if (this.action && !this.actionDone && !this.frozen) this.action.speedRatio = this.actionSpeed;
  }

  /** Volta ao normal (ao reviver). */
  reset(): void {
    if (this.stopLeft > 0) this.resume();
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
   * active = false (fora da tela ou longe): todos os pesos vão a 0 e o Babylon deixa de
   * calcular as animações; o esqueleto fica congelado na última pose até voltar.
   */
  update(dt: number, moving: boolean, runSpeedRatio: number, active = true): void {
    this.runRatio = runSpeedRatio;
    if (this.stopLeft > 0) {
      // Pausa de impacto: pose e pesos ficam como estão até ela acabar.
      this.stopLeft -= dt;
      if (this.stopLeft > 0) return;
      this.resume();
    }
    if (!active) {
      for (const g of this.anims.values()) g.weight = 0;
      return;
    }
    const k = 1 - Math.exp(-BLEND_RATE * dt);
    this.runWeight += ((moving && !this.holding ? 1 : 0) - this.runWeight) * k;
    // A ação chega ao fim no próximo quadro: para ali (velocidade 0, ainda na mistura). Em
    // loop o Babylon voltaria ao primeiro quadro, e o corpo pularia para o começo do golpe.
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
    if (idle) idle.weight = snap(loco * (1 - this.runWeight));
    if (run) {
      run.weight = snap(loco * this.runWeight);
      run.speedRatio = runSpeedRatio;
    }
    if (this.action) this.action.weight = Math.max(this.actionWeight, 0.001);
    for (const f of this.fading) f.g.weight = f.w;
  }
}

function snap(w: number): number {
  return w < 0.01 ? 0 : w > 0.99 ? 1 : w;
}

/** Quadros por segundo do clipe (os glTF vêm em 60, o golem em 30). */
function fps(g: AnimationGroup): number {
  return g.targetedAnimations[0]?.animation.framePerSecond || 60;
}
