import { Color3, DynamicTexture, Mesh, MeshBuilder, Scene, StandardMaterial, Texture, VertexBuffer } from "@babylonjs/core";
import type { WorldData } from "./world-data";

/**
 * Nitidez do chão. O chão pintado (cores, estradas, praça) é uma textura só para o mapa
 * inteiro (~10 px/m) e fica borrado de perto. Por cima dele:
 * - grama: "detail map" repetido a cada poucos metros (folhinhas, manchas), mantendo as cores;
 * - praça: piso próprio de pedras, repetido;
 * - estradas: faixas de terra com sulcos e pedrinhas, com as bordas esmaecendo na grama.
 * Tudo gerado aqui (canvas), sem arquivos extras para baixar.
 */
export function addGroundDetail(scene: Scene, world: WorldData, groundMat: StandardMaterial): void {
  const detail = grassDetail(scene);
  detail.uScale = detail.vScale = world.size / GRASS_TILE_M;
  groundMat.detailMap.texture = detail;
  groundMat.detailMap.diffuseBlendLevel = 0.9;
  groundMat.detailMap.bumpLevel = 0;
  groundMat.detailMap.isEnabled = true;

  addPlaza(scene, world.wall.apothem + 0.5);
  addRoads(scene, world.roads);
}

/** Metros cobertos por uma repetição de cada textura. */
const GRASS_TILE_M = 2.5;
const STONE_TILE_M = 4;
const ROAD_TILE_M = 5;

/** Desenha f em (x, y) e nas cópias deslocadas, para a textura emendar sem costura. */
function wrapDraw(size: number, x: number, y: number, margin: number, f: (x: number, y: number) => void): void {
  for (const dx of [-size, 0, size]) {
    for (const dy of [-size, 0, size]) {
      const px = x + dx;
      const py = y + dy;
      if (px > -margin && px < size + margin && py > -margin && py < size + margin) f(px, py);
    }
  }
}

