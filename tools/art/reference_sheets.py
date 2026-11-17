"""Folhas de referência do guia de arte (docs/guia-de-arte.md): proporção, silhueta e cor.

Bonecos de blocagem feitos de primitivas, só para validar proporção (6,5–7 cabeças),
silhueta por classe e paleta antes de modelar de verdade (tunnel#91). Rodar com:

    /Applications/Blender.app/Contents/MacOS/Blender --background --factory-startup \\
        --python tools/art/reference_sheets.py -- docs/arte
"""

import math
import os
import sys

import bpy
from mathutils import Vector

OUT = sys.argv[sys.argv.index("--") + 1] if "--" in sys.argv else "docs/arte"
os.makedirs(OUT, exist_ok=True)

H = 1.8 / 7  # uma cabeça (personagem de 1,8 m com 7 cabeças)


def hexrgba(h, a=1.0):
    h = h.lstrip("#")
    srgb = [int(h[i:i + 2], 16) / 255 for i in (0, 2, 4)]
    lin = [c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4 for c in srgb]
    return (*lin, a)


_mats = {}


def mat(color):
    if color not in _mats:
        m = bpy.data.materials.new(color)
        m.diffuse_color = hexrgba(color)
        _mats[color] = m
    return _mats[color]


def reset():
    bpy.ops.wm.read_factory_settings(use_empty=True)
    _mats.clear()


def part(kind, loc, scale, color, rot=(0, 0, 0), **kw):
    if kind == "cyl":
        bpy.ops.mesh.primitive_cylinder_add(vertices=kw.get("v", 16), radius=1, depth=1, location=loc, rotation=rot)
    elif kind == "cone":
        bpy.ops.mesh.primitive_cone_add(vertices=kw.get("v", 20), radius1=1, radius2=kw.get("r2", 0), depth=1, location=loc, rotation=rot)
    elif kind == "sph":
        bpy.ops.mesh.primitive_uv_sphere_add(segments=20, ring_count=12, radius=1, location=loc, rotation=rot)
    else:
        bpy.ops.mesh.primitive_cube_add(size=1, location=loc, rotation=rot)
    o = bpy.context.object
    o.scale = scale
    o.data.materials.append(mat(color))
    return o


def limb(a, b, r0, r1, color):
    """Cilindro afunilado de a até b (raios r0 → r1)."""
    a, b = Vector(a), Vector(b)
    mid, d = (a + b) / 2, b - a
    bpy.ops.mesh.primitive_cone_add(vertices=14, radius1=r0, radius2=r1, depth=d.length, location=mid)
    o = bpy.context.object
    o.rotation_mode = "QUATERNION"
    o.rotation_quaternion = Vector((0, 0, 1)).rotation_difference(d.normalized())
    o.data.materials.append(mat(color))
    return o


# ---------------------------------------------------------------------------------------
# Corpo base (heroico): pernas longas, ombros largos, mãos e pés ~15% maiores.
# ---------------------------------------------------------------------------------------

def body(c, shoulder=0.24, waist=0.15, chest=0.2, skin="#d9b08c", cloth="#8a8f99", leather="#5a4030", boots="#3b2c22"):
    objs = []
    # pernas e pés
    for s in (-1, 1):
        objs.append(limb((s * 0.1, 0, 0.92), (s * 0.1, 0, 0.5), 0.085, 0.07, cloth))
        objs.append(limb((s * 0.1, 0, 0.5), (s * 0.1, 0, 0.09), 0.07, 0.055, boots))
        objs.append(part("cube", (s * 0.1, -0.04, 0.045), (0.11, 0.24, 0.09), boots))
    # quadril, tronco (afunilado: ombro largo, cintura fina), pescoço e cabeça
    objs.append(part("cyl", (0, 0, 0.95), (waist + 0.03, 0.13, 0.14), leather))
    objs.append(limb((0, 0, 1.02), (0, 0, 1.48), waist, shoulder * 0.95, cloth))
    objs.append(part("sph", (0, 0, 1.36), (shoulder, chest * 0.75, 0.16), cloth))
    objs.append(part("cyl", (0, 0, 1.52), (0.055, 0.055, 0.08), skin))
    objs.append(part("sph", (0, -0.01, 1.8 - H / 2), (0.105, 0.115, H / 2), skin))
    # braços em leve A, mãos maiores
    for s in (-1, 1):
        sh = (s * (shoulder + 0.02), 0, 1.44)
        el = (s * (shoulder + 0.09), 0, 1.13)
        wr = (s * (shoulder + 0.13), 0, 0.86)
        objs.append(part("sph", sh, (0.07, 0.07, 0.07), cloth))
        objs.append(limb(sh, el, 0.06, 0.05, cloth))
        objs.append(limb(el, wr, 0.05, 0.042, skin))
        objs.append(part("sph", (wr[0] + s * 0.01, 0, 0.8), (0.055, 0.04, 0.075), skin))
    return objs


