import { api, ApiError, type CharacterInfo } from "./api";
import { SkillTarget, type ServerMessage, type SkillInfo, type Welcome } from "./gen/erevia/v1/game_pb";
import { Connection } from "./net/connection";
import { Chat } from "./ui/chat";
import { showCharacters, showLogin, showMessage, showScreen } from "./ui/screens";
import { EntityManager } from "./world/entities";
import { Hud } from "./ui/hud";
import { Minimap } from "./ui/minimap";
import { WorldMap } from "./ui/worldmap";
import { setupPerfPanel } from "./ui/perf";
import { Fx } from "./world/fx";
import { setupInput } from "./world/input";
import { CharacterModels } from "./world/models";
import { loadEnvironment, type Environment } from "./world/environment";
import { DayNight, hourLabel, isNight, WorldClock } from "./world/daynight";
import { renderMapCanvas } from "./world/map-painter";
import { createWorldScene } from "./world/scene";
import { Collision, loadWorldData } from "./world/world-data";

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;
const canvas = $<HTMLCanvasElement>("game");

void boot();

/** Decide a tela inicial: login ou lista de personagens. */
async function boot(): Promise<void> {
  showScreen("screen-loading");

  // Erro devolvido pelo callback OAuth (?login_error=...). Limpa a URL.
  const params = new URLSearchParams(location.search);
  const loginError = params.get("login_error");
  if (loginError) history.replaceState(null, "", location.pathname);

  try {
    await api.me();
    await openCharacters();
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      try {
        showLogin(await api.providers(), loginError, () => void openCharacters());
      } catch (e) {
        showMessage("Servidor indisponível", (e as Error).message, "Tentar de novo", () => location.reload());
      }
      return;
    }
    showMessage("Servidor indisponível", (err as Error).message, "Tentar de novo", () => location.reload());
  }
}

async function openCharacters(): Promise<void> {
  try {
    await showCharacters(play, () => location.reload());
  } catch (err) {
    showMessage("Erro", (err as Error).message, "Tentar de novo", () => location.reload());
  }
}

/** Entra no mundo com o personagem escolhido. */
function play(character: CharacterInfo): void {
  showScreen("screen-loading");
  const conn = new Connection(Connection.defaultUrl(), character.id);
  let started = false;
  let kickedReason = "";
  let game: Game | undefined;
  // Mensagens que chegam enquanto a cena 3D ainda está sendo criada.
  const backlog: ServerMessage[] = [];

  conn.onMessage = (msg) => {
    if (msg.msg.case === "welcome") {
      started = true;
      void startGame(conn, character, msg.msg.value).then((g) => {
        game = g;
        for (const m of backlog.splice(0)) g.apply(m);
      });
      return;
    }
    if (msg.msg.case === "kicked") {
      kickedReason = msg.msg.value.reason;
      return;
    }
    if (game) game.apply(msg);
    else backlog.push(msg);
  };

  conn.onClose = (reason) => {
    $("chat").hidden = true;
    $("hud").hidden = true;
    for (const e of document.querySelectorAll(".game-hud, .minimap, .worldmap")) e.remove();
    const text = kickedReason || (started ? `A conexão caiu (${reason}).` : `Não foi possível entrar (${reason}).`);
    showMessage(started ? "Desconectado" : "Erro ao entrar", text, "Voltar", () => location.reload());
  };
}

interface Game {
  apply(msg: ServerMessage): void;
}

