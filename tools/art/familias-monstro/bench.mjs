// Mede n monstros animados na tela (lab.html): draw calls, tempo de animação e de quadro (SwiftShader: só a comparação vale).
//   node tools/art/familias-monstro/bench.mjs [porta]
import { createRequire } from 'node:module';
const require = createRequire(process.env.PW_E2E ?? 'C:/Users/DanielTeofilo/erevia-wt/remaster-padrao/e2e/');
const { chromium } = require('@playwright/test');
const port = process.argv[2] ?? '5197';
const browser = await chromium.launch({ args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist'] });
const page = await browser.newPage({ viewport: { width: 960, height: 540 } });
page.on('pageerror', (e) => console.log('[pageerror]', String(e).slice(0, 200)));
await page.goto(`http://localhost:${port}/lab.html`, { waitUntil: 'domcontentloaded' });
await page.waitForFunction(() => window.lab, null, { timeout: 120000 });
const runs = (process.env.RUNS ?? 'wolf:1,wolf:20,spider:20,slime:20,frog:20,skeleton_minion:20,skeleton_warrior:20').split(',').map((r) => { const [m, n] = r.split(':'); return [m, Number(n)]; });
for (const [model, n] of runs) {
  const r = await page.evaluate(([m, k]) => window.lab.bench(m, k, 12), [model, n]);
  console.log(model, n, JSON.stringify(r));
}
await browser.close();
