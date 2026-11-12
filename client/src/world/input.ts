import { Color3, Mesh, MeshBuilder, PointerEventTypes, Scene, StandardMaterial } from "@babylonjs/core";

/** Segurando o botão, reenvia o destino no máximo a cada X ms (servidor aceita ~10/s). */
const HOLD_RESEND_MS = 150;

/**
 * Clique esquerdo no chão → onMove(x, z). Segurar o botão continua andando
 * em direção ao cursor, como no L2. Mostra um marcador no destino.
 */
export function setupClickToMove(
  scene: Scene,
  ground: Mesh,
  onMove: (x: number, z: number) => { x: number; z: number },
): void {
  const marker = createMarker(scene);
  let holding = false;
  let lastSent = 0;

  const tryMove = (force: boolean) => {
    const now = performance.now();
    if (!force && now - lastSent < HOLD_RESEND_MS) return;
    const pick = scene.pick(scene.pointerX, scene.pointerY, (m) => m === ground);
    if (!pick?.hit || !pick.pickedPoint) return;
    lastSent = now;
    const dest = onMove(pick.pickedPoint.x, pick.pickedPoint.z);
    marker.position.set(dest.x, 0.03, dest.z);
    marker.setEnabled(true);
    marker.scaling.setAll(1);
    marker.visibility = 1;
  };

  scene.onPointerObservable.add((info) => {
    const ev = info.event as PointerEvent;
    switch (info.type) {
      case PointerEventTypes.POINTERDOWN:
        if (ev.button !== 0) return;
        holding = true;
        tryMove(true);
        break;
      case PointerEventTypes.POINTERMOVE:
        if (holding) tryMove(false);
        break;
      case PointerEventTypes.POINTERUP:
        if (ev.button === 0) holding = false;
        break;
    }
  });

  // Marcador encolhe e some.
  scene.onBeforeRenderObservable.add(() => {
    if (!marker.isEnabled()) return;
    const dt = scene.getEngine().getDeltaTime() / 1000;
    marker.visibility -= dt * 1.2;
    marker.scaling.scaleInPlace(1 - dt * 0.8);
    if (marker.visibility <= 0) marker.setEnabled(false);
  });
}

function createMarker(scene: Scene): Mesh {
  const ring = MeshBuilder.CreateTorus("moveMarker", { diameter: 1.2, thickness: 0.08, tessellation: 32 }, scene);
  const mat = new StandardMaterial("markerMat", scene);
  mat.emissiveColor = new Color3(0.95, 0.8, 0.4);
  mat.disableLighting = true;
  ring.material = mat;
  ring.isPickable = false;
  ring.setEnabled(false);
  return ring;
}
