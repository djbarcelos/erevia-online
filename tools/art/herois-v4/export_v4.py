"""Heróis v4: exporta o GLB cru para o jogo (corpo com LODs, esqueleto e todos os clipes).

Acrescenta `handslot.r` e `handslot.l` (o nome que o cliente usa para prender armas) nos encaixes
`socket_hand_*` do contrato. Depois: `node scripts/import-asset.mjs <saida.glb> personagem heroi_v4 --resize=1024`
(LOD já vem do Blender, textura 1024 em KTX2, manifesto).

    Blender --background --factory-startup base.blend --python tools/art/herois-v4/export_v4.py -- saida.glb
"""
import sys

import bpy

_a = sys.argv[sys.argv.index("--") + 1:]
out = _a[0]
fists_ts = _a[1] if len(_a) > 1 else None
arm = bpy.data.objects["heroi"]
arm.hide_set(False)
bpy.ops.object.select_all(action="DESELECT")
arm.select_set(True)
bpy.context.view_layer.objects.active = arm
bpy.ops.object.mode_set(mode="EDIT")
eb = arm.data.edit_bones
from mathutils import Matrix, Quaternion

# handslot.* no referencial em que as pegas do jogo (WEAPON_GRIPS) foram calibradas: no "osso da mão" B,
# X lateral, Y ao longo do braço até os dedos, Z para a frente (o lado do polegar); o encaixe é B girado
# por (0,5 0,5 0,5 0,5), que o cliente desfaz com SLOT_INV. Calculado pela anatomia de cada mão.
R = Quaternion((0.5, 0.5, 0.5, 0.5)).to_matrix()
for s, slot in (("r", "handslot.r"), ("l", "handslot.l")):
    if slot in eb:
        eb.remove(eb[slot])
    hand = eb[f"hand_{s}"]
    H, K = hand.head.copy(), hand.tail.copy()
    a = (K - H).normalized()
    tip = eb[f"thumb_03_{s}"].tail
    tv = (tip - H) - a * (tip - H).dot(a)
    z = tv.normalized()
    x = a.cross(z).normalized()
    B = Matrix((x, a, z)).transposed()  # colunas X, Y, Z
    S = B @ R
    b = eb.new(slot)
    b.head = (H + K) / 2
    b.tail = b.head + S.col[1] * 0.05
    b.align_roll(S.col[2])
    b.parent = hand
    b.use_deform = False
bpy.ops.object.mode_set(mode="OBJECT")

# ---- punho fechado (HERO_FISTS do cliente): medido com os dedos na pose das animações ----
import json
import math
from mathutils import Euler, Vector
FIST = {"index": (55, 70, 45), "fingers": (75, 85), "thumb": (20, 25, 20)}
pb = arm.pose.bones
arm.animation_data_create()
arm.animation_data.action = None
for p_ in pb:  # zera a pose que ficou da última animação: só os dedos se mexem na medida
    p_.matrix_basis = Matrix.Identity(4)
for s_ in ("l", "r"):
    for f_, angs in FIST.items():
        for k, ang in enumerate(angs, 1):
            b_ = pb.get(f"{f_}_{k:02d}_{s_}")
            if b_:
                b_.rotation_mode = "XYZ"
                b_.rotation_euler = Euler((math.radians(ang), 0, 0))
bpy.context.view_layer.update()
fists = {}
for s_, slot in (("r", "handslot.r"), ("l", "handslot.l")):
    sl = arm.data.bones[slot]
    S_ = sl.matrix_local.to_3x3().normalized()
    Bf = S_ @ R.inverted()                  # referencial do osso da mão das pegas (X lateral, Y braço, Z frente)
    o = sl.head_local
    P = lambda n, end="tail": getattr(pb[n], end)
    center = (P(f"fingers_01_{s_}") + P(f"fingers_02_{s_}") + P(f"index_01_{s_}") + P(f"index_02_{s_}")) / 4
    knuck = P(f"index_01_{s_}", "head") - P(f"fingers_01_{s_}", "head")  # linha dos nós: do lado do mínimo ao indicador
    to_b = Bf.transposed()
    pos = to_b @ (center - o) / 0.4         # em unidades do encaixe (escala 0,4)
    axis = (to_b @ knuck).normalized()
    y = Vector((0, 1, 0))                   # dedos estendidos = eixo da mão (como nos heróis do remaster), sem a parte do cabo
    fing = (y - axis * y.dot(axis)).normalized()
    fists[s_] = {"pos": [round(x, 3) for x in pos], "axis": [round(x, 3) for x in axis], "fingers": [round(x, 3) for x in fing]}
for p_ in pb:
    p_.rotation_euler = (0, 0, 0)
bpy.context.view_layer.update()
print("PUNHO", json.dumps(fists))
if fists_ts:
    open(fists_ts, "w").write(
        "// Gerado por tools/art/herois-v4/export_v4.py (não editar à mão).\n"
        "// Punho fechado do corpo dos heróis v4, no espaço do encaixe handslot (sem a rotação SLOT_INV), como HERO_FISTS:\n"
        "// pos = centro do punho, axis = direção do cabo (do mínimo para o indicador), fingers = direção dos dedos.\n"
        "import type { FistFrame } from \"./hero-fists\";\n"
        f"export const HERO_FISTS_V4: {{ r: FistFrame; l: FistFrame }} = {json.dumps(fists)};\n")

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
