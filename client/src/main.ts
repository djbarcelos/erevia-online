import { api, ApiError, type CharacterInfo } from "./api";
import { Color3 } from "@babylonjs/core";
import { create } from "@bufbuild/protobuf";
import {
  EntityKind,
  ItemKind,
  PartyCmdKind,
  PartyStateSchema,
  SelfPvpSchema,
  SkillTarget,
  type PartyState,
  type Progress,
  type SelfPvp,
  type SelfStats,
  type ServerMessage,
  type SkillInfo,
  type Welcome,
} from "./gen/erevia/v1/game_pb";
import { classInfo } from "./classes";
import { Connection } from "./net/connection";
import { Chat } from "./ui/chat";
import { showCharacters, showLogin, showMessage, showScreen } from "./ui/screens";
import { EntityManager } from "./world/entities";
import { Hud, HOTBAR_KEYS, HOTBAR_SIZE, type TargetExtras, type WindowId } from "./ui/hud";
import { setupPerfPanel } from "./ui/perf";
import { GRADE_COLOR, itemLabel, ItemStore, ItemTip, PLACE_BAG } from "./ui/items";
import { ShopWindow } from "./ui/shop";
import { applyEnvClasses, isCompact, settings, toast, WindowManager, type WindowDef } from "./ui/kit";
import { DEFAULT_LOOK } from "./ui/looks";
import { buildRegions } from "./ui/regions";
import { buildInventory } from "./ui/windows/inventory";
import { buildWorldMap } from "./ui/windows/worldmap";
import { buildSettings } from "./ui/windows/settings";
import { buildCharacter } from "./ui/windows/character";
import { buildSkills } from "./ui/windows/skills";
import { buildParty } from "./ui/windows/party";
import { InvitePrompt } from "./ui/invite";
import { AutoHunt } from "./world/autohunt";
import { GroundLoot } from "./world/loot";
import { Fx } from "./world/fx";
import { setupInput } from "./world/input";
import { setupTouchCamera } from "./world/touch-camera";
import { CharacterModels } from "./world/models";
import { loadEnvironment, type Environment } from "./world/environment";
import { DayNight, WorldClock } from "./world/daynight";
import { renderMapCanvas } from "./world/map-painter";
import { createWorldScene } from "./world/scene";
import { Collision, loadWorldData } from "./world/world-data";

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;
const canvas = $<HTMLCanvasElement>("game");

// Classes de ambiente (toque, escala da interface, canhoto) seguem as configurações.
applyEnvClasses();
settings.on(applyEnvClasses);
addEventListener("resize", applyEnvClasses);

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
    game?.dispose();
    for (const e of document.querySelectorAll(".hud, .gwin-layer, .toasts, .tip, [data-menu], [data-sheet]")) e.remove();
    const text = kickedReason || (started ? `A conexão caiu (${reason}).` : `Não foi possível entrar (${reason}).`);
    showMessage(started ? "Desconectado" : "Erro ao entrar", text, "Voltar", () => location.reload());
  };
}

interface Game {
  apply(msg: ServerMessage): void;
  /** Para os loops e ouvintes do jogo (ao desconectar). */
  dispose(): void;
}

