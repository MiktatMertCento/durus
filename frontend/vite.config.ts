import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    // Keep phase SVGs as real files so <img> CSS animations stay stable.
    assetsInlineLimit: (filePath) => (filePath.endsWith('.svg') ? false : undefined),
  },
  server: {
    port: 5173,
    proxy: {
      '/ws': {
        target: 'ws://127.0.0.1:8080',
        ws: true,
      },
      '/api': {
        target: 'http://127.0.0.1:8080',
      },
    },
  },
})
