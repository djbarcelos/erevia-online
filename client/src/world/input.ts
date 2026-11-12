import { Color3, Mesh, MeshBuilder, PointerEventTypes, Scene, StandardMaterial } from "@babylonjs/core";

/** Segurando o botão, reenvia o destino no máximo a cada X ms (servidor aceita ~10/s). */
const HOLD_RESEND_MS = 150;
const HOVER_CHECK_MS = 80;
/** Ao sair de cima do inimigo, espera um pouco antes de voltar o cursor (sem piscar na borda). */
const HOVER_GRACE_MS = 150;

/** Cursor de ataque: espadinha na diagonal, com a ponta (ponto do clique) no canto superior esquerdo. */
const SWORD_SVG = `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 32 32">
<g stroke-linecap="round" stroke-linejoin="round">
<path d="M3 3 L7 4 L21 18 L18 21 L4 7 Z" fill="#eef2f8" stroke="#1a1410" stroke-width="1.6"/>
<path d="M5 5 L19.5 19.5" stroke="#9aa6b8" stroke-width="1"/>
<path d="M15 24 L24 15" stroke="#1a1410" stroke-width="5"/>
<path d="M15 24 L24 15" stroke="#d6b46e" stroke-width="2.6"/>
<path d="M20.5 20.5 L26 26" stroke="#1a1410" stroke-width="4.6"/>
<path d="M20.5 20.5 L26 26" stroke="#7a4a2a" stroke-width="2.4"/>
<circle cx="27" cy="27" r="2.6" fill="#d6b46e" stroke="#1a1410" stroke-width="1.4"/>
</g></svg>`;
const SWORD_CURSOR = `url("data:image/svg+xml,${encodeURIComponent(SWORD_SVG)}") 3 3, crosshair`;

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
  let lastEnemyAt = 0;
  let cursor = "";
  // O cursor é só nosso: o Babylon voltava ao padrão a cada movimento do mouse (a mira piscava).
  scene.doNotHandleCursors = true;

  const setCursor = (c: string) => {
    if (c === cursor) return;
    cursor = c;
    canvas.style.cursor = c;
  };

  /** Atualiza o cursor conforme o que está sob o mouse (também com o mouse parado). */
  const updateHover = () => {
    const now = performance.now();
    if (now - lastHover < HOVER_CHECK_MS) return;
    lastHover = now;
    const id = pickEntity();
    if (id !== null && h.isEnemy(id)) {
      lastEnemyAt = now;
      setCursor(SWORD_CURSOR);
    } else if (now - lastEnemyAt > HOVER_GRACE_MS) {
      setCursor(id !== null ? "pointer" : "");
    }
  };

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
        updateHover();
        break;
      }
      case PointerEventTypes.POINTERUP:
        if (ev.button === 0) holdingGround = false;
        break;
    }
  });

  // Marcador encolhe e some; o cursor acompanha monstros andando sob o mouse parado.
  scene.onBeforeRenderObservable.add(() => {
    updateHover();
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
