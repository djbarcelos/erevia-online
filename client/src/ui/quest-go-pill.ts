/**
 * Pílula do "Ir" das missões (acima da pílula da auto-caça): para onde está indo e a
 * distância, a oferta da Pedra de Teleporte, e o "Caçar aqui" ao chegar numa área de caça.
 * Atualizada pelo intervalo do Ir (4×/s, só enquanto ativo); os botões só são refeitos
 * quando o estado muda e o texto só quando muda.
 */

import type { QuestGo } from "../world/questgo";
import { button, el, fmt } from "./kit";

export interface GoPillActions {
  stone(): void;
  hunt(): void;
  stop(): void;
}

export class GoPill {
  readonly el = el("div", "go-pill");
  private text = el("span");
  private actions = el("div", "go-actions");
  private key = "";
  private lastText = "";

  constructor(
    private go: QuestGo,
    private a: GoPillActions,
  ) {
    this.el.hidden = true;
    this.el.setAttribute("role", "status");
    this.el.append(el("span", "auto-dot"), this.text, this.actions);
  }

  update(): void {
    const g = this.go;
    const t = g.target;
    // Caçando, a pílula da auto-caça mostra o resto (e tem o Parar).
    if (!t || g.phase === "idle" || g.phase === "hunting") {
      if (!this.el.hidden) this.el.hidden = true;
      this.key = "";
      return;
    }
    const o = g.offer;
    const key = `${g.phase}|${o ? `${o.npcId}:${o.destId}:${o.afford}` : ""}`;
    if (key !== this.key) {
      this.key = key;
      const btns: HTMLButtonElement[] = [];
      if (g.phase === "walking" && o) {
        const b = button(`Viajar pela Pedra (${fmt.format(o.price)} moedas)`, "btn btn-sm btn-gold", () => this.a.stone());
        b.disabled = !o.afford;
        b.title = o.afford
          ? `Ande até a pedra ${o.from} e escolha ${o.to}: ~${Math.round(o.walk)} m a pé em vez de ~${Math.round(o.direct)} m.`
          : `A viagem para ${o.to} custa ${fmt.format(o.price)} moedas.`;
        btns.push(b);
      }
      if (g.phase === "arrived") btns.push(button("Caçar aqui", "btn btn-sm btn-gold", () => this.a.hunt()));
      btns.push(button(g.phase === "arrived" ? "✕" : "Parar", "btn btn-sm btn-ghost", () => this.a.stop()));
      this.actions.replaceChildren(...btns);
    }
    const text =
      g.phase === "walking"
        ? `Indo até ${t.name} · ${Math.round(g.distance)} m`
        : g.phase === "toStone" && o
          ? `Pedra ${o.from} → ${o.to} · escolha o destino na pedra`
          : `Você chegou: ${t.name}`;
    if (text !== this.lastText) this.text.textContent = this.lastText = text;
    if (this.el.hidden) this.el.hidden = false;
  }
}
