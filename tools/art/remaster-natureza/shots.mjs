// Prints do jogo por região (Playwright, SwiftShader), com e sem ?remaster=1.
// uso: node tools/art/remaster-natureza/shots.mjs <sufixo> "<query>" [regiao,regiao...]  -> <OUT>/jogo-<regiao>-<sufixo>.png
// env: PW_E2E (pasta e2e com @playwright/test), OUT, BASE (padrão http://localhost:8095), DB
import { createRequire } from "node:module";
import { execSync } from "node:child_process";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [suffix, query = "", only] = process.argv.slice(2);
const BASE = process.env.BASE ?? "http://localhost:8095";
const out = process.env.OUT ?? "assets-src/natureza-shots/";
const REGIONS = { arredores: [50, 12], campos: [78, -24], lapides: [60, 52], ermos: [-108, 36], borda: [138, -100], cripta: [88, 88], vale: [0, -236], fundo: [-20, -305] };
const user = "n" + suffix.replace(/\W/g, "").slice(0, 4) + Date.now().toString(36).slice(-6);
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist", "--disable-accelerated-2d-canvas"] });
const page = await (await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: "pt-BR" })).newPage();
page.on("pageerror", (e) => console.log("[pageerror]", String(e).slice(0, 400)));
page.on("console", (m) => { if (process.env.ALL || m.type() === "error") console.log("[error]", m.text().slice(0, 300)); });
await page.addInitScript(() => { try { localStorage.setItem("erevia.settings", JSON.stringify({ quality: "media", qualityPicked: true })); } catch {} });
await page.request.post(BASE + "/api/auth/dev", { data: { username: user } });
const creation = await (await page.request.get(BASE + "/api/creation")).json();
const knight = creation.classes.knight;
await page.request.post(BASE + "/api/characters", { data: { name: "Nat" + user.slice(-6), class: "knight", look: { weapon: knight.weapons[0].id, offhand: knight.offhands[0].id, emblem: knight.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } } });
execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${process.env.DB ?? "erevia_natureza"} -c "UPDATE accounts SET gm=true"`, { stdio: "ignore" });
await page.goto(BASE + "/?e2e=1&" + query);
await page.getByRole("button", { name: "Entrar no mundo" }).click();
await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 180000 });
await page.waitForTimeout(6000);
for (let i = 0; i < 4; i++) { const b = page.getByRole("button", { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click(); await page.waitForTimeout(400); } }
const say = async (t) => { await page.keyboard.press("Enter"); await page.keyboard.type(t); await page.keyboard.press("Enter"); };
const wanted = only ? only.split(",") : Object.keys(REGIONS);
for (const [name, [x, z]] of Object.entries(REGIONS)) {
  if (!wanted.includes(name)) continue;
  const back = page.getByRole("button", { name: /voltar à cidade/i });
  if (await back.isVisible().catch(() => false)) { await back.click(); await page.waitForTimeout(3000); }
  await say(`/tp ${x} ${z}`);
  await page.waitForTimeout(3500);
  await page.mouse.move(640, 300);
  for (let i = 0; i < 3; i++) { await page.mouse.wheel(0, 300); await page.waitForTimeout(80); }
  await page.waitForTimeout(3500);
  await page.screenshot({ path: `${out}jogo-${name}-${suffix}.png` });
  for (let i = 0; i < 3; i++) { await page.mouse.wheel(0, -300); await page.waitForTimeout(80); }
  console.log(name, "ok");
}
await browser.close();
