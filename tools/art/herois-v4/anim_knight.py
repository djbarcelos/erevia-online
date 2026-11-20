"""Heróis v4: camada do Cavaleiro (docs/arte/herois-v4/movimento.md, seção 4), animada no Blender.

Cada pose diz onde fica cada pulso e para onde apontam a lâmina e o escudo; o braço chega lá por IK
(cotovelo com polo), e a mão copia a rotação do alvo. A direção da lâmina é a linha dos nós do punho
fechado (do mínimo ao indicador), a mesma que o jogo usa para encaixar a espada (HERO_FISTS_V4); o escudo
olha para onde aponta a linha dos nós da mão esquerda (k). Pés presos no chão por IK, quadril e
coluna por rotação em eixos do mundo (+X esquerda do personagem, -Y frente, +Z cima). No fim tudo é
convertido (bake) para os ossos do contrato, os controles saem, e o .blend fica com os clipes novos.

Clipes: kn.idle (120, laço), kn.stance (60, laço), kn.attack.sword.1 (30, impacto 13),
kn.attack.sword.2 (30, impacto 12), kn.attack.sword.3 (36, impacto 16), kn.block (20).

    Blender --background --factory-startup base.blend --python tools/art/herois-v4/anim_knight.py -- clips-kn.json
"""
import json
import math
import os
import sys

import bpy
from mathutils import Euler, Matrix, Quaternion, Vector

sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from anim_util import FIST  # noqa: E402

out_json = sys.argv[sys.argv.index("--") + 1]
FPS = 30
sc = bpy.context.scene
sc.render.fps = FPS
arm = bpy.data.objects["heroi"]
arm.hide_set(False)
arm.hide_viewport = False
B = arm.data.bones
H = {b.name: b.head_local.copy() for b in B}
T = {b.name: b.tail_local.copy() for b in B}
REST = {b.name: b.matrix_local.copy() for b in B}
UP = Vector((0, 0, 1))
SH = {s: H[f"upperarm_{s}"] for s in ("l", "r")}


def R(out, fwd, up):
    """Ponto em relação ao ombro direito: para fora, para a frente, para cima (m)."""
    return SH["r"] + Vector((-out, -fwd, up))


def L(out, fwd, up):
    return SH["l"] + Vector((out, -fwd, up))


def dR(out, fwd, up):
    return Vector((-out, -fwd, up)).normalized()


def dL(out, fwd, up):
    return Vector((out, -fwd, up)).normalized()


# ---------- rig de controle ----------
bpy.ops.object.select_all(action="DESELECT")
arm.select_set(True)
bpy.context.view_layer.objects.active = arm
bpy.ops.object.mode_set(mode="EDIT")
eb = arm.data.edit_bones


def ctrl(name, head, tail, roll=0.0):
    b = eb.new(name)
    b.head, b.tail = head, tail
    b.roll = roll
    b.use_deform = False
    b.parent = eb["root"]
    return b


for s in ("l", "r"):
    ctrl(f"ik_foot_{s}", H[f"foot_{s}"], T[f"foot_{s}"], eb[f"foot_{s}"].roll)
    knee = H[f"calf_{s}"] + Vector((0, -0.5, 0))
    ctrl(f"pole_knee_{s}", knee, knee + UP * 0.05)
    ctrl(f"ik_hand_{s}", H[f"hand_{s}"], T[f"hand_{s}"], eb[f"hand_{s}"].roll)
    elbow = H[f"forearm_{s}"] + Vector((0, 0.45, 0))
    ctrl(f"pole_elbow_{s}", elbow, elbow + UP * 0.05)
bpy.ops.object.mode_set(mode="POSE")
pb = arm.pose.bones
CTRL_REST = {b.name: b.matrix_local.copy() for b in arm.data.bones}
for s in ("l", "r"):
    c = pb[f"calf_{s}"].constraints.new("IK")
    c.target, c.subtarget = arm, f"ik_foot_{s}"
    c.pole_target, c.pole_subtarget = arm, f"pole_knee_{s}"
    c.chain_count = 2
    cr = pb[f"foot_{s}"].constraints.new("COPY_ROTATION")
    cr.target, cr.subtarget = arm, f"ik_foot_{s}"
    ca = pb[f"forearm_{s}"].constraints.new("IK")
    ca.target, ca.subtarget = arm, f"ik_hand_{s}"
    ca.pole_target, ca.pole_subtarget = arm, f"pole_elbow_{s}"
    ca.chain_count = 2
    ch = pb[f"hand_{s}"].constraints.new("COPY_ROTATION")
    ch.target, ch.subtarget = arm, f"ik_hand_{s}"


