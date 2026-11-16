"""
Biblioteca da fatia vertical (tunnel#93): modela personagens do estilo B em bpy puro.

Roda DENTRO do Blender (execute_blender_code / exec). Nada de rede nem de gerador.
- Atlas de paleta 8x4 (mesmo layout do KayKit, 128x256): cada parte usa uma célula e o degradê
  vertical da célula dá o volume pintado (luz de cima, sombra embaixo). O cliente repinta pele,
  cabelo e roupa trocando as células, como já faz com o KayKit.
- Rig de corpo rígido: cada vértice com peso 1 num osso (armadura em placas) + esferas nas juntas.
- LOD0/1/2 gerados pelo mesmo script com menos segmentos.
"""
import math
import bpy
import bmesh
import numpy as np
from mathutils import Vector, Matrix, Euler, Quaternion

# (segmentos de cilindro, (anéis, segmentos) de esfera) por LOD
SEGS = [(14, (9, 14)), (9, (6, 9)), (6, (4, 6))]
ATLAS_W, ATLAS_H = 128, 256


def hexrgb(h):
    h = h.lstrip("#")
    return tuple(int(h[i:i + 2], 16) / 255 for i in (0, 2, 4))


def shade(hexcol, k):
    r, g, b = hexrgb(hexcol)
    return (min(1, r * k), min(1, g * k), min(1, b * k))


def make_atlas(path, cells):
    """cells: {(row,col): (hex_topo, hex_base)}. Linha 0 = topo da imagem. Grava PNG 128x256."""
    img = np.zeros((ATLAS_H, ATLAS_W, 4), dtype=np.float32)
    img[..., 3] = 1
    img[..., :3] = 0.5
    cw, ch = ATLAS_W // 8, ATLAS_H // 4
    for (row, col), (top, bot) in cells.items():
        t, b = np.array(hexrgb(top)), np.array(hexrgb(bot))
        for y in range(ch):
            k = y / (ch - 1)
            img[row * ch + y, col * cw:(col + 1) * cw, :3] = t + (b - t) * k
    # srgb -> o PNG guarda o valor como está (a imagem é marcada sRGB)
    im = bpy.data.images.get("atlas") or bpy.data.images.new("atlas", ATLAS_W, ATLAS_H, alpha=False)
    im.scale(ATLAS_W, ATLAS_H)
    flipped = img[::-1]  # o Blender guarda de baixo para cima
    im.pixels.foreach_set(np.ascontiguousarray(flipped, dtype=np.float32).ravel())
    im.filepath_raw = path
    im.file_format = "PNG"
    im.save()
    im.pack()
    return im


def make_material(name, image):
    mat = bpy.data.materials.get(name) or bpy.data.materials.new(name)
    mat.use_nodes = True
    nt = mat.node_tree
    nt.nodes.clear()
    out = nt.nodes.new("ShaderNodeOutputMaterial")
    bsdf = nt.nodes.new("ShaderNodeBsdfPrincipled")
    tex = nt.nodes.new("ShaderNodeTexImage")
    tex.image = image
    tex.interpolation = "Closest"
    bsdf.inputs["Roughness"].default_value = 0.75
    bsdf.inputs["Metallic"].default_value = 0.0
    nt.links.new(tex.outputs["Color"], bsdf.inputs["Base Color"])
    nt.links.new(bsdf.outputs["BSDF"], out.inputs["Surface"])
    return mat


def cell_uv(cell, t):
    """UV dentro da célula (row, col); t = 0 (base) .. 1 (topo da peça)."""
    row, col = cell
    u = (col + 0.5) / 8
    vt, vb = 1 - row / 4, 1 - (row + 1) / 4
    t = min(1, max(0, t))
    return u, vb + (vt - vb) * (0.07 + 0.86 * t)


