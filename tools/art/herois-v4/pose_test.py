"""Teste de deformação do rig dos heróis v4: põe o esqueleto `heroi` em poses extremas e renderiza.

Poses: punho fechado nas duas mãos, braço direito erguido, tronco girado, perna esquerda dobrada,
cabeça virada. Gera a vista inteira (frente e 3/4) e a mão esquerda de perto.

    Blender --background --factory-startup corpo.blend --python tools/art/herois-v4/pose_test.py -- saida-prefixo
"""
import math
import sys

import bpy
from mathutils import Euler, Vector

out = sys.argv[sys.argv.index("--") + 1]
arm = bpy.data.objects["heroi"]
LOD = sys.argv[sys.argv.index("--") + 2] if len(sys.argv) > sys.argv.index("--") + 2 else ""
for o in bpy.data.objects:  # mostra só um nível de detalhe (padrão: o LOD0, "corpo")
    if o.type == "MESH":
        o.hide_render = o.name != ("corpo" + (f"_{LOD}" if LOD else ""))
pb = arm.pose.bones


from mathutils import Matrix


def rot(name, x=0, y=0, z=0):
    """Giro local (eixos do osso): usado nos dedos, onde X dobra para a palma."""
    if name in pb:
        b = pb[name]
        b.rotation_mode = "XYZ"
        b.rotation_euler = Euler((math.radians(x), math.radians(y), math.radians(z)))
        bpy.context.view_layer.update()


def wrot(name, axis, deg):
    """Giro em torno de um eixo do mundo, pela cabeça do osso."""
    b = pb[name]
    axis = Vector(axis) if not isinstance(axis, str) else axis
    piv = b.head.copy()
    b.matrix = Matrix.Translation(piv) @ Matrix.Rotation(math.radians(deg), 4, axis) @ Matrix.Translation(-piv) @ b.matrix
    bpy.context.view_layer.update()


for s in ("l", "r"):
    for k in (1, 2, 3):
        rot(f"index_{k:02d}_{s}", x=70)
        rot(f"thumb_{k:02d}_{s}", x=30)
    for k in (1, 2):
        rot(f"fingers_{k:02d}_{s}", x=85)
_d = (pb["upperarm_r"].tail - pb["upperarm_r"].head).normalized()
wrot("upperarm_r", _d.cross(Vector((0, -1, 0))).normalized(), 100)  # braço direito para a frente e para cima, como num golpe
_d = (pb["forearm_r"].tail - pb["forearm_r"].head).normalized()
wrot("forearm_r", _d.cross(Vector((0, -1, 0))).normalized(), 60)  # cotovelo dobrado
wrot("upperarm_l", "Y", -30)    # braço esquerdo mais aberto
wrot("spine_02", "Z", 15)
wrot("chest", "Z", 12)
wrot("thigh_l", "X", 55)        # joelho para a frente
wrot("calf_l", "X", -80)        # canela para trás
wrot("head", "Z", -25)
bpy.context.view_layer.update()

sc = bpy.context.scene
sc.render.engine = "BLENDER_WORKBENCH"
sc.display.shading.light = "STUDIO"
sc.display.shading.color_type = "TEXTURE"
sc.world = bpy.data.worlds.new("w")
sc.world.color = (0.45, 0.45, 0.45)
cam = bpy.data.objects.new("cam", bpy.data.cameras.new("cam"))
sc.collection.objects.link(cam)
sc.camera = cam
cam.data.type = "ORTHO"


def shot(path, center, direction, scale, res=900):
    sc.render.resolution_x = sc.render.resolution_y = res
    cam.data.ortho_scale = scale
    d = Vector(direction).normalized()
    cam.location = Vector(center) - d * 3
    cam.rotation_euler = d.to_track_quat("-Z", "Z" if abs(d.z) > 0.9 else "Y").to_euler()
    sc.render.filepath = path
    bpy.ops.render.render(write_still=True)


shot(f"{out}-frente.png", (0, 0, 0.95), (0, 1, 0), 2.1)
shot(f"{out}-34.png", (0, 0, 0.95), (0.7, 1, -0.1), 2.1)
hand = arm.matrix_world @ pb["hand_l"].head
shot(f"{out}-mao.png", hand + Vector((0.03, 0, -0.06)), (0, 1, 0), 0.3, 600)
hand = arm.matrix_world @ pb["hand_r"].head
shot(f"{out}-mao2.png", hand + Vector((0, -0.04, 0)), (0.3, 1, 0), 0.34, 600)
print("OK")
