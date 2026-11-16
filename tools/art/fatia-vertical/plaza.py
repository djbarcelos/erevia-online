"""Kit da praça do estilo B (tunnel#93): fonte, piso em rosa dos ventos e poste com estandarte.
Rode no Blender: exec(open(este_arquivo).read()). Cada peça sai num .glb próprio (o import-asset
junta nós irmãos do mesmo material)."""
import math, os
import bpy
from mathutils import Vector

ROOT = os.environ.get("FATIA_DIR", r"C:\Users\DanielTeofilo\erevia-wt\fatia-vertical")
exec(open(os.path.join(ROOT, "tools/art/fatia-vertical/lib.py"), encoding="utf-8").read())
OUT = os.path.join(ROOT, "assets-src/fatia-vertical")
SEGS[:] = [(28, (8, 16)), (14, (5, 8)), (8, (3, 5))]

for o in list(bpy.data.objects):
    bpy.data.objects.remove(o, do_unlink=True)
for m in list(bpy.data.meshes):
    bpy.data.meshes.remove(m)
for im in list(bpy.data.images):
    bpy.data.images.remove(im)

# paleta da cidade (guia, seção 3.3): pedra quente #c9b79c, madeira #7a5232, estandarte #9e2b25, ouro #d6b46e
CELLS = {
    (0, 0): ("#c4b494", "#8e7a5e"),  # pedra clara
    (0, 1): ("#ad9a7c", "#6e5c46"),  # pedra quente
    (0, 2): ("#8c7a66", "#4a3e32"),  # pedra escura
    (0, 3): ("#f2dc9a", "#a8883c"),  # ouro
    (0, 4): ("#a8383a", "#5a1618"),  # estandarte
    (0, 5): ("#9a6a3c", "#4a2e18"),  # madeira
    (0, 6): ("#8fd0ee", "#3478a8"),  # água
    (0, 7): ("#524a5c", "#26202e"),  # ferro
}
STONE_L, STONE, STONE_D, GOLD, RED, WOOD, WATER, IRON = [(0, i) for i in range(8)]
img = make_atlas(os.path.join(OUT, "plaza_atlas.png"), CELLS)
mat = make_material("Plaza", img)


def kit(name, fn, lods=(0,)):
    out = []
    for q in lods:
        B = Builder(q, ["root"])
        fn(B, q)
        out.append(B.finish(name + ("" if q == 0 else "_LOD%d" % q), mat, None))
    return out


def fountain(B, q):
    z = (0, 0, 0)
    B.tube((0, 0, 0), (0, 0, 0.5), 1.62, 1.58, "root", STONE)              # parede da bacia
    B.tube((0, 0, 0.5), (0, 0, 0.62), 1.74, 1.74, "root", STONE_L)         # borda
    B.disc((0, 0, 0.46), 1.55, 0.04, "root", WATER, axis=(0, 0, 1))        # água da bacia
    B.tube((0, 0, 0.0), (0, 0, 0.12), 1.8, 1.8, "root", STONE_D)           # degrau
    B.tube((0, 0, 0.4), (0, 0, 1.25), 0.3, 0.2, "root", STONE)             # coluna
    B.tube((0, 0, 1.2), (0, 0, 1.5), 0.3, 0.8, "root", STONE_L)            # taça
    B.disc((0, 0, 1.47), 0.7, 0.04, "root", WATER, axis=(0, 0, 1))         # água da taça
    B.tube((0, 0, 1.5), (0, 0, 2.1), 0.14, 0.1, "root", STONE)             # haste
    B.ell((0, 0, 2.22), (0.16, 0.16, 0.2), "root", GOLD)                    # ponta dourada
    if q < 2:
        B.tube((0, 0, 0.5), (0, 0, 0.56), 1.76, 1.76, "root", GOLD)         # friso dourado na borda
        for i in range(4):
            a = i * math.pi / 2 + math.pi / 4
            B.ell((math.cos(a) * 1.68, math.sin(a) * 1.68, 0.64), (0.13, 0.13, 0.13), "root", GOLD, res=(4, 6))
        for i in range(8):                                                   # pilares baixos da borda
            a = i * math.pi / 4
            B.box((math.cos(a) * 1.78, math.sin(a) * 1.78, 0.68), (0.2, 0.2, 0.14), "root", STONE_L, rot=(0, 0, math.degrees(a)))


