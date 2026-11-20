"""Heróis v4: cria o shape key `feminino` no corpo masculino e a textura feminina no mesmo UV.

1. Importa o corpo feminino rigado (mixamo, A-pose, fix_arms.py).
2. Posa o esqueleto `heroi` com as juntas da mulher (cada osso vai da junta à junta dela, com escala no
   comprimento) -> o corpo masculino fica com as proporções dela.
3. Gruda cada vértice na superfície feminina mais próxima (até 4 cm; as mãos e os dedos ficam de fora).
4. Desfaz a pose (inversa da pele linear, vértice a vértice) -> o shape key fica no repouso masculino:
   as juntas não mudam, só a forma. O mesmo esqueleto anima os dois (o feminino com escala 0,945).
5. Assa a cor da mulher no UV do corpo (Cycles, selecionado para ativo) -> textura feminina.

    Blender --background --factory-startup corpo.blend --python tools/art/herois-v4/make_female.py -- corpo-f-bracos.glb saida.blend textura-f.png
"""
import sys

import bpy
from mathutils import Matrix, Vector
from mathutils.bvhtree import BVHTree

f_glb, out_blend, out_tex = sys.argv[sys.argv.index("--") + 1:][:3]
body = bpy.data.objects["corpo"]
arm = bpy.data.objects["heroi"]
before = set(bpy.data.objects)
bpy.ops.import_scene.gltf(filepath=f_glb)
new = [o for o in bpy.data.objects if o not in before]
for o in list(new):
    if o.type == "MESH" and o.name.startswith("Icosphere"):
        new.remove(o)
        bpy.data.objects.remove(o)
farm = next(o for o in new if o.type == "ARMATURE")
fmesh = next(o for o in new if o.type == "MESH")
P = "mixamorig:"


def fj(n):
    return farm.matrix_world @ farm.data.bones[P + n].head_local


# osso do contrato -> (junta de cabeça, junta de cauda) da mulher
MAP = {"pelvis": ("Hips", "Spine"), "spine_01": ("Spine", "Spine1"), "spine_02": ("Spine1", "Spine2"),
       "chest": ("Spine2", "Neck"), "neck": ("Neck", "Head"), "head": ("Head", "HeadTop_End")}
for s, side in (("l", "Left"), ("r", "Right")):
    MAP.update({f"clavicle_{s}": (f"{side}Shoulder", f"{side}Arm"), f"upperarm_{s}": (f"{side}Arm", f"{side}ForeArm"),
                f"forearm_{s}": (f"{side}ForeArm", f"{side}Hand"), f"hand_{s}": (f"{side}Hand", f"{side}HandMiddle1"),
                f"thigh_{s}": (f"{side}UpLeg", f"{side}Leg"), f"calf_{s}": (f"{side}Leg", f"{side}Foot"),
                f"foot_{s}": (f"{side}Foot", f"{side}ToeBase"), f"toe_{s}": (f"{side}ToeBase", f"{side}Toe_End")})

# ---- 2. pose do esqueleto masculino nas juntas da mulher ----
bpy.context.view_layer.objects.active = arm
bpy.ops.object.mode_set(mode="POSE")
for pb in arm.pose.bones:
    pb.bone.inherit_scale = "NONE"


def depth(b):
    d = 0
    while b.parent:
        b, d = b.parent, d + 1
    return d


aw = arm.matrix_world.inverted()
for pb in sorted(arm.pose.bones, key=lambda b: depth(b.bone)):
    if pb.name not in MAP:
        continue
    h_f = aw @ fj(MAP[pb.name][0])
    t_f = aw @ fj(MAP[pb.name][1])
    rest = pb.bone.matrix_local
    h_m, t_m = rest.to_translation(), (pb.bone.tail_local)
    d_m, d_f = (t_m - h_m), (t_f - h_f)
    rot = d_m.normalized().rotation_difference(d_f.normalized()).to_matrix().to_4x4()
    k = d_f.length / max(1e-6, d_m.length)
    y = rest.to_3x3().col[1].normalized()  # eixo do osso
    sc = Matrix.Identity(3) + (k - 1) * Matrix([[y.x * y.x, y.x * y.y, y.x * y.z], [y.y * y.x, y.y * y.y, y.y * y.z], [y.z * y.x, y.z * y.y, y.z * y.z]])
    M = Matrix.Translation(h_f) @ rot @ sc.to_4x4() @ Matrix.Translation(-h_m)
    pb.matrix = M @ rest
    bpy.context.view_layer.update()
twist = {}
for s in ("l", "r"):  # o twist fica no meio do braço da mulher
    pb = arm.pose.bones[f"upperarm_twist_{s}"]
    up = arm.pose.bones[f"upperarm_{s}"]
    mid = (up.head + up.tail) / 2
    pb.matrix = Matrix.Translation(mid - pb.head) @ pb.matrix
bpy.context.view_layer.update()
bpy.ops.object.mode_set(mode="OBJECT")

# ---- 3. corpo posado + grudar na superfície da mulher ----
dg = bpy.context.evaluated_depsgraph_get()
ev = body.evaluated_get(dg)
posed = [body.matrix_world @ v.co for v in ev.data.vertices]
fev = fmesh.evaluated_get(dg)
fverts = [fmesh.matrix_world @ v.co for v in fev.data.vertices]
bvh = BVHTree.FromPolygons(fverts, [p.vertices[:] for p in fev.data.polygons])
names = {g.index: g.name for g in body.vertex_groups}
HANDS = ("hand_", "thumb_", "index_", "fingers_")
TRUNK = ("pelvis", "spine_", "chest", "clavicle_")
final = []
moved = 0
delta = []
w_hand = []
for v, p in zip(body.data.vertices, posed):
    wh = sum(g.weight for g in v.groups if names[g.group].startswith(HANDS))
    wt = sum(g.weight for g in v.groups if names[g.group].startswith(TRUNK))
    hit = bvh.find_nearest(p, 0.04 + 0.06 * min(1.0, wt))  # tronco e clavícula vão mais longe (até 10 cm)
    w_hand.append(wh)
    if hit[0] is not None and wh < 0.5:
        delta.append((hit[0] - p) * (1.0 - wh * 2))
        moved += 1
    else:
        delta.append(Vector())
