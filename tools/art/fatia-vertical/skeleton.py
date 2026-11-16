"""Esqueleto guerreiro do estilo B (tunnel#93). Rode no Blender: exec(open(este_arquivo).read())."""
import math, os
import bpy
from mathutils import Vector

ROOT = os.environ.get("FATIA_DIR", r"C:\Users\DanielTeofilo\erevia-wt\fatia-vertical")
exec(open(os.path.join(ROOT, "tools/art/fatia-vertical/lib.py"), encoding="utf-8").read())
OUT = os.path.join(ROOT, "assets-src/fatia-vertical")
SEGS[:] = [(8, (6, 10)), (6, (4, 6)), (4, (3, 4))]

for o in list(bpy.data.objects):
    bpy.data.objects.remove(o, do_unlink=True)
for a in list(bpy.data.actions):
    bpy.data.actions.remove(a)
for im in list(bpy.data.images):
    bpy.data.images.remove(im)
for m in list(bpy.data.meshes):
    bpy.data.meshes.remove(m)

CELLS = {
    (0, 0): ("#f4ecd4", "#b8aa88"),  # osso claro (#e2d9bf do guia)
    (0, 1): ("#dccfae", "#8c7e5e"),  # osso
    (0, 2): ("#a89a78", "#4e4430"),  # osso na sombra / juntas
    (0, 3): ("#c0763c", "#5a2c18"),  # ferrugem (#b04a2e do guia, mais quente)
    (0, 4): ("#8088a0", "#2c3040"),  # ferro
    (0, 5): ("#7a4a62", "#32202c"),  # trapo
    (0, 6): ("#d8c0ff", "#8a5cff"),  # brilho dos olhos (roxo da cripta)
    (0, 7): ("#2a2230", "#0c0812"),  # órbitas
    (1, 0): ("#9a7448", "#3a2a1c"),  # couro
    (1, 2): ("#8a5e3a", "#33200f"),  # madeira do escudo
}
BONE_L, BONE, BONE_D, RUST, IRON, RAG, GLOW, SOCK = [(0, i) for i in range(8)]
LEATHER, _, WOOD = (1, 0), None, (1, 2)

img = make_atlas(os.path.join(OUT, "skeleton_atlas.png"), CELLS)
mat = make_material("Skeleton", img)

SLOT_ROLL = float(os.environ.get("SLOT_ROLL", "-90"))
spec = [
    ("hips", None, (0, 0, 0.90), (0, 0, 1.0), True),
    ("spine", "hips", (0, 0, 1.0), (0, 0, 1.15), True),
    ("chest", "spine", (0, 0, 1.15), (0, 0, 1.50), True),
    ("neck", "chest", (0, 0, 1.50), (0, 0, 1.56), True),
    ("head", "neck", (0, 0, 1.56), (0, 0, 1.80), True),
]
for s, n in ((1, ".l"), (-1, ".r")):
    spec += [
        ("upperarm" + n, "chest", (0.27 * s, 0, 1.46), (0.34 * s, 0, 1.19), True),
        ("lowerarm" + n, "upperarm" + n, (0.34 * s, 0, 1.19), (0.39 * s, 0, 0.93), True),
        ("hand" + n, "lowerarm" + n, (0.39 * s, 0, 0.93), (0.40 * s, 0, 0.83), True),
        ("handslot" + n, "hand" + n, (0.40 * s, 0, 0.87), (0.40 * s, -0.08, 0.87), False, SLOT_ROLL * s),
        ("upperleg" + n, "hips", (0.10 * s, 0, 0.92), (0.10 * s, 0, 0.50), True),
        ("lowerleg" + n, "upperleg" + n, (0.10 * s, 0, 0.50), (0.10 * s, 0, 0.10), True),
        ("foot" + n, "lowerleg" + n, (0.10 * s, 0, 0.10), (0.10 * s, -0.15, 0.04), True),
    ]
arm = make_armature("Skeleton_Rig", spec)
BONES = [s[0] for s in spec if s[4]]


