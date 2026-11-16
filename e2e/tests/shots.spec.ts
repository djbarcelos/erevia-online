import { mkdirSync, writeFileSync } from "node:fs";
import { expect, test } from "./fixtures";

/**
 * Prints de regressão visual: cenas fixas do cenário (praça de dia e de noite, Arredores e Vale)
 * com a câmera parada, o relógio do dia fixado e as entidades escondidas (`?shots=1`, ver
 * setupShots em client/src/ui/bench.ts). Os PNG ficam em test-results/**\/shot-*.png e o CI
 * publica como o artefato "prints", para olhar a olho quando uma mudança de arte ou de
 * iluminação aparecer num PR.
 *
 * Não compara com uma base por padrão: o SwiftShader não é estável o bastante entre execuções
 * (ver "Prints de regressão visual" no README). `SHOTS_COMPARE=1` liga a comparação com as bases
 * de e2e/tests/shots.spec.ts-snapshots, a tolerância de SHOTS_TOLERANCE (fração de pixels
 * diferentes, padrão 0,02), e `--update-snapshots` grava as bases.
 */

const stamp = Date.now().toString(36);
const COMPARE = !!process.env.SHOTS_COMPARE;
const TOLERANCE = Number(process.env.SHOTS_TOLERANCE ?? 0.02);

declare global {
  interface Window {
    __ereviaShots?: { ready: boolean; names: string[]; show(name: string): Promise<{ x: number; z: number; phase: number }> };
  }
}

test("prints do cenário: praça de dia e de noite, Arredores e Vale", async ({ page, problems }, info) => {
  test.setTimeout(6 * 60_000);
  // Cada cena leva dezenas de segundos no WebGL por software: o cliente relata low_fps, como deve.
  problems.allow("low_fps no WebGL por software", /^relatório de erro do cliente: client_invariant: low_fps /);

  // Qualidade gráfica Baixa, como no roteiro do jogador (jogo.spec.ts).
  await page.addInitScript(() => {
    try {
      if (!localStorage.getItem("erevia.settings")) localStorage.setItem("erevia.settings", JSON.stringify({ quality: "baixa", qualityPicked: true }));
    } catch {
      // sem storage, fica o padrão do aparelho
    }
  });

  await test.step("conta e personagem novos pela API", async () => {
    const login = await page.request.post("/api/auth/dev", { data: { username: `shots${stamp}` } });
    expect(login.status(), await login.text()).toBeLessThan(300);
    const creation = await (await page.request.get("/api/creation")).json();
    const knight = creation.classes.knight;
    const res = await page.request.post("/api/characters", {
      data: {
        name: `Prints${stamp}`,
        class: "knight",
        look: { weapon: knight.weapons[0].id, offhand: knight.offhands[0].id, emblem: knight.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false },
      },
    });
    expect(res.status(), await res.text()).toBeLessThan(300);
  });

  await test.step("entrar no mundo com ?shots=1", async () => {
    await page.goto("/?shots=1");
    await page.getByRole("button", { name: "Entrar no mundo" }).click();
    await expect.poll(() => page.evaluate(() => window.__ereviaShots?.ready ?? false), { message: "o modo de prints ligou", timeout: 180_000 }).toBe(true);
  });
  // Só o canvas do jogo: sem HUD, chat nem janelas por cima.
  await page.evaluate(() => {
    for (const id of ["hud", "overlay"]) document.getElementById(id)?.style.setProperty("display", "none", "important");
  });

  const names = await page.evaluate(() => window.__ereviaShots!.names);
  expect(names.length).toBeGreaterThanOrEqual(4);
  mkdirSync(info.outputDir, { recursive: true });
  const meta: Record<string, unknown> = {};
  for (const name of names) {
    await test.step(`cena ${name}`, async () => {
      meta[name] = await page.evaluate((n) => window.__ereviaShots!.show(n), name);
      const png = await page.locator("#game").screenshot({ animations: "disabled" });
      writeFileSync(info.outputPath(`shot-${name}.png`), png);
      await info.attach(`shot-${name}.png`, { body: png, contentType: "image/png" });
      if (COMPARE) {
        expect(png, `print ${name} igual à base`).toMatchSnapshot(`${name}.png`, { maxDiffPixelRatio: TOLERANCE });
      }
    });
  }
  writeFileSync(info.outputPath("shot-cenas.json"), JSON.stringify(meta, null, 2));
  problems.check();
});
