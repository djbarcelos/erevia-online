import type { ClassId } from "../api";
import { bodyPalettes, normalLook, sameLook, type BodyLook } from "../world/body-look";
import { bodyEditor, type BodyEditor } from "./body-editor";
import { goldIcon, type ItemStore } from "./items";
import { button, el, fmt } from "./kit";
import { createPreview, type Appearance, type Preview } from "./preview3d";

export interface BarberActions {
  /** Pede ao servidor o visual novo (ele cobra e avisa a todos). */
  change(npcId: number, look: BodyLook): void;
  /** Classe, armas e emblema atuais (para o palco) e o visual atual. */
  me(): { myClass: string; look: Appearance; body: BodyLook };
}

/** Barbeiro: muda rosto, pele, cabelo, roupa e chapéu, com prévia 3D, por moedas. */
export class BarberWindow {
  readonly root = el("div", "shop-panel barber-panel frame");
  private title = el("h2");
  private stage = el("div", "barber-stage");
  private editorBox = el("div", "barber-editor");
  private goldEl = el("span", "shop-gold-value");
  private priceEl = el("span", "shop-hint");
  private confirm: HTMLButtonElement;
  private preview: Preview | null = null;
  private editor: BodyEditor | null = null;
  private current: BodyLook | null = null;
  private draft: BodyLook | null = null;
  private price = 0;
  private cls = "knight";
  private unsubscribe: () => void;
  private seq = 0;
  npcId = 0;

  constructor(
    private store: ItemStore,
    private actions: BarberActions,
    parent: HTMLElement,
  ) {
    this.root.setAttribute("role", "dialog");
    this.root.hidden = true;
    const close = button("✕", "x-btn", () => this.close());
    close.title = "Fechar (Esc)";
    const gi = el("span", "shop-gold-icon");
    gi.innerHTML = goldIcon();
    this.confirm = button("Confirmar", "btn btn-gold", () => {
      if (!this.draft) return;
      this.actions.change(this.npcId, this.draft);
      this.close();
    });
    this.root.append(
      el("div", "panel-head", this.title, close),
      el("div", "barber-body", this.stage, this.editorBox),
      el("div", "shop-foot barber-foot", el("div", "shop-gold", gi, this.goldEl), this.priceEl, this.confirm),
    );
    parent.append(this.root);
    this.unsubscribe = store.onChange(() => this.render());
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  async open(npcId: number, title: string): Promise<void> {
    const my = ++this.seq;
    this.npcId = npcId;
    this.title.textContent = title;
    this.root.hidden = false;
    const pal = await bodyPalettes();
    if (my !== this.seq || !this.isOpen) return;
    if (!pal) {
      this.editorBox.replaceChildren(el("p", "muted small", "Não foi possível carregar as opções. Tente de novo."));
      return;
    }
    const me = this.actions.me();
    this.cls = me.myClass;
    this.price = pal.barberPrice;
    this.current = normalLook(this.cls, me.body, pal);
    this.draft = { ...this.current };
    this.editor = bodyEditor(pal, this.cls, this.draft, (l) => {
      this.draft = l;
      this.preview?.setBody(l);
      this.render();
    });
    this.editorBox.replaceChildren(this.editor.root);
    // Palco 3D (um contexto WebGL só enquanto a janela está aberta).
    const canvas = el("canvas", "barber-canvas");
    canvas.setAttribute("aria-label", "Prévia do visual — arraste para girar");
    this.stage.replaceChildren(canvas);
    this.preview?.dispose();
    this.preview = createPreview(canvas, { radius: 4.4 });
    void this.preview.show(this.cls as ClassId, { ...me.look, body: this.draft });
    this.render();
  }

  close(): void {
    this.seq++;
    this.root.hidden = true;
    this.npcId = 0;
    this.preview?.dispose();
    this.preview = null;
    this.stage.replaceChildren();
  }

  dispose(): void {
    this.close();
    this.unsubscribe();
    this.root.remove();
  }

  private render(): void {
    this.goldEl.textContent = fmt.format(this.store.gold);
    if (!this.isOpen || !this.draft || !this.current) return;
    const changed = !sameLook(this.draft, this.current);
    const poor = this.store.gold < this.price;
    this.priceEl.textContent = changed ? `Custa ${fmt.format(this.price)} moedas` : "Escolha um visual novo";
    this.priceEl.classList.toggle("bad", changed && poor);
    this.confirm.disabled = !changed || poor;
  }
}
