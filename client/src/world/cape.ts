import type { Node, Observer, Scene, TransformNode } from "@babylonjs/core";
import { Quaternion, Vector3 } from "@babylonjs/core";

/**
 * Mola da capa dos heróis v4 (contrato, seção 7): depois das animações de cada quadro, os 4 ossos de capa
 * (`cape_l_01/02`, `cape_r_01/02`) ganham uma inclinação que segue o movimento do personagem. Correndo, a
 * capa sobe para trás; ao parar, volta balançando; ao virar, abre para o lado de fora da curva. O giro em X
 * do osso levanta a capa para trás (positivo) ou a joga contra as pernas (negativo, quase não deixado).
 * Desligada de longe (LOD1 em diante). Devolve a função que desliga.
 */
export function attachCapeSpring(scene: Scene, root: TransformNode, byName: Map<string, Node>, far: () => boolean): () => void {
  const chains = (["l", "r"] as const)
    .map((s) => [byName.get(`cape_${s}_01`), byName.get(`cape_${s}_02`)] as (TransformNode | undefined)[])
    .filter((c): c is TransformNode[] => !!c[0] && !!c[1]);
  if (!chains.length) return () => {};
  const prev = root.getAbsolutePosition().clone();
  let prevYaw = yawOf(root);
  let pitch = 0, vPitch = 0, side = 0, vSide = 0, fresh = true;
  const tmp = new Quaternion();
  const obs: Observer<Scene> = scene.onAfterAnimationsObservable.add(() => {
    const dt = Math.min(0.05, Math.max(0.001, scene.getEngine().getDeltaTime() / 1000));
    const p = root.getAbsolutePosition();
    const yaw = yawOf(root);
    if (fresh) {
      prev.copyFrom(p);
      prevYaw = yaw;
      fresh = false;
    }
    const v = p.subtract(prev).scaleInPlace(1 / dt);
    prev.copyFrom(p);
    let dyaw = yaw - prevYaw;
    if (dyaw > Math.PI) dyaw -= 2 * Math.PI;
    if (dyaw < -Math.PI) dyaw += 2 * Math.PI;
    prevYaw = yaw;
    if (v.length() > 30) v.setAll(0); // teleporte: sem chicote
    const speed = Math.hypot(v.x, v.z); // o personagem sempre corre para onde olha
    const yawRate = Math.max(-6, Math.min(6, dyaw / dt));
    // alvos sutis: tecido pesado. Correndo a capa se afasta um pouco das pernas (até ~10°); virando, abre
    // um pouco para fora da curva. (Com 32° ela voava na horizontal e puxava o conjunto: ficou ridículo.)
    const pitchTarget = Math.min(0.18, Math.max(-0.04, speed * 0.036));
    const sideTarget = Math.max(-0.12, Math.min(0.12, -yawRate * 0.04));
    // mola amortecida (subamortecida: um balanço curto ao parar)
    vPitch += ((pitchTarget - pitch) * K - vPitch * C) * dt;
    pitch += vPitch * dt;
    vSide += ((sideTarget - side) * K - vSide * C) * dt;
    side += vSide * dt;
    if (far()) return;
    for (const [a, b] of chains) {
      // o alto da capa quase não mexe; a barra balança mais (com atraso natural da mola)
      addRot(a, pitch * 0.35, side * 0.4, tmp);
      addRot(b, pitch * 0.9, side, tmp);
    }
  });
  return () => obs.remove();
}

const K = 45; // rigidez (1/s²): mais mole, balanço mais lento
const C = 9; // amortecimento (1/s)

function addRot(n: TransformNode, pitch: number, side: number, tmp: Quaternion): void {
  if (!n.rotationQuaternion) return;
  Quaternion.RotationYawPitchRollToRef(0, pitch, side, tmp); // X = levantar para trás, Z = abrir para o lado
  n.rotationQuaternion.multiplyInPlace(tmp);
}

function yawOf(n: TransformNode): number {
  const f = n.getDirection(Vector3.Forward());
  return Math.atan2(f.x, f.z);
}
