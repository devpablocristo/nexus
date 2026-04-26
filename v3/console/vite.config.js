import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const nexusTarget = process.env.NEXUS_PROXY_TARGET || 'http://nexus:8080'
const nexusAPIKey =
  process.env.NEXUS_PROXY_API_KEY ||
  process.env.NEXUS_ADMIN_API_KEY ||
  'nexus-admin-dev-key'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/v1': {
        target: nexusTarget,
        changeOrigin: true,
        headers: {
          'X-API-Key': nexusAPIKey,
        },
      },
    },
  },
})
