import { expect, test } from "./fixtures";

/**
 * Confere o próprio guarda de erros (tests/fixtures.ts): sem ele, o roteiro passaria verde
 * com a página quebrada. Roda só a tela de login (rápido, sem o mundo 3D).
 */

test("a tela de login abre sem erro (o 401 de /api/me é a única exceção)", async ({ page, problems }) => {
  const me401 = page.waitForResponse((r) => r.url().endsWith("/api/me") && r.status() === 401);
  await page.goto("/");
  await me401;
  await expect(page.getByLabel("Usuário de desenvolvimento")).toBeVisible({ timeout: 30_000 });
  expect(problems.list).toEqual([]);
});

test("um erro de JavaScript ou um console.error na página reprova o teste", async ({ page, problems }) => {
  await page.goto("/");
  await expect(page.getByLabel("Usuário de desenvolvimento")).toBeVisible({ timeout: 30_000 });

  await page.evaluate(() => setTimeout(() => { throw new Error("erro de propósito"); }));
  await expect.poll(() => problems.list.join("\n")).toContain("pageerror: Error: erro de propósito");
  expect(() => problems.check(), "o guarda reprova um pageerror").toThrow();

  problems.list.length = 0;
  await page.evaluate(() => console.error("console.error de propósito"));
  await expect.poll(() => problems.list.join("\n")).toContain("console.error: console.error de propósito");
  expect(() => problems.check(), "o guarda reprova um console.error").toThrow();

  // Limpa para o próprio guarda (no fim do teste) não reprovar este teste.
  problems.list.length = 0;
});
