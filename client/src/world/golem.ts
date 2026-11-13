import {
  Animation,
  AnimationGroup,
  Color3,
  DynamicTexture,
  EasingFunction,
  Mesh,
  MeshBuilder,
  SineEase,
  StandardMaterial,
  Texture,
  TransformNode,
  Vector3,
  VertexBuffer,
  VertexData,
  type AbstractMesh,
  type GlowLayer,
  type Scene,
} from "@babylonjs/core";
import { mulberry32 } from "./map-painter";
import type { AnimRole, CharacterInstance } from "./models";
import { heightToNormal } from "./teleport-stone";

/**
 * Golem de Pedra, todo feito em código: rochas deformadas por ruído (musgo nas faces de
 * cima, por cor de vértice), presas a juntas invisíveis por magia — olhos, núcleo no
 * peito e juntas brilham em âmbar. As animações são AnimationGroups por junta, então o
 * AnimController e o combate tratam o golem como qualquer outro modelo.
 */

export const GOLEM_MODEL = "stone_golem";

const FPS = 30;
const HIPS_Y = 1.1;
const GLOW = new Color3(1, 0.55, 0.16);

type V3 = [number, number, number];
const JOINTS = ["hips", "torso", "head", "shL", "elL", "shR", "elR", "hipL", "knL", "hipR", "knR"] as const;
type Joint = (typeof JOINTS)[number];

// ---------------------------------------------------------------------------------------
// Materiais (uma vez por cena)
// ---------------------------------------------------------------------------------------

interface Mats {
  rock: StandardMaterial;
  glow: StandardMaterial;
}

const matsByScene = new WeakMap<Scene, Mats>();

/** Rachaduras sinuosas (as mesmas na cor, no relevo e na brasa). */
function crackPaths(seed: number, count: number, S: number): [number, number][][] {
  const rand = mulberry32(seed);
  const out: [number, number][][] = [];
  for (let i = 0; i < count; i++) {
    let x = rand() * S;
    let y = rand() * S;
    let a = rand() * Math.PI * 2;
    const pts: [number, number][] = [[x, y]];
    for (let k = 0; k < 8; k++) {
      a += (rand() - 0.5) * 1.1;
      x += Math.cos(a) * S * 0.035;
      y += Math.sin(a) * S * 0.035;
      pts.push([x, y]);
    }
    out.push(pts);
  }
  return out;
}

function strokePaths(ctx: CanvasRenderingContext2D, paths: [number, number][][]): void {
  for (const pts of paths) {
    ctx.beginPath();
    pts.forEach(([x, y], k) => (k ? ctx.lineTo(x, y) : ctx.moveTo(x, y)));
    ctx.stroke();
  }
}

