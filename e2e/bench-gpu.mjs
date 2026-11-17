/**
 * Medição de FPS com GPU de verdade (docs/desempenho-remaster.md). Abre o Chromium do Playwright com a
 * GPU (D3D11 via ANGLE, sem SwiftShader), prova qual placa é lendo WEBGL_debug_renderer_info e roda o
 * ?bench=1 completo em cada variante pedida. Sem janela (headless), a placa de alto desempenho.
 *
 *   node bench-gpu.mjs --base http://localhost:8097 --out ../.bench/antes \
 *       "base:remaster=0" "rm:" "rm-baixa:remaster=1&quality=baixa"
 *
 * Cada variante é "nome:query" (a query vai depois de ?bench=1&). Opções:
 *   --capped   mantém o limite de 60 FPS do vsync (padrão: sem limite, para ver a folga)
 *   --headed   com janela
 *   --igpu     usa a placa integrada (Intel) em vez da NVIDIA
 *   --profile  com bench=aqui: grava o perfil de CPU (CDP Profiler) da medição em <out>/<nome>.profile.txt: as funções que mais
 *              gastam tempo próprio. Use um build sem minificar (vite build --minify false) para os nomes saírem legíveis.
 *   --cpu N    a CPU roda N vezes mais devagar (CDP Emulation.setCPUThrottlingRate): com --igpu, um celular médio
 *              (a máquina de desenvolvimento é rápida demais: o FPS fica preso na CPU do navegador e esconde o custo)
 *   --tp X,Z   (com bench=aqui) leva o personagem até lá com /tp antes de medir: região de caça cheia.
 *              Precisa de --db <banco> para dar GM e nível 45 ao personagem (docker exec erevia-postgres-1 psql).
 *              Variante de exemplo: "ossos:bench=aqui" (sem ?bench=1 na frente quando a query já tem bench=)
 *   --width/--height/--dpr   tamanho da janela (padrão 1280x720 @1; 1920x1080 @2 aperta a GPU)
 * Escreve <out>/<nome>.json (o resultado do bench + renderer + nvidia-smi) e <out>/<nome>.md.
 */
import { chromium } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { setTimeout as sleep } from "node:timers/promises";
import { mkdirSync, writeFileSync } from "node:fs";

const argv = process.argv.slice(2);
const opt = { base: "http://localhost:8080", out: "bench-gpu-out", capped: false, igpu: false, headed: false, width: 1280, height: 720, dpr: 1 };
const variants = [];
for (let i = 0; i < argv.length; i++) {
  const a = argv[i];
  if (a === "--base") opt.base = argv[++i];
  else if (a === "--out") opt.out = argv[++i];
  else if (a === "--capped") opt.capped = true;
  else if (a === "--headed") opt.headed = true;
  else if (a === "--igpu") opt.igpu = true;
  else if (a === "--width") opt.width = +argv[++i];
  else if (a === "--height") opt.height = +argv[++i];
  else if (a === "--dpr") opt.dpr = +argv[++i];
  else if (a === "--profile") opt.profile = true;
  else if (a === "--cpu") opt.cpu = +argv[++i];
  else if (a === "--tp") opt.tp = argv[++i];
  else if (a === "--db") opt.db = argv[++i];
  else variants.push(a);
}
mkdirSync(opt.out, { recursive: true });

export function gpuArgs(capped, igpu = false) {
  const args = ["--use-angle=d3d11", "--enable-gpu", "--ignore-gpu-blocklist"];
  // Sem a flag, o Chrome fica com a placa integrada (a Intel Arc desta máquina), bem mais fraca: serve de "aparelho médio".
  if (!igpu) args.push("--force-high-performance-gpu");
  if (!capped) args.push("--disable-frame-rate-limit", "--disable-gpu-vsync");
  return args;
}

function smi() {
  try {
    const o = execFileSync("nvidia-smi", ["--query-gpu=memory.used,utilization.gpu,power.draw,clocks.gr", "--format=csv,noheader,nounits"], { encoding: "utf8" });
    const [mem, util, power, clock] = o.trim().split(",").map((x) => +x.trim());
    return { memUsedMB: mem, utilPct: util, powerW: power, clockMHz: clock };
  } catch {
    return null;
  }
}

