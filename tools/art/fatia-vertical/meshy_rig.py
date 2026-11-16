"""Pesos por distância às cápsulas dos ossos + UV cilíndrica por região (Meshy -> jogo). Roda no Blender.

O smart_project estilhaça malhas decimadas e ruidosas em centenas de ilhas; aqui cada face vai para o
osso dominante e é projetada num cilindro ao redor do eixo dele. As regiões viram ~20 ilhas, empacotadas
em prateleiras com densidade de texel uniforme.
"""
import math
import bpy
import numpy as np
from mathutils import Vector


def seg_dist(P, a, b):
    ab = b - a
    t = np.clip(((P - a) @ ab) / (ab @ ab), 0, 1)
    return np.linalg.norm(P - (a + t[:, None] * ab), axis=1)


def auto_weights(ob, arm, radii, cape_rule=None, extra=None, topn=4):
    """radii: {prefixo do osso (antes do '.'): raio da cápsula}. Devolve (W, nomes, V)."""
    bones = [b for b in arm.data.bones if b.use_deform]
    segs = [(b.name, np.array(b.head_local), np.array(b.tail_local), radii[b.name.split(".")[0]]) for b in bones]
    V = np.array([v.co[:] for v in ob.data.vertices])
    D = np.stack([np.maximum(seg_dist(V, a, b) - r, 0) for n, a, b, r in segs], 1)
    names = [s[0] for s in segs]
    if cape_rule:
        for i, n in enumerate(names):
            if n.startswith("cape"):
                D[:, i] = np.where(cape_rule(V), D[:, i], 9)
    W = 1.0 / (D + 0.02) ** 3
    idx = np.argsort(-W, 1)[:, topn:]
    np.put_along_axis(W, idx, 0, 1)
    W /= W.sum(1, keepdims=True)
    if extra:
        extra(W, V, names)
    for g in list(ob.vertex_groups):
        ob.vertex_groups.remove(g)
    vg = {n: ob.vertex_groups.new(name=n) for n in names}
    for vi in range(len(V)):
        for j in np.nonzero(W[vi] > 0.01)[0]:
            vg[names[j]].add([vi], float(W[vi, j]), "REPLACE")
    return W, names, V


def cylinder_uv(ob, arm, W, names, fit=0.94, pad_px=6, size=1024):
    me = ob.data
    uvl = me.uv_layers.active or me.uv_layers.new(name="UVMap")
    bonemap = {b.name: b for b in arm.data.bones}
    dom = W.argmax(1)
    face_region = {}
    for p in me.polygons:
        face_region[p.index] = int(np.bincount(dom[list(p.vertices)], minlength=len(names)).argmax())
    P = np.array([v.co[:] for v in me.vertices])
    regions = {}
    for pi, ri in face_region.items():
        regions.setdefault(ri, []).append(pi)
    raw = {}
    for ri, faces in regions.items():
        b = bonemap[names[ri]]
        h, t = np.array(b.head_local), np.array(b.tail_local)
        a = (t - h) / np.linalg.norm(t - h)
        ref = np.array([0.0, 1.0, 0.0]) if abs(a[1]) < 0.9 else np.array([1.0, 0.0, 0.0])
        e1 = np.cross(a, ref); e1 /= np.linalg.norm(e1)
        e2 = np.cross(a, e1)
        loops = {}
        rs = []
        for fi in faces:
            pl = me.polygons[fi]
            th = []
            for li in pl.loop_indices:
                d = P[me.loops[li].vertex_index] - h
                x, y = d @ e1, d @ e2
                th.append(math.atan2(y, x) / (2 * math.pi))
                rs.append(math.hypot(x, y))
                loops[li] = [th[-1], d @ a]
            ref_u = th[0]
            for li, u in zip(pl.loop_indices, th):
                while u - ref_u > 0.5: u -= 1
                while u - ref_u < -0.5: u += 1
                loops[li][0] = u
        us = [v[0] for v in loops.values()]
        ts = [v[1] for v in loops.values()]
        rm = max(0.02, float(np.mean(rs)))
        width = (max(us) - min(us)) * 2 * math.pi * rm
        height = max(ts) - min(ts)
        raw[ri] = (loops, min(us), min(ts), max(width, 0.01), max(height, 0.01))
    # empacota (prateleiras) com busca binária da escala
    def pack(s):
        x = y = rowh = 0.0
        pos = {}
        pad = pad_px / size / fit * 1.0
        for ri in sorted(raw, key=lambda r: -raw[r][4]):
            w, h = raw[ri][3] * s + pad, raw[ri][4] * s + pad
            if x + w > 1.0:
                x, y, rowh = 0.0, y + rowh, 0.0
            pos[ri] = (x, y)
            x += w; rowh = max(rowh, h)
        return pos, y + rowh
    lo, hi_ = 0.01, 50.0
    for _ in range(40):
        mid = (lo + hi_) / 2
        _, tot = pack(mid)
        if tot <= 1.0: lo = mid
        else: hi_ = mid
    s = lo
    pos, _ = pack(s)
    for ri, (loops, u0, t0, w, h) in raw.items():
        ox, oy = pos[ri]
        wspan = max(v[0] for v in loops.values()) - u0
        for li, (u, t) in loops.items():
            uu = ox + ((u - u0) / max(wspan, 1e-6)) * w * s
            vv = oy + (t - t0) * s
            uvl.data[li].uv = (uu * fit, vv * fit)
    return s, len(raw)
