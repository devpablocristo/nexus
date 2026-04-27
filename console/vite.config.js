import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const governanceTarget = process.env.GOVERNANCE_PROXY_TARGET || 'http://governance:8080'
const governanceAPIKey =
  process.env.GOVERNANCE_PROXY_API_KEY ||
  process.env.GOVERNANCE_ADMIN_API_KEY ||
  'governance-admin-dev-key'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/v1': {
        target: governanceTarget,
        changeOrigin: true,
        headers: {
          'X-API-Key': governanceAPIKey,
        },
      },
    },
  },
})
