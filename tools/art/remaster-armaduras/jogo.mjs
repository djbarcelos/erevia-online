// Prints do jogo com armadura por raridade (Playwright, SwiftShader), tunnel#96.
// Cada conta é um personagem com peito, elmo e capa de uma raridade (itens inseridos no banco antes de entrar no
// mundo); todos vão ao mesmo ponto da praça e a primeira conta tira os prints, de frente, de lado e de costas.
//   MODE=tiers   -> 3 cavaleiros: comum, raro, épico
//   MODE=classes -> as 4 classes em épico
//   node tools/art/remaster-armaduras/jogo.mjs <pasta-de-saida> [zoom]
// env: PW_E2E (pasta e2e com o @playwright/test), BASE (servidor), DB (banco)
import { createRequire } from 'node:module';
import { execSync } from 'node:child_process';
const require = createRequire(process.env.PW_E2E ?? 'C:/Users/DanielTeofilo/erevia-wt/perf-ci/e2e/');
const { chromium } = require('@playwright/test');
const [out, zoom = '10'] = process.argv.slice(2);
const BASE = process.env.BASE ?? 'http://localhost:8098';
const DB = process.env.DB ?? 'erevia_armaduras';
const MODE = process.env.MODE ?? 'tiers';
const SUFFIX = ['common', 'good', 'rare', 'epic'];
const people = MODE === 'classes'
  ? ['knight', 'barbarian', 'mage', 'rogue'].map((cls) => ({ cls, g: 3 }))
  : [0, 2, 3].map((g) => ({ cls: 'knight', g }));
const tag = Date.now().toString(36).slice(-5);
const sql = (q) => execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${DB} -c "${q}"`, { stdio: 'ignore' });
const browser = await chromium.launch({ args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist'] });
const pages = [];
for (const [i, p] of people.entries()) {
  const ctx = await browser.newContext({ viewport: i === 0 ? { width: 1280, height: 720 } : { width: 256, height: 144 }, locale: 'pt-BR' });
  const page = await ctx.newPage();
  page.setDefaultTimeout(300000);
  page.on('pageerror', (e) => console.log(`[pageerror ${i}]`, String(e).slice(0, 300)));
  await page.addInitScript(() => { try { localStorage.setItem('erevia.settings', JSON.stringify({ quality: 'baixa', qualityPicked: true })); } catch {} });
  const user = `a${i}${tag}`;
  let r = await page.request.post(`${BASE}/api/auth/dev`, { data: { username: user } });
  const creation = await (await page.request.get(`${BASE}/api/creation`)).json();
  const c = creation.classes[p.cls];
  const name = `${p.cls.slice(0, 3)}${i}${tag}`.slice(0, 10);
  r = await page.request.post(`${BASE}/api/characters`, { data: { name, class: p.cls, look: { weapon: c.weapons[0].id, offhand: c.offhands?.[0]?.id ?? '', emblem: c.emblem, skin: 0, hair: 0, outfit: 0, head: '', hideHat: false } } });
  console.log('char', p.cls, p.g, r.status());
  const s = SUFFIX[p.g];
  sql(`UPDATE characters SET level = 18 WHERE name = '${name}'`);
  for (const [slot, item] of [[2, `armor_${s}`], [3, `helm_${s}`], [7, `cape_${s}`]]) {
    sql(`INSERT INTO character_items (character_id, place, slot, item_id, count) SELECT id, 1, ${slot}, '${item}', 1 FROM characters WHERE name = '${name}'`);
  }
  sql('UPDATE accounts SET gm = true');
  pages.push(page);
}
// as contas pequenas entram primeiro e a principal por último (o SwiftShader divide a CPU entre as páginas)
for (const [i, page] of [...pages.entries()].reverse()) {
  await page.goto(`${BASE}/?e2e=1&remaster=1`, { waitUntil: 'domcontentloaded', timeout: 900000 });
  await page.waitForTimeout(3000);
  await page.getByRole('button', { name: 'Entrar no mundo' }).click({ timeout: 600000, noWaitAfter: true });
  await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 240000 });
  await page.waitForTimeout(2500);
  for (let k = 0; k < 4; k++) { const b = page.getByRole('button', { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click({ timeout: 600000, noWaitAfter: true }); await page.waitForTimeout(300); } }
  if (await page.getByText('O que há de novo').isVisible().catch(() => false)) { await page.keyboard.press('Escape'); await page.waitForTimeout(500); }
  console.log('no mundo', i);
}
const say = async (page, t) => { await page.keyboard.press('Enter'); await page.keyboard.type(t); await page.keyboard.press('Enter'); };
for (const [i, page] of pages.entries()) { await say(page, `/tp ${2 + i * 1.4} 7`); await page.waitForTimeout(700); }
const main = pages[0];
const wheel = async (n) => { await main.mouse.move(640, 300); for (let i = 0; i < n; i++) { await main.mouse.wheel(0, -300); await main.waitForTimeout(80); } };
await wheel(+zoom);
await main.waitForTimeout(8000); // as texturas de raridade e as peças baixam sob demanda
await main.screenshot({ path: `${out}/jogo-${MODE}.png` });
const drag = async (dx) => { await main.mouse.move(640, 360); await main.mouse.down({ button: 'right' }); await main.mouse.move(640 + dx, 360, { steps: 8 }); await main.mouse.up({ button: 'right' }); await main.waitForTimeout(1500); };
await drag(220);
await main.screenshot({ path: `${out}/jogo-${MODE}-lado.png` });
await drag(220);
await main.screenshot({ path: `${out}/jogo-${MODE}-costas.png` });
await browser.close();
console.log('ok');
