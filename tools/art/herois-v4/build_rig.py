"""Heróis v4: monta o esqueleto do contrato (docs/arte/herois-v4/contrato.md, seção 2) sobre o corpo.

Entrada: o corpo com o rig do Meshy (mixamo) já em A-pose (fix_arms.py). Saída: .blend e .glb com o
esqueleto `heroi` (44 ossos que deformam + root + encaixes), pesos convertidos e dedos novos.

- Juntas: as do mixamo, cada lado com as suas (a malha do Meshy não é simétrica; simetrizar quebra o rosto).
- Pesos: os grupos do mixamo renomeados; o braço é dividido entre `upperarm` e `upperarm_twist` pela
  posição ao longo do osso.
- Dedos (o Meshy só dá o médio): em cada mão, as pontas são os máximos da distância ao pulso andando pela
  malha; cada dedo é a região que sobe até a sua ponta, e a base é a membrana. Polegar = a ponta mais
  afastada das outras; indicador = a vizinha do polegar; `fingers` = médio, anelar e mínimo juntos.

    Blender --background --factory-startup --python tools/art/herois-v4/build_rig.py -- corpo-bracos.glb saida.blend saida.glb
"""
import heapq
import sys

import bmesh
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
SIDES = (("l", "Left"), ("r", "Right"))

# solda os vértices duplicados nas costuras de UV (o glTF separa); as UVs ficam por canto e não se perdem
bm = bmesh.new()
bm.from_mesh(body.data)
n0 = len(bm.verts)
bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)
bm.to_mesh(body.data)
bm.free()
print("vértices soldados:", n0, "->", len(body.data.vertices))
MW = body.matrix_world
POS = {v.index: MW @ v.co for v in body.data.vertices}
VG = {g.index: g.name for g in body.vertex_groups}


def j(name):
    return old.matrix_world @ old.data.bones[P + name].head_local


# ---------- juntas ----------
C = {k: j(n) for k, n in [("pelvis", "Hips"), ("spine_01", "Spine"), ("spine_02", "Spine1"), ("chest", "Spine2"),
                           ("neck", "Neck"), ("head", "Head"), ("head_end", "HeadTop_End")]}
J = {s: {k: j(side + n) for k, n in [("clavicle", "Shoulder"), ("upperarm", "Arm"), ("forearm", "ForeArm"), ("hand", "Hand"),
                                      ("thigh", "UpLeg"), ("calf", "Leg"), ("foot", "Foot"), ("toe", "ToeBase"),
                                      ("toe_end", "Toe_End"), ("knuckle", "HandMiddle1")]}
     for s, side in SIDES}


