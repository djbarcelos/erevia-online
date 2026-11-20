"""Heróis v4: monta o esqueleto do contrato (docs/arte/herois-v4/contrato.md, seção 2) sobre o corpo.

Entrada: o corpo com o rig do Meshy (mixamo) já em A-pose (fix_arms.py). Saída: .blend e .glb com o
esqueleto `heroi` (44 ossos que deformam + root + encaixes), pesos convertidos e dedos novos.

- Posições das juntas: as do mixamo, simetrizadas (lado esquerdo espelhado no direito).
- Pesos: os grupos do mixamo renomeados; o braço é dividido entre `upperarm` e `upperarm_twist` pela
  posição ao longo do osso; os dedos (o Meshy só dá o médio) são achados pela geometria da mão:
  polegar = o que sai para a frente antes dos nós dos dedos; indicador = o dedo mais à frente;
  `fingers` = médio, anelar e mínimo juntos.

    Blender --background --factory-startup --python tools/art/herois-v4/build_rig.py -- corpo-m-bracos.glb saida.blend saida.glb
"""
import math
import sys

import bpy
from mathutils import Vector

src, out_blend, out_glb = sys.argv[sys.argv.index("--") + 1:][:3]
bpy.ops.wm.read_factory_settings(use_empty=True)
bpy.ops.import_scene.gltf(filepath=src)
for o in list(bpy.data.objects):
    if o.type == "MESH" and o.name.startswith("Icosphere"):
        bpy.data.objects.remove(o)
old = next(o for o in bpy.data.objects if o.type == "ARMATURE")
body = next(o for o in bpy.data.objects if o.type == "MESH")
body.name = "corpo"
P = "mixamorig:"
# solda os vértices duplicados nas costuras de UV (o glTF separa); as UVs ficam por canto e não se perdem
import bmesh
bm = bmesh.new()
bm.from_mesh(body.data)
n0 = len(bm.verts)
bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)
# simetria: fica a metade esquerda (+X) e ela é espelhada (o corpo do Meshy não é simétrico)
geom = bm.verts[:] + bm.edges[:] + bm.faces[:]
bmesh.ops.bisect_plane(bm, geom=geom, plane_co=(0, 0, 0), plane_no=(1, 0, 0), clear_inner=True, dist=1e-5)
bm.to_mesh(body.data)
bm.free()
mir = body.modifiers.new("Simetria", "MIRROR")
mir.use_axis[0] = True
mir.use_clip = True
mir.use_mirror_merge = True
mir.merge_threshold = 1e-4
mir.use_mirror_vertex_groups = True
body.modifiers.move(len(body.modifiers) - 1, 0)
bpy.context.view_layer.objects.active = body
bpy.ops.object.modifier_apply(modifier=mir.name)
print("vértices soldados e simetrizados:", n0, "->", len(body.data.vertices))


def j(name):
    return old.matrix_world @ old.data.bones[P + name].head_local


def sym(v, side):
    """Ponto do lado esquerdo (+X) espelhado para o lado pedido."""
    return Vector((abs(v.x) if side == "l" else -abs(v.x), v.y, v.z))


def avg_lr(left, right):
    """A malha foi simetrizada pela metade esquerda: as juntas vêm só do lado esquerdo."""
    return Vector((abs(left.x), left.y, left.z))


# ---------- juntas ----------
J = {}
for k, n in [("pelvis", "Hips"), ("spine_01", "Spine"), ("spine_02", "Spine1"), ("chest", "Spine2"), ("neck", "Neck"), ("head", "Head")]:
    v = j(n)
    J[k] = Vector((0, v.y, v.z))
J["head_end"] = Vector((0, j("HeadTop_End").y, j("HeadTop_End").z))
for k, n in [("clavicle", "Shoulder"), ("upperarm", "Arm"), ("forearm", "ForeArm"), ("hand", "Hand"),
             ("thigh", "UpLeg"), ("calf", "Leg"), ("foot", "Foot"), ("toe", "ToeBase"), ("toe_end", "Toe_End"),
             ("knuckle", "HandMiddle1")]:
    J[k] = avg_lr(j("Left" + n), j("Right" + n))

# ---------- dedos pela geometria da mão (lado esquerdo) ----------
vg = {g.index: g.name for g in body.vertex_groups}
hand_groups = {i for i, n in vg.items() if "LeftHand" in n}
H, K = J["hand"], J["knuckle"]
a = (K - H).normalized()                       # eixo da mão
f = Vector((0, -1, 0))                          # frente do personagem (-Y no Blender)
f = (f - a * f.dot(a)).normalized()            # frente, perpendicular ao eixo da mão
hand_verts = []
for v in body.data.vertices:
    w = sum(g.weight for g in v.groups if g.group in hand_groups)
    p = body.matrix_world @ v.co
    if w > 0.5 and p.x > 0:
        hand_verts.append((v.index, p))
