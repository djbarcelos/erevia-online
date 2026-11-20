"""Heróis v4: corpo simétrico a partir do lado esquerdo (o direito do Meshy veio ruim).

O corpo do Meshy veio com o braço e a mão direitos torcidos (~75°, palma para trás) e a lateral direita do
tronco pior que a esquerda; os consertos depois (giro da palma, pesos) eram remendo. Aqui o lado esquerdo
(+X; o personagem olha para -Y) vira os dois:

1. o plano do meio é a média em X da coluna (Hips, Spine, Spine1, Spine2, Neck, Head);
2. a malha é cortada nesse plano, o lado direito sai, e o esquerdo é espelhado (Mirror com solda no meio);
   os grupos de vértice `Left*` do espelho viram `Right*`; o UV do espelho é o mesmo do esquerdo;
3. os ossos `Right*` passam a ser o espelho dos `Left*` (cabeça, cauda e roll), e os do meio vão para o plano.

Entra o corpo com o rig do Meshy já em A-pose (fix_arms.py); sai no mesmo formato, para o build_rig.py.

    Blender --background --factory-startup --python tools/art/herois-v4/mirror_left.py -- corpo-bracos.glb saida.glb
"""
import sys

import bmesh
import bpy
from mathutils import Vector

src, dst = sys.argv[sys.argv.index("--") + 1:][:2]
bpy.ops.wm.read_factory_settings(use_empty=True)
bpy.ops.import_scene.gltf(filepath=src)
for o in list(bpy.data.objects):
    if o.type == "MESH" and o.name.startswith("Icosphere"):
        bpy.data.objects.remove(o)
arm = next(o for o in bpy.data.objects if o.type == "ARMATURE")
body = next(o for o in bpy.data.objects if o.type == "MESH" and o.vertex_groups)
P = "mixamorig:"
print("matrizes:", [list(map(lambda v: round(v, 3), r)) for r in arm.matrix_world], [list(map(lambda v: round(v, 3), r)) for r in body.matrix_world])

mid = [arm.matrix_world @ arm.data.bones[P + n].head_local for n in ("Hips", "Spine", "Spine1", "Spine2", "Neck", "Head")]
x0 = sum(p.x for p in mid) / len(mid)
print(f"plano do meio: x = {x0:.4f}")

# ---- malha: corta no meio, tira o lado direito (x < x0), espelha o esquerdo ----
inv = body.matrix_world.inverted()
co0 = inv @ Vector((x0, 0, 0))
no = (inv.to_3x3() @ Vector((1, 0, 0))).normalized()
bm = bmesh.new()
bm.from_mesh(body.data)
bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)
geom = bm.verts[:] + bm.edges[:] + bm.faces[:]
bmesh.ops.bisect_plane(bm, geom=geom, plane_co=co0, plane_no=no, clear_inner=True, dist=1e-5)
for v in bm.verts:  # o corte fica exatamente no plano (a solda do espelho pega)
    if abs((v.co - co0).dot(no)) < 2e-4:
        v.co -= no * (v.co - co0).dot(no)
bm.to_mesh(body.data)
bm.free()

# grupos para o padrão que o Blender sabe espelhar (.L/.R), e de volta depois
ren = {}
for g in body.vertex_groups:
    n = g.name
    if n.startswith(P + "Left"):
        ren[g.name] = P + n[len(P) + 4:] + ".L"
    elif n.startswith(P + "Right"):
        ren[g.name] = P + n[len(P) + 5:] + ".R"
for a, b in ren.items():
    body.vertex_groups[a].name = b

emp = bpy.data.objects.new("plano_meio", None)
bpy.context.scene.collection.objects.link(emp)
emp.location = (x0, 0, 0)
mod = body.modifiers.new("Espelho", "MIRROR")
mod.use_axis = (True, False, False)
mod.mirror_object = emp
mod.use_clip = True
mod.use_mirror_merge = True
mod.merge_threshold = 1e-4
mod.use_mirror_vertex_groups = True
mod.use_mirror_u = False
mod.use_mirror_v = False
# o Mirror precisa vir antes do Armature na pilha
bpy.context.view_layer.objects.active = body
while body.modifiers.find("Espelho") > 0:
    bpy.ops.object.modifier_move_up(modifier="Espelho")
bpy.ops.object.modifier_apply(modifier="Espelho")
bpy.data.objects.remove(emp)
for a, b in ren.items():
    body.vertex_groups[b].name = a
print("vértices:", len(body.data.vertices), "| triângulos:", sum(len(p.vertices) - 2 for p in body.data.polygons))

# ---- esqueleto: Right* = espelho de Left*, o meio no plano ----
bpy.ops.object.select_all(action="DESELECT")
arm.select_set(True)
bpy.context.view_layer.objects.active = arm
bpy.ops.object.mode_set(mode="EDIT")
eb = arm.data.edit_bones
aw = arm.matrix_world
ai = aw.inverted()


def mirror_pt(p):
    w = aw @ p
    w.x = 2 * x0 - w.x
    return ai @ w


for b in eb:
    n = b.name
    if n.startswith(P + "Left"):
        r = eb.get(P + "Right" + n[len(P) + 4:])
        if r:
            r.head = mirror_pt(b.head)
            r.tail = mirror_pt(b.tail)
            r.roll = -b.roll
    elif not n.startswith(P + "Right") and n.startswith(P):
        for end in ("head", "tail"):
            w = aw @ getattr(b, end)
            w.x = x0
            setattr(b, end, ai @ w)
bpy.ops.object.mode_set(mode="OBJECT")

bpy.ops.object.select_all(action="SELECT")
bpy.ops.export_scene.gltf(filepath=dst, export_format="GLB", use_selection=True, export_animations=False, export_apply=False)
print("SALVO", dst)
