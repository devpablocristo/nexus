import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const reviewTarget = process.env.NEXUS_REVIEW_PROXY_TARGET || 'http://review:8080'
const companionTarget = process.env.NEXUS_COMPANION_PROXY_TARGET || 'http://companion:8080'
const reviewAPIKey =
  process.env.NEXUS_REVIEW_PROXY_API_KEY ||
  process.env.NEXUS_REVIEW_ADMIN_API_KEY ||
  'nexus-review-admin-dev-key'
const companionAPIKey =
  process.env.NEXUS_COMPANION_PROXY_API_KEY ||
  process.env.NEXUS_COMPANION_ADMIN_API_KEY ||
  'nexus-companion-admin-dev-key'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/v1': {
        target: reviewTarget,
        changeOrigin: true,
        headers: {
          'X-API-Key': reviewAPIKey,
        },
      },
      '/companion': {
        target: companionTarget,
        changeOrigin: true,
        headers: {
          'X-API-Key': companionAPIKey,
        },
        rewrite: (p) => p.replace(/^\/companion/, '') || '/',
      },
    },
  },
})