PALETTE = {
    "comum": {
        "knight": dict(cloth="#8a8f99", accent="#9e2b25", trim="#b8bcc4"),
        "barbarian": dict(cloth="#6b4a2f", accent="#8c7a64", trim="#a8a08c"),
        "mage": dict(cloth="#3d4f8a", accent="#d6b46e", trim="#e8e1d4"),
        "rogue": dict(cloth="#3f4a3a", accent="#4b3a2e", trim="#8a8f99"),
    },
    "epico": {
        "knight": dict(cloth="#c9cdd6", accent="#5b2a8c", trim="#c08cff"),
        "barbarian": dict(cloth="#4a2f22", accent="#2e2440", trim="#c08cff"),
        "mage": dict(cloth="#2a1f55", accent="#c08cff", trim="#f0e6ff"),
        "rogue": dict(cloth="#1f2430", accent="#3a2a55", trim="#c08cff"),
    },
}


def figure(cls, tier="comum", neutral=False):
    """Monta o boneco da classe na origem e devolve o objeto unido."""
    p = PALETTE[tier][cls]
    if neutral:
        objs = body(None, cloth="#bfbfbf", skin="#d0d0d0", leather="#a8a8a8", boots="#9a9a9a")
    elif cls == "knight":
        objs = body(None, shoulder=0.25, cloth=p["cloth"], leather=p["accent"])
        big = 1.3 if tier == "epico" else 1.0
        for s in (-1, 1):  # ombreiras grandes e redondas
            objs.append(part("sph", (s * 0.29, 0, 1.47), (0.14 * big, 0.13 * big, 0.1 * big), p["trim"]))
        objs.append(part("cyl", (0, -0.01, 1.66), (0.125, 0.13, 0.1), p["trim"]))  # elmo de topo plano
        if tier == "epico":
            objs.append(part("cube", (0, -0.01, 1.8), (0.03, 0.22, 0.09), p["accent"]))  # crista
        objs.append(part("cyl", (-0.5, -0.12, 1.0), (0.26, 0.26, 0.04), p["accent"], rot=(math.radians(90), 0, math.radians(10)), v=24))  # escudo
        objs.append(part("cube", (0, 0.17, 1.15), (0.42, 0.03, 0.6), p["accent"]))  # capa curta
        objs.append(limb((0.42, -0.1, 0.8), (0.42, -0.1, 1.75), 0.025, 0.02, p["trim"]))  # espada erguida
    elif cls == "barbarian":
        objs = body(None, shoulder=0.31, waist=0.13, chest=0.24, cloth="#c08a63", leather=p["cloth"], boots="#3b2c22")
        objs.append(part("sph", (0, 0.02, 1.47), (0.4, 0.22, 0.09), p["accent"]))  # manto de pele nos ombros
        objs.append(part("cube", (0, 0, 0.82), (0.36, 0.2, 0.18), p["cloth"]))  # saiote de couro
        objs.append(limb((0.52, -0.05, 0.3), (0.3, -0.05, 1.95), 0.03, 0.03, "#5a4030"))  # machado de duas mãos
        blade = 1.4 if tier == "epico" else 1.0
        objs.append(part("cube", (0.27, -0.05, 1.82), (0.34 * blade, 0.04, 0.3 * blade), p["trim"], rot=(0, math.radians(-8), 0)))
    elif cls == "mage":
        objs = body(None, shoulder=0.21, cloth=p["cloth"], leather=p["cloth"])
        objs.append(part("cone", (0, 0, 0.62), (0.44, 0.38, 1.24), p["cloth"], r2=0.4))  # manto até o chão
        for s in (-1, 1):  # mangas largas
            objs.append(limb((s * 0.25, 0, 1.4), (s * 0.36, 0, 0.98), 0.07, 0.15, p["cloth"]))
        objs.append(part("cyl", (0, 0, 1.73), (0.24, 0.24, 0.015), p["accent"], v=24))  # aba do chapéu
        objs.append(part("cone", (0, 0.03, 2.0), (0.13, 0.13, 0.55), p["accent"], rot=(math.radians(-10), 0, 0)))  # chapéu alto
        objs.append(limb((0.45, -0.05, 0.0), (0.45, -0.05, 2.15), 0.025, 0.02, "#5a4030"))  # cajado mais alto que ele
        objs.append(part("sph", (0.45, -0.05, 2.2), (0.08, 0.08, 0.08), p["trim"]))
    else:  # rogue
        objs = body(None, shoulder=0.21, waist=0.13, chest=0.17, cloth=p["cloth"], leather=p["accent"])
        objs.append(part("cone", (0, 0.02, 1.76), (0.14, 0.15, 0.32), p["cloth"], rot=(math.radians(-25), 0, 0)))  # capuz pontudo
        objs.append(part("cube", (0.08, 0.22, 1.1), (0.3, 0.02, 0.7), p["accent"], rot=(math.radians(-18), 0, math.radians(8))))  # capa esvoaçante
        for s in (-1, 1):  # duas lâminas curtas
            objs.append(limb((s * 0.39, -0.1, 0.78), (s * 0.43, -0.25, 0.55), 0.018, 0.008, p["trim"]))
    bpy.ops.object.select_all(action="DESELECT")
    for o in objs:
        o.select_set(True)
    bpy.context.view_layer.objects.active = objs[0]
    bpy.ops.object.join()
    fig = bpy.context.object
    fig.name = cls
    # A transformação vinha da primeira peça (um cilindro girado): aplica na malha, para
    # o boneco ficar com pivô no chão, no centro, e girar em Euler.
    bpy.ops.object.transform_apply(location=True, rotation=True, scale=True)
    fig.rotation_mode = "XYZ"
    if cls == "rogue" and not neutral:
        fig.rotation_euler[0] = math.radians(-7)  # postura de bote
    return fig


