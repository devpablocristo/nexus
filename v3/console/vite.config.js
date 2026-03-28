import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/v1': { target: 'http://review:8080', changeOrigin: true },
      '/companion': {
        target: 'http://companion:8080',
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/companion/, '') || '/',
      },
    },
  },
})