function paint(scene: Scene, name: string, S: number, draw: (ctx: CanvasRenderingContext2D) => void): DynamicTexture {
  const tex = new DynamicTexture(name, { width: S, height: S }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  draw(ctx);
  tex.update(true);
  tex.wrapU = tex.wrapV = Texture.WRAP_ADDRESSMODE;
  return tex;
}

function golemMats(scene: Scene): Mats {
  let m = matsByScene.get(scene);
  if (m) return m;
  const S = 256;
  const cracks = crackPaths(808, 18, S);
  const hot = cracks.slice(0, 4); // só algumas rachaduras têm brasa
  const blob = (ctx: CanvasRenderingContext2D, x: number, y: number, r: number, c: string) => {
    const g = ctx.createRadialGradient(x, y, 0, x, y, r);
    g.addColorStop(0, c);
    g.addColorStop(1, c.replace(/[\d.]+\)$/, "0)"));
    ctx.fillStyle = g;
    ctx.fillRect(x - r, y - r, r * 2, r * 2);
  };
  const albedo = paint(scene, "golemAlbedo", S, (ctx) => {
    const rand = mulberry32(51);
    ctx.fillStyle = "#a39e95";
    ctx.fillRect(0, 0, S, S);
    for (let i = 0; i < 90; i++) blob(ctx, rand() * S, rand() * S, 8 + rand() * 36, `rgba(${rand() < 0.55 ? "60,58,54" : "175,170,160"},${(0.12 + rand() * 0.2).toFixed(2)})`);
    for (let i = 0; i < 18; i++) blob(ctx, rand() * S, rand() * S, 6 + rand() * 20, `rgba(140,110,80,${(0.08 + rand() * 0.1).toFixed(2)})`);
    for (let i = 0; i < 4000; i++) {
      ctx.fillStyle = rand() < 0.5 ? `rgba(30,28,26,${(0.1 + rand() * 0.2).toFixed(2)})` : `rgba(225,220,210,${(0.08 + rand() * 0.14).toFixed(2)})`;
      ctx.fillRect(rand() * S, rand() * S, 1 + rand(), 1 + rand());
    }
    ctx.strokeStyle = "rgba(205,200,190,0.25)";
    ctx.lineWidth = 1.4;
    ctx.translate(0, 1);
    strokePaths(ctx, cracks);
    ctx.translate(0, -1);
    ctx.strokeStyle = "rgba(26,24,22,0.8)";
    strokePaths(ctx, cracks);
  });
  const bump = paint(scene, "golemBump", S, (ctx) => {
    const rand = mulberry32(52);
    ctx.fillStyle = "rgb(128,128,128)";
    ctx.fillRect(0, 0, S, S);
    for (let i = 0; i < 80; i++) blob(ctx, rand() * S, rand() * S, 6 + rand() * 30, rand() < 0.5 ? "rgba(255,255,255,0.2)" : "rgba(0,0,0,0.2)");
    for (let i = 0; i < 3000; i++) {
      ctx.fillStyle = rand() < 0.5 ? "rgba(0,0,0,0.3)" : "rgba(255,255,255,0.25)";
      ctx.fillRect(rand() * S, rand() * S, 1 + rand() * 2, 1 + rand() * 2);
    }
    ctx.strokeStyle = "rgba(0,0,0,0.9)";
    ctx.lineWidth = 2.5;
    strokePaths(ctx, cracks);
    heightToNormal(ctx, S, S, 3);
  });
  // A cor da brasa fica na textura (a emissiveColor soma na superfície toda).
  const ember = paint(scene, "golemEmber", S, (ctx) => {
    ctx.fillStyle = "#000";
    ctx.fillRect(0, 0, S, S);
    ctx.shadowColor = "rgba(255,120,30,1)";
    ctx.shadowBlur = 5;
    ctx.strokeStyle = "rgba(255,110,25,0.55)";
    ctx.lineWidth = 1.8;
    strokePaths(ctx, hot);
    ctx.shadowBlur = 0;
    ctx.strokeStyle = "rgba(255,200,110,0.85)";
    ctx.lineWidth = 0.7;
    strokePaths(ctx, hot);
  });
  const rock = new StandardMaterial("golemRock", scene);
  rock.diffuseTexture = albedo;
  bump.level = 1;
  rock.bumpTexture = bump;
  rock.emissiveTexture = ember;
  rock.emissiveColor = Color3.Black();
  rock.useEmissiveAsIllumination = true;
  rock.specularColor = new Color3(0.08, 0.08, 0.08);
  rock.specularPower = 20;

  const glow = new StandardMaterial("golemGlow", scene);
  glow.emissiveColor = GLOW.clone();
  glow.diffuseColor = Color3.Black();
  glow.specularColor = Color3.Black();
  glow.disableLighting = true;
  // Pulso lento do núcleo (todos os golens juntos: o material é compartilhado).
  scene.onBeforeRenderObservable.add(() => {
    const k = 0.85 + 0.25 * Math.sin(performance.now() / 700);
    glow.emissiveColor.set(GLOW.r * k, GLOW.g * k, GLOW.b * k);
  });
  m = { rock, glow };
  matsByScene.set(scene, m);
  return m;
}

// ---------------------------------------------------------------------------------------
// Geometria do molde (uma vez por cena; cada golem clona e compartilha a geometria)
// ---------------------------------------------------------------------------------------