def place(obj, x, rot_z=0.0):
    obj.location.x = x
    obj.rotation_euler[2] = math.radians(rot_z)


def setup(width, height, ortho, center, light="STUDIO", bg="#efe9df", color_type="MATERIAL"):
    sc = bpy.context.scene
    sc.render.engine = "BLENDER_WORKBENCH"
    sc.render.resolution_x, sc.render.resolution_y = width, height
    sc.render.resolution_percentage = 100
    sc.display.shading.light = light
    sc.display.shading.color_type = color_type
    sc.display.shading.show_shadows = False
    sc.display.shading.show_cavity = light == "STUDIO"
    sc.world = bpy.data.worlds.new("w")
    sc.world.color = hexrgba(bg)[:3]
    sc.view_settings.view_transform = "Standard"
    bpy.ops.object.camera_add(location=(center[0], -10, center[1]), rotation=(math.radians(90), 0, 0))
    cam = bpy.context.object
    cam.data.type = "ORTHO"
    cam.data.ortho_scale = ortho
    sc.camera = cam


def render(name):
    path = os.path.join(OUT, name)
    bpy.context.scene.render.filepath = path
    bpy.ops.render.render(write_still=True)
    print("SHEET", path)


# 1. Proporção: boneco neutro de frente, de lado e de costas, com as 7 cabeças marcadas.
reset()
for i, rz in enumerate((0, 90, 180)):
    place(figure("knight", neutral=True), (i - 1) * 1.1, rz)
for k in range(8):
    line = part("cube", (0, 0.6, k * H), (3.6, 0.005, 0.006), "#7a6a55")
    if k < 7:
        bpy.ops.object.text_add(location=(-1.75, 0.5, k * H + H / 2 - 0.04))
        t = bpy.context.object
        t.data.body = f"{k + 1}"
        t.data.size = 0.12
        t.rotation_euler[0] = math.radians(90)
        t.data.materials.append(mat("#7a6a55"))
setup(1600, 1000, 3.9, (0, 0.95))
render("1-proporcao.png")

# 2. Silhueta: as 4 classes em preto, em tamanho de leitura e do tamanho real no jogo (85 px).
for name, w, h in (("2-silhueta.png", 1600, 900), ("2-silhueta-85px.png", 300, 100)):
    reset()
    for i, cls in enumerate(("knight", "barbarian", "mage", "rogue")):
        o = figure(cls)
        place(o, (i - 1.5) * 1.25)
        for slot in o.material_slots:
            slot.material = mat("#111111")
    setup(w, h, 5.4, (0, 1.05), light="FLAT")
    render(name)

# 3. Cor: as 4 classes com equipamento comum (em cima) e épico (embaixo).
reset()
for row, tier in enumerate(("comum", "epico")):
    for i, cls in enumerate(("knight", "barbarian", "mage", "rogue")):
        o = figure(cls, tier)
        place(o, (i - 1.5) * 1.25)
        o.location.z = 2.5 if row == 0 else 0
setup(1600, 1600, 5.6, (0, 2.35))
render("3-cor.png")
