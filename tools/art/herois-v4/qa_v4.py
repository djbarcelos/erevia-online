"""Heróis v4: portão de qualidade automático (plano, seção 7) sobre o base.blend antes de exportar.

Confere o que já deu problema e o usuário teve de apontar:
- clipes: todos os que o jogo pede existem;
- laço: parado, andar, correr e guarda fecham sem salto (1º e último quadro iguais);
- punho: as duas mãos fechadas em todos os quadros (a mão abria quando os dedos não exportavam);
- escudo: o pulso esquerdo fica preso ao antebraço (lock_wrist.py), fora dos trechos livres;
- golpes: têm quadro de impacto; locomoção tem velocidade do pé;
- malha: triângulos por LOD no orçamento e mãos com detalhe (o punho virava pontas).
Sai com a lista do que falhou ("QA FALHOU ...") ou "QA OK".

    Blender --background --factory-startup base.blend --python tools/art/herois-v4/qa_v4.py -- clips-base.json clips-kn.json
"""
import json
import math
import sys

import bpy
from mathutils import Quaternion

args = sys.argv[sys.argv.index("--") + 1:]
clips = {}
for p in args:
    clips.update(json.load(open(p)))
arm = bpy.data.objects["heroi"]
pb = arm.pose.bones
sc = bpy.context.scene
fails = []
REQUIRED = ["rc.idle", "base.walk", "base.run", "base.hit", "base.hit2", "base.death", "base.death2",
            "kn.attack.sword.1", "kn.attack.sword.2", "kn.attack.sword.3", "kn.skill.heavy", "kn.stance", "kn.block"]
LOOPS = ["rc.idle", "base.walk", "base.run", "kn.stance"]
ATTACKS = ["kn.attack.sword.1", "kn.attack.sword.2", "kn.attack.sword.3", "kn.skill.heavy"]
FREE = {"kn.attack.sword.3": (16, 44)}  # trecho em que o pulso do escudo é livre (com a transição)


def use(act):
    arm.animation_data_create()
    arm.animation_data.action = act
    try:
        arm.animation_data.action_slot = arm.animation_data.action_suitable_slots[0]
    except (AttributeError, IndexError):
        pass


def pose_rots():
    return {b.name: b.matrix.to_quaternion() for b in pb if b.bone.use_deform}


def curl(s):
    h = (pb[f"hand_{s}"].tail - pb[f"hand_{s}"].head).normalized()
    f = (pb[f"fingers_02_{s}"].tail - pb[f"fingers_02_{s}"].head).normalized()
    return math.degrees(h.angle(f))


def ang(q1, q2):
    """Ângulo entre duas rotações (graus); q e -q são a mesma rotação."""
    a = q1.rotation_difference(q2).angle
    return math.degrees(min(a, 2 * math.pi - a))


def wrist_rel():
    return pb["forearm_l"].matrix.to_quaternion().inverted() @ pb["hand_l"].matrix.to_quaternion()


acts = {a.name: a for a in bpy.data.actions if a.use_fake_user}
for n in REQUIRED:
    if n not in acts:
        fails.append(f"clipe faltando: {n}")

use(acts["rc.idle"])
sc.frame_set(0)
REF_WRIST = wrist_rel()

for name, act in sorted(acts.items()):
    if name not in REQUIRED:
        continue
    use(act)
    f0, f1 = (int(round(x)) for x in act.frame_range)
    worst_curl, worst_wrist = 999.0, 0.0
    for f in range(f0, f1 + 1, 2):
        sc.frame_set(f)
        worst_curl = min(worst_curl, curl("l"), curl("r"))
        a, b = FREE.get(name, (10**6, -1))
        if not (a <= f <= b):
            worst_wrist = max(worst_wrist, ang(wrist_rel(), REF_WRIST))
    if worst_curl < 120:
        fails.append(f"{name}: mão aberta (dedo a {worst_curl:.0f}° do eixo da mão, mínimo 120°)")
    if worst_wrist > 6:
        fails.append(f"{name}: pulso do escudo solto ({worst_wrist:.0f}° fora da trava)")
    if name in LOOPS:
        sc.frame_set(f0)
        r0 = pose_rots()
        sc.frame_set(f1)
        r1 = pose_rots()
        jump = max(ang(r0[k], r1[k]) for k in r0)
        if jump > 3:
            fails.append(f"{name}: laço não fecha (salto de {jump:.1f}°)")
    info = clips.get(name, {})
    if name in ATTACKS and not info.get("impact"):
        fails.append(f"{name}: sem quadro de impacto")
    if name in ("base.walk", "base.run") and not info.get("footSpeed"):
        fails.append(f"{name}: sem velocidade do pé")
    print(f"QA {name}: punho {worst_curl:.0f}°, pulso {worst_wrist:.1f}°")

BUDGET = {"corpo": 8000, "corpo_LOD1": 3000, "corpo_LOD2": 1000}
HANDS = ("hand_", "thumb_", "index_", "fingers_")
for o in bpy.data.objects:
    if o.type != "MESH" or o.name not in BUDGET:
        continue
    tris = sum(len(p.vertices) - 2 for p in o.data.polygons)
    if tris > BUDGET[o.name]:
        fails.append(f"{o.name}: {tris} triângulos (máximo {BUDGET[o.name]})")
    if o.name == "corpo":
        names = {g.index: g.name for g in o.vertex_groups}
        hw = [sum(g.weight for g in v.groups if names[g.group].startswith(HANDS)) for v in o.data.vertices]
        ht = sum(len(p.vertices) - 2 for p in o.data.polygons if all(hw[i] > 0.5 for i in p.vertices))
        print(f"QA malha: {tris} triângulos, mãos {ht}")
        if ht < 1400:
            fails.append(f"corpo: mãos com {ht} triângulos (mínimo 1400: o punho vira pontas)")

if fails:
    for f in fails:
        print("QA FALHOU", f)
else:
    print("QA OK")
