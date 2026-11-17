// Prints da cidade no jogo (Playwright, SwiftShader), em JPG 1280x720.
// uso: node tools/art/remaster-cidade/shots.mjs <prefixo> "<query>" <pontos>   (pontos: nome,nome,...)
// env: PW_E2E (pasta e2e com @playwright/test), OUT (pasta), BASE (Vite), DB (banco do servidor de prova)
import { createRequire } from "node:module";
import { execSync } from "node:child_process";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [name, query = "", list = "praca"] = process.argv.slice(2);
const BASE = process.env.BASE ?? "http://localhost:5198";
const out = process.env.OUT ?? "docs/arte/remaster-cidade/";
// nome: [x, z, zoom (rodas da roda do mouse), giro do mouse em px (arrasta com o botão direito)]
const POINTS = {
  praca: [2, 7, -8, 0],
  praca2: [-2, -3, -8, 380],
  portao: [28, 0, -8, 0],
  portao2: [-28, 0, -8, 380],
  mercado: [8, -8, -8, 0],
  taverna: [8, 10, -8, 0],
  igreja: [-8, 10, -8, 0],
  ferreiro: [-8, -8, -8, 0],
  muralha: [20, 24, -8, 0],
  casas: [22, 8, -8, 0],
  campos: [5, 86, -8, 0],
  ermos: [-124, 4, -8, 0],
  vale: [6, -190, -8, 0],
};
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist", "--disable-accelerated-2d-canvas"] });
const page = await (await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: "pt-BR" })).newPage();
page.on("pageerror", (e) => console.log("[pageerror]", String(e).slice(0, 300)));
page.on("console", (m) => { if (m.type() === "error") console.log("[erro]", m.text().slice(0, 200)); });
await page.addInitScript(() => { try { localStorage.setItem("erevia.settings", JSON.stringify({ quality: "media", qualityPicked: true })); } catch {} });
const user = "c" + Date.now().toString(36).slice(-8);
await page.request.post(BASE + "/api/auth/dev", { data: { username: user } });
const creation = await (await page.request.get(BASE + "/api/creation")).json();
const knight = creation.classes.knight;
await page.request.post(BASE + "/api/characters", { data: { name: "Cid" + user.slice(-7), class: "knight", look: { weapon: knight.weapons[0].id, offhand: knight.offhands[0].id, emblem: knight.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } } });
execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${process.env.DB ?? "erevia_cidade"} -c "UPDATE accounts SET gm=true"`, { stdio: "ignore" });
await page.goto(BASE + "/?e2e=1&" + query);
await page.getByRole("button", { name: "Entrar no mundo" }).click({ timeout: 180000 });
await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 180000 });
await page.waitForTimeout(8000);
for (let i = 0; i < 4; i++) { const b = page.getByRole("button", { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click(); await page.waitForTimeout(400); } }
const say = async (t) => { await page.keyboard.press("Enter"); await page.keyboard.type(t); await page.keyboard.press("Enter"); };
let zoomed = 0;
for (const p of list.split(",")) {
  const [x, z, zoom, turn] = POINTS[p];
  await say(`/tp ${x} ${z}`);
  await page.waitForTimeout(1500);
  await page.mouse.move(640, 300);
  const dz = zoom - zoomed;
  for (let i = 0; i < Math.abs(dz); i++) { await page.mouse.wheel(0, dz > 0 ? -300 : 300); await page.waitForTimeout(80); }
  zoomed = zoom;
  if (turn) { await page.mouse.move(640, 300); await page.mouse.down({ button: "right" }); await page.mouse.move(640 + turn, 300, { steps: 12 }); await page.mouse.up({ button: "right" }); }
  await page.waitForTimeout(3500);
  await page.screenshot({ path: `${out}${name}-${p}.jpg`, type: "jpeg", quality: 82, timeout: 180000 });
  console.log("ok", p);
}
await browser.close();
