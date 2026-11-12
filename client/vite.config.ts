import { defineConfig } from "vite";

export default defineConfig({
  server: {
    port: 5173,
    proxy: {
      // O navegador fala com o Vite, e o Vite repassa o WebSocket ao servidor Go.
      "/ws": { target: "ws://localhost:8080", ws: true },
    },
  },
  build: {
    target: "es2022",
    chunkSizeWarningLimit: 8000,
  },
});
