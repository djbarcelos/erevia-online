// Prints do laboratório (client/lab.html, `vite dev`): as famílias de monstro (tunnel#135).
//   node tools/art/familias-monstro/lab.mjs <pasta-de-saida> [porta]
// Por modelo: tira de poses (parado, andar, golpe, dano, morte) de 3/4 e de lado; depois as quatro famílias juntas,
// e as variantes de cor do Brejo (tint). Usa o mesmo CharacterModels do jogo (SwiftShader).
// env: PW_E2E (pasta e2e com o @playwright/test)
import { createRequire } from 'node:module';
import { mkdirSync } from 'node:fs';
const require = createRequire(process.env.PW_E2E ?? 'C:/Users/DanielTeofilo/erevia-wt/remaster-padrao/e2e/');
const { chromium } = require('@playwright/test');
const [out, port = '5197'] = process.argv.slice(2);
mkdirSync(out, { recursive: true });
const browser = await chromium.launch({ args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--ignore-gpu-blocklist'] });
const page = await browser.newPage({ viewport: { width: 1280, height: 520 } });
page.on('pageerror', (e) => console.log('[pageerror]', String(e).slice(0, 300)));
page.on('console', (m) => { if (m.type() === 'error') console.log('[console.error]', m.text().slice(0, 200)); });
await page.goto(`http://localhost:${port}/lab.html`, { waitUntil: 'domcontentloaded' });
await page.waitForFunction(() => window.lab, null, { timeout: 120000 });
const poses = [['idle', 0.2], ['walk', 0.3], ['run', 0.3], ['attack', 0.8], ['hit', 0.3], ['death', 1]];
const size = { wolf: { gap: 2.3, h: 0.5, r: 7 }, spider: { gap: 1.5, h: 0.25, r: 5 }, slime: { gap: 1.3, h: 0.4, r: 4.5 }, frog: { gap: 1.3, h: 0.55, r: 5 } };
const shot = async (name, spec) => {
  await page.evaluate((s) => window.lab.show(s), { settle: 2500, ...spec });
  await page.screenshot({ path: `${out}/${name}.png` });
  console.log(name);
};
for (const [m, s] of Object.entries(size)) {
  for (const [cam, yaw] of [['three', 0.9], ['side', 1.571]]) {
    // a câmera fica de frente (o eixo x aparece espelhado: a lista vai ao contrário) e o modelo gira para mostrar o perfil
    await shot(`${m}-poses-${cam}`, { cam: 'front', items: poses.map(([role, t]) => ({ model: m, role, t, yaw })).reverse(), gap: s.gap, height: s.h, radius: s.gap * poses.length * 0.62 });
  }
}
for (const cam of ['three', 'front', 'back']) {
  await shot(`familias-${cam}`, { cam, items: ['wolf', 'spider', 'slime', 'frog'].map((m) => ({ model: m, role: 'idle', t: 0.2, yaw: cam === 'front' ? 0.7 : 0 })), gap: 2.4, height: 0.5, radius: 6.5 });
}
// variantes de cor (Brejo e Bosque Gris): o mesmo modelo, outra cor por Entity.tint
const tints = {
  slime: ['#ffffff', '#8aa05a', '#6f8a4a', '#a8794f', '#7fb3c8'],
  frog: ['#ffffff', '#c28a4a', '#6f8fa8', '#a86f6f', '#7a6fa8'],
  wolf: ['#ffffff', '#7a6a5a', '#d8d8e0', '#b08a5a', '#5a4a3a'],
  spider: ['#ffffff', '#b09a6a', '#7a8fb0', '#a86a5a', '#6a5a7a'],
};
for (const [m, list] of Object.entries(tints)) {
  await shot(`${m}-variantes`, { cam: 'front', items: list.map((tint) => ({ model: m, role: 'idle', t: 0.2, tint, yaw: 0.6 })).reverse(), gap: size[m].gap, height: size[m].h, radius: size[m].gap * list.length * 0.62 });
}
await browser.close();
