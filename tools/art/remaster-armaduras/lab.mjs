// Prints do laboratório (client/lab.html, `vite dev`): armadura por raridade (tunnel#96).
//   node tools/art/remaster-armaduras/lab.mjs <pasta-de-saida> [porta]
// Gera, por classe, um print com as 5 raridades lado a lado (comum, bom, raro, épico, lendário) de frente, de costas e de 3/4,
// e um com as 4 classes em comum, rara e épica. Usa o mesmo `CharacterModels` do jogo (SwiftShader).
// env: PW_E2E (pasta e2e com o @playwright/test), CAM (front|back|three), ONLY (classe), STACK (raridade do conjunto inteiro)
import { createRequire } from 'node:module';
import { mkdirSync } from 'node:fs';
const require = createRequire(process.env.PW_E2E ?? 'C:/Users/DanielTeofilo/erevia-wt/perf-ci/e2e/');
const { chromium } = require('@playwright/test');
const [out, port = '5196'] = process.argv.slice(2);
mkdirSync(out, { recursive: true });
const browser = await chromium.launch({ args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist'] });
const page = await browser.newPage({ viewport: { width: 1280, height: 520 } });
page.on('pageerror', (e) => console.log('[pageerror]', String(e).slice(0, 300)));
page.on('console', (m) => { if (m.type() === 'error') console.log('[console.error]', m.text().slice(0, 200)); });
await page.goto(`http://localhost:${port}/lab.html?remaster=1`, { waitUntil: 'domcontentloaded' });
await page.waitForFunction(() => window.lab, null, { timeout: 120000 });
const sets = (g) => ({ chest: g, helm: g, cape: g, legs: g, hands: g, feet: g });
const classes = ['knight', 'barbarian', 'mage', 'rogue'];
const weapons = { knight: ['sword_1handed', 'shield_badge_color'], barbarian: ['axe_1handed', null], mage: ['staff', null], rogue: ['dagger', 'dagger'] };
const only = process.env.ONLY;
const cams = (process.env.CAM ?? 'front,back').split(',');
for (const cls of only ? [only] : classes) {
  for (const cam of cams) {
    const items = [1, 2, 3, 4, 5].map((g) => ({ model: cls, right: weapons[cls][0], left: weapons[cls][1], armor: sets(g) }));
    await page.evaluate((spec) => window.lab.show(spec), { cam, items, gap: 1.25, height: 0.95, radius: 4.4, settle: 6000 });
    await page.screenshot({ path: `${out}/${cls}-5-raridades-${cam}.png` });
    console.log(cls, cam);
  }
}
// de perto: épico e lendário de 3/4 e de costas, para ver a silhueta (ombreiras, elmo, capa)
for (const cls of only ? [only] : classes) {
  for (const cam of ['three', 'back']) {
    const items = [3, 4, 5].map((g) => ({ model: cls, right: weapons[cls][0], left: weapons[cls][1], armor: sets(g) }));
    await page.evaluate((spec) => window.lab.show(spec), { cam, items, gap: 1.35, height: 1.05, radius: 3.4, settle: 6000 });
    await page.screenshot({ path: `${out}/${cls}-de-perto-${cam}.png` });
    console.log(cls, 'de perto', cam);
  }
}
if (!only) {
  for (const cam of cams) {
    for (const g of [1, 3, 4]) {
      const items = classes.map((cls) => ({ model: cls, right: weapons[cls][0], left: weapons[cls][1], armor: sets(g) }));
      await page.evaluate((spec) => window.lab.show(spec), { cam, items, gap: 1.3, height: 0.95, radius: 4.2, settle: 6000 });
      await page.screenshot({ path: `${out}/4-classes-grau${g}-${cam}.png` });
    }
  }
}
await browser.close();
