// Bench de contadores (draw calls, malhas) por região. env: PW_E2E, BASE, OUT (pasta do JSON).
// uso: node tools/art/remaster-cidade/bench.mjs <nome> "<query>"  (query ex.: bench=contadores ou bench=contadores&remaster=1)
import { createRequire } from "node:module";
import { writeFileSync } from "node:fs";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [name, query] = process.argv.slice(2);
const BASE = process.env.BASE ?? "http://localhost:8091";
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist", "--disable-accelerated-2d-canvas"] });
const page = await (await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: "pt-BR" })).newPage();
await page.addInitScript(() => { try { localStorage.setItem("erevia.settings", JSON.stringify({ quality: "baixa", qualityPicked: true })); } catch {} });
const user = "b" + name.replace(/\W/g, "").slice(0, 4) + Date.now().toString(36).slice(-6);
await page.request.post(BASE + "/api/auth/dev", { data: { username: user } });
const creation = await (await page.request.get(BASE + "/api/creation")).json();
const knight = creation.classes.knight;
await page.request.post(BASE + "/api/characters", { data: { name: "Bn" + user.slice(-8), class: "knight", look: { weapon: knight.weapons[0].id, offhand: knight.offhands[0].id, emblem: knight.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } } });
await page.goto(BASE + "/?" + query);
await page.getByRole("button", { name: "Entrar no mundo" }).click({ timeout: 180000 });
await page.waitForFunction(() => window.__ereviaBench?.done === true, null, { timeout: 25 * 60_000, polling: 3000 });
const result = await page.evaluate(() => window.__ereviaBench.result);
writeFileSync(`${process.env.OUT ?? "assets-src/shots/"}bench-${name}.json`, JSON.stringify(result, null, 1));
console.log(result.table);
await browser.close();
