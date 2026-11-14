import { create, fromBinary, toBinary, type MessageInitShape } from "@bufbuild/protobuf";
import {
  AchievementCmdKind,
  ChatChannel,
  ClanCmdKind,
  ClanRank,
  ClientMessageSchema,
  PartyCmdKind,
  ServerMessageSchema,
  type FriendCmdKind,
  type SpecCmdKind,
  type MasteryCmdKind,
  type MailCmdKind,
  type ServerMessage,
  type TradeCmdKind,
} from "../gen/erevia/v1/game_pb";
import { reportClientError } from "./errors";

const PING_INTERVAL_MS = 2000;

/** Conexão WebSocket binária (Protobuf) com o servidor de jogo. A sessão vai no cookie. */
export class Connection {
  /** Último ping medido (ida e volta), em ms. */
  ping = 0;
  onMessage: (msg: ServerMessage) => void = () => {};
  onClose: (reason: string, code: number) => void = () => {};

  private ws: WebSocket;
  private pingTimer: number | undefined;

  constructor(url: string, characterId: number) {
    this.ws = new WebSocket(url);
    this.ws.binaryType = "arraybuffer";

    this.ws.onopen = () => {
      this.send({ case: "join", value: { characterId } });
      this.pingTimer = window.setInterval(() => this.sendPing(), PING_INTERVAL_MS);
      this.sendPing();
    };

    this.ws.onmessage = (ev: MessageEvent<ArrayBuffer>) => {
      const bytes = new Uint8Array(ev.data);
      let msg: ServerMessage;
      try {
        msg = fromBinary(ServerMessageSchema, bytes);
      } catch (err) {
        // Cliente e servidor fora de sincronia (proto mudou?): relata e descarta a mensagem.
        reportClientError("decode", `mensagem do servidor não decodifica: ${(err as Error)?.message ?? err}`, {
          bytes: bytes.length,
          head: [...bytes.subarray(0, 16)].map((b) => b.toString(16).padStart(2, "0")).join(""),
        }, err);
        return;
      }
      if (msg.msg.case === "pong") {
        this.ping = Math.round(performance.now() - msg.msg.value.clientTime);
        return;
      }
      this.onMessage(msg);
    };

    this.ws.onclose = (ev) => {
      window.clearInterval(this.pingTimer);
      this.onClose(ev.reason || "conexão encerrada", ev.code);
    };
  }

  /** URL padrão: mesmo host da página, rota /ws. */
  static defaultUrl(): string {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    return `${proto}://${location.host}/ws`;
  }

  moveTo(x: number, z: number): void {
    this.send({ case: "moveTo", value: { x, z } });
  }

  /** Para de andar onde estiver no servidor (soltou o WASD/joystick). */
  stop(): void {
    this.send({ case: "moveTo", value: { stop: true } });
  }

  /** channel: GRUPO ou CLÃ (sem canal = geral, ou sussurro se "to" estiver preenchido). */
  chat(text: string, to: string, channel = ChatChannel.UNSPECIFIED): void {
    this.send({ case: "chatSend", value: { text, to, channel } });
  }

  /** Grupo: convidar (pelo nome ou id), sair, expulsar ou passar a liderança. */
  partyCmd(kind: PartyCmdKind, name = "", targetId = 0): void {
    this.send({ case: "partyCmd", value: { kind, name, targetId } });
  }

  /** Clã: criar, convidar, sair, expulsar, cargo, líder, desfazer, aviso. */
  clanCmd(kind: ClanCmdKind, opts: { name?: string; targetId?: number; rank?: ClanRank; text?: string } = {}): void {
    this.send({ case: "clanCmd", value: { kind, name: opts.name ?? "", targetId: opts.targetId ?? 0, rank: opts.rank ?? ClanRank.MEMBER, text: opts.text ?? "" } });
  }

  /** Amigos: adicionar/remover (pelo nome ou pelo id) ou pedir a lista com as posições. */
  friendCmd(kind: FriendCmdKind, opts: { name?: string; targetId?: number } = {}): void {
    this.send({ case: "friendCmd", value: { kind, name: opts.name ?? "", targetId: opts.targetId ?? 0 } });
  }

  /** Correio: enviar, listar a caixa, marcar como lida ou apagar. */
  mailCmd(kind: MailCmdKind, opts: { to?: string; subject?: string; body?: string; mailId?: bigint } = {}): void {
    this.send({ case: "mailCmd", value: { kind, to: opts.to ?? "", subject: opts.subject ?? "", body: opts.body ?? "", mailId: opts.mailId ?? 0n } });
  }

  /** Barbeiro: mudar o visual do corpo (custa moedas). */
  changeLook(npcId: number, look: { skin: number; hair: number; outfit: number; head: string; hideHat: boolean }): void {
    this.send({ case: "changeLook", value: { npcId, look } });
  }

  /** Pedra de Teleporte: viajar para um destino. */
  teleport(npcId: number, destId: number): void {
    this.send({ case: "teleport", value: { npcId, destId } });
  }

