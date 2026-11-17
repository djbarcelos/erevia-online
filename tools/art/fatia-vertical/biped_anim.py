"""Animações do bípede (cavaleiro e esqueleto): parado, andar, correr, golpe, dano, morte, braço erguido.

Rode depois de ter `arm` (a armadura). Eixos do mundo: rx positivo = o alto vai para a frente (-Y) e
os pés vão para trás. `pref` prefixa o nome dos clipes (KB_ cavaleiro, SW_ esqueleto).
Os ossos que a armadura não tem (capa) são ignorados.
"""
import math

IMPACT_FRAME = 14  # golpe: 14/30 s = 0,47 s


def build_clips(arm, pref, style=1.0):
    S = math.sin
    C = math.cos
    an = Animator(arm)
    for pb in list(arm.animation_data.nla_tracks):
        arm.animation_data.nla_tracks.remove(pb)

    def sampled(name, n, fn, step=2):
        an.begin(pref + name)
        for f in range(0, n + 1, step):
            r, l = fn(f / n)
            an.key(f, r, l)
        an.end()

    # parado: respiração lenta, capa balançando (60 quadros, 2 s)
    def idle(t):
        p = 2 * math.pi * t
        r = {
            "chest": (1.2 * S(p), 0, 0), "spine": (0.6 * S(p), 0, 0), "head": (-0.8 * S(p), 1.5 * S(p), 0),
            "upperarm.l": (0, 0, 0), "upperarm.r": (0, 0, 0),
            "lowerarm.l": (-8 - 1.2 * S(p), 0, 0), "lowerarm.r": (-8 - 1.2 * S(p), 0, 0),
            "cape1": (3 + 1.5 * S(p), 0, 0), "cape2": (3 + 2.5 * S(p + 0.8), 0, 0),
        }
        return r, {"hips": (0, 0, 0.006 * S(p))}
    sampled("Idle", 60, idle, 5)

    def gait(amp, knee, armamp, lean, bob, twist, elbow):
        def fn(t):
            p = 2 * math.pi * t
            ul, ur = -amp * S(p), amp * S(p)
            ll, lr = knee * max(0, C(p)), knee * max(0, -C(p))
            r = {
                "upperleg.l": (ul, 0, 0), "upperleg.r": (ur, 0, 0),
                "lowerleg.l": (ll, 0, 0), "lowerleg.r": (lr, 0, 0),
                "foot.l": (-0.65 * (ul + ll), 0, 0), "foot.r": (-0.65 * (ur + lr), 0, 0),
                "upperarm.l": (armamp * S(p), 0, 0), "upperarm.r": (-armamp * S(p), 0, 0),
                "lowerarm.l": (-elbow - 0.25 * elbow * (1 + S(p)), 0, 0), "lowerarm.r": (-elbow - 0.25 * elbow * (1 - S(p)), 0, 0),
                "hips": (lean * 0.5, 0, twist * S(p)), "spine": (lean * 0.3, 0, -twist * 0.9 * S(p)),
                "chest": (lean * 0.2, 0, -twist * 0.6 * S(p)), "head": (-lean * 0.8, 0, twist * 0.5 * S(p)),
                "cape1": (8 + lean + 5 * C(2 * p), 0, 0), "cape2": (6 + 1.6 * lean + 8 * S(2 * p + 1), 0, 0),
            }
            return r, {"hips": (0, 0, bob * C(2 * p))}
        return fn
    sampled("Walk", 30, gait(24, 40, 20, 3, 0.018, 4, 14), 3)
    sampled("Run", 20, gait(46, 85, 48, 14, 0.04, 8, 70), 2)

    # golpe (27 quadros), impacto no quadro 14
    def atk():
        an.begin(pref + "Attack")
        an.key(0, {"cape1": (4, 0, 0)})
        an.key(6, {  # antecipação: braço de armas para trás e para cima, tronco gira para trás
            "upperarm.r": (-160, 0, 12), "lowerarm.r": (-70, 0, 0), "chest": (0, 0, -26), "spine": (-8, 0, -8),
            "hips": (0, 0, -6), "upperleg.l": (-12, 0, 0), "upperleg.r": (10, 0, 0), "head": (-6, 0, 0),
            "upperarm.l": (-30, 0, -12), "lowerarm.l": (-60, 0, 0), "cape1": (14, 0, 0), "cape2": (18, 0, 0),
        }, {"hips": (0, 0.03, 0)})
        an.key(10, {  # varre rápido
            "upperarm.r": (-95, 0, 6), "lowerarm.r": (-35, 0, 0), "chest": (6, 0, 6), "spine": (6, 0, 6),
            "hips": (0, 0, 6), "upperleg.l": (-30, 0, 0), "upperleg.r": (16, 0, 0), "lowerleg.l": (14, 0, 0),
            "upperarm.l": (-30, 0, -12), "lowerarm.l": (-60, 0, 0), "cape1": (8, 0, 0),
        }, {"hips": (0, -0.08, -0.03)})
        an.key(IMPACT_FRAME, {  # impacto: tronco inclinado para a frente, braço estendido
            "upperarm.r": (-52, 0, 4), "lowerarm.r": (-14, 0, 0), "chest": (16, 0, 24), "spine": (12, 0, 10),
            "hips": (4, 0, 8), "upperleg.l": (-36, 0, 0), "upperleg.r": (20, 0, 0), "lowerleg.l": (20, 0, 0),
            "lowerleg.r": (10, 0, 0), "head": (6, 0, 0), "upperarm.l": (-24, 0, -14), "lowerarm.l": (-70, 0, 0),
            "cape1": (-2, 0, 0), "cape2": (6, 0, 0),
        }, {"hips": (0, -0.12, -0.05)})
        an.key(20, {
            "upperarm.r": (-38, 0, 4), "lowerarm.r": (-22, 0, 0), "chest": (8, 0, 14), "spine": (6, 0, 6),
            "upperleg.l": (-24, 0, 0), "upperleg.r": (12, 0, 0), "lowerleg.l": (10, 0, 0), "cape1": (12, 0, 0),
        }, {"hips": (0, -0.06, -0.02)})
        an.key(27, {"cape1": (4, 0, 0)})
        an.end()
    atk()

    # dano: tranco para trás (12 quadros)
    an.begin(pref + "Hit")
    an.key(0, {})
    an.key(3, {"spine": (-14, 0, 0), "chest": (-10, 0, 0), "head": (-16, 0, 6), "upperarm.l": (14, 0, 0), "upperarm.r": (14, 0, 0),
               "cape1": (-10, 0, 0)}, {"hips": (0, 0.06, 0)})
    an.key(12, {})
    an.end()

    # morte: cai de costas e fica no chão (36 quadros, não repete)
    an.begin(pref + "Death")
    an.key(0, {})
    an.key(5, {"spine": (-14, 0, 0), "head": (-18, 0, 0), "upperarm.l": (20, 0, -10), "upperarm.r": (20, 0, 10)}, {"hips": (0, 0.05, 0)})
    an.key(18, {"hips": (-52, 0, 0), "spine": (-10, 0, 0), "head": (-14, 0, 0), "upperleg.l": (30, 0, 0), "upperleg.r": (38, 0, 0),
                "lowerleg.l": (60, 0, 0), "lowerleg.r": (74, 0, 0), "upperarm.l": (-20, 0, -26), "upperarm.r": (-30, 0, 26),
                "lowerarm.l": (-30, 0, 0), "lowerarm.r": (-24, 0, 0), "cape1": (30, 0, 0)}, {"hips": (0, 0.22, -0.36)})
    an.key(30, {"hips": (-90, 0, 0), "spine": (-4, 0, 0), "head": (-6, 0, 4), "upperleg.l": (4, 0, -6), "upperleg.r": (8, 0, 8),
                "lowerleg.l": (24, 0, 0), "lowerleg.r": (32, 0, 0), "upperarm.l": (-8, 0, -34), "upperarm.r": (-14, 0, 38),
                "lowerarm.l": (-16, 0, 0), "lowerarm.r": (-10, 0, 0), "cape1": (-20, 0, 0), "cape2": (-20, 0, 0)},
           {"hips": (0, 0.46, -0.78)})
    an.key(36, {"hips": (-90, 0, 0), "spine": (-3, 0, 0), "head": (-5, 0, 3), "upperleg.l": (4, 0, -6), "upperleg.r": (8, 0, 8),
                "lowerleg.l": (22, 0, 0), "lowerleg.r": (30, 0, 0), "upperarm.l": (-6, 0, -36), "upperarm.r": (-12, 0, 40),
                "lowerarm.l": (-14, 0, 0), "lowerarm.r": (-8, 0, 0), "cape1": (-20, 0, 0), "cape2": (-20, 0, 0)},
           {"hips": (0, 0.46, -0.78)})
    an.end()

    # braço erguido (buff, acenar, dançar): 30 quadros em ciclo
    def raise_arm(t):
        p = 2 * math.pi * t
        return {"upperarm.r": (0, 150, 0), "lowerarm.r": (-10, 28 * S(2 * p), 0), "chest": (0, 0, 4 * S(p)),
                "head": (-4, 0, -4 * S(p)), "cape1": (4 + 2 * S(p), 0, 0)}, {}
    sampled("Raise", 30, raise_arm, 3)
    return an
