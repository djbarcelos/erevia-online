"""Heróis v4: reduz o corpo rigado aos orçamentos do contrato e gera os LODs.

Abre o .blend do corpo (build_rig.py), cria `corpo` (LOD0), `corpo_LOD1` e `corpo_LOD2` por Decimate
(colapso, mantém UVs e grupos de vértices), limita a 4 influências e normaliza, e salva o .blend e o .glb.

    Blender --background --factory-startup corpo.blend --python tools/art/herois-v4/make_lods.py -- saida.blend saida.glb [lod0 lod1 lod2]
"""
import sys

import bpy

args = sys.argv[sys.argv.index("--") + 1:]
out_blend, out_glb = args[:2]
targets = [int(x) for x in args[2:5]] if len(args) >= 5 else [8000, 3000, 1000]  # o teto do contrato (personagem)

src = bpy.data.objects["corpo"]
src.name = "corpo_alta"
arm = bpy.data.objects["heroi"]


def tris(o):
    return sum(len(p.vertices) - 2 for p in o.data.polygons)


HAND_KEEP = [0.993, 0.997, 1.0]  # LOD0, LOD1, LOD2
base = tris(src)
made = []
for k, target in enumerate(targets):
    o = src.copy()
    o.data = src.data.copy()
    bpy.context.scene.collection.objects.link(o)
    o.name = "corpo" if k == 0 else f"corpo_LOD{k}"
    o.data.name = o.name
    for m in list(o.modifiers):
        if m.type == "ARMATURE":
            o.modifiers.remove(m)
    # mãos protegidas: com o peso 1 em tudo, as duas mãos ficavam com ~450 triângulos cada e os dedos do punho
    # fechado viravam pontas (de longe, mão aberta). Peso menor nas mãos = menos colapso ali (o efeito é muito
    # sensível: 0,993 dá ~900 triângulos por mão no LOD0).
    hw = HAND_KEEP[k]
    names = {g.index: g.name for g in o.vertex_groups}
    vg = o.vertex_groups.new(name="decimar")
    for v in o.data.vertices:
        h = min(1.0, 2 * sum(g.weight for g in v.groups if names[g.group].startswith(("hand_", "thumb_", "index_", "fingers_"))))
        vg.add([v.index], 1.0 - (1.0 - hw) * h, "REPLACE")
    dec = o.modifiers.new("Decimate", "DECIMATE")
    dec.decimate_type = "COLLAPSE"
    dec.ratio = target / base
    dec.vertex_group = "decimar"
    dec.vertex_group_factor = 1.0
    dec.use_collapse_triangulate = True
    bpy.context.view_layer.objects.active = o
    bpy.ops.object.modifier_apply(modifier=dec.name)
    o.vertex_groups.remove(o.vertex_groups["decimar"])  # antes do limite de 4 influências
    # pesos: no máximo 4 influências e somando 1
    bpy.ops.object.mode_set(mode="WEIGHT_PAINT")
    bpy.ops.object.vertex_group_limit_total(group_select_mode="ALL", limit=4)
    bpy.ops.object.vertex_group_normalize_all(group_select_mode="ALL", lock_active=False)
    bpy.ops.object.mode_set(mode="OBJECT")
    m = o.modifiers.new("Armature", "ARMATURE")
    m.object = arm
    o.parent = arm
    made.append(o)
    print(f"{o.name}: {tris(o)} triângulos (alvo {target})")

# a malha de alta fica só no .blend, fora do glb
src.hide_render = True
src.hide_set(True)
bpy.ops.wm.save_as_mainfile(filepath=out_blend, compress=True)
bpy.ops.object.select_all(action="DESELECT")
for o in made + [arm]:
    o.select_set(True)
bpy.ops.export_scene.gltf(filepath=out_glb, export_format="GLB", use_selection=True, export_animations=False,
                          export_image_format="AUTO")
print("SALVO", out_blend, out_glb)
