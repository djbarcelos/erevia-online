"""Confere a malha de um GLB: ilhas soltas, arestas abertas e vértices soltos.
    blender --background --factory-startup --python tools/art/remaster-v3/mesh-check.py -- <arquivo.glb>
"""
import sys
import bpy
import bmesh

bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()
bpy.ops.import_scene.gltf(filepath=sys.argv[sys.argv.index("--") + 1])
for o in [o for o in bpy.data.objects if o.type == "MESH"]:
    bm = bmesh.new()
    bm.from_mesh(o.data)
    # ilhas pela geometria (vértices na mesma posição são soldados antes de contar)
    bmesh.ops.remove_doubles(bm, verts=bm.verts, dist=1e-5)
    seen, islands = set(), []
    for f in bm.faces:
        if f.index in seen:
            continue
        stack, n = [f], 0
        seen.add(f.index)
        for _ in range(len(bm.faces) + 1):  # limite de iterações
            if not stack:
                break
            cur = stack.pop()
            n += 1
            for e in cur.edges:
                for g in e.link_faces:
                    if g.index not in seen:
                        seen.add(g.index)
                        stack.append(g)
        islands.append(n)
    islands.sort(reverse=True)
    boundary = sum(1 for e in bm.edges if e.is_boundary)
    print(f"MALHA {o.name}: {len(bm.faces)} faces, ilhas {len(islands)} (maiores {islands[:8]}, menores {islands[-5:]}), arestas abertas {boundary}, nao-manifold {sum(1 for e in bm.edges if not e.is_manifold)}")
    bm.free()