sK = (K - H).dot(a)
def s_(p): return (p - H).dot(a)
def c_(p): return (p - H).dot(f)
def r_(p):
    d = (p - H) - a * s_(p)
    return d.length
smax = max(s_(p) for _, p in hand_verts)
# ---- dedos pela distância geodésica ao pulso ----
# Pontas = máximos locais da distância andando pela malha; cada dedo = região que sobe até a sua ponta;
# base do dedo = nível da membrana (onde a região encosta na do dedo vizinho).
import heapq
hand_set = {i for i, _ in hand_verts}
P3 = {i: p for i, p in hand_verts}
adj = {i: set() for i in hand_set}
for e in body.data.edges:
    v0, v1 = e.vertices
    if v0 in hand_set and v1 in hand_set:
        adj[v0].add(v1)
        adj[v1].add(v0)
geo = {i: float("inf") for i in hand_set}
heap = []
for i, p in hand_verts:
    if s_(p) < 0.01:
        geo[i] = 0.0
        heapq.heappush(heap, (0.0, i))
while heap:
    d0, i = heapq.heappop(heap)
    if d0 > geo[i]:
        continue
    for j in adj[i]:
        nd = d0 + (P3[i] - P3[j]).length
        if nd < geo[j]:
            geo[j] = nd
            heapq.heappush(heap, (nd, j))
reach = {i for i in hand_set if geo[i] < float("inf")}


def ascend(i, memo={}):
    path = []
    while i not in memo:
        path.append(i)
        nxt = max(adj[i] & reach, key=lambda j: geo[j], default=i)
        if geo[nxt] <= geo[i]:
            memo[i] = i
            break
        i = nxt
    t = memo[i]
    for q in path:
        memo[q] = t
    return t


owner = {i: ascend(i) for i in reach}
cand = sorted({t for t in owner.values()}, key=lambda t: -geo[t])
tips = []
for t in cand:  # pontas distintas (mais de 1,5 cm entre si) e longe do pulso
    if geo[t] < 0.06:
        continue
    if all((P3[t] - P3[u]).length > 0.015 for u in tips):
        tips.append(t)
    if len(tips) == 5:
        break
# vértices de pontas descartadas vão para a ponta aceita mais próxima
for i, t in list(owner.items()):
    if t not in tips:
        owner[i] = min(tips, key=lambda u: (P3[u] - P3[t]).length) if geo[t] >= 0.06 else None
# os dedos do modelo ficam colados (malha de pouca resolução): a região de cada dedo vem da geometria.
# Cada vértice vai para o segmento (centro da palma -> ponta) mais próximo; o dedo começa a uma fração
# da distância geodésica da ponta (dedos ~45% do pulso à ponta, polegar ~60%).
tipc = {t: P3[t] for t in tips}
thumb_t = max(tips, key=lambda t: (P3[t] - sum((P3[u] for u in tips if u != t), Vector()) / 4).length)
others = [t for t in tips if t != thumb_t]
index_t = min(others, key=lambda t: (P3[t] - P3[thumb_t]).length)
rest_t = [t for t in others if t != index_t]
mid = [P3[i] for i in reach if 0.35 * max(geo[t] for t in tips) < geo[i] < 0.5 * max(geo[t] for t in tips)]
palm_c = sum(mid, Vector()) / len(mid)
FRAC = {t: (0.6 if t == thumb_t else 0.45) for t in tips}


def seg_dist(p, a0, a1):
    ax = a1 - a0
    t = max(0.0, min(1.0, (p - a0).dot(ax) / ax.length_squared))
    return (p - a0.lerp(a1, t)).length


region = {t: [] for t in tips}
web = {t: geo[t] * (1 - FRAC[t]) for t in tips}
for i in reach:
    t = min(tips, key=lambda u: seg_dist(P3[i], palm_c, P3[u]))
    if geo[i] >= web[t] - 0.012:
        region[t].append(i)


def base_of(t):
    near = [P3[i] for i in region[t] if geo[i] <= web[t] + 0.01] or [P3[i] for i in region[t]]
    return sum(near, Vector()) / len(near)


