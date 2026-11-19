import type { SkillInfo, SpecChoice, SpecInfo } from "../gen/erevia/v1/game_pb";
import { skillIcon } from "./icons";
import { uniqueIds } from "./art";
import { art, button, el, fmt, tooltip } from "./kit";
import { goldIcon, type ItemStore } from "./items";
import { classSpecs } from "./specs";

export interface SpecActions {
  choose(npcId: number, spec: string): void;
  /** Redistribuir a árvore da evolução (paga SpecChoice.respecCost). */
  respec(npcId: number): void;
  myClass(): string;
  /** Skills da classe (Welcome.skills): as de cada caminho têm spec preenchido. */
  skills(): SkillInfo[];
}

/**
 * Mestra dos Caminhos (missão A Encruzilhada): as duas evoluções da classe lado a lado, com
 * o bônus de atributos e a árvore de cada uma (as 2 iniciais primeiro). Evoluir pede um
 * segundo clique; quem já evoluiu pode trocar ou redistribuir a árvore pagando. O servidor
 * confere tudo de novo. Desenhada só ao abrir e quando chega um SpecChoice (nada por quadro).
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
      el("div", "shop-foot", el("div", "shop-gold", gi, this.goldEl), el("span", "shop-hint", "A árvore da evolução tem marcos nos níveis 20 a 40 (tecla K)")),
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
      ? `Trocar de evolução custa ${fmt.format(cost)} moedas e redistribuir a árvore, ${fmt.format(Number(c.respecCost))}; em ambos os casos os pontos voltam para você.`
      : "Ao evoluir, a árvore base dá lugar à da evolução e todos os pontos voltam para você. Trocar depois custa caro.";
    this.cards.replaceChildren(...classSpecs(this.actions.myClass()).map((s) => this.card(s, c, cost)));
  }

  private card(s: SpecInfo, c: SpecChoice, cost: number): HTMLElement {
    const mine = c.current === s.id;
    const starter = (k: SkillInfo) => (s.starterSkills.includes(k.id) ? 0 : 1);
    const skills = this.actions
      .skills()
      .filter((k) => k.spec === s.id)
      .sort((a, b) => starter(a) - starter(b) || a.levelReq - b.levelReq || a.branch - b.branch);
    const list = el("ul", "spec-skills");
    for (const k of skills) {
      const tag = starter(k) === 0 ? "inicial" : `Nv ${k.levelReq}`;
      const li = el("li", null, art(uniqueIds(skillIcon(k.fx)), `spec-icon ${k.passive ? "passive" : "active"}`), el("div", null, el("b", null, k.name), el("small", "muted", `${tag} · ${k.passive ? "passiva" : "ativa"}`)));
      tooltip(li, () => `<h4>${k.name}</h4><div class="sub">${k.passive ? "Passiva" : "Ativa"} · Nível ${k.levelReq}</div><div>${k.ranks[0]?.text ?? ""}</div><div class="desc">${k.description}</div>`);
      list.append(li);
    }
    let label = "Evoluir";
    if (mine) label = "Sua evolução";
    else if (c.current) label = `Trocar por ${fmt.format(cost)} moedas`;
    if (this.arming === s.id) label = c.current ? "Confirmar a troca" : "Confirmar a evolução";
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
    const respecCost = Number(c.respecCost);
    const respec = mine
      ? button(this.arming === "respec" ? "Confirmar: árvore volta às iniciais" : `Redistribuir a árvore (${fmt.format(respecCost)})`, `btn btn-sm ${this.arming === "respec" ? "btn-gold" : ""}`, () => {
          if (this.arming !== "respec") {
            this.arming = "respec";
            this.render();
            return;
          }
          this.arming = "";
          this.actions.respec(this.npcId);
        })
      : null;
    if (respec) respec.disabled = this.store.gold < respecCost;
    const card = el(
      "section",
      `spec-card${mine ? " mine" : ""}`,
      el("h3", "title spec-name", s.name),
      el("p", "spec-desc small", s.description),
      el("div", "spec-bonus", el("span", "label", "Bônus"), el("b", null, s.bonusText)),
      list,
      go,
    );
    if (respec) card.append(respec);
    card.style.setProperty("--sc", s.color);
    return card;
  }
}
