import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// In dev, the Go API is on :8080. Proxy the relevant prefixes so the SPA can
// use plain `/api/...` paths in both dev and prod builds.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
    },
  },
});
