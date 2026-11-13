import type { SkillInfo, SpecChoice, SpecInfo } from "../gen/erevia/v1/game_pb";
import { skillIcon } from "./icons";
import { uniqueIds } from "./art";
import { art, button, el, fmt, tooltip } from "./kit";
import { goldIcon, type ItemStore } from "./items";
import { classSpecs } from "./specs";

export interface SpecActions {
  choose(npcId: number, spec: string): void;
  myClass(): string;
  /** Skills da classe (Welcome.skills): as de cada caminho têm spec preenchido. */
  skills(): SkillInfo[];
}

/**
 * Mestra dos Caminhos (missão A Encruzilhada): os dois caminhos da classe lado a lado, com
 * o bônus de atributos e as 3 skills de cada um. Escolher pede um segundo clique (é
 * definitivo); quem já escolheu pode trocar pagando. O servidor confere tudo de novo.
 * Desenhada só ao abrir e quando chega um SpecChoice (nada por quadro).
 */
export class SpecWindow {
  readonly root = el("div", "shop-panel spec-panel frame");
  private title = el("h2");
  private cards = el("div", "spec-cards");
  private note = el("p", "spec-note small muted");
  private goldEl = el("span", "shop-gold-value");
  private choice: SpecChoice | null = null;
  private arming = "";
  private unsubscribe: () => void;
  npcId = 0;

  constructor(
    private store: ItemStore,
    private actions: SpecActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    const close = button("✕", "x-btn", () => this.close());
    close.title = "Fechar (Esc)";
    const gi = el("span", "shop-gold-icon");
    gi.innerHTML = goldIcon();
    this.root.append(
      el("div", "panel-head", this.title, close),
      this.cards,
      this.note,
      el("div", "shop-foot", el("div", "shop-gold", gi, this.goldEl), el("span", "shop-hint", "Skills novas nos níveis 20, 24 e 28 (tecla K)")),
    );
    parent.append(this.root);
    this.unsubscribe = store.onChange(() => this.isOpen && this.render());
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  open(title: string, c: SpecChoice): void {
    this.npcId = c.npcId;
    this.choice = c;
    this.arming = "";
    this.title.textContent = title;
    this.root.hidden = false;
    this.render();
  }

  close(): void {
    this.root.hidden = true;
    this.npcId = 0;
    this.arming = "";
  }

  dispose(): void {
    this.unsubscribe();
    this.root.remove();
  }

  private render(): void {
    const c = this.choice;
    if (!c) return;
    this.goldEl.textContent = fmt.format(this.store.gold);
    const cost = Number(c.changeCost);
    this.note.textContent = c.current
      ? `Trocar de caminho custa ${fmt.format(cost)} moedas; os pontos das skills do caminho atual voltam para você.`
      : "Escolha com calma: o caminho é para sempre (trocar depois custa caro).";
    this.cards.replaceChildren(...classSpecs(this.actions.myClass()).map((s) => this.card(s, c, cost)));
  }

  private card(s: SpecInfo, c: SpecChoice, cost: number): HTMLElement {
    const mine = c.current === s.id;
    const skills = this.actions.skills().filter((k) => k.spec === s.id).sort((a, b) => a.tier - b.tier);
    const list = el("ul", "spec-skills");
    for (const k of skills) {
      const li = el("li", null, art(uniqueIds(skillIcon(k.fx)), `spec-icon ${k.passive ? "passive" : "active"}`), el("div", null, el("b", null, k.name), el("small", "muted", `Nv ${k.levelReq} · ${k.passive ? "passiva" : "ativa"}`)));
      tooltip(li, () => `<h4>${k.name}</h4><div class="sub">${k.passive ? "Passiva" : "Ativa"} · Nível ${k.levelReq}</div><div>${k.ranks[0]?.text ?? ""}</div><div class="desc">${k.description}</div>`);
      list.append(li);
    }
    let label = "Escolher este caminho";
    if (mine) label = "Seu caminho";
    else if (c.current) label = `Trocar por ${fmt.format(cost)} moedas`;
    if (this.arming === s.id) label = c.current ? "Confirmar a troca" : "Confirmar (é definitivo)";
    const go = button(label, `btn ${this.arming === s.id ? "btn-gold" : ""} spec-go`, () => {
      if (this.arming !== s.id) {
        this.arming = s.id;
        this.render();
        return;
      }
      this.arming = "";
      this.actions.choose(this.npcId, s.id);
    });
    go.disabled = mine || (!!c.current && this.store.gold < cost);
    const card = el(
      "section",
      `spec-card${mine ? " mine" : ""}`,
      el("h3", "title spec-name", s.name),
      el("p", "spec-desc small", s.description),
      el("div", "spec-bonus", el("span", "label", "Bônus"), el("b", null, s.bonusText)),
      list,
      go,
    );
    card.style.setProperty("--sc", s.color);
    return card;
  }
}
