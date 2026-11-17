"""Ângulos de uma arma nova ao lado da KayKit equivalente (Blender headless).

    blender --background --factory-startup --python tools/art/remaster-estaticos/render.py -- \
        <novo.glb> <kaykit.glb> <saida.jpg>

Esquerda: a arma nova; direita: a KayKit na mesma origem (deslocada 1,6 m em X). Três vistas (frente, 3/4, lado).
"""
import math
import os
import sys

import bpy
import mathutils

new, old, out = sys.argv[sys.argv.index("--") + 1:][:3]
bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()


def load(path, dx):
    before = set(bpy.data.objects)
    bpy.ops.import_scene.gltf(filepath=path)
    objs = [o for o in bpy.data.objects if o not in before]
    for o in objs:
        if o.parent is None:
            o.location.x += dx
    return objs


load(new, 0.0)
load(old, 1.6)
sc = bpy.context.scene
sc.render.engine = next(e for e in ("BLENDER_EEVEE_NEXT", "BLENDER_EEVEE", "CYCLES") if e in [i.identifier for i in bpy.types.RenderSettings.bl_rna.properties["engine"].enum_items])
sc.render.resolution_x, sc.render.resolution_y = 420, 520
sc.render.image_settings.file_format = "JPEG"
sc.world = bpy.data.worlds.new("w")
sc.world.use_nodes = True
bg = next(n for n in sc.world.node_tree.nodes if n.type == "BACKGROUND")
bg.inputs[0].default_value = (0.78, 0.8, 0.84, 1)
bg.inputs[1].default_value = 1.6
sun = bpy.data.objects.new("sun", bpy.data.lights.new("sun", "SUN"))
sun.data.energy = 3.0
sun.rotation_euler = (math.radians(50), 0, math.radians(30))
sc.collection.objects.link(sun)
cam = bpy.data.objects.new("cam", bpy.data.cameras.new("cam"))
cam.data.type = "ORTHO"
cam.data.ortho_scale = 3.6
sc.collection.objects.link(cam)
sc.camera = cam
views = {"frente": (0.8, -6, 0.75, 90, 0, 0), "tres-quartos": (4.5, -4.5, 3.2, 70, 0, 45), "lado": (6.8, 0, 0.75, 90, 0, 90)}
tmp = []
for i, (name, (x, y, z, rx, ry, rz)) in enumerate(views.items()):
    cam.location = (x, y, z)
    cam.rotation_euler = tuple(math.radians(a) for a in (rx, ry, rz))
    p = out.replace(".jpg", f"-{i}.jpg")
    sc.render.filepath = p
    bpy.ops.render.render(write_still=True)
    tmp.append(p)
print("\n".join(tmp))