  /** Pergaminho de encantar (na bolsa) num item da bolsa (place 0) ou equipado (1). */
  enchant(scrollSlot: number, place: number, slot: number): void {
    this.send({ case: "enchant", value: { scrollSlot, place, slot } });
  }

  /** Armazém: mover entre bolsa (0) e armazém (2); count 0 = a pilha toda. */
  storageMove(npcId: number, fromPlace: number, fromSlot: number, toPlace: number, toSlot: number, count: number): void {
    this.send({ case: "storageMove", value: { npcId, fromPlace, fromSlot, toPlace, toSlot, count } });
  }

  /** Moedas no armazém: > 0 guarda, < 0 retira, 0 só abre. */
  storageGold(npcId: number, amount: number): void {
    this.send({ case: "storageGold", value: { npcId, amount: BigInt(amount) } });
  }

  /** Troca entre jogadores: pedir, oferecer/tirar item da bolsa, moedas, travar, confirmar, cancelar. */
  tradeCmd(kind: TradeCmdKind, opts: { targetId?: number; slot?: number; count?: number; gold?: number } = {}): void {
    this.send({ case: "tradeCmd", value: { kind, targetId: opts.targetId ?? 0, slot: opts.slot ?? 0, count: opts.count ?? 0, gold: BigInt(opts.gold ?? 0) } });
  }

  cancelChannel(): void {
    this.send({ case: "cancelChannel", value: {} });
  }

  inviteReply(inviteId: number, accept: boolean): void {
    this.send({ case: "inviteReply", value: { inviteId, accept } });
  }

  /** force: atacar um jogador que não está em combate nem tem karma (Ctrl + clique). */
  attack(targetId: number, force = false): void {
    this.send({ case: "attack", value: { targetId, force } });
  }

  useSkill(skillId: number, targetId: number, force = false): void {
    this.send({ case: "useSkill", value: { skillId, targetId, force } });
  }

  respawn(): void {
    this.send({ case: "respawn", value: {} });
  }

  pickUp(id: number): void {
    this.send({ case: "pickUp", value: { id } });
  }

  moveItem(fromPlace: number, fromSlot: number, toPlace: number, toSlot: number): void {
    this.send({ case: "moveItem", value: { fromPlace, fromSlot, toPlace, toSlot } });
  }

  useItem(place: number, slot: number): void {
    this.send({ case: "useItem", value: { place, slot } });
  }

  destroyItem(slot: number, count: number): void {
    this.send({ case: "destroyItem", value: { slot, count } });
  }

  buy(npcId: number, itemId: string, count: number): void {
    this.send({ case: "buy", value: { npcId, itemId, count } });
  }

  /** Telemetria: amostra de FPS (a cada ~60 s). */
  clientStats(fpsAvg: number, fpsLow: number, device: string, seconds: number): void {
    this.send({ case: "clientStats", value: { fpsAvg, fpsLow, device, seconds } });
  }

  /** Pede o ranking (a janela Glória, quando abre e o que tem guardado está velho). */
  rankingRequest(): void {
    this.send({ case: "rankingRequest", value: {} });
  }

  /** Mestra dos Caminhos: falar (anda A Encruzilhada e abre a escolha) ou escolher o caminho. */
  specCmd(kind: SpecCmdKind, npcId: number, spec = ""): void {
    this.send({ case: "specCmd", value: { kind, npcId, spec } });
  }

  /** Maestria (nível máximo): pedir a árvore e o estado, gastar 1 ponto no nó ou redistribuir. */
  masteryCmd(kind: MasteryCmdKind, node = ""): void {
    this.send({ case: "masteryCmd", value: { kind, node } });
  }

  /** Escolhe o título (id da conquista; "" tira). */
  setTitle(id: string): void {
    this.send({ case: "achievementCmd", value: { kind: AchievementCmdKind.ACHIEVEMENT_CMD_SET_TITLE, id } });
  }

  /** Ferreiro: criar um item com a receita. */
  craft(npcId: number, recipeId: string): void {
    this.send({ case: "craft", value: { npcId, recipeId } });
  }

  sell(npcId: number, slot: number, count: number): void {
    this.send({ case: "sell", value: { npcId, slot, count } });
  }

  /** Distribui pontos de atributo (soma aos já gastos). */
  allocateAttrs(add: { str?: number; dex?: number; con?: number; int?: number; wis?: number; spi?: number }): void {
    this.send({ case: "allocateAttrs", value: { add } });
  }

  /** Novos ranks de skills (o servidor confere nível, pontos e o nó de cima). */
  learnSkills(ranks: { skillId: number; rank: number }[]): void {
    this.send({ case: "learnSkills", value: { ranks } });
  }

  /** Barra de atalhos inteira (10 espaços). */
  setHotbar(slots: string[]): void {
    this.send({ case: "setHotbar", value: { slots } });
  }

  close(): void {
    this.ws.close();
  }

  private sendPing(): void {
    this.send({ case: "ping", value: { clientTime: performance.now() } });
  }

  private send(msg: MessageInitShape<typeof ClientMessageSchema>["msg"]): void {
    if (this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(toBinary(ClientMessageSchema, create(ClientMessageSchema, { msg })));
  }
}
