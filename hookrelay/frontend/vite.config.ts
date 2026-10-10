import { defineConfig } from "vite";
export default defineConfig({
  server: {
    port: 5174,
    proxy: {
      "/api": "http://localhost:8090",
      "/healthz": "http://localhost:8090",
    },
  },
});