def pick_pole(con, bones):
    best = None
    for deg in (-90, 0, 90, 180):
        con.pole_angle = math.radians(deg)
        bpy.context.view_layer.update()
        err = sum(pb[b].matrix.to_quaternion().rotation_difference(REST[b].to_quaternion()).angle for b in bones)
        if best is None or err < best[0]:
            best = (err, deg)
    con.pole_angle = math.radians(best[1])


for p in pb:
    p.rotation_mode = "QUATERNION"
for s in ("l", "r"):
    pick_pole(pb[f"calf_{s}"].constraints["IK"], [f"thigh_{s}", f"calf_{s}"])
    pick_pole(pb[f"forearm_{s}"].constraints["IK"], [f"upperarm_{s}", f"forearm_{s}"])

# punho fechado (dedos em FK, iguais em todos os quadros)
FISTQ = {}
for s in ("l", "r"):
    for f, angs in FIST.items():
        for k, ang in enumerate(angs, 1):
            FISTQ[f"{f}_{k:02d}_{s}"] = Euler((math.radians(ang), 0, 0)).to_quaternion()


def set_fist():
    for n, q in FISTQ.items():
        pb[n].rotation_quaternion = q
    bpy.context.view_layer.update()


# referencial da mão em repouso, com o punho fechado: k = linha dos nós (lâmina), y = eixo da mão
set_fist()
HAND0 = {}
for s in ("l", "r"):
    k = (pb[f"index_01_{s}"].head - pb[f"fingers_01_{s}"].head).normalized()
    y = (T[f"hand_{s}"] - H[f"hand_{s}"]).normalized()
    HAND0[s] = (k, y, REST[f"hand_{s}"].to_3x3().normalized())


def frame_of(k, y):
    k = k.normalized()
    y = (y - k * y.dot(k)).normalized()
    return Matrix((k, y, k.cross(y))).transposed()


def hand_rot(s, k, y):
    k0, y0, r0 = HAND0[s]
    Q = frame_of(k, y) @ frame_of(k0, y0).inverted()
    return Q @ r0


def world_rot(spec):
    """[("x", graus), ("z", graus), ...] -> matriz 3x3 (aplicadas na ordem)."""
    m = Matrix.Identity(3)
    for ax, deg in spec:
        m = Matrix.Rotation(math.radians(deg), 3, ax.upper()) @ m
    return m


def set_world_rot(name, spec):
    """Rotação em eixos do mundo, a partir do repouso do osso (gira junto com o pai)."""
    r = REST[name].to_3x3().normalized()
    local = r.inverted() @ world_rot(spec) @ r
    pb[name].rotation_quaternion = local.to_quaternion()


def place(name, pos, rot3=None):
    """Controle (filho do root, em repouso no lugar do osso) num ponto e rotação do armature."""
    rest = CTRL_REST[name]
    rot = rot3 if rot3 is not None else rest.to_3x3().normalized()
    want = Matrix.Translation(pos) @ rot.to_4x4()
    m = rest.inverted() @ want
    loc, q, _ = m.decompose()
    pb[name].location = loc
    pb[name].rotation_quaternion = q


TORSO = ("pelvis", "spine_01", "spine_02", "chest", "neck", "head")
DEFAULT_FEET = {"l": (Vector((0.02, -0.07, 0)), 0.0), "r": (Vector((-0.02, 0.08, 0)), 8.0)}


def pose(p):
    """Aplica uma pose (dict) aos controles e ao tronco."""
    for n in TORSO:
        set_world_rot(n, p.get(n, []))
    hip = p.get("hip", Vector((0, 0, -0.035)))
    pb["pelvis"].location = REST["pelvis"].to_3x3().inverted() @ hip
    for s in ("l", "r"):
        off, yaw = p.get(f"foot_{s}", DEFAULT_FEET[s])
        fr = world_rot([("z", yaw)]) @ REST[f"foot_{s}"].to_3x3().normalized()
        place(f"ik_foot_{s}", H[f"foot_{s}"] + off, fr)
        pos, k, y = p[f"hand_{s}"]
        place(f"ik_hand_{s}", pos, hand_rot(s, k, y))
        pole = p.get(f"elbow_{s}")
        if pole is None:
            pole = (R(0.35, -0.45, -0.25) if s == "r" else L(0.35, -0.45, -0.25))
        place(f"pole_elbow_{s}", pole)
    set_fist()


KEYED = list(TORSO) + [f"{c}_{s}" for c in ("ik_foot", "ik_hand", "pole_elbow") for s in ("l", "r")] + list(FISTQ)


