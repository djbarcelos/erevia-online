"""Heróis v4: troca as malhas do corpo pelas de um kit, mantendo esqueleto e animações (base.blend).

Tira `corpo`, `corpo_LOD1`, `corpo_LOD2` e `corpo_alta` do base.blend aberto, traz as do kit (kit-lod.blend,
saída do make_lods.py sobre o fit_kit.py) presas ao mesmo `heroi`, e salva como outro .blend para exportar.

    Blender --background --factory-startup base.blend --python tools/art/herois-v4/swap_kit.py -- kit-lod.blend saida.blend
"""
import sys

import bpy

kit_blend, out_blend = sys.argv[sys.argv.index("--") + 1:][:2]
arm = bpy.data.objects["heroi"]
NAMES = ("corpo", "corpo_LOD1", "corpo_LOD2", "corpo_alta")
for n in NAMES:
    if n in bpy.data.objects:
        bpy.data.objects.remove(bpy.data.objects[n])
with bpy.data.libraries.load(kit_blend, link=False) as (src, dst):
    dst.objects = [n for n in src.objects if n in NAMES]
for o in dst.objects:
    if o is None:
        continue
    bpy.context.scene.collection.objects.link(o)
    o.parent = arm
    for m in o.modifiers:
        if m.type == "ARMATURE":
            m.object = arm
    if o.name == "corpo_alta":
        o.hide_render = True
        o.hide_set(True)
    print("kit:", o.name, sum(len(p.vertices) - 2 for p in o.data.polygons), "triângulos", [m.name for m in o.data.materials])
bpy.ops.wm.save_as_mainfile(filepath=out_blend, compress=True)
print("SALVO", out_blend)
