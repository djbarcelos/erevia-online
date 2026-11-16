"""Cavaleiro do estilo B (tunnel#93). Rode no Blender: exec(open(este_arquivo).read())."""
import math, os
import bpy
from mathutils import Vector

ROOT = os.environ.get("FATIA_DIR", r"C:\Users\DanielTeofilo\erevia-wt\fatia-vertical")
exec(open(os.path.join(ROOT, "tools/art/fatia-vertical/lib.py"), encoding="utf-8").read())

OUT = os.path.join(ROOT, "assets-src/fatia-vertical")

# ---- limpa a cena --------------------------------------------------------------------------
for o in list(bpy.data.objects):
    bpy.data.objects.remove(o, do_unlink=True)
for a in list(bpy.data.actions):
    bpy.data.actions.remove(a)
for m in list(bpy.data.meshes):
    bpy.data.meshes.remove(m)
for im in list(bpy.data.images):
    bpy.data.images.remove(im)

# ---- atlas (mesmo layout 8x4 do KayKit: pele [0,0], cabelo [0,1], roupa [1,0]) --------------
CELLS = {
    (0, 0): ("#f0c4a0", "#b87a58"),  # pele
    (0, 1): ("#8a6236", "#3a2616"),  # cabelo
    (0, 2): ("#dde4ee", "#8794aa"),  # aço claro
    (0, 3): ("#aab6c8", "#566178"),  # aço
    (0, 4): ("#8a94a8", "#3e4658"),  # aço escuro
    (0, 5): ("#f4dea0", "#a8883c"),  # ouro
    (0, 6): ("#9a6a42", "#40281a"),  # couro
    (0, 7): ("#2c2634", "#120e18"),  # olhos / fendas
    (1, 0): ("#c24a3e", "#6a1a1a"),  # roupa: capa, tabardo, penacho
    (1, 1): ("#7686a6", "#363f5c"),  # gibão
    (1, 2): ("#a8764a", "#4a2e18"),  # madeira
}
SKIN, HAIR, STEEL_L, STEEL, STEEL_D, GOLD, LEATHER, DARK = [(0, i) for i in range(8)]
CLOTH, GAMB, WOOD = (1, 0), (1, 1), (1, 2)

img = make_atlas(os.path.join(OUT, "knight_atlas.png"), CELLS)
mat = make_material("Knight", img)

# Encaixe da arma: o eixo +Y do osso aponta para a frente e o rolamento imita o do KayKit com o braço abaixado.
SLOT_ROLL = float(os.environ.get("SLOT_ROLL", "-90"))

# ---- esqueleto -----------------------------------------------------------------------------
spec = [
    ("hips", None, (0, 0, 0.90), (0, 0, 1.0), True),
    ("spine", "hips", (0, 0, 1.0), (0, 0, 1.15), True),
    ("chest", "spine", (0, 0, 1.15), (0, 0, 1.50), True),
    ("neck", "chest", (0, 0, 1.50), (0, 0, 1.58), True),
    ("head", "neck", (0, 0, 1.58), (0, 0, 1.80), True),
    ("cape1", "chest", (0, 0.13, 1.50), (0, 0.17, 1.20), True),
    ("cape2", "cape1", (0, 0.17, 1.20), (0, 0.20, 0.95), True),
]
for s, n in ((1, ".l"), (-1, ".r")):
    spec += [
        ("upperarm" + n, "chest", (0.28 * s, 0, 1.47), (0.355 * s, 0, 1.19), True),
        ("lowerarm" + n, "upperarm" + n, (0.355 * s, 0, 1.19), (0.41 * s, 0, 0.93), True),
        ("hand" + n, "lowerarm" + n, (0.41 * s, 0, 0.93), (0.425 * s, 0, 0.83), True),
        ("handslot" + n, "hand" + n, (0.425 * s, 0, 0.87), (0.425 * s, -0.08, 0.87), False, SLOT_ROLL * s),
        ("upperleg" + n, "hips", (0.11 * s, 0, 0.92), (0.11 * s, 0, 0.50), True),
        ("lowerleg" + n, "upperleg" + n, (0.11 * s, 0, 0.50), (0.11 * s, 0, 0.10), True),
        ("foot" + n, "lowerleg" + n, (0.11 * s, 0, 0.10), (0.11 * s, -0.15, 0.04), True),
    ]
