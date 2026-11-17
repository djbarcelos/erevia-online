/**
 * Downloads do primeiro carregamento (docs/desempenho-remaster.md): bytes baixados e tempo até o primeiro
 * quadro jogável (a tela de carregamento some), com o cache vazio e a rede simulada pelo CDP.
 *
 *   node bench-download.mjs --base http://localhost:8097 --out ../.bench/downloads.json \
 *       "base:" "rm:remaster=1"          # variantes "nome:query"
 *   perfis de rede: --net sem,4g,3g      # padrão: sem,4g,3g
 */
import { chromium } from "@playwright/test";
import { mkdirSync, writeFileSync } from "node:fs";
import { dirname } from "node:path";

const argv = process.argv.slice(2);
const opt = { base: "http://localhost:8080", out: "downloads.json", net: "sem,4g,3g" };
const variants = [];
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === "--base") opt.base = argv[++i];
  else if (argv[i] === "--out") opt.out = argv[++i];
  else if (argv[i] === "--net") opt.net = argv[++i];
  else variants.push(argv[i]);
}
/** bytes/s e latência (ms) como no DevTools: "Fast 3G" = 1,6 Mbps e 150 ms de RTT (o Chrome soma 4x na latência de pedido); "4G" = 9 Mbps, 170 ms. */
const NETS = {
  sem: null,
  "4g": { downloadThroughput: (9 * 1024 * 1024) / 8, uploadThroughput: (9 * 1024 * 1024) / 8, latency: 170 },
  "3g": { downloadThroughput: (1.6 * 1024 * 1024) / 8, uploadThroughput: (750 * 1024) / 8, latency: 562.5 },
};

const browser = await chromium.launch({
  args: ["--use-angle=d3d11", "--enable-gpu", "--ignore-gpu-blocklist", "--force-high-performance-gpu"],
});
const out = [];
try {
  for (const v of variants) {
    const idx = v.indexOf(":");
    const name = idx < 0 ? v : v.slice(0, idx);
    const query = idx < 0 ? "" : v.slice(idx + 1);
    for (const net of opt.net.split(",")) {
      const ctx = await browser.newContext({ baseURL: opt.base, viewport: { width: 1280, height: 720 }, locale: "pt-BR" });
      await ctx.addInitScript(() => {
        try {
          localStorage.setItem("erevia.settings", JSON.stringify({ quality: "media", qualityPicked: true }));
        } catch {}
      });
      const page = await ctx.newPage();
      const user = `dl${Date.now().toString(36)}${Math.random().toString(36).slice(2, 5)}`;
      await page.request.post("/api/auth/dev", { data: { username: user } });
      const creation = await (await page.request.get("/api/creation")).json();
      const k = creation.classes.knight;
      await page.request.post("/api/characters", {
        data: { name: `D${user.slice(2, 11)}`, class: "knight", look: { weapon: k.weapons[0].id, offhand: k.offhands[0].id, emblem: k.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } },
      });
      const cdp = await ctx.newCDPSession(page);
      await cdp.send("Network.enable");
      await cdp.send("Network.setCacheDisabled", { cacheDisabled: true });
      const n = NETS[net];
      if (n) await cdp.send("Network.emulateNetworkConditions", { offline: false, ...n });
      const files = new Map();
      const urls = new Map();
      let t0 = 0;
      cdp.on("Network.requestWillBeSent", (e) => urls.set(e.requestId, { url: e.request.url, t: Date.now() - t0 }));
      cdp.on("Network.loadingFinished", (e) => {
        const u = urls.get(e.requestId);
        if (u) files.set(e.requestId, { url: u.url, bytes: e.encodedDataLength, done: Date.now() - t0 });
      });
      t0 = Date.now();
      await page.goto(`/${query ? "?" + query : ""}`, { waitUntil: "load", timeout: 10 * 60_000 });
      const btn = page.getByRole("button", { name: "Entrar no mundo" });
      await btn.waitFor({ timeout: 10 * 60_000 });
      const tLogin = Date.now() - t0;
      const bytesAtLogin = [...files.values()].reduce((s, f) => s + f.bytes, 0);
      const tClick = Date.now() - t0;
      await btn.click();
      await page.waitForFunction(() => document.getElementById("overlay")?.hidden === true, null, { timeout: 15 * 60_000, polling: 250 });
      await page.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
      const tPlay = Date.now() - t0;
      const bytesAtPlay = [...files.values()].reduce((s, f) => s + f.bytes, 0);
      // Os modelos (personagens) e o cenário chegam em segundo plano: espera os dois marcadores do cliente.
      const marks = async () => page.evaluate(() => Object.fromEntries(performance.getEntriesByType("mark").filter((m) => m.name.startsWith("erevia:")).map((m) => [m.name, Math.round(m.startTime)])));
      let mk = await marks();
      for (let i = 0; i < (net === "3g" ? 600 : 120) && !(mk["erevia:modelos"] && mk["erevia:cenario"]); i++) {
        await page.waitForTimeout(500);
        mk = await marks();
      }
      const sumNow = () => [...files.values()].reduce((x, f) => x + f.bytes, 0);
      const bytesAtReady = sumNow();
      // O que ainda chega depois (carga sob demanda e o tempo ocioso: o golem e os chefes começam 20 s depois dos modelos).
      await page.waitForTimeout(net === "3g" ? 30000 : 25000);
      const all = [...files.values()];
      const total = all.reduce((s, f) => s + f.bytes, 0);
      const kind = (u) => (/\.glb(\?|$)/.test(u) ? "glb" : /\.(ktx2?|basis)(\?|$)/.test(u) || /\/ktx2\//.test(u) ? "ktx2/decoder" : /\.js(\?|$)/.test(u) ? "js" : /\.(png|jpe?g|webp)(\?|$)/.test(u) ? "imagem" : /\.(mp3|ogg|wav|m4a)(\?|$)/.test(u) ? "audio" : "outros");
      const byKind = {};
      for (const f of all) byKind[kind(f.url)] = (byKind[kind(f.url)] ?? 0) + f.bytes;
      const glbs = all.filter((f) => kind(f.url) === "glb").map((f) => ({ file: f.url.replace(/^.*\/models\//, ""), kb: Math.round(f.bytes / 1024), at: f.done }));
      const row = { name, query, net, tLoginMs: tLogin, tClickMs: tClick, tPlayableMs: tPlay, marcas: mk, mbAtReady: +(bytesAtReady / 1048576).toFixed(2), enterToPlayMs: tPlay - tClick, mbAtLogin: +(bytesAtLogin / 1048576).toFixed(2), mbAtPlayable: +(bytesAtPlay / 1048576).toFixed(2), mbTotal: +(total / 1048576).toFixed(2), requests: all.length, mbByKind: Object.fromEntries(Object.entries(byKind).map(([a, b]) => [a, +(b / 1048576).toFixed(2)])), glbs };
      out.push(row);
      console.log(`[${name} / ${net}] login ${tLogin} ms, jogável ${tPlay} ms, modelos ${mk['erevia:modelos']} ms, cenário ${mk['erevia:cenario']} ms (entrar→jogar ${tPlay - tClick} ms), ${row.mbAtPlayable} MB até jogar, ${row.mbTotal} MB total, ${all.length} pedidos`, JSON.stringify(row.mbByKind));
      await ctx.close();
    }
  }
} finally {
  await browser.close();
  mkdirSync(dirname(opt.out), { recursive: true });
  writeFileSync(opt.out, JSON.stringify(out, null, 1));
}
