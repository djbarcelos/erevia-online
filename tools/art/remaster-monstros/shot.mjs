// Prints do jogo (Playwright, SwiftShader) dos monstros do remaster, de perto e na região.
// uso: node tools/art/remaster-monstros/shot.mjs "<query>" <prefixo> <alvo...>
//   alvo = nome:regexDoMonstro:x:z[:zoom]  (vai até (x,z), acha o monstro e fotografa de perto)
// env: PW_E2E (pasta com @playwright/test), OUT (pasta de saída), BASE, DB
import { createRequire } from "node:module";
import { execSync } from "node:child_process";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [query = "", prefix = "m", ...targets] = process.argv.slice(2);
const BASE = process.env.BASE ?? "http://localhost:8092";
const out = process.env.OUT ?? "docs/arte/remaster-monstros/";
const user = "m" + prefix.replace(/\W/g, "").slice(0, 4) + Date.now().toString(36).slice(-6);
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist", "--disable-accelerated-2d-canvas"] });
const page = await (await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: "pt-BR" })).newPage();
page.on("pageerror", (e) => console.log("[pageerror]", String(e).slice(0, 400)));
page.on("console", (m) => { if (m.type() === "error") console.log("[erro]", m.text().slice(0, 300)); });
await page.addInitScript(() => { try { localStorage.setItem("erevia.settings", JSON.stringify({ quality: "baixa", qualityPicked: true })); } catch {} });
await page.request.post(BASE + "/api/auth/dev", { data: { username: user } });
const creation = await (await page.request.get(BASE + "/api/creation")).json();
const knight = creation.classes.knight;
await page.request.post(BASE + "/api/characters", { data: { name: "Mo" + user.slice(-8), class: "knight", look: { weapon: knight.weapons[0].id, offhand: knight.offhands[0].id, emblem: knight.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } } });
execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${process.env.DB ?? "erevia_monstros"} -c "UPDATE accounts SET gm=true"`, { stdio: "ignore" });
// Personagem forte (nível 45, constituição alta): sobrevive aos monstros enquanto a câmera chega.
execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${process.env.DB ?? "erevia_monstros"} -c "UPDATE characters SET level=45, attr_con=250, hp=NULL, mp=NULL"`, { stdio: "ignore" });
await page.goto(BASE + "/?e2e=1&" + query);
await page.waitForTimeout(3000);
await page.getByRole("button", { name: "Entrar no mundo" }).click();
await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 120000 });
await page.waitForTimeout(6000);
for (let i = 0; i < 4; i++) { const b = page.getByRole("button", { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click(); await page.waitForTimeout(400); } }
const say = async (t) => { await page.keyboard.press("Enter"); await page.keyboard.type(t); await page.keyboard.press("Enter"); };
const wheel = async (n) => { await page.mouse.move(640, 300); for (let i = 0; i < n; i++) { await page.mouse.wheel(0, -300); await page.waitForTimeout(80); } };
const snap = async (path) => {
  // Sem a interface HTML (chat, barras): só o mundo e o que o Babylon GUI desenha (nomes, vida, dano).
  await page.evaluate(() => { for (const id of ["hud", "overlay"]) document.getElementById(id)?.style.setProperty("display", "none", "important"); });
  await page.waitForTimeout(150);
  await page.screenshot({ path, type: "jpeg", quality: 82 });
  await page.evaluate(() => { for (const id of ["hud", "overlay"]) document.getElementById(id)?.style.removeProperty("display"); });
};
let zoomed = 0;
const plain = (t) => t.split(":")[1] === "-";
for (const t of targets) {
  const [name, re, x, z, zoom = "6", dist = "6"] = t.split(":");
  await say(`/tp ${x} ${z}`);
  await page.waitForTimeout(3500);
  if (plain(t)) {
    if (+zoom !== zoomed) { await wheel(Math.abs(+zoom - zoomed)); zoomed = +zoom; }
    await page.waitForTimeout(4000);
    await snap(`${out}${prefix}-${name}.jpg`);
    console.log(name, "região ok");
    continue;
  }
  let mons = [];
  for (let i = 0; i < 12 && !mons.some((m) => new RegExp(re).test(m.name)); i++) { await page.waitForTimeout(500); mons = await page.evaluate(() => window.__ereviaE2E.entities("monster", true)); }
  const m = mons.find((q) => new RegExp(re).test(q.name));
  if (!m && process.env.DEBUG_SHOT) console.log(mons.map((q) => q.name + "@" + q.x.toFixed(0) + "," + q.z.toFixed(0)).join(" "));
  console.log(name, "->", m ? `${m.name} (${m.id}) em ${m.x.toFixed(0)},${m.z.toFixed(0)}` : "nenhum", `(${mons.length} monstros por perto)`);
  if (!m) continue;
  if (+zoom > zoomed) { await wheel(+zoom - zoomed); zoomed = +zoom; }
  let done = false;
  for (let k = 0; k < (process.env.ANG ? 3 : 28) && !done; k++) {
    const ang = process.env.SWEEP ? (k / 8) * Math.PI * 2 : process.env.ANG ? +process.env.ANG : process.env.CARD ? (k % 4) * Math.PI / 2 : (k / 7) * Math.PI * 2 + k * 0.3;
    const cur = (await page.evaluate(() => window.__ereviaE2E.entities("monster", true))).find((q) => q.id === m.id) ?? m;
    await say(`/tp ${(cur.x + Math.cos(ang) * +dist).toFixed(0)} ${(cur.z + Math.sin(ang) * +dist).toFixed(0)}`);
    await page.waitForTimeout(+(process.env.SETTLE ?? 1500));
    const sp = await page.evaluate((id) => window.__ereviaE2E.screenPos(id), m.id);
    if (process.env.DEBUG_SHOT) console.log("  tentativa", k, JSON.stringify(sp), cur.x.toFixed(0), cur.z.toFixed(0), JSON.stringify(await page.evaluate(() => window.__ereviaE2E.me())));
    if (process.env.SNAPALL) { await snap(`${out}${prefix}-${name}-${k}.jpg`); if (k >= +process.env.SNAPALL - 1) break; continue; }
    if (sp && sp.x > 250 && sp.x < 1030 && sp.y > 100 && sp.y < 520) {
      await page.waitForTimeout(+(process.env.SETTLE2 ?? 800));
      await snap(`${out}${prefix}-${name}.jpg`);
      done = true;
    }
  }
  console.log(name, done ? "ok" : "falhou");
}
await browser.close();
