// Prints do laboratório de heróis (client/lab.html): node tools/art/remaster-v3/lab-shot.mjs <saida.png> '<json do spec>' [query]
// Precisa do `vite dev` em BASE (padrão http://localhost:5199) e do Playwright (PW_E2E).
import { createRequire } from "node:module";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [out, spec, query = "remaster=1"] = process.argv.slice(2);
const BASE = process.env.BASE ?? "http://localhost:5199";
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist"] });
const page = await (await browser.newContext({ viewport: { width: +(process.env.W ?? 1100), height: +(process.env.H ?? 700) } })).newPage();
page.on("pageerror", (e) => console.log("[pageerror]", String(e).slice(0, 300)));
page.on("console", (m) => { if (m.type() === "error") console.log("[error]", m.text().slice(0, 200)); });
await page.goto(`${BASE}/lab.html?${query}`);
await page.waitForFunction(() => window.lab, null, { timeout: 60000 });
for (const [i, s] of (Array.isArray(JSON.parse(spec)) ? JSON.parse(spec) : [JSON.parse(spec)]).entries()) {
  await page.evaluate((s) => window.lab.show(s), s);
  await page.screenshot({ path: out.replace(/(\.\w+)$/, Array.isArray(JSON.parse(spec)) ? `-${i}$1` : "$1") });
}
await browser.close();
