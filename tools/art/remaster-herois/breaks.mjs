// Confere as pausas do parado no laboratório: node tools/art/remaster-herois/breaks.mjs <modelo> [segundos]
import { createRequire } from "node:module";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [model = "knight", secs = "45"] = process.argv.slice(2);
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist"] });
const page = await (await browser.newContext({ viewport: { width: 400, height: 300 } })).newPage();
await page.goto(`${process.env.BASE ?? "http://localhost:5199"}/lab.html?remaster=1`);
await page.waitForFunction(() => window.lab, null, { timeout: 60000 });
console.log(model, JSON.stringify(await page.evaluate(([m, s]) => window.lab.idleBreaks(m, +s), [model, secs])));
await browser.close();