/** Pedra: icosfera deformada por ruído, achatada nas faces; musgo onde a face olha para cima. */
function rock(scene: Scene, seed: number, pos: V3, size: V3, moss = 0.6, rot: V3 = [0, 0, 0]): Mesh {
  const rand = mulberry32(seed);
  const m = MeshBuilder.CreateIcoSphere(`golemRock-${seed}`, { radius: 1, subdivisions: 2, flat: false, updatable: true }, scene);
  const p = m.getVerticesData(VertexBuffer.PositionKind)!;
  const a = Array.from({ length: 6 }, () => rand() * 6.28);
  // Planos de corte: dão faces de pedra lascada sem quinas duras.
  const cuts = Array.from({ length: 4 }, () => {
    const v = new Vector3(rand() - 0.5, rand() - 0.5, rand() - 0.5).normalize();
    return { n: v, d: 0.72 + rand() * 0.15 };
  });
  for (let k = 0; k < p.length; k += 3) {
    const v = new Vector3(p[k], p[k + 1], p[k + 2]);
    let r = 1 + 0.16 * Math.sin(v.x * 2.4 + a[0]) * Math.sin(v.y * 2.9 + a[1]) * Math.sin(v.z * 2.2 + a[2]) + 0.06 * Math.sin(v.x * 6 + v.z * 5 + a[3]);
    for (const c of cuts) {
      const along = Vector3.Dot(v, c.n) * r;
      if (along > c.d) r -= (along - c.d) * 0.85;
    }
    p[k] = v.x * r;
    p[k + 1] = v.y * r;
    p[k + 2] = v.z * r;
  }
  m.updateVerticesData(VertexBuffer.PositionKind, p);
  m.scaling.set(...size);
  m.rotation.set(...rot);
  m.position.set(...pos);
  m.bakeCurrentTransformIntoVertices();
  const pos2 = m.getVerticesData(VertexBuffer.PositionKind)!;
  const idx = m.getIndices()!;
  const nrm: number[] = [];
  VertexData.ComputeNormals(pos2, idx, nrm);
  m.setVerticesData(VertexBuffer.NormalKind, nrm);
  // Cor de vértice: musgo nas faces de cima, pedra mais escura embaixo (sombra de contato).
  const cols: number[] = [];
  for (let k = 0; k < nrm.length; k += 3) {
    const up = nrm[k + 1];
    const n = 0.5 + 0.5 * Math.sin(pos2[k] * 9 + a[4]) * Math.sin(pos2[k + 2] * 8 + a[5]);
    const mossK = Math.max(0, Math.min(1, (up - 0.45) * 2.4 + (n - 0.5) * 0.8)) * moss;
    const shade = 0.78 + 0.22 * Math.max(0, Math.min(1, up * 0.5 + 0.6));
    const r = shade * (1 - mossK) + 0.42 * mossK;
    const g = shade * (1 - mossK) + 0.56 * mossK;
    const b = shade * (1 - mossK) + 0.28 * mossK;
    cols.push(r, g, b, 1);
  }
  m.setVerticesData(VertexBuffer.ColorKind, cols);
  return m;
}

/** Brilho mágico: esfera âmbar (olhos, núcleo, juntas). */
function ember(scene: Scene, name: string, pos: V3, size: V3): Mesh {
  const m = MeshBuilder.CreateSphere(name, { diameter: 1, segments: 10 }, scene);
  m.scaling.set(...size);
  m.position.set(...pos);
  m.bakeCurrentTransformIntoVertices();
  return m;
}

interface Part {
  joint: Joint;
  rocks: Mesh | null;
  glows: Mesh | null;
}

interface Template {
  /** Posição de descanso de cada junta (relativa ao pai). */
  rest: Record<Joint, V3>;
  parent: Record<Joint, Joint | null>;
  parts: Part[];
}

const templates = new WeakMap<Scene, Template>();

