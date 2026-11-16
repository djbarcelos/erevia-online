"""Bake do Meshy (alta) para a malha baixa do cavaleiro. Roda no Blender.
Duas passadas: a justa e a de raios longos só para os pixels que a primeira errou."""
import bpy, numpy as np

def run(hi_name="MeshyKnight", hands_name="HiHands", out_png=None, skin=(0.80, 0.52, 0.38)):
    sc = bpy.context.scene
    b = sc.render.bake
    img = bpy.data.images["knight_m_baked"]
    hi = bpy.data.objects[hi_name]
    hands = bpy.data.objects.get(hands_name)
    lows = [bpy.data.objects[n] for n in ("Knight_Body", "Hat")]
    mat = lows[0].data.materials[0]
    mat.node_tree.nodes.active = next(n for n in mat.node_tree.nodes if n.type == "TEX_IMAGE")

    def bake(src, ext, ray):
        b.cage_extrusion, b.max_ray_distance = ext, ray
        px = np.full(1024 * 1024 * 4, 0.5, dtype=np.float32)
        px[3::4] = 1
        img.pixels.foreach_set(px)
        sc.render.engine = "CYCLES"
        b.use_clear = False
        for o in lows:
            for x in bpy.context.selected_objects:
                x.select_set(False)
            src.hide_set(False); src.select_set(True); o.select_set(True)
            bpy.context.view_layer.objects.active = o
            bpy.ops.object.bake(type="DIFFUSE")
        b.use_clear = True
        src.hide_set(True)
        c = np.zeros(1024 * 1024 * 4, dtype=np.float32)
        img.pixels.foreach_get(c)
        c = c.reshape(1024, 1024, 4)
        c[..., :3] = np.clip(c[..., :3] ** 0.72 * 1.15, 0, 1)
        return c

    A = bake(hi, 0.03, 0.08)
    m = np.clip(0.5 ** 0.72 * 1.15, 0, 1)
    un = lambda x: (np.abs(x[..., :3] - m) < 0.004).all(-1)
    out = A
    for src, e, r in ((hands, 0.06, 0.15), (hi, 0.12, 0.4)):
        if src is None:
            continue
        C = bake(src, e, r)
        out = np.where((un(out) & ~un(C))[..., None], C, out)
    sc.render.engine = "BLENDER_EEVEE"
    out[960:1024, 960:1024, :3] = np.array(skin)
    out[960:1024, 960:1024, 3] = 1
    img.pixels.foreach_set(out.ravel())
    if out_png:
        img.filepath_raw = out_png; img.file_format = "PNG"; img.save()
    return int(un(out).sum())
