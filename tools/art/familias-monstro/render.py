"""Folha das quatro famílias no Blender (headless, Workbench com textura): as malhas finais lado a lado, de frente e de 3/4.

    blender --background --factory-startup --python tools/art/familias-monstro/render.py -- <pasta com os *_raw.glb> <pasta de saída>

Mostra o que foi montado (rig + textura do retexture) antes do jogo: o que se vê aqui é o que o import-asset recebe.
"""
import math
import os
import sys

import bpy
import mathutils

src, out = sys.argv[sys.argv.index("--") + 1:][:2]
os.makedirs(out, exist_ok=True)
bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()
sc = bpy.context.scene
ids = ["wolf", "spider", "slime", "frog"]
heights = {"wolf": 1.0, "spider": 0.5, "slime": 0.8, "frog": 1.1}  # as mesmas alturas do REMASTER_HEIGHTS
x = 0.0
for i in ids:
    before = set(bpy.data.objects)
    bpy.ops.import_scene.gltf(filepath=os.path.join(src, f"{i}_raw.glb"))
    new = [o for o in bpy.data.objects if o not in before]
    meshes = [o for o in new if o.type == "MESH"]
    arm = next(o for o in new if o.type == "ARMATURE")
    bpy.context.view_layer.update()
    zs = [(o.matrix_world @ mathutils.Vector(c)).z for o in meshes for c in o.bound_box]
    k = heights[i] / (max(zs) - min(zs))
    top = [o for o in new if o.parent is None]
    for o in top:
        o.scale = (o.scale[0] * k, o.scale[1] * k, o.scale[2] * k)
    bpy.context.view_layer.update()
    zs = [(o.matrix_world @ mathutils.Vector(c)).z for o in meshes for c in o.bound_box]
    for o in top:
        o.location.z -= min(zs)
        o.location.x += x
    x += 2.4 if i == "wolf" else 1.8
sc.render.engine = "BLENDER_WORKBENCH"
sc.display.shading.light = "STUDIO"
sc.display.shading.color_type = "TEXTURE"
sc.display.shading.show_shadows = True
sc.render.resolution_x, sc.render.resolution_y = 1280, 480
sc.render.image_settings.file_format = "JPEG"
sc.render.image_settings.quality = 85
sc.world = bpy.data.worlds.new("w")
sc.world.color = (0.55, 0.63, 0.72)
cam = bpy.data.objects.new("cam", bpy.data.cameras.new("cam"))
sc.collection.objects.link(cam)
sc.camera = cam
cam.data.lens = 50
mid = (x - 1.8) / 2
for name, az in (("frente", 0.0), ("tres-quartos", 40.0)):
    a = math.radians(az)
    dist = 11
    cam.location = (mid + dist * math.sin(a), -dist * math.cos(a), 0.9)
    target = mathutils.Vector((mid, 0, 0.45))
    d = target - cam.location
    cam.rotation_euler = d.to_track_quat("-Z", "Y").to_euler()
    sc.render.filepath = os.path.join(out, f"blender-{name}.jpg")
    bpy.ops.render.render(write_still=True)
    print("OK", sc.render.filepath)
