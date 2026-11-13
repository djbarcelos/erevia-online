import { InviteKind, type Invite } from "../gen/erevia/v1/game_pb";
import { button, el } from "./kit";

/**
 * Convites de grupo e de clã: um cartão no alto da tela com Aceitar/Recusar e a barra
 * do tempo que falta (30 s). Chegando outro convite, ele espera na fila.
 */
export class InvitePrompt {
  readonly root = el("div", "invite-card frame glass");
  private queue: { inv: Invite; at: number }[] = [];
  private timer = 0;
  private text = el("p", "invite-text");
  private fill = el("i");

  constructor(
    parent: HTMLElement,
    private reply: (id: number, accept: boolean) => void,
  ) {
    this.root.setAttribute("role", "alertdialog");
    this.root.hidden = true;
    const yes = button("Aceitar", "btn btn-gold btn-sm", () => this.answer(true));
    const no = button("Recusar", "btn btn-sm", () => this.answer(false));
    this.root.append(this.text, el("div", "invite-time", this.fill), el("div", "invite-actions", no, yes));
    parent.append(this.root);
  }

  get open(): boolean {
    return !this.root.hidden;
  }

  push(inv: Invite): void {
    this.queue.push({ inv, at: performance.now() });
    if (this.queue.length === 1) this.show();
  }

  /** Esc recusa o convite da vez. */
  answer(accept: boolean): void {
    const cur = this.queue.shift();
    if (cur) this.reply(cur.inv.id, accept);
    this.show();
  }

  dispose(): void {
    window.clearInterval(this.timer);
    this.root.remove();
  }

  private show(): void {
    window.clearInterval(this.timer);
    // Pula os que venceram enquanto esperavam.
    while (this.queue.length && this.left(this.queue[0]) <= 0) this.queue.shift();
    const cur = this.queue[0];
    this.root.hidden = !cur;
    if (!cur) return;
    const { inv } = cur;
    this.text.replaceChildren(
      el("b", null, inv.fromName),
      inv.kind === InviteKind.CLAN ? ` convidou você para o clã ` : inv.kind === InviteKind.TRADE ? " quer trocar com você." : " convidou você para o grupo.",
      inv.kind === InviteKind.CLAN ? el("b", "invite-clan", `‹${inv.extra}›`) : "",
    );
    const tick = () => {
      const left = this.left(cur);
      this.fill.style.width = `${Math.max(0, (left / inv.expiresMs) * 100)}%`;
      if (left <= 0) this.answer(false);
    };
    tick();
    this.timer = window.setInterval(tick, 200);
  }

  private left(q: { inv: Invite; at: number }): number {
    return q.inv.expiresMs - (performance.now() - q.at);
  }
}
