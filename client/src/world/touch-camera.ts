/**
 * Câmera no toque: arrastar um dedo no mundo gira a câmera; dois dedos (pinça) dão zoom.
 * (No mouse continua igual: botão direito gira, roda dá zoom — ver world/scene.ts.)
 */

import type { ArcRotateCamera } from "@babylonjs/core";

export function setupTouchCamera(canvas: HTMLCanvasElement, camera: ArcRotateCamera): () => void {
  const touches = new Map<number, { x: number; y: number }>();
  let pinch = 0;

  const down = (e: PointerEvent) => {
    if (e.pointerType !== "touch") return;
    touches.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (touches.size === 2) {
      const [a, b] = [...touches.values()];
      pinch = Math.hypot(a.x - b.x, a.y - b.y);
    }
  };
  const move = (e: PointerEvent) => {
    const prev = touches.get(e.pointerId);
    if (!prev) return;
    if (touches.size === 1) {
      camera.alpha -= (e.clientX - prev.x) * 0.006;
      const lo = camera.lowerBetaLimit ?? 0.35;
      const hi = camera.upperBetaLimit ?? 1.4;
      camera.beta = Math.max(lo, Math.min(hi, camera.beta - (e.clientY - prev.y) * 0.004));
    }
    touches.set(e.pointerId, { x: e.clientX, y: e.clientY });
    if (touches.size === 2) {
      const [a, b] = [...touches.values()];
      const d = Math.hypot(a.x - b.x, a.y - b.y);
      if (pinch > 0 && d > 0) {
        const lo = camera.lowerRadiusLimit ?? 5;
        const hi = camera.upperRadiusLimit ?? 45;
        camera.radius = Math.max(lo, Math.min(hi, camera.radius * (pinch / d)));
      }
      pinch = d;
    }
  };
  const up = (e: PointerEvent) => {
    touches.delete(e.pointerId);
    if (touches.size < 2) pinch = 0;
  };
  canvas.addEventListener("pointerdown", down);
  canvas.addEventListener("pointermove", move);
  canvas.addEventListener("pointerup", up);
  canvas.addEventListener("pointercancel", up);
  return () => {
    canvas.removeEventListener("pointerdown", down);
    canvas.removeEventListener("pointermove", move);
    canvas.removeEventListener("pointerup", up);
    canvas.removeEventListener("pointercancel", up);
  };
}
