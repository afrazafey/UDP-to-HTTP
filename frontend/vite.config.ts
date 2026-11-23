import { defineConfig, type Plugin } from "vite";
import { readdir, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import react from "@vitejs/plugin-react";
import tailwind from "@tailwindcss/vite";

// Proxy rewriting is development-only: the daemon still verifies its own Host and Origin.
// Remove stale build assets but keep the tracked embed placeholder.
const cleanAssets: Plugin = {
  name: "clean-dashboard-assets",
  apply: "build",
  async buildStart() {
    const output = fileURLToPath(
      new URL("../internal/web/dist/", import.meta.url),
    );
    for (const entry of await readdir(output))
      if (entry !== ".keep")
        await rm(`${output}/${entry}`, { recursive: true, force: true });
  },
};
export default defineConfig({
  plugins: [cleanAssets, react(), tailwind()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
        configure(proxy) {
          proxy.on("proxyReq", (request) => {
            request.removeHeader("origin");
            request.removeHeader("sec-fetch-site");
          });
        },
      },
      "/health": { target: "http://127.0.0.1:8080", changeOrigin: true },
    },
  },
  build: { outDir: "../internal/web/dist", emptyOutDir: false },
});
