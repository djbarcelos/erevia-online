"""Heróis v4: animações da base (contrato, seção 5; movimento.md, seção 2) feitas no Blender.

Monta um rig de controle sobre o esqueleto `heroi` (alvos de IK para pés e mãos, com polo de joelho e
cotovelo), anima os alvos e o quadril por curvas e converte (bake) para os ossos do contrato, sem root
motion. Gera, por enquanto:

- `base.walk`  30 quadros, laço, 1,6 m/s (2 passos de 0,8 m)
- `base.run`   21 quadros, laço, 5,0 m/s (2 passadas de 1,75 m)
- `rc.idle`   120 quadros, laço: respiração, troca de peso e um olhar de lado

O pé de apoio anda para trás exatamente na velocidade do clipe: no jogo, tocado a `velocidade / footSpeed`,
o pé fica parado no chão (deslize 1,00).

    Blender --background --factory-startup corpo-lod.blend --python tools/art/herois-v4/anim_base.py -- saida.blend saida-clips.json
"""
import json
import math
import sys

import bpy
from mathutils import Matrix, Vector

out_blend, out_json = sys.argv[sys.argv.index("--") + 1:][:2]
FPS = 30
arm = bpy.data.objects["heroi"]
sc = bpy.context.scene
sc.render.fps = FPS
for o in bpy.data.objects:  # o corpo de alta resolução e os LODs não importam para animar
    if o.type == "MESH":
        o.hide_viewport = False

rest = {b.name: b.matrix_local.copy() for b in arm.data.bones}
H = {b.name: b.head_local.copy() for b in arm.data.bones}
T = {b.name: b.tail_local.copy() for b in arm.data.bones}
FWD = Vector((0, -1, 0))  # frente do personagem
UP = Vector((0, 0, 1))

# ---------- rig de controle ----------
arm.hide_set(False)
arm.hide_viewport = False
bpy.ops.object.select_all(action="DESELECT")
arm.select_set(True)
bpy.context.view_layer.objects.active = arm
bpy.ops.object.mode_set(mode="EDIT")
eb = arm.data.edit_bones


def ctrl(name, head, tail, parent="root"):
    b = eb.new(name)
    b.head, b.tail = head, tail
    b.use_deform = False
    b.parent = eb[parent]
    return b


for s in ("l", "r"):
    fb = ctrl(f"ik_foot_{s}", H[f"foot_{s}"], T[f"foot_{s}"])  # mesma orientação do pé (com o roll):
    fb.roll = eb[f"foot_{s}"].roll                              # copiar a rotação não muda o repouso
    knee = H[f"calf_{s}"] + FWD * 0.5
    ctrl(f"pole_knee_{s}", knee, knee + UP * 0.05)
    ctrl(f"ik_hand_{s}", H[f"hand_{s}"], T[f"hand_{s}"])
    elbow = H[f"forearm_{s}"] + Vector((0, 0.45, 0))  # cotovelo aponta para trás
    ctrl(f"pole_elbow_{s}", elbow, elbow + UP * 0.05)
bpy.ops.object.mode_set(mode="POSE")
pb = arm.pose.bones
print("ossos com controle:", len(arm.data.bones), "ik_foot_l" in arm.data.bones)
for s in ("l", "r"):
    c = pb[f"calf_{s}"].constraints.new("IK")
    c.target, c.subtarget = arm, f"ik_foot_{s}"
    c.pole_target, c.pole_subtarget = arm, f"pole_knee_{s}"
    c.pole_angle = math.radians(-90)
    c.chain_count = 2
    cr = pb[f"foot_{s}"].constraints.new("COPY_ROTATION")
    cr.target, cr.subtarget = arm, f"ik_foot_{s}"
    ca = pb[f"forearm_{s}"].constraints.new("IK")
    ca.target, ca.subtarget = arm, f"ik_hand_{s}"
    ca.pole_target, ca.pole_subtarget = arm, f"pole_elbow_{s}"
    ca.pole_angle = math.radians(-90)
    ca.chain_count = 2
    ca.influence = 0.0  # braços em FK por padrão; ligar por clipe quando a mão precisa de alvo


