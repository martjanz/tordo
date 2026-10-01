import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig } from 'vite'

// The build goes where the Go server embeds it (public/.gitkeep keeps the
// folder in git). In development, `npm run dev` serves the app and sends
// /api and /stream to the Go server, started with:
// go run ./cmd/tordo -dir .run -port 4747 -no-browser
export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: '../internal/server/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:4747',
      '/stream': 'http://127.0.0.1:4747',
    },
  },
  test: {
    include: ['src/**/*.test.js'],
  },
})
