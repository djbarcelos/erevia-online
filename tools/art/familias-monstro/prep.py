"""Prepara um modelo CC0 (Quaternius) das famílias de monstro para o retexture do Meshy e para a montagem final.

    blender --background --factory-startup --python tools/art/familias-monstro/prep.py -- <nome> <saida>

Faz, para o modelo <nome> (ver MODELS):
  1. importa o GLB, tira os ajudantes (vazios de ponta de osso, esferas de IK) e junta as malhas em UMA, com UM material;
  2. desdobra a UV (Smart UV Project): os originais não têm UV, só cor por material, e o retexture precisa de UV;
  3. escolhe as ações pelos papéis do jogo, renomeia com o prefixo da família e cria as que faltam (dano) a partir da
     pose parada com um achatamento do osso raiz;
  4. grava <saida>/<nome>_uv.glb (com rig e clipes, material branco) e <saida>/<nome>_static.glb (malha sem esqueleto,
     de pé, ~2 m, para mandar ao Meshy).
Todo laço tem limite de iterações.
"""
import math
import os
import sys

import bpy

argv = sys.argv[sys.argv.index("--") + 1:]
name, out = argv[0], argv[1]
os.makedirs(out, exist_ok=True)

# papéis do jogo -> ação do arquivo ("derive" cria a ação)
MODELS = {
    "wolf": dict(src="animals/wolf_P1gU3Qkr9r.glb", prefix="WF_", root="Body", drop_mesh=["Icosphere"],
                 clips=dict(Idle="Idle", Walk="Walk", Run="Gallop", Attack="Attack", Hit="Idle_HitReact_Left", Death="Death", Break1="Idle_2", Break2="Idle_2_HeadLow")),
    "spider": dict(src="monsters/spider.glb", prefix="SP_", root=None, drop_mesh=["Icosphere"],
                   clips=dict(Idle="Spider_Idle", Walk="Spider_Walk", Run="copy:Walk", Attack="Spider_Attack", Death="Spider_Death", Hit="derive")),
    "slime": dict(src="monsters/slime_b.glb", prefix="SL_", root=None, drop_mesh=["Icosphere"],
                  clips=dict(Idle="Slime_Idle", Walk="Slime_Walk", Run="copy:Walk", Attack="Slime_Attack", Death="Slime_Death", Hit="derive")),
    "frog": dict(src="monsters/frog.glb", prefix="FR_", root=None, drop_mesh=["Icosphere"],
                 clips=dict(Idle="Idle", Walk="Walk", Run="Run", Attack="Punch", Hit="HitReact", Death="Death"),
                 arms_from=("Walk", 0.25), arms_in=("Idle", "Hit")),
}
cfg = MODELS[name]
SRC = os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../assets-src/quaternius", cfg["src"]))

bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()
bpy.ops.import_scene.gltf(filepath=SRC)
sc = bpy.context.scene
FPS = sc.render.fps / sc.render.fps_base  # o importador traz o fps do arquivo (24)

# 1. limpeza
for o in list(bpy.data.objects):
    if o.type == "EMPTY" and o.parent is not None and o.parent.type == "ARMATURE":
        bpy.data.objects.remove(o)
    elif o.type == "MESH" and any(o.name.startswith(d) for d in cfg["drop_mesh"]):
        bpy.data.objects.remove(o)
arm = next(o for o in bpy.data.objects if o.type == "ARMATURE")
meshes = [o for o in bpy.data.objects if o.type == "MESH"]
print("MALHAS", [(m.name, len(m.data.vertices)) for m in meshes])
bpy.ops.object.select_all(action="DESELECT")
for m in meshes:
    m.select_set(True)
bpy.context.view_layer.objects.active = meshes[0]
if len(meshes) > 1:
    bpy.ops.object.join()
mesh = bpy.context.view_layer.objects.active
mesh.name = name
# um material só, branco
mesh.data.materials.clear()
mat = bpy.data.materials.new("Body")
mat.use_nodes = True
mesh.data.materials.append(mat)

