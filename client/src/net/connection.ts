import { create, fromBinary, toBinary, type MessageInitShape } from "@bufbuild/protobuf";
import { ClientMessageSchema, ServerMessageSchema, type ServerMessage } from "../gen/erevia/v1/game_pb";

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

  chat(text: string, to: string): void {
    this.send({ case: "chatSend", value: { text, to } });
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
