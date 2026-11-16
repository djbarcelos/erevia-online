"""Esqueleto guerreiro do Meshy -> jogo. Roda no Blender; cada fase salva um .blend em assets-src.
Uso: exec(open(este).read()); phase1(); phase2(); phase3()   (uma fase por chamada)."""
import os, math
import bpy, bmesh
import numpy as np
from mathutils.bvhtree import BVHTree

ROOT = os.environ.get("FATIA_DIR", r"C:\Users\DanielTeofilo\erevia-wt\fatia-vertical")
OUT = os.path.join(ROOT, "assets-src/fatia-vertical")
exec(open(os.path.join(ROOT, "tools/art/fatia-vertical/lib.py"), encoding="utf-8").read())
exec(open(os.path.join(ROOT, "tools/art/fatia-vertical/meshy_rig.py"), encoding="utf-8").read())


def save(name):
    bpy.ops.wm.save_as_mainfile(filepath=os.path.join(OUT, name))


def decimate_to(o, target, max_iter=6):
    me = o.data
    mods = [(m.type, m.object) for m in o.modifiers if m.type == "ARMATURE"]
    for m in [m for m in o.modifiers if m.type == "ARMATURE"]:
        o.modifiers.remove(m)
    me.calc_loop_triangles()
    cur = len(me.loop_triangles)
    for _ in range(max_iter):
        if cur <= target * 1.05:
            break
        m = o.modifiers.new("d", "DECIMATE")
        m.ratio = max(target / cur, 0.1)
        m.use_collapse_triangulate = True
        with bpy.context.temp_override(object=o, active_object=o):
            bpy.ops.object.modifier_apply(modifier="d")
        me.calc_loop_triangles()
        new = len(me.loop_triangles)
        if new >= cur:  # não reduziu: para
            break
        cur = new
    for _t, ob in mods:
        mm = o.modifiers.new("Armature", "ARMATURE"); mm.object = ob
    return cur


def phase1():
    """Solda, decima para ~3.900, confere normais, cria material e imagem."""
    sc = bpy.context.scene
    hi = bpy.data.objects["MeshySkeleton"]
    me = hi.data.copy()
    bm = bmesh.new(); bm.from_mesh(me)
    bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=2e-5)
    bmesh.ops.delete(bm, geom=[v for v in bm.verts if not v.link_faces], context="VERTS")
    bm.to_mesh(me); bm.free()
    lo = bpy.data.objects.new("Skeleton_Body", me)
    sc.collection.objects.link(lo)
    print("tris", decimate_to(lo, 3900))
    bvh = BVHTree.FromObject(hi, bpy.context.evaluated_depsgraph_get())
    bm = bmesh.new(); bm.from_mesh(me)
    fl = [f for f in bm.faces if (lambda r: r[0] is not None and r[1].dot(f.normal) < 0)(bvh.find_nearest(f.calc_center_median()))]
    bmesh.ops.reverse_faces(bm, faces=fl); bm.to_mesh(me); bm.free()
    for p in me.polygons:
        p.use_smooth = True
    img = bpy.data.images.new("skel_m_baked", 1024, 1024, alpha=False)
    mat = bpy.data.materials.new("Skeleton"); mat.use_nodes = True
    nt = mat.node_tree
    tex = nt.nodes.new("ShaderNodeTexImage"); tex.image = img
    bsdf = next(n for n in nt.nodes if n.type == "BSDF_PRINCIPLED")
    nt.links.new(tex.outputs["Color"], bsdf.inputs["Base Color"]); nt.nodes.active = tex
    me.materials.clear(); me.materials.append(mat)
    save("skeleton_phase1.blend")


