import { test as base, expect, type ConsoleMessage, type Page } from "@playwright/test";

/**
 * Guarda de erros, ligado em todo teste: junta os pageerror, console.error, avisos graves e
 * relatórios de erro que o cliente manda ao servidor, e reprova o teste no fim se houver algum.
 *
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
  /**
   * Deixa passar, só neste teste, o problema que casar com `re` (`why` diz por quê). Ex.: a
   * medição de FPS passa minutos a 2–3 FPS no WebGL por software, e o cliente relata `low_fps`.
   */
  allow(why: string, re: RegExp): void;
}

function watch(page: Page): Problems {
  const list: string[] = [];
  const allowed: RegExp[] = [];
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
  // O cliente manda ao servidor os erros que capturou (client/src/net/errors.ts): nenhum pode sair.
  page.on("request", (r) => {
    if (pathOf(r.url()) !== "/api/client-errors") return;
    // Um lote pode trazer vários erros: um item por erro, para allow() pegar só o permitido.
    const body = r.postData() ?? "";
    let errors: { kind?: string; message?: string; stack?: string }[] | null = null;
    try {
      errors = (JSON.parse(body) as { errors?: typeof errors }).errors ?? null;
    } catch {
      // corpo que não é JSON: vai inteiro
    }
    if (!errors?.length) list.push(`relatório de erro do cliente: ${body.slice(0, 600)}`);
    else for (const e of errors) list.push(`relatório de erro do cliente: ${e.kind}: ${e.message} ${(e.stack ?? "").slice(0, 400)}`);
  });
  return {
    list,
    check: () => expect(list.filter((p) => !allowed.some((re) => re.test(p))), "erros de JavaScript no navegador").toEqual([]),
    allow: (_why, re) => void allowed.push(re),
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
