import { ChatChannel, type ChatMessage } from "../gen/erevia/v1/game_pb";

const MAX_LINES = 100;

/**
 * Painel de chat. Enter abre/envia, Esc fecha.
 * Comandos: /w Nome texto (sussurro), /r texto (responde ao último sussurro),
 * /g texto (grupo), /convidar Nome (grupo). Os outros (/amigo, /correio, os emotes
 * /acenar, /sentar, /dancar, /bug e os de GM) vão para o servidor, que responde no chat.
 * Todo texto entra com textContent: nada vindo da rede vira HTML.
 */
export class Chat {
  private log: HTMLOListElement;
  private input: HTMLInputElement;
  private lastWhisperFrom = "";
  /** /convidar Nome (grupo). */
  onPartyInvite: (name: string) => void = () => {};
  /** Troca de mapa: o chat velho para de ouvir o Enter (e não segura o HUD velho). */
  dispose: () => void;

  constructor(
    root: HTMLElement,
    private selfName: string,
    private onSend: (text: string, to: string, channel?: ChatChannel) => void,
  ) {
    this.log = root.querySelector("#chat-log")!;
    this.input = root.querySelector("#chat-input")!;
    const form = root.querySelector<HTMLFormElement>("#chat-form")!;

    form.addEventListener("submit", (ev) => {
      ev.preventDefault();
      this.submit();
    });
    this.input.addEventListener("keydown", (ev) => {
      if (ev.key === "Escape") {
        this.input.value = "";
        this.input.blur();
      }
    });
    const onEnter = (ev: KeyboardEvent) => {
      if (ev.key === "Enter" && document.activeElement !== this.input && !root.hidden) {
        ev.preventDefault();
        this.input.focus();
      }
    };
    window.addEventListener("keydown", onEnter);
    this.dispose = () => window.removeEventListener("keydown", onEnter);
    // Clicar no nome de alguém prepara um sussurro.
    this.log.addEventListener("click", (ev) => {
      const name = (ev.target as HTMLElement).dataset.name;
      if (name && name !== this.selfName) {
        this.input.value = `/w ${name} `;
        this.input.focus();
      }
    });

    root.hidden = false;
  }

  receive(m: ChatMessage): void {
    switch (m.channel) {
      case ChatChannel.GENERAL:
        this.line("general", [this.nameSpan(m.fromName), document.createTextNode(`: ${m.text}`)]);
        break;
      case ChatChannel.WHISPER: {
        const incoming = m.fromName !== this.selfName;
        if (incoming) this.lastWhisperFrom = m.fromName;
        const other = incoming ? m.fromName : m.toName;
        this.line("whisper", [
          document.createTextNode(incoming ? "De " : "Para "),
          this.nameSpan(other),
          document.createTextNode(`: ${m.text}`),
        ]);
        break;
      }
      case ChatChannel.PARTY:
        this.line("party", [document.createTextNode("[Grupo] "), this.nameSpan(m.fromName), document.createTextNode(`: ${m.text}`)]);
        break;
      case ChatChannel.CLAN:
        this.line("clan", [document.createTextNode("[Clã] "), this.nameSpan(m.fromName), document.createTextNode(`: ${m.text}`)]);
        break;
      default:
        this.system(m.text);
    }
  }

  /** Prepara um sussurro para alguém (botão do quadro do alvo). */
  whisperTo(name: string): void {
    this.input.value = `/w ${name} `;
    this.input.focus();
  }

  system(text: string): void {
    this.line("system", [document.createTextNode(text)]);
  }

  /** Anúncio para o servidor inteiro (linha dourada; aparece também na aba Sistema). */
  announce(text: string): void {
    this.line("system announce", [document.createTextNode(`★ ${text}`)]);
  }

  private submit(): void {
    const raw = this.input.value.trim();
    this.input.value = "";
    this.input.blur();
    if (!raw) return;

    if (raw.startsWith("/")) {
      const [cmd, ...rest] = raw.split(/\s+/);
      switch (cmd.toLowerCase()) {
        case "/w":
        case "/whisper": {
          const [to, ...words] = rest;
          const text = words.join(" ");
          if (!to || !text) return this.system("Uso: /w Nome mensagem");
          return this.onSend(text, to);
        }
        case "/r": {
          const text = rest.join(" ");
          if (!this.lastWhisperFrom) return this.system("Ninguém sussurrou para você ainda.");
          if (!text) return this.system("Uso: /r mensagem");
          return this.onSend(text, this.lastWhisperFrom);
        }
        case "/g":
        case "/grupo": {
          const text = rest.join(" ");
          if (!text) return this.system("Uso: /g mensagem para o grupo");
          return this.onSend(text, "", ChatChannel.PARTY);
        }
        case "/c":
        case "/cla":
        case "/clã": {
          const text = rest.join(" ");
          if (!text) return this.system("Uso: /c mensagem para o clã");
          return this.onSend(text, "", ChatChannel.CLAN);
        }
        // Comandos de teste do raid boss (o servidor só aceita em desenvolvimento).
        case "/raid":
          return this.onSend(raw, "");
        case "/convidar":
        case "/invite": {
          if (!rest[0]) return this.system("Uso: /convidar Nome");
          return this.onPartyInvite(rest[0]);
        }
        default:
          // /bug e os comandos de GM: quem decide é o servidor (responde no canal de sistema).
          return this.onSend(raw, "");
      }
    }
    this.onSend(raw, "");
  }

  private nameSpan(name: string): HTMLSpanElement {
    const s = document.createElement("span");
    s.className = "from";
    s.textContent = name;
    s.dataset.name = name;
    s.title = name === this.selfName ? "" : `Sussurrar para ${name}`;
    return s;
  }

  private line(kind: "general" | "whisper" | "system" | "system announce" | "party" | "clan", parts: Node[]): void {
    const stick = this.log.scrollTop + this.log.clientHeight >= this.log.scrollHeight - 8;
    const li = document.createElement("li");
    li.className = kind;
    li.append(...parts);
    this.log.append(li);
    while (this.log.children.length > MAX_LINES) this.log.firstElementChild?.remove();
    if (stick) this.log.scrollTop = this.log.scrollHeight;
  }
}
