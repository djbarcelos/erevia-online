import { defineConfig, loadEnv } from "vite";

/** Arquivos do portal de administração: o servidor só entrega /assets/admin/ para GM. */
const ADMIN_DIR = "assets/admin/";
const isAdmin = (names: readonly (string | undefined)[]) => names.some((n) => n?.startsWith("admin"));

export default defineConfig(({ mode }) => ({
  define: {
    // Versão do cliente nos relatórios de erro: o commit do build, o mesmo que o servidor
    // usa no /bug. A Railway passa RAILWAY_GIT_COMMIT_SHA ao build do Dockerfile (ARG);
    // localmente, `make docker` passa o `git rev-parse HEAD`. Sem ela, "dev".
    __APP_VERSION__: JSON.stringify(loadEnv(mode, ".", "RAILWAY_").RAILWAY_GIT_COMMIT_SHA?.trim() || "dev"),
  },
  server: {
    port: 5173,
    proxy: {
      // O navegador fala com o Vite, e o Vite repassa o WebSocket ao servidor Go.
      "/ws": { target: "ws://localhost:8080", ws: true },
      "/api": { target: "http://localhost:8080" },
    },
  },
  build: {
    target: "es2022",
    chunkSizeWarningLimit: 8000,
    // Sourcemaps para traduzir as stacks dos relatórios de erro (scripts/resolve-stack.mjs),
    // sem o comentário //# sourceMappingURL no .js. Os .map nunca são servidos: a imagem
    // Docker os tira da pasta pública e o servidor recusa *.map.
    sourcemap: "hidden",
    rollupOptions: {
      // Jogo (index.html), o protótipo visual das telas (prototipo.html), as páginas legais,
      // as novidades e o portal de administração (admin.html, só para GM).
      input: {
        main: "index.html",
        prototipo: "prototipo.html",
        privacidade: "privacidade.html",
        termos: "termos.html",
        novidades: "novidades.html",
        admin: "admin.html",
      },
      output: {
        entryFileNames: (chunk) => (chunk.name === "admin" ? `${ADMIN_DIR}[name]-[hash].js` : "assets/[name]-[hash].js"),
        assetFileNames: (asset) =>
          isAdmin(asset.names ?? []) ? `${ADMIN_DIR}[name]-[hash][extname]` : "assets/[name]-[hash][extname]",
      },
    },
  },
}));
