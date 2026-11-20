"""Heróis v4: veste um kit (personagem inteiro vestido, gerado no Meshy) no esqueleto do contrato.

1. Importa o kit rigado pelo Meshy (mixamo) e posa o esqueleto dele nas juntas do contrato (cada osso vai da
   junta à junta do nosso corpo, com escala no comprimento) -> ombros, cotovelos, joelhos e pulsos coincidem.
2. Aplica a pose na malha (o kit fica no repouso do contrato) e tira o esqueleto do Meshy.
3. Copia os pesos do corpo de alta resolução (Data Transfer, face mais próxima, interpolado): dedos, torção do
   braço e o resto vêm do corpo, e a mão do kit fecha na arma igual à do corpo.
4. Salva um .blend no formato do corpo (`corpo` = kit, `heroi` = esqueleto), pronto para make_lods.py,
   bake_normal.py e o resto do pipeline.

    Blender --background --factory-startup corpo.blend --python tools/art/herois-v4/fit_kit.py -- kit-rigado.glb saida.blend
"""
import sys

import bpy
from mathutils import Matrix, Vector

_args = sys.argv[sys.argv.index("--") + 1:]
kit_glb, out_blend = _args[:2]
CAPA_FILE = _args[_args.index("--capa") + 1] if "--capa" in _args else None  # kit gerado sem capa + capa avulsa
body = bpy.data.objects["corpo"]
arm = bpy.data.objects["heroi"]
before = set(bpy.data.objects)
bpy.ops.import_scene.gltf(filepath=kit_glb)
new = [o for o in bpy.data.objects if o not in before]
for o in list(new):
    if o.type == "MESH" and o.name.startswith("Icosphere"):
        new.remove(o)
        bpy.data.objects.remove(o)
karm = next(o for o in new if o.type == "ARMATURE")
kmesh = next(o for o in new if o.type == "MESH" and o.parent == karm)
P = "mixamorig:"
B = arm.data.bones


def jh(n):
    return arm.matrix_world @ B[n].head_local


def jt(n):
    return arm.matrix_world @ B[n].tail_local


# osso mixamo -> (ponto da cabeça, ponto da cauda) no nosso esqueleto
TGT = {"Hips": (jh("pelvis"), jh("spine_01")), "Spine": (jh("spine_01"), jh("spine_02")), "Spine1": (jh("spine_02"), jh("chest")),
       "Spine2": (jh("chest"), jh("neck")), "Neck": (jh("neck"), jh("head")), "Head": (jh("head"), jt("head"))}
for s, side in (("l", "Left"), ("r", "Right")):
    TGT.update({f"{side}Shoulder": (jh(f"clavicle_{s}"), jh(f"upperarm_{s}")), f"{side}Arm": (jh(f"upperarm_{s}"), jh(f"forearm_{s}")),
                f"{side}ForeArm": (jh(f"forearm_{s}"), jh(f"hand_{s}")), f"{side}Hand": (jh(f"hand_{s}"), jt(f"hand_{s}")),
                f"{side}UpLeg": (jh(f"thigh_{s}"), jh(f"calf_{s}")), f"{side}Leg": (jh(f"calf_{s}"), jh(f"foot_{s}")),
                f"{side}Foot": (jh(f"foot_{s}"), jh(f"toe_{s}")), f"{side}ToeBase": (jh(f"toe_{s}"), jt(f"toe_{s}"))})


def depth(b):
    d = 0
    while b.parent:
        b, d = b.parent, d + 1
    return d


# ---- 1. pose do esqueleto do kit nas nossas juntas ----
bpy.ops.object.select_all(action="DESELECT")
karm.select_set(True)
bpy.context.view_layer.objects.active = karm
bpy.ops.object.mode_set(mode="POSE")
for p in karm.pose.bones:
    p.bone.inherit_scale = "NONE"
kw = karm.matrix_world.inverted()
# a ponta de cada osso do kit é a junta do filho (a cauda do glTF importado é um palpite e esticava a cabeça)
CHILD = {"Hips": "Spine", "Spine": "Spine1", "Spine1": "Spine2", "Spine2": "Neck", "Neck": "Head", "Head": "HeadTop_End"}
for side in ("Left", "Right"):
    CHILD.update({f"{side}Shoulder": f"{side}Arm", f"{side}Arm": f"{side}ForeArm", f"{side}ForeArm": f"{side}Hand",
                  f"{side}UpLeg": f"{side}Leg", f"{side}Leg": f"{side}Foot", f"{side}Foot": f"{side}ToeBase", f"{side}ToeBase": f"{side}Toe_End"})
