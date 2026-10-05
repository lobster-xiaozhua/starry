import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@shared/api': path.resolve(__dirname, '../packages/shared-api/src/index.ts'),
    },
  },
  server: {
    allowedHosts: ['.monkeycode-ai.online'],
    port: 5173,
    proxy: {
      '/api/agent/chat': {
        target: 'http://127.0.0.1:3001',
        changeOrigin: true,
      },
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
