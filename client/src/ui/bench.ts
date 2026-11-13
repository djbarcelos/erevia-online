import { SceneInstrumentation, type AbstractEngine, type ArcRotateCamera, type Scene } from "@babylonjs/core";
import { EntityKind } from "../gen/erevia/v1/game_pb";
import type { MapMarker } from "../world/entities";
import { regionAt, type Region } from "./regions";

/**
 * Medição de FPS (?bench=1 na URL): a câmera visita cada região, de dia e de noite, e mede
 * FPS, tempo de frame, draw calls e malhas ativas. No fim mostra uma tabela para copiar
 * (a mesma fica em window.__ereviaBench para testes automatizados).
 *
 * O personagem não sai do lugar: só a câmera (e as sombras e tochas) vão até a região.
 * Funciona porque hoje o servidor manda todas as entidades do mapa; com a área de interesse
 * (#88) as regiões longe do jogador vão aparecer sem monstros.
 */

/** Fases do ciclo medidas: meio-dia e meia-noite (daynight.ts: dia 0–0,75). */
const PHASES = [
  { label: "dia", phase: 0.375 },
  { label: "noite", phase: 0.875 },
];
/** Ângulos (rad) dos pontos em cada anel; os que caem fora do anel ou do mapa são pulados. */
const RING_ANGLES = [Math.PI / 4, (Math.PI * 11) / 12, (Math.PI * 19) / 12];
const WARMUP_MS = 1500;
const SAMPLE_MS = 2500;
/** Raio (m) em volta do ponto para contar jogadores e monstros "na tela". */
const NEAR_RADIUS = 40;

export interface BenchDeps {
  engine: AbstractEngine;
  scene: Scene;
  camera: ArcRotateCamera;
  regions: Region[];
  bounds: { minX: number; maxX: number; minZ: number; maxZ: number };
  /** Fixa a hora (0–1) ou volta ao relógio do servidor (null). */
  setTime(phase: number | null): void;
  /** Ponto que a câmera, as sombras e as tochas seguem (null = o jogador). */
  setFocus(p: { x: number; z: number } | null): void;
  forEachMarker(cb: (m: MapMarker) => void): void;
  hud: HTMLElement;
}

interface Sample {
  region: string;
  time: string;
  x: number;
  z: number;
  fps: number;
  frameMs: number;
  p95Ms: number;
  cpuMs: number;
  /** Partes do tempo de CPU: desenho da cena, render targets (sombras, brilho), escolha das malhas ativas e animação. */
  renderMs: number;
  targetsMs: number;
  evalMs: number;
  animMs: number;
  draws: number;
  active: number;
  meshes: number;
  players: number;
  monsters: number;
}

export interface BenchResult {
  device: Record<string, string | number>;
  samples: Sample[];
  table: string;
  census: CensusRow[];
}

interface CensusRow {
  group: string;
  meshes: number;
  enabled: number;
  frozen: number;
  thin: number;
  skinned: number;
  transformNodes?: number;
}

/**
 * Quantas malhas a cena tem, por grupo de nome, e quantas estão congeladas (freezeWorldMatrix):
 * o Babylon percorre todas a cada quadro para escolher as ativas.
 */