KB = karm.data.bones
NOSCALE = {"Head", "LeftHand", "RightHand", "LeftToeBase", "RightToeBase"}
moved = 0
for p in sorted(karm.pose.bones, key=lambda b: depth(b.bone)):
    key = p.name.replace(P, "")
    if key not in TGT:
        continue
    h_t, t_t = (kw @ TGT[key][0]), (kw @ TGT[key][1])
    rest = p.bone.matrix_local
    h_k = rest.to_translation()
    child = KB.get(P + CHILD.get(key, ""))
    t_k = child.head_local if child else p.bone.tail_local
    d_k, d_t = (t_k - h_k), (t_t - h_t)
    # a cabeça só muda de lugar: girar até a nossa direção tombava o rosto para cima (o topo da cabeça do rig
    # do Meshy não aponta para cima)
    rot = Matrix.Identity(4) if key == "Head" else d_k.normalized().rotation_difference(d_t.normalized()).to_matrix().to_4x4()
    # cabeça, mão e ponta do pé só giram: as juntas-ponta do rig do Meshy (topo da cabeça, fim do dedão) ficam
    # perto demais da base e a escala esticava a cabeça a 3 m de altura
    k = d_t.length / max(1e-6, d_k.length) if child and key not in NOSCALE else 1.0
    y = rest.to_3x3().col[1].normalized()
    sc = Matrix.Identity(3) + (k - 1) * Matrix([[y.x * y.x, y.x * y.y, y.x * y.z], [y.y * y.x, y.y * y.y, y.y * y.z], [y.z * y.x, y.z * y.y, y.z * y.z]])
    p.matrix = Matrix.Translation(h_t) @ rot @ sc.to_4x4() @ Matrix.Translation(-h_k) @ rest
    bpy.context.view_layer.update()
    moved += 1
bpy.ops.object.mode_set(mode="OBJECT")
print("ossos do kit levados às juntas do contrato:", moved)

# ---- 2. aplica a pose na malha e tira o esqueleto do Meshy ----
bpy.ops.object.select_all(action="DESELECT")
kmesh.select_set(True)
bpy.context.view_layer.objects.active = kmesh
for m in list(kmesh.modifiers):
    if m.type == "ARMATURE":
        bpy.ops.object.modifier_apply(modifier=m.name)
mw = kmesh.matrix_world.copy()
kmesh.parent = None
kmesh.matrix_world = mw
bpy.ops.object.transform_apply(location=True, rotation=True, scale=True)
for g in list(kmesh.vertex_groups):
    kmesh.vertex_groups.remove(g)
for o in new:
    if o != kmesh and o.name in bpy.data.objects:
        bpy.data.objects.remove(o)

# solda costuras de UV (como no build_rig.py) para a transferência e a suavização não rasgarem
import bmesh
bm = bmesh.new()
bm.from_mesh(kmesh.data)
bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)
bm.to_mesh(kmesh.data)
bm.free()

# ---- 3. pesos do corpo ----
for g in body.vertex_groups:
    kmesh.vertex_groups.new(name=g.name)
dt = kmesh.modifiers.new("Pesos", "DATA_TRANSFER")
dt.object = body
dt.use_vert_data = True
dt.data_types_verts = {"VGROUP_WEIGHTS"}
dt.vert_mapping = "POLYINTERP_NEAREST"
dt.layers_vgroup_select_src = "ALL"
dt.layers_vgroup_select_dst = "NAME"
bpy.ops.object.modifier_apply(modifier=dt.name)
bpy.ops.object.mode_set(mode="WEIGHT_PAINT")
bpy.ops.object.vertex_group_limit_total(group_select_mode="ALL", limit=4)
bpy.ops.object.vertex_group_normalize_all(group_select_mode="ALL", lock_active=False)
bpy.ops.object.mode_set(mode="OBJECT")

