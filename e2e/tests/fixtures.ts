import { test as base, expect, type ConsoleMessage, type Page } from "@playwright/test";

/**
 * Erros que podem aparecer sem quebrar o teste. A lista é curta de propósito: qualquer
 * outro `pageerror` ou `console.error` deixa o teste vermelho.
 */
const ALLOWED_ERRORS: { why: string; match: (m: ConsoleMessage) => boolean }[] = [
  {
    // boot() pergunta GET /api/me para saber se já há sessão; antes do login a resposta é 401
    // e o Chrome registra o recurso que falhou.
    why: "401 de /api/me antes do login",
    match: (m) => m.text().includes("status of 401") && pathOf(m.location().url) === "/api/me",
  },
];

/** Avisos que, para o jogo, são falhas: o mundo carregou pela metade. */
const FATAL_WARNINGS = [/modelos 3D indispon/i, /cen[aá]rio indispon/i];

function pathOf(url: string): string {
  try {
    return new URL(url).pathname;
  } catch {
    return "";
  }
}

export interface Problems {
  list: string[];
  /** Falha agora se já houve algum erro (para o teste parar no passo que quebrou). */
  check(): void;
}

function watch(page: Page): Problems {
  const list: string[] = [];
  page.on("pageerror", (err) => list.push(`pageerror: ${err.stack ?? err.message}`));
  page.on("console", (m) => {
    const where = m.location().url ? ` (${m.location().url}:${m.location().lineNumber})` : "";
    if (m.type() === "error") {
      if (!ALLOWED_ERRORS.some((a) => a.match(m))) list.push(`console.error: ${m.text()}${where}`);
    } else if (m.type() === "warning" && FATAL_WARNINGS.some((re) => re.test(m.text()))) {
      list.push(`console.warn: ${m.text()}${where}`);
    }
  });
  page.on("crash", () => list.push("a aba do navegador travou (crash)"));
  return {
    list,
    check: () => expect(list, "erros de JavaScript no navegador").toEqual([]),
  };
}

export const test = base.extend<{ problems: Problems }>({
  problems: [
    async ({ page }, use) => {
      const p = watch(page);
      await use(p);
      p.check();
    },
    { auto: true },
  ],
});

export { expect };