function census(scene: Scene): CensusRow[] {
  const groups = new Map<string, CensusRow>();
  const key = (name: string) => name.replace(/[\d#:.\-_ ]+.*$/, "") || name.slice(0, 12) || "(sem nome)";
  const row = (g: string) => {
    let r = groups.get(g);
    if (!r) groups.set(g, (r = { group: g, meshes: 0, enabled: 0, frozen: 0, thin: 0, skinned: 0, transformNodes: 0 }));
    return r;
  };
  for (const m of scene.meshes) {
    const r = row(key(m.name));
    r.meshes++;
    if (m.isEnabled()) r.enabled++;
    if (m.isWorldMatrixFrozen) r.frozen++;
    if ((m as unknown as { thinInstanceCount?: number }).thinInstanceCount) r.thin++;
    if (m.skeleton) r.skinned++;
  }
  for (const t of scene.transformNodes) row("[nó] " + key(t.name)).transformNodes!++;
  return [...groups.values()].sort((a, b) => b.meshes + b.transformNodes! - (a.meshes + a.transformNodes!));
}

declare global {
  interface Window {
    __ereviaBench?: { done: boolean; progress: string; result?: BenchResult };
  }
}

export function wantBench(): boolean {
  return new URLSearchParams(location.search).has("bench");
}

export async function runBench(d: BenchDeps): Promise<BenchResult> {
  window.__ereviaBench = { done: false, progress: "começando" };
  const box = document.createElement("div");
  box.className = "bench";
  box.style.cssText =
    "position:fixed;left:50%;top:12px;transform:translateX(-50%);z-index:9999;max-width:calc(100vw - 32px);" +
    "max-height:calc(100vh - 24px);overflow:auto;background:rgba(10,10,14,.92);color:#eee;padding:10px 12px;" +
    "border-radius:8px;font:12px/1.4 ui-monospace,monospace;white-space:pre";
  d.hud.appendChild(box);
  const say = (s: string) => {
    box.textContent = s;
    window.__ereviaBench!.progress = s;
  };

  const inst = new SceneInstrumentation(d.scene);
  inst.captureFrameTime = true;
  inst.captureRenderTime = true;
  inst.captureRenderTargetsRenderTime = true;
  inst.captureActiveMeshesEvaluationTime = true;
  inst.captureAnimationsTime = true;
  const cam = { alpha: d.camera.alpha, beta: d.camera.beta, radius: d.camera.radius };
  d.camera.alpha = -Math.PI / 2;
  d.camera.beta = 1.0;
  d.camera.radius = 18;

  const points = benchPoints(d.regions, d.bounds);
  const samples: Sample[] = [];
  const total = points.length * PHASES.length;
  // Volta rápida antes de medir: compila os shaders e carrega o que falta em cada lugar
  // (senão o primeiro ponto medido sai bem pior que os outros).
  say("Aquecendo…");
  for (const ph of PHASES) {
    d.setTime(ph.phase);
    for (const p of points) {
      d.setFocus(p);
      await wait(250);
    }
  }
  let n = 0;
  for (const ph of PHASES) {
    d.setTime(ph.phase);
    for (const p of points) {
      n++;
      say(`Medindo FPS ${n}/${total}: ${p.region}, ${ph.label}…`);
      d.setFocus(p);
      await wait(WARMUP_MS);
      samples.push({ region: p.region, time: ph.label, x: p.x, z: p.z, ...(await measure(d, inst, p)) });
    }
  }
  d.setFocus(null);
  d.setTime(null);
  Object.assign(d.camera, cam);
  inst.dispose();

  const device = deviceInfo(d.engine);
  const table = renderTable(device, samples);
  const result = { device, samples, table, census: census(d.scene) };
  window.__ereviaBench = { done: true, progress: "pronto", result };
  console.log(table);

  box.textContent = table + "\n";
  const copy = document.createElement("button");
  copy.textContent = "Copiar";
  copy.onclick = () => void navigator.clipboard?.writeText(table).then(() => (copy.textContent = "Copiado"));
  const close = document.createElement("button");
  close.textContent = "Fechar";
  close.style.marginLeft = "8px";
  close.onclick = () => box.remove();
  box.append(copy, close);
  return result;
}

function benchPoints(regions: Region[], b: BenchDeps["bounds"]): { region: string; x: number; z: number }[] {
  const out: { region: string; x: number; z: number }[] = [];
  const inside = (x: number, z: number) => x > b.minX + 10 && x < b.maxX - 10 && z > b.minZ + 10 && z < b.maxZ - 10;
  for (const r of regions) {
    if (r.area) {
      out.push({ region: r.name, x: r.area.x, z: r.area.z });
      continue;
    }
    if (r.safe) {
      out.push({ region: r.name, x: 0, z: 0 });
      continue;
    }
    const mid = (r.rMin + r.rMax) / 2;
    for (const a of RING_ANGLES) {
      const x = Math.cos(a) * mid;
      const z = Math.sin(a) * mid;
      if (inside(x, z) && regionAt(regions, x, z) === r) out.push({ region: r.name, x: Math.round(x), z: Math.round(z) });
    }
  }
  return out;
}

async function measure(d: BenchDeps, inst: SceneInstrumentation, p: { x: number; z: number }) {
  const dts: number[] = [];
  let cpu = 0;
  let render = 0;
  let targets = 0;
  let evalT = 0;
  let anim = 0;
  let draws = 0;
  let active = 0;
  let frames = 0;
  const start = performance.now();
  await new Promise<void>((done) => {
    const obs = d.scene.onAfterRenderObservable.add(() => {
      dts.push(d.engine.getDeltaTime());
      cpu += inst.frameTimeCounter.current;
      render += inst.renderTimeCounter.current;
      targets += inst.renderTargetsRenderTimeCounter.current;
      evalT += inst.activeMeshesEvaluationTimeCounter.current;
      anim += inst.animationsTimeCounter.current;
      draws += inst.drawCallsCounter.current;
      active += d.scene.getActiveMeshes().length;
      frames++;
      if (performance.now() - start >= SAMPLE_MS) {
        d.scene.onAfterRenderObservable.remove(obs);
        done();
      }
    });
  });
  const elapsed = performance.now() - start;
  dts.sort((a, b) => a - b);
  let players = 0;
  let monsters = 0;
  d.forEachMarker((m) => {
    if (Math.hypot(m.x - p.x, m.z - p.z) > NEAR_RADIUS) return;
    if (m.kind === EntityKind.PLAYER) players++;
    else if (m.kind === EntityKind.MONSTER) monsters++;
  });
  const f = Math.max(frames, 1);
  return {
    fps: round((frames * 1000) / elapsed, 1),
    frameMs: round(dts.reduce((s, v) => s + v, 0) / Math.max(dts.length, 1), 1),
    p95Ms: round(dts[Math.min(dts.length - 1, Math.floor(dts.length * 0.95))] ?? 0, 1),
    cpuMs: round(cpu / f, 1),
    renderMs: round(render / f, 1),
    targetsMs: round(targets / f, 1),
    evalMs: round(evalT / f, 1),
    animMs: round(anim / f, 1),
    draws: Math.round(draws / f),
    active: Math.round(active / f),
    meshes: d.scene.meshes.length,
    players,
    monsters,
  };
}

function deviceInfo(engine: AbstractEngine): Record<string, string | number> {
  const gl = (engine as unknown as { getGlInfo?: () => { renderer: string } }).getGlInfo?.();
  return {
    navegador: navigator.userAgent,
    gpu: gl?.renderer ?? "?",
    tela: `${screen.width}×${screen.height} @${devicePixelRatio}`,
    canvas: `${engine.getRenderWidth()}×${engine.getRenderHeight()}`,
    escala: engine.getHardwareScalingLevel(),
    data: new Date().toISOString(),
  };
}

/** Uma linha por região e hora: média dos pontos, e o pior p95 / maior número de draws. */
function renderTable(device: Record<string, string | number>, samples: Sample[]): string {
  const lines = Object.entries(device).map(([k, v]) => `${k}: ${v}`);
  lines.push("");
  lines.push("| Região | Hora | FPS | Frame (ms) | p95 (ms) | CPU (ms) | Draws | Malhas ativas / total | Jogadores | Monstros |");
  lines.push("|---|---|---|---|---|---|---|---|---|---|");
  const groups = new Map<string, Sample[]>();
  for (const s of samples) {
    const k = `${s.region}|${s.time}`;
    groups.set(k, [...(groups.get(k) ?? []), s]);
  }
  for (const g of groups.values()) {
    const avg = (f: (s: Sample) => number) => round(g.reduce((t, s) => t + f(s), 0) / g.length, 1);
    const max = (f: (s: Sample) => number) => Math.max(...g.map(f));
    lines.push(
      `| ${g[0].region} | ${g[0].time} | ${avg((s) => s.fps)} | ${avg((s) => s.frameMs)} | ${max((s) => s.p95Ms)} | ` +
        `${avg((s) => s.cpuMs)} | ${max((s) => s.draws)} | ${max((s) => s.active)} / ${g[0].meshes} | ` +
        `${max((s) => s.players)} | ${max((s) => s.monsters)} |`,
    );
  }
  lines.push("");
  lines.push("Onde vai o tempo de CPU (ms por quadro, média):");
  lines.push("| Região | Hora | CPU | Desenho | Sombras/brilho | Malhas ativas | Animação |");
  lines.push("|---|---|---|---|---|---|---|");
  for (const g of groups.values()) {
    const avg = (f: (s: Sample) => number) => round(g.reduce((t, s) => t + f(s), 0) / g.length, 1);
    lines.push(
      `| ${g[0].region} | ${g[0].time} | ${avg((s) => s.cpuMs)} | ${avg((s) => s.renderMs)} | ${avg((s) => s.targetsMs)} | ` +
        `${avg((s) => s.evalMs)} | ${avg((s) => s.animMs)} |`,
    );
  }
  return lines.join("\n");
}

const wait = (ms: number) => new Promise((r) => setTimeout(r, ms));
const round = (v: number, k: number) => Math.round(v * 10 ** k) / 10 ** k;
