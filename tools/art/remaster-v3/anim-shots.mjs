// Prints do jogo (Playwright, SwiftShader) no mesmo ponto da praça e do esqueleto.
// Quadros de animação no jogo: parado e correndo (segura W), recortados em volta do herói.
// uso: node tools/art/remaster-v3/anim-shots.mjs <nome> "<query>" [zoomWheel]   -> <OUT>/<nome>-idle.jpg e -run<N>.jpg
// env: PW_E2E (pasta e2e com o @playwright/test instalado), OUT (pasta de saída), LOOK (JSON do visual), BASE (servidor), DB (banco do servidor)
import { createRequire } from "node:module";
import { execSync } from "node:child_process";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [name, query = "", zoom = "6"] = process.argv.slice(2);
const BASE = process.env.BASE ?? "http://localhost:8091";
const out = process.env.OUT ?? "assets-src/remaster-v3/shots/";
const user = "f" + name.replace(/W/g, "").slice(0, 4) + Date.now().toString(36).slice(-6);
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist", "--disable-accelerated-2d-canvas"] });
const ctx = await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: "pt-BR" });
const page = await ctx.newPage();
page.on("console", (m) => { if (["error", "warning"].includes(m.type())) console.log("[" + m.type() + "]", m.text().slice(0, 300)); });
page.on("pageerror", (e) => console.log("[pageerror]", String(e).slice(0, 400)));
await page.addInitScript(() => { try { localStorage.setItem("erevia.settings", JSON.stringify({ quality: "baixa", qualityPicked: true })); } catch {} });
let r = await page.request.post(BASE + "/api/auth/dev", { data: { username: user } }); console.log("login", r.status(), (await r.text()).slice(0,200));
const creation = await (await page.request.get(BASE + "/api/creation")).json();
const knight = creation.classes.knight;
r = await page.request.post(BASE + "/api/characters", { data: { name: "Fatia" + user.slice(-6), class: "knight", look: { weapon: knight.weapons[0].id, offhand: knight.offhands[0].id, emblem: knight.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false, ...JSON.parse(process.env.LOOK || "{}") } } });
console.log("char", r.status());
execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${process.env.DB ?? "erevia_remasterv3"} -c "UPDATE accounts SET gm=true"`);
await page.goto(BASE + "/?e2e=1&" + query);
await page.waitForTimeout(4000); await page.screenshot({ path: out + "debug.png" });
await page.getByRole("button", { name: "Entrar no mundo" }).click();
await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 120000 });
await page.waitForTimeout(8000);
for (let i = 0; i < 4; i++) { const b = page.getByRole("button", { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click(); await page.waitForTimeout(400); } }
const say = async (t) => { await page.keyboard.press("Enter"); await page.keyboard.type(t); await page.keyboard.press("Enter"); };
const wheel = async (n) => { await page.mouse.move(640, 300); for (let i = 0; i < n; i++) { await page.mouse.wheel(0, -300); await page.waitForTimeout(80); } };
await say("/tp 2 7");
await page.waitForTimeout(1500);
await wheel(+zoom);
await page.waitForTimeout(3500);
const clip = { x: 440, y: 200, width: 400, height: 400 };
await page.screenshot({ path: out + name + "-idle.jpg", clip, type: "jpeg", quality: 85 });
await page.keyboard.down("KeyW");
for (let i = 0; i < 5; i++) {
  await page.waitForTimeout(450);
  const p = await page.evaluate(() => window.__ereviaE2E.me());
  await page.screenshot({ path: out + name + "-run" + i + ".jpg", clip: { x: 440, y: 200, width: 400, height: 400 }, type: "jpeg", quality: 85 });
  console.log("run", i, JSON.stringify(p));
}
await page.keyboard.up("KeyW");
await browser.close();
