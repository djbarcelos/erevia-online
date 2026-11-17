"""Instante do impacto do golpe (CLIP_IMPACT): o fim do movimento rápido do osso que bate.

    blender --background --factory-startup --python tools/art/familias-monstro/impact.py -- <arquivo.glb> <prefixo> <Clipe> <osso>

Anda o clipe quadro a quadro, mede a velocidade do osso e devolve o primeiro quadro, depois do pico de velocidade, em que
ela cai abaixo de 30% do pico (o golpe encosta). Laço limitado ao tamanho do clipe.
"""
import sys

import bpy

argv = sys.argv[sys.argv.index("--") + 1:]
glb, prefix, clip, bone = argv[0], argv[1], argv[2], argv[3]
bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()
bpy.ops.import_scene.gltf(filepath=glb)
sc = bpy.context.scene
FPS = sc.render.fps / sc.render.fps_base
arm = next(o for o in bpy.data.objects if o.type == "ARMATURE")
arm.animation_data_create()
a = bpy.data.actions[f"{prefix}{clip}"]
arm.animation_data.action = a
s, e = int(a.frame_range[0]), int(a.frame_range[1])
pos = []
for f in range(s, min(e, s + 400) + 1):
    sc.frame_set(f)
    bpy.context.view_layer.update()
    pos.append(arm.matrix_world @ arm.pose.bones[bone].tail)
sp = [((pos[i + 1] - pos[i]).length) for i in range(len(pos) - 1)]
peak = max(range(len(sp)), key=lambda i: sp[i])
imp = next((i for i in range(peak, len(sp)) if sp[i] < 0.3 * sp[peak]), len(sp) - 1)
print("IMPACTO", prefix + clip, "osso", bone, "pico em", round(peak / FPS, 2), "s; impacto em", round((imp + 1) / FPS, 2), "s de", round((e - s) / FPS, 2))
