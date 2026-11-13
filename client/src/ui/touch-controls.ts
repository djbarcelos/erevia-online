/**
 * Joystick virtual flutuante: o toque em qualquer ponto da zona inferior esquerda (ou
 * direita, no modo canhoto) vira o centro do joystick. Devolve um vetor (-1..1).
 */

import { el, settings } from "./kit";

export interface Joystick {
  root: HTMLDivElement;
  dispose(): void;
}

export function createJoystick(onMove: (v: { x: number; y: number } | null) => void): Joystick {
  const zone = el("div", "joy-zone");
  const base = el("div", "joy-base", el("span", "joy-ring"));
  const knob = el("div", "joy-knob");
  base.append(knob);
  zone.append(base);
  zone.setAttribute("aria-label", "Joystick de movimento");

  let active: number | null = null;
  let cx = 0;
  let cy = 0;

  const apply = () => {
    const s = settings.get();
    zone.style.setProperty("--joy", `${(130 * s.joySize) / 100}px`);
    zone.style.setProperty("--joy-o", String(s.joyOpacity / 100));
  };
  apply();
  const off = settings.on(apply);

  const radius = () => base.offsetWidth / 2;

  const rest = () => {
    base.classList.remove("on");
    base.style.left = "";
    base.style.top = "";
    knob.style.transform = "";
    onMove(null);
  };

  zone.addEventListener("pointerdown", (ev) => {
    if (active !== null) return;
    active = ev.pointerId;
    zone.setPointerCapture(ev.pointerId);
    const r = zone.getBoundingClientRect();
    cx = ev.clientX;
    cy = ev.clientY;
    base.style.left = `${cx - r.left}px`;
    base.style.top = `${cy - r.top}px`;
    base.classList.add("on");
    ev.preventDefault();
  });
  zone.addEventListener("pointermove", (ev) => {
    if (ev.pointerId !== active) return;
    const R = radius();
    let dx = ev.clientX - cx;
    let dy = ev.clientY - cy;
    const d = Math.hypot(dx, dy);
    if (d > R) {
      dx = (dx / d) * R;
      dy = (dy / d) * R;
    }
    knob.style.transform = `translate(${dx}px, ${dy}px)`;
    const k = Math.min(1, d / R);
    // Zona morta pequena para não andar sem querer.
    if (k < 0.12) onMove(null);
    else onMove({ x: (dx / R) * (k > 0.85 ? 1 / k : 1), y: (dy / R) * (k > 0.85 ? 1 / k : 1) });
  });
  const end = (ev: PointerEvent) => {
    if (ev.pointerId !== active) return;
    active = null;
    rest();
  };
  zone.addEventListener("pointerup", end);
  zone.addEventListener("pointercancel", end);

  return {
    root: zone,
    dispose() {
      off();
    },
  };
}