# ---------- dedos pela distância geodésica ao pulso ----------
def hand_fingers(s, side):
    H, K = J[s]["hand"], J[s]["knuckle"]
    a = (K - H).normalized()
    hand_groups = {i for i, n in VG.items() if f"{side}Hand" in n}
    hv = {}
    for v in body.data.vertices:
        w = sum(g.weight for g in v.groups if g.group in hand_groups)
        if w > 0.5:
            hv[v.index] = POS[v.index]
    s_ = lambda p: (p - H).dot(a)
    adj = {i: set() for i in hv}
    for e in body.data.edges:
        v0, v1 = e.vertices
        if v0 in hv and v1 in hv:
            adj[v0].add(v1)
            adj[v1].add(v0)
    geo = {i: float("inf") for i in hv}
    heap = []
    for i, p in hv.items():
        if s_(p) < 0.01:
            geo[i] = 0.0
            heapq.heappush(heap, (0.0, i))
    while heap:
        d0, i = heapq.heappop(heap)
        if d0 > geo[i]:
            continue
        for k in adj[i]:
            nd = d0 + (hv[i] - hv[k]).length
            if nd < geo[k]:
                geo[k] = nd
                heapq.heappush(heap, (nd, k))
    reach = {i for i in hv if geo[i] < float("inf")}
    memo = {}

    def ascend(i):
        path = []
        while i not in memo:
            path.append(i)
            nxt = max(adj[i] & reach, key=lambda k: geo[k], default=i)
            if geo[nxt] <= geo[i]:
                memo[i] = i
                break
            i = nxt
        t = memo[i]
        for q in path:
            memo[q] = t
        return t

    owner = {i: ascend(i) for i in reach}
    # máximos locais da distância ao pulso, agrupados (uma ponta por grupo de 1 cm)
    maxima = sorted({t for t in owner.values() if geo[t] >= 0.05}, key=lambda t: -geo[t])
    reps = []
    for t in maxima:
        if all((hv[t] - hv[u]).length > 0.01 for u in reps):
            reps.append(t)
    REPS[s] = [(round(geo[t], 4), tuple(hv[t])) for t in reps]
    top = reps[0]
    finger_tips = []
    for t in reps:  # as 4 pontas dos dedos: perto da mais distante, separadas entre si
        if (hv[t] - hv[top]).length < 0.05 and all((hv[t] - hv[u]).length > 0.015 for u in finger_tips):
            finger_tips.append(t)
        if len(finger_tips) == 4:
            break
    # polegar: entre as pontas longe do pulso (> 12 cm), a mais afastada das quatro pontas dos dedos
    far = [t for t in reps if geo[t] > 0.12 and t not in finger_tips]
    thumb_tip = max(far, key=lambda t: min((hv[t] - hv[u]).length for u in finger_tips))
    tips = finger_tips + [thumb_tip]
    for i, t in list(owner.items()):
        if t not in tips:
            owner[i] = min(tips, key=lambda u: (hv[u] - hv[t]).length) if geo[t] >= 0.05 else None
    # polegar = a ponta menos avançada ao longo do eixo da mão (o dedo mais curto);
    # indicador = entre os outros, ordenados pela largura da mão, o mais perto do polegar.
    thumb_t = thumb_tip
    others = [t for t in tips if t != thumb_t]
    bend = Vector()
    for t in others:
        r = hv[t] - H
        bend += r - a * r.dot(a)
    palm0 = (bend - a * bend.dot(a)).normalized()
    wax = palm0.cross(a).normalized()
    L = lambda t: (hv[t] - H).dot(wax)
    others.sort(key=L)
    index_t = min(others, key=lambda t: (hv[t] - hv[thumb_t]).length)
    rest_t = [t for t in others if t != index_t]
    cands = [t for t in sorted(set(memo.values()), key=lambda t: -geo[t]) if geo[t] >= 0.05][:12]
    DEBUG[s] = {"tips": tips, "owner": owner, "thumb": thumb_t, "index": index_t, "cands": cands, "geo": geo, "hv": hv}
    # base de cada dedo: a fração da distância do pulso à ponta onde o dedo se solta da palma
    web = {t: geo[t] * (1 - (0.55 if t == thumb_t else 0.42)) for t in tips}
    # cada vértice além da membrana vai para o dedo cujo segmento (base -> ponta) está mais perto
    base0 = {}
    for t in tips:
        near = [hv[i] for i in reach if abs(geo[i] - web[t]) < 0.006 and owner.get(i) == t] or [hv[t]]
        base0[t] = sum(near, Vector()) / len(near)

    def seg_d(p, t):
        a0, a1 = base0[t], hv[t]
        ax = a1 - a0
        u = max(0.0, min(1.0, (p - a0).dot(ax) / ax.length_squared))
        return (p - a0.lerp(a1, u)).length

    for _ in range(3):  # região pela distância ao segmento; a base sai da própria região; repete
        region = {t: [] for t in tips}
        for i in reach:
            t = min(tips, key=lambda u: seg_d(hv[i], u))
            if geo[i] >= web[t] - 0.01:
                region[t].append(i)
        for t in tips:
            near = [hv[i] for i in region[t] if abs(geo[i] - web[t]) < 0.006]
            if near:
                base0[t] = sum(near, Vector()) / len(near)
    def base_of(t):
        return base0[t]

    def chain(ts, n):
        b0 = sum((base_of(t) for t in ts), Vector()) / len(ts)
        b1 = sum((hv[t] for t in ts), Vector()) / len(ts)
        return [b0.lerp(b1, k / n) for k in range(n + 1)]

    ch = {"thumb": chain([thumb_t], 3), "index": chain([index_t], 3), "fingers": chain(rest_t, 2)}
    fparam = {}
    for fname, ts in (("thumb", [thumb_t]), ("index", [index_t]), ("fingers", rest_t)):
        for t in ts:
            g0, g1 = web[t] - 0.01, geo[t]
            for i in region[t]:
                fparam[i] = (fname, max(0.0, min(1.0, (geo[i] - g0) / max(1e-4, g1 - g0))))
    # palma pela anatomia: com t = o lado do polegar (perpendicular ao eixo da mão), a palma é t x a na mão
    # direita e a x t na esquerda (vale para qualquer pose; a curvatura dos dedos do modelo engana).
    tv = hv[thumb_t] - H
    tv = (tv - a * tv.dot(a)).normalized()
    palm = (tv.cross(a) if s == "r" else a.cross(tv)).normalized()
    cnt = {n: sum(1 for f, _ in fparam.values() if f == n) for n in ("thumb", "index", "fingers")}
    print(f"mão {s}: {len(hv)} vértices, pontas a {[round(geo[t] * 100, 1) for t in tips]} cm, {cnt}, palma {tuple(round(x, 2) for x in palm)}")
    return ch, fparam, palm


