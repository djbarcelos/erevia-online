import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import {
  ClientMessageSchema,
  ServerMessageSchema,
  type ClientMessage,
  type ServerMessage,
} from "../gen/erevia/v1/game_pb";

const PING_INTERVAL_MS = 2000;

/** Conexão WebSocket binária (Protobuf) com o servidor de jogo. */
export class Connection {
  /** Último ping medido (ida e volta), em ms. */
  ping = 0;
  onMessage: (msg: ServerMessage) => void = () => {};
  onClose: (reason: string) => void = () => {};

  private ws: WebSocket;
  private pingTimer: number | undefined;

  constructor(url: string, name: string) {
    this.ws = new WebSocket(url);
    this.ws.binaryType = "arraybuffer";

    this.ws.onopen = () => {
      this.send({ case: "join", value: { $typeName: "erevia.v1.Join", name } });
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
      this.onClose(ev.reason || `conexão encerrada (código ${ev.code})`);
    };
  }

  /** URL padrão: mesmo host da página, rota /ws. */
  static defaultUrl(): string {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    return `${proto}://${location.host}/ws`;
  }

  moveTo(x: number, z: number): void {
    this.send({ case: "moveTo", value: { $typeName: "erevia.v1.MoveTo", x, z } });
  }

  close(): void {
    this.ws.close();
  }

  private sendPing(): void {
    this.send({ case: "ping", value: { $typeName: "erevia.v1.Ping", clientTime: performance.now() } });
  }

  private send(msg: ClientMessage["msg"]): void {
    if (this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(toBinary(ClientMessageSchema, create(ClientMessageSchema, { msg })));
  }
}
