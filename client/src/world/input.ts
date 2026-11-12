import { Color3, Mesh, MeshBuilder, PointerEventTypes, Scene, StandardMaterial } from "@babylonjs/core";

/** Segurando o botão, reenvia o destino no máximo a cada X ms (servidor aceita ~10/s). */
const HOLD_RESEND_MS = 150;
const HOVER_CHECK_MS = 80;

export interface InputHandlers {
  /** Clique no chão. Devolve o destino aceito (ou null se não puder andar). */
  onGround(x: number, z: number): { x: number; z: number } | null;
  /** Clique numa entidade (monstro/jogador). */
  onEntity(id: number): void;
  /** Se o cursor deve indicar "atacar" ao passar sobre a entidade. */
  isEnemy(id: number): boolean;
}

/**
 * Clique esquerdo: no monstro seleciona e ataca; no chão anda (segurar continua
 * andando em direção ao cursor, como no L2). Mostra um marcador no destino.
 */
export function setupInput(scene: Scene, ground: Mesh, h: InputHandlers): void {
  const canvas = scene.getEngine().getRenderingCanvas()!;
  const marker = createMarker(scene);
  let holdingGround = false;
  let lastSent = 0;
  let lastHover = 0;

  const pickEntity = (): number | null => {
    const pick = scene.pick(scene.pointerX, scene.pointerY, (m) => m.isPickable && m.metadata?.entityId !== undefined);
    return pick?.hit && pick.pickedMesh ? (pick.pickedMesh.metadata.entityId as number) : null;
  };

  const moveToCursor = (force: boolean) => {
    const now = performance.now();
    if (!force && now - lastSent < HOLD_RESEND_MS) return;
    const pick = scene.pick(scene.pointerX, scene.pointerY, (m) => m === ground);
    if (!pick?.hit || !pick.pickedPoint) return;
    lastSent = now;
    const dest = h.onGround(pick.pickedPoint.x, pick.pickedPoint.z);
    if (!dest) return;
    marker.position.set(dest.x, 0.03, dest.z);
    marker.setEnabled(true);
    marker.scaling.setAll(1);
    marker.visibility = 1;
  };

  scene.onPointerObservable.add((info) => {
    const ev = info.event as PointerEvent;
    switch (info.type) {
      case PointerEventTypes.POINTERDOWN: {
        if (ev.button !== 0) return;
        const id = pickEntity();
        if (id !== null) {
          h.onEntity(id);
          return;
        }
        holdingGround = true;
        moveToCursor(true);
        break;
      }
      case PointerEventTypes.POINTERMOVE: {
        if (holdingGround) moveToCursor(false);
        const now = performance.now();
        if (now - lastHover > HOVER_CHECK_MS) {
          lastHover = now;
          const id = pickEntity();
          canvas.style.cursor = id !== null && h.isEnemy(id) ? "crosshair" : id !== null ? "pointer" : "";
        }
        break;
      }
      case PointerEventTypes.POINTERUP:
        if (ev.button === 0) holdingGround = false;
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
