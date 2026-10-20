/// <reference types="vitest/config" />
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

const backend = process.env.TRUCO_BACKEND ?? 'http://localhost:8080'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  build: {
    // CSP do servidor: sem inline de áudio/fontes (data: só vale para img-src).
    assetsInlineLimit: (file: string) => (/\.(ogg|mp3|woff2?)$/.test(file) ? false : undefined),
  },
  server: {
    port: 5173,
    strictPort: true,
    // Sem changeOrigin: o backend confere Origin contra APP_BASE_URL (http://localhost:5173).
    proxy: {
      '/api': { target: backend },
      '/ws': { target: backend, ws: true },
    },
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['src/test/setup.ts'],
    include: ['src/**/*.test.ts'],
    restoreMocks: true,
  },
})