if not CAPA_FILE:
    # ---- 3c. capa: os vértices de tecido nas costas obedecem aos ossos de capa (a mola do jogo balança) ----
    from mathutils.bvhtree import BVHTree
    dg = bpy.context.evaluated_depsgraph_get()
    bev = body.evaluated_get(dg)
    bvh = BVHTree.FromPolygons([body.matrix_world @ v.co for v in bev.data.vertices], [p.vertices[:] for p in bev.data.polygons])
    c0, c1, c2 = (B["cape_l_01"].head_local, B["cape_l_02"].head_local, B["cape_l_02"].tail_local)
    cx = (B["cape_l_01"].head_local.x + B["cape_r_01"].head_local.x) / 2
    back_y = B["chest"].head_local.y
    t1 = (c0.z - c1.z) / (c0.z - c2.z)
    names = {g.index: g.name for g in kmesh.vertex_groups}
    ARMS = ("upperarm", "forearm", "hand", "thumb", "index", "fingers")  # a clavícula não: o alto da capa pesa nela


    def sstep(e0, e1, x):
        u = max(0.0, min(1.0, (x - e0) / (e1 - e0)))
        return u * u * (3 - 2 * u)


    # só o tecido marrom da capa (pela cor da textura no UV do vértice): o tabardo azul e a cota ficam com o corpo
    img = next(n.image for m in kmesh.data.materials if m and m.use_nodes for n in m.node_tree.nodes if n.type == "TEX_IMAGE" and n.image)
    W, H = img.size
    px = list(img.pixels)
    uv = kmesh.data.uv_layers.active.data
    vuv = {}
    for lp in kmesh.data.loops:
        vuv.setdefault(lp.vertex_index, uv[lp.index].uv.copy())


    def is_cape_color(vi):
        u = vuv.get(vi)
        if u is None:
            return False
        x = min(W - 1, max(0, int(u.x * W)))
        y = min(H - 1, max(0, int(u.y * H)))
        k = (y * W + x) * 4
        r, g, b = px[k], px[k + 1], px[k + 2]
        return r > b * 1.08 and r > 0.06 and (r - b) > 0.02  # marrom: vermelho acima do azul (o tabardo é azulado, a cota é cinza)


    # 1) candidatos: atrás das costas, longe do corpo, fora dos braços, na faixa da capa e com cor de capa
    nv = len(kmesh.data.vertices)
    cand = [False] * nv
    free = [0.0] * nv
    pos = [kmesh.matrix_world @ v.co for v in kmesh.data.vertices]
    for v in kmesh.data.vertices:
        p = pos[v.index]
        if p.y < back_y - 0.02 or p.z > c0.z + 0.16 or p.z < c2.z - 0.15:  # do capuz até a barra
            continue
        if sum(g.weight for g in v.groups if names[g.group].split("_")[0] in ARMS) > 0.35:
            continue
        hit = bvh.find_nearest(p)
        if hit[0] is None:
            continue
        cand[v.index] = is_cape_color(v.index)  # o alto da capa encosta nas costas: a cor decide
        if hit[3] < 0.015:
            continue
        t = (c0.z - p.z) / (c0.z - c2.z)
        free[v.index] = sstep(0.0, 0.14, t) * sstep(0.015, 0.04, hit[3])
    # 2) votação entre vizinhos: a sombra e o avesso da capa (cor mais escura) entram, manchas soltas saem
    nb = [[] for _ in range(nv)]
    for e in kmesh.data.edges:
        a_, b_ = e.vertices
        nb[a_].append(b_)
        nb[b_].append(a_)
    for _ in range(6):
        cand = [(sum(cand[j] for j in nb[i]) * 2 > len(nb[i]) if nb[i] else cand[i]) for i in range(nv)]
    # 3) campos de peso da capa, suavizados pela malha (sem fronteira dura, sem rasgo)
    F = {k: [0.0] * nv for k in ("cape_l_01", "cape_l_02", "cape_r_01", "cape_r_02")}
    for i in range(nv):
        if not cand[i]:
            continue
        p = pos[i]
        t = (c0.z - p.z) / (c0.z - c2.z)
        w2 = sstep(t1 * 0.6, 1.0, t)
        sl = sstep(-0.05, 0.05, p.x - cx)
        f = free[i]
        F["cape_l_01"][i] = f * (1 - w2) * sl
        F["cape_l_02"][i] = f * w2 * sl
        F["cape_r_01"][i] = f * (1 - w2) * (1 - sl)
        F["cape_r_02"][i] = f * w2 * (1 - sl)
    for _ in range(8):
        for k in F:
            a_ = F[k]
            F[k] = [0.5 * a_[i] + 0.5 * (sum(a_[j] for j in nb[i]) / len(nb[i]) if nb[i] else a_[i]) for i in range(nv)]
    cape_n = 0
    for v in kmesh.data.vertices:
        i = v.index
        tot_c = sum(F[k][i] for k in F)
        if tot_c < 1e-3:
            continue
        tot_c = min(1.0, tot_c)
        old = {names[g.group]: g.weight for g in v.groups}
        for gi in [g.group for g in v.groups]:
            kmesh.vertex_groups[gi].remove([i])
        merged = {k: w * (1 - tot_c) for k, w in old.items()}
        s_ = sum(F[k][i] for k in F)
        for k in F:
            if F[k][i] > 0:
                merged[k] = merged.get(k, 0) + F[k][i] / s_ * tot_c
        items = sorted(((k, w) for k, w in merged.items() if w > 1e-4), key=lambda kv: -kv[1])[:4]
        tw = sum(w for _, w in items)
        for k, w in items:
            kmesh.vertex_groups[k].add([i], w / tw, "REPLACE")
        cape_n += 1
    print("vértices da capa nos ossos de capa:", cape_n)

    # ---- 3d. capa independente: vira uma ilha de malha (sem aresta ligada ao corpo), só com ossos de capa e
    # peito; no lugar dela, um forro colado às costas e às pernas (a casca do Meshy não tem nada debaixo) ----
    from mathutils.kdtree import KDTree
    bnames = {g.index: g.name for g in body.vertex_groups}
    bpos = [body.matrix_world @ v.co for v in bev.data.vertices]
    kd = KDTree(len(bpos))
    for i, p in enumerate(bpos):
        kd.insert(p, i)
    kd.balance()
    bw = [{bnames[g.group]: g.weight for g in v.groups} for v in body.data.vertices]
    bm = bmesh.new()
    bm.from_mesh(kmesh.data)
    dl = bm.verts.layers.deform.verify()
    gidx = {g.name: g.index for g in kmesh.vertex_groups}
    CAPE_G = [gidx[k] for k in ("cape_l_01", "cape_l_02", "cape_r_01", "cape_r_02")]
    chest_g = gidx["chest"]


    def cape_w(v):
        d = v[dl]
        return sum(d.get(g, 0.0) for g in CAPE_G)


    cape_set = {f for f in bm.faces if sum(cand[v.index] for v in f.verts) * 2 > len(f.verts)}  # pela cor (votada)
    # fecha buracos: face cercada de capa (2+ vizinhas) e atrás das costas também é capa
    for _ in range(4):
        add = set()
        for f in bm.faces:
            if f in cape_set or (kmesh.matrix_world @ f.calc_center_median()).y < back_y:
                continue
            if sum(1 for e in f.edges for g in e.link_faces if g is not f and g in cape_set) >= 2:
                add.add(f)
        cape_set |= add
    cape_faces = list(cape_set)
    # forro: cópia das faces da capa, assentada no corpo e com os pesos do corpo
    dup = bmesh.ops.duplicate(bm, geom=cape_faces)
    for v in [e for e in dup["geom"] if isinstance(e, bmesh.types.BMVert)]:
        w = kmesh.matrix_world @ v.co
        hit = bvh.find_nearest(w)
        if hit[0] is not None:
            v.co = kmesh.matrix_world.inverted() @ (hit[0] + hit[1] * 0.006)
        _, bi, _ = kd.find(kmesh.matrix_world @ v.co)
        d = v[dl]
        d.clear()
        for n, wt in bw[bi].items():
            if n in gidx and wt > 1e-4:
                d[gidx[n]] = wt
    # corta a capa do resto: as arestas da borda viram duplas (ilha separada)
    border = [e for e in bm.edges if len(e.link_faces) == 2 and ((e.link_faces[0] in cape_set) != (e.link_faces[1] in cape_set))]
    bmesh.ops.split_edges(bm, edges=border)
    # pesos da capa, agora peça solta: só a altura manda (presa nos ombros, solta até a barra); nada das pernas
    # nem do quadril, e a distância ao corpo não conta mais (a barra rente às canelas ficava presa e esticava)
    GL = {s_: (gidx[f"cape_{s_}_01"], gidx[f"cape_{s_}_02"]) for s_ in ("l", "r")}
    cape_verts = {v for f in cape_set for v in f.verts}
    for v in cape_verts:
        p = kmesh.matrix_world @ v.co
        t = (c0.z - p.z) / (c0.z - c2.z)
        fr = sstep(0.0, 0.16, t)
        w2 = sstep(t1 * 0.6, 1.0, t)
        sl = sstep(-0.06, 0.06, p.x - cx)
        d = v[dl]
        d.clear()
        if fr < 1.0:
            d[chest_g] = 1.0 - fr
        for s_, ws in (("l", sl), ("r", 1.0 - sl)):
            if fr * ws * (1 - w2) > 1e-4:
                d[GL[s_][0]] = fr * ws * (1 - w2)
            if fr * ws * w2 > 1e-4:
                d[GL[s_][1]] = fr * ws * w2
    bm.to_mesh(kmesh.data)
    bm.free()
    print("capa independente:", len(cape_faces), "faces; forro colado no corpo, arestas de borda cortadas:", len(border))

