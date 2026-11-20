"""Heróis v4: veste um kit (personagem inteiro vestido, gerado no Meshy) no esqueleto do contrato.

1. Importa o kit rigado pelo Meshy (mixamo) e posa o esqueleto dele nas juntas do contrato (cada osso vai da
   junta à junta do nosso corpo, com escala no comprimento) -> ombros, cotovelos, joelhos e pulsos coincidem.
2. Aplica a pose na malha (o kit fica no repouso do contrato) e tira o esqueleto do Meshy.
3. Copia os pesos do corpo de alta resolução (Data Transfer, face mais próxima, interpolado): dedos, torção do
   braço e o resto vêm do corpo, e a mão do kit fecha na arma igual à do corpo.
4. Salva um .blend no formato do corpo (`corpo` = kit, `heroi` = esqueleto), pronto para make_lods.py,
   bake_normal.py e o resto do pipeline.

    Blender --background --factory-startup corpo.blend --python tools/art/herois-v4/fit_kit.py -- kit-rigado.glb saida.blend
"""
import sys

import bpy
from mathutils import Matrix

kit_glb, out_blend = sys.argv[sys.argv.index("--") + 1:][:2]
body = bpy.data.objects["corpo"]
arm = bpy.data.objects["heroi"]
before = set(bpy.data.objects)
bpy.ops.import_scene.gltf(filepath=kit_glb)
new = [o for o in bpy.data.objects if o not in before]
for o in list(new):
    if o.type == "MESH" and o.name.startswith("Icosphere"):
        new.remove(o)
        bpy.data.objects.remove(o)
karm = next(o for o in new if o.type == "ARMATURE")
kmesh = next(o for o in new if o.type == "MESH" and o.parent == karm)
P = "mixamorig:"
B = arm.data.bones


def jh(n):
    return arm.matrix_world @ B[n].head_local


def jt(n):
    return arm.matrix_world @ B[n].tail_local


# osso mixamo -> (ponto da cabeça, ponto da cauda) no nosso esqueleto
TGT = {"Hips": (jh("pelvis"), jh("spine_01")), "Spine": (jh("spine_01"), jh("spine_02")), "Spine1": (jh("spine_02"), jh("chest")),
       "Spine2": (jh("chest"), jh("neck")), "Neck": (jh("neck"), jh("head")), "Head": (jh("head"), jt("head"))}
for s, side in (("l", "Left"), ("r", "Right")):
    TGT.update({f"{side}Shoulder": (jh(f"clavicle_{s}"), jh(f"upperarm_{s}")), f"{side}Arm": (jh(f"upperarm_{s}"), jh(f"forearm_{s}")),
                f"{side}ForeArm": (jh(f"forearm_{s}"), jh(f"hand_{s}")), f"{side}Hand": (jh(f"hand_{s}"), jt(f"hand_{s}")),
                f"{side}UpLeg": (jh(f"thigh_{s}"), jh(f"calf_{s}")), f"{side}Leg": (jh(f"calf_{s}"), jh(f"foot_{s}")),
                f"{side}Foot": (jh(f"foot_{s}"), jh(f"toe_{s}")), f"{side}ToeBase": (jh(f"toe_{s}"), jt(f"toe_{s}"))})


def depth(b):
    d = 0
    while b.parent:
        b, d = b.parent, d + 1
    return d


# ---- 1. pose do esqueleto do kit nas nossas juntas ----
bpy.ops.object.select_all(action="DESELECT")
karm.select_set(True)
bpy.context.view_layer.objects.active = karm
bpy.ops.object.mode_set(mode="POSE")
for p in karm.pose.bones:
    p.bone.inherit_scale = "NONE"
kw = karm.matrix_world.inverted()
# a ponta de cada osso do kit é a junta do filho (a cauda do glTF importado é um palpite e esticava a cabeça)
CHILD = {"Hips": "Spine", "Spine": "Spine1", "Spine1": "Spine2", "Spine2": "Neck", "Neck": "Head", "Head": "HeadTop_End"}
for side in ("Left", "Right"):
    CHILD.update({f"{side}Shoulder": f"{side}Arm", f"{side}Arm": f"{side}ForeArm", f"{side}ForeArm": f"{side}Hand",
                  f"{side}UpLeg": f"{side}Leg", f"{side}Leg": f"{side}Foot", f"{side}Foot": f"{side}ToeBase", f"{side}ToeBase": f"{side}Toe_End"})
