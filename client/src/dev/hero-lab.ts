/**
 * Laboratório de heróis (ferramenta de arte, não faz parte do jogo): abre `lab.html` no `vite dev`
 * e mostra personagens com armas e poses congeladas, de frente, de lado ou de costas, usando o mesmo
 * `CharacterModels` do jogo. Os prints dos heróis (docs/arte/remaster-herois/) saem daqui.
 *
 * window.lab.show({ cam: "front"|"side"|"back"|"three", items: [{ model, right, left, role, t, look }] })
 *   role = papel de animação (idle, walk, run, attack, hit, death, buff...), t = 0..1 do clipe,
 *   grade = grau do item (1-5): escolhe a forma da arma (comum, rara, alta) e o brilho, como no jogo.
 */
import { ArcRotateCamera, Axis, Quaternion, Color4, Engine, HemisphericLight, DirectionalLight, Scene, TransformNode, Vector3, type AnimationGroup } from "@babylonjs/core";
import { configureKtx2 } from "../world/assets";
import { AnimController } from "../world/anim";
import { CharacterModels, WEAPON_GRIPS, type AnimRole, type CharacterInstance, type Grip } from "../world/models";
import { setBodyPalettes, type BodyLook } from "../world/body-look";
import { toArmorGrades, type ArmorGrades } from "../world/armor";

