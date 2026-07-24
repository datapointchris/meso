import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    // @ is shorthand for /src, e.g. import Foo from '@/components/Foo.vue'
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  css: {
    // Use the modern Dart Sass compiler API (sass-embedded), not the deprecated
    // legacy JS API.
    preprocessorOptions: { scss: { api: 'modern-compiler' } },
  },
  server: {
    port: 3001,
    // Dev server proxies /api to the local Go API so the SPA talks to it the
    // same relative way it does in production (where Caddy owns the split).
    proxy: {
      '/api': { target: 'http://localhost:8088', changeOrigin: true },
    },
  },
})