function rng(seed: number): () => number {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/**
 * Detail map da grama: canal vermelho = brilho (128 = neutro). Folhinhas claras e escuras
 * e manchas suaves. Verde e alfa são lidos como relevo (normal) pelo Babylon e precisam
 * ficar em 128 (plano) — com alfa 255 a normal "deitava" e o chão ficava preto.
 */
function grassDetail(scene: Scene): DynamicTexture {
  const S = 512;
  const tex = new DynamicTexture("grassDetail", { width: S, height: S }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  const rand = rng(11);
  const gray = (v: number, a = 1) => `rgba(${v},128,128,${a})`;
  ctx.fillStyle = gray(128);
  ctx.fillRect(0, 0, S, S);
  // Manchas grandes e suaves.
  for (let i = 0; i < 70; i++) {
    const x = rand() * S;
    const y = rand() * S;
    const r = 20 + rand() * 60;
    const v = rand() < 0.5 ? 100 : 156;
    wrapDraw(S, x, y, r, (px, py) => {
      const g = ctx.createRadialGradient(px, py, 0, px, py, r);
      g.addColorStop(0, gray(v, 0.35));
      g.addColorStop(1, gray(v, 0));
      ctx.fillStyle = g;
      ctx.fillRect(px - r, py - r, r * 2, r * 2);
    });
  }
  // Folhinhas: traços curtos em direções variadas.
  ctx.lineCap = "round";
  for (let i = 0; i < 2600; i++) {
    const x = rand() * S;
    const y = rand() * S;
    const len = 5 + rand() * 11;
    const ang = -Math.PI / 2 + (rand() - 0.5) * 1.6;
    const v = rand() < 0.55 ? 150 + rand() * 60 : 60 + rand() * 50;
    ctx.strokeStyle = gray(Math.round(v), 0.55);
    ctx.lineWidth = 1 + rand() * 1.6;
    wrapDraw(S, x, y, 20, (px, py) => {
      ctx.beginPath();
      ctx.moveTo(px, py);
      ctx.lineTo(px + Math.cos(ang) * len, py + Math.sin(ang) * len);
      ctx.stroke();
    });
  }
  const img = ctx.getImageData(0, 0, S, S);
  for (let i = 0; i < img.data.length; i += 4) {
    img.data[i + 1] = 128;
    img.data[i + 2] = 128;
    img.data[i + 3] = 128;
  }
  ctx.putImageData(img, 0, 0);
  tex.update(true, false); // sem pré-multiplicar o alfa (ele é dado, não transparência)
  tex.hasAlpha = false;
  tex.wrapU = tex.wrapV = Texture.WRAP_ADDRESSMODE;
  tex.anisotropicFilteringLevel = 8;
  return tex;
}

/** Piso de pedras irregulares (fileiras desencontradas), com rejunte e um pouco de volume. */
function cobbleTexture(scene: Scene): DynamicTexture {
  const S = 1024;
  const tex = new DynamicTexture("cobble", { width: S, height: S }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  const rand = rng(23);
  ctx.fillStyle = "#5d564d"; // rejunte
  ctx.fillRect(0, 0, S, S);
  const rows = 8;
  const rowH = S / rows;
  const gap = 7;
  for (let r = 0; r < rows; r++) {
    let x = (r % 2) * (rowH * 0.5) - rowH; // fileiras desencontradas
    while (x < S) {
      const w = rowH * (0.75 + rand() * 0.7);
      const tone = 128 + Math.floor(rand() * 45);
      const warm = Math.floor(rand() * 12);
      const y = r * rowH;
      const draw = (px: number, py: number) => stone(ctx, px + gap / 2, py + gap / 2, w - gap, rowH - gap, tone, warm, rand);
      wrapDraw(S, x, y, rowH * 1.6, draw);
      x += w;
    }
  }
  // Sujeira e desgaste.
  for (let i = 0; i < 1600; i++) {
    ctx.fillStyle = rand() < 0.5 ? "rgba(50,45,40,0.12)" : "rgba(220,210,190,0.08)";
    const s = 2 + rand() * 6;
    ctx.fillRect(rand() * S, rand() * S, s, s);
  }
  tex.update(true);
  tex.wrapU = tex.wrapV = Texture.WRAP_ADDRESSMODE;
  tex.anisotropicFilteringLevel = 8;
  return tex;
}

function stone(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  tone: number,
  warm: number,
  rand: () => number,
): void {
  const r = Math.min(w, h) * (0.25 + rand() * 0.15);
  const path = () => {
    ctx.beginPath();
    ctx.roundRect(x, y, w, h, r);
  };
  // Corpo da pedra com luz vinda de cima à esquerda.
  const g = ctx.createLinearGradient(x, y, x + w, y + h);
  g.addColorStop(0, `rgb(${tone + 22 + warm},${tone + 18 + warm},${tone + 12})`);
  g.addColorStop(1, `rgb(${tone - 26 + warm},${tone - 28 + warm},${tone - 32})`);
  path();
  ctx.fillStyle = g;
  ctx.fill();
  ctx.lineWidth = 3;
  ctx.strokeStyle = "rgba(40,36,32,0.45)";
  ctx.stroke();
  // Manchinhas na superfície.
  for (let i = 0; i < 6; i++) {
    ctx.fillStyle = `rgba(${rand() < 0.5 ? "60,55,50" : "235,225,205"},0.1)`;
    ctx.beginPath();
    ctx.arc(x + rand() * w, y + rand() * h, 3 + rand() * 9, 0, Math.PI * 2);
    ctx.fill();
  }
}

/** Terra batida: sulcos de roda no comprimento, pedrinhas; bordas transparentes (canal alfa). */
function dirtTexture(scene: Scene): DynamicTexture {
  const S = 512;
  const tex = new DynamicTexture("dirt", { width: S, height: S }, scene, true);
  const ctx = tex.getContext() as CanvasRenderingContext2D;
  const rand = rng(37);
  ctx.fillStyle = "#8a6d48";
  ctx.fillRect(0, 0, S, S);
  // Manchas de terra mais clara/escura (emendam no comprimento: eixo y).
  for (let i = 0; i < 260; i++) {
    const x = rand() * S;
    const y = rand() * S;
    const r = 6 + rand() * 26;
    ctx.fillStyle = rand() < 0.5 ? "rgba(98,74,48,0.22)" : "rgba(170,140,100,0.2)";
    wrapDraw(S, x, y, r, (px, py) => {
      ctx.beginPath();
      ctx.ellipse(px, py, r * 1.4, r, 0, 0, Math.PI * 2);
      ctx.fill();
    });
  }
  // Sulcos das rodas.
  for (const cx of [S * 0.34, S * 0.66]) {
    const g = ctx.createLinearGradient(cx - 22, 0, cx + 22, 0);
    g.addColorStop(0, "rgba(70,52,34,0)");
    g.addColorStop(0.5, "rgba(70,52,34,0.35)");
    g.addColorStop(1, "rgba(70,52,34,0)");
    ctx.fillStyle = g;
    ctx.fillRect(cx - 22, 0, 44, S);
  }
  // Pedrinhas.
  for (let i = 0; i < 520; i++) {
    const x = S * 0.12 + rand() * S * 0.76;
    const y = rand() * S;
    const s = 1.5 + rand() * 4;
    const v = 120 + Math.floor(rand() * 90);
    wrapDraw(S, x, y, 8, (px, py) => {
      ctx.fillStyle = `rgb(${v},${v - 12},${v - 30})`;
      ctx.beginPath();
      ctx.ellipse(px, py, s, s * 0.75, rand() * 3, 0, Math.PI * 2);
      ctx.fill();
      ctx.fillStyle = "rgba(40,30,20,0.35)";
      ctx.fillRect(px - s * 0.6, py + s * 0.5, s * 1.2, 1.2);
    });
  }
  // Bordas: alfa sobe de 0 a 1 nos primeiros/últimos 22% da largura.
  const img = ctx.getImageData(0, 0, S, S);
  for (let px = 0; px < S; px++) {
    const u = px / (S - 1);
    const edge = Math.min(u, 1 - u) / 0.22;
    const a = Math.round(255 * Math.min(1, edge * edge * (3 - 2 * Math.min(1, edge))));
    for (let py = 0; py < S; py++) img.data[(py * S + px) * 4 + 3] = a;
  }
  ctx.putImageData(img, 0, 0);
  tex.update(true);
  tex.hasAlpha = true;
  tex.wrapU = Texture.CLAMP_ADDRESSMODE;
  tex.wrapV = Texture.WRAP_ADDRESSMODE;
  tex.anisotropicFilteringLevel = 8;
  return tex;
}

function addPlaza(scene: Scene, radius: number): void {
  const plaza = MeshBuilder.CreateDisc("plaza", { radius, tessellation: 96 }, scene);
  plaza.rotation.x = Math.PI / 2; // disco nasce no plano XY
  plaza.position.y = 0.02;
  const mat = new StandardMaterial("plazaMat", scene);
  const tex = cobbleTexture(scene);
  // O disco mapeia o UV de 0 a 1 no diâmetro.
  tex.uScale = tex.vScale = (radius * 2) / STONE_TILE_M;
  mat.diffuseTexture = tex;
  mat.specularColor = new Color3(0.05, 0.05, 0.05);
  mat.zOffset = -1;
  plaza.material = mat;
  plaza.receiveShadows = true;
  plaza.isPickable = false; // o clique atravessa e acerta o chão embaixo
  freeze(plaza);
}

function addRoads(scene: Scene, roads: WorldData["roads"]): void {
  const tex = dirtTexture(scene);
  const mat = new StandardMaterial("roadMat", scene);
  mat.diffuseTexture = tex;
  mat.useAlphaFromDiffuseTexture = true;
  mat.specularColor = new Color3(0.03, 0.03, 0.03);
  mat.zOffset = -2;
  for (const [i, r] of roads.entries()) {
    const dx = r.x2 - r.x1;
    const dz = r.z2 - r.z1;
    const len = Math.hypot(dx, dz);
    const width = r.w + 2.4; // um pouco mais larga que a pintada: as bordas esmaecem
    const road = MeshBuilder.CreateGround(`road${i}`, { width, height: len }, scene);
    road.position.set((r.x1 + r.x2) / 2, 0.03, (r.z1 + r.z2) / 2);
    road.rotation.y = Math.atan2(dx, dz); // o comprimento (eixo Z local) segue a estrada
    // Mesma textura para todas: a repetição no comprimento vai direto nas coordenadas UV.
    const uv = road.getVerticesData(VertexBuffer.UVKind)!;
    for (let k = 1; k < uv.length; k += 2) uv[k] *= len / ROAD_TILE_M;
    road.setVerticesData(VertexBuffer.UVKind, uv);
    road.material = mat;
    road.receiveShadows = true;
    road.isPickable = false;
    freeze(road);
  }
}

function freeze(mesh: Mesh): void {
  mesh.freezeWorldMatrix();
  mesh.doNotSyncBoundingInfo = true;
}
