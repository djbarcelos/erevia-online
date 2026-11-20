"""Tira de quadros de um clipe (lado e frente), para conferir a animação a olho.

    Blender --background --factory-startup anim.blend --python tools/art/herois-v4/strip.py -- clipe saida.png [quadros]
"""
import sys

import bpy
from mathutils import Vector

a = sys.argv[sys.argv.index("--") + 1:]
clip, out = a[0], a[1]
n = int(a[2]) if len(a) > 2 else 8
arm = bpy.data.objects["heroi"]
act = bpy.data.actions[clip]
arm.animation_data_create()
arm.animation_data.action = act
f0, f1 = (int(x) for x in act.frame_range)
for o in bpy.data.objects:
    if o.type == "MESH":
        o.hide_render = o.name != "corpo"
sc = bpy.context.scene
sc.render.engine = "BLENDER_WORKBENCH"
sc.display.shading.light = "STUDIO"
sc.display.shading.color_type = "TEXTURE"
sc.world = bpy.data.worlds.new("w")
sc.world.color = (0.45, 0.45, 0.45)
sc.render.resolution_x, sc.render.resolution_y = 300, 420
cam = bpy.data.objects.new("c", bpy.data.cameras.new("c"))
sc.collection.objects.link(cam)
sc.camera = cam
cam.data.type = "ORTHO"
cam.data.ortho_scale = 2.1
import os
frames = []
for view, d in (("lado", Vector((1, 0, 0))), ("frente", Vector((0, 1, 0)))):
    cam.location = Vector((0, 0, 0.95)) - d * 3
    cam.rotation_euler = d.to_track_quat("-Z", "Y").to_euler()
    for k in range(n):
        f = f0 + round((f1 - f0) * k / n)
        sc.frame_set(f)
        p = f"{out}.{view}.{k:02d}.png"
        sc.render.filepath = p
        bpy.ops.render.render(write_still=True)
        frames.append(p)
print("QUADROS", len(frames))