def phase2():
    """Armadura, pesos, UV cilíndrica, bake. Antes: phase1."""
    SLOT_ROLL = -90.0
    spec = [
        ("hips", None, (0, 0.03, 0.88), (0, 0.03, 1.0), True),
        ("spine", "hips", (0, 0.03, 1.0), (0, 0.03, 1.15), True),
        ("chest", "spine", (0, 0.03, 1.15), (0, 0.02, 1.43), True),
        ("neck", "chest", (0, 0.02, 1.43), (0, -0.03, 1.52), True),
        ("head", "neck", (0, -0.03, 1.52), (0, -0.04, 1.70), True),
    ]
    for s, n in ((1, ".l"), (-1, ".r")):
        spec += [
            ("upperarm" + n, "chest", (0.20 * s, 0.02, 1.40), (0.35 * s, 0.03, 1.05), True),
            ("lowerarm" + n, "upperarm" + n, (0.35 * s, 0.03, 1.05), (0.44 * s, 0.0, 0.85), True),
            ("hand" + n, "lowerarm" + n, (0.44 * s, 0.0, 0.85), (0.49 * s, -0.01, 0.66), True),
            ("handslot" + n, "hand" + n, (0.47 * s, -0.01, 0.74), (0.47 * s, -0.09, 0.74), False, SLOT_ROLL * s),
            ("upperleg" + n, "hips", (0.10 * s, 0.03, 0.92), (0.14 * s, 0.03, 0.50), True),
            ("lowerleg" + n, "upperleg" + n, (0.14 * s, 0.03, 0.50), (0.21 * s, 0.06, 0.10), True),
            ("foot" + n, "lowerleg" + n, (0.21 * s, 0.06, 0.10), (0.22 * s, -0.10, 0.03), True),
        ]
    arm = make_armature("Skeleton_Rig", spec)
    lo = bpy.data.objects["Skeleton_Body"]
    R = {"hips": .12, "spine": .09, "chest": .14, "neck": .04, "head": .09, "upperarm": .04, "lowerarm": .04,
         "hand": .04, "upperleg": .05, "lowerleg": .05, "foot": .04}
    W, names, V = auto_weights(lo, arm, R)
    cylinder_uv(lo, arm, W, names)
    lo.parent = arm
    m = lo.modifiers.new("Armature", "ARMATURE"); m.object = arm
    # bake
    sc = bpy.context.scene; b = sc.render.bake
    img = bpy.data.images["skel_m_baked"]; hi = bpy.data.objects["MeshySkeleton"]
    mat = lo.data.materials[0]; mat.node_tree.nodes.active = next(n for n in mat.node_tree.nodes if n.type == "TEX_IMAGE")
    b.use_selected_to_active = True; b.margin = 6; b.margin_type = "EXTEND"
    b.use_pass_direct = False; b.use_pass_indirect = False; b.use_pass_color = True

    def bake(ext, ray):
        b.cage_extrusion, b.max_ray_distance = ext, ray
        px = np.full(1024 * 1024 * 4, 0.5, dtype=np.float32); px[3::4] = 1
        img.pixels.foreach_set(px)
        sc.render.engine = "CYCLES"; b.use_clear = False
        for x in bpy.context.selected_objects: x.select_set(False)
        hi.hide_set(False); lo.hide_set(False); hi.select_set(True); lo.select_set(True)
        bpy.context.view_layer.objects.active = lo
        bpy.ops.object.bake(type="DIFFUSE"); b.use_clear = True; hi.hide_set(True)
        c = np.zeros(1024 * 1024 * 4, dtype=np.float32); img.pixels.foreach_get(c); c = c.reshape(1024, 1024, 4)
        c[..., :3] = np.clip(c[..., :3] ** 0.8 * 1.1, 0, 1)
        return c
    A = bake(0.05, 0.1); C = bake(0.1, 0.25)
    sc.render.engine = "BLENDER_EEVEE"
    mk = np.clip(0.5 ** 0.8 * 1.1, 0, 1)
    un = lambda x: (np.abs(x[..., :3] - mk) < 0.004).all(-1)
    out = np.where((un(A) & ~un(C))[..., None], C, A)
    img.pixels.foreach_set(out.ravel())
    img.filepath_raw = os.path.join(OUT, "skel_m_baked.png"); img.file_format = "PNG"; img.save()
    save("skeleton_phase2.blend")


def phase3():
    """Clipes, LODs, export."""
    arm = bpy.data.objects["Skeleton_Rig"]
    ARM_ADD = 22.0
    _key = Animator.key

    def key(self, frame, rots=None, locs=None, ease=None):
        rots = dict(rots or {})
        for n, s in (("upperarm.l", 1), ("upperarm.r", -1)):
            r = list(rots.get(n, (0, 0, 0))); r[1] += s * ARM_ADD; rots[n] = tuple(r)
        return _key(self, frame, rots, locs, ease)
    Animator.key = key
    exec(open(os.path.join(ROOT, "tools/art/fatia-vertical/biped_anim.py"), encoding="utf-8").read(), globals())
    build_clips(arm, "SW_")
    Animator.key = _key
    res = []
    for name, target in (("Skeleton_Body_LOD1", 1400), ("Skeleton_Body_LOD2", 450)):
        s = bpy.data.objects["Skeleton_Body"]
        o = s.copy(); o.data = s.data.copy(); o.name = name
        bpy.context.scene.collection.objects.link(o)
        o.parent = arm; o.modifiers.clear()
        m = o.modifiers.new("Armature", "ARMATURE"); m.object = arm
        res.append(decimate_to(o, target))
    bpy.data.images["skel_m_baked"].pack()
    for o in bpy.data.objects:
        o.hide_set(not (o.name.startswith("Skeleton")))
    arm.animation_data.action = None
    export_glb(os.path.join(OUT, "MN_SkeletonWarrior_v2.glb"), arm)
    save("skeleton_v2.blend")
    return res