function golemTemplate(scene: Scene): Template {
  let t = templates.get(scene);
  if (t) return t;
  const rest: Record<Joint, V3> = {
    hips: [0, HIPS_Y, 0],
    torso: [0, 0.16, 0],
    head: [0, 1.0, 0.12],
    shL: [0.7, 0.78, 0],
    elL: [0, -0.62, 0],
    shR: [-0.7, 0.78, 0],
    elR: [0, -0.62, 0],
    hipL: [0.27, -0.08, 0],
    knL: [0, -0.5, 0],
    hipR: [-0.27, -0.08, 0],
    knR: [0, -0.5, 0],
  };
  const parent: Record<Joint, Joint | null> = {
    hips: null,
    torso: "hips",
    head: "torso",
    shL: "torso",
    elL: "shL",
    shR: "torso",
    elR: "shR",
    hipL: "hips",
    knL: "hipL",
    hipR: "hips",
    knR: "hipR",
  };
  const { rock: rockMat, glow: glowMat } = golemMats(scene);
  const merge = (list: Mesh[], mat: StandardMaterial): Mesh | null => {
    if (!list.length) return null;
    const out = list.length === 1 ? list[0] : Mesh.MergeMeshes(list, true, true)!;
    out.material = mat;
    out.setEnabled(false);
    return out;
  };
  let seed = 9000;
  const R = (pos: V3, size: V3, moss = 0.6, rot: V3 = [0, 0, 0]) => rock(scene, seed++, pos, size, moss, rot);
  const G = (pos: V3, size: V3) => ember(scene, `golemGlow-${seed++}`, pos, size);
  const side = (s: 1 | -1, v: V3): V3 => [v[0] * s, v[1], v[2]];

  const parts: Part[] = [
    { joint: "hips", rocks: merge([R([0, 0, 0], [0.42, 0.26, 0.32], 0.3), R([0, -0.12, 0.12], [0.26, 0.14, 0.18], 0.2)], rockMat), glows: null },
    {
      joint: "torso",
      rocks: merge(
        [
          R([0, 0.58, 0], [0.62, 0.48, 0.44], 0.7, [0.1, 0, 0]),
          R([0, 0.16, 0.04], [0.38, 0.24, 0.3], 0.3),
          R([0.2, 0.86, -0.18], [0.3, 0.22, 0.26], 0.9),
          R([-0.24, 0.82, -0.16], [0.28, 0.2, 0.24], 0.9),
          R([0, 0.5, 0.38], [0.2, 0.12, 0.08], 0.2, [0, 0, 0.6]),
        ],
        rockMat,
      ),
      glows: merge([G([0, 0.36, 0.36], [0.17, 0.17, 0.12]), G([0, 0.86, 0.12], [0.1, 0.06, 0.1])], glowMat),
    },
    {
      joint: "head",
      rocks: merge([R([0, 0.12, 0], [0.26, 0.22, 0.25], 0.8), R([0, 0.2, 0.13], [0.25, 0.07, 0.12], 0.5), R([0, 0.0, 0.14], [0.17, 0.08, 0.1], 0.2)], rockMat),
      glows: merge([G([0.085, 0.13, 0.22], [0.075, 0.035, 0.04]), G([-0.085, 0.13, 0.22], [0.075, 0.035, 0.04])], glowMat),
    },
    ...([1, -1] as const).flatMap((s): Part[] => {
      const L = s === 1 ? "L" : "R";
      return [
        {
          joint: `sh${L}` as Joint,
          rocks: merge([R(side(s, [0.06, 0.06, 0]), [0.32, 0.27, 0.32], 1), R(side(s, [0.01, -0.34, 0]), [0.17, 0.26, 0.17], 0.3)], rockMat),
          glows: null,
        },
        {
          joint: `el${L}` as Joint,
          rocks: merge([R([0, -0.26, 0.02], [0.2, 0.27, 0.2], 0.3), R([0, -0.62, 0.04], [0.28, 0.25, 0.28], 0.5), R(side(s, [0.1, -0.7, 0.1]), [0.12, 0.1, 0.12], 0.3)], rockMat),
          glows: merge([G([0, 0.02, 0], [0.11, 0.11, 0.11])], glowMat),
        },
        {
          joint: `hip${L}` as Joint,
          rocks: merge([R([0, -0.25, 0], [0.2, 0.26, 0.2], 0.3)], rockMat),
          glows: null,
        },
        {
          joint: `kn${L}` as Joint,
          rocks: merge([R([0, -0.22, 0], [0.18, 0.23, 0.18], 0.2), R([0, -0.47, 0.07], [0.25, 0.11, 0.31], 0.4)], rockMat),
          glows: merge([G([0, 0.02, 0], [0.1, 0.1, 0.1])], glowMat),
        },
      ];
    }),
  ];
  t = { rest, parent, parts };
  templates.set(scene, t);
  return t;
}

// ---------------------------------------------------------------------------------------
// Animações
// ---------------------------------------------------------------------------------------

/** Rotação das juntas em cada quadro-chave; "y" desce/sobe o quadril; "core" = brilho. */
type Pose = Partial<Record<Joint, V3>> & { y?: number; core?: number };
interface Clip {
  frames: number;
  keys: [number, Pose][];
}

