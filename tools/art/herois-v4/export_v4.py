"""Heróis v4: exporta o GLB cru para o jogo (corpo com LODs, esqueleto e todos os clipes).

Acrescenta `handslot.r` e `handslot.l` (o nome que o cliente usa para prender armas) nos encaixes
`socket_hand_*` do contrato. Depois: `node scripts/import-asset.mjs <saida.glb> personagem heroi_v4 --resize=1024`
(LOD já vem do Blender, textura 1024 em KTX2, manifesto).

    Blender --background --factory-startup base.blend --python tools/art/herois-v4/export_v4.py -- saida.glb
"""
import sys

import bpy

out = sys.argv[sys.argv.index("--") + 1]
arm = bpy.data.objects["heroi"]
arm.hide_set(False)
bpy.ops.object.select_all(action="DESELECT")
arm.select_set(True)
bpy.context.view_layer.objects.active = arm
bpy.ops.object.mode_set(mode="EDIT")
eb = arm.data.edit_bones
for s, slot in (("r", "handslot.r"), ("l", "handslot.l")):
    if slot in eb:
        continue
    src = eb[f"socket_hand_{s}"]
    b = eb.new(slot)
    b.head, b.tail, b.roll = src.head.copy(), src.tail.copy(), src.roll
    b.parent = eb[f"hand_{s}"]
    b.use_deform = False
bpy.ops.object.mode_set(mode="OBJECT")

keep = {"corpo", "corpo_LOD1", "corpo_LOD2", "heroi"}
bpy.ops.object.select_all(action="DESELECT")
for o in bpy.data.objects:
    if o.name in keep:
        o.hide_set(False)
        o.select_set(True)
for a in bpy.data.actions:
    a.use_fake_user = True
arm.animation_data_create()
arm.animation_data.action = None
bpy.ops.export_scene.gltf(filepath=out, export_format="GLB", use_selection=True, export_animations=True,
                          export_animation_mode="ACTIONS", export_force_sampling=True, export_frame_range=False,
                          export_def_bones=False, export_morph=True, export_apply=False)
print("EXPORTADO", out, [a.name for a in bpy.data.actions])