print("pontas:", len(tips), "| geodésica (cm):", [round(geo[t] * 100, 1) for t in tips], "| polegar", round(geo[thumb_t] * 100, 1))


def chain(ts, n):
    b0 = sum((base_of(t) for t in ts), Vector()) / len(ts)
    b1 = sum((P3[t] for t in ts), Vector()) / len(ts)
    return [b0.lerp(b1, k / n) for k in range(n + 1)]


CH = {"thumb": chain([thumb_t], 3), "index": chain([index_t], 3), "fingers": chain(rest_t, 2)}
# parâmetro ao longo do próprio dedo (0 na base, 1 na ponta), para os pesos
FPARAM = {}
for fname, ts in (("thumb", [thumb_t]), ("index", [index_t]), ("fingers", rest_t)):
    for t in ts:
        g0, g1 = web[t] - 0.012, geo[t]
        for i in region[t]:
            FPARAM[i] = (fname, max(0.0, min(1.0, (geo[i] - g0) / max(1e-4, g1 - g0))))
thumb = [(i, P3[i]) for i, (fn, _) in FPARAM.items() if fn == "thumb"]
index = [(i, P3[i]) for i, (fn, _) in FPARAM.items() if fn == "index"]
rest = [(i, P3[i]) for i, (fn, _) in FPARAM.items() if fn == "fingers"]
print(f"mão: {len(hand_verts)} vértices; polegar {len(thumb)}, indicador {len(index)}, demais {len(rest)}")
for _n, _c in CH.items():
    print(_n, "base", tuple(round(x, 3) for x in _c[0]), "ponta", tuple(round(x, 3) for x in _c[-1]))
def rad(p): return (p - H) - a * s_(p)

# normal da palma: para onde as pontas dos dedos já se curvam (o modelo vem com a mão meio fechada)
tips = [p for _, p in hand_verts if s_(p) > sK + 0.6 * (smax - sK)]
bend = sum((rad(p) for p in tips), Vector()) / max(1, len(tips))
PALM = (bend - a * bend.dot(a)).normalized()
print("palma", tuple(round(x, 2) for x in PALM))

# ---------- armadura nova ----------
arm_data = bpy.data.armatures.new("heroi")
arm = bpy.data.objects.new("heroi", arm_data)
bpy.context.scene.collection.objects.link(arm)
bpy.context.view_layer.objects.active = arm
bpy.ops.object.mode_set(mode="EDIT")
eb = arm_data.edit_bones


def bone(name, head, tail, parent=None, deform=True, roll_up=Vector((0, 0, 1))):
    b = eb.new(name)
    b.head, b.tail = head, tail
    if (b.tail - b.head).length < 1e-4:
        b.tail = b.head + Vector((0, 0, 0.02))
    b.align_roll(roll_up)
    b.use_deform = deform
    if parent:
        b.parent = eb[parent]
        b.use_connect = False
    return b