const browser = await chromium.launch({ headless: !opt.headed, args: gpuArgs(opt.capped, opt.igpu) });
console.log("Chromium", browser.version());
try {
  for (const v of variants) {
    const idx = v.indexOf(":");
    const name = idx < 0 ? v : v.slice(0, idx);
    const query = idx < 0 ? "" : v.slice(idx + 1);
    const ctx = await browser.newContext({ baseURL: opt.base, viewport: { width: opt.width, height: opt.height }, deviceScaleFactor: opt.dpr, locale: "pt-BR" });
    await ctx.addInitScript(() => {
      try {
        if (!localStorage.getItem("erevia.settings")) localStorage.setItem("erevia.settings", JSON.stringify({ quality: "media", qualityPicked: true }));
      } catch {}
    });
    const page = await ctx.newPage();
    const errors = [];
    page.on("pageerror", (e) => errors.push(String(e)));
    const user = `gpu${Date.now().toString(36)}${Math.random().toString(36).slice(2, 5)}`;
    const login = await page.request.post("/api/auth/dev", { data: { username: user } });
    if (login.status() >= 300) throw new Error("login: " + (await login.text()));
    const creation = await (await page.request.get("/api/creation")).json();
    const knight = creation.classes.knight;
    const mk = await page.request.post("/api/characters", {
      data: { name: `G${user.slice(3, 12)}`, class: "knight", look: { weapon: knight.weapons[0].id, offhand: knight.offhands[0].id, emblem: knight.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } },
    });
    if (mk.status() >= 300) throw new Error("personagem: " + (await mk.text()));

    if (opt.tp) {
      for (const sql of ["UPDATE accounts SET gm=true", "UPDATE characters SET level=45, attr_con=250, hp=NULL, mp=NULL"]) {
        execFileSync("docker", ["exec", "erevia-postgres-1", "psql", "-U", "erevia", "-d", opt.db, "-c", sql], { stdio: "ignore" });
      }
    }
    if (opt.cpu > 1) await (await ctx.newCDPSession(page)).send("Emulation.setCPUThrottlingRate", { rate: opt.cpu });
    const smiBefore = smi();
    const t0 = Date.now();
    await page.goto(query.includes("bench=") ? `/?${query}` : `/?bench=1${query ? "&" + query : ""}`);
    await page.getByRole("button", { name: "Entrar no mundo" }).click();
    if (opt.tp) {
      await page.waitForFunction(() => document.getElementById("overlay")?.hidden === true, null, { timeout: 120_000 });
      await sleep(8000);
      for (let i = 0; i < 4; i++) {
        const b = page.getByRole("button", { name: /^(Fechar|FECHAR)$/ }).first();
        if (await b.isVisible().catch(() => false)) {
          await b.click();
          await sleep(400);
        }
      }
      await page.keyboard.press("Enter");
      await page.keyboard.type(`/tp ${opt.tp.replace(",", " ")}`);
      await page.keyboard.press("Enter");
      await sleep(25_000); // modelos novos baixando e a região entrando na área de interesse
      await page.evaluate(() => (window.__ereviaBenchGo = true));
    }
    let cdpProf = null;
    if (opt.profile) {
      cdpProf = await ctx.newCDPSession(page);
      await cdpProf.send("Profiler.enable");
      await cdpProf.send("Profiler.setSamplingInterval", { interval: 200 });
      await cdpProf.send("Profiler.start");
    }
    await page.waitForFunction(() => window.__ereviaBench?.done === true, null, { timeout: 6 * 60_000, polling: 2000 });
    const seconds = (Date.now() - t0) / 1000;
    if (cdpProf) {
      const { profile } = await cdpProf.send("Profiler.stop");
      writeFileSync(`${opt.out}/${name}.cpuprofile`, JSON.stringify(profile));
      const self = new Map();
      const byId = new Map(profile.nodes.map((n) => [n.id, n]));
      const dt = profile.timeDeltas;
      profile.samples.forEach((id, i) => {
        const n = byId.get(id);
        const k = `${n.callFrame.functionName || "(anônima)"} ${n.callFrame.url.split("/").pop()}:${n.callFrame.lineNumber + 1}`;
        self.set(k, (self.get(k) ?? 0) + (dt[i] ?? 0));
      });
      const total = [...self.values()].reduce((a, b) => a + b, 0);
      const rows = [...self.entries()].sort((a, b) => b[1] - a[1]).slice(0, 45);
      writeFileSync(`${opt.out}/${name}.profile.txt`, rows.map(([k, v]) => `${((100 * v) / total).toFixed(1).padStart(5)}%  ${(v / 1000).toFixed(0).padStart(6)} ms  ${k}`).join(String.fromCharCode(10)) + String.fromCharCode(10));
    }
    const smiAfter = smi();
    const result = await page.evaluate(() => window.__ereviaBench.result);
    const renderer = await page.evaluate(() => {
      const c = document.createElement("canvas");
      const g = c.getContext("webgl2");
      const e = g?.getExtension("WEBGL_debug_renderer_info");
      return e ? `${g.getParameter(e.UNMASKED_RENDERER_WEBGL)} / ${g.getParameter(e.UNMASKED_VENDOR_WEBGL)}` : "?";
    });
    writeFileSync(`${opt.out}/${name}.json`, JSON.stringify({ name, query, renderer, seconds, smiBefore, smiAfter, errors, result }, null, 1));
    writeFileSync(`${opt.out}/${name}.md`, `renderer: ${renderer}\nquery: ${query}\nsegundos: ${seconds}\n\n${result.table}\n`);
    console.log(`[${name}] ${renderer} em ${seconds.toFixed(0)} s, ${result.samples.length} amostras, ${errors.length} erros de página`);
    await ctx.close();
  }
} finally {
  await browser.close();
}
