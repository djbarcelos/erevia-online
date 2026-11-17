"""Prova de uma peça estática do kit da cidade no Blender (headless): medidas, triângulos e 4 vistas.

    blender --background --factory-startup --python tools/art/remaster-cidade/piece.py -- \
        <pasta de saída> <arquivo.glb>[,<arquivo.glb>...] [largura-por-vista]

Para cada GLB grava <saída>/<nome>.jpg (frente, 3/4, lado e costas, ortográfico, textura do GLB,
luz de estúdio) e imprime as dimensões (X largura, Y altura, Z fundo no espaço glTF) e os triângulos.
Laços com limite de iterações.
"""
import math
import os
import sys

import bpy
import mathutils
import numpy as np

argv = sys.argv[sys.argv.index("--") + 1:]
out = argv[0]
files = argv[1].split(",")
W = int(argv[2]) if len(argv) > 2 else 320
os.makedirs(out, exist_ok=True)
sc = bpy.context.scene
sc.render.engine = "BLENDER_WORKBENCH"
sh = sc.display.shading
sh.light = "STUDIO"
sh.color_type = "TEXTURE"
sh.show_shadows = True
sh.show_cavity = True
sc.render.image_settings.file_format = "PNG"
w = bpy.data.worlds.new("w")
sc.world = w
w.use_nodes = True
next(n for n in w.node_tree.nodes if n.type == "BACKGROUND").inputs[0].default_value = (0.82, 0.84, 0.88, 1)

cam = bpy.data.cameras.new("c")
cam.type = "ORTHO"
camo = bpy.data.objects.new("c", cam)
sc.collection.objects.link(camo)
sc.camera = camo


def clear():
    for o in list(bpy.data.objects):
        if o.type == "MESH":
            bpy.data.objects.remove(o, do_unlink=True)


def bounds(objs):
    vs = [o.matrix_world @ mathutils.Vector(c) for o in objs for c in o.bound_box]
    lo = mathutils.Vector((min(v.x for v in vs), min(v.y for v in vs), min(v.z for v in vs)))
    hi = mathutils.Vector((max(v.x for v in vs), max(v.y for v in vs), max(v.z for v in vs)))
    return lo, hi


for f in files:
    clear()
    bpy.ops.import_scene.gltf(filepath=f)
    objs = [o for o in bpy.data.objects if o.type == "MESH"]
    tris = sum(len(p.vertices) - 2 for o in objs for p in o.data.polygons)
    lo, hi = bounds(objs)
    # espaço glTF: X = Blender X, Y(alto) = Blender Z, Z(fundo) = -Blender Y
    print(f"PECA {os.path.basename(f)}: tris {tris} largura {hi.x - lo.x:.2f} altura {hi.z - lo.z:.2f} fundo {hi.y - lo.y:.2f} minY {lo.z:.2f} centro x {(hi.x + lo.x) / 2:.2f} z {-(hi.y + lo.y) / 2:.2f}")
    c = (lo + hi) / 2
    size = max(hi.x - lo.x, hi.y - lo.y, hi.z - lo.z)
    cam.ortho_scale = size * 1.25
    imgs = []
    for name, d in (("frente", (0, -1, 0.1)), ("tq", (-0.7, -0.7, 0.45)), ("lado", (-1, 0, 0.1)), ("costas", (0, 1, 0.1))):
        dv = mathutils.Vector(d).normalized()
        camo.location = c + dv * (size * 4)
        camo.rotation_euler = (-dv).to_track_quat("-Z", "Y").to_euler()
        sc.render.resolution_x, sc.render.resolution_y = W, int(W * 0.85)
        p = os.path.join(out, f"_{name}.png")
        sc.render.filepath = p
        bpy.ops.render.render(write_still=True)
        img = bpy.data.images.load(p)
        img.reload()
        imgs.append(img)
    w_, h_ = imgs[0].size
    canvas = np.ones((h_ * 2, w_ * 2, 4), dtype=np.float32)
    for i, img in enumerate(imgs[:4]):
        px = np.array(img.pixels[:], dtype=np.float32).reshape(h_, w_, 4)
        r, col = divmod(i, 2)
        canvas[(1 - r) * h_:(2 - r) * h_, col * w_:(col + 1) * w_] = px
    o = bpy.data.images.new("sheet", w_ * 2, h_ * 2)
    o.pixels = canvas.ravel().tolist()
    o.filepath_raw = os.path.join(out, os.path.splitext(os.path.basename(f))[0] + ".jpg")
    o.file_format = "JPEG"
    o.save()
    for n in ("frente", "tq", "lado", "costas"):
        os.remove(os.path.join(out, f"_{n}.png"))