/** Pose de descanso das animações: braços um pouco abertos e cotovelos dobrados. */
const BASE: Pose = {
  torso: [0.05, 0, 0],
  shL: [0.05, 0, 0.14],
  shR: [0.05, 0, -0.14],
  elL: [-0.18, 0, 0],
  elR: [-0.18, 0, 0],
  hipL: [-0.05, 0, 0],
  hipR: [-0.05, 0, 0],
  knL: [0.1, 0, 0],
  knR: [0.1, 0, 0],
  y: -0.02,
  core: 1,
};
const at = (p: Pose): Pose => ({ ...BASE, ...p });

const CLIPS: Record<AnimRole, Clip> = {
  idle: {
    frames: 72,
    keys: [
      [0, at({})],
      [36, at({ torso: [0.09, 0, 0], shL: [0.1, 0, 0.17], shR: [0.1, 0, -0.17], head: [0.06, 0.12, 0], knL: [0.16, 0, 0], knR: [0.16, 0, 0], y: -0.045, core: 1.2 })],
      [72, at({})],
    ],
  },
  run: {
    frames: 20,
    keys: [
      [0, at({ hipL: [-0.5, 0, 0], knL: [0.15, 0, 0], hipR: [0.35, 0, 0], knR: [0.45, 0, 0], shL: [0.4, 0, 0.16], shR: [-0.4, 0, -0.16], torso: [0.14, 0.08, 0.05], y: -0.07 })],
      [5, at({ hipL: [-0.1, 0, 0], knL: [0.1, 0, 0], hipR: [-0.2, 0, 0], knR: [0.95, 0, 0], shL: [0.1, 0, 0.16], shR: [-0.1, 0, -0.16], torso: [0.12, 0, 0], y: 0.03 })],
      [10, at({ hipR: [-0.5, 0, 0], knR: [0.15, 0, 0], hipL: [0.35, 0, 0], knL: [0.45, 0, 0], shR: [0.4, 0, -0.16], shL: [-0.4, 0, 0.16], torso: [0.14, -0.08, -0.05], y: -0.07 })],
      [15, at({ hipR: [-0.1, 0, 0], knR: [0.1, 0, 0], hipL: [-0.2, 0, 0], knL: [0.95, 0, 0], shR: [0.1, 0, -0.16], shL: [-0.1, 0, 0.16], torso: [0.12, 0, 0], y: 0.03 })],
      [20, at({ hipL: [-0.5, 0, 0], knL: [0.15, 0, 0], hipR: [0.35, 0, 0], knR: [0.45, 0, 0], shL: [0.4, 0, 0.16], shR: [-0.4, 0, -0.16], torso: [0.14, 0.08, 0.05], y: -0.07 })],
    ],
  },
  // Soco de cima para baixo com o braço direito.
  attack: {
    frames: 36,
    keys: [
      [0, at({})],
      [12, at({ shR: [-2.7, 0, -0.3], elR: [-0.35, 0, 0], torso: [-0.18, 0.3, 0], shL: [0.3, 0, 0.22], head: [-0.1, 0, 0], y: 0.02 })],
      [18, at({ shR: [-0.45, 0, -0.1], elR: [-0.1, 0, 0], torso: [0.42, -0.18, 0], head: [0.2, 0, 0], hipL: [-0.35, 0, 0], knL: [0.55, 0, 0], hipR: [0.15, 0, 0], knR: [0.3, 0, 0], y: -0.13 })],
      [24, at({ shR: [-0.45, 0, -0.1], elR: [-0.1, 0, 0], torso: [0.42, -0.18, 0], head: [0.2, 0, 0], hipL: [-0.35, 0, 0], knL: [0.55, 0, 0], hipR: [0.15, 0, 0], knR: [0.3, 0, 0], y: -0.13 })],
      [36, at({})],
    ],
  },
  // As duas mãos sobem juntas e esmagam o chão.
  skill: {
    frames: 44,
    keys: [
      [0, at({})],
      [15, at({ shL: [-2.8, 0, 0.3], shR: [-2.8, 0, -0.3], elL: [-0.3, 0, 0], elR: [-0.3, 0, 0], torso: [-0.28, 0, 0], head: [-0.2, 0, 0], y: 0.05, core: 1.6 })],
      [22, at({ shL: [-0.35, 0, 0.1], shR: [-0.35, 0, -0.1], elL: [-0.05, 0, 0], elR: [-0.05, 0, 0], torso: [0.6, 0, 0], head: [0.25, 0, 0], hipL: [-0.5, 0, 0], hipR: [-0.5, 0, 0], knL: [0.75, 0, 0], knR: [0.75, 0, 0], y: -0.22, core: 1.8 })],
      [30, at({ shL: [-0.35, 0, 0.1], shR: [-0.35, 0, -0.1], elL: [-0.05, 0, 0], elR: [-0.05, 0, 0], torso: [0.6, 0, 0], head: [0.25, 0, 0], hipL: [-0.5, 0, 0], hipR: [-0.5, 0, 0], knL: [0.75, 0, 0], knR: [0.75, 0, 0], y: -0.22, core: 1.3 })],
      [44, at({})],
    ],
  },
  // Rugido: peito aberto, braços para os lados.
  buff: {
    frames: 38,
    keys: [
      [0, at({})],
      [14, at({ torso: [-0.3, 0, 0], head: [-0.45, 0, 0], shL: [-0.35, 0, 1.15], shR: [-0.35, 0, -1.15], elL: [-0.65, 0, 0], elR: [-0.65, 0, 0], y: 0.03, core: 1.9 })],
      [26, at({ torso: [-0.3, 0, 0], head: [-0.45, 0, 0], shL: [-0.35, 0, 1.15], shR: [-0.35, 0, -1.15], elL: [-0.65, 0, 0], elR: [-0.65, 0, 0], y: 0.03, core: 1.6 })],
      [38, at({})],
    ],
  },
  hit: {
    frames: 14,
    keys: [
      [0, at({})],
      [4, at({ torso: [-0.22, 0.12, 0.05], head: [-0.22, -0.1, 0], shL: [0.3, 0, 0.32], shR: [0.3, 0, -0.32], y: 0.02, core: 1.4 })],
      [14, at({})],
    ],
  },
  // Cai de joelhos, tomba para a frente e o brilho se apaga.
  death: {
    frames: 54,
    keys: [
      [0, at({})],
      [10, at({ torso: [-0.3, 0, 0], head: [-0.35, 0, 0], shL: [0.2, 0, 0.4], shR: [0.2, 0, -0.4], knL: [0.3, 0, 0], knR: [0.3, 0, 0], core: 1.6 })],
      [26, at({ hipL: [-1.1, 0, 0], hipR: [-1.0, 0, 0], knL: [1.9, 0, 0], knR: [1.8, 0, 0], torso: [0.35, 0.1, 0], head: [0.3, 0, 0], shL: [-0.2, 0, 0.5], shR: [-0.3, 0, -0.5], y: -0.5, core: 0.9 })],
      [40, at({ hipL: [-1.3, 0, 0], hipR: [-1.2, 0, 0], knL: [2.2, 0, 0], knR: [2.1, 0, 0], torso: [1.35, 0.15, 0.12], head: [0.55, 0.45, 0], shL: [-0.4, 0, 1.0], shR: [-1.3, 0, -0.5], elR: [-0.6, 0, 0], y: -0.74, core: 0.4 })],
      [54, at({ hipL: [-1.3, 0, 0], hipR: [-1.2, 0, 0], knL: [2.2, 0, 0], knR: [2.1, 0, 0], torso: [1.42, 0.15, 0.12], head: [0.6, 0.5, 0], shL: [-0.4, 0, 1.05], shR: [-1.35, 0, -0.5], elR: [-0.6, 0, 0], y: -0.78, core: 0.15 })],
    ],
  },
};

