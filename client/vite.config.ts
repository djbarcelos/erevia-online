import { defineConfig } from "vite";

/** Arquivos do portal de administração: o servidor só entrega /assets/admin/ para GM. */
const ADMIN_DIR = "assets/admin/";
const isAdmin = (names: readonly (string | undefined)[]) => names.some((n) => n?.startsWith("admin"));

export default defineConfig({
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
    rollupOptions: {
      // Jogo (index.html), o protótipo visual das telas (prototipo.html), as páginas legais
      // e o portal de administração (admin.html, só para GM).
      input: {
        main: "index.html",
        prototipo: "prototipo.html",
        privacidade: "privacidade.html",
        termos: "termos.html",
        admin: "admin.html",
      },
      output: {
        entryFileNames: (chunk) => (chunk.name === "admin" ? `${ADMIN_DIR}[name]-[hash].js` : "assets/[name]-[hash].js"),
        assetFileNames: (asset) =>
          isAdmin(asset.names ?? []) ? `${ADMIN_DIR}[name]-[hash][extname]` : "assets/[name]-[hash][extname]",
      },
    },
  },
});