async function startGame(conn: Connection, character: CharacterInfo, welcome: Welcome): Promise<Game> {
  showScreen(null);
  $("hud").hidden = false;

  const worldData = await loadWorldData();
  const world = await createWorldScene(canvas, worldData);
  const fx = new Fx(world.scene, world.gui);
  const clock = new WorldClock(Number(welcome.serverTimeMs), welcome.dayCycleMs);
  const dayNight = new DayNight(world, worldData);
  const collision = new Collision(worldData);
  const entities = new EntityManager(world.scene, world.gui, welcome.yourId, welcome.zoneSize / 2, fx, world.shadows, collision);
  for (const e of welcome.entities) entities.add(e);

  // Cenário (árvores, cidade, montanhas) em segundo plano.
  let env: Environment | null = null;
  loadEnvironment(world.scene, worldData, world.shadows)
    .then((e) => {
      env = e;
      console.info(`cenário: ${e.drawn} objetos`);
    })
    .catch((err) => console.warn("cenário indisponível", err));

  // Modelos 3D carregam em segundo plano; até lá, cápsulas coloridas.
  CharacterModels.load(world.scene)
    .then((models) => entities.setModels(models))
    .catch((err) => console.warn("modelos 3D indisponíveis, usando cápsulas", err));

  const chat = new Chat($("chat"), character.name, (text, to) => conn.chat(text, to));
  chat.system(`Bem-vindo a Erevia, ${character.name}! Clique num monstro para atacar; teclas 1 e 2 usam suas skills.`);

  // ---------- Alvo e skills ----------
  let targetId = 0;
  const selectTarget = (id: number) => {
    targetId = id;
    entities.setTarget(id);
    hud.setTarget(entities.info(id));
  };
  const useSkill = (s: SkillInfo) => {
    if (entities.localDead) return;
    if (!hud.isReady(s.id)) return;
    if (!hud.hasMana(s)) {
      chat.system("Mana insuficiente.");
      return;
    }
    if (s.target === SkillTarget.ENEMY && !entities.isEnemy(targetId)) {
      chat.system("Selecione um monstro como alvo.");
      return;
    }
    conn.useSkill(s.id, targetId);
  };
  const hud = new Hud(welcome.skills, useSkill, () => conn.respawn());

  // ---------- Mapa ----------
  const mapSource = {
    world: worldData,
    mapCanvas: renderMapCanvas(worldData),
    forEachMarker: entities.forEachMarker.bind(entities),
    // A câmera fica em target + (cos α, sin α)·r e olha para o alvo: direção = -(cos α, sin α).
    cameraHeading: () => Math.atan2(-Math.cos(world.camera.alpha), -Math.sin(world.camera.alpha)),
  };
  const worldMap = new WorldMap(mapSource, welcome.zones, welcome.safeRadius);
  const minimap = new Minimap(mapSource, () => worldMap.toggle());

  window.addEventListener("keydown", (ev) => {
    if (document.activeElement instanceof HTMLInputElement) return; // digitando no chat
    if (ev.key === "m" || ev.key === "M") {
      ev.preventDefault();
      worldMap.toggle();
      return;
    }
    if (ev.key === "Escape") {
      if (worldMap.isOpen) worldMap.close(); // Esc fecha o mapa antes de limpar o alvo
      else selectTarget(0);
      return;
    }
    const s = welcome.skills.find((k) => String(k.key) === ev.key);
    if (s) {
      ev.preventDefault();
      useSkill(s);
    }
  });

  setupInput(world.scene, world.ground, {
    onGround: (x, z) => {
      const dest = entities.predictLocalMove(x, z);
      if (dest) conn.moveTo(dest.x, dest.z);
      return dest;
    },
    onEntity: (id) => {
      if (id === welcome.yourId) return;
      selectTarget(id);
      if (entities.isEnemy(id) && !entities.localDead) conn.attack(id);
    },
    isEnemy: (id) => entities.isEnemy(id),
  });

  // A câmera segue o jogador local.
  world.scene.onBeforeRenderObservable.add(() => {
    const dt = Math.min(world.engine.getDeltaTime() / 1000, 0.1);
    entities.update(dt);
    const pos = entities.localPosition();
    if (pos) {
      world.camera.target.x = pos.x;
      world.camera.target.y = 1.4;
      world.camera.target.z = pos.z;
      world.followShadows(pos.x, pos.z);
    }
    dayNight.update(clock.phase(), pos);
    // Árvores que tapam o jogador ou o alvo ficam translúcidas.
    env?.fader.update(dt, world.camera.position, entities.focusPoints());
  });

  // Painel de diagnóstico + quadro do alvo
  const fpsEl = $("hud-fps");
  const pingEl = $("hud-ping");
  const onlineEl = $("hud-online");
  const posEl = $("hud-pos");
  window.setInterval(() => {
    fpsEl.textContent = world.engine.getFps().toFixed(0);
    pingEl.textContent = `${conn.ping} ms`;
    onlineEl.textContent = String(entities.count);
    const p = entities.localPosition();
    posEl.textContent = p ? `${p.x.toFixed(1)}, ${p.z.toFixed(1)}` : "–";
    const phase = clock.phase();
    minimap.setClock(hourLabel(phase), isNight(phase), dayNight.night);
  }, 500);
  setupPerfPanel(world.scene, world.gui, $("hud"));

  canvas.focus();

  // Só em desenvolvimento: ganchos para testes automatizados no navegador.
  if (import.meta.env.DEV) {
    (window as unknown as Record<string, unknown>).__erevia = {
      entities,
      camera: () => ({ alpha: world.camera.alpha, beta: world.camera.beta, radius: world.camera.radius }),
      targetId: () => targetId,
      screenPos: (id: number) => entities.screenPos(id, world.scene),
      nearestMonster: () => entities.nearestMonster(),
      scene: world.scene,
      env: () => env,
      minimap,
      worldMap,
      /** Fixa a hora do dia (0–1) ou volta ao relógio do servidor (null). */
      setTime: (p: number | null) => clock.setFixed(p),
      phase: () => clock.phase(),
    };
  }

  return {
    apply(msg: ServerMessage) {
      switch (msg.msg.case) {
        case "spawn":
          if (msg.msg.value.entity) entities.add(msg.msg.value.entity);
          break;
        case "despawn":
          if (msg.msg.value.id === targetId) selectTarget(0);
          entities.remove(msg.msg.value.id);
          break;
        case "snapshot": {
          const snap = msg.msg.value;
          for (const s of snap.states) entities.applyState(s);
          for (const u of snap.hp) entities.applyHp(u);
          for (const ev of snap.combat) entities.onCombat(ev);
          for (const ev of snap.effects) entities.onEffect(ev);
          for (const id of snap.deaths) entities.onDeath(id);
          for (const r of snap.revives) entities.onRevive(r);
          for (const l of snap.levelUps) entities.onLevelUp(l);
          if (targetId) hud.setTarget(entities.info(targetId));
          break;
        }
        case "selfStats":
          hud.setStats(msg.msg.value);
          break;
        case "skillCooldown":
          hud.cooldown(msg.msg.value.skillId, msg.msg.value.readyInMs);
          break;
        case "chat":
          chat.receive(msg.msg.value);
          break;
      }
    },
  };
}
