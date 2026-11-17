/**
 * Medição de FPS com GPU de verdade (docs/desempenho-remaster.md). Abre o Chromium do Playwright com a
 * GPU (D3D11 via ANGLE, sem SwiftShader), prova qual placa é lendo WEBGL_debug_renderer_info e roda o
 * ?bench=1 completo em cada variante pedida. Sem janela (headless), a placa de alto desempenho.
 *
 *   node bench-gpu.mjs --base http://localhost:8097 --out ../.bench/antes \
 *       "base:" "rm:remaster=1" "rm-baixa:remaster=1&quality=baixa"
 *
 * Cada variante é "nome:query" (a query vai depois de ?bench=1&). Opções:
 *   --capped   mantém o limite de 60 FPS do vsync (padrão: sem limite, para ver a folga)
 *   --headed   com janela
 *   --width/--height   tamanho da janela (padrão 1280x720)
 * Escreve <out>/<nome>.json (o resultado do bench + renderer + nvidia-smi) e <out>/<nome>.md.
 */
import { chromium } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";

const argv = process.argv.slice(2);
const opt = { base: "http://localhost:8080", out: "bench-gpu-out", capped: false, headed: false, width: 1280, height: 720 };
const variants = [];
for (let i = 0; i < argv.length; i++) {
  const a = argv[i];
  if (a === "--base") opt.base = argv[++i];
  else if (a === "--out") opt.out = argv[++i];
  else if (a === "--capped") opt.capped = true;
  else if (a === "--headed") opt.headed = true;
  else if (a === "--width") opt.width = +argv[++i];
  else if (a === "--height") opt.height = +argv[++i];
  else variants.push(a);
}
mkdirSync(opt.out, { recursive: true });

export function gpuArgs(capped) {
  const args = ["--use-angle=d3d11", "--enable-gpu", "--ignore-gpu-blocklist", "--force-high-performance-gpu", "--disable-accelerated-2d-canvas"];
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

const browser = await chromium.launch({ headless: !opt.headed, args: gpuArgs(opt.capped) });
console.log("Chromium", browser.version());
try {
  for (const v of variants) {
    const idx = v.indexOf(":");
    const name = idx < 0 ? v : v.slice(0, idx);
    const query = idx < 0 ? "" : v.slice(idx + 1);
    const ctx = await browser.newContext({ baseURL: opt.base, viewport: { width: opt.width, height: opt.height }, locale: "pt-BR" });
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

    const smiBefore = smi();
    const t0 = Date.now();
    await page.goto(`/?bench=1${query ? "&" + query : ""}`);
    await page.getByRole("button", { name: "Entrar no mundo" }).click();
    await page.waitForFunction(() => window.__ereviaBench?.done === true, null, { timeout: 15 * 60_000, polling: 2000 });
    const seconds = (Date.now() - t0) / 1000;
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