bone("root", Vector((0, 0, 0)), Vector((0, -0.25, 0)), deform=False)
bone("pelvis", J["pelvis"], J["spine_01"], "root", roll_up=Vector((0, -1, 0)))
bone("spine_01", J["spine_01"], J["spine_02"], "pelvis", roll_up=Vector((0, -1, 0)))
bone("spine_02", J["spine_02"], J["chest"], "spine_01", roll_up=Vector((0, -1, 0)))
bone("chest", J["chest"], J["neck"], "spine_02", roll_up=Vector((0, -1, 0)))
bone("neck", J["neck"], J["head"], "chest", roll_up=Vector((0, -1, 0)))
bone("head", J["head"], J["head_end"], "neck", roll_up=Vector((0, -1, 0)))
for s in ("l", "r"):
    up = Vector((0, -1, 0))  # roll: eixo Z do osso para a frente, igual nos dois lados
    bone(f"clavicle_{s}", sym(J["clavicle"], s), sym(J["upperarm"], s), "chest", roll_up=up)
    mid = (J["upperarm"] + J["forearm"]) / 2
    bone(f"upperarm_{s}", sym(J["upperarm"], s), sym(J["forearm"], s), f"clavicle_{s}", roll_up=up)
    bone(f"upperarm_twist_{s}", sym(mid, s), sym(J["forearm"], s), f"upperarm_{s}", roll_up=up)
    bone(f"forearm_{s}", sym(J["forearm"], s), sym(J["hand"], s), f"upperarm_{s}", roll_up=up)
    palm = PALM.copy() if s == "l" else Vector((-PALM.x, PALM.y, PALM.z))  # direção: espelha só o X
    bone(f"hand_{s}", sym(J["hand"], s), sym(J["knuckle"], s), f"forearm_{s}", roll_up=palm)
    for fname, pts in CH.items():
        if not pts:
            continue
        for k in range(len(pts) - 1):
            parent = f"hand_{s}" if k == 0 else f"{fname}_{k:02d}_{s}"
            bone(f"{fname}_{k + 1:02d}_{s}", sym(pts[k], s), sym(pts[k + 1], s), parent, roll_up=palm)
    bone(f"thigh_{s}", sym(J["thigh"], s), sym(J["calf"], s), "pelvis", roll_up=up)
    bone(f"calf_{s}", sym(J["calf"], s), sym(J["foot"], s), f"thigh_{s}", roll_up=up)
    bone(f"foot_{s}", sym(J["foot"], s), sym(J["toe"], s), f"calf_{s}", roll_up=Vector((0, 0, 1)))
    bone(f"toe_{s}", sym(J["toe"], s), sym(J["toe_end"], s), f"foot_{s}", roll_up=Vector((0, 0, 1)))
    # capa: duas cadeias nas costas, do ombro até o joelho
    back = J["chest"].y + 0.16
    c0 = Vector((0.09, back, J["chest"].z))
    c1 = Vector((0.11, back + 0.04, J["pelvis"].z))
    c2 = Vector((0.12, back + 0.08, J["calf"].z))
    bone(f"cape_{s}_01", sym(c0, s), sym(c1, s), "chest", roll_up=Vector((0, 1, 0)))
    bone(f"cape_{s}_02", sym(c1, s), sym(c2, s), f"cape_{s}_01", roll_up=Vector((0, 1, 0)))
    # encaixes (não deformam)
    palm = sym((J["hand"] + J["knuckle"]) / 2 + f * 0.0, s)
    bone(f"socket_hand_{s}", palm, palm + Vector((0, -0.08, 0)), f"hand_{s}", deform=False)
    bone(f"socket_hip_{s}", sym(Vector((0.16, 0.0, J["pelvis"].z - 0.04)), s), sym(Vector((0.16, 0.0, J["pelvis"].z - 0.14)), s), "pelvis", deform=False)
bone("socket_shield", J["forearm"] * 0 + sym((J["forearm"] + J["hand"]) / 2, "l"), sym((J["forearm"] + J["hand"]) / 2 + Vector((0.08, 0, 0)), "l"), "forearm_l", deform=False)
bone("socket_back", Vector((0, J["chest"].y + 0.14, J["chest"].z)), Vector((0, J["chest"].y + 0.14, J["chest"].z + 0.1)), "chest", deform=False)
bone("socket_head", Vector((0, J["head_end"].y, J["head_end"].z)), Vector((0, J["head_end"].y, J["head_end"].z + 0.1)), "head", deform=False)
bone("socket_chest", Vector((0, J["chest"].y - 0.12, J["chest"].z)), Vector((0, J["chest"].y - 0.22, J["chest"].z)), "chest", deform=False)
bpy.ops.object.mode_set(mode="OBJECT")
deform = [b.name for b in arm_data.bones if b.use_deform]
print("ossos que deformam:", len(deform), "| total:", len(arm_data.bones))

# ---------- pesos ----------
RENAME = {"Hips": "pelvis", "Spine": "spine_01", "Spine1": "spine_02", "Spine2": "chest", "Neck": "neck", "Head": "head"}
for side, s in (("Left", "l"), ("Right", "r")):
    RENAME.update({f"{side}Shoulder": f"clavicle_{s}", f"{side}Arm": f"upperarm_{s}", f"{side}ForeArm": f"forearm_{s}",
                   f"{side}Hand": f"hand_{s}", f"{side}UpLeg": f"thigh_{s}", f"{side}Leg": f"calf_{s}",
                   f"{side}Foot": f"foot_{s}", f"{side}ToeBase": f"toe_{s}"})
    for n in range(1, 5):
        RENAME[f"{side}HandMiddle{n}"] = f"hand_{s}"  # o médio do Meshy volta para a mão; os dedos novos saem da geometria

weights = {}  # vértice -> {grupo: peso}
for v in body.data.vertices:
    d = {}
    for g in v.groups:
        n = vg[g.group].replace(P, "")
        n = RENAME.get(n)
        if n:
            d[n] = d.get(n, 0) + g.weight
    weights[v.index] = d

