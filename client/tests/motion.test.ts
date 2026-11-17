// Estado de movimento desenhado e velocidade dos clipes (client/src/world/motion.ts), sem navegador.
import assert from "node:assert/strict";
import { test } from "node:test";
import { MOVE_OFF_SPEED, MOVE_ON_SPEED, stepRatios, updateMotion, type MotionState } from "../src/world/motion.ts";

const fresh = (): MotionState => ({ px: 0, pz: 0, vis: 0, walking: false });
/** Anda `seconds` s a `speed` m/s em z, a 60 quadros por segundo; devolve o estado e a lista de `walking` por quadro. */
function walkFor(s: MotionState, speed: number, seconds: number, startZ = s.pz): boolean[] {
  const flags: boolean[] = [];
  let z = startZ;
  for (let i = 0; i < Math.round(seconds * 60); i++) {
    z += speed / 60;
    updateMotion(s, 0, z, 1 / 60);
    flags.push(s.walking);
  }
  return flags;
}

test("anda assim que a posição desenhada anda, mesmo sem destino do servidor (jogador pelo teclado)", () => {
  const s = fresh();
  const flags = walkFor(s, 5, 1);
  assert.equal(flags.indexOf(true) <= 3, true, "liga em até 4 quadros (~65 ms)");
  assert.equal(flags.slice(6).every(Boolean), true);
  assert.ok(Math.abs(s.vis - 5) < 0.1, `velocidade desenhada ${s.vis}`);
});

test("para só quando a posição desenhada para, não quando o servidor chega ao destino", () => {
  const s = fresh();
  walkFor(s, 5, 1);
  // a posição desenhada ainda se aproxima do destino, devagar (suavização): continua andando
  let z = s.pz;
  let still = 0;
  for (let i = 0; i < 60; i++) {
    z += 0.5 * 0.9 ** i * 0.1;
    updateMotion(s, 0, z, 1 / 60);
    if (s.walking) still++;
  }
  assert.ok(still > 0, "enquanto desliza até o destino, anda");
  for (let i = 0; i < 30; i++) updateMotion(s, 0, z, 1 / 60);
  assert.equal(s.walking, false);
});

test("histerese: não liga e desliga no limiar", () => {
  const s = fresh();
  const between = (MOVE_ON_SPEED + MOVE_OFF_SPEED) / 2;
  walkFor(s, between, 1);
  assert.equal(s.walking, false, "abaixo do limiar de ligar");
  walkFor(s, MOVE_ON_SPEED * 2, 0.5);
  assert.equal(s.walking, true);
  walkFor(s, between, 1);
  assert.equal(s.walking, true, "entre os dois limiares continua andando");
});

test("teletransporte não conta como andar", () => {
  const s = fresh();
  updateMotion(s, 0, 30, 1 / 60);
  assert.equal(s.walking, false);
  assert.equal(s.vis, 0);
});

test("o clipe toca na velocidade do chão (pé não desliza)", () => {
  // KayKit: o correr é de 1,98 m/s; a 5 m/s tem de tocar a 2,5x
  const k = stepRatios(5, 1.98, 0);
  assert.ok(Math.abs(k.run - 5 / 1.98) < 1e-9);
  assert.equal(k.walk, false);
  // Meshy: anda a 1,36 e corre a 4,13; o cruzamento fica na média geométrica (2,37 m/s)
  assert.equal(stepRatios(2, 4.13, 1.36).walk, true);
  assert.equal(stepRatios(3, 4.13, 1.36).walk, false);
  const w = stepRatios(2, 4.13, 1.36);
  assert.ok(Math.abs(w.walkRatio - 2 / 1.36) < 1e-9);
  // limites: nunca para nem passa de 3x
  assert.equal(stepRatios(0, 4, 1).run, 0.35);
  assert.equal(stepRatios(40, 1, 0).run, 3);
});
