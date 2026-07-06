import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

// Wails serves the built assets from frontend/dist. Emit a relative base so
// the bundled asset URLs resolve under the Wails asset server.
export default defineConfig({
  plugins: [svelte()],
  base: "./",
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