interface Item { model: string; right?: string | null; left?: string | null; role?: AnimRole; t?: number; look?: BodyLook; grade?: number; armor?: Partial<ArmorGrades> }
interface Spec { grips?: Record<string, Grip>; radius?: number; cam?: "front" | "side" | "back" | "three"; items: Item[]; gap?: number; height?: number; settle?: number }

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
    await Promise.all(spec.items.map((i) => models!.ensure(i.model))); // NPCs das outras regiões carregam sob demanda
    Object.assign(WEAPON_GRIPS, spec.grips ?? {});
    for (const i of live) i.dispose();
    live = [];
    const gap = spec.gap ?? 1.4;
    const n = spec.items.length;
    spec.items.forEach((it, k) => {
      const inst = models!.instantiate(it.model, `lab${k}`, { "handslot.r": it.right ?? null, "handslot.l": it.left ?? null }, it.look);
      if (!inst) return;
      if (it.armor) inst.setArmor(toArmorGrades(it.armor)); // raridade das peças (tunnel#96)
      if (it.grade) inst.setWeapons(it.right ?? null, it.left ?? null, it.grade, 0);
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
    await new Promise((r) => setTimeout(r, spec.settle ?? 1500)); // texturas e peças de armadura carregam sob demanda
    return live.map((i) => (i.root as TransformNode).name).join(",");
  },
  /** Toca o parado de verdade (AnimController com as pausas) e diz quando cada pausa começou, por até `seconds` s. */
  async idleBreaks(model: string, seconds: number): Promise<{ t: number; role: string }[]> {
    await ready;
    for (const i of live) i.dispose();
    live = [];
    const inst = models!.instantiate(model, "labb");
    if (!inst) return [];
    live.push(inst);
    const ctl = new AnimController(inst.anims, inst.resetPose, inst.impacts);
    const out: { t: number; role: string }[] = [];
    const seen = new Set<string>();
    const t0 = performance.now();
    let last = t0;
    while (performance.now() - t0 < seconds * 1000) {
      await new Promise((r) => setTimeout(r, 100));
      const now = performance.now();
      ctl.update((now - last) / 1000, false, 1);
      last = now;
      for (const r of ["break1", "break2"] as const) {
        const playing = !!inst.anims.get(r)?.isPlaying;
        if (playing && !seen.has(r)) out.push({ t: Math.round((now - t0) / 100) / 10, role: r });
        if (playing) seen.add(r); else seen.delete(r);
      }
    }
    return out;
  },
  /**
   * Velocidade "natural" dos clipes de andar e correr de um modelo, em m/s do mundo (escala do modelo incluída): a
   * distância que o pé de apoio (o mais baixo) percorre para trás, em relação ao quadril, por segundo. É a velocidade
   * a que o personagem deve se mover para o pé não deslizar no chão. Também diz quanto o quadril anda no clipe
   * (root motion), o que deveria ser 0.
   */
  async footSpeeds(model: string): Promise<{ role: string; clip: string; seconds: number; speed: number; hipsTravel: number }[]> {
    await ready;
    for (const i of live) i.dispose();
    live = [];
    const inst = models!.instantiate(model, "labf");
    if (!inst) return [];
    live.push(inst);
    scene.render();
    const nodes = inst.root.getDescendants(false).filter((n): n is TransformNode => n instanceof TransformNode);
    const base = (n: TransformNode) => n.name.split("#")[0];
    const feet = nodes.filter((n) => /foot/i.test(base(n)) && !/toe|handslot/i.test(base(n)));
    const hips = nodes.find((n) => /hips$/i.test(base(n)));
    const out: { role: string; clip: string; seconds: number; speed: number; hipsTravel: number }[] = [];
    if (!hips || feet.length < 2) return out;
    for (const role of ["walk", "run"] as const) {
      const g = inst.anims.get(role);
      if (!g) continue;
      g.start(false, 1, g.from, g.to);
      g.pause();
      const fps = g.targetedAnimations[0]?.animation.framePerSecond || 30;
      const frames = 48;
      const rel: { x: number; z: number; y: number }[][] = [];
      const hp: { x: number; z: number }[] = [];
      for (let k = 0; k <= frames; k++) {
        g.goToFrame(g.from + ((g.to - g.from) * k) / frames);
        for (const n of nodes) n.computeWorldMatrix(true);
        const h = hips.getAbsolutePosition();
        hp.push({ x: h.x, z: h.z });
        rel.push(feet.map((f) => { const p = f.getAbsolutePosition(); return { x: p.x - h.x, z: p.z - h.z, y: p.y }; }));
      }
      const dt = (g.to - g.from) / fps / frames;
      let sum = 0;
      let n = 0;
      let minY = Infinity;
      for (const fr of rel) for (const f of fr) minY = Math.min(minY, f.y);
      for (let k = 0; k < frames; k++) {
        // pé de apoio: o mais baixo (e perto do chão) nos dois quadros
        const i0 = rel[k][0].y <= rel[k][1].y ? 0 : 1;
        const a = rel[k][i0];
        const b = rel[k + 1][i0];
        if (a.y > minY + 0.05 || b.y > minY + 0.05) continue;
        sum += Math.hypot(b.x - a.x, b.z - a.z) / dt;
        n++;
      }
      out.push({
        role,
        clip: g.name.split("#")[0],
        seconds: Math.round(((g.to - g.from) / fps) * 100) / 100,
        speed: n ? Math.round((sum / n) * 100) / 100 : 0,
        hipsTravel: Math.round(Math.hypot(hp[frames].x - hp[0].x, hp[frames].z - hp[0].z) * 100) / 100,
      });
    }
    return out;
  },
  /** Nomes dos clipes de andar/correr que existem (os moldes de todos os arquivos). */
  clipNames: () => [...((models as unknown as { animTemplates: Map<string, AnimationGroup> }).animTemplates.keys())],
  /** Velocidade natural (m/s) de um clipe qualquer aplicado a um modelo (ver footSpeeds). */
  async clipSpeed(model: string, clip: string): Promise<{ seconds: number; speed: number } | null> {
    await ready;
    for (const i of live) i.dispose();
    live = [];
    const inst = models!.instantiate(model, "labc");
    const tpl = (models as unknown as { animTemplates: Map<string, AnimationGroup> }).animTemplates.get(clip);
    if (!inst || !tpl) return null;
    live.push(inst);
    scene.render();
    const nodes = inst.root.getDescendants(false).filter((n): n is TransformNode => n instanceof TransformNode);
    const base = (n: TransformNode) => n.name.split("#")[0];
    const byName = new Map(nodes.map((n) => [base(n), n]));
    const g = tpl.clone(`${clip}#c`, (t) => byName.get(t.name.split("#")[0]) ?? null);
    const feet = nodes.filter((n) => /foot/i.test(base(n)) && !/toe|handslot/i.test(base(n)));
    const hips = nodes.find((n) => /hips$/i.test(base(n)));
    if (!hips || feet.length < 2) return null;
    g.start(false, 1, g.from, g.to);
    g.pause();
    const fps = g.targetedAnimations[0]?.animation.framePerSecond || 60;
    const frames = 48;
    const rel: { x: number; z: number; y: number }[][] = [];
    for (let k = 0; k <= frames; k++) {
      g.goToFrame(g.from + ((g.to - g.from) * k) / frames);
      for (const n of nodes) n.computeWorldMatrix(true);
      const h = hips.getAbsolutePosition();
      rel.push(feet.map((f) => { const p = f.getAbsolutePosition(); return { x: p.x - h.x, z: p.z - h.z, y: p.y }; }));
    }
    const dt = (g.to - g.from) / fps / frames;
    let minY = Infinity;
    for (const fr of rel) for (const f of fr) minY = Math.min(minY, f.y);
    let sum = 0;
    let n = 0;
    for (let k = 0; k < frames; k++) {
      const i0 = rel[k][0].y <= rel[k][1].y ? 0 : 1;
      const a = rel[k][i0];
      const b = rel[k + 1][i0];
      if (a.y > minY + 0.05 || b.y > minY + 0.05) continue;
      sum += Math.hypot(b.x - a.x, b.z - a.z) / dt;
      n++;
    }
    g.dispose();
    return { seconds: Math.round(((g.to - g.from) / fps) * 100) / 100, speed: n ? Math.round((sum / n) * 100) / 100 : 0 };
  },
  /** Gira as peças presas nos ossos (depuração: orientação de peças novas). */
  turnPieces(item: number, deg: number, parent = "Head"): void {
    for (const n of scene.transformNodes) {
      if (n.name === `node0#lab${item}` && n.parent?.name.startsWith(`mixamorig:${parent}`) && n.rotationQuaternion) n.rotationQuaternion = n.rotationQuaternion.multiply(Quaternion.RotationAxis(Axis.Y, (deg * Math.PI) / 180));
    }
  },
  models: () => models,
  /** Nós da cena cujo nome casa (depuração das peças anexadas). */
  find: (re: string) => scene.transformNodes.concat(scene.meshes as never[]).filter((n) => new RegExp(re).test(n.name)).map((n) => ({ name: n.name, parent: n.parent?.name, on: n.isEnabled(), pos: (n as TransformNode).position?.asArray(), sc: (n as TransformNode).scaling?.asArray(), rq: (n as TransformNode).rotationQuaternion?.asArray() })),
  cam: () => ({ a: cam.alpha, b: cam.beta, r: cam.radius, p: cam.position.asArray() }),
};
