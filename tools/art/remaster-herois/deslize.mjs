// Mede o deslize dos pés no jogo (tunnel#94): um jogador anda e para (WASD), outro o observa de longe (entidade
// remota); os dois registram a cada quadro a posição, a velocidade desenhada, `walking` e o peso dos clipes.
//   node tools/art/remaster-herois/deslize.mjs <saida.json> "<query>" [classe]
// query: "" (KayKit) ou "remaster=1"; "legacymove=1" liga a regra antiga (para o "antes").
// env: PW_E2E, BASE (servidor), DB
import { createRequire } from 'node:module';
import { execSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';
const require = createRequire(process.env.PW_E2E ?? 'C:/Users/DanielTeofilo/erevia-wt/fatia-vertical/e2e/');
const { chromium } = require('@playwright/test');
const [out, query = '', cls = 'knight'] = process.argv.slice(2);
const BASE = process.env.BASE ?? 'http://localhost:8093';
const DB = process.env.DB ?? 'erevia_herois';
const tag = Date.now().toString(36).slice(-5);
const browser = await chromium.launch({ args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist'] });
async function login(i) {
  const ctx = await browser.newContext({ viewport: { width: +(process.env.W ?? 1024), height: +(process.env.H ?? 576) }, locale: 'pt-BR' });
  const page = await ctx.newPage();
  page.setDefaultTimeout(300000);
  await page.addInitScript(() => { try { localStorage.setItem('erevia.settings', JSON.stringify({ quality: 'baixa', qualityPicked: true })); } catch {} });
  const user = `d${i}${tag}`;
  await page.request.post(`${BASE}/api/auth/dev`, { data: { username: user } });
  const c = (await (await page.request.get(`${BASE}/api/creation`)).json()).classes[cls];
  await page.request.post(`${BASE}/api/characters`, { data: { name: `${cls.slice(0, 3)}${i}${tag}`.slice(0, 10) + 'x', class: cls, look: { weapon: c.weapons[0].id, offhand: c.offhands?.[0]?.id ?? '', emblem: c.emblem, skin: 0, hair: 0, outfit: 0, head: '', hideHat: false } } });
  execSync(`docker exec erevia-postgres-1 psql -U erevia -d ${DB} -c "UPDATE accounts SET gm=true"`, { stdio: 'ignore' });
  await page.goto(`${BASE}/?e2e=1&${query}`, { waitUntil: 'domcontentloaded', timeout: 240000 });
  await page.waitForTimeout(3000);
  await page.getByRole('button', { name: 'Entrar no mundo' }).click();
  await page.waitForFunction(() => window.__ereviaE2E?.me?.(), null, { timeout: 240000 });
  await page.waitForTimeout(2500);
  if (await page.getByText('O que há de novo').isVisible().catch(() => false)) { await page.keyboard.press('Escape'); await page.waitForTimeout(500); }
  return page;
}
const mover = await login(0);
const watcher = await login(1);
const say = async (p, t) => { await p.keyboard.press('Enter'); await p.keyboard.type(t); await p.keyboard.press('Enter'); };
await say(mover, '/tp 2 7');
await say(watcher, '/tp 4 7');
await mover.waitForTimeout(1500);
const moverId = await mover.evaluate(() => window.__ereviaE2E.localId());
const sample = (page, id, ms) => page.evaluate(([id, ms]) => new Promise((done) => {
  const rows = [];
  const t0 = performance.now();
  const tick = () => {
    const t = performance.now() - t0;
    const p = window.__ereviaE2E.motionProbe(id);
    if (p) rows.push({ t: Math.round(t), ...p });
    if (t < ms) requestAnimationFrame(tick); else done(rows);
  };
  tick();
}), [id, ms]);
const total = 9000;
const pa = sample(mover, moverId, total);
const pb = sample(watcher, moverId, total);
// anda 3 s, para 2 s, anda 2 s (WASD): a câmera gira só pelo mouse, então a direção fica a mesma
await mover.keyboard.down('w'); await mover.waitForTimeout(3000); await mover.keyboard.up('w');
await mover.waitForTimeout(2000);
await mover.keyboard.down('w'); await mover.waitForTimeout(2000); await mover.keyboard.up('w');
const [self, remote] = await Promise.all([pa, pb]);
writeFileSync(out, JSON.stringify({ query, cls, self, remote }));
console.log('ok', self.length, remote.length);
await browser.close();
