// Velocidade natural (m/s) de clipes de andar e correr em um modelo: node tools/art/remaster-herois/clip-speeds.mjs <modelo> [padrão]
import { createRequire } from "node:module";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [model, pat = "Walk|Run|Sprint|Skeletons"] = process.argv.slice(2);
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist"] });
const page = await (await browser.newContext({ viewport: { width: 300, height: 200 } })).newPage();
await page.goto(`${process.env.BASE ?? "http://localhost:5199"}/lab.html?${process.env.QUERY ?? ""}`);
await page.waitForFunction(() => window.lab, null, { timeout: 90000 });
await page.evaluate((m) => window.lab.show({ items: [{ model: m }] }), model);
const names = (await page.evaluate(() => window.lab.clipNames())).filter((n) => new RegExp(pat).test(n));
for (const n of names) console.log(model, n, JSON.stringify(await page.evaluate(([m, c]) => window.lab.clipSpeed(m, c), [model, n])));
await browser.close();
