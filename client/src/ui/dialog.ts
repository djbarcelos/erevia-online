/**
 * Conversa com NPCs (tunnel#71): a fala do NPC e as opções que o servidor mandou no
 * NpcDialog (aceitar e entregar missões, perguntar sobre a região, o serviço de sempre do
 * NPC e sair). Nada de texto de jogo aqui: tudo vem dos dados do servidor. Cada escolha volta
 * como NpcTalk, e o servidor responde com outro NpcDialog (ou abre o serviço).
 */

import { DialogOptionKind, type DialogOption, type NpcDialog } from "../gen/erevia/v1/game_pb";
import { button, el } from "./kit";
import "../styles/dialog.css";

export interface DialogActions {
  accept(npcId: number, questId: string): void;
  turnIn(npcId: number, questId: string): void;
  ask(npcId: number, topic: string): void;
  /** O serviço de sempre do NPC (loja, armazém, contratos...): a janela dele. */
  service(npcId: number): void;
}

/** Uma linha da lista de opções, na ordem em que aparece. */
export interface DialogRow {
  option: DialogOption;
  /** Destaque: entregar e aceitar missão vêm primeiro e em dourado. */
  primary: boolean;
  /** Texto de apoio (o que a missão pede) e a recompensa. */
  detail: string;
  reward: string;
}

const ORDER: Record<number, number> = {
  [DialogOptionKind.DIALOG_OPTION_TURN_IN]: 0,
  [DialogOptionKind.DIALOG_OPTION_ACCEPT]: 1,
  [DialogOptionKind.DIALOG_OPTION_SERVICE]: 2,
  [DialogOptionKind.DIALOG_OPTION_ASK]: 3,
  [DialogOptionKind.DIALOG_OPTION_LEAVE]: 4,
};

/** As opções na ordem da janela: entregar, aceitar, serviço, perguntas e sair (estável). */
export function dialogRows(d: NpcDialog): DialogRow[] {
  return d.options
    .map((option, i) => ({ option, i }))
    .sort((a, b) => (ORDER[a.option.kind] ?? 9) - (ORDER[b.option.kind] ?? 9) || a.i - b.i)
    .map(({ option }) => ({
      option,
      primary: option.kind === DialogOptionKind.DIALOG_OPTION_TURN_IN || option.kind === DialogOptionKind.DIALOG_OPTION_ACCEPT,
      detail: option.kind === DialogOptionKind.DIALOG_OPTION_ACCEPT ? option.detail : "",
      reward: option.reward ? `Recompensa: ${option.reward}` : "",
    }));
}

export class NpcDialogWindow {
  readonly root = el("div", "shop-panel dlg-panel frame");
  private title = el("h2");
  private text = el("p", "dlg-text");
  private list = el("div", "dlg-options");
  npcId = 0;

  constructor(
    private actions: DialogActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    const close = button("✕", "x-btn", () => this.close());
    close.title = "Fechar (Esc)";
    this.root.append(el("div", "panel-head", this.title, close), el("div", "dlg-body", this.text, this.list));
    parent.append(this.root);
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  open(d: NpcDialog): void {
    this.npcId = d.npcId;
    this.title.textContent = d.title ? `${d.title} · ${d.name}` : d.name;
    this.text.textContent = d.text;
    this.list.replaceChildren(...dialogRows(d).map((r) => this.row(r)));
    this.root.hidden = false;
  }

  close(): void {
    this.root.hidden = true;
    this.npcId = 0;
  }

  dispose(): void {
    this.root.remove();
  }

  private row(r: DialogRow): HTMLElement {
    const o = r.option;
    const b = button(o.label, r.primary ? "btn btn-gold dlg-btn" : "btn dlg-btn", () => this.choose(o));
    if (!r.detail && !r.reward) return b;
    return el("div", "dlg-quest", b, r.detail ? el("p", "dlg-detail small", r.detail) : null, r.reward ? el("p", "dlg-reward small", r.reward) : null);
  }

  private choose(o: DialogOption): void {
    const id = this.npcId;
    switch (o.kind) {
      case DialogOptionKind.DIALOG_OPTION_ACCEPT:
        this.actions.accept(id, o.questId);
        break;
      case DialogOptionKind.DIALOG_OPTION_TURN_IN:
        this.actions.turnIn(id, o.questId);
        break;
      case DialogOptionKind.DIALOG_OPTION_ASK:
        this.actions.ask(id, o.topic);
        break;
      case DialogOptionKind.DIALOG_OPTION_SERVICE:
        this.close();
        this.actions.service(id);
        break;
      default:
        this.close();
    }
  }
}
