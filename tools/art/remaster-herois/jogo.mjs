// Prints do jogo com os 4 heróis lado a lado (Playwright, SwiftShader), tunnel#94.
// Entram 4 contas (uma por classe), todas vão ao mesmo ponto da praça, e a primeira tira os prints de frente,
// de lado e de costas (girando a câmera com o mouse) e com os emotes.
//   node tools/art/remaster-herois/jogo.mjs <pasta-de-saida> "<query>" [zoom]
// env: PW_E2E (pasta e2e com o @playwright/test), BASE (servidor), DB (banco), LOOK (JSON do visual)
import { createRequire } from 'node:module';
import { execSync } from 'node:child_process';
const require = createRequire(process.env.PW_E2E ?? 'C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/');
const { chromium } = require('@playwright/test');
const [out, query = 'remaster=1', zoom = '10'] = process.argv.slice(2);
const BASE = process.env.BASE ?? 'http://localhost:8093';
const DB = process.env.DB ?? 'erevia_herois';
const tag = Date.now().toString(36).slice(-5);
const browser = await chromium.launch({ args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist'] });
const classes = (process.env.CLASSES ?? 'knight,barbarian,mage,rogue').split(',');
const pages = [];
for (const [i, cls] of classes.entries()) {
  const ctx = await browser.newContext({ viewport: i === 0 ? { width: 1280, height: 720 } : { width: 480, height: 270 }, locale: 'pt-BR' });
  const page = await ctx.newPage();
  page.setDefaultTimeout(300000);
  page.on('pageerror', (e) => console.log(`[pageerror ${cls}]`, String(e).slice(0, 300)));
  await page.addInitScript(() => { try { localStorage.setItem('erevia.settings', JSON.stringify({ quality: 'baixa', qualityPicked: true })); } catch {} });
  const user = `h${i}${tag}`;
  let r = await page.request.post(`${BASE}/api/auth/dev`, { data: { username: user } });
  console.log('login', cls, r.status());
  const creation = await (await page.request.get(`${BASE}/api/creation`)).json();
  const c = creation.classes[cls];
  r = await page.request.post(`${BASE}/api/characters`, { data: { name: `${cls.slice(0, 3)}${tag}`.slice(0, 10) + 'x', class: cls, look: { weapon: c.weapons[0].id, offhand: c.offhands?.[0]?.id ?? '', emblem: c.emblem, skin: 0, hair: 0, outfit: 0, head: '', hideHat: false, ...JSON.parse(process.env.LOOK || '{}') } } });
  console.log('char', cls, r.status(), c.weapons[0].id, c.offhands?.[0]?.id);
  execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${DB} -c "UPDATE accounts SET gm=true"`, { stdio: 'ignore' });
  pages.push(page);
}
for (const [i, page] of pages.entries()) {
  await page.goto(`${BASE}/?e2e=1&${query}`, { waitUntil: 'domcontentloaded', timeout: 120000 });
  await page.waitForTimeout(3000);
  await page.getByRole('button', { name: 'Entrar no mundo' }).click({ timeout: 240000 });
  await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 240000 });
  await page.waitForTimeout(2500);
  for (let k = 0; k < 4; k++) { const b = page.getByRole('button', { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click({ timeout: 240000 }); await page.waitForTimeout(300); } }
  console.log('no mundo', classes[i]);
}
const say = async (page, t) => { await page.keyboard.press('Enter'); await page.keyboard.type(t); await page.keyboard.press('Enter'); };
for (const [i, page] of pages.entries()) { await say(page, `/tp ${2 + i * 1.4} 7`); await page.waitForTimeout(700); }
const main = pages[0];
const wheel = async (n) => { await main.mouse.move(640, 300); for (let i = 0; i < n; i++) { await main.mouse.wheel(0, -300); await main.waitForTimeout(80); } };
await wheel(+zoom);
await main.waitForTimeout(3500);
await main.screenshot({ path: `${out}/jogo-${process.env.TAG ?? '4-classes'}.png` });
// de costas e de lado: gira a câmera arrastando o mouse
const drag = async (dx) => { await main.mouse.move(640, 360); await main.mouse.down({ button: 'right' }); await main.mouse.move(640 + dx, 360, { steps: 8 }); await main.mouse.up({ button: 'right' }); await main.waitForTimeout(1500); };
await drag(220);
await main.screenshot({ path: `${out}/jogo-${process.env.TAG ?? '4-classes'}-lado.png` });
await drag(220);
await main.screenshot({ path: `${out}/jogo-${process.env.TAG ?? '4-classes'}-costas.png` });
await drag(-440);
// emotes: cada conta faz um (acenar, dançar, sentar) e a primeira tira o print
const emotes = ['/acenar', '/dancar', '/sentar', '/acenar'].slice(0, classes.length);
for (const [i, page] of pages.entries()) await say(page, emotes[i]);
await main.waitForTimeout(2500);
await main.screenshot({ path: `${out}/jogo-${process.env.TAG ?? '4-classes'}-emotes.png` });
await browser.close();
console.log('ok');