class Builder:
    def __init__(self, q, bone_names):
        self.q = q
        self.seg, self.sph = SEGS[q]
        self.bm = bmesh.new()
        self.uv = self.bm.loops.layers.uv.new("UVMap")
        self.dl = self.bm.verts.layers.deform.new()
        self.bones = list(bone_names)

    # ---- núcleo -------------------------------------------------------------------------
    def _done(self, verts, bone, cell, zrange=None):
        bm = self.bm
        vs = [v for v in verts if v.is_valid]
        zmin = min(v.co.z for v in vs)
        zmax = max(v.co.z for v in vs)
        if zrange:
            zmin, zmax = zrange
        span = max(1e-5, zmax - zmin)
        idx = self.bones.index(bone)
        for v in vs:
            v[self.dl][idx] = 1.0
        faces = {f for v in vs for f in v.link_faces}
        for f in faces:
            for l in f.loops:
                l[self.uv].uv = cell_uv(cell, (l.vert.co.z - zmin) / span)

    def tube(self, p0, p1, r0, r1, bone, cell, sx=1.0, sy=1.0, caps=True, seg=None):
        p0, p1 = Vector(p0), Vector(p1)
        d = p1 - p0
        L = d.length
        rot = Vector((0, 0, 1)).rotation_difference(d.normalized()).to_matrix().to_4x4()
        m = Matrix.Translation((p0 + p1) / 2) @ rot @ Matrix.Diagonal((sx, sy, 1, 1))
        r = bmesh.ops.create_cone(self.bm, cap_ends=caps, cap_tris=False, segments=seg or self.seg,
                                  radius1=r0, radius2=r1, depth=L, matrix=m)
        self._done(r["verts"], bone, cell)

    def ell(self, c, radii, bone, cell, rot=(0, 0, 0), res=None):
        m = Matrix.Translation(c) @ Euler([math.radians(a) for a in rot]).to_matrix().to_4x4() @ \
            Matrix.Diagonal((radii[0], radii[1], radii[2], 1))
        ring, segs = res if (res and self.q == 0) else (min(res[0], self.sph[0]), min(res[1], self.sph[1])) if res else self.sph
        r = bmesh.ops.create_uvsphere(self.bm, u_segments=segs, v_segments=ring, radius=1.0, matrix=m)
        self._done(r["verts"], bone, cell)

    def box(self, c, size, bone, cell, rot=(0, 0, 0), taper=None, zrange=None):
        """taper=(escala_x_topo, escala_x_base) afunila no eixo local X."""
        m = Matrix.Translation(c) @ Euler([math.radians(a) for a in rot]).to_matrix().to_4x4() @ \
            Matrix.Diagonal((size[0], size[1], size[2], 1))
        r = bmesh.ops.create_cube(self.bm, size=1.0, matrix=m)
        vs = r["verts"]
        if taper:
            inv = m.inverted()
            for v in vs:
                lp = inv @ v.co
                t = lp.z + 0.5  # 0 base .. 1 topo no espaço local
                lp.x *= taper[1] + (taper[0] - taper[1]) * t
                v.co = m @ lp
        self._done(vs, bone, cell, zrange)

    def prism(self, pts, z0, z1, bone, cell):
        """Prisma reto sobre o polígono 2D `pts` (sentido anti-horário), de z0 a z1."""
        bm = self.bm
        top = [bm.verts.new((x, y, z1)) for x, y in pts]
        bot = [bm.verts.new((x, y, z0)) for x, y in pts]
        n = len(pts)
        bm.faces.new(top)
        for i in range(n):
            j = (i + 1) % n
            bm.faces.new((bot[i], bot[j], top[j], top[i]))
        self._done(top + bot, bone, cell)

    def disc(self, c, r, thick, bone, cell, axis=(0, -1, 0), r2=None, seg=None):
        """Disco (escudo) com o eixo apontando para `axis`."""
        self.tube(Vector(c) - Vector(axis).normalized() * thick / 2, Vector(c) + Vector(axis).normalized() * thick / 2,
                  r, r2 if r2 is not None else r, bone, cell, seg=seg)

    # ---- fim ----------------------------------------------------------------------------
    def finish(self, name, mat, armature, angle=40):
        bm = self.bm
        for f in bm.faces:
            f.smooth = True
        lim = math.radians(angle)
        for e in bm.edges:
            if len(e.link_faces) == 2:
                e.smooth = e.calc_face_angle(0) < lim
        bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)
        me = bpy.data.meshes.new(name)
        ob = bpy.data.objects.new(name, me)
        bpy.context.scene.collection.objects.link(ob)
        for b in self.bones:
            ob.vertex_groups.new(name=b)
        bm.to_mesh(me)
        bm.free()
        me.materials.append(mat)
        if armature:
            ob.parent = armature
            mod = ob.modifiers.new("Armature", "ARMATURE")
            mod.object = armature
        return ob


