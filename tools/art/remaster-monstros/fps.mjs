// Medição da região cheia (VAT): o personagem vai até a região e conta quadros por 20 s, com e sem ?remaster=1.
// uso: node tools/art/remaster-monstros/fps.mjs "<query>" <x> <z>   (query: "" ou "remaster=1")
// SwiftShader (WebGL por software): o FPS não vale como GPU real, só a comparação entre as duas rodadas.
import { createRequire } from "node:module";
import { execSync } from "node:child_process";
const require = createRequire(process.env.PW_E2E ?? "C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/");
const { chromium } = require("@playwright/test");
const [query = "", x = "95", z = "0"] = process.argv.slice(2);
const BASE = process.env.BASE ?? "http://localhost:8092";
const user = "f" + Date.now().toString(36).slice(-7);
const browser = await chromium.launch({ args: ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist", "--disable-accelerated-2d-canvas"] });
const page = await (await browser.newContext({ viewport: { width: 1280, height: 720 }, locale: "pt-BR" })).newPage();
await page.addInitScript(() => { try { localStorage.setItem("erevia.settings", JSON.stringify({ quality: "baixa", qualityPicked: true })); } catch {} });
await page.request.post(BASE + "/api/auth/dev", { data: { username: user } });
const creation = await (await page.request.get(BASE + "/api/creation")).json();
const knight = creation.classes.knight;
await page.request.post(BASE + "/api/characters", { data: { name: "Fp" + user.slice(-8), class: "knight", look: { weapon: knight.weapons[0].id, offhand: knight.offhands[0].id, emblem: knight.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } } });
execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${process.env.DB ?? "erevia_monstros"} -c "UPDATE accounts SET gm=true"`, { stdio: "ignore" });
// Personagem forte (nível 45, constituição alta): sobrevive aos monstros enquanto a câmera chega.
execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${process.env.DB ?? "erevia_monstros"} -c "UPDATE characters SET level=45, attr_con=250, hp=NULL, mp=NULL"`, { stdio: "ignore" });
await page.goto(BASE + "/?e2e=1&" + query);
await page.waitForTimeout(3000);
await page.getByRole("button", { name: "Entrar no mundo" }).click();
await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 180000 });
await page.waitForTimeout(5000);
for (let i = 0; i < 4; i++) { const b = page.getByRole("button", { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click(); await page.waitForTimeout(400); } }
await page.keyboard.press("Enter"); await page.keyboard.type(`/tp ${x} ${z}`); await page.keyboard.press("Enter");
await page.waitForTimeout(15000); // modelos carregando e transcodificando
const r = await page.evaluate(() => new Promise((ok) => {
  let n = 0, worst = 0, last = performance.now();
  const t0 = last;
  const tick = () => { const t = performance.now(); worst = Math.max(worst, t - last); last = t; n++; if (t - t0 < 20000) requestAnimationFrame(tick); else ok({ frames: n, fps: +(n / ((t - t0) / 1000)).toFixed(2), worstMs: Math.round(worst) }); };
  requestAnimationFrame(tick);
}));
const mons = await page.evaluate(() => window.__ereviaE2E.entities("monster", true));
console.log(JSON.stringify({ query, x, z, ...r, monstros: mons.length }));
await browser.close();
