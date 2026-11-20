"""Heróis v4: mapa de normais do corpo (o relevo da malha de alta resolução na malha do jogo).

O LOD0 (8000 triângulos) perde o desenho do peito, do abdômen e das costelas que a malha do Meshy (62 mil)
tem; a textura de cor traz esse relevo pintado com luz de outra forma, e na malha lisa vira mancha. Aqui o
Cycles assa as normais de `corpo_alta` (selecionado para ativo, no repouso) no UV do `corpo`, e o mapa entra
no material (Normal Map, espaço tangente) que o LOD0, o LOD1 e o LOD2 dividem: o glTF leva como
`normalTexture`. A imagem fica empacotada no .blend e é gravada em PNG ao lado.

    Blender --background --factory-startup corpo-lod.blend --python tools/art/herois-v4/bake_normal.py -- normal.png [tamanho]
"""
import sys

import bpy

args = sys.argv[sys.argv.index("--") + 1:]
out_png = args[0]
size = int(args[1]) if len(args) > 1 else 2048
low = bpy.data.objects["corpo"]
high = bpy.data.objects["corpo_alta"]
arm = bpy.data.objects["heroi"]
arm.data.pose_position = "REST"
for o in (low, high):
    o.hide_set(False)
    o.hide_render = False
    o.hide_viewport = False

img = bpy.data.images.get("corpo_normal") or bpy.data.images.new("corpo_normal", size, size, alpha=False, float_buffer=False)
img.colorspace_settings.name = "Non-Color"
img.generated_color = (0.5, 0.5, 1.0, 1.0)

mat = low.data.materials[0]
nt = mat.node_tree
for n in [n for n in nt.nodes if n.name in ("normal_tex", "normal_map")]:
    nt.nodes.remove(n)
tex = nt.nodes.new("ShaderNodeTexImage")
tex.name = "normal_tex"
tex.image = img
tex.interpolation = "Linear"
nt.nodes.active = tex
# a malha de alta usa o mesmo material: o mapa só é ligado depois de assar (senão entraria na própria leitura)
if high.data.materials and high.data.materials[0] == mat:
    hm = mat.copy()
    hm.name = "alta_sem_normal"
    for n in [n for n in hm.node_tree.nodes if n.name == "normal_tex"]:
        hm.node_tree.nodes.remove(n)
    high.data.materials[0] = hm

sc = bpy.context.scene
sc.render.engine = "CYCLES"
sc.cycles.device = "CPU"
sc.cycles.samples = 1
sc.render.bake.use_selected_to_active = True
sc.render.bake.use_cage = False
sc.render.bake.cage_extrusion = 0.02
sc.render.bake.max_ray_distance = 0.05
sc.render.bake.normal_space = "TANGENT"
sc.render.bake.margin = 16
bpy.ops.object.select_all(action="DESELECT")
high.select_set(True)
low.select_set(True)
bpy.context.view_layer.objects.active = low
bpy.ops.object.bake(type="NORMAL")
img.filepath_raw = out_png
img.file_format = "PNG"
img.save()
img.pack()
print("assado:", out_png)

# liga o mapa ao Principled do material do jogo
bsdf = next(n for n in nt.nodes if n.type == "BSDF_PRINCIPLED")
nm = nt.nodes.new("ShaderNodeNormalMap")
nm.name = "normal_map"
nm.space = "TANGENT"
nt.links.new(tex.outputs["Color"], nm.inputs["Color"])
nt.links.new(nm.outputs["Normal"], bsdf.inputs["Normal"])
arm.data.pose_position = "POSE"
high.hide_render = True
high.hide_set(True)
bpy.ops.wm.save_as_mainfile(filepath=bpy.data.filepath, compress=True)
print("SALVO", bpy.data.filepath)