FIX = {}  # lado -> (índice da ponta do polegar, índice da ponta do indicador), na ordem de `tips`
REPS = {}
DEBUG = {}
FING = {s: hand_fingers(s, side) for s, side in SIDES}
import json, os
if os.environ.get("HV4_REPS"):
    json.dump({k: {"reps": v, "hand": tuple(J[k]["hand"]), "knuckle": tuple(J[k]["knuckle"])} for k, v in REPS.items()}, open(os.environ["HV4_REPS"], "w"))
for s in DEBUG:
    tt = DEBUG[s]["tips"]
    print(f"pontas {s}: polegar = {tt.index(DEBUG[s]['thumb'])}, indicador = {tt.index(DEBUG[s]['index'])}")
# cores por ponta (atributo de cor "pontas") para conferir a olho
_ca = body.data.color_attributes.new("pontas", "FLOAT_COLOR", "POINT")
_cols = [(1, 0.2, 0.2, 1), (0.2, 1, 0.2, 1), (0.2, 0.4, 1, 1), (1, 1, 0.2, 1), (1, 0.3, 1, 1)]
for v in body.data.vertices:
    _ca.data[v.index].color = (0.6, 0.6, 0.6, 1)
_fc = {"thumb": (1, 0.2, 0.2, 1), "index": (0.2, 1, 0.2, 1), "fingers": (0.3, 0.5, 1, 1)}
for s in FING:
    for i, (fname, _) in FING[s][1].items():
        _ca.data[i].color = _fc[fname]

# ---------- armadura nova ----------
arm_data = bpy.data.armatures.new("heroi")
arm = bpy.data.objects.new("heroi", arm_data)
bpy.context.scene.collection.objects.link(arm)
bpy.context.view_layer.objects.active = arm
bpy.ops.object.mode_set(mode="EDIT")
eb = arm_data.edit_bones
FWD = Vector((0, -1, 0))  # frente do personagem no Blender


def bone(name, head, tail, parent=None, deform=True, roll_up=FWD):
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


bone("root", Vector((0, 0, 0)), Vector((0, -0.25, 0)), deform=False, roll_up=Vector((0, 0, 1)))
bone("pelvis", C["pelvis"], C["spine_01"], "root")
bone("spine_01", C["spine_01"], C["spine_02"], "pelvis")
bone("spine_02", C["spine_02"], C["chest"], "spine_01")
bone("chest", C["chest"], C["neck"], "spine_02")
bone("neck", C["neck"], C["head"], "chest")
bone("head", C["head"], C["head_end"], "neck")
for s, _ in SIDES:
    Js = J[s]
    sign = 1 if s == "l" else -1
    ch, _, palm = FING[s]
    bone(f"clavicle_{s}", Js["clavicle"], Js["upperarm"], "chest")
    bone(f"upperarm_{s}", Js["upperarm"], Js["forearm"], f"clavicle_{s}")
    bone(f"upperarm_twist_{s}", (Js["upperarm"] + Js["forearm"]) / 2, Js["forearm"], f"upperarm_{s}")
    bone(f"forearm_{s}", Js["forearm"], Js["hand"], f"upperarm_{s}")
    bone(f"hand_{s}", Js["hand"], Js["knuckle"], f"forearm_{s}", roll_up=palm)  # Z da mão e dos dedos para a palma: dobrar = girar em X
    for fname, pts in ch.items():
        for k in range(len(pts) - 1):
            parent = f"hand_{s}" if k == 0 else f"{fname}_{k:02d}_{s}"
            bone(f"{fname}_{k + 1:02d}_{s}", pts[k], pts[k + 1], parent, roll_up=palm)
    bone(f"thigh_{s}", Js["thigh"], Js["calf"], "pelvis")
    bone(f"calf_{s}", Js["calf"], Js["foot"], f"thigh_{s}")
    bone(f"foot_{s}", Js["foot"], Js["toe"], f"calf_{s}", roll_up=Vector((0, 0, 1)))
    bone(f"toe_{s}", Js["toe"], Js["toe_end"], f"foot_{s}", roll_up=Vector((0, 0, 1)))
    # capa: duas cadeias nas costas, do ombro até o joelho
    back = C["chest"].y + 0.16
    cx = C["chest"].x
    c0 = Vector((cx + sign * 0.09, back, C["chest"].z))
    c1 = Vector((cx + sign * 0.11, back + 0.04, C["pelvis"].z))
    c2 = Vector((cx + sign * 0.12, back + 0.08, Js["calf"].z))
    bone(f"cape_{s}_01", c0, c1, "chest", roll_up=Vector((0, 1, 0)))
    bone(f"cape_{s}_02", c1, c2, f"cape_{s}_01", roll_up=Vector((0, 1, 0)))
    # encaixes (não deformam)
    grip = (Js["hand"] + Js["knuckle"]) / 2 + palm * 0.025
    bone(f"socket_hand_{s}", grip, grip + FWD * 0.08, f"hand_{s}", deform=False)
    hip = Vector((C["pelvis"].x + sign * 0.16, C["pelvis"].y, C["pelvis"].z - 0.04))
    bone(f"socket_hip_{s}", hip, hip - Vector((0, 0, 0.1)), "pelvis", deform=False)
