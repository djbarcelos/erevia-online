import {
  Color3,
  DynamicTexture,
  Mesh,
  MeshBuilder,
  StandardMaterial,
  Texture,
  TransformNode,
  VertexBuffer,
  VertexData,
  type Scene,
} from "@babylonjs/core";
import type { Fx } from "./fx";
import { mulberry32 } from "./map-painter";

/**
 * Pedra de Teleporte: monólito de pedra antiga com runas entalhadas que brilham, um
 * núcleo de luz flutuando no topo, luz azul no chão e grãos de luz subindo. Tudo
 * desenhado em código (geometria deformada por ruído e texturas pintadas em canvas).
 */

/** Altura do topo da rocha. */
const ROCK_H = 2.6;
/** Altura do núcleo de luz. */
const CORE_Y = 3.05;
/** Lado das texturas da rocha (u = volta em torno da pedra, v = altura). */
const TEX = 512;

export interface TeleportStone {
  /** Malhas que projetam sombra (rocha e pedregulhos). */
  shadowMeshes: Mesh[];
}

// ---------------------------------------------------------------------------------------
// Texturas (pintadas uma vez por cena e compartilhadas por todas as pedras)
// ---------------------------------------------------------------------------------------

interface Stroke {
  pts: [number, number][];
}

/** Glifo inventado: haste e ramos tortos, às vezes um ponto (traços em pixels). */
function glyph(cx: number, cy: number, s: number, rand: () => number): Stroke[] {
  const out: Stroke[] = [];
  const jit = () => (rand() - 0.5) * s * 0.18;
  const stem = rand() < 0.75;
  if (stem) out.push({ pts: [[cx + jit(), cy - s], [cx + jit(), cy + s]] });
  const branches = 1 + Math.floor(rand() * 3);
  for (let i = 0; i < branches; i++) {
    const y0 = cy + (rand() - 0.5) * s * 1.4;
    const dir = rand() < 0.5 ? -1 : 1;
    const x1 = cx + dir * s * (0.55 + rand() * 0.35);
    const y1 = y0 + (rand() - 0.5) * s * 1.1;
    if (rand() < 0.4) {
      // Ramo dobrado (gancho).
      out.push({ pts: [[cx, y0], [x1, y1], [x1 + jit(), y1 + (rand() < 0.5 ? -1 : 1) * s * 0.5]] });
    } else {
      out.push({ pts: [[cx, y0], [x1, y1]] });
    }
  }
  if (!stem || rand() < 0.3) {
    const a = rand() * Math.PI * 2;
    const r = s * 0.22;
    const pts: [number, number][] = [];
    for (let k = 0; k <= 8; k++) pts.push([cx + Math.cos(a + (k / 8) * Math.PI * 2) * r + s * 0.4, cy + Math.sin(a + (k / 8) * Math.PI * 2) * r]);
    out.push({ pts });
  }
  return out;
}

/** Rachadura: passeio aleatório que vai afinando. */
function crack(x: number, y: number, len: number, angle: number, rand: () => number): Stroke {
  const pts: [number, number][] = [[x, y]];
  let a = angle;
  for (let i = 0; i < 7; i++) {
    a += (rand() - 0.5) * 0.9;
    x += Math.cos(a) * (len / 7);
    y += Math.sin(a) * (len / 7);
    pts.push([x, y]);
  }
  return { pts };
}

/** Desenha nos dois lados da emenda horizontal (a textura dá a volta na pedra). */
function wrapped(x: number, w: number, draw: (dx: number) => void): void {
  draw(0);
  if (x - w < 0) draw(TEX);
  if (x + w > TEX) draw(-TEX);
}

function strokePath(ctx: CanvasRenderingContext2D, st: Stroke, dx: number): void {
  ctx.beginPath();
  st.pts.forEach(([x, y], i) => (i ? ctx.lineTo(x + dx, y) : ctx.moveTo(x + dx, y)));
  ctx.stroke();
}

function strokeAll(ctx: CanvasRenderingContext2D, list: Stroke[]): void {
  for (const st of list) {
    const xs = st.pts.map((p) => p[0]);
    const x = (Math.min(...xs) + Math.max(...xs)) / 2;
    const w = (Math.max(...xs) - Math.min(...xs)) / 2 + 12;
    wrapped(x, w, (dx) => strokePath(ctx, st, dx));
  }
}

function blob(ctx: CanvasRenderingContext2D, x: number, y: number, r: number, color: string): void {
  wrapped(x, r, (dx) => {
    const g = ctx.createRadialGradient(x + dx, y, 0, x + dx, y, r);
    g.addColorStop(0, color);
    g.addColorStop(1, color.replace(/[\d.]+\)$/, "0)"));
    ctx.fillStyle = g;
    ctx.fillRect(x + dx - r, y - r, r * 2, r * 2);
  });
}