# ---- 3b. mãos do corpo no lugar das do kit (os dedos do kit vêm fundidos e viram garra) ----
HANDS = ("hand_", "thumb_", "index_", "fingers_")


def hand_weight(o):
    names = {g.index: g.name for g in o.vertex_groups}
    return [sum(g.weight for g in v.groups if names[g.group].startswith(HANDS)) for v in o.data.vertices]


def keep_only(o, keep):
    bm = bmesh.new()
    bm.from_mesh(o.data)
    bm.verts.ensure_lookup_table()
    bmesh.ops.delete(bm, geom=[v for v in bm.verts if not keep[v.index]], context="VERTS")
    bm.to_mesh(o.data)
    bm.free()


CUT = 0.5  # o corte acompanha o pulso (onde a braçadeira do kit termina)
hw = hand_weight(kmesh)
keep_only(kmesh, [w < CUT for w in hw])
hands = body.copy()
hands.data = body.data.copy()
bpy.context.scene.collection.objects.link(hands)
for m in list(hands.modifiers):
    hands.modifiers.remove(m)
hands.parent = None
hands.matrix_world = body.matrix_world.copy()
hw = hand_weight(hands)
keep_only(hands, [w >= CUT * 0.9 for w in hw])  # um pouco antes do corte: a mão entra sob a braçadeira
bpy.ops.object.select_all(action="DESELECT")
hands.select_set(True)
kmesh.select_set(True)
bpy.context.view_layer.objects.active = kmesh
bpy.ops.object.join()
print("mãos do corpo no kit; materiais:", [m.name for m in kmesh.data.materials])