def key_all(f):
    for n in KEYED:
        pb[n].keyframe_insert("rotation_quaternion", frame=f)
        if n.startswith(("ik_", "pole_")) or n == "pelvis":
            pb[n].keyframe_insert("location", frame=f)


def lerp_pose(a, b, t):
    """Mistura de duas poses (para respiração e variações): pontos e direções interpolados."""
    out = {}
    for key in set(a) | set(b):
        va, vb = a.get(key), b.get(key)
        if va is None or vb is None:
            out[key] = va if vb is None else vb
        elif key.startswith("hand_"):
            out[key] = tuple(x.lerp(y, t) for x, y in zip(va, vb))
        elif key.startswith("foot_"):
            out[key] = (va[0].lerp(vb[0], t), va[1] + (vb[1] - va[1]) * t)
        elif key.startswith("elbow_") or key == "hip":
            out[key] = va.lerp(vb, t)
        else:
            da, db = dict(va), dict(vb)
            out[key] = [(ax, da.get(ax, 0) + (db.get(ax, 0) - da.get(ax, 0)) * t) for ax in sorted(set(da) | set(db))]
    return out


def mod(p, **kw):
    q = dict(p)
    q.update(kw)
    return q


# ---------- poses do Cavaleiro ----------
# Postura: tronco ereto, quadril baixo, joelhos dobrados; escudo diante do peito, espada baixa ao lado.
POSTURE = {
    "hip": Vector((0, 0.01, -0.045)),
    "spine_01": [("x", -2)], "spine_02": [("x", -1)], "chest": [("x", 1)], "head": [("x", 2)],
    "hand_r": (R(0.07, 0.13, -0.40), dR(0.12, 1, -0.5), dR(0.1, 0.25, -1)),
    "hand_l": (L(-0.20, 0.30, -0.17), dL(0.05, 1, 0.05), dL(-1, 0.1, -0.1)),
    "elbow_l": L(0.55, -0.05, -0.3),
    "elbow_r": R(0.3, -0.5, -0.2),
}
# Guarda alta: escudo na altura do queixo, espada por cima apontando para a frente.
STANCE = mod(POSTURE,
             hip=Vector((0, 0.02, -0.06)),
             spine_02=[("x", 3)], chest=[("x", 2)], head=[("x", -2)],
             hand_l=(L(-0.10, 0.30, -0.06), dL(0.05, 1, 0.1), dL(-1, 0.1, 0.05)),
             hand_r=(R(0.02, 0.22, -0.02), dR(-0.15, 1, 0.25), dR(-0.6, 0.1, 0.8)),
             elbow_r=R(0.45, -0.2, -0.45), elbow_l=L(0.55, -0.05, -0.35),
             foot_l=(Vector((0.03, -0.10, 0)), -5.0), foot_r=(Vector((-0.03, 0.12, 0)), 12.0))

# Golpe 1: corte diagonal de cima para baixo; o escudo recua para o quadril.
A1_UP = mod(STANCE,
            spine_02=[("x", 0), ("z", -10)], chest=[("z", -8)], head=[("z", 10)],
            hand_r=(R(0.02, 0.02, 0.30), dR(0.15, -0.45, 1), dR(0.3, 0.8, 0.4)),
            elbow_r=R(0.5, 0.0, 0.1),
            hand_l=(L(-0.08, 0.28, -0.12), dL(0.05, 1, 0.05), dL(-1, 0.1, 0)))
A1_HIT = mod(STANCE,
             hip=Vector((0, -0.03, -0.08)),
             spine_01=[("x", 4)], spine_02=[("x", 8), ("z", 12)], chest=[("x", 4), ("z", 8)], head=[("z", -12), ("x", -6)],
             hand_r=(R(-0.30, 0.42, -0.36), dR(-0.55, 0.7, -0.5), dR(-0.6, 0.4, -0.6)),
             elbow_r=R(0.4, -0.2, -0.3),
             hand_l=(L(0.04, 0.10, -0.40), dL(0.5, 1, 0), dL(0, 0.2, -1)),
             elbow_l=L(0.4, -0.3, -0.2))
A1_FOLLOW = mod(A1_HIT, hand_r=(R(-0.26, 0.36, -0.42), dR(-0.6, 0.55, -0.6), dR(-0.6, 0.3, -0.7)))