/**
 * Converte o mapa de altura (canal vermelho, cinza) pintado no canvas em mapa de normais,
 * no próprio canvas. Repete na horizontal (textura que dá a volta) e prende na vertical.
 */
export function heightToNormal(ctx: CanvasRenderingContext2D, w: number, hgt: number, strength: number): void {
  const src = ctx.getImageData(0, 0, w, hgt).data;
  const h = (x: number, y: number) => src[((y < 0 ? 0 : y >= hgt ? hgt - 1 : y) * w + ((x + w) % w)) << 2] / 255;
  const out = ctx.createImageData(w, hgt);
  const d = out.data;
  for (let y = 0; y < hgt; y++) {
    for (let x = 0; x < w; x++) {
      const dx = (h(x + 1, y) - h(x - 1, y)) * strength;
      const dy = (h(x, y + 1) - h(x, y - 1)) * strength;
      // Canvas cresce para baixo; na textura v cresce para cima.
      let nx = -dx;
      let ny = dy;
      let nz = 1;
      const l = Math.hypot(nx, ny, nz);
      nx /= l;
      ny /= l;
      nz /= l;
      const i = (y * w + x) << 2;
      d[i] = (nx * 0.5 + 0.5) * 255;
      d[i + 1] = (ny * 0.5 + 0.5) * 255;
      d[i + 2] = (nz * 0.5 + 0.5) * 255;
      d[i + 3] = 255;
    }
  }
  ctx.putImageData(out, 0, 0);
}

interface StoneLayout {
  runes: Stroke[];
  /** Rachaduras por onde a luz das runas vaza. */
  litCracks: Stroke[];
  cracks: Stroke[];
}

function layout(): StoneLayout {
  const rand = mulberry32(7171);
  const runes: Stroke[] = [];
  // Coluna de runas na frente (u = 0,5 olha para +Z, para a praça).
  for (let i = 0; i < 5; i++) {
    const v = 0.72 - i * 0.085;
    runes.push(...glyph(TEX / 2, (1 - v) * TEX, 15, rand));
  }
  // Algumas atrás, mais espaçadas.
  for (let i = 0; i < 3; i++) {
    const v = 0.62 - i * 0.11;
    runes.push(...glyph(4 + i * 3, (1 - v) * TEX, 12, rand));
  }
  // Cinta de runas pequenas perto da base.
  for (let i = 0; i < 14; i++) {
    const u = (i + 0.5) / 14;
    runes.push(...glyph(u * TEX, (1 - 0.22 - Math.sin(u * 9) * 0.012) * TEX, 7, rand));
  }
  const litCracks: Stroke[] = [];
  for (let i = 0; i < 4; i++) {
    const y = (1 - (0.4 + rand() * 0.3)) * TEX;
    const dir = i % 2 ? 0 : Math.PI;
    litCracks.push(crack(TEX / 2 + (dir ? -14 : 14), y, 50 + rand() * 50, dir + (rand() - 0.5) * 0.8, rand));
  }
  const cracks: Stroke[] = [];
  for (let i = 0; i < 16; i++) cracks.push(crack(rand() * TEX, rand() * TEX, 40 + rand() * 90, Math.PI / 2 + (rand() - 0.5) * 1.6, rand));
  return { runes, litCracks, cracks };
}

