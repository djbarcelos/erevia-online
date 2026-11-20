"""Renderiza um ou mais GLB de frente, de lado (direita), de costas e de 3/4, numa tira só.

    /Applications/Blender.app/Contents/MacOS/Blender --background --factory-startup \
        --python tools/art/herois-v4/render_views.py -- saida.png modelo1.glb [modelo2.glb ...] [--wire] [--solid]

--solid usa o Workbench sem textura (forma pura); --wire sobrepõe a malha. Sem flags: textura com luz plana.
Imprime triângulos, altura e o tamanho da caixa de cada modelo.
"""
import math
import sys

import bpy
from mathutils import Vector

argv = sys.argv[sys.argv.index("--") + 1:]
flags = {a for a in argv if a.startswith("--")}
out, files = argv[0], [a for a in argv[1:] if not a.startswith("--")]

bpy.ops.wm.read_factory_settings(use_empty=True)
scene = bpy.context.scene
scene.render.engine = "BLENDER_WORKBENCH"
sh = scene.display.shading
sh.light = "FLAT" if "--solid" not in flags else "STUDIO"
sh.color_type = "TEXTURE" if "--solid" not in flags else "SINGLE"
sh.single_color = (0.75, 0.72, 0.68)
sh.show_object_outline = False
scene.display.render_aa = "8"
if "--wire" in flags:
    sh.show_wireframes = True if hasattr(sh, "show_wireframes") else False
scene.world = bpy.data.worlds.new("w")
scene.world.color = (0.5, 0.5, 0.5)
scene.render.film_transparent = False

VIEWS = [("frente", 0), ("lado", 90), ("costas", 180), ("3/4", 35)]
CELL = 512
scene.render.resolution_x = CELL * len(VIEWS) * max(1, len(files))
scene.render.resolution_y = CELL

offset = 0.0
roots = []
for f in files:
    before = set(bpy.data.objects)
    bpy.ops.import_scene.gltf(filepath=f)
    objs = [o for o in bpy.data.objects if o not in before]
    meshes = [o for o in objs if o.type == "MESH"]
    tris = sum(sum(len(p.vertices) - 2 for p in o.data.polygons) for o in meshes)
    pts = [o.matrix_world @ Vector(c) for o in meshes for c in o.bound_box]
    lo = Vector((min(p.x for p in pts), min(p.y for p in pts), min(p.z for p in pts)))
    hi = Vector((max(p.x for p in pts), max(p.y for p in pts), max(p.z for p in pts)))
    print(f"MODELO {f}: {tris} triângulos, altura {hi.z - lo.z:.3f}, caixa {hi - lo}")
    roots.append((objs, lo, hi))

# uma cópia de cada modelo por vista, girada, lado a lado
height = max(hi.z - lo.z for _, lo, hi in roots)
step = height * 0.62
x = 0.0
for objs, lo, hi in roots:
    center = (lo + hi) / 2
    for i, (_, ang) in enumerate(VIEWS):
        empty = bpy.data.objects.new("v", None)
        scene.collection.objects.link(empty)
        empty.location = (x, 0, -lo.z)
        empty.rotation_euler = (0, 0, math.radians(ang))
        for o in objs:
            if o.parent is None:
                c = o.copy()
                if o.data:
                    c.data = o.data
                scene.collection.objects.link(c)
                c.location = (o.location.x - center.x, o.location.y - center.y, o.location.z)
                c.parent = empty
                for ch in o.children_recursive:
                    cc = ch.copy()
                    scene.collection.objects.link(cc)
                    cc.parent = c
        x += step
    for o in objs:
        o.hide_render = True

cam = bpy.data.objects.new("cam", bpy.data.cameras.new("cam"))
scene.collection.objects.link(cam)
cam.data.type = "ORTHO"
cam.data.ortho_scale = max(x, height * 1.1)
cam.location = ((x - step) / 2, -10, height / 2)
cam.rotation_euler = (math.radians(90), 0, 0)
scene.camera = cam
scene.render.resolution_x = int(CELL * x / step)
scene.render.resolution_y = int(CELL * height * 1.1 / step)
cam.data.ortho_scale = max(x, height * 1.1)
scene.render.filepath = out
bpy.ops.render.render(write_still=True)
print("SALVO", out)
