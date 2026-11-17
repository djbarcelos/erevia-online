"""Velocidade natural do passo de cada família, medida no Blender (headless) sobre <id>_uv.glb.

    blender --background --factory-startup --python tools/art/familias-monstro/speeds.py -- <arquivo.glb> <altura-alvo-m> <prefixo> <osso-do-corpo> <osso-de-pe>...

Para cada clipe de andar e correr (<prefixo>Walk e <prefixo>Run): o pé de apoio (o mais baixo) anda para trás em relação ao
corpo a essa velocidade; ela, na escala do modelo no jogo (altura-alvo / altura do corpo), é o FOOT_SPEEDS do models.ts.
Também diz quanto o corpo anda no clipe (deveria ser ~0) e a altura do pé mais baixo. Laços com limite fixo.
"""
import sys

import bpy

argv = sys.argv[sys.argv.index("--") + 1:]
glb, target_h, prefix, body_bone = argv[0], float(argv[1]), argv[2], argv[3]
foot_bones = argv[4:]

bpy.ops.object.select_all(action="SELECT")
bpy.ops.object.delete()
bpy.ops.import_scene.gltf(filepath=glb)
sc = bpy.context.scene
FPS = sc.render.fps / sc.render.fps_base
arm = next(o for o in bpy.data.objects if o.type == "ARMATURE")
mesh = max((o for o in bpy.data.objects if o.type == "MESH"), key=lambda o: len(o.data.vertices))
arm.animation_data_create()


def bounds_h():
    bpy.context.view_layer.update()
    ev = mesh.evaluated_get(bpy.context.evaluated_depsgraph_get())
    zs = [(ev.matrix_world @ v.co).z for v in ev.data.vertices]
    return max(zs) - min(zs)


acts = {a.name: a for a in bpy.data.actions}
arm.animation_data.action = acts[f"{prefix}Idle"]
sc.frame_set(int(acts[f"{prefix}Idle"].frame_range[0]))
h = bounds_h()
k = target_h / h
print("ALTURA_BLENDER", round(h, 3), "escala para o jogo", round(k, 4))


def foot(n):
    pb = arm.pose.bones[n]
    return arm.matrix_world @ pb.tail


for role in ("Walk", "Run"):
    a = acts.get(f"{prefix}{role}")
    if not a:
        continue
    arm.animation_data.action = a
    s, e = a.frame_range
    frames = 48
    rel, body = [], []
    for i in range(frames + 1):
        sc.frame_set(int(s) + int(round((e - s) * i / frames)))
        bpy.context.view_layer.update()
        b = arm.matrix_world @ arm.pose.bones[body_bone].head
        body.append(b)
        rel.append([(foot(n) - b) for n in foot_bones])
    dt = (e - s) / FPS / frames
    min_z = min(f.z for fr in rel for f in fr)
    tot, cnt = 0.0, 0
    for i in range(frames):
        for j in range(len(foot_bones)):
            a0, b0 = rel[i][j], rel[i + 1][j]
            if a0.z > min_z + 0.08 * h or b0.z > min_z + 0.08 * h:
                continue
            tot += ((b0.x - a0.x) ** 2 + (b0.y - a0.y) ** 2) ** 0.5 / dt
            cnt += 1
    sp = (tot / cnt * k) if cnt else 0
    travel = ((body[-1].x - body[0].x) ** 2 + (body[-1].y - body[0].y) ** 2) ** 0.5 * k
    print("VELOCIDADE", role, "clipe", a.name, "segundos", round((e - s) / FPS, 2), "m/s", round(sp, 2), "corpo andou (m)", round(travel, 2), "amostras", cnt)
