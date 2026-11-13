import { defineConfig } from "vite";

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
      // Jogo (index.html), o protótipo visual das telas (prototipo.html) e as páginas legais.
      input: { main: "index.html", prototipo: "prototipo.html", privacidade: "privacidade.html", termos: "termos.html" },
    },
  },
});
