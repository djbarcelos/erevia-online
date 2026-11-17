"""Distância entre as mãos em cada clipe (as armas de duas mãos pedem as duas no cabo).

    blender --background --factory-startup --python tools/art/remaster-herois/hands.py -- <glb> [<clipe> ...]
"""
import sys

import bpy

argv = sys.argv[sys.argv.index("--") + 1:]
glb, only = argv[0], argv[1:]
bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()
bpy.ops.import_scene.gltf(filepath=glb)
arm = next(o for o in bpy.data.objects if o.type == "ARMATURE")
sc = bpy.context.scene
arm.animation_data_create()
for act in bpy.data.actions:
    if only and act.name not in only:
        continue
    arm.animation_data.action = act
    s, e = int(act.frame_range[0]), int(act.frame_range[1])
    ds = []
    for f in range(s, min(e, s + 600) + 1, 3):
        sc.frame_set(f)
        r = arm.matrix_world @ arm.pose.bones["mixamorig:RightHand"].head
        l = arm.matrix_world @ arm.pose.bones["mixamorig:LeftHand"].head
        ds.append((r - l).length)
    print(f"MAOS {act.name}: {(e - s) / 30:.1f} s, distância entre as mãos {min(ds):.2f}..{max(ds):.2f} m (média {sum(ds) / len(ds):.2f})")