async function startGame(conn: Connection, character: CharacterInfo, welcome: Welcome): Promise<Game> {
  showScreen(null);

  const worldData = await loadWorldData();
  const world = await createWorldScene(canvas, worldData);
  const clock = new WorldClock(Number(welcome.serverTimeMs), welcome.dayCycleMs);
  const dayNight = new DayNight(world, worldData);
  const fx = new Fx(world.scene, world.gui, dayNight.glow, world.camera);
  const collision = new Collision(worldData);
  const entities = new EntityManager(world.scene, world.gui, welcome.yourId, welcome.zoneSize / 2, fx, world.shadows, collision);
  entities.setSkillCatalog([...welcome.skillCatalog, ...welcome.skills]);
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

  // ---------- Estado do jogador ----------
  const emblem = character.emblem ?? DEFAULT_LOOK[character.class]?.emblem ?? "#d6b46e";
  let lastStats: SelfStats | null = null;
  const statsListeners = new Set<() => void>();
  const onStats = (fn: () => void) => {
    statsListeners.add(fn);
    return () => statsListeners.delete(fn);
  };
  const me = () => ({ myClass: character.class, myLevel: lastStats?.level ?? character.level ?? 1, name: character.name, emblem });

  // ---------- PvP (karma, PKs, vitórias, flag) ----------
  let pvp: SelfPvp = welcome.pvp ?? create(SelfPvpSchema, {});
  let pvpAt = performance.now();
  const pvpListeners = new Set<() => void>();
  const onPvp = (fn: () => void) => {
    pvpListeners.add(fn);
    return () => pvpListeners.delete(fn);
  };
  /** Segundos de flag que ainda faltam. */
  const flagLeft = () => Math.max(0, (pvp.flagMs - (performance.now() - pvpAt)) / 1000);

  // ---------- Grupo ----------
  let party: PartyState = create(PartyStateSchema, {});
  const partyListeners = new Set<() => void>();
  const onParty = (fn: () => void) => {
    partyListeners.add(fn);
    return () => partyListeners.delete(fn);
  };
  const inParty = (id: number) => party.members.some((m) => m.id === id);
  /** Convidar: ele fora do grupo, e você sem grupo ou líder. */
  const canInviteParty = (id: number) =>
    !inParty(id) && (party.members.length === 0 || party.leaderId === welcome.yourId) && party.members.length < 6;
  const inviteParty = (name: string) => conn.partyCmd(PartyCmdKind.PARTY_CMD_INVITE, name);
  const PARTY_NEAR = 30;

  // ---------- Clã (bloco 3) ----------
  const canInviteClan = (_id: number) => false;
  const inviteClan = (_name: string) => {};

  // ---------- Progressão: atributos, árvore de skills e barra (vêm do servidor) ----------
  let progress: Progress | null = welcome.progress ?? null;
  const progressListeners = new Set<() => void>();
  const onProgress = (fn: () => void) => {
    progressListeners.add(fn);
    return () => progressListeners.delete(fn);
  };
  const rankOf = (id: number) => progress?.ranks.find((r) => r.skillId === id)?.rank ?? 0;
  /**
   * Skill como ela é agora: valores do rank aprendido, mana com Sobrecarga e recarga
   * dividida pela velocidade de conjuração (mesma conta do servidor).
   */
  const effectiveSkill = (s: SkillInfo): SkillInfo => {
    const r = Math.max(1, rankOf(s.id));
    const ri = s.ranks[Math.min(r, s.ranks.length) - 1];
    if (!ri) return s;
    const overload = welcome.skills.find((x) => x.fx === "m_over");
    const mpMult = overload && rankOf(overload.id) > 0 ? 1.1 : 1;
    const cs = lastStats?.castSpeed || 1;
    return {
      ...s,
      mpCost: Math.round(ri.mpCost * mpMult),
      cooldownMs: Math.round(ri.cooldownMs / cs),
      power: ri.power,
      effectMs: ri.effectMs || s.effectMs,
      effectValue: ri.effectValue || s.effectValue,
    };
  };
  /** Ativas aprendidas (as que podem ir para a barra e ser usadas). */
  const learnedActives = () => welcome.skills.filter((s) => !s.passive && rankOf(s.id) > 0).map(effectiveSkill);

  // ---------- Itens ----------
  const store = new ItemStore(welcome.itemCatalog);
  store.setInventory(welcome.inventory);
  const itemTip = new ItemTip(document.body);
  const npcStock = new Map(welcome.npcs.map((n) => [n.id, n.stock]));
  const loot = new GroundLoot(world.scene, world.gui, store, welcome.yourId);
  loot.add(welcome.groundItems);

  // ---------- Barra de atalhos (10 teclas): salva no servidor ----------
  // A barra salva no navegador (versão anterior) é enviada uma vez ao servidor e apagada.
  const migrateLocalHotbar = () => {
    const key = `erevia.hotbar2.${character.id}`;
    try {
      const saved = JSON.parse(localStorage.getItem(key) ?? "null");
      localStorage.removeItem(key);
      localStorage.removeItem(`erevia.hotbar.${character.id}`);
      if (Array.isArray(saved) && saved.length === HOTBAR_SIZE && saved.some((e) => e)) {
        const valid = saved.map((e: string | null) => {
          if (!e) return "";
          if (e.startsWith("skill:")) return rankOf(Number(e.slice(6))) > 0 ? e : "";
          const info = store.get(e.slice(5));
          return info && (info.healHp || info.healMp) ? e : "";
        });
        conn.setHotbar(valid);
      }
    } catch {
      /* sem storage: fica a do servidor */
    }
  };

  const useItemById = (id: string): "ok" | "cooldown" | "none" => {
    const info = store.get(id);
    if (!info || entities.localDead) return "none";
    const slot = store.slotOf(info.id);
    if (slot < 0) {
      chat.system(`Você não tem ${info.name}.`);
      return "none";
    }
    if (info.cooldownGroup && !hud.itemReady(info.cooldownGroup)) return "cooldown";
    conn.useItem(PLACE_BAG, slot);
    return "ok";
  };

  // ---------- Alvo e skills ----------
  let targetId = 0;
  /** Jogador atacado com Ctrl (ou pelo botão Atacar): as skills nele também forçam. */
  let forcedId = 0;
  const targetExtras = (id: number): TargetExtras | undefined =>
    entities.isPlayer(id) && id !== welcome.yourId
      ? { ally: entities.isAlly(id), canParty: canInviteParty(id), canClan: canInviteClan(id) }
      : undefined;
  const refreshTarget = () => hud.setTarget(entities.info(targetId), targetExtras(targetId));
  const selectTarget = (id: number) => {
    if (id !== targetId) {
      hud.setQueued(0); // o servidor troca a ação ao mudar de alvo
      forcedId = 0;
    }
    targetId = id;
    entities.setTarget(id);
    refreshTarget();
  };
  /** Atacar um monstro (auto-caça, botão de ataque, clique). */
  const attackTarget = (id: number) => {
    if (entities.localDead || !entities.isEnemy(id)) return;
    selectTarget(id);
    pendingShop = 0;
    conn.attack(id);
  };
  /** Atacar um jogador: só se for hostil, ou com force (Ctrl / botão Atacar). */
  const attackPlayer = (id: number, force: boolean) => {
    if (entities.localDead || !entities.canAttack(id, force)) return;
    selectTarget(id);
    pendingShop = 0;
    if (force) forcedId = id;
    conn.attack(id, force);
  };
  const isAttackable = (id: number) => entities.isEnemy(id) || entities.canAttack(id, forcedId === id);
  const useSkill = (s: SkillInfo) => {
    if (entities.localDead || s.passive || rankOf(s.id) < 1) return;
    if (!hud.isReady(s.id)) return;
    if (!hud.hasMana(s)) {
      chat.system("Mana insuficiente.");
      return;
    }
    const onPlayer = s.target === SkillTarget.ENEMY && entities.isPlayer(targetId) && entities.canAttack(targetId, forcedId === targetId);
    if (s.target === SkillTarget.ENEMY && !entities.isEnemy(targetId) && !onPlayer) {
      const n = entities.nearestMonster();
      if (!n || n.dist > 25) {
        chat.system("Selecione um monstro como alvo.");
        return;
      }
      selectTarget(n.id);
    }
    conn.useSkill(s.id, targetId, onPlayer && forcedId === targetId);
    // Skill de alvo pode precisar andar até o alcance: fica "na fila" até o servidor usar.
    if (s.target === SkillTarget.ENEMY) hud.setQueued(s.id);
    else hud.startLocalCooldown(s.id);
  };
  /** Próximo monstro vivo, do mais perto para o mais longe (tecla Tab / botão de alvo). */
  const cycleTarget = () => {
    const p = entities.localPosition();
    if (!p) return;
    const list: { id: number; d: number }[] = [];
    entities.forEachMarker((m) => {
      if (m.kind === EntityKind.MONSTER) list.push({ id: m.id, d: Math.hypot(m.x - p.x, m.z - p.z) });
    });
    list.sort((a, b) => a.d - b.d);
    const near = list.filter((m) => m.d < 30);
    if (!near.length) return;
    const i = near.findIndex((m) => m.id === targetId);
    selectTarget(near[(i + 1) % near.length].id);
  };

  // ---------- Andar (joystick e WASD) ----------
  // O servidor anda por destino: o joystick manda um ponto à frente a cada 150 ms.
  let stick: { x: number; y: number } | null = null;
  let stickWasOn = false;
  const stepStick = () => {
    const p = entities.localPosition();
    if (!p || entities.localDead) return;
    if (!stick) {
      if (stickWasOn) {
        stickWasOn = false;
        const dest = entities.predictLocalMove(p.x, p.z);
        if (dest) conn.moveTo(dest.x, dest.z);
      }
      return;
    }
    stickWasOn = true;
    const a = world.camera.alpha;
    const fx_ = -Math.cos(a);
    const fz = -Math.sin(a);
    // Frente = para onde a câmera olha; direita = perpendicular.
    let vx = fx_ * -stick.y + fz * stick.x;
    let vz = fz * -stick.y - fx_ * stick.x;
    const len = Math.hypot(vx, vz) || 1;
    vx /= len;
    vz /= len;
    const dest = entities.predictLocalMove(p.x + vx * 3, p.z + vz * 3);
    if (dest) {
      hud.setQueued(0);
      pendingShop = 0;
      conn.moveTo(dest.x, dest.z);
    }
  };
  const stickTimer = window.setInterval(stepStick, 150);
  const keysHeld = new Set<string>();
  const keyStick = () => {
    const x = (keysHeld.has("KeyD") ? 1 : 0) - (keysHeld.has("KeyA") ? 1 : 0);
    const y = (keysHeld.has("KeyS") ? 1 : 0) - (keysHeld.has("KeyW") ? 1 : 0);
    stick = x || y ? { x, y } : null;
    stepStick();
  };

  // ---------- Mapa e regiões ----------
  const { regions, bosses } = buildRegions(welcome.zones, welcome.safeRadius);
  const mapSource = {
    world: worldData,
    mapCanvas: renderMapCanvas(worldData),
    forEachMarker: entities.forEachMarker.bind(entities),
    // A câmera fica em target + (cos α, sin α)·r e olha para o alvo: direção = -(cos α, sin α).
    cameraHeading: () => Math.atan2(-Math.cos(world.camera.alpha), -Math.sin(world.camera.alpha)),
  };

  /** Toma uma poção de vida ou mana (primeiro as da barra, depois qualquer uma da bolsa). */
  const drink = (kind: "hp" | "mp"): "ok" | "cooldown" | "none" => {
    const ids = hud
      .getHotbar()
      .filter((e): e is string => !!e && e.startsWith("item:"))
      .map((e) => e.slice(5));
    const all = [...ids, ...store.bag.filter((s) => s).map((s) => s!.info.id)];
    const id = all.find((i) => {
      const info = store.get(i);
      return !!info && store.countOf(i) > 0 && (kind === "hp" ? info.healHp > 0 : info.healMp > 0);
    });
    return id ? useItemById(id) : "none";
  };

  // ---------- Auto-caça ----------
  const auto = new AutoHunt({
    dead: () => entities.localDead,
    stats: () => lastStats,
    nearestMonster: () => entities.nearestMonster(),
    targetId: () => targetId,
    isEnemy: (id) => entities.isEnemy(id),
    distanceTo: (id) => entities.distanceTo(id),
    attack: attackTarget,
    hotbarSkills: () =>
      hud
        .getHotbar()
        .filter((e): e is string => !!e && e.startsWith("skill:"))
        .map((e) => learnedActives().find((s) => s.id === Number(e.slice(6))))
        .filter((s): s is SkillInfo => !!s),
    skillReady: (s) => hud.isReady(s.id) && hud.hasMana(s),
    useSkill,
    drink,
    lootNearby: (radius) => {
      const p = entities.localPosition();
      const near = p ? loot.nearest(p) : null;
      if (!near || near.dist > radius) return false;
      const g = loot.info(near.id);
      if (g && g.ownerId && g.ownerId !== welcome.yourId && g.reservedMs > 0) return false;
      conn.pickUp(near.id);
      return true;
    },
    goToTown: () => {
      const dest = entities.predictLocalMove(0, 0);
      if (dest) conn.moveTo(dest.x, dest.z);
    },
    onStop: (reason) => {
      hud.setAuto(false, false);
      chat.system(reason);
      toast(reason, "warn");
    },
  });

  // ---------- HUD ----------
  const hud = new Hud(learnedActives(), store, {
    onSkill: useSkill,
    onItem: (id) => void useItemById(id),
    onRespawn: () => conn.respawn(),
    onHotbarChange: (entries) => conn.setHotbar(entries.map((e) => e ?? "")),
    onAuto: (on) => (on ? auto.start() : auto.stop()),
    openWindow: (id: WindowId) => wm.toggle(id),
    onAttack: () => {
      if (entities.isEnemy(targetId)) return attackTarget(targetId);
      if (entities.isPlayer(targetId) && entities.canAttack(targetId, forcedId === targetId)) return attackPlayer(targetId, forcedId === targetId);
      const n = entities.nearestMonster();
      if (n && n.dist < 30) attackTarget(n.id);
      else toast("Nenhum monstro por perto.", "warn");
    },
    onTargetAction: (action) => {
      const info = entities.info(targetId);
      if (!info || !entities.isPlayer(targetId)) return;
      if (action === "attack") attackPlayer(targetId, true);
      else if (action === "whisper") chat.whisperTo(info.name);
      else if (action === "party") inviteParty(info.name);
      else if (action === "clan") inviteClan(info.name);
    },
    onPartyMember: (id) => selectTarget(id),
    onCycleTarget: cycleTarget,
    onStick: (v) => {
      stick = v;
      if (v) stepStick();
    },
    me,
    targetDistance: () => (isAttackable(targetId) ? entities.distanceTo(targetId) : null),
    map: mapSource,
    regions,
    stats: () => ({ fps: world.engine.getFps(), ping: conn.ping }),
    clock: () => clock.phase(),
  });
  hud.setIdentity(character.name, classInfo(character.class).label[0], emblem);
  hud.setHotbar(welcome.hotbar?.slots ?? []);
  migrateLocalHotbar();
  /** Pontos livres viram uma bolinha nos botões de Personagem e Skills. */
  const refreshBadges = () =>
    hud.setBadges({ character: progress?.freeAttrPoints ?? 0, skills: progress?.freeSkillPoints ?? 0 });
  refreshBadges();

  const chat = new Chat(hud.chatRoot, character.name, (text, to, channel) => conn.chat(text, to, channel));
  chat.onPartyInvite = inviteParty;
  const invites = new InvitePrompt(hud.element, (id, accept) => conn.inviteReply(id, accept));
  /** Atualiza tudo que depende do grupo (nomes azuis, painel, janela, quadro do alvo). */
  const applyParty = () => {
    entities.setPartyIds(party.members.map((m) => m.id).filter((id) => id !== welcome.yourId));
    hud.setParty(party, welcome.yourId, (id) => (entities.distanceTo(id) ?? Infinity) <= PARTY_NEAR);
    for (const fn of partyListeners) fn();
    if (targetId) refreshTarget();
  };
  chat.system(`Bem-vindo a Erevia, ${character.name}! Clique num monstro para atacar; as teclas 1–0 usam a barra de atalhos.`);

  const toHotbar = (entry: string, index?: number) => {
    const bar = hud.getHotbar();
    const i = index ?? (bar.includes(entry) ? bar.indexOf(entry) : bar.findIndex((e) => !e));
    if (i < 0) {
      toast("Barra cheia: arraste para um atalho.", "warn");
      return;
    }
    hud.assign(i, entry);
    toast(`Na tecla ${i === 9 ? 0 : i + 1}.`);
  };

  // ---------- Loja ----------
  const SHOP_OPEN_DIST = 4.5;
  const SHOP_KEEP_DIST = 7;
  let pendingShop = 0;
  const shop = new ShopWindow(
    store,
    itemTip,
    {
      buy: (npc, id, n) => conn.buy(npc, id, n),
      sell: (npc, slot, n) => conn.sell(npc, slot, n),
      me,
      // Loja e inventário lado a lado, centralizados (se couberem; senão o inventário por cima).
      onOpen: () => {
        if (isCompact()) return void wm.show("inventory");
        const shopW = shop.root.offsetWidth || 360;
        const invW = wm.widthOf("inventory");
        const left = Math.max(12, (innerWidth - shopW - 12 - invW) / 2);
        shop.root.style.left = `${left}px`;
        void wm.show("inventory", { left: left + shopW + 12, top: shop.root.offsetTop });
      },
    },
    hud.element,
  );
  const openShop = (id: number) => {
    const info = entities.info(id);
    const d = entities.distanceTo(id);
    const root = entities.rootOf(id);
    if (!info || d === null || !root) return;
    if (d <= SHOP_OPEN_DIST) {
      pendingShop = 0;
      shop.open(id, `${info.title} ${info.name}`, npcStock.get(id) ?? []);
      return;
    }
    // Para a ~2 m do NPC, do lado de quem chega.
    const mePos = entities.localPosition();
    if (!mePos) return;
    const dir = mePos.subtract(root.position);
    dir.y = 0;
    dir.normalize().scaleInPlace(2);
    const dest = entities.predictLocalMove(root.position.x + dir.x, root.position.z + dir.z);
    if (dest) conn.moveTo(dest.x, dest.z);
    pendingShop = id;
  };

  // ---------- Janelas ----------
  const keys = () => settings.get().keys;
  const defs = new Map<string, WindowDef>();
  const reg = (d: WindowDef) => defs.set(d.id, d);
  reg({
    id: "inventory",
    title: "Inventário",
    hotkey: keys().inventory,
    width: 880,
    height: 640,
    build: () =>
      buildInventory({
        store,
        me,
        stats: () => lastStats,
        onStats,
        move: (fp, fs, tp, ts) => conn.moveItem(fp, fs, tp, ts),
        use: (place, slot) => conn.useItem(place, slot),
        destroy: (slot, count) => conn.destroyItem(slot, count),
        toHotbar: (id, i) => toHotbar(`item:${id}`, i),
        sell: (slot) => {
          const s = store.bag[slot];
          if (!shop.isOpen || !s || s.info.sellPrice <= 0) return false;
          conn.sell(shop.npcId, slot, s.count);
          return true;
        },
        shopOpen: () => shop.isOpen,
      }),
    onClose: () => shop.isOpen && shop.close(),
  });
  reg({
    id: "character",
    title: "Personagem",
    hotkey: keys().character,
    width: 900,
    height: 600,
    build: () =>
      buildCharacter({
        store,
        me,
        stats: () => lastStats,
        progress: () => progress,
        onStats,
        onProgress,
        allocate: (add) => conn.allocateAttrs(add),
        pvp: () => ({ karma: pvp.karma, pks: pvp.pkCount, wins: pvp.pvpKills, flag: flagLeft() }),
        onPvp,
      }),
  });
  reg({
    id: "skills",
    title: "Skills",
    hotkey: keys().skills,
    width: 1000,
    height: 700,
    build: () =>
      buildSkills({
        skills: welcome.skills,
        branches: welcome.branches,
        level: () => me().myLevel,
        progress: () => progress,
        onProgress,
        learn: (ranks) => conn.learnSkills(ranks),
        toHotbar: (id, i) => toHotbar(`skill:${id}`, i),
      }),
  });
  reg({
    id: "party",
    title: "Grupo",
    hotkey: keys().party,
    width: 460,
    build: () =>
      buildParty({
        localId: welcome.yourId,
        state: () => party,
        onChange: onParty,
        invite: inviteParty,
        kick: (id) => conn.partyCmd(PartyCmdKind.PARTY_CMD_KICK, "", id),
        promote: (id) => conn.partyCmd(PartyCmdKind.PARTY_CMD_LEADER, "", id),
        leave: () => conn.partyCmd(PartyCmdKind.PARTY_CMD_LEAVE),
        select: (id) => selectTarget(id),
      }),
  });
  reg({
    id: "map",
    title: "Mapa do mundo",
    hotkey: keys().map,
    width: 1080,
    height: 740,
    build: () =>
      buildWorldMap({
        map: mapSource,
        regions,
        bosses,
        nameOf: (id) => entities.info(id)?.name ?? "",
        moveTo: (x, z) => {
          hud.setQueued(0);
          pendingShop = 0;
          const dest = entities.predictLocalMove(x, z);
          if (dest) conn.moveTo(dest.x, dest.z);
        },
      }),
  });
  reg({
    id: "settings",
    title: "Configurações",
    hotkey: keys().settings,
    width: 820,
    height: 640,
    build: (close) =>
      buildSettings({
        close,
        exit: () => location.reload(),
        logout: async () => {
          await api.logout().catch(() => undefined);
          location.reload();
        },
        info: () => ({ account: "", character: character.name, level: me().myLevel, ping: conn.ping, online: entities.count }),
      }),
  });
  const wm = new WindowManager(defs);
  wm.onChange = () => {
    for (const id of defs.keys()) hud.setWindowOpen(id, wm.isOpen(id));
  };

  // ---------- Teclado ----------
  const onKey = (ev: KeyboardEvent) => {
    const typing = (ev.target as HTMLElement | null)?.closest?.("input, textarea, select");
    if (typing || document.querySelector(".rebinding")) return;
    if (["KeyW", "KeyA", "KeyS", "KeyD"].includes(ev.code)) {
      if (!keysHeld.has(ev.code)) {
        keysHeld.add(ev.code);
        keyStick();
      }
      return;
    }
    const k = keys();
    if (ev.code === k.settings) {
      ev.preventDefault();
      if (invites.open) invites.answer(false);
      else if (shop.isOpen) shop.close();
      else if (wm.closeTop()) return;
      else if (targetId) selectTarget(0);
      else wm.toggle("settings");
      return;
    }
    const slot = HOTBAR_KEYS.indexOf(ev.code);
    if (slot >= 0) {
      ev.preventDefault();
      hud.useSlot(slot);
      return;
    }
    const actions: Record<string, () => void> = {
      [k.inventory]: () => wm.toggle("inventory"),
      [k.skills]: () => wm.toggle("skills"),
      [k.party]: () => wm.toggle("party"),
      [k.character]: () => wm.toggle("character"),
      [k.map]: () => wm.toggle("map"),
      [k.auto]: () => hud.setAuto(!hud.autoOn, true),
      [k.target]: cycleTarget,
      [k.potion]: () => {
        if (drink("hp") === "none") toast("Sem poção de vida.", "warn");
      },
    };
    const fn = actions[ev.code];
    if (fn) {
      ev.preventDefault();
      fn();
    }
  };
  const onKeyUp = (ev: KeyboardEvent) => {
    if (keysHeld.delete(ev.code)) keyStick();
  };
  window.addEventListener("keydown", onKey);
  window.addEventListener("keyup", onKeyUp);

  setupInput(world.scene, world.ground, {
    onGround: (x, z) => {
      hud.setQueued(0); // andar cancela a skill na fila
      pendingShop = 0;
      const dest = entities.predictLocalMove(x, z);
      if (dest) conn.moveTo(dest.x, dest.z);
      return dest;
    },
    onEntity: (id, force) => {
      if (id === welcome.yourId) return;
      selectTarget(id);
      if (entities.isNpc(id)) {
        if (!entities.localDead) openShop(id);
        return;
      }
      pendingShop = 0;
      if (entities.isEnemy(id) && !entities.localDead) conn.attack(id);
      // Jogador: clique simples só seleciona (a não ser que ele já seja hostil); Ctrl ataca.
      else if (entities.isPlayer(id)) attackPlayer(id, force);
    },
    onLoot: (id) => {
      if (entities.localDead) return;
      hud.setQueued(0);
      pendingShop = 0;
      conn.pickUp(id);
    },
    isEnemy: (id, force) => entities.isEnemy(id) || entities.canAttack(id, force || forcedId === id),
  });
  const stopTouchCamera = setupTouchCamera(canvas, world.camera);

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
    loot.update(pos);
    if (pendingShop) {
      const d = entities.distanceTo(pendingShop);
      if (d !== null && d <= SHOP_OPEN_DIST) openShop(pendingShop);
    }
    if (shop.isOpen) {
      const d = entities.distanceTo(shop.npcId);
      if (d === null || d > SHOP_KEEP_DIST) shop.close();
    }
  });

  // Efeitos (buffs) e status da auto-caça
  const effectsTimer = window.setInterval(() => {
    hud.setEffects(entities.effectsOf(welcome.yourId), targetId ? entities.effectsOf(targetId) : []);
    if (auto.on) hud.setAutoStatus(auto.status());
    hud.setAttacking(isAttackable(targetId));
  }, 100);
  setupPerfPanel(world.scene, world.gui, $("hud"));

  canvas.focus();
  if (settings.get().touchMode !== "off" && matchMedia("(pointer: coarse)").matches) {
    toast("Joystick à esquerda · toque num monstro para atacar · AUTO caça sozinho");
  }

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
      /** Fixa a hora do dia (0–1) ou volta ao relógio do servidor (null). */
      setTime: (p: number | null) => clock.setFixed(p),
      phase: () => clock.phase(),
      fx,
      store,
      loot,
      shop,
      hud,
      wm,
      auto,
      stats: () => lastStats,
      pickUp: (id: number) => conn.pickUp(id),
      nearestLoot: () => {
        const p = entities.localPosition();
        return p ? loot.nearest(p) : null;
      },
      openShop: (id: number) => openShop(id),
      npcs: () => welcome.npcs.map((n) => n.id),
      useItem: (place: number, slot: number) => conn.useItem(place, slot),
      buy: (npc: number, id: string, n: number) => conn.buy(npc, id, n),
      moveTo: (x: number, z: number) => {
        entities.predictLocalMove(x, z);
        conn.moveTo(x, z);
      },
      attack: (id: number) => attackTarget(id),
      attackPlayer: (id: number, force: boolean) => attackPlayer(id, force),
      select: (id: number) => selectTarget(id),
      info: (id: number) => entities.info(id),
      pvp: () => ({ ...pvp, flag: flagLeft() }),
      party: () => party,
      partyInvite: (name: string) => inviteParty(name),
      answerInvite: (accept: boolean) => invites.answer(accept),
    };
  }

  let lastXp: bigint | null = null;
  return {
    dispose() {
      auto.stop();
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("keyup", onKeyUp);
      window.clearInterval(effectsTimer);
      window.clearInterval(stickTimer);
      stopTouchCamera();
      wm.closeAll();
      wm.layer.remove();
      hud.dispose();
      invites.dispose();
      shop.dispose();
      itemTip.root.remove();
    },
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
          for (const ev of snap.combat) {
            entities.onCombat(ev);
            // Um jogador atacou você: a auto-caça para (nunca revida sozinha).
            if (ev.targetId === welcome.yourId && ev.sourceId !== welcome.yourId && entities.isPlayer(ev.sourceId)) auto.onPlayerAttack();
          }
          for (const ev of snap.casts) entities.onSkillCast(ev);
          for (const ev of snap.effects) entities.onEffect(ev);
          for (const ev of snap.effectEnds) entities.onEffectEnd(ev);
          for (const id of snap.deaths) {
            entities.onDeath(id);
            auto.onDeath(id);
          }
          for (const r of snap.revives) entities.onRevive(r);
          for (const l of snap.levelUps) {
            entities.onLevelUp(l);
            if (l.id === welcome.yourId) hud.levelBanner(l.level);
          }
          if (targetId) refreshTarget();
          break;
        }
        case "progress":
          progress = msg.msg.value;
          hud.setSkills(learnedActives());
          refreshBadges();
          for (const fn of progressListeners) fn();
          break;
        case "hotbar":
          hud.setHotbar(msg.msg.value.slots);
          break;
        case "selfStats": {
          const s = msg.msg.value;
          const castChanged = s.castSpeed !== lastStats?.castSpeed;
          // XP ganha vira um aviso no canto (só subidas; morrer perde um pouco).
          if (lastXp !== null && s.xp > lastXp && s.level === lastStats?.level) hud.lootLine(`+${(s.xp - lastXp).toLocaleString("pt-BR")} XP`, "xp");
          lastXp = s.xp;
          lastStats = s;
          hud.setStats(s);
          if (castChanged) hud.setSkills(learnedActives());
          for (const fn of statsListeners) fn();
          break;
        }
        case "inventory":
          store.setInventory(msg.msg.value);
          break;
        case "groundAdded":
          loot.add(msg.msg.value.items);
          break;
        case "groundRemoved": {
          const { ids, pickedBy } = msg.msg.value;
          if (pickedBy === welcome.yourId) {
            for (const id of ids) {
              const at = loot.position(id);
              const g = loot.info(id);
              const info = g && store.get(g.itemId);
              const head = entities.headOf(welcome.yourId);
              if (!at || !info) continue;
              const color = Color3.FromHexString(info.kind === ItemKind.GOLD ? "#f3d488" : (GRADE_COLOR[info.grade] ?? "#e8e1d4"));
              fx.pickup(at, color);
              if (head) fx.damageNumber(head, `+${itemLabel(info, g.count)}`, color.toHexString(), false);
              hud.lootLine(itemLabel(info, g.count));
            }
          }
          loot.remove(ids);
          break;
        }
        case "itemCooldown": {
          const c = msg.msg.value;
          hud.itemCooldown(c.group, c.readyInMs, c.totalMs);
          // Recarga cheia = acabou de beber: efeito no personagem.
          const root = entities.rootOf(welcome.yourId);
          if (root && c.readyInMs >= c.totalMs - 250) fx.potion(root, c.group === 2 ? new Color3(0.35, 0.6, 1) : new Color3(1, 0.35, 0.3));
          break;
        }
        case "appearance":
          entities.setAppearance(msg.msg.value);
          break;
        case "party":
          party = msg.msg.value;
          applyParty();
          break;
        case "invite":
          invites.push(msg.msg.value);
          break;
        case "playerStatus":
          entities.setStatus(msg.msg.value);
          if (msg.msg.value.id === targetId || msg.msg.value.id === welcome.yourId) refreshTarget();
          break;
        case "selfPvp": {
          const was = pvp;
          pvp = msg.msg.value;
          pvpAt = performance.now();
          if (pvp.karma > 0 && was.karma === 0) toast("Você tem karma: seu nome está vermelho.", "bad");
          for (const fn of pvpListeners) fn();
          break;
        }
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
