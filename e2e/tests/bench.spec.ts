import { appendFileSync, existsSync, readFileSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { compare, markdown, snapshot, type BenchResult, type Snapshot } from "./bench-compare";
import { expect, test } from "./fixtures";

/**
 * Contadores da medição de FPS (?bench=1) contra a linha de base versionada
 * (e2e/bench-baseline.json): draw calls e malhas do cenário por região e hora, o censo das
 * malhas do cenário e as malhas de cada entidade. O que comparar e com que tolerância está em
 * bench-compare.ts. Nunca FPS nem ms: no WebGL por software e numa máquina compartilhada,
 * o tempo não diz nada.
 *
 * `npm run bench:baseline` (ou BENCH_UPDATE=1) grava a base em vez de comparar.
 * A tabela antes e depois vai para o resumo do job no CI ($GITHUB_STEP_SUMMARY).
 */

const BASELINE = fileURLToPath(new URL("../bench-baseline.json", import.meta.url));
const stamp = Date.now().toString(36);

declare global {
  interface Window {
    __ereviaBench?: { done: boolean; progress: string; result?: unknown };
  }
}

test("medição de FPS: contadores do cenário iguais à linha de base", async ({ page, problems }, info) => {
  // Entrar no mundo + a volta da câmera por todas as regiões, de dia e de noite.
  test.setTimeout(8 * 60_000);
  // A volta toda roda a 2–3 FPS no SwiftShader: o cliente relata low_fps, como deve.
  problems.allow("low_fps no WebGL por software", /^relatório de erro do cliente: client_invariant: low_fps /);
  const update = !!process.env.BENCH_UPDATE || info.config.updateSnapshots === "all" || info.config.updateSnapshots === "changed";

  // Qualidade gráfica Baixa, como no roteiro do jogador (jogo.spec.ts).
  await page.addInitScript(() => {
    try {
      if (!localStorage.getItem("erevia.settings")) localStorage.setItem("erevia.settings", JSON.stringify({ quality: "baixa", qualityPicked: true }));
    } catch {
      // sem storage, fica o padrão do aparelho
    }
  });

  await test.step("conta e personagem novos pela API", async () => {
    const login = await page.request.post("/api/auth/dev", { data: { username: `bench${stamp}` } });
    expect(login.status(), await login.text()).toBeLessThan(300);
    const creation = await (await page.request.get("/api/creation")).json();
    const knight = creation.classes.knight;
    const res = await page.request.post("/api/characters", {
      data: {
        name: `Bench${stamp}`,
        class: "knight",
        look: { weapon: knight.weapons[0].id, offhand: knight.offhands[0].id, emblem: knight.emblem, skin: 0, hair: 0, outfit: 0, head: "", hideHat: false },
      },
    });
    expect(res.status(), await res.text()).toBeLessThan(300);
  });

  // ?bench=contadores: a amostra de FPS de cada ponto fica curta (só os contadores importam).
  const t0 = Date.now();
  await test.step("entrar no mundo com ?bench=contadores", async () => {
    await page.goto("/?bench=contadores");
    await page.getByRole("button", { name: "Entrar no mundo" }).click();
    await expect.poll(() => page.evaluate(() => window.__ereviaBench?.progress ?? null), { message: "a medição começou", timeout: 180_000 }).not.toBeNull();
  });
  await test.step("a câmera percorre as regiões, de dia e de noite", async () => {
    await expect
      .poll(() => page.evaluate(() => window.__ereviaBench?.done ?? false), { message: "a medição terminou", timeout: 5 * 60_000, intervals: [2000] })
      .toBe(true);
  });
  const seconds = (Date.now() - t0) / 1000;
  const result = (await page.evaluate(() => window.__ereviaBench!.result)) as BenchResult;
  problems.check();

  const now = snapshot(result);
  const base: Snapshot | null = existsSync(BASELINE) ? JSON.parse(readFileSync(BASELINE, "utf8")) : null;
  if (update) writeFileSync(BASELINE, JSON.stringify(now, null, 2) + "\n");
  const rep = compare(base ?? { regioes: {}, censo: {}, entidades: {} }, now);
  const title = `Medição de FPS: contadores do cenário${info.retry ? ` (tentativa ${info.retry + 1})` : ""}`;
  const md = markdown(rep, { title, seconds, updated: update });

  writeFileSync(info.outputPath("bench-result.json"), JSON.stringify(result, null, 1));
  writeFileSync(info.outputPath("bench-report.md"), md);
  // A base que esta rodada gravaria: no CI (Linux), copie do artefato "bench" para e2e/bench-baseline.json.
  writeFileSync(info.outputPath("bench-baseline.json"), JSON.stringify(now, null, 2) + "\n");
  await info.attach("bench-report.md", { path: info.outputPath("bench-report.md"), contentType: "text/markdown" });
  await info.attach("bench-result.json", { path: info.outputPath("bench-result.json"), contentType: "application/json" });
  if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY, md + "\n");
  console.log(md);
  console.log(result.table);

  if (update) return;
  expect(base, "e2e/bench-baseline.json existe (crie com npm run bench:baseline)").not.toBeNull();
  expect(
    rep.failed.map((r) => `${r.key} ${r.metric}: ${r.base ?? "–"} → ${r.now}`),
    "números do cenário que pioraram além da tolerância (ver bench-compare.ts; se for de propósito, npm run bench:baseline)",
  ).toEqual([]);
});