function paintAlbedo(scene: Scene, lay: StoneLayout): DynamicTexture {
  const tex = new DynamicTexture("tpStoneAlbedo", { width: TEX, height: TEX }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  const rand = mulberry32(911);
  // Granito cinza-azulado, um pouco mais claro no alto (desgastado pelo tempo).
  const base = ctx.createLinearGradient(0, 0, 0, TEX);
  base.addColorStop(0, "#8e8c87");
  base.addColorStop(0.5, "#78766f");
  base.addColorStop(1, "#5f5c56");
  ctx.fillStyle = base;
  ctx.fillRect(0, 0, TEX, TEX);
  // Manchas de tons diferentes.
  for (let i = 0; i < 140; i++) {
    const dark = rand() < 0.55;
    const c = dark ? `rgba(56,56,58,${(0.12 + rand() * 0.2).toFixed(2)})` : `rgba(176,174,166,${(0.1 + rand() * 0.16).toFixed(2)})`;
    blob(ctx, rand() * TEX, rand() * TEX, 12 + rand() * 55, c);
  }
  // Leve tom quente em alguns pontos (minerais).
  for (let i = 0; i < 24; i++) blob(ctx, rand() * TEX, rand() * TEX, 10 + rand() * 30, `rgba(150,120,95,${(0.08 + rand() * 0.1).toFixed(2)})`);
  // Granulação.
  for (let i = 0; i < 9000; i++) {
    ctx.fillStyle = rand() < 0.5 ? `rgba(30,32,38,${(0.1 + rand() * 0.2).toFixed(2)})` : `rgba(220,222,228,${(0.08 + rand() * 0.15).toFixed(2)})`;
    const s = 1 + rand() * 1.6;
    ctx.fillRect(rand() * TEX, rand() * TEX, s, s);
  }
  // Rachaduras: sulco escuro com borda clara embaixo.
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.lineWidth = 1.6;
  ctx.strokeStyle = "rgba(200,205,212,0.25)";
  ctx.translate(0, 1.2);
  strokeAll(ctx, [...lay.cracks, ...lay.litCracks]);
  ctx.translate(0, -1.2);
  ctx.strokeStyle = "rgba(28,30,36,0.7)";
  strokeAll(ctx, [...lay.cracks, ...lay.litCracks]);
  // Musgo: denso na base, alguns tufos no topo (onde a chuva assenta).
  const moss = ["93,122,58", "112,138,62", "72,99,46", "128,140,70"];
  for (let i = 0; i < 320; i++) {
    const top = rand() < 0.15;
    const y = top ? rand() * TEX * 0.14 : TEX * (1 - rand() ** 2.2 * 0.6);
    const c = moss[Math.floor(rand() * moss.length)];
    blob(ctx, rand() * TEX, y, 4 + rand() * 16, `rgba(${c},${(0.25 + rand() * 0.4).toFixed(2)})`);
  }
  // Líquen claro em manchinhas.
  for (let i = 0; i < 70; i++) blob(ctx, rand() * TEX, rand() * TEX, 2 + rand() * 6, `rgba(190,188,140,${(0.3 + rand() * 0.3).toFixed(2)})`);
  // Terra na base.
  const dirt = ctx.createLinearGradient(0, TEX * 0.86, 0, TEX);
  dirt.addColorStop(0, "rgba(60,48,36,0)");
  dirt.addColorStop(1, "rgba(60,48,36,0.75)");
  ctx.fillStyle = dirt;
  ctx.fillRect(0, TEX * 0.86, TEX, TEX * 0.14);
  // Runas entalhadas: sulco escuro (o brilho vem da textura emissiva).
  ctx.lineWidth = 4.5;
  ctx.strokeStyle = "rgba(24,34,48,0.85)";
  strokeAll(ctx, lay.runes);
  tex.update(true);
  tex.wrapU = Texture.WRAP_ADDRESSMODE;
  tex.wrapV = Texture.CLAMP_ADDRESSMODE;
  tex.anisotropicFilteringLevel = 8;
  return tex;
}

/** Relevo: mapa de altura pintado e convertido em mapa de normais. */
function paintNormal(scene: Scene, lay: StoneLayout): DynamicTexture {
  const tex = new DynamicTexture("tpStoneNormal", { width: TEX, height: TEX }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  const rand = mulberry32(313);
  ctx.fillStyle = "rgb(128,128,128)";
  ctx.fillRect(0, 0, TEX, TEX);
  for (let i = 0; i < 160; i++) {
    const up = rand() < 0.5;
    blob(ctx, rand() * TEX, rand() * TEX, 8 + rand() * 40, up ? `rgba(255,255,255,${(0.1 + rand() * 0.15).toFixed(2)})` : `rgba(0,0,0,${(0.1 + rand() * 0.15).toFixed(2)})`);
  }
  for (let i = 0; i < 7000; i++) {
    ctx.fillStyle = rand() < 0.5 ? "rgba(0,0,0,0.25)" : "rgba(255,255,255,0.22)";
    const s = 1 + rand() * 2;
    ctx.fillRect(rand() * TEX, rand() * TEX, s, s);
  }
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  ctx.strokeStyle = "rgba(0,0,0,0.75)";
  ctx.lineWidth = 2;
  strokeAll(ctx, [...lay.cracks, ...lay.litCracks]);
  ctx.lineWidth = 5;
  ctx.strokeStyle = "rgba(0,0,0,0.9)";
  strokeAll(ctx, lay.runes);

  heightToNormal(ctx, TEX, TEX, 3.2);
  tex.update(true, false);
  tex.wrapU = Texture.WRAP_ADDRESSMODE;
  tex.wrapV = Texture.CLAMP_ADDRESSMODE;
  return tex;
}

/**
 * Só as runas e as rachaduras acesas, já na cor (o resto é preto: não brilha). No
 * StandardMaterial a textura emissiva SOMA à emissiveColor, então a cor fica aqui e a
 * emissiveColor do material fica preta.
 */
/** Cores das runas acesas: azul na Pedra de Teleporte, âmbar fraco nos monólitos do vale. */
interface GlowPalette {
  name: string;
  /** Reflexo de luz no alto e na base (só a Pedra de Teleporte, que tem núcleo de luz). */
  wash: boolean;
  shadow: string;
  halo: string;
  crack: string;
  core: string;
  coreCrack: string;
}

const BLUE_RUNES: GlowPalette = {
  name: "tpStoneGlow",
  wash: true,
  shadow: "rgba(90,170,255,0.95)",
  halo: "rgba(80,160,255,0.45)",
  crack: "rgba(70,150,255,0.45)",
  core: "rgb(170,225,255)",
  coreCrack: "rgba(130,200,255,0.75)",
};

const AMBER_RUNES: GlowPalette = {
  name: "monolithGlow",
  wash: false,
  shadow: "rgba(255,140,40,0.9)",
  halo: "rgba(255,120,30,0.35)",
  crack: "rgba(255,110,30,0.3)",
  core: "rgb(255,190,110)",
  coreCrack: "rgba(255,160,80,0.6)",
};

function paintEmissive(scene: Scene, lay: StoneLayout, pal: GlowPalette = BLUE_RUNES): DynamicTexture {
  const tex = new DynamicTexture(pal.name, { width: TEX, height: TEX }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  ctx.fillStyle = "#000";
  ctx.fillRect(0, 0, TEX, TEX);
  if (pal.wash) {
    // Reflexo do núcleo de luz no alto da pedra e da luz do chão na base.
    const wash = ctx.createLinearGradient(0, 0, 0, TEX);
    wash.addColorStop(0, "rgba(70,130,220,0.32)");
    wash.addColorStop(0.3, "rgba(40,80,160,0.06)");
    wash.addColorStop(0.85, "rgba(30,60,140,0)");
    wash.addColorStop(1, "rgba(40,90,190,0.14)");
    ctx.fillStyle = wash;
    ctx.fillRect(0, 0, TEX, TEX);
  }
  ctx.lineCap = "round";
  ctx.lineJoin = "round";
  // Halo macio em volta dos sulcos.
  ctx.shadowColor = pal.shadow;
  ctx.shadowBlur = 10;
  ctx.strokeStyle = pal.halo;
  ctx.lineWidth = 6;
  strokeAll(ctx, lay.runes);
  ctx.lineWidth = 2.4;
  ctx.strokeStyle = pal.crack;
  strokeAll(ctx, lay.litCracks);
  // Miolo forte.
  ctx.shadowBlur = 4;
  ctx.strokeStyle = pal.core;
  ctx.lineWidth = 2.6;
  strokeAll(ctx, lay.runes);
  ctx.strokeStyle = pal.coreCrack;
  ctx.lineWidth = 1.1;
  strokeAll(ctx, lay.litCracks);
  tex.update(true);
  tex.wrapU = Texture.WRAP_ADDRESSMODE;
  tex.wrapV = Texture.CLAMP_ADDRESSMODE;
  return tex;
}

/**
 * Círculo do chão desenhado "à mão": anel de traços tortos e macios, runas soltas em
 * volta e fios de luz em espiral para o centro (nada de linhas perfeitas).
 */
function paintCircle(scene: Scene): DynamicTexture {
  const S = 512;
  const tex = new DynamicTexture("tpCircleTex", { width: S, height: S }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  const rand = mulberry32(5150);
  ctx.clearRect(0, 0, S, S);
  ctx.translate(S / 2, S / 2);
  ctx.lineCap = "round";
  ctx.shadowColor = "rgba(255,255,255,0.9)";
  // Névoa de luz bem suave no anel.
  const haze = ctx.createRadialGradient(0, 0, S * 0.2, 0, 0, S * 0.5);
  haze.addColorStop(0, "rgba(255,255,255,0)");
  haze.addColorStop(0.55, "rgba(255,255,255,0.12)");
  haze.addColorStop(0.75, "rgba(255,255,255,0.05)");
  haze.addColorStop(1, "rgba(255,255,255,0)");
  ctx.fillStyle = haze;
  ctx.fillRect(-S / 2, -S / 2, S, S);
  // Anel: arcos curtos com raio e força variando, com falhas.
  for (const base of [0.4, 0.31]) {
    let a = rand() * Math.PI * 2;
    const end = a + Math.PI * 2;
    while (a < end) {
      const len = 0.25 + rand() * 0.6;
      const r = S * (base + (rand() - 0.5) * 0.012);
      ctx.shadowBlur = 8;
      ctx.strokeStyle = `rgba(255,255,255,${(0.35 + rand() * 0.5).toFixed(2)})`;
      ctx.lineWidth = base > 0.35 ? 2 + rand() * 2 : 1.2 + rand() * 1.2;
      ctx.beginPath();
      ctx.arc(0, 0, r, a, Math.min(end, a + len));
      ctx.stroke();
      a += len + (rand() < 0.35 ? 0.05 + rand() * 0.15 : 0.01);
    }
  }
  // Runas soltas entre os anéis, viradas para fora.
  const n = 16;
  for (let i = 0; i < n; i++) {
    const a = (i / n) * Math.PI * 2 + (rand() - 0.5) * 0.1;
    ctx.save();
    ctx.rotate(a);
    ctx.translate(0, -S * 0.355);
    ctx.shadowBlur = 6;
    ctx.strokeStyle = `rgba(255,255,255,${(0.45 + rand() * 0.5).toFixed(2)})`;
    ctx.lineWidth = 2;
    for (const st of glyph(0, 0, 11, rand)) {
      ctx.beginPath();
      st.pts.forEach(([x, y], k) => (k ? ctx.lineTo(x, y) : ctx.moveTo(x, y)));
      ctx.stroke();
    }
    ctx.restore();
  }
  // Fios de luz em espiral, sumindo em direção ao centro.
  for (let i = 0; i < 7; i++) {
    let a = rand() * Math.PI * 2;
    let r = S * (0.28 + rand() * 0.03);
    const steps = 30;
    for (let k = 0; k < steps; k++) {
      const a2 = a + 0.06 + rand() * 0.02;
      const r2 = r - S * 0.006;
      ctx.shadowBlur = 5;
      ctx.strokeStyle = `rgba(255,255,255,${(0.45 * (1 - k / steps)).toFixed(3)})`;
      ctx.lineWidth = 1.6 * (1 - k / steps) + 0.4;
      ctx.beginPath();
      ctx.moveTo(Math.cos(a) * r, Math.sin(a) * r);
      ctx.lineTo(Math.cos(a2) * r2, Math.sin(a2) * r2);
      ctx.stroke();
      a = a2;
      r = r2;
    }
  }
  tex.update(false);
  tex.hasAlpha = true;
  tex.wrapU = tex.wrapV = Texture.CLAMP_ADDRESSMODE;
  return tex;
}

interface Assets {
  rock: StandardMaterial;
  /** Mesma pedra, sem runas (pedregulhos). */
  rubble: StandardMaterial;
  /** Cor do brilho das runas na camada de brilho (pulsa; compartilhada pelas pedras). */
  glowTint: Color3;
  /** Círculo mágico do chão (branco; a cor vem do material). */
  circle: DynamicTexture;
  /** Desenho das runas e rachaduras (reaproveitado pelos monólitos). */
  lay: StoneLayout;
}

const assets = new WeakMap<Scene, Assets>();

function stoneAssets(scene: Scene): Assets {
  let a = assets.get(scene);
  if (a) return a;
  const lay = layout();
  const rock = new StandardMaterial("tpStone", scene);
  rock.diffuseTexture = paintAlbedo(scene, lay);
  const bump = paintNormal(scene, lay);
  bump.level = 0.9;
  rock.bumpTexture = bump;
  rock.emissiveTexture = paintEmissive(scene, lay);
  rock.emissiveColor = Color3.Black();
  rock.specularColor = new Color3(0.12, 0.13, 0.15);
  rock.specularPower = 24;
  // Sem isto o StandardMaterial multiplica o emissivo pela textura (sulco escuro = runa apagada).
  rock.useEmissiveAsIllumination = true;
  const rubble = new StandardMaterial("tpRubble", scene);
  rubble.diffuseTexture = rock.diffuseTexture;
  rubble.bumpTexture = bump;
  rubble.specularColor = rock.specularColor.clone();
  rubble.specularPower = 24;
  a = { rock, rubble, glowTint: Color3.White(), circle: paintCircle(scene), lay };
  assets.set(scene, a);
  return a;
}

// ---------------------------------------------------------------------------------------
// Geometria
// ---------------------------------------------------------------------------------------

/** Garante que as normais apontam para fora (inverte a ordem dos triângulos se preciso). */
export function outward(positions: number[], indices: number[], probe: number, cx = 0, cz = 0): number[] {
  const normals: number[] = [];
  VertexData.ComputeNormals(positions, indices, normals);
  const rx = positions[probe * 3] - cx;
  const rz = positions[probe * 3 + 2] - cz;
  if (normals[probe * 3] * rx + normals[probe * 3 + 2] * rz < 0) {
    for (let i = 0; i < indices.length; i += 3) [indices[i + 1], indices[i + 2]] = [indices[i + 2], indices[i + 1]];
    VertexData.ComputeNormals(positions, indices, normals);
  }
  return normals;
}

/**
 * Monólito: um torno (anéis em volta do eixo) com raio deformado por ruído suave, base
 * alargada, leve inclinação e topo arredondado e cortado em viés.
 */
function rockGeometry(seed: number): VertexData {
  const rand = mulberry32(seed);
  const ph = Array.from({ length: 8 }, () => rand() * Math.PI * 2);
  const lean = 0.025 + rand() * 0.03;
  const SEG = 44;
  const RINGS = 34;
  const positions: number[] = [];
  const uvs: number[] = [];
  const indices: number[] = [];
  for (let j = 0; j <= RINGS; j++) {
    const v = j / RINGS;
    let y: number;
    let r: number;
    if (v <= 0.8) {
      const k = v / 0.8;
      y = k * 2.18;
      r = 0.56 - 0.22 * k + 0.16 * Math.max(0, 1 - k / 0.1) ** 2;
    } else {
      const a = ((v - 0.8) / 0.2) * (Math.PI / 2);
      y = 2.18 + Math.sin(a) * 0.42;
      r = 0.34 * Math.cos(a) ** 0.7;
    }
    for (let i = 0; i <= SEG; i++) {
      const u = i / SEG;
      const th = (u - 0.5) * Math.PI * 2;
      const n =
        0.06 * Math.sin(2 * th + ph[6] + y * 0.7) +
        0.075 * Math.sin(3 * th + ph[0] + y * 1.1) +
        0.045 * Math.sin(5 * th + ph[1] - y * 1.9) +
        0.025 * Math.sin(9 * th + ph[2] + y * 3.7) +
        0.02 * Math.sin(y * 6.3 + ph[3] + th * 2);
      // Seção achatada (lâmina de pedra) que torce um pouco ao subir.
      const ell = 1 + 0.2 * Math.cos(2 * th + ph[4] + y * 0.25);
      const rr = r * (1 + n) * ell;
      // Topo quebrado em viés.
      const yy = y - 0.18 * (1 + Math.cos(th - ph[5])) * 0.5 * (y / ROCK_H) ** 4;
      positions.push(Math.sin(th) * rr + lean * yy * yy, yy - 0.04, Math.cos(th) * rr);
      uvs.push(u, Math.min(1, yy / ROCK_H));
    }
  }
  const row = SEG + 1;
  for (let j = 0; j < RINGS; j++) {
    for (let i = 0; i < SEG; i++) {
      const a = j * row + i;
      const b = a + row;
      indices.push(a, b, a + 1, a + 1, b, b + 1);
    }
  }
  const normals = outward(positions, indices, Math.floor(RINGS / 2) * row + SEG / 4);
  // Emenda (u = 0 e u = 1) com a mesma normal; ponta do topo também.
  for (let j = 0; j <= RINGS; j++) {
    const a = j * row * 3;
    const b = (j * row + SEG) * 3;
    for (let c = 0; c < 3; c++) normals[a + c] = normals[b + c] = (normals[a + c] + normals[b + c]) / 2;
  }
  const top = RINGS * row * 3;
  let tx = 0;
  let ty = 0;
  let tz = 0;
  for (let i = 0; i < row; i++) {
    tx += normals[top + i * 3];
    ty += normals[top + i * 3 + 1];
    tz += normals[top + i * 3 + 2];
  }
  const tl = Math.hypot(tx, ty, tz) || 1;
  for (let i = 0; i < row; i++) normals.splice(top + i * 3, 3, tx / tl, ty / tl, tz / tl);
  const vd = new VertexData();
  vd.positions = positions;
  vd.indices = indices;
  vd.normals = normals;
  vd.uvs = uvs;
  return vd;
}

export interface RubbleOpts {
  /** Quantidade mínima (pode vir 1 a mais). */
  count: number;
  /** Distância do centro (m). */
  ring: [number, number];
  /** Tamanho de cada pedra (m). */
  size: [number, number];
}

/** Pedregulhos meio enterrados em volta de um ponto (uma malha só, sem pai). */
/** Tipos de monólito rúnico colocados pelo gerador do mundo (Vale das Pedras Vivas). */
export const MONOLITH_KINDS = ["monolith", "monolith_broken", "monolith_fallen"] as const;

/**
 * Moldes dos monólitos rúnicos do vale (o cenário desenha cópias com thin instances):
 * a mesma pedra da Pedra de Teleporte, mais fina, com runas âmbar quase apagadas.
 * - monolith: inteiro, de pé (~1,1 × 2,6 m);
 * - monolith_broken: partido em ~1,3 m, com o topo lascado;
 * - monolith_fallen: tombado, deitado ao longo de X (~2,6 m) e meio enterrado.
 */
export function monolithProtos(scene: Scene): Map<string, Mesh> {
  const a = stoneAssets(scene);
  const mat = new StandardMaterial("monolith", scene);
  mat.diffuseTexture = a.rock.diffuseTexture;
  mat.bumpTexture = a.rock.bumpTexture;
  const glow = paintEmissive(scene, a.lay, AMBER_RUNES);
  mat.emissiveTexture = glow;
  mat.emissiveColor = Color3.Black();
  mat.useEmissiveAsIllumination = true;
  mat.specularColor = new Color3(0.1, 0.1, 0.1);
  mat.specularPower = 24;
  // Pulso lento e fraco: magia antiga, quase apagada (acorda um pouco à noite pelo contraste).
  scene.onBeforeRenderObservable.add(() => {
    glow.level = 0.42 + 0.18 * Math.sin(performance.now() / 1700);
  });

  const build = (kind: (typeof MONOLITH_KINDS)[number], seed: number): Mesh => {
    const vd = rockGeometry(seed);
    const p = vd.positions as number[];
    for (let i = 0; i < p.length; i += 3) {
      p[i] *= 0.8;
      p[i + 2] *= 0.8;
      if (kind === "monolith_broken") {
        // Quebra: tudo acima de um corte irregular desce até ele (vira um topo lascado).
        const th = Math.atan2(p[i], p[i + 2]);
        const cut = 1.28 + 0.16 * Math.sin(3 * th + 1) + 0.08 * Math.sin(7 * th);
        if (p[i + 1] > cut) p[i + 1] = cut + (p[i + 1] - cut) * 0.06;
      }
      if (kind === "monolith_fallen") {
        // Deita ao longo de +X e afunda um pouco no chão.
        const x = p[i];
        const y = p[i + 1];
        p[i] = y - 1.3;
        p[i + 1] = -x + 0.34;
      }
    }
    const normals: number[] = [];
    VertexData.ComputeNormals(p, vd.indices as number[], normals);
    vd.normals = normals;
    const m = new Mesh(`proto-${kind}`, scene);
    vd.applyToMesh(m);
    m.material = mat;
    return m;
  };
  return new Map<string, Mesh>([
    ["monolith", build("monolith", 4242)],
    ["monolith_broken", build("monolith_broken", 5151)],
    ["monolith_fallen", build("monolith_fallen", 6363)],
  ]);
}

export function rubbleMesh(scene: Scene, name: string, seed: number, o: RubbleOpts): Mesh {
  const rand = mulberry32(seed);
  const parts: Mesh[] = [];
  const count = o.count + Math.floor(rand() * 2);
  for (let i = 0; i < count; i++) {
    const m = MeshBuilder.CreateIcoSphere(`${name}-${i}`, { radius: 1, subdivisions: 2, flat: false, updatable: true }, scene);
    const p = m.getVerticesData(VertexBuffer.PositionKind)!;
    const a = [rand() * 6.28, rand() * 6.28, rand() * 6.28, rand() * 6.28];
    for (let k = 0; k < p.length; k += 3) {
      const [x, y, z] = [p[k], p[k + 1], p[k + 2]];
      const n = 1 + 0.2 * Math.sin(x * 2.6 + a[0]) * Math.sin(y * 3.1 + a[1]) * Math.sin(z * 2.3 + a[2]) + 0.08 * Math.sin(x * 6 + z * 5 + a[3]);
      p[k] = x * n;
      p[k + 1] = y * n;
      p[k + 2] = z * n;
    }
    m.updateVerticesData(VertexBuffer.PositionKind, p);
    const idx = m.getIndices()!;
    const nrm: number[] = [];
    VertexData.ComputeNormals(p, idx, nrm);
    m.updateVerticesData(VertexBuffer.NormalKind, nrm);
    const ang = (i / count) * Math.PI * 2 + (rand() - 0.5) * 0.7;
    const dist = o.ring[0] + rand() * (o.ring[1] - o.ring[0]);
    const s = o.size[0] + rand() * (o.size[1] - o.size[0]);
    m.scaling.set(s * (1 + rand() * 0.5), s * (0.55 + rand() * 0.3), s * (1 + rand() * 0.4));
    m.rotation.set(rand() * 0.4, rand() * Math.PI * 2, rand() * 0.4);
    m.position.set(Math.sin(ang) * dist, s * 0.18, Math.cos(ang) * dist);
    parts.push(m);
  }
  const merged = Mesh.MergeMeshes(parts, true, true)!;
  merged.name = name;
  return merged;
}

// ---------------------------------------------------------------------------------------
// Montagem
// ---------------------------------------------------------------------------------------

/**
 * Monta a pedra: rocha e pedregulhos em `visual` (gira com o NPC), luzes do chão em
 * `root`. Animação e partículas param sozinhas quando as malhas são descartadas.
 */
export function buildTeleportStone(scene: Scene, fx: Fx, id: number, visual: TransformNode, root: TransformNode): TeleportStone {
  const { rock: rockMat, rubble: rubbleMat, circle: circleTex, glowTint } = stoneAssets(scene);
  const seed = 1009 + id * 7919;

  const rock = new Mesh(`tpRock${id}`, scene);
  rockGeometry(seed).applyToMesh(rock);
  rock.parent = visual;
  rock.material = rockMat;
  rock.receiveShadows = true;

  const rubble = rubbleMesh(scene, `tpRubble${id}`, seed + 1, { count: 5, ring: [0.62, 0.9], size: [0.14, 0.3] });
  rubble.parent = visual;
  rubble.material = rubbleMat;
  rubble.receiveShadows = true;

  // Núcleo de luz: esfera bem clara + halo macio, com fios de luz em órbita.
  const tex = fx.textures;
  const coreNode = new TransformNode(`tpCore${id}`, scene);
  coreNode.parent = visual;
  coreNode.position.y = CORE_Y;
  const core = MeshBuilder.CreateSphere(`tpCoreOrb${id}`, { diameter: 0.2, segments: 12 }, scene);
  core.parent = coreNode;
  core.material = fx.glowMaterial(new Color3(0.8, 0.94, 1), null);
  const halo = MeshBuilder.CreatePlane(`tpHalo${id}`, { size: 1 }, scene);
  halo.parent = coreNode;
  halo.billboardMode = Mesh.BILLBOARDMODE_ALL;
  halo.material = fx.glowMaterial(new Color3(0.32, 0.62, 1), tex.flare);
  const wispMat = fx.glowMaterial(new Color3(0.6, 0.86, 1), tex.flare);
  const wisps = [0, 1, 2].map((i) => {
    const w = MeshBuilder.CreatePlane(`tpWisp${id}-${i}`, { size: 0.2 }, scene);
    w.parent = coreNode;
    w.billboardMode = Mesh.BILLBOARDMODE_ALL;
    w.material = wispMat;
    return w;
  });

  // Chão: poça de luz azul (faz o papel de uma luz de verdade) e círculo de runas.
  const pool = MeshBuilder.CreateGround(`tpPool${id}`, { width: 6, height: 6 }, scene);
  pool.parent = root;
  pool.position.y = 0.06;
  pool.material = fx.glowMaterial(new Color3(0.2, 0.44, 0.95), tex.flare);
  const circle = MeshBuilder.CreateGround(`tpCircle${id}`, { width: 3.4, height: 3.4 }, scene);
  circle.parent = root;
  circle.position.y = 0.07;
  circle.material = fx.glowMaterial(new Color3(0.36, 0.7, 1), circleTex);

  const all = [rock, rubble, core, halo, ...wisps, pool, circle];
  for (const m of all) m.isPickable = false;
  // A pedra não sai do lugar: rocha, pedregulhos e poça não refazem a matriz a cada quadro
  // (o núcleo, os fios de luz e o círculo de runas se mexem).
  for (const m of [rock, rubble, pool]) m.freezeWorldMatrix();
  const glowing = [rock, core, halo, ...wisps];
  for (const m of glowing) fx.glowInclude(m);
  fx.setGlowColor(rock, glowTint);

  const aura = fx.teleportAura(root, CORE_Y);

  const t0 = performance.now() - id * 977;
  const tilt = [0.35, -0.6, 0.9];
  const obs = scene.onBeforeRenderObservable.add(() => {
    if (rock.isDisposed()) {
      scene.onBeforeRenderObservable.remove(obs);
      aura.stop();
      for (const m of glowing) fx.glowExclude(m);
      return;
    }
    const t = (performance.now() - t0) / 1000;
    const breath = 0.5 + 0.5 * Math.sin(t * 1.6);
    coreNode.position.y = CORE_Y + Math.sin(t * 1.05) * 0.08;
    core.scaling.setAll(0.9 + 0.2 * breath);
    halo.scaling.setAll(1.1 + 0.45 * breath);
    halo.visibility = 0.6 + 0.35 * breath;
    wisps.forEach((w, i) => {
      const a = t * (1.3 + i * 0.35) + i * 2.1;
      const r = 0.34 + i * 0.07;
      w.position.set(Math.cos(a) * r, Math.sin(a) * r * tilt[i], Math.sin(a) * r);
      w.visibility = 0.55 + 0.45 * Math.sin(t * 3 + i);
    });
    pool.visibility = 0.5 + 0.15 * breath;
    circle.rotation.y = t * 0.12;
    circle.visibility = 0.3 + 0.2 * breath;
    // Material compartilhado: o pulso das runas segue o relógio global (todas juntas).
    const k = 0.75 + 0.35 * (0.5 + 0.5 * Math.sin((performance.now() / 1000) * 1.6));
    if (rockMat.emissiveTexture) rockMat.emissiveTexture.level = k;
    glowTint.set(k, k, k);
  });

  return { shadowMeshes: [rock, rubble] };
}
