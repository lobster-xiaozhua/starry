import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import { VitePWA } from 'vite-plugin-pwa'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig({
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      includeAssets: ['favicon.svg'],
      manifest: {
        name: 'Starry 工作台',
        short_name: 'Starry',
        description: '统一工作台：用户系统、AI 对话、Agent 工作模式与云笔记',
        theme_color: '#0f172a',
        background_color: '#f8fafc',
        display: 'standalone',
        start_url: '/',
        icons: [
          { src: 'icon-192.png', sizes: '192x192', type: 'image/png' },
          { src: 'icon-512.png', sizes: '512x512', type: 'image/png' },
        ],
      },
      workbox: {
        globPatterns: ['**/*.{js,css,html,svg,png,woff2}'],
        navigateFallback: '/index.html',
        runtimeCaching: [
          {
            urlPattern: /\/api\/.*/,
            handler: 'NetworkFirst',
            options: {
              cacheName: 'api-cache',
              networkTimeoutSeconds: 5,
              expiration: { maxEntries: 60, maxAgeSeconds: 30 },
            },
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      '@shared/api': path.resolve(__dirname, '../packages/shared-api/src/index.ts'),
    },
  },
  build: {
    // 主包曾达 1.1MB 并触发构建告警。按「变更频率」拆分为稳定的 vendor 块，
    // 让浏览器长期缓存依赖，应用代码更新时不失效。
    rollupOptions: {
      output: {
        manualChunks: {
          'react-vendor': ['react', 'react-dom', 'react-router-dom'],
          'antd-vendor': ['antd'],
          'editor-vendor': ['marked', 'dompurify'],
        },
      },
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
      // 长程任务：创建/运行/进度流均在 Agent 服务，必须优先于通用 /api 规则
      '/api/agent/tasks': {
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