# ---- 3e. capa avulsa (extract_cape.py): peça solta sobre o kit sem capa, só com ossos de capa e peito ----
if CAPA_FILE:
    with bpy.data.libraries.load(CAPA_FILE, link=False) as (src_, dst_):
        dst_.objects = ["capa"]
    capa = dst_.objects[0]
    bpy.context.scene.collection.objects.link(capa)
    capa.parent = None
    for m in list(capa.modifiers):
        capa.modifiers.remove(m)
    bpy.ops.object.select_all(action="DESELECT")
    capa.select_set(True)
    kmesh.select_set(True)
    bpy.context.view_layer.objects.active = kmesh
    bpy.ops.object.join()
    with bpy.context.temp_override(object=kmesh, active_object=kmesh, selected_objects=[kmesh]):
        bpy.ops.object.material_slot_remove_unused()
    print("capa avulsa vestida; materiais:", [m.name for m in kmesh.data.materials])

# ---- 4. o kit vira o "corpo" ----
kname = "corpo"
bpy.data.objects.remove(body)
kmesh.name = kname
kmesh.data.name = kname
kmesh.parent = arm
mod = kmesh.modifiers.new("Armature", "ARMATURE")
mod.object = arm
print("kit:", len(kmesh.data.vertices), "vértices,", sum(len(p.vertices) - 2 for p in kmesh.data.polygons), "triângulos")
bpy.ops.wm.save_as_mainfile(filepath=out_blend, compress=True)
print("SALVO", out_blend)
