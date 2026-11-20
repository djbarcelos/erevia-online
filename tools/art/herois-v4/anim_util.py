"""Heróis v4: peças comuns das animações base (punho, giro da palma, velocidade do pé)."""
import math

from mathutils import Euler, Matrix, Vector

FWD = Vector((0, -1, 0))  # frente do personagem
FIST = {"index": (55, 70, 45), "fingers": (75, 85), "thumb": (20, 25, 20)}


def palm_twist(arm, s):
    """Giro (graus, no eixo do antebraço) que deixa a linha dos nós (do mínimo ao indicador) para a frente,
    com a palma virada para o corpo, medido no repouso. O corpo do Meshy veio com a mão direita girada
    ~75° (palma para trás): sem isto a arma sai de lado."""
    B = arm.data.bones
    a = (B[f"hand_{s}"].head_local - B[f"forearm_{s}"].head_local).normalized()
    k = B[f"index_01_{s}"].head_local - B[f"fingers_01_{s}"].head_local
    k = (k - a * k.dot(a)).normalized()
    t = (FWD - a * FWD.dot(a)).normalized()
    return math.degrees(math.atan2(k.cross(t).dot(a), k.dot(t)))


def fist_basis():
    """Rotação local (matriz de base) de cada osso de dedo no punho fechado."""
    out = {}
    for s in ("l", "r"):
        for f, angs in FIST.items():
            for k, ang in enumerate(angs, 1):
                out[f"{f}_{k:02d}_{s}"] = Euler((math.radians(ang), 0, 0)).to_matrix().to_4x4()
    return out


def foot_speed(arm, scene, act, frames, fps):
    """Velocidade do pé de apoio (o mais baixo) em relação ao quadril, em m/s, medida no clipe pronto."""
    arm.animation_data.action = act
    pb = arm.pose.bones
    sp, prev = [], None
    for f in range(frames):
        scene.frame_set(f)
        fl = arm.matrix_world @ pb["foot_l"].head
        fr = arm.matrix_world @ pb["foot_r"].head
        low = fl if fl.z < fr.z else fr
        if prev is not None and abs(low.z - prev.z) < 0.01:
            sp.append((low - prev).length * fps)
        prev = low
    sp.sort()
    return round(sp[len(sp) // 2], 2) if sp else None


def rot_y(deg):
    return Matrix.Rotation(math.radians(deg), 4, "Y")
