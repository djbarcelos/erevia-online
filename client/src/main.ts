import { api, ApiError, type CharacterInfo } from "./api";
import { Color3, Vector3 } from "@babylonjs/core";
import { create } from "@bufbuild/protobuf";
import {
  EntityKind,
  ItemKind,
  ChannelKind,
  ClanCmdKind,
  NpcRole,
  TradeCmdKind,
  ClanRank,
  ClanStateSchema,
  PartyCmdKind,
  PartyStateSchema,
  SelfPvpSchema,
  SkillTarget,
  type ClanState,
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
import { clearResume, readResume, reconnect, type Resume } from "./net/resume";
import { Chat } from "./ui/chat";
import { showCharacters, showLogin, showMessage, showScreen } from "./ui/screens";
import { EntityManager } from "./world/entities";
import { Hud, HOTBAR_KEYS, HOTBAR_SIZE, type TargetExtras, type WindowId } from "./ui/hud";
import { setupPerfPanel } from "./ui/perf";
import { runBench, wantBench } from "./ui/bench";
import { GRADE_COLOR, itemLabel, ItemStore, ItemTip, PLACE_BAG, setEnchantRules } from "./ui/items";
import { ShopWindow } from "./ui/shop";
import { TeleportWindow } from "./ui/teleport";
import { BarberWindow } from "./ui/barber";
import { toBodyLook } from "./world/body-look";
import { PLACE_STORAGE, StorageWindow } from "./ui/storage";
import { TradeWindow } from "./ui/trade";
import { applyEnvClasses, isCompact, settings, toast, WindowManager, type WindowDef } from "./ui/kit";
import { DEFAULT_LOOK } from "./ui/looks";
import { buildRegions } from "./ui/regions";
import { buildInventory } from "./ui/windows/inventory";
import { buildWorldMap } from "./ui/windows/worldmap";
import { buildSettings } from "./ui/windows/settings";
import { buildCharacter } from "./ui/windows/character";
import { buildSkills } from "./ui/windows/skills";
import { buildParty } from "./ui/windows/party";
import { buildClan } from "./ui/windows/clan";
import { InvitePrompt } from "./ui/invite";
import { RaidUi } from "./ui/raid";
import { RaidDirector } from "./world/raid";
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
import { Collision, loadWorldData, worldBounds } from "./world/world-data";

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
    // A conexão caiu e a página recarregou para voltar: entra direto no mesmo personagem.
    const resume = readResume();
    if (resume) {
      const char = (await api.characters()).characters.find((c) => c.id === resume.id);
      if (char) return play(char, resume);
      clearResume();
    }
    await openCharacters();
  } catch (err) {
    clearResume();
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

/** Entra no mundo com o personagem escolhido (resume: voltando de uma queda de conexão). */
function play(character: CharacterInfo, resume?: Resume): void {
  showScreen("screen-loading", resume ? "Reconectando…" : undefined);
  const conn = new Connection(Connection.defaultUrl(), character.id);
  let started = false;
  let kickedReason = "";
  let game: Game | undefined;
  // Mensagens que chegam enquanto a cena 3D ainda está sendo criada.
  const backlog: ServerMessage[] = [];

  conn.onMessage = (msg) => {
    if (msg.msg.case === "welcome") {
      started = true;
      clearResume();
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
    // Caiu sem expulsão (estava jogando, ou é uma tentativa de voltar que ainda não entrou): reconecta sozinho.
    if (!kickedReason && (started || resume)) {
      const r: Resume = !started && resume ? resume : { id: character.id, since: Date.now(), reloads: 0 };
      showScreen("screen-loading", "Conexão perdida. Reconectando…");
      void reconnect(r, () =>
        showMessage("Desconectado", "Não foi possível reconectar. Confira a sua internet.", "Tentar de novo", () => location.reload()),
      );
      return;
    }
    clearResume();
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
  const entities = new EntityManager(world.scene, world.gui, welcome.yourId, worldBounds(worldData), fx, world.shadows, collision);
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
  const me = () => ({
    myClass: character.class,
    myLevel: lastStats?.level ?? character.level ?? 1,
    name: character.name,
    emblem,
    body: entities.lookOf(welcome.yourId) ?? toBodyLook(character.appearance),
  });

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

  // ---------- Clã ----------
  let clan: ClanState = welcome.clan ?? create(ClanStateSchema, {});
  const clanListeners = new Set<() => void>();
  const onClan = (fn: () => void) => {
    clanListeners.add(fn);
    return () => clanListeners.delete(fn);
  };
  /** Convidar ao clã: você é líder/oficial e ele não tem clã. */
  const canInviteClan = (id: number) =>
    !!clan.id && clan.myRank >= ClanRank.OFFICER && entities.isPlayer(id) && !entities.info(id)?.clanName && clan.members.length < clan.maxMembers;
  const inviteClan = (name: string) => conn.clanCmd(ClanCmdKind.CLAN_CMD_INVITE, { name });

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
  setEnchantRules(welcome.enchant);
  store.setInventory(welcome.inventory);
  const itemTip = new ItemTip(document.body);
  const npcStock = new Map(welcome.npcs.map((n) => [n.id, n.stock]));
  const npcRecipes = new Map(welcome.npcs.map((n) => [n.id, n.recipes]));
  const npcInfo = new Map(welcome.npcs.map((n) => [n.id, n]));
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
        // Quem decide onde parar é o servidor: qualquer ponto que o cliente mande já
        // ficou para trás (latência), e o personagem voltaria virado ao contrário.
        entities.stopLocal();
        conn.stop();
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
    distanceTo: (id) => entities.reachDistanceTo(id),
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
      // Pergaminho de Retorno, se tiver (e não estiver em recarga); senão, a pé.
      if (pvp.karma === 0 && store.countOf("scroll_return") > 0 && useItemById("scroll_return") === "ok") return;
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
      else if (action === "trade") conn.tradeCmd(TradeCmdKind.TRADE_CMD_REQUEST, { targetId });
    },
    onPartyMember: (id) => selectTarget(id),
    onCancelChannel: () => conn.cancelChannel(),
    onCycleTarget: cycleTarget,
    onStick: (v) => {
      stick = v;
      if (v) stepStick();
    },
    me,
    targetDistance: () => (isAttackable(targetId) ? entities.reachDistanceTo(targetId) : null),
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
  // Raid boss: barra no topo, anúncios, avisos no chão e o estado dos chefes no mapa.
  const serverOffset = Number(welcome.serverTimeMs) - Date.now();
  const serverNow = () => Date.now() + serverOffset;
  const raidUi = new RaidUi(serverNow);
  hud.mount(raidUi.bar, raidUi.banner);
  const raid = new RaidDirector(world.scene, fx, entities, raidUi, welcome.yourId, (text) => chat.announce(text));
  raid.setBosses(welcome.bosses);
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
  const storageWin = new StorageWindow(
    store,
    {
      move: (npc, fp, fs, tp, ts, n) => conn.storageMove(npc, fp, fs, tp, ts, n),
      gold: (npc, amount) => conn.storageGold(npc, amount),
      me,
      onOpen: () => {
        if (isCompact()) return void wm.show("inventory");
        const w = storageWin.root.offsetWidth || 360;
        const invW = wm.widthOf("inventory");
        const left = Math.max(12, (innerWidth - w - 12 - invW) / 2);
        storageWin.root.style.left = `${left}px`;
        void wm.show("inventory", { left: left + w + 12, top: storageWin.root.offsetTop });
      },
    },
    hud.element,
  );
  const tradeWin = new TradeWindow(
    store,
    {
      remove: (slot) => conn.tradeCmd(TradeCmdKind.TRADE_CMD_REMOVE_ITEM, { slot }),
      gold: (amount) => conn.tradeCmd(TradeCmdKind.TRADE_CMD_SET_GOLD, { gold: amount }),
      lock: () => conn.tradeCmd(TradeCmdKind.TRADE_CMD_LOCK),
      confirm: () => conn.tradeCmd(TradeCmdKind.TRADE_CMD_CONFIRM),
      cancel: () => conn.tradeCmd(TradeCmdKind.TRADE_CMD_CANCEL),
      me,
      onOpen: () => {
        if (isCompact()) return void wm.show("inventory");
        const w = tradeWin.root.offsetWidth || 420;
        const invW = wm.widthOf("inventory");
        const left = Math.max(12, (innerWidth - w - 12 - invW) / 2);
        tradeWin.root.style.left = `${left}px`;
        void wm.show("inventory", { left: left + w + 12, top: tradeWin.root.offsetTop });
      },
    },
    hud.element,
  );
  const teleportWin = new TeleportWindow(store, { travel: (npc, dest) => conn.teleport(npc, dest), karma: () => pvp.karma }, hud.element);
  const barberWin = new BarberWindow(
    store,
    {
      change: (npc, look) => conn.changeLook(npc, look),
      me: () => {
        const m = me();
        const w = store.equip[0]?.info;
        const o = store.equip[1]?.info;
        return { myClass: m.myClass, body: m.body, look: { right: w?.model || null, left: o?.model || w?.offModel || null, emblem: m.emblem } };
      },
    },
    hud.element,
  );
  // Enquanto canaliza, um brilho azul sobe do personagem.
  const channelFx = window.setInterval(() => {
    const root = entities.rootOf(welcome.yourId);
    if (hud.channeling && root) fx.castGlow(root.position.add(new Vector3(0, 1, 0)), new Color3(0.45, 0.75, 1));
  }, 450);
  const SHOP_OPEN_DIST = 4.5;
  const SHOP_KEEP_DIST = 7;
  let pendingShop = 0;
  const shop = new ShopWindow(
    store,
    itemTip,
    {
      buy: (npc, id, n) => conn.buy(npc, id, n),
      sell: (npc, slot, n) => conn.sell(npc, slot, n),
      craft: (npc, recipe) => conn.craft(npc, recipe),
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
      const npc = npcInfo.get(id);
      shop.close();
      teleportWin.close();
      storageWin.close();
      barberWin.close();
      if (npc?.role === NpcRole.BARBER) void barberWin.open(id, `${info.title} · ${info.name}`);
      else if (npc?.role === NpcRole.TELEPORT) teleportWin.open(id, `${info.title} · ${info.name}`, npc.dests);
      else if (npc?.role === NpcRole.STORAGE) {
        storageWin.open(id, `${info.title} · ${info.name}`);
        conn.storageGold(id, 0); // pede o conteúdo
      } else shop.open(id, `${info.title} ${info.name}`, npcStock.get(id) ?? [], npcRecipes.get(id) ?? []);
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
        enchant: (scroll, place, slot) => conn.enchant(scroll, place, slot),
        storageOpen: () => storageWin.isOpen,
        tradeOpen: () => tradeWin.isOpen,
        offer: (bagSlot) => conn.tradeCmd(TradeCmdKind.TRADE_CMD_ADD_ITEM, { slot: bagSlot }),
        deposit: (bagSlot, toSlot) => conn.storageMove(storageWin.npcId, PLACE_BAG, bagSlot, PLACE_STORAGE, toSlot, 0),
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
    id: "clan",
    title: "Clã",
    hotkey: keys().clan,
    width: 560,
    build: () =>
      buildClan({
        localId: welcome.yourId,
        state: () => clan,
        onChange: onClan,
        gold: () => store.gold,
        level: () => me().myLevel,
        create: (name) => conn.clanCmd(ClanCmdKind.CLAN_CMD_CREATE, { name }),
        invite: inviteClan,
        kick: (id) => conn.clanCmd(ClanCmdKind.CLAN_CMD_KICK, { targetId: id }),
        setRank: (id, rank) => conn.clanCmd(ClanCmdKind.CLAN_CMD_RANK, { targetId: id, rank }),
        promoteLeader: (id) => conn.clanCmd(ClanCmdKind.CLAN_CMD_LEADER, { targetId: id }),
        notice: (text) => conn.clanCmd(ClanCmdKind.CLAN_CMD_NOTICE, { text }),
        leave: () => conn.clanCmd(ClanCmdKind.CLAN_CMD_LEAVE),
        disband: () => conn.clanCmd(ClanCmdKind.CLAN_CMD_DISBAND),
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
        stones: welcome.npcs
          .filter((n) => n.role === NpcRole.TELEPORT)
          .map((n) => ({ id: n.id, name: entities.info(n.id)?.name ?? "", x: entities.rootOf(n.id)?.position.x ?? 0, z: entities.rootOf(n.id)?.position.z ?? 0 })),
        nameOf: (id) => entities.info(id)?.name ?? "",
        bossStatus: (b) => {
          const st = raid.statusAt(b.x, b.z);
          return st && { alive: st.alive, windowStartMs: Number(st.windowStartMs), windowEndMs: Number(st.windowEndMs) };
        },
        serverNow,
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
      else if (hud.channeling) conn.cancelChannel();
      else if (teleportWin.isOpen) teleportWin.close();
      else if (storageWin.isOpen) storageWin.close();
      else if (tradeWin.isOpen) conn.tradeCmd(TradeCmdKind.TRADE_CMD_CANCEL);
      else if (barberWin.isOpen) barberWin.close();
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
      [k.clan]: () => wm.toggle("clan"),
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

  // A câmera segue o jogador local (ou o ponto da medição de FPS, ?bench=1).
  let benchFocus: Vector3 | null = null;
  world.scene.onBeforeRenderObservable.add(() => {
    const dt = Math.min(world.engine.getDeltaTime() / 1000, 0.1);
    entities.setLatency(conn.ping, welcome.tickRate);
    entities.update(dt);
    raid.update();
    const pos = entities.localPosition();
    const focus = benchFocus ?? pos;
    if (focus) {
      world.camera.target.x = focus.x;
      world.camera.target.y = 1.4;
      world.camera.target.z = focus.z;
      world.followShadows(focus.x, focus.z);
    }
    dayNight.update(clock.phase(), focus);
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
    if (teleportWin.isOpen) {
      const d = entities.distanceTo(teleportWin.npcId);
      if (d === null || d > SHOP_KEEP_DIST) teleportWin.close();
    }
    if (storageWin.isOpen) {
      const d = entities.distanceTo(storageWin.npcId);
      if (d === null || d > SHOP_KEEP_DIST) storageWin.close();
    }
    if (barberWin.isOpen) {
      const d = entities.distanceTo(barberWin.npcId);
      if (d === null || d > SHOP_KEEP_DIST) barberWin.close();
    }
  });

  // Efeitos (buffs) e status da auto-caça
  const effectsTimer = window.setInterval(() => {
    hud.setEffects(entities.effectsOf(welcome.yourId), targetId ? entities.effectsOf(targetId) : []);
    if (auto.on) hud.setAutoStatus(auto.status());
    hud.setAttacking(isAttackable(targetId));
  }, 100);
  setupPerfPanel(world.scene, world.gui, $("hud"));
  if (wantBench()) {
    // Espera a cena assentar (modelos, sombras) antes de medir.
    window.setTimeout(() => {
      void runBench({
        engine: world.engine,
        scene: world.scene,
        camera: world.camera,
        regions,
        bounds: worldBounds(worldData),
        setTime: (p) => clock.setFixed(p),
        setFocus: (p) => (benchFocus = p ? new Vector3(p.x, 0, p.z) : null),
        forEachMarker: entities.forEachMarker.bind(entities),
        hud: $("hud"),
      });
    }, 3000);
  }

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
      useItemId: (id: string) => useItemById(id),
      stones: () => welcome.npcs.filter((n) => n.role === NpcRole.TELEPORT).map((n) => ({ id: n.id, name: entities.info(n.id)?.name })),
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
      clan: () => clan,
      raid: () => raid.all(),
      raidCmd: (args: string) => conn.chat(`/raid ${args}`, ""),
      clanCmd: (kind: number, opts: { name?: string; targetId?: number; text?: string }) => conn.clanCmd(kind, opts),
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
      teleportWin.dispose();
      storageWin.dispose();
      tradeWin.dispose();
      barberWin.dispose();
      window.clearInterval(channelFx);
      raid.dispose();
      raidUi.dispose();
      itemTip.root.remove();
    },
    apply(msg: ServerMessage) {
      switch (msg.msg.case) {
        case "spawn": {
          const e = msg.msg.value.entity;
          if (e) {
            entities.add(e, true);
            raid.onSpawn(e.id, e.model, e.x, e.z);
          }
          break;
        }
        case "despawn":
          if (msg.msg.value.id === targetId) selectTarget(0);
          entities.remove(msg.msg.value.id);
          break;
        case "snapshot": {
          const snap = msg.msg.value;
          for (const s of snap.states) entities.applyState(s);
          // Ordem de encenação: primeiro os golpes (cada um sabe quando encosta), depois a
          // vida, os efeitos e as mortes, que esperam o golpe chegar no alvo.
          for (const ev of snap.casts) entities.onSkillCast(ev);
          for (const ev of snap.combat) {
            entities.onCombat(ev);
            // Um jogador atacou você: a auto-caça para (nunca revida sozinha).
            if (ev.targetId === welcome.yourId && ev.sourceId !== welcome.yourId && entities.isPlayer(ev.sourceId)) auto.onPlayerAttack();
          }
          for (const u of snap.hp) entities.applyHp(u);
          for (const ev of snap.effects) entities.onEffect(ev);
          for (const ev of snap.effectEnds) entities.onEffectEnd(ev);
          for (const id of snap.deaths) {
            entities.onDeath(id);
            auto.onDeath(id);
          }
          for (const r of snap.revives) entities.onRevive(r);
          for (const t of snap.telegraphs) raid.onTelegraph(t);
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
        case "groundAdded": {
          // O saque cai junto com o corpo (a morte espera o golpe encostar).
          const items = msg.msg.value.items;
          entities.afterDeaths(() => loot.add(items));
          break;
        }
        case "groundRemoved": {
          // Na mesma fila do saque: nunca sai antes de entrar.
          const { ids, pickedBy } = msg.msg.value;
          entities.afterDeaths(() => {
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
          });
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
        case "storage":
          storageWin.set(msg.msg.value);
          break;
        case "trade":
          tradeWin.set(msg.msg.value);
          break;
        case "questLog": {
          const log = msg.msg.value;
          hud.setQuests(log);
          if (log.completed) toast(`Missão concluída: ${log.completed}`);
          break;
        }
        case "enchantResult": {
          const r = msg.msg.value;
          const root = entities.rootOf(welcome.yourId);
          const at = root?.position.add(new Vector3(0, 1.2, 0));
          if (r.ok) {
            toast(`Sucesso! +${r.level} ${r.itemName}`);
            if (at) fx.pickup(at, new Color3(1, 0.8, 0.3));
          } else {
            toast(r.broke ? `O encantamento falhou: ${r.itemName} quebrou.` : `Falhou: ${r.itemName} voltou a +0.`, "bad");
            if (at) fx.castGlow(at, new Color3(0.35, 0.3, 0.3));
          }
          break;
        }
        case "channelBar": {
          const b = msg.msg.value;
          hud.setChannel(b.label, b.kind === ChannelKind.NONE ? 0 : b.totalMs);
          break;
        }
        case "clan":
          clan = msg.msg.value;
          for (const fn of clanListeners) fn();
          if (targetId) refreshTarget();
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
        case "announce":
          raid.announce(msg.msg.value);
          break;
        case "bossStatus":
          raid.onStatus(msg.msg.value);
          break;
      }
    },
  };
}