def pick_pole(con, bones):
    """Escolhe o ângulo do polo que deixa a cadeia em repouso com o alvo no lugar (sem torcer o osso)."""
    best = None
    for deg in (-90, 0, 90, 180):
        con.pole_angle = math.radians(deg)
        bpy.context.view_layer.update()
        err = sum((pb[b].matrix - arm.data.bones[b].matrix_local).to_3x3().to_quaternion().angle if False else
                  pb[b].matrix.to_quaternion().rotation_difference(arm.data.bones[b].matrix_local.to_quaternion()).angle for b in bones)
        if best is None or err < best[0]:
            best = (err, deg)
    con.pole_angle = math.radians(best[1])
    print(f"polo {con.subtarget}: {best[1]}° (erro {math.degrees(best[0]):.1f}°)")


for s in ("l", "r"):
    pick_pole(pb[f"calf_{s}"].constraints["IK"], [f"thigh_{s}", f"calf_{s}"])
    bpy.context.view_layer.update()
    d = pb[f"foot_{s}"].matrix.to_quaternion().rotation_difference(arm.data.bones[f"foot_{s}"].matrix_local.to_quaternion()).angle
    print(f"pé {s} em repouso com o rig: {math.degrees(d):.1f}° do repouso")
    ca = pb[f"forearm_{s}"].constraints["IK"]
    ca.influence = 1.0
    pick_pole(ca, [f"upperarm_{s}", f"forearm_{s}"])
    ca.influence = 0.0


def palm_twist(s):
    """Giro (graus, no eixo do antebraço) que deixa a linha dos nós (do mínimo ao indicador) para a frente,
    com a palma virada para o corpo. O corpo do Meshy veio com a mão direita girada ~90° (palma para trás):
    sem isto a arma sai de lado. Repartido meio a meio entre o antebraço e a mão (sem torcer o pulso)."""
    a = (H[f"hand_{s}"] - H[f"forearm_{s}"]).normalized()
    k = H[f"index_01_{s}"] - H[f"fingers_01_{s}"]
    k = (k - a * k.dot(a)).normalized()
    t = (FWD - a * FWD.dot(a)).normalized()
    return math.degrees(math.atan2(k.cross(t).dot(a), k.dot(t)))


TWIST = {s: palm_twist(s) for s in ("l", "r")}
print("giro da palma (graus):", {s: round(v, 1) for s, v in TWIST.items()})


def twist_keys(f, bend):
    """Antebraço: primeiro gira no próprio eixo (Y), depois dobra o cotovelo (X) no plano de sempre."""
    for s in ("l", "r"):
        key(f"forearm_{s}", f, rot=(bend[s], TWIST[s] * 0.5, 0), mode="YXZ")
        key(f"hand_{s}", f, rot=(0, TWIST[s] * 0.5, 0))


def key(bone, frame, loc=None, rot=None, mode="XYZ"):
    """loc: deslocamento em relação ao repouso, em coordenadas do mundo; rot: Euler (graus) local, ordem `mode`."""
    p = pb[bone]
    if loc is not None:
        # converte deslocamento de mundo para o espaço local do osso
        m = arm.data.bones[bone].matrix_local.to_3x3().inverted()
        p.location = m @ Vector(loc)
        p.keyframe_insert("location", frame=frame)
    if rot is not None:
        p.rotation_mode = mode
        p.rotation_euler = [math.radians(a) for a in rot]
        p.keyframe_insert("rotation_euler", frame=frame)


def clear():
    for p in pb:
        p.location = (0, 0, 0)
        p.rotation_mode = "XYZ"
        p.rotation_euler = (0, 0, 0)
        p.rotation_quaternion = (1, 0, 0, 0)
        p.scale = (1, 1, 1)
    arm.animation_data_create()
    arm.animation_data.action = None


def cycle(t, a, b):
    """Interpolação suave de a para b em t (0..1)."""
    return a + (b - a) * (0.5 - 0.5 * math.cos(math.pi * t))


