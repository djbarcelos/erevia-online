"""Heróis v4: transfere animações da biblioteca do Meshy (esqueleto mixamo) para o esqueleto do contrato.

O Meshy anima o próprio rig do corpo v4 (task de rigging `hv4-rig-m-2k`); aqui cada osso do contrato copia a
orientação do osso mixamo correspondente no mundo. A diferença de repouso (os braços que o fix_arms.py girou
e o roll de cada osso) sai de um desvio constante por osso, medido no repouso: o osso do contrato aponta para
onde o osso da fonte aponta, e o giro em torno do próprio eixo vem da fonte. Por cima entram o punho fechado
na arma e o giro da palma (anim_util.py). O quadril perde o avanço (clipe no lugar, sem root motion) e fica
com o sobe e desce. O último quadro repete o primeiro (laço sem tranco). Substitui só os clipes pedidos.

    Blender --background --factory-startup base.blend --python tools/art/herois-v4/retarget_meshy.py -- \
        merged_animations.glb clips-base.json base.run=Run_02 [base.walk=walking_2 ...] [--no-twist]
"""
import json
import os
import sys

import bpy
from mathutils import Matrix

sys.path.append(os.path.dirname(os.path.abspath(__file__)))
from anim_util import fist_basis, foot_speed, palm_twist, rot_y  # noqa: E402

args = sys.argv[sys.argv.index("--") + 1:]
src_glb, clips_json = args[0], args[1]
pairs = [a.split("=", 1) for a in args[2:] if "=" in a]
TWIST_ON = "--no-twist" not in args
FPS = 30
P = "mixamorig:"

sc = bpy.context.scene
sc.render.fps = FPS  # o importador converte o tempo do glTF em quadros pela taxa da cena
arm = bpy.data.objects["heroi"]
before = set(bpy.data.objects)
bpy.ops.import_scene.gltf(filepath=src_glb)
new = [o for o in bpy.data.objects if o not in before]
src = next(o for o in new if o.type == "ARMATURE")

RENAME = {"Hips": "pelvis", "Spine": "spine_01", "Spine1": "spine_02", "Spine2": "chest", "Neck": "neck", "Head": "head"}
for s, side in (("l", "Left"), ("r", "Right")):
    RENAME.update({f"{side}Shoulder": f"clavicle_{s}", f"{side}Arm": f"upperarm_{s}", f"{side}ForeArm": f"forearm_{s}",
                   f"{side}Hand": f"hand_{s}", f"{side}UpLeg": f"thigh_{s}", f"{side}Leg": f"calf_{s}",
                   f"{side}Foot": f"foot_{s}", f"{side}ToeBase": f"toe_{s}"})
MAP = {t: P + s for s, t in RENAME.items() if P + s in src.data.bones}
print("ossos transferidos:", len(MAP))


def depth(b):
    d = 0
    while b.parent:
        b, d = b.parent, d + 1
    return d


TB = sorted(arm.data.bones, key=depth)
R = {b.name: b.matrix_local.copy() for b in TB}
REL = {b.name: (R[b.parent.name].inverted() @ R[b.name]) if b.parent else R[b.name] for b in TB}
smw = src.matrix_world.copy()

# desvio constante por osso: L = (Q · S0)⁻¹ · T0, com Q levando a direção do osso da fonte à do alvo
L = {}
for t, s in MAP.items():
    S0 = (smw @ src.data.bones[s].matrix_local).to_3x3().normalized()
    T0 = R[t].to_3x3().normalized()
    Q = S0.col[1].rotation_difference(T0.col[1]).to_matrix()
    L[t] = (Q @ S0).inverted() @ T0
hips_rest = smw @ src.data.bones[P + "Hips"].head_local
k_hip = R["pelvis"].to_translation().z / max(1e-6, hips_rest.z)
TW = {s: (palm_twist(arm, s) if TWIST_ON else 0.0) for s in ("l", "r")}
FISTB = fist_basis()
print("giro da palma:", {s: round(v, 1) for s, v in TW.items()})


def set_action(obj, act):
    obj.animation_data_create()
    obj.animation_data.action = act
    try:
        slots = obj.animation_data.action_suitable_slots
        if slots:
            obj.animation_data.action_slot = slots[0]
    except AttributeError:
        pass


