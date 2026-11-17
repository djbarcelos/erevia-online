/**
 * Movimento desenhado dos personagens (sem Babylon, para testar): quando o corpo "anda" de verdade e quão rápido o
 * clipe de andar ou correr deve tocar para o pé não deslizar no chão.
 *
 * O jogo desenhava `moving = destino do servidor != posição extrapolada`. Para o jogador que anda pelo teclado (o
 * cliente dirige a posição) o destino é sempre igual à posição, então `moving` ficava falso e o personagem
 * deslizava tocando o parado; nos outros, `moving` caía assim que o servidor chegava ao destino, antes de a posição
 * desenhada (suavizada) alcançá-lo. Agora o estado vem da velocidade da posição DESENHADA.
 */

/** Velocidade desenhada (m/s) acima da qual o personagem anda (liga) e abaixo da qual para (desliga): histerese. */
export const MOVE_ON_SPEED = 0.3;
export const MOVE_OFF_SPEED = 0.12;
/** Acima disto (m/s) a posição saltou (teleporte): não conta como andar. */
export const TELEPORT_SPEED = 40;
/** Suavização da velocidade desenhada (1/s): ~70 ms. */
export const VIS_SMOOTH_RATE = 14;
/** Limites da velocidade do clipe (1 = como foi feito) quando ela acompanha o chão. */
export const MIN_STEP_RATIO = 0.35;
export const MAX_STEP_RATIO = 3;

export interface MotionState {
  /** Posição desenhada no quadro anterior. */
  px: number;
  pz: number;
  /** Velocidade desenhada (m/s), suavizada. */
  vis: number;
  /** Está andando de fato (com histerese). */
  walking: boolean;
}

/** Atualiza a velocidade desenhada e o estado de andar com a posição desenhada deste quadro. */
export function updateMotion(s: MotionState, x: number, z: number, dt: number, dead = false): void {
  const drawn = dt > 0 ? Math.hypot(x - s.px, z - s.pz) / dt : 0;
  s.px = x;
  s.pz = z;
  if (drawn < TELEPORT_SPEED) s.vis += (drawn - s.vis) * (1 - Math.exp(-VIS_SMOOTH_RATE * dt));
  else s.vis = 0;
  s.walking = s.vis > (s.walking ? MOVE_OFF_SPEED : MOVE_ON_SPEED) && !dead;
}

const clampRatio = (r: number) => Math.min(MAX_STEP_RATIO, Math.max(MIN_STEP_RATIO, r));

/**
 * Velocidade do clipe de correr e de andar para a velocidade desenhada `speed`, dadas as velocidades naturais dos
 * clipes (m/s do mundo, já com a escala do corpo; walkNat 0 = o modelo não tem clipe de andar). `walk` diz se
 * entra o clipe de andar: abaixo da velocidade em que os dois tocam na mesma proporção da passada natural.
 */
export function stepRatios(speed: number, runNat: number, walkNat: number): { run: number; walkRatio: number; walk: boolean } {
  return {
    run: clampRatio(speed / runNat),
    walkRatio: walkNat ? clampRatio(speed / walkNat) : 1,
    walk: walkNat > 0 && speed < Math.sqrt(walkNat * runNat),
  };
}
