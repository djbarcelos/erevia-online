"""Acha o instante do golpe de cada clipe de ataque: o fim do movimento rápido das mãos.

    blender --background --factory-startup --python tools/art/remaster-herois/impact.py -- <glb> <clipe> [<clipe> ...]

Para cada clipe imprime o quadro (30 fps) do pico de velocidade da mão e o primeiro quadro depois do
pico em que a velocidade cai abaixo de 35% dele (o "impacto" que o adaptador usa). Laços limitados.
"""
import sys

import bpy

argv = sys.argv[sys.argv.index("--") + 1:]
glb, clips = argv[0], argv[1:]
bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()
bpy.ops.import_scene.gltf(filepath=glb)
arm = next(o for o in bpy.data.objects if o.type == "ARMATURE")
sc = bpy.context.scene
arm.animation_data_create()
for name in clips:
    act = next((a for a in bpy.data.actions if a.name == name), None)
    if not act:
        print("IMPACTO", name, "não achei")
        continue
    arm.animation_data.action = act
    s, e = int(act.frame_range[0]), int(act.frame_range[1])
    prev, speeds = None, []
    for f in range(s, min(e, s + 400) + 1):
        sc.frame_set(f)
        pos = [(arm.matrix_world @ arm.pose.bones[b].head) for b in ("mixamorig:RightHand", "mixamorig:LeftHand")]
        if prev:
            speeds.append(max((p - q).length for p, q in zip(pos, prev)) * 30)
        else:
            speeds.append(0.0)
        prev = pos
    peak = max(range(len(speeds)), key=lambda i: speeds[i])
    after = next((i for i in range(peak, len(speeds)) if speeds[i] < 0.35 * speeds[peak]), len(speeds) - 1)
    print(f"IMPACTO {name}: {len(speeds)} quadros, pico {speeds[peak]:.1f} m/s no quadro {peak}, impacto no quadro {after} ({after / 30:.2f} s)")
