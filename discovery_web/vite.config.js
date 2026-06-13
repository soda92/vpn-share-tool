import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    vue(),
    vueDevTools(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
  server: {
    proxy: {
      '/login': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/logout': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/check-auth': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/instances': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/cluster-proxies': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/change-password': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/tagged-urls': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/latest-version': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/trigger-update-remote': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/create-proxy': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/toggle-debug-proxy': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/toggle-captcha-proxy': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/update-proxy-settings': { target: 'https://localhost:8080', changeOrigin: true, secure: false },
      '/logs': { target: 'https://localhost:8080', changeOrigin: true, secure: false, ws: true },
    }
  }
})
