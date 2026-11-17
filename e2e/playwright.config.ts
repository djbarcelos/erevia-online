import { defineConfig, devices } from "@playwright/test";

/**
 * Testes de navegador do Erevia. Rodam contra um servidor já no ar (com DEV_LOGIN=true e o
 * cliente compilado em STATIC_DIR), apontado por E2E_BASE_URL. Ver o README da raiz.
 *
 * - E2E_BASE_URL: padrão http://localhost:8080.
 * - E2E_CHANNEL=chrome: usa o Chrome instalado na máquina em vez do Chromium do Playwright.
 */
const CI = !!process.env.CI;

export default defineConfig({
  testDir: "./tests",
  // Um teste só (o roteiro depende da ordem); o servidor é compartilhado.
  workers: 1,
  fullyParallel: false,
  forbidOnly: CI,
  // O mundo é vivo (monstros andam, reaparecem): no CI, uma segunda chance antes de
  // ficar vermelho. Um erro de JavaScript falha nas duas tentativas do mesmo jeito.
  retries: CI ? 1 : 0,
  timeout: 5 * 60_000,
  expect: { timeout: 15_000 },
  reporter: CI ? [["github"], ["list"], ["html", { open: "never" }]] : [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:8080",
    ...devices["Desktop Chrome"],
    viewport: { width: 1280, height: 720 },
    channel: process.env.E2E_CHANNEL || undefined,
    locale: "pt-BR",
    screenshot: "only-on-failure",
    // Vídeo custa CPU (e o WebGL já é por software): no CI, só na segunda tentativa, a de quem falhou.
    video: CI ? "on-first-retry" : "retain-on-failure",
    trace: "retain-on-failure",
    actionTimeout: 30_000,
    launchOptions: {
      args: [
        // WebGL por software (SwiftShader, via ANGLE): o runner do CI não tem GPU.
        "--use-angle=swiftshader",
        "--enable-unsafe-swiftshader",
        "--ignore-gpu-blocklist",
        // Canvas 2D na CPU. Acelerado, ele também passa pelo SwiftShader, e as texturas que o
        // cliente pinta (chão, mapa) mais o getImageData travavam a página por mais de 30 s ao
        // entrar no mundo: sem Ping, o servidor derrubava a conexão.
        "--disable-accelerated-2d-canvas",
      ],
    },
  },
});
