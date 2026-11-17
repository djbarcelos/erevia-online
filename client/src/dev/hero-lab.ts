/**
 * Laboratório de heróis (ferramenta de arte, não faz parte do jogo): abre `lab.html?remaster=1` no `vite dev`
 * e mostra personagens com armas e poses congeladas, de frente, de lado ou de costas, usando o mesmo
 * `CharacterModels` do jogo. Os prints dos heróis (docs/arte/remaster-herois/) saem daqui.
 *
 * window.lab.show({ cam: "front"|"side"|"back"|"three", items: [{ model, right, left, role, t, look }] })
 *   role = papel de animação (idle, walk, run, attack, hit, death, buff...), t = 0..1 do clipe.
 */
import { ArcRotateCamera, Color4, Engine, HemisphericLight, DirectionalLight, Scene, Vector3, type AnimationGroup, type TransformNode } from "@babylonjs/core";
import { configureKtx2 } from "../world/assets";
import { CharacterModels, WEAPON_GRIPS, type AnimRole, type CharacterInstance, type Grip } from "../world/models";
import { setBodyPalettes, type BodyLook } from "../world/body-look";

interface Item { model: string; right?: string | null; left?: string | null; role?: AnimRole; t?: number; look?: BodyLook }
interface Spec { grips?: Record<string, Grip>; radius?: number; cam?: "front" | "side" | "back" | "three"; items: Item[]; gap?: number; height?: number }

configureKtx2();
// Paletas de mentira (o laboratório não tem servidor): 1-3 em cada eixo.
const sh = (name: string, top: string) => ({ name, top, bottom: top });
setBodyPalettes({
  skins: [sh("Escura", "#7a4a2a"), sh("Pálida", "#f0d2b8"), sh("Verde", "#7da35a")],
  hairs: [sh("Loiro", "#e8c85a"), sh("Preto", "#1c1814"), sh("Branco", "#e8e8ee")],
  outfits: [sh("Azul", "#2a58c8"), sh("Roxo", "#7a3ac0"), sh("Preto", "#2a2a30")],
  heads: [], hatClasses: ["knight", "barbarian", "mage"], barberPrice: 0,
});
const canvas = document.getElementById("c") as HTMLCanvasElement;
const engine = new Engine(canvas, true, { preserveDrawingBuffer: true });
const scene = new Scene(engine);
scene.clearColor = new Color4(0.55, 0.63, 0.72, 1);
const hemi = new HemisphericLight("h", new Vector3(0, 1, 0.3), scene);
hemi.intensity = 0.9;
const sun = new DirectionalLight("s", new Vector3(-0.4, -1, 0.6), scene);
sun.intensity = 0.8;
const cam = new ArcRotateCamera("cam", 0, 1.3, 5, new Vector3(0, 1, 0), scene);
cam.minZ = 0.05;
let models: CharacterModels | null = null;
let live: CharacterInstance[] = [];

engine.runRenderLoop(() => scene.render());
const ready = CharacterModels.load(scene).then((m) => { models = m; });

(window as unknown as { lab: unknown }).lab = {
  async show(spec: Spec): Promise<string> {
    await ready;
    Object.assign(WEAPON_GRIPS, spec.grips ?? {});
    for (const i of live) i.dispose();
    live = [];
    const gap = spec.gap ?? 1.4;
    const n = spec.items.length;
    spec.items.forEach((it, k) => {
      const inst = models!.instantiate(it.model, `lab${k}`, { "handslot.r": it.right ?? null, "handslot.l": it.left ?? null }, it.look);
      if (!inst) return;
      inst.root.position.x = (k - (n - 1) / 2) * gap;
      const g: AnimationGroup | undefined = inst.anims.get(it.role ?? "idle");
      if (g) {
        g.start(false, 1, g.from, g.to);
        g.goToFrame(g.from + (g.to - g.from) * (it.t ?? 0));
        g.pause();
      }
      live.push(inst);
    });
    const alpha = { front: Math.PI / 2, side: 0, back: -Math.PI / 2, three: Math.PI / 4 }[spec.cam ?? "front"];
    const h = spec.height ?? 1;
    cam.target = new Vector3(0, h, 0);
    cam.alpha = alpha; cam.beta = 1.45;
    cam.radius = spec.radius ?? Math.max(3.2, n * gap * 0.8 + 1.5);
    for (let f = 0; f < 6; f++) await new Promise((r) => requestAnimationFrame(r));
    await new Promise((r) => setTimeout(r, 1500));
    return live.map((i) => (i.root as TransformNode).name).join(",");
  },
  models: () => models,
  cam: () => ({ a: cam.alpha, b: cam.beta, r: cam.radius, p: cam.position.asArray() }),
};