def tri_count(ob):
    me = ob.data
    me.calc_loop_triangles()
    return len(me.loop_triangles)


# ---- esqueleto ---------------------------------------------------------------------------
def make_armature(name, spec):
    """spec: lista (osso, pai, cabeça, cauda, deforma[, roll]) em metros."""
    arm = bpy.data.armatures.new(name)
    ob = bpy.data.objects.new(name, arm)
    bpy.context.scene.collection.objects.link(ob)
    bpy.context.view_layer.objects.active = ob
    bpy.ops.object.mode_set(mode="EDIT")
    for s in spec:
        b = arm.edit_bones.new(s[0])
        b.head, b.tail = Vector(s[2]), Vector(s[3])
        b.use_deform = s[4]
        if len(s) > 5:
            b.roll = math.radians(s[5])
    for s in spec:
        if s[1]:
            arm.edit_bones[s[0]].parent = arm.edit_bones[s[1]]
    bpy.ops.object.mode_set(mode="OBJECT")
    arm.display_type = "STICK"
    return ob


# ---- animação ----------------------------------------------------------------------------
class Animator:
    """Chaves em eixos do mundo (X = balançar para trás, -Y = frente), por osso."""

    def __init__(self, arm, fps=30):
        self.arm = arm
        self.fps = fps
        bpy.context.scene.render.fps = fps
        arm.animation_data_create()
        self.names = [b.name for b in arm.data.bones]
        self.deform_only = {b.name for b in arm.data.bones if b.use_deform}
        for pb in arm.pose.bones:
            pb.rotation_mode = "QUATERNION"
        self.M = {b.name: b.matrix_local.to_quaternion() for b in arm.data.bones}
        self.Mfull = {b.name: b.matrix_local.copy() for b in arm.data.bones}

    def begin(self, name):
        arm = self.arm
        for pb in arm.pose.bones:
            pb.rotation_quaternion = (1, 0, 0, 0)
            pb.location = (0, 0, 0)
        self.act = bpy.data.actions.new(name)
        self.act.use_fake_user = True
        arm.animation_data.action = self.act
        self.last = 0

    def key(self, frame, rots=None, locs=None, ease=None):
        """rots: {osso: (rx, ry, rz) graus em eixos do mundo, relativos ao pai}; locs: {osso: (dx,dy,dz) m}."""
        rots, locs = rots or {}, locs or {}
        for n in self.names:
            pb = self.arm.pose.bones[n]
            if n not in self.deform_only:
                continue
            d = Euler([math.radians(a) for a in rots.get(n, (0, 0, 0))], "XYZ").to_quaternion()
            M = self.M[n]
            pb.rotation_quaternion = M.inverted() @ d @ M
            dl = Vector(locs.get(n, (0, 0, 0)))
            pb.location = M.inverted() @ dl
            pb.keyframe_insert("rotation_quaternion", frame=frame)
            pb.keyframe_insert("location", frame=frame)
        self.last = frame

    def end(self, loop=True):
        arm = self.arm
        act = self.act
        end = self.last
        arm.animation_data.action = None
        tr = arm.animation_data.nla_tracks.new()
        tr.name = act.name
        st = tr.strips.new(act.name, 0, act)
        return end


def cyc(f, n):
    return 2 * math.pi * f / n


def lerp(a, b, t):
    return a + (b - a) * t


def export_glb(path, arm):
    """Exporta a armadura, as malhas filhas (todos os LODs) e os clipes das trilhas NLA."""
    for t in arm.animation_data.nla_tracks:
        t.mute = False
        t.is_solo = False
    for o in bpy.data.objects:
        o.hide_set(False)
        o.select_set(False)
    arm.select_set(True)
    for c in arm.children:
        c.select_set(True)
    bpy.context.view_layer.objects.active = arm
    bpy.ops.export_scene.gltf(
        filepath=path, export_format="GLB", use_selection=True, export_animations=True,
        export_animation_mode="NLA_TRACKS", export_skins=True, export_yup=True, export_apply=False,
        export_image_format="AUTO", export_cameras=False, export_lights=False,
        export_optimize_animation_size=True, export_anim_single_armature=True)