# 2. UV
for uv in list(mesh.data.uv_layers):
    mesh.data.uv_layers.remove(uv)
bpy.context.view_layer.objects.active = mesh
mesh.select_set(True)
bpy.ops.object.mode_set(mode="EDIT")
bpy.ops.mesh.select_all(action="SELECT")
# os originais têm as faces soltas (flat shading): sem soldar, cada face vira uma ilha de UV
bpy.ops.mesh.remove_doubles(threshold=0.0002)
bpy.ops.mesh.mark_seam(clear=True)
bpy.ops.uv.smart_project(angle_limit=math.radians(70), island_margin=0.012, area_weight=0.6, correct_aspect=True)
bpy.ops.mesh.set_sharpness_by_angle(angle=math.radians(40))  # mantém o aspecto facetado onde o ângulo é forte
bpy.ops.object.mode_set(mode="OBJECT")
bpy.ops.object.shade_smooth()
print("ILHAS_FACES", len(mesh.data.polygons), len(mesh.data.vertices))
print("UV", [u.name for u in mesh.data.uv_layers])

# 3. ações
acts = {a.name: a for a in bpy.data.actions}


def find(n):
    return acts.get(n) or next((a for k, a in acts.items() if k.split("|")[-1] == n and "|" not in n), None)


keep = {}
for role, clip in cfg["clips"].items():
    if clip == "derive" or clip.startswith("copy:"):
        continue
    a = find(clip)
    if a is None:
        raise SystemExit(f"clipe {clip} não existe em {name}: {sorted(acts)}")
    keep[role] = a

# "copy:X": o mesmo movimento com outro nome (o Babylon guarda um grupo por nome, e andar e correr não podem dividir um)
for role, clip in cfg["clips"].items():
    if clip.startswith("copy:"):
        keep[role] = keep[clip[5:]].copy()

idle = keep["Idle"]

ARM_BONES = ("Shoulder", "UpperArm", "LowerArm", "Index", "Middle", "Pinky", "Thumb")


def fix_arms(src_role, at, roles):
    """Os clipes parado e dano do sapo não têm canal nos braços: eles ficariam abertos em T (a pose de repouso).
    Copia a pose dos braços de um quadro do andar (a fração `at` do clipe) para esses clipes, fixa do começo ao fim."""
    arm.animation_data_create()
    a = keep[src_role]
    arm.animation_data.action = a
    s0, e0 = a.frame_range
    sc.frame_set(int(s0 + (e0 - s0) * at))
    bpy.context.view_layer.update()
    names = [pb.name for pb in arm.pose.bones if pb.name.startswith(ARM_BONES)]
    vals = {n: (arm.pose.bones[n].location.copy(), arm.pose.bones[n].rotation_quaternion.copy()) for n in names}
    for role in roles:
        act = keep[role]
        arm.animation_data.action = act
        s1, e1 = act.frame_range
        for f in (int(s1), int(e1)):
            for n in names:
                pb = arm.pose.bones[n]
                pb.location, pb.rotation_quaternion = vals[n][0], vals[n][1]
                pb.keyframe_insert("location", frame=f)
                pb.keyframe_insert("rotation_quaternion", frame=f)


if cfg.get("arms_from"):
    fix_arms(cfg["arms_from"][0], cfg["arms_from"][1], cfg["arms_in"])


