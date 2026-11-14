import type { ClassLevel, SubclassState } from "../gen/erevia/v1/game_pb";
import { CLASSES, classInfo } from "../classes";
import { button, el } from "./kit";
import { specInfo } from "./specs";

export interface SubclassActions {
  choose(npcId: number, cls: string): void;
  switchTo(npcId: number, cls: string): void;
}

/**
 * Guardião das Sendas (subclasse): as classes do personagem, com o nível de cada uma e a
 * ativa marcada, e o botão de trocar; depois do juramento, as outras classes base para
 * escolher a subclasse (pede um segundo clique: é definitivo). O servidor confere tudo de
 * novo. Desenhada só ao abrir e quando chega um SubclassState (nada por quadro): a recarga
 * é um texto, redesenhado uma vez quando ela acaba.
 */
export class SubclassWindow {
  readonly root = el("div", "shop-panel spec-panel sub-panel frame");
  private title = el("h2");
  private cards = el("div", "spec-cards");
  private note = el("p", "spec-note small muted");
  private state: SubclassState | null = null;
  /** performance.now() em que a troca libera. */
  private readyAt = 0;
  private readyTimer = 0;
  private arming = "";
  npcId = 0;

  constructor(
    private actions: SubclassActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    const close = button("✕", "x-btn", () => this.close());
    close.title = "Fechar (Esc)";
    this.root.append(
      el("div", "panel-head", this.title, close),
      this.cards,
      this.note,
      el("div", "shop-foot", el("span", "shop-hint", "Equipamento, bolsa, ouro, missões, conquistas e Maestria são do personagem")),
    );
    parent.append(this.root);
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  /** O estado mais recente (ao entrar no jogo e depois de cada mudança). */
  get current(): SubclassState | null {
    return this.state;
  }

  open(title: string, s: SubclassState): void {
    this.npcId = s.npcId;
    this.arming = "";
    this.title.textContent = title;
    this.root.hidden = false;
    this.update(s);
  }

  /** Estado novo sem abrir a janela: redesenha se ela estiver aberta. */
  update(s: SubclassState): void {
    this.state = s;
    this.readyAt = performance.now() + s.cooldownMs;
    window.clearTimeout(this.readyTimer);
    if (s.cooldownMs > 0) this.readyTimer = window.setTimeout(() => this.isOpen && this.render(), s.cooldownMs + 50);
    if (this.isOpen) this.render();
  }

  close(): void {
    this.root.hidden = true;
    this.npcId = 0;
    this.arming = "";
  }

  dispose(): void {
    window.clearTimeout(this.readyTimer);
    this.root.remove();
  }

  private render(): void {
    const s = this.state;
    if (!s) return;
    const left = Math.max(0, this.readyAt - performance.now());
    const bonus = s.bonus
      ? `Duas Sendas: +${Math.round(s.bonusXp * 100)}% de XP para a classe mais baixa.`
      : `Com uma subclasse no nível ${s.bonusLevel}: o título Duas Sendas e +${Math.round(s.bonusXp * 100)}% de XP para a classe mais baixa.`;
    const wait = left > 0 ? ` Próxima troca em ${Math.ceil(left / 60_000)} min.` : "";
    this.note.textContent = `${bonus}${wait} A troca é só fora de combate; o que a classe nova não pode usar vai para a bolsa.`;
    const cards: HTMLElement[] = s.classes.map((c) => this.mine(c, s, left > 0));
    if (s.canChoose) {
      const have = new Set(s.classes.map((c) => c.class));
      for (const c of CLASSES) if (!have.has(c.id)) cards.push(this.option(c.id, s));
    }
    this.cards.replaceChildren(...cards);
  }

  /** Uma classe do personagem: nível, caminho e o botão de trocar. */
  private mine(c: ClassLevel, s: SubclassState, waiting: boolean): HTMLElement {
    const info = classInfo(c.class);
    const active = c.class === s.active;
    const spec = specInfo(c.spec);
    const go = button(active ? "Classe ativa" : `Trocar para ${info.label}`, "btn spec-go", () => this.actions.switchTo(this.npcId, c.class));
    go.disabled = active || waiting;
    const card = el(
      "section",
      `spec-card${active ? " mine" : ""}`,
      el("h3", "title spec-name", info.label),
      el("p", "spec-desc small", c.main ? "Classe principal (a do corpo)" : "Subclasse"),
      el("div", "spec-bonus", el("span", "label", "Nível"), el("b", null, String(c.level))),
      el("div", "spec-bonus", el("span", "label", "Caminho"), el("b", null, spec?.name ?? "—")),
      go,
    );
    card.style.setProperty("--sc", info.color);
    return card;
  }

  /** Uma classe para escolher como subclasse (depois do juramento). */
  private option(cls: string, s: SubclassState): HTMLElement {
    const info = classInfo(cls);
    const armed = this.arming === cls;
    const go = button(armed ? "Confirmar (é definitivo)" : `Escolher ${info.label}`, `btn ${armed ? "btn-gold" : ""} spec-go`, () => {
      if (!armed) {
        this.arming = cls;
        this.render();
        return;
      }
      this.arming = "";
      this.actions.choose(this.npcId, cls);
    });
    const card = el(
      "section",
      "spec-card sub-option",
      el("h3", "title spec-name", info.label),
      el("p", "spec-desc small", info.desc),
      el("div", "spec-bonus", el("span", "label", "Começa no"), el("b", null, `nível ${s.startLevel}`)),
      go,
    );
    card.style.setProperty("--sc", info.color);
    return card;
  }
}
