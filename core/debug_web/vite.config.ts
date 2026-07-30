import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  base: '/debug/',
  plugins: [
    vue(),
    vueDevTools(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
  test: {
    environment: 'happy-dom',
    root: fileURLToPath(new URL('./', import.meta.url)),
  },
  server: {
    proxy: {
      '/api/debug': 'http://localhost:10081',
      '/api/debug/ws': {
        target: 'ws://localhost:10081',
        ws: true,
      },
    }
  }
})