function buildGroup(role: AnimRole, id: string, nodes: Record<Joint, TransformNode>, glows: Mesh[], scene: Scene): AnimationGroup {
  const clip = CLIPS[role];
  const group = new AnimationGroup(`golem-${role}#${id}`, scene);
  const ease = new SineEase();
  ease.setEasingMode(EasingFunction.EASINGMODE_EASEINOUT);
  // Toda animação mexe em todas as juntas (sem canal faltando, o peso mistura certo).
  for (const j of JOINTS) {
    if (j === "hips") continue;
    let last: V3 = [0, 0, 0];
    const keys = clip.keys.map(([frame, pose]) => {
      last = pose[j] ?? last;
      return { frame, value: new Vector3(...last) };
    });
    const a = new Animation(`${role}-${j}`, "rotation", FPS, Animation.ANIMATIONTYPE_VECTOR3, Animation.ANIMATIONLOOPMODE_CYCLE);
    a.setKeys(keys);
    a.setEasingFunction(ease);
    group.addTargetedAnimation(a, nodes[j]);
  }
  const hipsRot = new Animation(`${role}-hipsRot`, "rotation", FPS, Animation.ANIMATIONTYPE_VECTOR3, Animation.ANIMATIONLOOPMODE_CYCLE);
  let lastHips: V3 = [0, 0, 0];
  hipsRot.setKeys(clip.keys.map(([frame, pose]) => ({ frame, value: new Vector3(...(lastHips = pose.hips ?? lastHips)) })));
  hipsRot.setEasingFunction(ease);
  group.addTargetedAnimation(hipsRot, nodes.hips);
  const hipsY = new Animation(`${role}-hipsY`, "position.y", FPS, Animation.ANIMATIONTYPE_FLOAT, Animation.ANIMATIONLOOPMODE_CYCLE);
  let lastY = 0;
  hipsY.setKeys(clip.keys.map(([frame, pose]) => ({ frame, value: HIPS_Y + (lastY = pose.y ?? lastY) })));
  hipsY.setEasingFunction(ease);
  group.addTargetedAnimation(hipsY, nodes.hips);
  let lastCore = 1;
  const coreKeys = clip.keys.map(([frame, pose]) => ({ frame, value: (lastCore = pose.core ?? lastCore) }));
  for (const g of glows) {
    const a = new Animation(`${role}-core`, "scaling", FPS, Animation.ANIMATIONTYPE_VECTOR3, Animation.ANIMATIONLOOPMODE_CYCLE);
    a.setKeys(coreKeys.map((k) => ({ frame: k.frame, value: new Vector3(k.value, k.value, k.value) })));
    a.setEasingFunction(ease);
    group.addTargetedAnimation(a, g);
  }
  group.normalize(0, clip.frames);
  return group;
}

