// Tira os prints da interface (login, personagens, criação, carregamento, HUD e uma janela),
// em 1280x720 e em celular (390x844). Uso (dentro de e2e/, com o servidor no ar):
//   E2E_BASE_URL=http://localhost:8087 node interface-shots.mjs antes|depois [pasta]
// Os PNG vão para docs/arte/interface/<prefixo>-<tela>-<pc|celular>.png.
import { chromium, devices } from "@playwright/test";
import { mkdirSync } from "node:fs";

const base = process.env.E2E_BASE_URL ?? "http://localhost:8080";
const prefix = process.argv[2] ?? "depois";
const out = process.argv[3] ?? "../docs/arte/interface";
mkdirSync(out, { recursive: true });
const stamp = Date.now().toString(36);

const args = ["--use-angle=swiftshader", "--enable-unsafe-swiftshader", "--ignore-gpu-blocklist", "--disable-accelerated-2d-canvas"];
const profiles = {
  pc: { viewport: { width: 1280, height: 720 } },
  celular: { ...devices["Pixel 7"], viewport: { width: 390, height: 844 }, deviceScaleFactor: 2 },
};

/** Espera o mundo estar de pé (a conexão pode cair e voltar quando a máquina está carregada). */
const inWorld = (page) => page.waitForFunction(() => document.getElementById("overlay")?.hidden === true && !!document.querySelector(".hud-me"), null, { timeout: 240_000 });

const settle = (page, ms = 1500) => page.waitForTimeout(ms);

async function run(kind) {
  const browser = await chromium.launch({ args, channel: process.env.E2E_CHANNEL || undefined });
  const ctx = await browser.newContext({ ...profiles[kind], locale: "pt-BR", baseURL: base });
  const page = await ctx.newPage();
  let scenery = false;
  page.on("console", (m) => {
    if (m.text().startsWith("cenário:")) scenery = true;
  });
  page.on("pageerror", (e) => console.log("pageerror", e.message));
  // Marca a janela "O que há de novo" como vista: ela abriria sozinha por cima do HUD.
  const latest = (await (await page.request.get("/novidades.json")).json()).latest;
  await page.addInitScript((v) => {
    try {
      localStorage.setItem("erevia.novidades.vista", v);
    } catch {}
  }, latest);
  await page.addInitScript(() => {
    try {
      localStorage.setItem("erevia.settings", JSON.stringify({ quality: "baixa", qualityPicked: true }));
    } catch {}
  });
  const shot = async (name) => {
    await page.screenshot({ path: `${out}/${prefix}-${name}-${kind}.png` });
    console.log("ok", `${prefix}-${name}-${kind}`);
  };

  await page.goto("/?e2e=1");
  await page.locator(".login-card").waitFor();
  await settle(page, 2500);
  await shot("login");

  const user = `ui${kind}${stamp}`;
  await page.locator(".dev-login input").fill(user);
  await page.locator(".dev-login button[type=submit]").click();
  // Sem personagem, a tela abre direto na criação.
  await page.locator(".char-screen").waitFor();
  await settle(page, 6000);
  await shot("criacao");
  const classBtns = page.locator(".class-card2");
  if (await classBtns.count()) {
    await classBtns.nth(2).click();
    await settle(page, 2500);
    await shot("criacao-mago");
  }
  if (kind === "celular") {
    await page.locator(".mobile-tabs .tab").nth(1).click().catch(() => {});
    await settle(page, 800);
    await shot("criacao-aparencia");
  }

  // Personagem pela API (mesma sessão), para a lista.
  const creation = await (await page.request.get("/api/creation")).json();
  const k = creation.classes.knight;
  const r = await page.request.post("/api/characters", {
    data: { name: `Heroi${kind === "pc" ? "A" : "B"}${stamp}`.slice(0, 16), class: "knight", look: { weapon: k.weapons[0].id, offhand: k.offhands[0].id, emblem: k.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false } },
  });
  if (r.status() >= 300) console.log("criar personagem", r.status(), await r.text());
  await page.reload();
  await page.locator(".roster-slot").first().waitFor();
  await settle(page, 6000);
  await shot("personagens");

  await page.getByRole("button", { name: "Entrar no mundo" }).click();
  await page.locator(".entry-loading").waitFor();
  await settle(page, 400);
  await shot("carregamento");

  if (process.env.ENTRY) {
    await browser.close();
    return;
  }
  await page.locator(".hud-me").waitFor({ timeout: 180_000 });
  await page.waitForFunction(() => document.getElementById("overlay")?.hidden === true, null, { timeout: 180_000 });
  // Espera o cenário (cidade, árvores) terminar de carregar para o print não sair com o mundo pela metade.
  for (let i = 0; i < 60 && !scenery; i++) await settle(page, 1000);
  await settle(page, 2000);
  await inWorld(page);
  // A janela "O que há de novo" abre sozinha na primeira entrada: espera ela e fecha.
  await page.locator(".gwin").first().waitFor({ timeout: 10_000 }).catch(() => {});
  await page.getByRole("button", { name: "Fechar", exact: true }).last().click({ timeout: 3000 }).catch(() => {});
  await page.locator(".gwin").first().waitFor({ state: "detached", timeout: 3000 }).catch(() => {});
  await settle(page, 1500);
  await inWorld(page);
  await shot("hud");

  // alvo: clica numa entidade conhecida pelo gancho de teste, se houver
  const spot = await page.evaluate(() => {
    const h = window.__ereviaE2E;
    if (!h) return null;
    const list = [...h.entities("npc"), ...h.entities("monster")];
    const c = document.getElementById("game").getBoundingClientRect();
    for (const e of list) {
      const p = h.screenPos(e.id);
      if (p && p.x > 40 && p.y > 160 && p.x < innerWidth - 40 && p.y < innerHeight - 200) return { x: c.left + p.x, y: c.top + p.y };
    }
    return null;
  });
  if (spot) {
    await inWorld(page);
    await page.mouse.click(spot.x, spot.y);
    await settle(page, 1200);
    await shot("hud-alvo");
  }

  await page.keyboard.press("Escape");
  for (let i = 0; i < 3 && (await page.locator(".gwin").count()); i++) {
    await page.locator(".gwin .x-btn").first().click({ timeout: 2000 }).catch(() => {});
    await settle(page, 500);
  }
  if (kind === "pc") {
    await page.keyboard.press("i");
  } else {
    const burger = page.locator(".menu-burger");
    if (await burger.count()) {
      await burger.click();
      await settle(page, 600);
      await shot("menu");
      await page.locator(".mm-item", { hasText: "Inventário" }).first().click().catch(() => {});
    }
  }
  await settle(page, 1200);
  await inWorld(page);
  await shot("janela-inventario");
  await browser.close();
}

for (const kind of (process.env.KINDS ?? "pc,celular").split(",")) await run(kind);
