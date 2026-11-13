import type { Locator, Page } from "@playwright/test";
import { expect, test } from "./fixtures";

/**
 * Roteiro de um jogador novo, do login à volta depois de recarregar a página, contra o
 * build de produção do cliente. Tudo o que o jogador faz passa pelo mouse e pelo teclado.
 *
 * `?e2e=1` liga em client/src/main.ts um gancho só de leitura (window.__ereviaE2E) que diz
 * onde estão na tela a Mercadora e os monstros, para o teste saber onde clicar.
 */

interface E2EHook {
  me(): { x: number; z: number } | null;
  screenPos(id: number): { x: number; y: number } | null;
  entities(kind: "npc" | "monster"): { id: number; name: string; x: number; z: number }[];
  targetId(): number;
}
declare global {
  interface Window {
    __ereviaE2E?: E2EHook;
  }
}

const URL = "/?e2e=1";
const stamp = Date.now().toString(36);
const USER = `e2e${stamp}`;
const NAME = `Heroi${stamp}`;

/** Posição do jogador como o HUD mostra (X e Y embaixo do minimapa). */
async function hudCoords(page: Page): Promise<{ x: number; z: number }> {
  const v = page.locator(".mm-coords .mm-cv");
  return { x: Number(await v.nth(0).textContent()), z: Number(await v.nth(1).textContent()) };
}

/** Quanto da barra (vida, mana) está cheia, de 0 a 100. */
async function barPct(bar: Locator): Promise<number> {
  return bar.evaluate((e) => parseFloat((e as HTMLElement).style.getPropertyValue("--v")) || 0);
}

/** Onde a entidade aparece no canvas agora (pixels da página), ou null se está fora da tela. */
async function onScreen(page: Page, id: number): Promise<{ x: number; y: number } | null> {
  const box = (await page.locator("#game").boundingBox())!;
  const p = await page.evaluate((id) => window.__ereviaE2E!.screenPos(id), id);
  // Margem para não clicar embaixo do HUD (minimapa, barra de atalhos).
  if (!p || p.x < 80 || p.x > box.width - 260 || p.y < 80 || p.y > box.height - 110) return null;
  return { x: box.x + p.x, y: box.y + p.y };
}

/** Quantidade mostrada no atalho da tecla n (1–9) da barra. */
function hotbarQty(page: Page, n: number): Locator {
  return page.locator(".hotbar .hb-slot").nth(n - 1).locator(".qty");
}

/** Clica no canvas do jogo onde a entidade aparece (espera ela entrar na tela). */
async function clickEntity(page: Page, id: number, timeout = 15_000): Promise<void> {
  await expect.poll(() => onScreen(page, id), { message: `entidade ${id} visível na tela`, timeout }).not.toBeNull();
  const p = (await onScreen(page, id))!;
  await page.mouse.click(p.x, p.y);
}

/** Posição exata do jogador (o HUD arredonda). */
async function myPos(page: Page): Promise<{ x: number; z: number }> {
  return (await page.evaluate(() => window.__ereviaE2E!.me()))!;
}

/** Abre o mapa (M), clica no ponto (x, z) do mundo e fecha. O mapa abre centrado no jogador. */
async function walkByMap(page: Page, x: number, z: number): Promise<void> {
  await page.keyboard.press("m");
  const map = page.locator('[data-win="map"] .wmap-canvas');
  await expect(map).toBeVisible();
  // Espera o mapa assentar (ele se desenha no requestAnimationFrame).
  await page.waitForTimeout(300);
  const box = (await map.boundingBox())!;
  const me = await myPos(page);
  const scale = 3.2; // px por metro na abertura (client/src/ui/windows/worldmap.ts)
  const px = box.x + box.width / 2 + (x - me.x) * scale;
  const py = box.y + box.height / 2 - (z - me.z) * scale;
  expect(px > box.x && px < box.x + box.width && py > box.y && py < box.y + box.height, "destino cabe no mapa").toBe(true);
  await page.mouse.click(px, py);
  await page.keyboard.press("m");
  await expect(page.locator('[data-win="map"]')).toHaveCount(0);
}

/** O canvas 3D tem imagem de verdade (não é uma tela de uma cor só). */
async function expectSceneDrawn(page: Page): Promise<void> {
  const png = await page.locator("#game").screenshot();
  const colors = await page.evaluate(async (b64) => {
    const img = new Image();
    img.src = `data:image/png;base64,${b64}`;
    await img.decode();
    const c = document.createElement("canvas");
    c.width = 64;
    c.height = 36;
    const ctx = c.getContext("2d")!;
    ctx.drawImage(img, 0, 0, c.width, c.height);
    const d = ctx.getImageData(0, 0, c.width, c.height).data;
    const set = new Set<number>();
    for (let i = 0; i < d.length; i += 4) set.add(((d[i] >> 3) << 10) | ((d[i + 1] >> 3) << 5) | (d[i + 2] >> 3));
    return set.size;
  }, png.toString("base64"));
  expect(colors, "cores distintas no canvas 3D").toBeGreaterThan(40);
}