// ---------------------------------------------------------------------------------------
// Instância
// ---------------------------------------------------------------------------------------

/** Cria um golem animável (mesmo contrato dos modelos glTF). */
export function createGolem(scene: Scene, id: string): CharacterInstance {
  const t = golemTemplate(scene);
  const root = new TransformNode(`model#${id}`, scene);
  const nodes = {} as Record<Joint, TransformNode>;
  for (const j of JOINTS) {
    const n = new TransformNode(`golem-${j}#${id}`, scene);
    n.position.set(...t.rest[j]);
    nodes[j] = n;
  }
  for (const j of JOINTS) nodes[j].parent = t.parent[j] ? nodes[t.parent[j]!] : root;

  const rocks: Mesh[] = [];
  const glows: Mesh[] = [];
  for (const p of t.parts) {
    for (const [src, list] of [
      [p.rocks, rocks],
      [p.glows, glows],
    ] as const) {
      if (!src) continue;
      const c = src.clone(`${src.name}#${id}`, nodes[p.joint])!;
      c.setEnabled(true);
      c.isPickable = false;
      list.push(c);
    }
  }
  for (const r of rocks) r.receiveShadows = true;
  // O brilho mágico entra na camada de brilho do mundo (se houver).
  const glowLayer = (scene as unknown as { getGlowLayerByName?: (n: string) => GlowLayer | null }).getGlowLayerByName?.("glow") ?? null;
  for (const g of glows) glowLayer?.addIncludedOnlyMesh(g);

  const anims = new Map<AnimRole, AnimationGroup>();
  for (const role of Object.keys(CLIPS) as AnimRole[]) anims.set(role, buildGroup(role, id, nodes, glows, scene));

  const resetPose = () => {
    for (const j of JOINTS) {
      nodes[j].position.set(...t.rest[j]);
      const r = BASE[j] ?? [0, 0, 0];
      nodes[j].rotation.set(...r);
    }
    nodes.hips.position.y = HIPS_Y + (BASE.y ?? 0);
    for (const g of glows) g.scaling.setAll(1);
  };
  resetPose();

  const all: AbstractMesh[] = [...rocks, ...glows];
  return {
    root,
    anims,
    resetPose,
    setWeapons: () => {},
    setLook: () => {},
    meshes: () => all,
    dispose: () => {
      for (const g of anims.values()) g.dispose();
      for (const g of glows) glowLayer?.removeIncludedOnlyMesh(g);
      root.dispose(false, false);
    },
  };
}
