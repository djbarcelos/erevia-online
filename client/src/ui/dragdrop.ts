/**
 * Arrastar e soltar com eventos de ponteiro (funciona com mouse e toque). Um "fantasma"
 * com o ícone segue o dedo/cursor; ao soltar, chama onDrop com o elemento embaixo.
 */

import { el, hideTip } from "./kit";

export interface DragOpts {
  icon: string;
  /** Chamado a cada movimento com o elemento embaixo (para destacar alvos). */
  onOver?(target: HTMLElement | null): void;
  onDrop(target: HTMLElement | null): void;
  /** Distância mínima para começar (px). */
  threshold?: number;
}

/** Liga ao pointerdown de um elemento. Devolve true se virou arrasto (para ignorar o clique). */
export function beginDrag(ev: PointerEvent, opts: DragOpts): void {
  if (ev.button !== 0) return;
  const sx = ev.clientX;
  const sy = ev.clientY;
  const threshold = opts.threshold ?? (ev.pointerType === "mouse" ? 6 : 12);
  let ghost: HTMLDivElement | null = null;
  const src = ev.currentTarget as HTMLElement;
  // No toque, o arrasto só começa depois de segurar um pouco (para não brigar com a rolagem).
  const holdUntil = ev.pointerType === "mouse" ? 0 : performance.now() + 260;

  const under = (x: number, y: number) => {
    if (ghost) ghost.style.display = "none";
    const t = document.elementFromPoint(x, y) as HTMLElement | null;
    if (ghost) ghost.style.display = "";
    return t;
  };

  const move = (e: PointerEvent) => {
    if (!ghost) {
      if (Math.hypot(e.clientX - sx, e.clientY - sy) < threshold) return;
      if (performance.now() < holdUntil) {
        cleanup();
        return;
      }
      ghost = el("div", "drag-ghost");
      ghost.innerHTML = opts.icon;
      document.body.append(ghost);
      src.classList.add("dragging");
      hideTip();
    }
    ghost.style.left = `${e.clientX}px`;
    ghost.style.top = `${e.clientY}px`;
    opts.onOver?.(under(e.clientX, e.clientY));
    e.preventDefault();
  };

  const up = (e: PointerEvent) => {
    const wasDrag = !!ghost;
    const target = wasDrag ? under(e.clientX, e.clientY) : null;
    cleanup();
    if (wasDrag) {
      // Impede o clique que viria logo depois.
      src.addEventListener("click", stop, { capture: true, once: true });
      opts.onOver?.(null);
      opts.onDrop(target);
    }
  };

  const stop = (e: Event) => {
    e.stopPropagation();
    e.preventDefault();
  };

  function cleanup() {
    ghost?.remove();
    src.classList.remove("dragging");
    removeEventListener("pointermove", move);
    removeEventListener("pointerup", up);
    removeEventListener("pointercancel", up);
  }

  addEventListener("pointermove", move, { passive: false });
  addEventListener("pointerup", up);
  addEventListener("pointercancel", up);
}
