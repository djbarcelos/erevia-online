// Velocidade natural (m/s) dos clipes de andar e correr de cada modelo, no laboratório (client/lab.html).
//   node tools/art/remaster-herois/foot-speeds.mjs [remaster=1|""] modelo [modelo...]
import { createRequire } from "node:module";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [query, ...ms] = process.argv.slice(2);
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist"] });
const page = await (await browser.newContext({ viewport: { width: 400, height: 300 } })).newPage();
await page.goto(`${process.env.BASE ?? "http://localhost:5199"}/lab.html?${query}`);
await page.waitForFunction(() => window.lab, null, { timeout: 90000 });
for (const m of ms) console.log(m, JSON.stringify(await page.evaluate((x) => window.lab.footSpeeds(x), m)));
await browser.close();