mid_fa = (J["l"]["forearm"] + J["l"]["hand"]) / 2
bone("socket_shield", mid_fa, mid_fa + Vector((0.08, 0, 0)), "forearm_l", deform=False)
bone("socket_back", C["chest"] + Vector((0, 0.14, 0)), C["chest"] + Vector((0, 0.14, 0.1)), "chest", deform=False)
bone("socket_head", C["head_end"], C["head_end"] + Vector((0, 0, 0.1)), "head", deform=False)
bone("socket_chest", C["chest"] + Vector((0, -0.12, 0)), C["chest"] + Vector((0, -0.22, 0)), "chest", deform=False)
bpy.ops.object.mode_set(mode="OBJECT")
deform = [b.name for b in arm_data.bones if b.use_deform]
print("ossos que deformam:", len(deform), "| total:", len(arm_data.bones))

# ---------- pesos ----------
RENAME = {"Hips": "pelvis", "Spine": "spine_01", "Spine1": "spine_02", "Spine2": "chest", "Neck": "neck", "Head": "head"}
for s, side in SIDES:
    RENAME.update({f"{side}Shoulder": f"clavicle_{s}", f"{side}Arm": f"upperarm_{s}", f"{side}ForeArm": f"forearm_{s}",
                   f"{side}Hand": f"hand_{s}", f"{side}UpLeg": f"thigh_{s}", f"{side}Leg": f"calf_{s}",
                   f"{side}Foot": f"foot_{s}", f"{side}ToeBase": f"toe_{s}"})
    for n in range(1, 5):
        RENAME[f"{side}HandMiddle{n}"] = f"hand_{s}"  # o médio do Meshy volta para a mão; os dedos novos saem da geometria

weights = {}
for v in body.data.vertices:
    d = {}
    for g in v.groups:
        n = RENAME.get(VG[g.group].replace(P, ""))
        if n:
            d[n] = d.get(n, 0) + g.weight
    weights[v.index] = d

NB = {"thumb": 3, "index": 3, "fingers": 2}
for s, _ in SIDES:
    # braço: parte do peso vai para o twist, pela posição ao longo do osso
    a0, a1 = J[s]["upperarm"], J[s]["forearm"]
    ax = a1 - a0
    for i, d in weights.items():
        w = d.get(f"upperarm_{s}")
        if w:
            t = (POS[i] - a0).dot(ax) / ax.length_squared
            tw = max(0.0, min(1.0, (t - 0.35) / 0.4))  # 0 perto do ombro, 1 perto do cotovelo
            d[f"upperarm_{s}"] = w * (1 - tw)
            d[f"upperarm_twist_{s}"] = w * tw
    # dedos: osso pelo parâmetro ao longo do próprio dedo, misturando com o anterior perto da junta
    for i, (fname, t) in FING[s][1].items():
        d = weights[i]
        total = sum(d.values()) or 1.0
        n = NB[fname]
        blend = max(0.0, min(1.0, t / 0.15))  # perto da base, parte fica na mão
        x = t * n
        k = min(n - 1, int(x))
        fr = x - k
        seg = {k: 0.5 + fr * 2, k - 1: 0.5 - fr * 2} if (fr < 0.25 and k > 0) else {k: 1.0}
        for key in list(d):
            d[key] *= (1 - blend)
        for kk, w in seg.items():
            name = f"{fname}_{kk + 1:02d}_{s}"
            d[name] = d.get(name, 0) + total * blend * w

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
for n in [n for n in deform if n.split("_")[0] in ("pelvis", "spine", "chest", "neck", "clavicle", "upperarm", "thigh")]:
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
print("vértices:", len(body.data.vertices), "| triângulos:", sum(len(p.vertices) - 2 for p in body.data.polygons))

bpy.ops.wm.save_as_mainfile(filepath=out_blend, compress=True)
bpy.ops.export_scene.gltf(filepath=out_glb, export_format="GLB", export_animations=False)
print("SALVO", out_blend, out_glb)