# suaviza o deslocamento entre vizinhos: sem degraus nem pontas
nb = [[] for _ in body.data.vertices]
for e in body.data.edges:
    i0, i1 = e.vertices
    nb[i0].append(i1)
    nb[i1].append(i0)
for _ in range(12):
    delta = [d * 0.5 + (sum((delta[k] for k in nb[i]), Vector()) / len(nb[i])) * 0.5 if nb[i] else d for i, d in enumerate(delta)]
final = [p + d for p, d in zip(posed, delta)]
print(f"grudados na superfície feminina: {moved} de {len(final)} vértices")

# ---- 4. desfaz a pose: inversa da pele linear em cada vértice ----
pose_m = {pb.name: arm.matrix_world @ pb.matrix @ pb.bone.matrix_local.inverted() @ arm.matrix_world.inverted() for pb in arm.pose.bones}
rest_pos = []
for v, q in zip(body.data.vertices, final):
    M = Matrix(((0, 0, 0, 0),) * 4)
    tot = 0
    for g in v.groups:
        n = names[g.group]
        if n in pose_m and g.weight > 0:
            M = M + pose_m[n] * g.weight
            tot += g.weight
    if tot < 1e-6:
        rest_pos.append(body.matrix_world.inverted() @ q)
        continue
    M = M * (1.0 / tot)
    rest_pos.append(body.matrix_world.inverted() @ (M.inverted_safe() @ q))

if not body.data.shape_keys:
    body.shape_key_add(name="Basis", from_mix=False)
key = body.data.shape_keys.key_blocks.get("feminino") or body.shape_key_add(name="feminino", from_mix=False)
for i, p in enumerate(rest_pos):
    key.data[i].co = p
key.value = 0.0
print("shape key 'feminino' criado")

# ---- 5. textura feminina no UV do corpo (assar com o corpo posado e grudado) ----
bake = body.copy()
bake.data = body.data.copy()
bpy.context.scene.collection.objects.link(bake)
bake.shape_key_clear()
for m in list(bake.modifiers):
    bake.modifiers.remove(m)
bake.parent = None
bake.matrix_world = Matrix.Identity(4)
for i, q in enumerate(final):
    bake.data.vertices[i].co = q
img = bpy.data.images.new("corpo_f", 2048, 2048, alpha=True)
img.pixels.foreach_set([0.0] * (2048 * 2048 * 4))  # transparente: o que o assado não cobrir fica com a cor do masculino
mat = bpy.data.materials.new("bake_f")
mat.use_nodes = True
node = mat.node_tree.nodes.new("ShaderNodeTexImage")
node.image = img
mat.node_tree.nodes.active = node
bake.data.materials.clear()
bake.data.materials.append(mat)
sc = bpy.context.scene
sc.render.engine = "CYCLES"
sc.cycles.device = "CPU"
sc.cycles.samples = 4
sc.render.bake.use_selected_to_active = True
sc.render.bake.cage_extrusion = 0.06
sc.render.bake.max_ray_distance = 0.2
sc.render.bake.use_pass_direct = False
sc.render.bake.use_pass_indirect = False
sc.render.bake.margin = 8
bpy.ops.object.select_all(action="DESELECT")
fmesh.select_set(True)
bake.select_set(True)
bpy.context.view_layer.objects.active = bake
bpy.ops.object.bake(type="DIFFUSE", pass_filter={"COLOR"})
# onde o assado não achou a mulher (alfa 0), entra a cor do masculino no mesmo UV
import array
src_img = next(n.image for m in body.data.materials if m and m.use_nodes for n in m.node_tree.nodes if n.type == "TEX_IMAGE" and n.image)
if tuple(src_img.size) != (2048, 2048):
    src_img = src_img.copy()
    src_img.scale(2048, 2048)
fpx = array.array("f", [0.0]) * (2048 * 2048 * 4)
mpx = array.array("f", [0.0]) * (2048 * 2048 * 4)
img.pixels.foreach_get(fpx)
src_img.pixels.foreach_get(mpx)
miss = 0
for k in range(0, len(fpx), 4):
    al = fpx[k + 3]
    if al < 0.999:
        miss += 1
        for c in range(3):
            fpx[k + c] = fpx[k + c] * al + mpx[k + c] * (1 - al)
    fpx[k + 3] = 1.0
img.pixels.foreach_set(fpx)
print(f"texels completados com o masculino: {miss}")
img.filepath_raw = out_tex
img.file_format = "PNG"
img.save()
print("textura feminina:", out_tex)

# limpa: tira a pose, os objetos de apoio e salva
for pb in arm.pose.bones:
    pb.matrix_basis = Matrix.Identity(4)
    pb.bone.inherit_scale = "FULL"
for o in [bake, fmesh, farm] + [o for o in new if o.name in bpy.data.objects and o not in (fmesh, farm)]:
    if o.name in bpy.data.objects:
        bpy.data.objects.remove(o)
bpy.ops.wm.save_as_mainfile(filepath=out_blend, compress=True)
print("SALVO", out_blend)
