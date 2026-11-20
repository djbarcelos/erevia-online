"""Heróis v4: pulso esquerdo travado (mão do escudo) em todas as animações.

Nas animações capturadas a mão esquerda é uma mão livre: o pulso dobra e o antebraço gira (gestos, dano,
o fim do empurrão com escudo). Com o escudo preso na mão, ele deitava e atravessava o braço. Aqui, em cada
quadro de cada ação, a mão esquerda fica com a rotação que tem no quadro de referência (o parado, onde o
encaixe do escudo foi ajustado) e o antebraço perde o giro no próprio eixo (fica o da referência); a dobra do
cotovelo e o resto do braço continuam como na animação.

    Blender --background --factory-startup base.blend --python tools/art/herois-v4/lock_wrist.py -- [ação_ref quadro_ref]
"""
import sys

import bpy
from mathutils import Quaternion

args = sys.argv[sys.argv.index("--") + 1:] if "--" in sys.argv else []
ref_act = args[0] if args else "rc.idle"
ref_frame = int(args[1]) if len(args) > 1 else 0
arm = bpy.data.objects["heroi"]
pb = arm.pose.bones
sc = bpy.context.scene
FA, HA = "forearm_l", "hand_l"


def use(act):
    arm.animation_data_create()
    arm.animation_data.action = act
    try:
        arm.animation_data.action_slot = arm.animation_data.action_suitable_slots[0]
    except (AttributeError, IndexError):
        pass


def twist_y(q):
    """Parte do giro em torno do eixo Y do osso (o eixo do antebraço)."""
    t = Quaternion((q.w, 0.0, q.y, 0.0))
    return t.normalized() if t.magnitude > 1e-8 else Quaternion()


use(bpy.data.actions[ref_act])
sc.frame_set(ref_frame)
REF_HAND = pb[HA].rotation_quaternion.copy()
REF_TWIST = twist_y(pb[FA].rotation_quaternion.copy())
print("referência:", ref_act, ref_frame, [round(x, 3) for x in REF_HAND])

# Trechos em que a animação manda no pulso (o escudo é a arma): empurrão do golpe 3. Fora deles a trava volta
# aos poucos (RAMP quadros), sem estalo.
FREE = {"kn.attack.sword.3": (22, 38)}
RAMP = 6


def lock_weight(name, f):
    if name not in FREE:
        return 1.0
    a, b = FREE[name]
    d = max(a - f, f - b, 0)  # distância até o trecho livre (0 dentro dele)
    u = min(1.0, d / RAMP)
    return u * u * (3 - 2 * u)


for act in [a for a in bpy.data.actions if a.use_fake_user]:
    use(act)
    f0, f1 = (int(round(x)) for x in act.frame_range)
    for f in range(f0, f1 + 1):
        sc.frame_set(f)
        w = lock_weight(act.name, f)
        q = pb[FA].rotation_quaternion.copy()
        h = pb[HA].rotation_quaternion.copy()
        swing = q @ twist_y(q).inverted()
        pb[FA].rotation_quaternion = q.slerp(swing @ REF_TWIST, w)
        pb[HA].rotation_quaternion = h.slerp(REF_HAND, w)
        pb[FA].keyframe_insert("rotation_quaternion", frame=f)
        pb[HA].keyframe_insert("rotation_quaternion", frame=f)
    print("PULSO", act.name, f1 - f0 + 1, "quadros")
arm.animation_data.action = None
for p in pb:
    p.matrix_basis.identity()
bpy.ops.wm.save_as_mainfile(filepath=bpy.data.filepath, compress=True)
print("SALVO", bpy.data.filepath)
