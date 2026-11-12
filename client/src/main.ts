import { api, ApiError, type CharacterInfo } from "./api";
import type { ServerMessage, Welcome } from "./gen/erevia/v1/game_pb";
import { Connection } from "./net/connection";
import { Chat } from "./ui/chat";
import { showCharacters, showLogin, showMessage, showScreen } from "./ui/screens";
import { EntityManager } from "./world/entities";
import { setupClickToMove } from "./world/input";
import { CharacterModels } from "./world/models";
import { createWorldScene } from "./world/scene";

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
  let game: { entities: EntityManager; chat: Chat } | undefined;
  // Mensagens que chegam enquanto a cena 3D ainda está sendo criada.
  const backlog: ServerMessage[] = [];

  const apply = (msg: ServerMessage) => {
    if (!game) return;
    switch (msg.msg.case) {
      case "spawn":
        if (msg.msg.value.entity) game.entities.add(msg.msg.value.entity);
        break;
      case "despawn":
        game.entities.remove(msg.msg.value.id);
        break;
      case "snapshot":
        for (const s of msg.msg.value.states) game.entities.applyState(s);
        break;
      case "chat":
        game.chat.receive(msg.msg.value);
        break;
    }
  };

  conn.onMessage = (msg) => {
    if (msg.msg.case === "welcome") {
      started = true;
      void startGame(conn, character, msg.msg.value).then((g) => {
        game = g;
        for (const m of backlog.splice(0)) apply(m);
      });
      return;
    }
    if (msg.msg.case === "kicked") {
      kickedReason = msg.msg.value.reason;
      return;
    }
    if (game) apply(msg);
    else backlog.push(msg);
  };

  conn.onClose = (reason) => {
    $("chat").hidden = true;
    $("hud").hidden = true;
    $("help").hidden = true;
    const text = kickedReason || (started ? `A conexão caiu (${reason}).` : `Não foi possível entrar (${reason}).`);
    showMessage(started ? "Desconectado" : "Erro ao entrar", text, "Voltar", () => location.reload());
  };
}

async function startGame(
  conn: Connection,
  character: CharacterInfo,
  welcome: Welcome,
): Promise<{ entities: EntityManager; chat: Chat }> {
  showScreen(null);
  $("hud").hidden = false;
  $("help").hidden = false;

  const world = await createWorldScene(canvas, welcome.zoneSize);
  const entities = new EntityManager(world.scene, world.gui, welcome.yourId, welcome.zoneSize / 2);
  for (const e of welcome.entities) entities.add(e);

  // Modelos 3D carregam em segundo plano; até lá, cápsulas na cor da classe.
  CharacterModels.load(world.scene)
    .then((models) => entities.setModels(models))
    .catch((err) => console.warn("modelos 3D indisponíveis, usando cápsulas", err));

  const chat = new Chat($("chat"), character.name, (text, to) => conn.chat(text, to));
  chat.system(`Bem-vindo a Erevia, ${character.name}! Enter para conversar, /w Nome para sussurrar.`);

  setupClickToMove(world.scene, world.ground, (x, z) => {
    const dest = entities.predictLocalMove(x, z);
    conn.moveTo(dest.x, dest.z);
    return dest;
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
    }
  });

  // HUD
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
  }, 500);

  canvas.focus();
  return { entities, chat };
}
