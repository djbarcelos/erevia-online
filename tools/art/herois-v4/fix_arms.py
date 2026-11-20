"""Corpo v4: leva os braços para o plano do corpo (A-pose de frente) e aplica como pose de repouso.

O corpo do Meshy (multi-image) veio com os braços girados no eixo vertical (um para a frente,
outro para trás). O rig do Meshy dá pesos bons; aqui cada braço gira em torno do ombro, no eixo Z
do mundo, até a mão ficar na mesma profundidade (Y) do ombro. Depois a pose vira o repouso.

    Blender --background --factory-startup --python tools/art/herois-v4/fix_arms.py -- entrada.glb saida.glb
"""
import math
import sys

import bpy
from mathutils import Matrix, Vector

src, dst = sys.argv[sys.argv.index("--") + 1:][:2]
bpy.ops.wm.read_factory_settings(use_empty=True)
bpy.ops.import_scene.gltf(filepath=src)
arm = next(o for o in bpy.data.objects if o.type == "ARMATURE")
mesh = next(o for o in bpy.data.objects if o.type == "MESH" and o.vertex_groups)
for o in list(bpy.data.objects):  # a esfera de referência do Meshy não serve
    if o.type == "MESH" and not o.vertex_groups:
        me = o.data
        bpy.data.objects.remove(o)
        bpy.data.meshes.remove(me)
for pb in arm.pose.bones:
    pb.custom_shape = None

bpy.context.view_layer.objects.active = arm
bpy.ops.object.mode_set(mode="POSE")
mw = arm.matrix_world
def bounds():
    deps = bpy.context.evaluated_depsgraph_get()
    ev = mesh.evaluated_get(deps)
    vs = [mesh.matrix_world @ v.co for v in ev.data.vertices]
    return [round(min(v[i] for v in vs), 3) for i in range(3)], [round(max(v[i] for v in vs), 3) for i in range(3)]

print("antes", bounds())
for side, target in (("Left", 0.0), ("Right", math.pi)):
    upper = arm.pose.bones[f"mixamorig:{side}Arm"]
    hand = arm.pose.bones[f"mixamorig:{side}Hand"]
    for _ in range(3):  # converge (o setter de matrix arredonda pela pose dos filhos)
        s = mw @ upper.head
        h = mw @ hand.head
        phi = math.atan2(h.y - s.y, h.x - s.x)
        d = (target - phi + math.pi) % (2 * math.pi) - math.pi
        pivot = upper.head.copy()
        upper.matrix = Matrix.Translation(pivot) @ Matrix.Rotation(d, 4, "Z") @ Matrix.Translation(-pivot) @ upper.matrix
        bpy.context.view_layer.update()
    # e o ângulo do braço: 45° abaixo da horizontal (A-pose do contrato)
    for _ in range(3):
        s = mw @ upper.head
        h = mw @ hand.head
        horiz = abs(h.x - s.x)
        elev = math.atan2(h.z - s.z, horiz)
        d = math.radians(-45) - elev
        axis = "Y"
        ang = -d if side == "Left" else d
        pivot = upper.head.copy()
        upper.matrix = Matrix.Translation(pivot) @ Matrix.Rotation(ang, 4, axis) @ Matrix.Translation(-pivot) @ upper.matrix
        bpy.context.view_layer.update()
    s = mw @ upper.head
    h2 = mw @ hand.head
    print(f"{side}: mão {tuple(round(x, 3) for x in h2)}, ombro {tuple(round(x, 3) for x in s)}, elevação {math.degrees(math.atan2(h2.z - s.z, abs(h2.x - s.x))):.1f}°")
print("depois", bounds())

# aplica a pose na malha e como novo repouso
bpy.ops.object.mode_set(mode="OBJECT")
bpy.context.view_layer.objects.active = mesh
mod = next(m for m in mesh.modifiers if m.type == "ARMATURE")
bpy.ops.object.modifier_apply(modifier=mod.name)
bpy.context.view_layer.objects.active = arm
bpy.ops.object.mode_set(mode="POSE")
bpy.ops.pose.armature_apply(selected=False)
bpy.ops.object.mode_set(mode="OBJECT")
m = mesh.modifiers.new("Armature", "ARMATURE")
m.object = arm

print("final", bounds())
bpy.ops.export_scene.gltf(filepath=dst, export_format="GLB", export_animations=False)
print("SALVO", dst)