# braço: parte do peso vai para o twist, pela posição ao longo do osso (o direito vem do espelho)
for s in ("l",):
    a0, a1 = sym(J["upperarm"], s), sym(J["forearm"], s)
    ax = a1 - a0
    for i, d in weights.items():
        w = d.get(f"upperarm_{s}")
        if not w:
            continue
        t = (body.matrix_world @ body.data.vertices[i].co - a0).dot(ax) / ax.length_squared
        tw = max(0.0, min(1.0, (t - 0.35) / 0.4))  # 0 perto do ombro, 1 perto do cotovelo
        d[f"upperarm_{s}"] = w * (1 - tw)
        d[f"upperarm_twist_{s}"] = w * tw


NB = {"thumb": 3, "index": 3, "fingers": 2}
for i, (fname, t) in FPARAM.items():
    d = weights[i]
    total = sum(d.values()) or 1.0
    n = NB[fname]
    blend = max(0.0, min(1.0, t / 0.15))  # perto da base, metade fica na mão
    x = t * n
    k = min(n - 1, int(x))
    fr = x - k
    seg = {k: 1.0}
    if fr < 0.25 and k > 0:  # mistura com o osso anterior perto da junta
        seg = {k: 0.5 + fr * 2, k - 1: 0.5 - fr * 2}
    for kk, w in seg.items():
        d[f"{fname}_{kk + 1:02d}_l"] = d.get(f"{fname}_{kk + 1:02d}_l", 0) + total * blend * w
    for key in list(d):
        if not key.split("_")[0] in ("thumb", "index", "fingers"):
            d[key] *= (1 - blend)

# lado direito: cópia exata do esquerdo (a malha é simétrica), trocando _l por _r
from mathutils.kdtree import KDTree
pos = {v.index: body.matrix_world @ v.co for v in body.data.vertices}
left_ids = [i for i, p in pos.items() if p.x > 1e-4]
kd = KDTree(len(left_ids))
for n, i in enumerate(left_ids):
    kd.insert(pos[i], n)
kd.balance()


def flip(name):
    if name.endswith("_l"):
        return name[:-2] + "_r"
    if "_l_" in name:
        return name.replace("_l_", "_r_")
    return name


far = 0
for i, p in pos.items():
    if p.x >= -1e-4:
        continue
    _, n, dist = kd.find(Vector((-p.x, p.y, p.z)))
    far = max(far, dist)
    weights[i] = {flip(k): w for k, w in weights[left_ids[n]].items()}
print(f"lado direito espelhado; maior distância ao par: {far * 1000:.2f} mm")

# grava os grupos novos (normalizados, no máximo 4 influências)
for g in list(body.vertex_groups):
    body.vertex_groups.remove(g)
groups = {n: body.vertex_groups.new(name=n) for n in deform}
for i, d in weights.items():
    items = sorted(((k, w) for k, w in d.items() if k in groups and w > 1e-4), key=lambda x: -x[1])[:4]
    tot = sum(w for _, w in items)
    for k, w in items:
        groups[k].add([i], w / tot, "REPLACE")

# suaviza os pesos do tronco, ombros e quadril (o Meshy deixa degraus na axila e na virilha)
bpy.context.view_layer.objects.active = body
body.select_set(True)
bpy.ops.object.mode_set(mode="WEIGHT_PAINT")
SMOOTH = [n for n in deform if n.split("_")[0] in ("pelvis", "spine", "chest", "neck", "clavicle", "upperarm", "thigh")]
for n in SMOOTH:
    body.vertex_groups.active_index = body.vertex_groups[n].index
    bpy.ops.object.vertex_group_smooth(group_select_mode="ACTIVE", factor=0.5, repeat=4, expand=0.2)
bpy.ops.object.vertex_group_normalize_all(group_select_mode="BONE_DEFORM", lock_active=False)
bpy.ops.object.vertex_group_limit_total(group_select_mode="ALL", limit=4)
bpy.ops.object.vertex_group_normalize_all(group_select_mode="BONE_DEFORM", lock_active=False)
bpy.ops.object.mode_set(mode="OBJECT")

for m in list(body.modifiers):
    body.modifiers.remove(m)
body.parent = arm
mod = body.modifiers.new("Armature", "ARMATURE")
mod.object = arm
bpy.data.objects.remove(old)
print("grupos usados:", sum(1 for n in deform if any(True for _ in [0])), "| vértices:", len(body.data.vertices))

bpy.ops.wm.save_as_mainfile(filepath=out_blend, compress=True)
bpy.ops.export_scene.gltf(filepath=out_glb, export_format="GLB", export_animations=False)
print("SALVO", out_blend, out_glb)
