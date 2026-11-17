"""Prints do Blender (Workbench, textura) em PNG para montar em JPG. Roda no Blender.
render_views(prefix, rig, names, outdir): 4 ângulos em repouso + 3 poses (correr, golpe, morte)."""
import os, math
import bpy
from mathutils import Vector


def _setup(names):
    sc = bpy.context.scene
    sc.render.engine = "BLENDER_WORKBENCH"
    sc.render.resolution_x = sc.render.resolution_y = 640
    sc.render.image_settings.file_format = "PNG"
    sh = sc.display.shading
    sh.light = "STUDIO"; sh.color_type = "TEXTURE"; sh.show_cavity = False
    sc.world.use_nodes = False
    sc.world.color = (0.16, 0.17, 0.2)
    cam = bpy.data.objects.get("PrintCam") or bpy.data.objects.new("PrintCam", bpy.data.cameras.new("PrintCam"))
    if cam.name not in sc.collection.objects:
        sc.collection.objects.link(cam)
    cam.data.type = "ORTHO"; cam.data.ortho_scale = 2.3
    sc.camera = cam
    for o in bpy.data.objects:
        if o.type == "MESH":
            o.hide_render = not any(o.name.startswith(n) for n in names)
    return sc, cam


def _shot(sc, cam, direction, path, z=0.9):
    d = Vector(direction).normalized()
    cam.location = Vector((0, 0, z)) + d * 6
    cam.rotation_euler = (-d).to_track_quat("-Z", "Y").to_euler()
    sc.render.filepath = path
    bpy.ops.render.render(write_still=True)


def render_views(prefix, rig, names, outdir):
    sc, cam = _setup(names)
    arm = bpy.data.objects[rig]
    for t in arm.animation_data.nla_tracks:
        t.mute = True
    arm.animation_data.action = None
    files = []
    for tag, d in (("front", (0, -1, 0)), ("right", (1, 0, 0)), ("back", (0, 1, 0)), ("tq", (0.7, -0.7, 0.25))):
        p = os.path.join(outdir, f"{prefix}-{tag}.png"); _shot(sc, cam, d, p); files.append(p)
    pose = [("run", "Run", 6), ("attack", "Attack", 14), ("death", "Death", 36)]
    pre = "KB_" if prefix == "knight" else "SW_"
    for tag, act, fr in pose:
        arm.animation_data.action = bpy.data.actions[pre + act]
        sc.frame_set(fr)
        p = os.path.join(outdir, f"{prefix}-{tag}.png"); _shot(sc, cam, (0.8, -0.6, 0.15), p, z=0.7); files.append(p)
    arm.animation_data.action = None
    for t in arm.animation_data.nla_tracks:
        t.mute = False
    return files