def foot_track(phase, stride, lift, duty):
    """Pé num ciclo: no apoio (fração `duty`) anda para trás de +stride/2 a -stride/2; no balanço volta
    pelo ar. Devolve (avanço em Y-frente, altura, inclinação da sola em graus)."""
    if phase < duty:
        u = phase / duty
        return stride / 2 - stride * u, 0.0, (-10 * (1 - u) if u < 0.15 else (25 * (u - 0.8) / 0.2 if u > 0.8 else 0))
    u = (phase - duty) / (1 - duty)
    fwd = -stride / 2 + stride * (0.5 - 0.5 * math.cos(math.pi * u))
    h = lift * math.sin(math.pi * u) ** 1.2
    return fwd, h, 30 * math.sin(math.pi * u) * (1 - u)


def fist(f):
    """Mãos fechadas na arma (o herói está sempre armado): punho do teste de pose (pose_test.py)."""
    for s in ("l", "r"):
        for k, ang in ((1, 55), (2, 70), (3, 45)):
            key(f"index_{k:02d}_{s}", f, rot=(ang, 0, 0))
        for k, ang in ((1, 75), (2, 85)):
            key(f"fingers_{k:02d}_{s}", f, rot=(ang, 0, 0))
        for k, ang in ((1, 20), (2, 25), (3, 20)):
            key(f"thumb_{k:02d}_{s}", f, rot=(ang, 0, 0))


def make_locomotion(name, frames, speed, lift, duty, bob, lean, arm_swing, spine_twist, elbow=15):
    clear()
    act = bpy.data.actions.new(name)
    arm.animation_data.action = act
    stride = speed * duty * frames / FPS  # no apoio o pé anda para trás exatamente na velocidade do clipe
    for f in range(frames + 1):
        ph = f / frames
        bend = {}
        for s, off in (("l", 0.0), ("r", 0.5)):
            fwd, h, pitch = foot_track((ph + off) % 1.0, stride, lift, duty)
            key(f"ik_foot_{s}", f, loc=FWD * fwd + UP * h, rot=(pitch, 0, 0))
            # braço oposto à perna: balança para frente quando a perna do outro lado vai
            sw = arm_swing * math.cos(2 * math.pi * (ph + off))
            sg = 1 if s == "l" else -1
            key(f"upperarm_{s}", f, rot=(sw, 0, sg * -38))  # braço abaixado ao lado do corpo e balançando
            bend[s] = -abs(sw) * 0.4 - elbow
        # quadril: desce no contato e sobe na passagem (duas vezes por ciclo)
        z = -bob * (0.5 + 0.5 * math.cos(4 * math.pi * ph))
        sway = 0.02 * math.sin(2 * math.pi * ph)
        key("pelvis", f, loc=UP * z + Vector((sway, 0, 0)), rot=(0, 0, spine_twist * math.sin(2 * math.pi * ph)))
        key("spine_02", f, rot=(lean, 0, -spine_twist * 1.4 * math.sin(2 * math.pi * ph)))
        key("chest", f, rot=(lean * 0.5, 0, -spine_twist * 0.6 * math.sin(2 * math.pi * ph)))
        key("head", f, rot=(-lean * 0.8, 0, spine_twist * 0.8 * math.sin(2 * math.pi * ph)))
        twist_keys(f, bend)
        fist(f)
    return act


def make_idle(name, frames):
    clear()
    act = bpy.data.actions.new(name)
    arm.animation_data.action = act
    for f in range(frames + 1):
        ph = f / frames
        br = math.sin(2 * math.pi * ph * 4)  # 4 respirações no ciclo
        shift = math.sin(2 * math.pi * ph)   # troca de peso: uma ida e volta
        look = math.sin(2 * math.pi * ph) ** 3
        bend = {}
        for s, sg in (("l", 1), ("r", -1)):
            key(f"ik_foot_{s}", f, loc=Vector((sg * 0.04, 0, 0)), rot=(0, 0, 0))
            key(f"upperarm_{s}", f, rot=(4 + 1.5 * br, 0, sg * -38))  # braços caídos ao lado do corpo
            bend[s] = -12 - 2 * br
            key(f"clavicle_{s}", f, rot=(0, 0, sg * 1.5 * br))
        key("pelvis", f, loc=Vector((0.025 * shift, 0, -0.01 - 0.005 * br)), rot=(0, 2.5 * shift, 0))
        key("spine_02", f, rot=(2 + 1.2 * br, -1.5 * shift, 0))
        key("chest", f, rot=(1.5 * br, -1 * shift, 0))
        key("head", f, rot=(-2, 0, 18 * look))
        twist_keys(f, bend)
        fist(f)
    return act