# Golpe 2: corte horizontal da esquerda para a direita, passo à frente.
A2_WIND = mod(STANCE,
              spine_02=[("z", 14)], chest=[("z", 10)], head=[("z", -14)],
              hand_r=(R(-0.36, 0.26, -0.06), dR(-1, 0.25, 0.1), dR(-0.2, 0.2, -1)),
              elbow_r=R(0.2, -0.3, -0.4),
              hand_l=(L(0.0, 0.18, -0.20), dL(0.2, 1, 0.1), dL(-0.8, 0.2, -0.3)))
A2_STEP = mod(A2_WIND, foot_r=(Vector((-0.03, -0.05, 0.08)), 0.0))
A2_HIT = mod(STANCE,
             hip=Vector((0, -0.12, -0.08)),
             spine_01=[("x", 3)], spine_02=[("x", 5), ("z", -14)], chest=[("z", -10)], head=[("z", 12)],
             hand_r=(R(0.22, 0.50, -0.08), dR(0.65, 0.75, 0.05), dR(0.1, 0.3, -1)),
             elbow_r=R(0.5, -0.1, -0.35),
             hand_l=(L(0.02, 0.14, -0.26), dL(0.3, 1, 0.1), dL(-0.6, 0.3, -0.6)),
             foot_r=(Vector((-0.03, -0.20, 0)), 4.0), foot_l=(Vector((0.03, -0.06, 0)), -5.0))
A2_FOLLOW = mod(A2_HIT, hand_r=(R(0.36, 0.34, -0.10), dR(1, 0.3, 0), dR(0.2, 0.2, -1)))

# Golpe 3: pancada de escudo + estocada curta (fecha o combo).
A3_COCK = mod(STANCE,
              spine_02=[("z", -8)], chest=[("z", -6)],
              hand_l=(L(0.06, 0.10, -0.12), dL(0.1, 1, 0.1), dL(-0.7, 0.4, -0.2)),
              hand_r=(R(0.10, 0.02, -0.12), dR(-0.1, 1, 0.05), dR(-0.5, 0.2, -0.8)))
A3_BASH = mod(A3_COCK,
              hip=Vector((0, -0.10, -0.07)),
              spine_01=[("x", 4)], spine_02=[("x", 6), ("z", 16)], chest=[("z", 10)], head=[("z", -14)],
              hand_l=(L(-0.14, 0.56, -0.08), dL(0, 1, 0.05), dL(-1, 0.25, 0)),
              elbow_l=L(0.45, -0.1, -0.1),
              foot_l=(Vector((0.04, -0.22, 0)), -5.0))
A3_THRUST = mod(A3_BASH,
                spine_02=[("x", 6), ("z", -12)], chest=[("z", -8)], head=[("z", 8)],
                hand_r=(R(-0.06, 0.60, -0.12), dR(-0.12, 1, 0.02), dR(-0.4, 0.4, -0.8)),
                elbow_r=R(0.4, -0.3, -0.4),
                hand_l=(L(0.10, 0.28, -0.22), dL(0.3, 1, 0.1), dL(-0.8, 0.3, -0.3)))

# Bloqueio: escudo para cima, corpo se fecha atrás dele, recua meio passo.
BLOCK = mod(STANCE,
            hip=Vector((0, 0.08, -0.10)),
            spine_01=[("x", 6)], spine_02=[("x", 8), ("z", 6)], chest=[("x", 4)], head=[("x", -4)],
            hand_l=(L(-0.10, 0.38, 0.04), dL(0, 1, 0.2), dL(-1, 0.05, 0.1)),
            hand_r=(R(0.0, 0.14, -0.12), dR(-0.2, 1, 0.3), dR(-0.6, 0.1, 0.7)),
            foot_r=(Vector((-0.03, 0.22, 0)), 14.0), foot_l=(Vector((0.03, 0.0, 0)), -5.0))


def make(name, keys, loop=False, ease=None):
    """keys: [(quadro, pose)] — chaves com interpolação de Bézier (o Blender suaviza)."""
    for p in pb:
        p.location = (0, 0, 0)
        p.rotation_quaternion = (1, 0, 0, 0)
    arm.animation_data_create()
    act = bpy.data.actions.new(name)
    arm.animation_data.action = act
    for f, p in keys:
        pose(p)
        key_all(f)
    return act


def breathe(base, frames, amp=1.5, shift=0.012):
    """Parado vivo: respiração (4 por laço) e troca de peso lenta, por cima de uma pose."""
    keys = []
    n = 12
    for i in range(n + 1):
        f = round(frames * i / n)
        ph = i / n
        br = math.sin(2 * math.pi * ph * 4)
        sw = math.sin(2 * math.pi * ph)
        p = dict(base)
        p["spine_02"] = base.get("spine_02", []) + [("x", -amp * br)]
        p["chest"] = base.get("chest", []) + [("x", -amp * 0.6 * br)]
        p["hip"] = base.get("hip", Vector()) + Vector((shift * sw, 0, -0.004 * br))
        p["pelvis"] = [("y", 1.5 * sw)]
        keys.append((f, p))
    return keys


