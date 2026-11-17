// Prints do jogo (Playwright, SwiftShader) do Brejo de Lamagris (tunnel#139): vai até cada ponto com /tp e fotografa o mundo.
// uso: node tools/art/remaster-brejo/shot.mjs <prefixo> <alvo...>      alvo = nome:x:z[:zoom[:noite]]
// env: PW_E2E (pasta com @playwright/test), OUT (pasta de saída), BASE (servidor), DB (banco do servidor), QUERY (parâmetros da URL)
import { createRequire } from "node:module";
import { execSync } from "node:child_process";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/remaster-padrao/e2e/");
const { chromium } = require("@playwright/test");
const [prefix = "b", ...targets] = process.argv.slice(2);
const BASE = process.env.BASE ?? "http://localhost:18139";
const DB = process.env.DB ?? "erevia_brejo";
const out = process.env.OUT ?? "";
const user = "b" + prefix.replace(/\W/g, "").slice(0, 4) + Date.now().toString(36).slice(-6);
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist", "--disable-accelerated-2d-canvas"] });
const page = await (await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: "pt-BR" })).newPage();
page.on("pageerror", (e) => console.log("[pageerror]", String(e).slice(0, 400)));
page.on("console", (m) => { if (m.type() === "error") console.log("[erro]", m.text().slice(0, 300)); });
await page.addInitScript(() => { try { localStorage.setItem("erevia.settings", JSON.stringify({ quality: "baixa", qualityPicked: true })); } catch {} });
await page.request.post(BASE + "/api/auth/dev", { data: { username: user } });
const creation = await (await page.request.get(BASE + "/api/creation")).json();
const knight = creation.classes.knight;
await page.request.post(BASE + "/api/characters", { data: { name: "Br" + user.slice(-8), class: "knight", look: { weapon: knight.weapons[0].id, offhand: knight.offhands[0].id, emblem: knight.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } } });
const psql = (sql) => execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${DB} -tA -c "${sql}"`).toString().trim();
// O personagem pode demorar a existir no banco (a máquina está ocupada): espera, e só então vira GM nível 45.
for (let i = 0; i < 30 && psql("SELECT count(*) FROM characters") === "0"; i++) await new Promise((r) => setTimeout(r, 1000));
psql("UPDATE accounts SET gm=true");
psql("UPDATE characters SET level=45, attr_con=250, hp=NULL, mp=NULL");
console.log("nível no banco:", psql("SELECT max(level) FROM characters"));
await page.goto(BASE + "/?e2e=1&" + (process.env.QUERY ?? ""));
await page.waitForTimeout(3000);
await page.getByRole("button", { name: "Entrar no mundo" }).click();
await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 120000 });
await page.waitForTimeout(+(process.env.PRE ?? 6000));
for (let i = 0; i < 6; i++) { const b = page.getByRole("button", { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click({ timeout: 3000 }).catch(() => {}); await page.waitForTimeout(400); } }
const say = async (t) => { await page.keyboard.press("Enter"); await page.keyboard.type(t); await page.keyboard.press("Enter"); };
const wheel = async (n) => { await page.mouse.move(640, 300); for (let i = 0; i < Math.abs(n); i++) { await page.mouse.wheel(0, n < 0 ? 300 : -300); await page.waitForTimeout(80); } };
if (process.env.EVAL) console.log(JSON.stringify(await page.evaluate(process.env.EVAL)));
let zoomed = 0;
for (const t of targets) {
  const [name, x, z, zoom = "0"] = t.split(":");
  // O /tp pelo chat falha quando a máquina está ocupada: repete até o personagem chegar.
  for (let i = 0; i < 8; i++) {
    await say(`/tp ${x} ${z}`);
    await page.waitForTimeout(3500);
    const me = await page.evaluate(() => window.__ereviaE2E.me());
    if (me && Math.hypot(me.x - +x, me.z - +z) < 6) break;
  }
  if (+zoom !== zoomed) { await wheel(+zoom - zoomed); zoomed = +zoom; }
  await page.waitForTimeout(+(process.env.WAIT ?? 4500));
  for (let i = 0; i < 3; i++) { const b = page.getByRole("button", { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click({ timeout: 5000 }).catch(() => {}); await page.waitForTimeout(500); } }
  await page.evaluate(() => { for (const id of ["hud", "overlay"]) document.getElementById(id)?.style.setProperty("display", "none", "important"); });
  await page.waitForTimeout(150);
  await page.screenshot({ path: `${out}${prefix}-${name}.jpg`, type: "jpeg", quality: 82, timeout: 180000 });
  await page.evaluate(() => { for (const id of ["hud", "overlay"]) document.getElementById(id)?.style.removeProperty("display"); });
  console.log(name, "ok");
}
await browser.close();
