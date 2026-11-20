"""Heróis v4: instante do impacto de cada golpe, pelo pico de velocidade da mão (a ponta do golpe).

Para cada clipe=mão (r ou l), acha o quadro em que a mão anda mais rápido e grava em clips.json ("impact",
em quadros a 30 fps). O cliente usa esse instante (CLIP_IMPACT) para o dano aparecer junto do golpe.

    Blender --background --factory-startup base.blend --python tools/art/herois-v4/impacts.py -- clips.json clipe=r [clipe=l ...]
"""
import json
import sys

import bpy

args = sys.argv[sys.argv.index("--") + 1:]
path = args[0]
clips = json.load(open(path))
arm = bpy.data.objects["heroi"]
sc = bpy.context.scene
for spec in args[1:]:
    name, side = spec.split("=")
    act = bpy.data.actions[name]
    arm.animation_data_create()
    arm.animation_data.action = act
    try:
        arm.animation_data.action_slot = arm.animation_data.action_suitable_slots[0]
    except (AttributeError, IndexError):
        pass
    n = int(act.frame_range[1])
    prev, best = None, (0, 0)
    for f in range(n + 1):
        sc.frame_set(f)
        p = arm.matrix_world @ arm.pose.bones[f"hand_{side}"].tail
        if prev is not None:
            v = (p - prev).length
            if v > best[0]:
                best = (v, f)
        prev = p
    clips.setdefault(name, {})["impact"] = [best[1]]
    print("IMPACTO", name, best[1], f"{best[1] / 30:.2f}s", "de", n)
json.dump(clips, open(path, "w"), indent=1)