def sample(src_act):
    """Pose do alvo (matriz de base por osso) em cada quadro da ação da fonte."""
    set_action(src, src_act)
    n = max(2, round(src_act.frame_range[1]))
    hips = []
    frames = []
    for f in range(n):
        sc.frame_set(f)
        W = {s: smw @ src.pose.bones[s].matrix for s in MAP.values()}
        hips.append(W[P + "Hips"].to_translation())
        frames.append(W)
    # avanço do quadril: tira a reta do primeiro ao último quadro em X e Y (o laço fecha no mesmo lugar)
    h0, h1 = hips[0], hips[-1] + (hips[-1] - hips[-2])  # o quadro n (= 0 do próximo ciclo), extrapolado
    out = []
    for f, W in enumerate(frames):
        u = f / n
        d = hips[f] - hips_rest
        d.x -= (h0.x - hips_rest.x) + (h1.x - h0.x) * u
        d.y -= (h0.y - hips_rest.y) + (h1.y - h0.y) * u
        d *= k_hip
        Pp, Ps, basis = {}, {}, {}
        for b in TB:
            n_ = b.name
            par = b.parent.name if b.parent else None
            if n_ in MAP:
                rot = (W[MAP[n_]].to_3x3().normalized() @ L[n_]).to_4x4()
                if n_ == "pelvis":
                    loc = R["pelvis"].to_translation() + d
                else:
                    loc = (Ps[par] @ REL[n_]).to_translation()
                want = Matrix.Translation(loc) @ rot
                bs = REL[n_].inverted() @ Ps[par].inverted() @ want
                Ps[n_] = want
                for s in ("l", "r"):
                    if n_ in (f"forearm_{s}", f"hand_{s}"):
                        bs = bs @ rot_y(TW[s] * 0.5)  # gira no próprio eixo antes do movimento
            else:
                bs = FISTB.get(n_, Matrix.Identity(4))
            basis[n_] = bs
            Pp[n_] = (Pp[par] @ REL[n_] @ bs) if par else (REL[n_] @ bs)
            if n_ not in MAP:
                Ps[n_] = (Ps[par] @ REL[n_] @ bs) if par else Pp[n_]
        out.append(basis)
    return out


def write(name, poses):
    old = bpy.data.actions.get(name)
    if old:
        bpy.data.actions.remove(old)
    act = bpy.data.actions.new(name)
    set_action(arm, act)
    prevq = {}
    seq = poses + [poses[0]]  # o último quadro repete o primeiro
    for f, basis in enumerate(seq):
        for b in TB:
            pb = arm.pose.bones[b.name]
            pb.rotation_mode = "QUATERNION"
            loc, q, _ = basis[b.name].decompose()
            if b.name in prevq:
                q.make_compatible(prevq[b.name])
            prevq[b.name] = q
            pb.rotation_quaternion = q
            pb.location = loc if b.name == "pelvis" else (0, 0, 0)
            pb.scale = (1, 1, 1)
            pb.keyframe_insert("rotation_quaternion", frame=f)
            if b.name == "pelvis":
                pb.keyframe_insert("location", frame=f)
    act.use_fake_user = True
    return act, len(seq) - 1


clips = json.load(open(clips_json)) if os.path.exists(clips_json) else {}
for target, src_name in pairs:
    sa = bpy.data.actions[src_name]
    act, frames = write(target, sample(sa))
    fs = None if "idle" in target else foot_speed(arm, sc, act, frames, FPS)
    clips[target] = {"frames": frames, "loop": True, "impact": [], "footSpeed": fs, "capeSpring": 1, "fonte": f"meshy:{src_name}"}
    print("CLIPE", target, "<-", src_name, clips[target])

for o in new:
    if o.name in bpy.data.objects:
        bpy.data.objects.remove(o)
for a in [a for a in bpy.data.actions if a.users == 0 and not a.use_fake_user]:
    bpy.data.actions.remove(a)
arm.animation_data.action = None
for p in arm.pose.bones:
    p.matrix_basis = Matrix.Identity(4)
json.dump(clips, open(clips_json, "w"), indent=1)
out = args[args.index("--out") + 1] if "--out" in args else bpy.data.filepath
bpy.ops.wm.save_as_mainfile(filepath=out, compress=True)
print("SALVO", out)
