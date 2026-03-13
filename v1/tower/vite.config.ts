import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), 'VITE_');
  return {
  plugins: [react()],
  define: {
    'import.meta.env.VITE_NEXUS_SCOPES': JSON.stringify(env.VITE_NEXUS_SCOPES || process.env.VITE_NEXUS_SCOPES || ''),
    'import.meta.env.VITE_NEXUS_API_KEY': JSON.stringify(env.VITE_NEXUS_API_KEY || process.env.VITE_NEXUS_API_KEY || ''),
    'import.meta.env.VITE_NEXUS_CORE_URL': JSON.stringify(env.VITE_NEXUS_CORE_URL || process.env.VITE_NEXUS_CORE_URL || ''),
    'import.meta.env.VITE_NEXUS_SAAS_URL': JSON.stringify(env.VITE_NEXUS_SAAS_URL || process.env.VITE_NEXUS_SAAS_URL || ''),
    'import.meta.env.VITE_NEXUS_GRAFANA_URL': JSON.stringify(env.VITE_NEXUS_GRAFANA_URL || process.env.VITE_NEXUS_GRAFANA_URL || ''),
    'import.meta.env.VITE_CLERK_PUBLISHABLE_KEY': JSON.stringify(env.VITE_CLERK_PUBLISHABLE_KEY || process.env.VITE_CLERK_PUBLISHABLE_KEY || ''),
    'import.meta.env.VITE_ALLOW_API_KEY_FALLBACK': JSON.stringify(env.VITE_ALLOW_API_KEY_FALLBACK || process.env.VITE_ALLOW_API_KEY_FALLBACK || ''),
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-react': ['react', 'react-dom', 'react-router-dom', '@tanstack/react-query'],
          'vendor-charts': ['recharts'],
        },
      },
    },
  },
  server: {
    host: '0.0.0.0',
    port: 5173,
  },
  preview: {
    host: '0.0.0.0',
    port: 4173,
  },
  };
});
