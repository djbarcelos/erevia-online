/**
 * Resume as rodadas de bench-gpu.mjs numa tabela (markdown): por variante, a média e a pior região.
 *   node bench-summary.mjs ../.bench/antes [../.bench/depois]
 * FPS p5 = 1000 / p95 do tempo de frame (o FPS que 95% dos quadros alcançam); pior região = a de menor FPS médio.
 */
import { readdirSync, readFileSync } from "node:fs";

const avg = (a) => a.reduce((s, x) => s + x, 0) / Math.max(a.length, 1);
const r1 = (v) => Math.round(v * 10) / 10;

export function load(dir) {
  const out = {};
  for (const f of readdirSync(dir).filter((f) => f.endsWith(".json"))) out[f.replace(/\.json$/, "")] = JSON.parse(readFileSync(`${dir}/${f}`, "utf8"));
  return out;
}

export function summarize(run) {
  const s = run.result.samples;
  const groups = new Map();
  for (const x of s) groups.set(`${x.region}|${x.time}`, [...(groups.get(`${x.region}|${x.time}`) ?? []), x]);
  const regs = [...groups.entries()].map(([k, g]) => ({ k, fps: avg(g.map((x) => x.fps)), p95: Math.max(...g.map((x) => x.p95Ms)) }));
  const worst = regs.reduce((a, b) => (b.fps < a.fps ? b : a));
  const day = (t) => s.filter((x) => x.time === t);
  return {
    fps: r1(avg(s.map((x) => x.fps))),
    fpsWorst: r1(worst.fps),
    worstRegion: worst.k.replace("|", " · "),
    p5: r1(avg(regs.map((r) => 1000 / r.p95))),
    p5Worst: r1(Math.min(...regs.map((r) => 1000 / r.p95))),
    frameMs: r1(avg(s.map((x) => x.frameMs))),
    gpuMs: r1(avg(s.map((x) => x.gpuMs))),
    cpuMs: r1(avg(s.map((x) => x.cpuMs))),
    animMs: r1(avg(s.map((x) => x.animMs))),
    renderMs: r1(avg(s.map((x) => x.renderMs))),
    tris: Math.round(avg(s.map((x) => x.tris)) / 1000),
    draws: Math.round(avg(s.map((x) => x.draws))),
    fpsDay: r1(avg(day("dia").map((x) => x.fps))),
    fpsNight: r1(avg(day("noite").map((x) => x.fps))),
    texMB: run.result.memory?.textureMB,
    heapMB: run.result.memory?.heapMB,
    vramMB: run.smiAfter && run.smiBefore ? run.smiAfter.memUsedMB - run.smiBefore.memUsedMB : null,
  };
}

const header = "| Variante | FPS médio | FPS pior região | FPS p5 médio | FPS p5 pior | Frame (ms) | GPU (ms) | CPU (ms) | Desenho (ms) | Animação (ms) | Triângulos (mil) | Draws | Dia | Noite | Texturas (MB) | VRAM (MB) |\n|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|";
const row = (n, x) => `| ${n} | ${x.fps} | ${x.fpsWorst} (${x.worstRegion}) | ${x.p5} | ${x.p5Worst} | ${x.frameMs} | ${x.gpuMs} | ${x.cpuMs} | ${x.renderMs} | ${x.animMs} | ${x.tris} | ${x.draws} | ${x.fpsDay} | ${x.fpsNight} | ${x.texMB ?? ""} | ${x.vramMB ?? ""} |`;

if (process.argv[1].endsWith("bench-summary.mjs")) {
  for (const dir of process.argv.slice(2)) {
    const runs = load(dir);
    console.log(`\n### ${dir}\n`);
    console.log(header);
    for (const [n, r] of Object.entries(runs)) console.log(row(n, summarize(r)));
  }
}
