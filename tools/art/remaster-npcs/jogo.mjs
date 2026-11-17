// Prints do jogo com os NPCs do remaster (Playwright, SwiftShader).
//   node tools/art/remaster-npcs/jogo.mjs <pasta-de-saida> "<query>" [zoom]
// Entra com um cavaleiro, vai a cada grupo de NPCs da praça (/tp, é GM) e tira o print; depois clica num NPC
// (abre a janela e dispara o cumprimento) e tira outro. env: PW_E2E (pasta e2e com o @playwright/test), BASE, DB, SHOT_GROUPS (JSON [[nome,x,z,giro,zoom]...]), CLICK (nome do NPC a clicar)
import { createRequire } from 'node:module';
import { execSync } from 'node:child_process';
const require = createRequire(process.env.PW_E2E ?? `${import.meta.dirname}/../../../e2e/`); // npm ci em e2e/ instala o @playwright/test
const { chromium } = require('@playwright/test');
const [out, query = 'remaster=1', zoom = '3'] = process.argv.slice(2);
const BASE = process.env.BASE ?? 'http://localhost:8097';
const DB = process.env.DB ?? 'erevia_npcs';
const tag = Date.now().toString(36).slice(-5);
const browser = await chromium.launch({ args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist'] });
const ctx = await browser.newContext({ viewport: { width: 1024, height: 576 }, locale: 'pt-BR' });
const page = await ctx.newPage();
page.setDefaultTimeout(300000);
page.on('pageerror', (e) => console.log('[pageerror]', String(e).slice(0, 300)));
page.on('console', (m) => { if (['error', 'warning'].includes(m.type())) console.log('[' + m.type() + ']', m.text().slice(0, 240)); });
await page.addInitScript(() => { try { localStorage.setItem('erevia.settings', JSON.stringify({ quality: 'baixa', qualityPicked: true })); } catch {} });
const user = `n${tag}`;
await page.request.post(`${BASE}/api/auth/dev`, { data: { username: user } });
const creation = await (await page.request.get(`${BASE}/api/creation`)).json();
const c = creation.classes.knight;
await page.request.post(`${BASE}/api/characters`, { data: { name: `Vis${tag}`.slice(0, 10), class: 'knight', look: { weapon: c.weapons[0].id, offhand: c.offhands?.[0]?.id ?? '', emblem: c.emblem, skin: 0, hair: 0, outfit: 0, head: '', hideHat: false } } });
execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${DB} -c "UPDATE accounts SET gm=true"`, { stdio: 'ignore' });
await page.addInitScript(() => { try { localStorage.setItem('erevia.novidades.vista', '9999-99-99'); } catch {} });
await page.goto(`${BASE}/?e2e=1&${query}`, { waitUntil: 'domcontentloaded', timeout: 120000 });
await page.waitForTimeout(3000);
await page.getByRole('button', { name: 'Entrar no mundo' }).click({ timeout: 240000 });
await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 240000 });
await page.waitForTimeout(3000);
for (let k = 0; k < 4; k++) { const b = page.getByRole('button', { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) { await b.click(); await page.waitForTimeout(300); } }
if (await page.getByText('O que há de novo').isVisible().catch(() => false)) { await page.keyboard.press('Escape'); await page.waitForTimeout(500); }
// Reconexões e a janela de novidades aparecem de vez em quando (SwiftShader é lento): espera o jogo voltar e fecha a janela.
const ready = async () => {
  for (let k = 0; k < 40; k++) {
    const ok = await page.evaluate(() => !!window.__ereviaE2E?.me?.()).catch(() => false);
    const news = await page.getByText('O que há de novo').isVisible().catch(() => false);
    if (news) { const b = page.getByRole('button', { name: /^(Fechar|FECHAR)$/ }).first(); if (await b.isVisible().catch(() => false)) await b.click(); else await page.keyboard.press('Escape'); }
    if (ok && !news) return;
    await page.waitForTimeout(1500);
  }
};
const say = async (t) => { await page.keyboard.press('Enter'); await page.keyboard.type(t); await page.keyboard.press('Enter'); };
const wheel = async (n) => { await page.mouse.move(512, 250); for (let i = 0; i < Math.abs(n); i++) { await page.mouse.wheel(0, n > 0 ? -300 : 300); await page.waitForTimeout(60); } };
const drag = async (dx) => { await page.mouse.move(512, 290); await page.mouse.down({ button: 'right' }); await page.mouse.move(512 + dx, 290, { steps: 8 }); await page.mouse.up({ button: 'right' }); await page.waitForTimeout(1200); };
await page.waitForFunction(() => window.__ereviaE2E?.entities?.('npc').length > 3, null, { timeout: 280000 });
await page.waitForTimeout(4000);
for (let k = 0; k < 3; k++) { if (await page.getByText('O que há de novo').isVisible().catch(() => false)) { await page.keyboard.press('Escape'); await page.waitForTimeout(600); } }
await page.waitForFunction(() => window.__ereviaE2E?.entities?.('npc').length > 3, null, { timeout: 280000 });
const npcs = await page.evaluate(() => window.__ereviaE2E.entities('npc'));
console.log('npcs', JSON.stringify(npcs));
// Grupos: [arquivo, x, z] (o personagem fica 3 m a frente dos NPCs, olhando para eles)
const groups = JSON.parse(process.env.SHOT_GROUPS ?? '[["cal0",0,-3,0],["cal1",0,-3,100],["cal2",0,-3,200],["cal3",0,-3,300]]');
await wheel(+zoom);
for (const [name, x, z, turn = 0, zoomMore = 0] of groups) {
  await ready();
  await say(`/tp ${x} ${z}`);
  await page.waitForTimeout(3500);
  await ready();
  if (turn) await drag(turn);
  if (zoomMore) await wheel(zoomMore);
  await page.waitForTimeout(2500);
  await ready();
  await page.screenshot({ path: `${out}/jogo-${name}.png` });
  if (zoomMore) await wheel(-zoomMore);
  if (turn) await drag(-turn);
}
if (process.env.CLICK) {
  const n = npcs.find((m) => m.name === process.env.CLICK);
  if (n) {
    await say(`/tp ${n.x + 2} ${n.z + 2}`);
    await page.waitForTimeout(2500);
    const p = await page.evaluate((id) => window.__ereviaE2E.screenPos(id), n.id);
    console.log('screenPos', JSON.stringify(p));
    if (p) { await page.mouse.click(p.x, p.y); await page.waitForTimeout(900); await page.screenshot({ path: `${out}/jogo-cumprimento-${n.name}.png` }); }
  }
}
await browser.close();
console.log('ok');
