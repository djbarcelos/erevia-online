import type { AnimationGroup } from "@babylonjs/core";
import type { AnimRole } from "./models";

/** Velocidade das transições de peso (1/s). */
const BLEND_RATE = 10;

/**
 * Controla as animações de um personagem em duas camadas:
 * - locomoção: parado ↔ correndo, sempre tocando, misturadas por peso;
 * - ação: ataque/skill/apanhar/morrer, toca uma vez por cima e devolve o corpo.
 * A morte segura a pose final até o personagem reviver.
 */
export class AnimController {
  private runWeight = 0;
  private actionWeight = 0;
  private action: AnimationGroup | null = null;
  private actionDone = true;
  private holding = false;

  constructor(private anims: Map<AnimRole, AnimationGroup>) {
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
  }

  /** Toca uma ação uma vez. hold = mantém a pose final (morte). */
  play(role: AnimRole, opts: { hold?: boolean; speed?: number } = {}): void {
    const g = this.anims.get(role);
    if (!g) return;
    if (this.holding && !opts.hold) return; // morto não reage
    // Apanhar não interrompe um ataque em andamento.
    if (role === "hit" && this.action && !this.actionDone && this.action !== g) return;

    if (this.action && this.action !== g) this.action.stop();
    g.stop(); // start() não faz nada num grupo já iniciado (mesmo pausado)
    this.action = g;
    this.actionDone = false;
    this.holding = !!opts.hold;
    g.speedRatio = opts.speed ?? 1;
    // Toca em loop para o grupo não ser removido ao terminar (senão o corpo "pisca" para a
    // pose de repouso durante a transição). Na primeira volta, congela no último quadro:
    // a morte fica assim; as outras ações saem aos poucos pelo peso em update().
    g.start(true, g.speedRatio);
    g.onAnimationGroupLoopObservable.addOnce(() => {
      if (this.action !== g) return;
      g.goToFrame(g.to);
      g.pause();
      if (!this.holding) this.actionDone = true;
    });
  }

  /** Volta ao normal (ao reviver). */
  reset(): void {
    this.holding = false;
    this.actionDone = true;
    this.action?.stop();
    this.action = null;
    this.actionWeight = 0;
  }

  /**
   * active = false (fora da tela ou longe): todos os pesos vão a 0 e o Babylon deixa de
   * calcular as animações; o esqueleto fica congelado na última pose até voltar.
   */
  update(dt: number, moving: boolean, runSpeedRatio: number, active = true): void {
    if (!active) {
      for (const g of this.anims.values()) g.weight = 0;
      return;
    }
    const k = 1 - Math.exp(-BLEND_RATE * dt);
    this.runWeight += ((moving && !this.holding ? 1 : 0) - this.runWeight) * k;
    const actionActive = this.action !== null && (!this.actionDone || this.holding);
    this.actionWeight += ((actionActive ? 1 : 0) - this.actionWeight) * k * 1.5;
    if (!actionActive && this.actionWeight < 0.01 && this.action) {
      this.action.stop();
      this.action = null;
    }

    const loco = 1 - this.actionWeight;
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
  }
}

function snap(w: number): number {
  return w < 0.01 ? 0 : w > 0.99 ? 1 : w;
}
