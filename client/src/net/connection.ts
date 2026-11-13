import { create, fromBinary, toBinary, type MessageInitShape } from "@bufbuild/protobuf";
import { ChatChannel, ClanCmdKind, ClanRank, ClientMessageSchema, PartyCmdKind, ServerMessageSchema, type ServerMessage, type TradeCmdKind } from "../gen/erevia/v1/game_pb";

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
      const msg = fromBinary(ServerMessageSchema, new Uint8Array(ev.data));
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
