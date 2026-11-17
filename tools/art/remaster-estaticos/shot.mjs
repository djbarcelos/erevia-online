// Prints do jogo com o herói segurando cada tipo de arma (Playwright, SwiftShader).
// uso: node tools/art/remaster-estaticos/shot.mjs "<query>" <sufixo>   -> <OUT>/jogo-<classe>-<arma>-<sufixo>.png
// env: PW_E2E, BASE, OUT, DB, ONLY (regex nos nomes dos casos)
import { createRequire } from "node:module";
import { execSync } from "node:child_process";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [query = "", suffix = "rm"] = process.argv.slice(2);
const BASE = process.env.BASE ?? "http://localhost:8097";
const OUT = process.env.OUT ?? "assets-src/shots/";
const cases = [
  ["knight", "sword_common", "shield_buckler", "espada-escudo-redondo"],
  ["knight", "blade_common", "shield_badge", "lamina-curva-brasao"],
  ["knight", "sword_common", "shield_tower", "espada-escudo-pesado"],
  ["barbarian", "axe_common", "shield_buckler", "machado-broquel"],
  ["mage", "staff_common", "", "cajado"],
  ["mage", "wand_common", "", "varinha"],
  ["mage", "staff_dark_common", "", "cajado-sombrio"],
  ["rogue", "daggers_common", "", "adagas"],
].filter((c) => new RegExp(process.env.ONLY ?? ".").test(c[3]));
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist", "--disable-accelerated-2d-canvas"] });
for (const [cls, weapon, offhand, label] of cases) {
  const ctx = await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: "pt-BR" });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => console.log("[pageerror]", String(e).slice(0, 300)));
  await page.addInitScript(() => { try { localStorage.setItem("erevia.settings", JSON.stringify({ quality: "baixa", qualityPicked: true })); } catch {} });
  const user = "w" + label.replace(/\W/g, "").slice(0, 4) + Date.now().toString(36).slice(-6);
  await page.request.post(BASE + "/api/auth/dev", { data: { username: user } });
  const creation = await (await page.request.get(BASE + "/api/creation")).json();
  const r = await page.request.post(BASE + "/api/characters", { data: { name: "Arma" + user.slice(-8), class: cls, look: { weapon, offhand, emblem: creation.classes[cls].emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } } });
  console.log(label, "char", r.status());
  execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${process.env.DB ?? "erevia_estaticos"} -c "UPDATE accounts SET gm=true"`, { stdio: "ignore" });
  await page.goto(BASE + "/?e2e=1&" + query);
  await page.waitForTimeout(3000);
  await page.getByRole("button", { name: "Entrar no mundo" }).click();
  await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 120000 });
  await page.waitForTimeout(6000);
  for (let i = 0; i < 4; i++) { const b = page.getByRole("button", { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click(); await page.waitForTimeout(300); } }
  const say = async (t) => { await page.keyboard.press("Enter"); await page.keyboard.type(t); await page.keyboard.press("Enter"); };
  await say("/tp 2 7");
  await page.waitForTimeout(1500);
  await page.mouse.move(640, 300);
  for (let i = 0; i < 12; i++) { await page.mouse.wheel(0, -300); await page.waitForTimeout(80); }
  if (process.env.TURN) { await page.mouse.move(640, 360); await page.mouse.down({ button: "right" }); await page.mouse.move(640 + Number(process.env.TURN), 360, { steps: 20 }); await page.mouse.up({ button: "right" }); }
  await page.waitForTimeout(2500);
  await page.screenshot({ path: `${OUT}jogo-${label}-${suffix}.png`, clip: { x: 340, y: 60, width: 600, height: 600 } });
  await ctx.close();
}
await browser.close();