/** test.step que também imprime quanto tempo o passo levou (para acompanhar no log do CI). */
async function step(name: string, fn: () => Promise<void>): Promise<void> {
  const t0 = Date.now();
  await test.step(name, fn);
  console.log(`  ✓ ${name} (${((Date.now() - t0) / 1000).toFixed(1)} s)`);
}

/** Espera o HUD do personagem no mundo. */
async function expectInWorld(page: Page): Promise<void> {
  await expect(page.locator(".hud.hud-screen")).toBeVisible({ timeout: 60_000 });
  await expect(page.locator(".me-name")).toContainText(NAME);
  await expect.poll(() => page.evaluate(() => window.__ereviaE2E?.me() ?? null), { timeout: 30_000 }).not.toBeNull();
}

test("jogador novo: entrar, andar, janelas, loja, combate, poção e voltar depois de recarregar", async ({ page, problems }) => {
  // O cliente avisa no console quando o cenário (árvores, cidade, montanhas) termina de carregar.
  let sceneryLoaded = false;
  page.on("console", (m) => {
    if (m.text().startsWith("cenário:")) sceneryLoaded = true;
  });

  await step("login de desenvolvimento pela tela inicial", async () => {
    await page.goto(URL);
    const user = page.getByLabel("Usuário de desenvolvimento");
    await expect(user).toBeVisible({ timeout: 30_000 });
    await user.fill(USER);
    await page.locator(".dev-login").getByRole("button", { name: "Entrar" }).click();
    // Conta nova: a tela de personagens abre na criação.
    await expect(page.locator(".char-screen")).toBeVisible();
    await expect(page.getByRole("tab", { name: "Criar novo" })).toBeVisible();
    problems.check();
  });

  await step("criar o personagem pela API e entrar no mundo", async () => {
    const creation = await (await page.request.get("/api/creation")).json();
    const knight = creation.classes.knight;
    const res = await page.request.post("/api/characters", {
      data: {
        name: NAME,
        class: "knight",
        look: {
          weapon: knight.weapons[0].id,
          offhand: knight.offhands[0].id,
          emblem: knight.emblem,
          skin: 0,
          hair: 0,
          outfit: 0,
          head: "",
          hideHat: false,
        },
      },
    });
    expect(res.status(), await res.text()).toBeLessThan(300);

    await page.reload();
    await expect(page.locator(".roster-slot", { hasText: NAME })).toBeVisible({ timeout: 30_000 });
    await page.getByRole("button", { name: "Entrar no mundo" }).click();
    await expectInWorld(page);
    await expect.poll(() => sceneryLoaded, { message: "cenário carregado", timeout: 120_000 }).toBe(true);
    await expectSceneDrawn(page);
    problems.check();
  });

  await step("andar com um clique no chão", async () => {
    const before = await hudCoords(page);
    const box = (await page.locator("#game").boundingBox())!;
    await page.mouse.click(box.x + box.width / 2 + 160, box.y + box.height / 2 + 90);
    await expect
      .poll(async () => {
        const now = await hudCoords(page);
        return Math.hypot(now.x - before.x, now.z - before.z);
      }, { timeout: 20_000 })
      .toBeGreaterThanOrEqual(2);
    problems.check();
  });

  await step("abrir e fechar inventário (I), personagem (C), skills (K) e mapa (M)", async () => {
    for (const [key, id] of [["i", "inventory"], ["c", "character"], ["k", "skills"], ["m", "map"]] as const) {
      const win = page.locator(`[data-win="${id}"]`);
      await page.keyboard.press(key);
      await expect(win, `janela ${id} abre com ${key}`).toBeVisible();
      await expect(win.locator(".gwin-body > *").first()).toBeVisible();
      await page.keyboard.press(key);
      await expect(win, `janela ${id} fecha com ${key}`).toHaveCount(0);
    }
  });

  await step("abrir e fechar configurações (Esc)", async () => {
    const win = page.locator('[data-win="settings"]');
    await page.keyboard.press("Escape");
    await expect(win).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(win).toHaveCount(0);
    problems.check();
  });

  await step("comprar uma Poção de Mana Menor da Mercadora Lívia", async () => {
    const livia = (await page.evaluate(() => window.__ereviaE2E!.entities("npc"))).find((n) => n.name.includes("Lívia"));
    expect(livia, "Mercadora Lívia na praça").toBeTruthy();
    // Fora da tela (o clique no chão pode ter levado para longe): vai até perto dela pelo mapa.
    if (!(await onScreen(page, livia!.id))) await walkByMap(page, livia!.x - 2, livia!.z + 2);
    await clickEntity(page, livia!.id, 30_000);
    const shop = page.locator(".shop-panel[data-drop=\"sell\"]");
    await expect(shop).toBeVisible({ timeout: 20_000 });
    const gold = shop.locator(".shop-gold-value");
    await expect(gold).toHaveText("50");
    await shop.locator(".shop-row", { hasText: "Poção de Mana Menor" }).getByRole("button", { name: "Comprar" }).click();
    await expect(gold).toHaveText("25");
    // A loja abre junto com o inventário: a poção está na bolsa e na barra (tecla 4).
    const inv = page.locator('[data-win="inventory"]');
    await expect(inv.locator('.bag-slot[aria-label*="Poção de Mana Menor"]')).toHaveCount(1);
    await expect(hotbarQty(page, 4)).toHaveText("1");
    await page.keyboard.press("Escape"); // fecha a loja
    await expect(shop).toBeHidden();
    await page.keyboard.press("Escape"); // fecha o inventário
    await expect(inv).toHaveCount(0);
    await page.keyboard.press("Escape"); // larga a Lívia como alvo
    await expect(page.locator(".hud-target")).toBeHidden();
    problems.check();
  });

  await step("sair da cidade até os Arredores pelo mapa", async () => {
    // Estrada leste, perto da Pedra de Teleporte dos Arredores (60, 5); os Lacaios ficam a 46–74 m do centro.
    await walkByMap(page, 52, 4);
    await expect.poll(async () => (await hudCoords(page)).x, { timeout: 45_000 }).toBeGreaterThan(46);
    await expect(page.locator(".mm-zone")).toContainText("Arredores");
    problems.check();
  });

  await step("selecionar um monstro (Tab) e usar a skill da tecla 1", async () => {
    const target = page.locator(".hud-target");
    for (let tries = 1; ; tries++) {
      // Tab: o monstro vivo mais perto, até 30 m.
      await page.keyboard.press("Tab");
      const picked = await target.waitFor({ state: "visible", timeout: 2000 }).then(() => true, () => false);
      if (picked) break;
      expect(tries, "achar um monstro a até 30 m").toBeLessThan(4);
      // Nenhum por perto: anda (pelo mapa) na direção do mais perto que o cliente conhece.
      const me = await myPos(page);
      const mons = await page.evaluate(() => window.__ereviaE2E!.entities("monster"));
      expect(mons.length, "monstros conhecidos pelo cliente").toBeGreaterThan(0);
      mons.sort((a, b) => Math.hypot(a.x - me.x, a.z - me.z) - Math.hypot(b.x - me.x, b.z - me.z));
      await walkByMap(page, me.x + (mons[0].x - me.x) * 0.7, me.z + (mons[0].z - me.z) * 0.7);
      await page.waitForTimeout(4000);
    }
    // Tecla 1: Golpe de Escudo no alvo (anda até o alcance e golpeia; gasta mana).
    await page.keyboard.press("1");
    await expect.poll(() => barPct(page.locator(".hud-me .pbar.mp")), { message: "a skill gastou mana", timeout: 30_000 }).toBeLessThan(100);
    problems.check();
  });

  await step("tomar a poção de mana comprada (tecla 4)", async () => {
    const mp = page.locator(".hud-me .pbar.mp");
    const before = await barPct(mp);
    expect(before).toBeLessThan(100);
    await page.keyboard.press("4");
    await expect(hotbarQty(page, 4)).toHaveText("0");
    await expect.poll(() => barPct(mp), { timeout: 10_000 }).toBeGreaterThan(before);
    problems.check();
  });

  await step("atacar o monstro com um clique", async () => {
    const target = page.locator(".hud-target");
    const targetHp = target.locator(".pbar.hp");
    const monsterId = await page.evaluate(() => window.__ereviaE2E!.targetId());
    // O Golpe de Escudo já pode ter derrubado; senão, o clique no monstro ataca até ele cair.
    if (monsterId && (await target.isVisible())) await clickEntity(page, monsterId);
    await expect
      .poll(async () => !(await target.isVisible()) || (await target.evaluate((e) => e.classList.contains("dead"))) || (await barPct(targetHp)) < 100, {
        message: "o monstro perdeu vida",
        timeout: 30_000,
      })
      .toBe(true);
    problems.check();
  });

  await step("recarregar a página e voltar ao mesmo personagem", async () => {
    const where = await hudCoords(page);
    await page.reload();
    // Volta pela tela de personagens (ou direto ao mundo, se a página guardou a reconexão).
    const play = page.getByRole("button", { name: "Entrar no mundo" });
    await expect(play.or(page.locator(".hud.hud-screen"))).toBeVisible({ timeout: 60_000 });
    if (await play.isVisible()) {
      await expect(page.locator(".roster-slot", { hasText: NAME })).toHaveAttribute("aria-pressed", "true");
      await play.click();
    }
    await expectInWorld(page);
    // Mesmo personagem, no mesmo lugar (fora da cidade), não um novo na praça.
    const now = await hudCoords(page);
    expect(Math.hypot(now.x - where.x, now.z - where.z), `posição depois de recarregar ${JSON.stringify(now)} vs ${JSON.stringify(where)}`).toBeLessThan(15);
    await page.keyboard.press("i");
    await expect(page.locator('[data-win="inventory"] .inv-gold')).toContainText("25");
    problems.check();
  });
});