KB = karm.data.bones
NOSCALE = {"Head", "LeftHand", "RightHand", "LeftToeBase", "RightToeBase"}
moved = 0
for p in sorted(karm.pose.bones, key=lambda b: depth(b.bone)):
    key = p.name.replace(P, "")
    if key not in TGT:
        continue
    h_t, t_t = (kw @ TGT[key][0]), (kw @ TGT[key][1])
    rest = p.bone.matrix_local
    h_k = rest.to_translation()
    child = KB.get(P + CHILD.get(key, ""))
    t_k = child.head_local if child else p.bone.tail_local
    d_k, d_t = (t_k - h_k), (t_t - h_t)
    # a cabeça só muda de lugar: girar até a nossa direção tombava o rosto para cima (o topo da cabeça do rig
    # do Meshy não aponta para cima)
    rot = Matrix.Identity(4) if key == "Head" else d_k.normalized().rotation_difference(d_t.normalized()).to_matrix().to_4x4()
    # cabeça, mão e ponta do pé só giram: as juntas-ponta do rig do Meshy (topo da cabeça, fim do dedão) ficam
    # perto demais da base e a escala esticava a cabeça a 3 m de altura
    k = d_t.length / max(1e-6, d_k.length) if child and key not in NOSCALE else 1.0
    y = rest.to_3x3().col[1].normalized()
    sc = Matrix.Identity(3) + (k - 1) * Matrix([[y.x * y.x, y.x * y.y, y.x * y.z], [y.y * y.x, y.y * y.y, y.y * y.z], [y.z * y.x, y.z * y.y, y.z * y.z]])
    p.matrix = Matrix.Translation(h_t) @ rot @ sc.to_4x4() @ Matrix.Translation(-h_k) @ rest
    bpy.context.view_layer.update()
    moved += 1
bpy.ops.object.mode_set(mode="OBJECT")
print("ossos do kit levados às juntas do contrato:", moved)

# ---- 2. aplica a pose na malha e tira o esqueleto do Meshy ----
bpy.ops.object.select_all(action="DESELECT")
kmesh.select_set(True)
bpy.context.view_layer.objects.active = kmesh
for m in list(kmesh.modifiers):
    if m.type == "ARMATURE":
        bpy.ops.object.modifier_apply(modifier=m.name)
mw = kmesh.matrix_world.copy()
kmesh.parent = None
kmesh.matrix_world = mw
bpy.ops.object.transform_apply(location=True, rotation=True, scale=True)
for g in list(kmesh.vertex_groups):
    kmesh.vertex_groups.remove(g)
for o in new:
    if o != kmesh and o.name in bpy.data.objects:
        bpy.data.objects.remove(o)

# solda costuras de UV (como no build_rig.py) para a transferência e a suavização não rasgarem
import bmesh
bm = bmesh.new()
bm.from_mesh(kmesh.data)
bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)
bm.to_mesh(kmesh.data)
bm.free()

# ---- 3. pesos do corpo ----
for g in body.vertex_groups:
    kmesh.vertex_groups.new(name=g.name)
dt = kmesh.modifiers.new("Pesos", "DATA_TRANSFER")
dt.object = body
dt.use_vert_data = True
dt.data_types_verts = {"VGROUP_WEIGHTS"}
dt.vert_mapping = "POLYINTERP_NEAREST"
dt.layers_vgroup_select_src = "ALL"
dt.layers_vgroup_select_dst = "NAME"
bpy.ops.object.modifier_apply(modifier=dt.name)
bpy.ops.object.mode_set(mode="WEIGHT_PAINT")
bpy.ops.object.vertex_group_limit_total(group_select_mode="ALL", limit=4)
bpy.ops.object.vertex_group_normalize_all(group_select_mode="ALL", lock_active=False)
bpy.ops.object.mode_set(mode="OBJECT")

# ---- 3b. mãos do corpo no lugar das do kit (os dedos do kit vêm fundidos e viram garra) ----
HANDS = ("hand_", "thumb_", "index_", "fingers_")


def hand_weight(o):
    names = {g.index: g.name for g in o.vertex_groups}
    return [sum(g.weight for g in v.groups if names[g.group].startswith(HANDS)) for v in o.data.vertices]


def keep_only(o, keep):
    bm = bmesh.new()
    bm.from_mesh(o.data)
    bm.verts.ensure_lookup_table()
    bmesh.ops.delete(bm, geom=[v for v in bm.verts if not keep[v.index]], context="VERTS")
    bm.to_mesh(o.data)
    bm.free()


CUT = 0.5  # o corte acompanha o pulso (onde a braçadeira do kit termina)
hw = hand_weight(kmesh)
keep_only(kmesh, [w < CUT for w in hw])
hands = body.copy()
hands.data = body.data.copy()
bpy.context.scene.collection.objects.link(hands)
for m in list(hands.modifiers):
    hands.modifiers.remove(m)
hands.parent = None
hands.matrix_world = body.matrix_world.copy()
hw = hand_weight(hands)
keep_only(hands, [w >= CUT * 0.9 for w in hw])  # um pouco antes do corte: a mão entra sob a braçadeira
bpy.ops.object.select_all(action="DESELECT")
hands.select_set(True)
kmesh.select_set(True)
bpy.context.view_layer.objects.active = kmesh
bpy.ops.object.join()
print("mãos do corpo no kit; materiais:", [m.name for m in kmesh.data.materials])

# ---- 4. o kit vira o "corpo" ----
kname = "corpo"
bpy.data.objects.remove(body)
kmesh.name = kname
kmesh.data.name = kname
kmesh.parent = arm
mod = kmesh.modifiers.new("Armature", "ARMATURE")
mod.object = arm
print("kit:", len(kmesh.data.vertices), "vértices,", sum(len(p.vertices) - 2 for p in kmesh.data.polygons), "triângulos")
bpy.ops.wm.save_as_mainfile(filepath=out_blend, compress=True)
print("SALVO", out_blend)