def make_hit(root):
    """Dano: parte da pose parada e faz um achatamento rápido do osso raiz (encolhe e estica), 0,4 s."""
    arm.animation_data_create()
    arm.animation_data.action = idle
    s, _ = idle.frame_range
    sc.frame_set(int(s))
    bpy.context.view_layer.update()
    rb = arm.pose.bones[root]
    wm = (arm.matrix_world @ rb.matrix).to_3x3()
    cols = [wm.col[i].normalized() for i in range(3)]
    up = max(range(3), key=lambda i: abs(cols[i].z))  # eixo do osso mais alinhado ao vertical do mundo
    base = {pb.name: (pb.location.copy(), pb.rotation_quaternion.copy(), pb.rotation_euler.copy(), pb.scale.copy()) for pb in arm.pose.bones}
    act = bpy.data.actions.new("Hit")
    arm.animation_data.action = act
    keys = [(1, 1.0, 1.0), (4, 0.78, 1.14), (8, 1.1, 0.95), (13, 1.0, 1.0)]  # quadro, escala no eixo vertical, nos outros
    for f, v, h in keys:
        for pb in arm.pose.bones:
            loc, rq, re, sca = base[pb.name]
            pb.location, pb.rotation_quaternion, pb.rotation_euler, pb.scale = loc, rq, re, sca.copy()
        for i in range(3):
            rb.scale[i] = v if i == up else h
        # a pose de todos os ossos entra no clipe: sem isso os ossos fora dele travariam na mistura
        for pb in arm.pose.bones:
            pb.keyframe_insert("location", frame=f)
            pb.keyframe_insert("rotation_quaternion" if pb.rotation_mode == "QUATERNION" else "rotation_euler", frame=f)
            pb.keyframe_insert("scale", frame=f)
    return act


roots = [b.name for b in arm.data.bones if b.parent is None]
root = cfg["root"] or roots[0]
print("RAIZ", root, "OSSOS", len(arm.data.bones))
if cfg["clips"].get("Hit") == "derive":
    keep["Hit"] = make_hit(root)

arm.animation_data.action = None
final = {}
for role, a in keep.items():
    final[a.name] = f"{cfg['prefix']}{role}"
for a in list(bpy.data.actions):
    if a.name not in final:
        bpy.data.actions.remove(a)
for old, new in final.items():
    bpy.data.actions[old].name = new
for a in bpy.data.actions:
    a.use_fake_user = True
    print("CLIPE", a.name, [round(v / FPS, 2) for v in a.frame_range])
arm.animation_data.action = bpy.data.actions[f"{cfg['prefix']}Idle"]
sc.frame_set(1)

# 4a. com esqueleto
for o in bpy.data.objects:
    o.select_set(o.type in ("ARMATURE", "MESH"))
uvglb = os.path.join(out, f"{name}_uv.glb")
bpy.ops.export_scene.gltf(filepath=uvglb, export_format="GLB", use_selection=True, export_animations=True,
                          export_animation_mode="ACTIONS", export_image_format="NONE", export_yup=True, export_extras=False)
print("OK", uvglb)

# 4b. malha estática de pé para o Meshy
bpy.ops.object.select_all(action="DESELECT")
arm.animation_data.action = None
mesh.select_set(True)
bpy.context.view_layer.objects.active = mesh
bpy.ops.object.duplicate()
st = bpy.context.view_layer.objects.active
bpy.ops.object.parent_clear(type="CLEAR_KEEP_TRANSFORM")
for md in list(st.modifiers):
    st.modifiers.remove(md)
st.vertex_groups.clear()
bpy.ops.object.transform_apply(location=True, rotation=True, scale=True)
k = 2.0 / max(st.dimensions)
st.scale = (k, k, k)
bpy.ops.object.transform_apply(scale=True)
bb = [st.matrix_world @ v.co for v in st.data.vertices]
st.location.z -= min(v.z for v in bb)
st.location.x -= (min(v.x for v in bb) + max(v.x for v in bb)) / 2
st.location.y -= (min(v.y for v in bb) + max(v.y for v in bb)) / 2
bpy.ops.object.transform_apply(location=True)
stglb = os.path.join(out, f"{name}_static.glb")
bpy.ops.export_scene.gltf(filepath=stglb, export_format="GLB", use_selection=True, export_animations=False, export_skins=False,
                          export_image_format="NONE", export_yup=True)
print("OK", stglb, tuple(round(v, 2) for v in st.dimensions))
