"""Conta as ilhas de UV de uma malha depois de desdobrar com diferentes parâmetros (diagnóstico).
    blender --background --factory-startup --python uvcount.py -- <arquivo.glb>
"""
import math
import sys

import bpy

bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()
bpy.ops.import_scene.gltf(filepath=sys.argv[sys.argv.index("--") + 1])
m = max((o for o in bpy.data.objects if o.type == "MESH"), key=lambda o: len(o.data.vertices))
bpy.context.view_layer.objects.active = m
m.select_set(True)


def islands(me):
    uv = me.uv_layers[0].data
    par = list(range(len(me.polygons)))

    def f(a):
        for _ in range(100000):
            if par[a] == a:
                return a
            par[a] = par[par[a]]
            a = par[a]
        return a

    seen = {}
    for p in me.polygons:
        for l in p.loop_indices:
            k = (me.loops[l].vertex_index, round(uv[l].uv.x, 4), round(uv[l].uv.y, 4))
            if k in seen:
                par[f(p.index)] = f(seen[k])
            else:
                seen[k] = p.index
    return len({f(i) for i in range(len(par))})


me = m.data
print("FACES", len(me.polygons), "VERTS", len(me.vertices))
for ang in (math.radians(66), math.radians(85), math.radians(89)):
    bpy.ops.object.mode_set(mode="EDIT")
    bpy.ops.mesh.select_all(action="SELECT")
    bpy.ops.uv.smart_project(angle_limit=ang, island_margin=0.01)
    bpy.ops.object.mode_set(mode="OBJECT")
    print("ANG", round(math.degrees(ang)), "ILHAS", islands(me))
# desdobramento por costuras: aresta é costura se o ângulo entre as faces passa do limite (o mesmo critério), depois unwrap
bpy.ops.object.mode_set(mode="EDIT")
bpy.ops.mesh.select_all(action="SELECT")
bpy.ops.mesh.remove_doubles(threshold=0.0005)
bpy.ops.mesh.mark_seam(clear=True)
bpy.ops.mesh.edges_select_sharp(sharpness=math.radians(70))
bpy.ops.mesh.mark_seam(clear=False)
bpy.ops.mesh.select_all(action="SELECT")
bpy.ops.uv.unwrap(method="CONFORMAL", margin=0.01)
bpy.ops.object.mode_set(mode="OBJECT")
print("SEAMS", "ILHAS", islands(me), "VERTS", len(me.vertices))
for thr in (0.0005, 0.003):
    bpy.ops.object.mode_set(mode="EDIT")
    bpy.ops.mesh.select_all(action="SELECT")
    bpy.ops.mesh.remove_doubles(threshold=thr)
    bpy.ops.mesh.mark_seam(clear=True)
    bpy.ops.uv.smart_project(angle_limit=math.radians(70), island_margin=0.01)
    bpy.ops.object.mode_set(mode="OBJECT")
    print("WELD", thr, "ILHAS", islands(me), "VERTS", len(me.vertices), "FACES", len(me.polygons))
