import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@web-ui": resolve(__dirname, "../web/src/components/ui"),
      "@": resolve(__dirname, "../web/src")
    }
  },
  server: {
    fs: {
      // Allow importing workspace packages from ../../packages/*
      allow: ["..", "../.."]
    }
  }
});
