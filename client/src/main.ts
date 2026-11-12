import type { ServerMessage, Welcome } from "./gen/erevia/v1/game_pb";
import { Connection } from "./net/connection";
import { EntityManager } from "./world/entities";
import { setupClickToMove } from "./world/input";
import { createWorldScene } from "./world/scene";

const NAME_RE = /^[A-Za-z0-9]{3,16}$/;

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;
const canvas = $<HTMLCanvasElement>("game");
const overlay = $<HTMLDivElement>("overlay");
const form = $<HTMLFormElement>("login");
const nameInput = $<HTMLInputElement>("name");
const errorEl = $<HTMLParagraphElement>("login-error");
const button = form.querySelector("button")!;

// Deixa o último nome usado preenchido (conveniência local).
try {
  nameInput.value = localStorage.getItem("erevia.name") ?? "";
} catch {
  /* armazenamento indisponível: ignora */
}

let reloadOnSubmit = false;

form.addEventListener("submit", (ev) => {
  ev.preventDefault();
  if (reloadOnSubmit) {
    location.reload();
    return;
  }
  const name = nameInput.value.trim();
  if (!NAME_RE.test(name)) {
    errorEl.textContent = "Nome inválido: use 3 a 16 letras ou números.";
    return;
  }
  try {
    localStorage.setItem("erevia.name", name);
  } catch {
    /* ignora */
  }
  errorEl.textContent = "";
  button.disabled = true;
  connect(name);
});

function connect(name: string): void {
  const conn = new Connection(Connection.defaultUrl(), name);
  let started = false;
  let entities: EntityManager | undefined;
  // Mensagens que chegam enquanto a cena 3D ainda está sendo criada.
  const backlog: ServerMessage[] = [];

  const apply = (em: EntityManager, msg: ServerMessage) => {
    switch (msg.msg.case) {
      case "spawn":
        if (msg.msg.value.entity) em.add(msg.msg.value.entity);
        break;
      case "despawn":
        em.remove(msg.msg.value.id);
        break;
      case "snapshot":
        for (const s of msg.msg.value.states) em.applyState(s);
        break;
    }
  };

  conn.onMessage = (msg) => {
    if (msg.msg.case === "welcome") {
      started = true;
      void startGame(conn, msg.msg.value).then((em) => {
        entities = em;
        for (const m of backlog.splice(0)) apply(em, m);
      });
      return;
    }
    if (entities) apply(entities, msg);
    else backlog.push(msg);
  };

  conn.onClose = (reason) => {
    button.disabled = false;
    overlay.hidden = false;
    if (started) {
      errorEl.textContent = `Desconectado: ${reason}.`;
      button.textContent = "Recarregar";
      reloadOnSubmit = true;
    } else {
      errorEl.textContent = `Não foi possível entrar: ${reason}. O servidor está rodando?`;
    }
  };
}

async function startGame(conn: Connection, welcome: Welcome): Promise<EntityManager> {
  overlay.hidden = true;
  $("hud").hidden = false;
  $("help").hidden = false;

  const world = await createWorldScene(canvas, welcome.zoneSize);
  const entities = new EntityManager(world.scene, world.gui, welcome.yourId, welcome.zoneSize / 2);
  for (const e of welcome.entities) entities.add(e);

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
  return entities;
}