def strap(keys, frames):
    """kn.idle: no meio do laço, o braço do escudo sobe e acomoda a alça."""
    out = []
    for f, p in keys:
        t = f / frames
        if 0.45 < t < 0.75:
            u = math.sin(math.pi * (t - 0.45) / 0.30)
            hl = p["hand_l"]
            p = dict(p)
            p["hand_l"] = (hl[0] + Vector((0.02, -0.03, 0.06)) * u, (hl[1] + Vector((0, 0, 0.35)) * u).normalized(), hl[2])
            p["head"] = p.get("head", []) + [("z", -10 * u), ("x", 6 * u)]
        out.append((f, p))
    return out


CLIPS = {}
specs = [
    ("kn.idle", 120, True, [], lambda: strap(breathe(POSTURE, 120), 120)),
    ("kn.stance", 60, True, [], lambda: breathe(STANCE, 60, amp=1.0, shift=0.006)),
    ("kn.attack.sword.1", 30, False, [13], lambda: [(0, STANCE), (7, A1_UP), (10, lerp_pose(A1_UP, A1_HIT, 0.55)), (13, A1_HIT), (18, A1_FOLLOW), (30, STANCE)]),
    ("kn.attack.sword.2", 30, False, [12], lambda: [(0, STANCE), (5, A2_WIND), (8, A2_STEP), (12, A2_HIT), (17, A2_FOLLOW), (30, STANCE)]),
    ("kn.attack.sword.3", 36, False, [16], lambda: [(0, STANCE), (5, A3_COCK), (9, A3_BASH), (12, lerp_pose(A3_BASH, A3_THRUST, 0.3)), (16, A3_THRUST), (22, A3_THRUST), (36, STANCE)]),
    ("kn.block", 20, False, [6], lambda: [(0, STANCE), (5, BLOCK), (13, BLOCK), (20, STANCE)]),
]

# ---------- bake para os ossos do contrato ----------


def fcurves_of(act):
    out = []
    if hasattr(act, "layers") and act.layers:
        for layer in act.layers:
            for strip in layer.strips:
                for cb in strip.channelbags:
                    out += [(cb.fcurves, fc) for fc in list(cb.fcurves)]
    elif hasattr(act, "fcurves"):
        out = [(act.fcurves, fc) for fc in list(act.fcurves)]
    return out


for name, frames, loop, impacts, fn in specs:
    old = bpy.data.actions.get(name)
    if old:
        bpy.data.actions.remove(old)
    act = make(name, fn(), loop)
    sc.frame_start, sc.frame_end = 0, frames
    bpy.ops.pose.select_all(action="DESELECT")
    for b in arm.data.bones:
        want = b.use_deform or b.name == "root"
        p = arm.pose.bones[b.name]
        if hasattr(p, "select"):
            p.select = want
        else:
            b.select = want
    bpy.ops.nla.bake(frame_start=0, frame_end=frames, only_selected=True, visual_keying=True,
                     clear_constraints=False, use_current_action=False, bake_types={"POSE"})
    baked = arm.animation_data.action
    bpy.data.actions.remove(act)
    baked.name = name
    for coll, fc in fcurves_of(baked):
        if any(k in fc.data_path for k in ("ik_", "pole_")):
            coll.remove(fc)
    baked.use_fake_user = True
    CLIPS[name] = {"frames": frames, "loop": loop, "impact": impacts, "footSpeed": None, "capeSpring": 1}
    print("CLIPE", name, CLIPS[name])

# tira o rig de controle
for p in pb:
    for c in list(p.constraints):
        p.constraints.remove(c)
arm.animation_data.action = None
bpy.ops.object.mode_set(mode="EDIT")
for n in [b.name for b in arm.data.edit_bones if b.name.startswith(("ik_", "pole_"))]:
    arm.data.edit_bones.remove(arm.data.edit_bones[n])
bpy.ops.object.mode_set(mode="OBJECT")
for p in arm.pose.bones:
    p.matrix_basis = Matrix.Identity(4)
json.dump(CLIPS, open(out_json, "w"), indent=1)
bpy.ops.wm.save_as_mainfile(filepath=bpy.data.filepath, compress=True)
print("SALVO", bpy.data.filepath, out_json)
