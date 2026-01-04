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
      '/instances': { target: 'http://localhost:8080', changeOrigin: true },
      '/cluster-proxies': { target: 'http://localhost:8080', changeOrigin: true },
      '/tagged-urls': { target: 'http://localhost:8080', changeOrigin: true },
      '/latest-version': { target: 'http://localhost:8080', changeOrigin: true },
      '/trigger-update-remote': { target: 'http://localhost:8080', changeOrigin: true },
      '/create-proxy': { target: 'http://localhost:8080', changeOrigin: true },
      '/toggle-debug-proxy': { target: 'http://localhost:8080', changeOrigin: true },
      '/toggle-captcha-proxy': { target: 'http://localhost:8080', changeOrigin: true },
      '/update-proxy-settings': { target: 'http://localhost:8080', changeOrigin: true },
    }
  }
})
