"""Velocidade das mãos por quadro de cada clipe (Blender headless), para achar o quadro de impacto e recortar.

    blender --background --factory-startup --python tools/art/remaster-monstros/speeds.py -- <glb> <clipe,clipe|todos>

Imprime, por clipe, a velocidade (m/s, mundo) da mão direita e da esquerda a cada 3 quadros e o quadro de pico.
Laços com limite de quadros."""
import sys
import bpy
import mathutils

argv = sys.argv[sys.argv.index("--") + 1:]
glb = argv[0]
want = argv[1].split(",") if len(argv) > 1 and argv[1] != "todos" else None
bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()
bpy.ops.import_scene.gltf(filepath=glb)
arm = next(o for o in bpy.data.objects if o.type == "ARMATURE")
sc = bpy.context.scene
sc.render.fps = 30
arm.animation_data_create()
for act in list(bpy.data.actions):
    if want and not any(w in act.name for w in want):
        continue
    arm.animation_data.action = act
    s, e = int(act.frame_range[0]), min(int(act.frame_range[1]), 240)
    prev = {}
    rows = []
    for f in range(s, e + 1):
        sc.frame_set(f)
        row = []
        for side in ("RightHand", "LeftHand"):
            b = arm.pose.bones.get("mixamorig:" + side)
            p = arm.matrix_world @ b.matrix @ mathutils.Vector((0, 0, 0))
            v = (p - prev[side]).length * 30 if side in prev else 0.0
            prev[side] = p
            row.append(v)
        hips = arm.pose.bones.get("mixamorig:Hips")
        row.append((arm.matrix_world @ hips.matrix @ mathutils.Vector((0, 0, 0))).z)
        rows.append(row)
    pk = max(range(len(rows)), key=lambda i: rows[i][0] + rows[i][1])
    print(f"## {act.name} quadros {s}-{e} pico no quadro {s + pk} ({(s + pk) / 30:.2f}s)")
    print("   t:maxMão/quadril  " + "  ".join(f"{(s + i) / 30:.1f}:{max(rows[i][0], rows[i][1]):.1f}/{rows[i][2]:.2f}" for i in range(0, len(rows), 4)))