def fcurves_of(act):
    """Curvas de uma ação (Blender 5: ação em camadas; Blender 4: act.fcurves)."""
    out = []
    if hasattr(act, "layers") and act.layers:
        for layer in act.layers:
            for strip in layer.strips:
                for cb in strip.channelbags:
                    out += [(cb.fcurves, fc) for fc in list(cb.fcurves)]
    elif hasattr(act, "fcurves"):
        out = [(act.fcurves, fc) for fc in list(act.fcurves)]
    return out


def bake(act, frames, loop):
    """Converte a ação de controle para os ossos do contrato (FK puro, sem os ossos de controle)."""
    arm.animation_data.action = act
    sc.frame_start, sc.frame_end = 0, frames
    bpy.ops.object.mode_set(mode="POSE")
    bpy.ops.pose.select_all(action="DESELECT")
    for b in arm.data.bones:
        want = b.use_deform or b.name == "root"
        p = arm.pose.bones[b.name]
        if hasattr(p, "select"):  # Blender 5: a seleção mora no osso da pose
            p.select = want
        else:
            b.select = want
    bpy.ops.nla.bake(frame_start=0, frame_end=frames, only_selected=True, visual_keying=True,
                     clear_constraints=False, use_current_action=False, bake_types={"POSE"})
    baked = arm.animation_data.action
    nm = act.name
    bpy.data.actions.remove(act)
    baked.name = nm  # depois de remover a de controle, para não virar "nome.001"
    # tira as curvas dos ossos de controle (não vão para o jogo)
    for coll, fc in fcurves_of(baked):
        if any(k in fc.data_path for k in ("ik_", "pole_")):
            coll.remove(fc)
    baked.use_fake_user = True
    return baked


def foot_speed(act, frames):
    """Velocidade do pé de apoio (o mais baixo) em relação ao quadril, em m/s, medida no clipe pronto."""
    arm.animation_data.action = act
    sp = []
    prev = None
    for f in range(frames):
        sc.frame_set(f)
        fl = arm.matrix_world @ pb["foot_l"].head
        fr = arm.matrix_world @ pb["foot_r"].head
        low = fl if fl.z < fr.z else fr
        if prev is not None and abs(low.z - prev.z) < 0.01:
            sp.append((low - prev).length * FPS)
        prev = low
    sp.sort()
    return round(sp[len(sp) // 2], 2) if sp else None


CLIPS = {}
specs = [
    ("base.walk", 30, lambda: make_locomotion("base.walk", 30, 1.6, 0.11, 0.62, 0.022, 4, 18, 6), True),
    ("base.run", 21, lambda: make_locomotion("base.run", 21, 5.0, 0.32, 0.3, 0.035, 12, 35, 10, elbow=80), True),
    ("rc.idle", 120, lambda: make_idle("rc.idle", 120), True),
]
for name, frames, fn, loop in specs:
    act = bake(fn(), frames, loop)
    CLIPS[name] = {"frames": frames, "loop": loop, "impact": [], "footSpeed": None, "capeSpring": 1}

# remove o rig de controle (constraints e ossos) e deixa o esqueleto do contrato em repouso
bpy.ops.object.mode_set(mode="POSE")
for p in pb:
    for c in list(p.constraints):
        p.constraints.remove(c)
arm.animation_data.action = None
bpy.ops.object.mode_set(mode="EDIT")
for n in [b.name for b in arm.data.edit_bones if b.name.startswith(("ik_", "pole_"))]:
    arm.data.edit_bones.remove(arm.data.edit_bones[n])
bpy.ops.object.mode_set(mode="OBJECT")
pb = arm.pose.bones
for name in CLIPS:  # mede o pé já sem o rig de controle (só os ossos do contrato)
    if "idle" not in name:
        CLIPS[name]["footSpeed"] = foot_speed(bpy.data.actions[name], CLIPS[name]["frames"])
    print(name, CLIPS[name])
arm.animation_data.action = None
bpy.ops.wm.save_as_mainfile(filepath=out_blend, compress=True)
json.dump(CLIPS, open(out_json, "w"), indent=1)
print("SALVO", out_blend, out_json)
