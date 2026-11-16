"""Confere um GLB do remaster v3 no Blender (headless) e gera as folhas de prints.

    blender --background --factory-startup --python tools/art/remaster-v3/render.py -- \
        <arquivo.glb> <pasta de saída> <prefixo> [angles|clips|stats]

- stats: por clipe, altura do quadril, pé mais baixo e deslizamento do pé (para ver "agachado",
  quadril caído e pés escorregando).
- angles: frente, 3/4, lado e costas na pose parada (primeiro quadro do Idle).
- clips: uma tira por clipe (8 quadros, vista de lado), cada uma em <prefixo>-clip-<nome>.jpg.
Laços sempre com limite de iterações.
"""
import math
import os
import sys

import bpy
import mathutils

argv = sys.argv[sys.argv.index("--") + 1:]
glb, out, prefix = argv[0], argv[1], argv[2]
mode = argv[3] if len(argv) > 3 else "angles"
os.makedirs(out, exist_ok=True)

bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()
bpy.ops.import_scene.gltf(filepath=glb)
arm = next(o for o in bpy.data.objects if o.type == "ARMATURE")
mesh = next(o for o in bpy.data.objects if o.type == "MESH" and o.parent == arm)
sc = bpy.context.scene
sc.render.fps = 30
actions = list(bpy.data.actions)
# o importador cria uma action por clipe; liga uma de cada vez
def play(act):
    arm.animation_data_create()
    arm.animation_data.action = act
    s, e = act.frame_range
    sc.frame_start, sc.frame_end = int(s), int(e)
    return int(s), int(e)

def bone_world(name, frame):
    sc.frame_set(frame)
    b = arm.pose.bones[name]
    return arm.matrix_world @ b.head

def feet(frame):
    sc.frame_set(frame)
    return [(arm.matrix_world @ arm.pose.bones[n].head) for n in ("mixamorig:LeftFoot", "mixamorig:RightFoot", "mixamorig:LeftToeBase", "mixamorig:RightToeBase")]

def mesh_bounds():
    bpy.context.view_layer.update()
    dg = bpy.context.evaluated_depsgraph_get()
    ev = mesh.evaluated_get(dg)
    vs = [ev.matrix_world @ v.co for v in ev.data.vertices]
    return min(v.z for v in vs), max(v.z for v in vs)

if mode == "stats":
    for act in actions:
        s, e = play(act)
        hz, minfoot = [], []
        steps = list(range(s, e + 1, max(1, (e - s) // 12)))[:40]
        for f in steps:
            hz.append(bone_world("mixamorig:Hips", f).z)
            minfoot.append(min(p.z for p in feet(f)))
        z0, z1 = mesh_bounds()
        print(f"{act.name}: frames {s}-{e} ({(e - s) / 30:.2f}s) quadril z {min(hz):.3f}..{max(hz):.3f} pé mais baixo {min(minfoot):.3f}..{max(minfoot):.3f} malha z {z0:.2f}..{z1:.2f}")
    sys.exit(0)

# --- cena de prova ---
world = bpy.data.worlds.new("w")
sc.world = world
world.use_nodes = True
bg = next(n for n in world.node_tree.nodes if n.type == "BACKGROUND")
bg.inputs[0].default_value = (0.82, 0.84, 0.88, 1)
sc.render.engine = "BLENDER_WORKBENCH"
sh = sc.display.shading
sh.light = "STUDIO"
sh.color_type = "TEXTURE"
sh.show_shadows = False
sc.render.image_settings.file_format = "PNG"
sc.render.film_transparent = False
z0, z1 = mesh_bounds()
h = z1 - z0
# chão
bpy.ops.mesh.primitive_plane_add(size=6, location=(0, 0, 0))
plane = bpy.context.active_object
bpy.ops.object.select_all(action="DESELECT")

cam = bpy.data.cameras.new("c")
cam.type = "ORTHO"
cam.ortho_scale = max(h * 1.35, 2.0)
camo = bpy.data.objects.new("c", cam)
sc.collection.objects.link(camo)
sc.camera = camo

def aim(direction, center):
    d = mathutils.Vector(direction).normalized()
    camo.location = center + d * 8
    camo.rotation_euler = (-d).to_track_quat("-Z", "Y").to_euler()

def shot(path, w=320, hgt=427):
    sc.render.resolution_x, sc.render.resolution_y = w, hgt
    sc.render.filepath = path
    bpy.ops.render.render(write_still=True)
    img = bpy.data.images.load(path)
    img.reload()  # o Blender guarda a imagem pelo caminho: sem isso todo quadro sai igual ao primeiro
    return img

def sheet(images, cols, path):
    import numpy as np
    w, hh = images[0].size
    rows = (len(images) + cols - 1) // cols
    canvas = np.ones((rows * hh, cols * w, 4), dtype=np.float32)
    for i, img in enumerate(images):
        px = np.array(img.pixels[:], dtype=np.float32).reshape(hh, w, 4)
        r, c = divmod(i, cols)
        canvas[(rows - 1 - r) * hh:(rows - r) * hh, c * w:(c + 1) * w] = px
    out_img = bpy.data.images.new("sheet", cols * w, rows * hh)
    out_img.pixels = canvas.ravel().tolist()
    out_img.filepath_raw = path
    out_img.file_format = "JPEG"
    out_img.save()

center = mathutils.Vector((0, 0, h / 2))
idle = next((a for a in actions if a.name.endswith("Idle")), actions[0])
s, e = play(idle)
sc.frame_set(s)
if mode == "angles":
    imgs = []
    for name, d in (("front", (0, -1, 0)), ("tq", (-0.7, -0.7, 0.15)), ("side", (-1, 0, 0)), ("back", (0, 1, 0))):
        aim(d, center)
        imgs.append(shot(os.path.join(out, f"_{name}.png")))
    sheet(imgs, 4, os.path.join(out, f"{prefix}-angulos.jpg"))
else:
    for act in actions:
        s, e = play(act)
        n = 7
        frames = [s + round(i * (e - s) / (n - 1)) for i in range(n)]
        imgs = []
        for f in frames:
            sc.frame_set(f)
            hips = bone_world("mixamorig:Hips", f)
            aim((-1, 0, 0), mathutils.Vector((hips.x, hips.y, h / 2)))  # câmera segue o quadril em x/y
            imgs.append(shot(os.path.join(out, f"_f{f}.png"), 160, 240))
        sheet(imgs, 7, os.path.join(out, f"{prefix}-clip-{act.name}.jpg"))
for f in os.listdir(out):
    if f.startswith("_"):
        os.remove(os.path.join(out, f))
