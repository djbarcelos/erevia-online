"""Heróis v4: recorta a capa de um kit já encaixado (fit_kit.py) como peça avulsa.

A capa do kit encaixado é uma ilha de malha só com pesos de capa e peito (fit_kit.py, 3d). Aqui essa ilha
sai como objeto `capa` (material de dupla face: dá para ver o avesso quando ela balança) num .blend próprio,
para vestir sobre um kit gerado sem capa (fit_kit.py ... --capa capa.blend).

    Blender --background --factory-startup kit-corpo.blend --python tools/art/herois-v4/extract_cape.py -- capa.blend
"""
import sys

import bmesh
import bpy

out = sys.argv[sys.argv.index("--") + 1]
kit = bpy.data.objects["corpo"]
names = {g.index: g.name for g in kit.vertex_groups}
CAPE_OK = ("cape_", "chest")
cape = kit.copy()
cape.data = kit.data.copy()
bpy.context.scene.collection.objects.link(cape)
cape.name = "capa"
cape.data.name = "capa"
bm = bmesh.new()
bm.from_mesh(cape.data)
dl = bm.verts.layers.deform.verify()


def is_cape_vert(v):
    d = v[dl]
    return any(names[g].startswith("cape_") for g in d.keys()) and all(names[g].startswith(CAPE_OK) for g in d.keys())


def only_cape_chest(v):
    return all(names[g].startswith(CAPE_OK) for g in v[dl].keys())


# a ilha da capa: faces só com capa/peito, ligadas a pelo menos um ponto com peso de capa (o alto da capa,
# preso no peito, entra; pedaços do corpo só com peito ficam fora porque não tocam a ilha, que foi cortada)
cand = {f for f in bm.faces if all(only_cape_chest(v) for v in f.verts)}
keep = set()
seen = set()
for f0 in cand:
    if f0 in seen:
        continue
    comp, stack = [], [f0]
    seen.add(f0)
    while stack:
        f = stack.pop()
        comp.append(f)
        for e in f.edges:
            for g in e.link_faces:
                if g in cand and g not in seen:
                    seen.add(g)
                    stack.append(g)
    if any(is_cape_vert(v) for f in comp for v in f.verts):
        keep.update(comp)
drop = [f for f in bm.faces if f not in keep]
bmesh.ops.delete(bm, geom=drop, context="FACES")
bmesh.ops.delete(bm, geom=[v for v in bm.verts if not v.link_faces], context="VERTS")
bm.to_mesh(cape.data)
bm.free()
for m in cape.data.materials:
    if m:
        m.use_backface_culling = False  # dupla face no glTF
bpy.data.objects.remove(kit)
for o in [o for o in bpy.data.objects if o not in (cape, cape.parent) and o.type == "MESH"]:
    bpy.data.objects.remove(o)
print("capa:", len(cape.data.vertices), "vértices,", sum(len(p.vertices) - 2 for p in cape.data.polygons), "triângulos")
bpy.ops.wm.save_as_mainfile(filepath=out, compress=True)
print("SALVO", out)
