import { fileURLToPath, URL } from "node:url"
import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"

const backend = process.env.API_PROXY_TARGET ?? "http://127.0.0.1:8080"

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    proxy: Object.fromEntries(
      ["/api", "/metrics", "/users/by-telegram", "/status", "/healthz", "/docs", "/db", "/grafana", "/prometheus", "/openapi.json", "/openapi-mocks.json"].map((route) => [route, { target: backend, changeOrigin: true }]),
    ),
  },
  build: { outDir: "dist", emptyOutDir: true },
})
