// Testes do "Ir" e do "Caçar aqui" das missões (client/src/world/questgo.ts), sem
// navegador: `npm run test:unit` (Node 22.6+ roda TypeScript tirando os tipos).
import assert from "node:assert/strict";
import { test } from "node:test";
import {
  IDLE_LIMIT_MS,
  QuestGo,
  STALL_MS,
  huntAllows,
  nearestAllowed,
  stoneOffer,
  type GoTarget,
  type QuestGoDeps,
  type StoneInfo,
} from "../src/world/questgo.ts";

/** Mundo de mentira: o personagem anda 1 m por tick na direção do último destino. */
function rig(opts: { keepIfHit?: boolean; gold?: number } = {}) {
  const log: string[] = [];
  const s = {
    t: 0,
    pos: { x: 0, z: 0 },
    dest: null as { x: number; z: number } | null,
    focused: true,
    dead: false,
    hunt: null as GoTarget | null,
    walks: 0,
    halts: 0,
    stones: [] as number[],
    /** O servidor não deixa sair do lugar (obstáculo, sem caminho). */
    stuck: false,
  };
  const deps: QuestGoDeps = {
    now: () => s.t,
    position: () => s.pos,
    dead: () => s.dead,
    focused: () => s.focused,
    keepIfHit: () => opts.keepIfHit ?? false,
    gold: () => opts.gold ?? 1000,
    walk: (x, z) => {
      s.walks++;
      if (!s.stuck) s.dest = { x, z };
    },
    halt: () => {
      s.halts++;
      s.dest = null;
    },
    openStone: (id) => s.stones.push(id),
    startHunt: (t) => (s.hunt = t),
    stopHunt: () => (s.hunt = null),
    hunting: () => s.hunt !== null,
    notify: (text) => log.push(text),
    changed: () => {},
    every: () => () => {},
  };
  const go = new QuestGo(deps);
  /** Avança n ticks de 250 ms (andando, se houver destino). */
  const run = (n: number) => {
    for (let i = 0; i < n; i++) {
      s.t += 250;
      if (s.dest) {
        const dx = s.dest.x - s.pos.x;
        const dz = s.dest.z - s.pos.z;
        const d = Math.hypot(dx, dz);
        const k = Math.min(1, 1 / Math.max(d, 1e-9));
        s.pos = { x: s.pos.x + dx * k, z: s.pos.z + dz * k };
      }
      go.tick();
    }
  };
  return { s, go, log, run };
}

const hunt = (over: Partial<GoTarget> = {}): GoTarget => ({
  questId: "tut_hunt",
  step: 0,
  x: 60,
  z: 0,
  radius: 8,
  name: "Esqueleto Lacaio",
  hunt: true,
  monsters: ["Esqueleto Lacaio"],
  ...over,
});

test("Ir anda até o lugar com o comando de andar e oferece Caçar aqui ao chegar", () => {
  const { s, go, log, run } = rig();
  go.go(hunt(), []);
  assert.equal(go.phase, "walking");
  assert.deepEqual(s.dest, { x: 60, z: 0 });
  run(60);
  assert.equal(go.phase, "arrived");
  assert.ok(Math.hypot(s.pos.x - 60, s.pos.z) <= 8, "parou dentro do raio");
  assert.ok(s.pos.x < 54, `parou ao entrar no raio, não no centro (x = ${s.pos.x})`);
  assert.equal(s.halts, 1);
  assert.match(log.at(-1)!, /Caçar aqui/);
});

test("Ir para quando o jogador mexe no movimento ou ataca (sem mandar parar por cima)", () => {
  const { s, go, run } = rig();
  go.go(hunt(), []);
  run(5);
  go.onPlayerControl(); // WASD, joystick, clique no chão ou ataque
  assert.equal(go.phase, "idle");
  const walks = s.walks;
  run(40);
  assert.equal(s.walks, walks, "não reenvia nada depois");
  assert.equal(s.halts, 0, "o comando do jogador manda; o Ir não para por cima");
});

test("Ir para ao levar dano, a não ser com 'Continuar andando se atacado'", () => {
  const a = rig();
  a.go.go(hunt(), []);
  a.run(3);
  a.go.onHit(0); // errou: não conta
  assert.equal(a.go.phase, "walking");
  a.go.onHit(12);
  assert.equal(a.go.phase, "idle");
  assert.equal(a.s.halts, 1);
  assert.match(a.log.at(-1)!, /atacado/);

  const b = rig({ keepIfHit: true });
  b.go.go(hunt(), []);
  b.run(3);
  b.go.onHit(12);
  assert.equal(b.go.phase, "walking");
  b.run(60);
  assert.equal(b.go.phase, "arrived");
});

test("Caçar aqui liga a caça filtrada e para (avisando) quando o passo termina", () => {
  const { s, go, log, run } = rig();
  go.go(hunt(), []);
  run(60);
  go.huntHere();
  assert.equal(go.phase, "hunting");
  assert.deepEqual(s.hunt?.monsters, ["Esqueleto Lacaio"]);
  go.questUpdate(hunt()); // progresso no mesmo passo: segue
  assert.equal(go.phase, "hunting");
  go.questUpdate(null); // passo concluído (a missão acabou)
  assert.equal(go.phase, "idle");
  assert.equal(s.hunt, null);
  assert.match(log.at(-1)!, /Passo concluído/);
});

test("Caçar aqui: se a auto-caça parar sozinha (F, morte, sem poção), o Ir volta a ficar parado", () => {
  const { s, go, run } = rig();
  go.go(hunt({ x: 3 }), []); // já está dentro
  assert.equal(go.phase, "arrived");
  go.huntHere();
  s.hunt = null;
  run(1);
  assert.equal(go.phase, "idle");
});