def paving(B, q):
    seg = (48, 24, 12)[q]
    B.tube((0, 0, 0), (0, 0, 0.05), 9.3, 9.3, "root", GOLD, seg=seg)       # borda dourada
    B.tube((0, 0, 0), (0, 0, 0.06), 8.9, 8.9, "root", STONE_L, seg=seg)    # anel externo
    B.tube((0, 0, 0), (0, 0, 0.07), 5.9, 5.9, "root", STONE_D, seg=seg)    # aro escuro
    B.tube((0, 0, 0), (0, 0, 0.08), 5.55, 5.55, "root", STONE, seg=seg)    # miolo quente
    for i in range(8):                                                      # rosa dos ventos
        a = i * math.pi / 4 + math.pi / 2
        long_pt = i % 2 == 0
        r1, w = (8.2, 0.85) if long_pt else (6.4, 0.6)
        ca, sa = math.cos(a), math.sin(a)
        pa, pb = -sa, ca  # perpendicular
        r0 = 2.2
        pts = [(ca * r0 + pa * w, sa * r0 + pb * w), (ca * r1, sa * r1), (ca * r0 - pa * w, sa * r0 - pb * w), (ca * (r0 - 0.4), sa * (r0 - 0.4))]
        B.prism(pts[::-1], 0.08, 0.1, "root", RED if long_pt else STONE_D)
    B.tube((0, 0, 0), (0, 0, 0.12), 2.35, 2.35, "root", GOLD, seg=seg)     # aro dourado da fonte
    B.tube((0, 0, 0), (0, 0, 0.13), 2.1, 2.1, "root", STONE_L, seg=seg)


def banner(B, q):
    B.tube((0, 0, 0), (0, 0, 0.32), 0.3, 0.24, "root", STONE)               # base
    B.tube((0, 0, 0.3), (0, 0, 3.5), 0.07, 0.06, "root", WOOD, seg=8)       # mastro
    B.box((0, 0, 3.3), (1.0, 0.07, 0.07), "root", WOOD)                     # travessa
    B.box((0, 0, 2.5), (0.74, 0.03, 1.5), "root", RED, taper=(1.0, 0.85))  # pano
    B.box((0, -0.02, 3.0), (0.4, 0.035, 0.38), "root", GOLD, taper=(1.0, 0.7))  # emblema dourado
    B.ell((0, 0, 3.58), (0.09, 0.09, 0.12), "root", GOLD, res=(4, 6))
    if q < 2:
        for s in (-1, 1):
            B.ell((0.5 * s, 0, 3.3), (0.06, 0.06, 0.06), "root", GOLD, res=(4, 6))
        B.box((0, 0, 1.85), (0.74, 0.035, 0.1), "root", GOLD)               # franja dourada


objs = kit("Fountain", fountain, (0, 1, 2)) + kit("Paving", paving, (0, 1)) + kit("BannerPost", banner, (0, 1, 2))
for o in objs:
    print(o.name, tri_count(o))


def exp(path, names):
    for o in bpy.data.objects:
        o.hide_set(False)
        o.select_set(False)
    for n in names:
        bpy.data.objects[n].select_set(True)
    bpy.context.view_layer.objects.active = bpy.data.objects[names[0]]
    bpy.ops.export_scene.gltf(filepath=path, export_format="GLB", use_selection=True, export_animations=False, export_yup=True,
                              export_apply=False, export_image_format="AUTO", export_cameras=False, export_lights=False)


exp(os.path.join(OUT, "PR_Fountain.glb"), ["Fountain", "Fountain_LOD1", "Fountain_LOD2"])
exp(os.path.join(OUT, "PR_Paving.glb"), ["Paving", "Paving_LOD1"])
exp(os.path.join(OUT, "PR_BannerPost.glb"), ["BannerPost", "BannerPost_LOD1", "BannerPost_LOD2"])
bpy.ops.wm.save_as_mainfile(filepath=os.path.join(OUT, "plaza.blend"))