def build(q, name):
    B = Builder(q, BONES)
    hi, mid = q == 0, q < 2
    # crânio e mandíbula (cabeça grande de propósito: lê de longe)
    B.ell((0, -0.005, 1.675), (0.105, 0.115, 0.125), "head", BONE_L, res=(10, 16))
    B.box((0, -0.055, 1.565), (0.115, 0.1, 0.06), "head", BONE, taper=(0.8, 1.0))                # mandíbula
    for s in (1, -1):
        B.ell((0.042 * s, -0.095, 1.69), (0.032, 0.03, 0.036), "head", SOCK, res=(5, 8))         # órbitas
        if mid:
            B.ell((0.042 * s, -0.112, 1.69), (0.014, 0.01, 0.014), "head", GLOW, res=(4, 6))     # brilho
    B.box((0, -0.113, 1.64), (0.026, 0.02, 0.04), "head", SOCK, taper=(0.6, 1.0))                 # nariz
    if hi:
        B.box((0, -0.105, 1.572), (0.09, 0.02, 0.022), "head", BONE_L)                            # dentes
    # coluna, costelas, esterno
    B.tube((0, 0.04, 0.98), (0, 0.045, 1.52), 0.03, 0.03, "spine", BONE_D)
    B.tube((0, 0.0, 1.52), (0, 0.0, 1.57), 0.032, 0.03, "neck", BONE_D)
    n_ribs = (5, 3, 2)[q]
    for i in range(n_ribs):
        z = 1.44 - i * (0.28 / (n_ribs - 1)) if n_ribs > 1 else 1.3
        w = 0.17 + 0.012 * (3 - abs(i - 1.5))
        B.ell((0, -0.01, z), (w, 0.115, 0.032), "chest", BONE, res=(4, 12))
    B.box((0, -0.115, 1.3), (0.04, 0.025, 0.3), "chest", BONE_L)                                   # esterno
    # pelve
    B.ell((0, 0.0, 0.95), (0.15, 0.095, 0.075), "hips", BONE)
    for s in (1, -1):
        if q < 2: B.ell((0.13 * s, 0.0, 0.98), (0.07, 0.07, 0.07), "hips", BONE_L, rot=(0, 0, 25 * s), res=(4, 8))
    B.box((0, -0.115, 0.84), (0.2, 0.02, 0.3), "hips", RAG, taper=(0.55, 1.0))                     # trapo na cintura
    B.tube((0, 0, 0.93), (0, 0, 1.0), 0.14, 0.14, "hips", LEATHER, sy=0.7)                 # cinto
    # pernas
    for s, n in ((1, ".l"), (-1, ".r")):
        x = 0.10 * s
        B.tube((x, 0, 0.92), (x, 0, 0.52), 0.05, 0.038, "upperleg" + n, BONE)               # fêmur
        if q < 2: B.ell((x, -0.01, 0.50), (0.062, 0.062, 0.062), "lowerleg" + n, BONE_L, res=(5, 8))         # joelho
        B.tube((x, 0, 0.50), (x, 0, 0.13), 0.04, 0.034, "lowerleg" + n, BONE)               # tíbia
        if q < 2: B.ell((x, -0.01, 0.105), (0.05, 0.05, 0.05), "lowerleg" + n, BONE_L, res=(4, 8))
        B.box((x, -0.075, 0.04), (0.1, 0.25, 0.08), "foot" + n, BONE, taper=None)                  # pé
        if mid:
            B.box((x, -0.2, 0.03), (0.09, 0.06, 0.06), "foot" + n, BONE_D)
    # braços
    for s, n in ((1, ".l"), (-1, ".r")):
        if q < 2: B.ell((0.27 * s, 0, 1.46), (0.06, 0.06, 0.06), "upperarm" + n, BONE_L, res=(5, 8))          # ombro
        B.tube((0.275 * s, 0, 1.45), (0.34 * s, 0, 1.2), 0.033, 0.027, "upperarm" + n, BONE)
        if q < 2: B.ell((0.34 * s, 0, 1.19), (0.045, 0.045, 0.045), "lowerarm" + n, BONE_L, res=(5, 8))       # cotovelo
        B.tube((0.34 * s, 0, 1.19), (0.39 * s, 0, 0.94), 0.03, 0.024, "lowerarm" + n, BONE)
        B.ell((0.40 * s, -0.003, 0.865), (0.048, 0.035, 0.07), "hand" + n, BONE_L)                  # mão (maior)
        if mid:                                                                                       # clavícula
            B.tube((0.04 * s, -0.03, 1.5), (0.26 * s, -0.01, 1.47), 0.018, 0.018, "chest", BONE)
    # ombreira de ferro enferrujada no ombro direito (-X) e peitoral parcial
    B.ell((-0.285, 0.0, 1.51), (0.14, 0.125, 0.085), "upperarm.r", RUST, rot=(0, 0, -14))
    if mid:
        B.tube((-0.29, 0, 1.52), (-0.295, 0, 1.49), 0.12, 0.105, "upperarm.r", IRON, sx=1.1)
    # espada enferrujada na mão direita (-X): aponta para a frente e para cima
    d = Vector((0, -0.64, 0.77))
    fist = Vector((-0.40, -0.01, 0.87))
    B.box(fist + d * 0.36, (0.09, 0.02, 0.56), "hand.r", RUST, rot=(40, 0, 0), taper=(0.45, 1.0), zrange=(0.9, 1.4))
    if q < 2: B.box(fist + d * 0.04, (0.16, 0.03, 0.035), "hand.r", IRON, rot=(40, 0, 0))
    if mid:
        B.ell(fist - d * 0.06, (0.03, 0.03, 0.03), "hand.r", IRON, res=(4, 6))
    # escudo redondo no braço esquerdo (+X), de frente
    B.disc((0.40, -0.115, 1.03), 0.25, 0.04, "lowerarm.l", RUST, axis=(0, -1, 0), seg=(12, 8, 6)[q])               # aro de ferro enferrujado
    B.disc((0.40, -0.13, 1.03), 0.215, 0.04, "lowerarm.l", WOOD, axis=(0, -1, 0), seg=(12, 8, 6)[q])                 # tábuas
    if q < 2: B.disc((0.40, -0.155, 1.03), 0.07, 0.03, "lowerarm.l", IRON, axis=(0, -1, 0))
    return B.finish(name, mat, arm)


objs = [build(q, "Skeleton_Body" + s) for q, s in enumerate(["", "_LOD1", "_LOD2"])]
for o in objs:
    print(o.name, tri_count(o))

# ---- animações e exportação -----------------------------------------------------------------
exec(open(os.path.join(ROOT, "tools/art/fatia-vertical/biped_anim.py"), encoding="utf-8").read())
build_clips(arm, "SW_")
export_glb(os.path.join(OUT, "MN_SkeletonWarrior.glb"), arm)
bpy.ops.wm.save_as_mainfile(filepath=os.path.join(OUT, "skeleton.blend"))