test("contra bot: só com a janela em foco e para depois de 5 minutos sem interação real", () => {
  const { s, go, log, run } = rig();
  s.focused = false;
  go.go(hunt(), []);
  assert.equal(go.phase, "idle");
  assert.match(log.at(-1)!, /foco/);

  s.focused = true;
  go.go(hunt({ x: 2 }), []);
  go.huntHere();
  run((IDLE_LIMIT_MS - 1000) / 250);
  assert.equal(go.phase, "hunting");
  go.input(); // o jogador mexeu: conta de novo
  run((IDLE_LIMIT_MS - 1000) / 250);
  assert.equal(go.phase, "hunting");
  run(8);
  assert.equal(go.phase, "idle");
  assert.equal(s.hunt, null);
  assert.match(log.at(-1)!, /5 minutos/);

  go.go(hunt(), []);
  s.focused = false; // trocou de janela no meio do caminho
  run(1);
  assert.equal(go.phase, "idle");
});

test("parado sem chegar: reenvia o andar a cada STALL_MS e desiste depois de algumas tentativas", () => {
  const { s, go, log, run } = rig();
  s.stuck = true;
  go.go(hunt(), []);
  const w = s.walks;
  run(STALL_MS / 250 + 1);
  assert.equal(s.walks, w + 1);
  run(80);
  assert.equal(go.phase, "idle");
  assert.match(log.at(-1)!, /caminho/);
});

test("filtro da caça: só os monstros do passo; vazio = qualquer um", () => {
  const near = [
    { id: 1, name: "Esqueleto Guerreiro", dist: 3 },
    { id: 2, name: "Esqueleto Lacaio", dist: 9 },
    { id: 3, name: "Esqueleto Lacaio", dist: 6 },
  ];
  assert.equal(nearestAllowed(near, ["Esqueleto Lacaio"])?.id, 3);
  assert.equal(nearestAllowed(near, [])?.id, 1);
  assert.equal(nearestAllowed(near, null)?.id, 1);
  assert.equal(nearestAllowed(near, ["Golem de Pedra"]), null);
  assert.equal(huntAllows(["Esqueleto Lacaio"], "Esqueleto Guerreiro"), false);
  assert.equal(huntAllows(["Esqueleto Lacaio"], undefined), false);
  assert.equal(huntAllows(["Esqueleto Lacaio", "Esqueleto Ladino"], "Esqueleto Ladino"), true);
});

// As pedras como o servidor monta (teleport.go): cada uma lista as outras, e o preço é o
// de chegar ao destino.
const STONES = [
  { name: "Cidade", x: 7, z: 7, price: 20 },
  { name: "Arredores", x: 60, z: 5, price: 40 },
  { name: "Campos dos Ossos", x: 5, z: 94, price: 90 },
  { name: "Ermos Partidos", x: -128, z: 5, price: 160 },
  { name: "Vale das Pedras Vivas", x: 6, z: -198, price: 220 },
];
const stones: StoneInfo[] = STONES.map((s, i) => ({
  npcId: 100 + i,
  name: s.name,
  x: s.x,
  z: s.z,
  dests: STONES.flatMap((d, j) => (j === i ? [] : [{ id: j + 1, name: d.name, x: d.x, z: d.z, price: d.price }])),
}));

test("oferta da pedra: só longe (mais de 150 m) e quando encurta o caminho", () => {
  // Da praça ao fundo do vale: ~270 m a pé; pela pedra, 3 m + ~75 m.
  const o = stoneOffer({ x: 4, z: 4 }, { x: 0, z: -270 }, stones, 500);
  assert.ok(o);
  assert.equal(o.from, "Cidade");
  assert.equal(o.to, "Vale das Pedras Vivas");
  assert.equal(o.price, 220);
  assert.equal(o.afford, true);
  assert.equal(stoneOffer({ x: 4, z: 4 }, { x: 0, z: -270 }, stones, 100)?.afford, false);
  // Perto: nada.
  assert.equal(stoneOffer({ x: 0, z: 0 }, { x: 60, z: 40 }, stones, 500), null);
  // Longe, mas nenhuma pedra ajuda (está entre as pedras, no meio do nada a leste).
  assert.equal(stoneOffer({ x: 140, z: -140 }, { x: 140, z: 40 }, stones, 500), null);
});

test("Viajar pela Pedra: anda até a pedra, espera o jogador escolher e segue do destino", () => {
  const { s, go, run } = rig();
  s.pos = { x: 4, z: 4 };
  go.go(hunt({ x: 0, z: -270, radius: 10 }), stones);
  assert.equal(go.offer?.to, "Vale das Pedras Vivas");
  go.takeStone();
  assert.equal(go.phase, "toStone");
  assert.deepEqual(s.stones, [100], "abre a pedra da Cidade (não viaja sozinho)");
  s.dest = { x: 7, z: 7 };
  run(30); // chegou na pedra: espera a escolha sem desistir
  assert.equal(go.phase, "toStone");
  s.pos = { x: 7, z: -197 }; // o jogador escolheu e viajou
  s.dest = null;
  run(1);
  assert.equal(go.phase, "walking");
  assert.deepEqual(s.dest, { x: 0, z: -270 });
  run(120);
  assert.equal(go.phase, "arrived");
});

test("Ir para um NPC para ao chegar, sem oferecer caça", () => {
  const { go, log, run } = rig();
  go.go({ questId: "q", step: 0, x: 0, z: 20, radius: 2.5, name: "Ysolde", hunt: false, monsters: [] }, []);
  run(30);
  assert.equal(go.phase, "idle");
  assert.match(log.at(-1)!, /Você chegou: Ysolde/);
});