arm = make_armature("Knight_Rig", spec)
BONES = [s[0] for s in spec if s[4]]


def build(q, name, hat_name):
    B = Builder(q, BONES)
    H = Builder(q, BONES)  # elmo (objeto à parte: dá para esconder)
    hi = q == 0
    # pernas e botas
    for s, n in ((1, ".l"), (-1, ".r")):
        x = 0.11 * s
        B.tube((x, 0, 0.93), (x, 0, 0.50), 0.108, 0.084, "upperleg" + n, GAMB)
        B.ell((x, -0.02, 0.50), (0.088, 0.088, 0.088), "lowerleg" + n, STEEL_L)             # joelheira
        B.tube((x, 0, 0.50), (x, 0, 0.16), 0.08, 0.062, "lowerleg" + n, STEEL)              # grevas
        B.tube((x, 0, 0.21), (x, 0, 0.12), 0.074, 0.07, "lowerleg" + n, LEATHER)            # cano da bota
        B.box((x, -0.065, 0.045), (0.125, 0.27, 0.09), "foot" + n, LEATHER, taper=None)       # bota (10-15% maior)
        if q < 2: B.box((x, -0.185, 0.035), (0.10, 0.07, 0.07), "foot" + n, STEEL_D)
        if hi:
            B.box((x, -0.03, 0.095), (0.1, 0.1, 0.03), "foot" + n, STEEL_D)
    # cintura, saia de placas, tabardo
    B.tube((0, 0, 0.88), (0, 0, 1.02), 0.185, 0.17, "hips", STEEL_D, sy=0.78)
    B.tube((0, 0, 1.0), (0, 0, 1.04), 0.19, 0.19, "spine", LEATHER, sy=0.8)               # cinto
    B.box((0, -0.15, 1.02), (0.05, 0.02, 0.05), "spine", GOLD)                             # fivela
    B.box((0, -0.13, 0.82), (0.17, 0.02, 0.36), "hips", CLOTH, taper=(0.7, 1.0))           # tabardo
    for s, n in ((1, ".l"), (-1, ".r")):
        B.box((0.13 * s, -0.095, 0.86), (0.1, 0.035, 0.2), "hips", STEEL, rot=(0, 0, 0))   # placas da saia
    # tronco
    B.tube((0, 0, 1.04), (0, 0, 1.17), 0.16, 0.18, "spine", GAMB, sy=0.8)
    B.tube((0, 0, 1.15), (0, 0, 1.50), 0.185, 0.245, "chest", STEEL, sy=0.72)              # peitoral em V
    B.ell((0, -0.04, 1.40), (0.17, 0.16, 0.11), "chest", STEEL_L)                           # peito
    if q < 2: B.tube((0, 0, 1.30), (0, 0, 1.34), 0.2, 0.2, "chest", GOLD, sy=0.78)
    if q < 2: B.ell((0, -0.145, 1.34), (0.05, 0.03, 0.05), "chest", GOLD)
    if q < 2: B.tube((0, 0, 1.49), (0, 0, 1.55), 0.12, 0.10, "chest", STEEL_L, sy=0.9)
    # pescoço, cabeça
    B.tube((0, 0, 1.52), (0, 0, 1.62), 0.055, 0.052, "neck", SKIN)
    B.ell((0, 0, 1.675), (0.088, 0.1, 0.108), "head", SKIN)
    B.ell((0, 0.035, 1.715), (0.093, 0.098, 0.085), "head", HAIR)                           # cabelo atrás
    if hi:
        B.box((0, -0.105, 1.66), (0.03, 0.04, 0.05), "head", SKIN)                          # nariz
        for s in (1, -1):
            B.box((0.036 * s, -0.09, 1.695), (0.026, 0.02, 0.016), "head", DARK)           # olhos
            B.box((0.036 * s, -0.095, 1.715), (0.04, 0.02, 0.01), "head", HAIR, rot=(0, 0, 8 * s))  # sobrancelhas
        B.box((0, -0.097, 1.625), (0.04, 0.02, 0.008), "head", DARK)                         # boca
    # braços
    for s, n in ((1, ".l"), (-1, ".r")):
        B.ell((0.30 * s, 0.0, 1.50), (0.14, 0.125, 0.095), "upperarm" + n, STEEL_L, rot=(0, 0, 14 * s))   # ombreira
        if q < 2: B.ell((0.325 * s, 0.0, 1.44), (0.15, 0.135, 0.075), "upperarm" + n, STEEL, rot=(0, 0, 12 * s))
        if q < 2: B.tube((0.33 * s, 0, 1.51), (0.335 * s, 0, 1.47), 0.12, 0.10, "upperarm" + n, GOLD, sx=1.1)
        B.tube((0.285 * s, 0, 1.46), (0.355 * s, 0, 1.19), 0.062, 0.054, "upperarm" + n, GAMB)
        B.ell((0.355 * s, 0, 1.19), (0.066, 0.066, 0.066), "lowerarm" + n, STEEL_L)           # cotoveleira
        B.tube((0.355 * s, 0, 1.19), (0.41 * s, 0, 0.94), 0.058, 0.05, "lowerarm" + n, STEEL)  # braçal
        if q < 2: B.tube((0.40 * s, 0, 0.97), (0.41 * s, 0, 0.93), 0.062, 0.062, "lowerarm" + n, GOLD)
        B.ell((0.425 * s, -0.005, 0.86), (0.058, 0.055, 0.072), "hand" + n, STEEL_D)         # manopla (mão 12% maior)
    # capa curta
    B.box((0, 0.14, 1.36), (0.40, 0.025, 0.30), "cape1", CLOTH, taper=(0.75, 1.0))
    B.box((0, 0.185, 1.08), (0.46, 0.025, 0.28), "cape2", CLOTH, taper=(0.9, 1.0))
    if q < 2: B.tube((0, 0.1, 1.50), (0, 0.11, 1.52), 0.125, 0.125, "chest", GOLD, sy=0.6)
    ob = B.finish(name, mat, arm)

    # elmo de topo plano com crista (seção 2 do guia)
    H.tube((0, 0, 1.60), (0, 0, 1.79), 0.118, 0.112, "head", STEEL, sy=1.07)
    H.tube((0, 0, 1.785), (0, 0, 1.805), 0.112, 0.112, "head", STEEL_L, sy=1.07)             # topo plano
    if q < 2: H.tube((0, 0, 1.60), (0, 0, 1.625), 0.126, 0.126, "head", GOLD, sy=1.07)
    if q < 2: H.tube((0, 0, 1.745), (0, 0, 1.765), 0.123, 0.123, "head", GOLD, sy=1.07)
    H.box((0, -0.126, 1.69), (0.16, 0.02, 0.028), "head", DARK)                              # fenda do olhar
    H.box((0, -0.128, 1.655), (0.03, 0.02, 0.085), "head", STEEL_D)                          # guarda do nariz
    H.box((0, 0.0, 1.835), (0.035, 0.2, 0.065), "head", CLOTH, taper=(1, 1))                 # crista (penacho)
    if q < 2: H.box((0, 0.1, 1.82), (0.035, 0.07, 0.11), "head", CLOTH)
    hat = H.finish(hat_name, mat, arm)
    return ob, hat


objs = []
for q, suf in enumerate(["", "_LOD1", "_LOD2"]):
    objs += list(build(q, "Knight_Body" + suf, "Hat" + suf))

for o in objs:
    print(o.name, tri_count(o))
print("tris LOD0", sum(tri_count(o) for o in objs[:2]), "LOD1", sum(tri_count(o) for o in objs[2:4]), "LOD2", sum(tri_count(o) for o in objs[4:]))

# ---- animações e exportação -----------------------------------------------------------------
exec(open(os.path.join(ROOT, "tools/art/fatia-vertical/biped_anim.py"), encoding="utf-8").read())
build_clips(arm, "KB_")
export_glb(os.path.join(OUT, "CH_Knight.glb"), arm)
bpy.ops.wm.save_as_mainfile(filepath=os.path.join(OUT, "knight.blend"))
