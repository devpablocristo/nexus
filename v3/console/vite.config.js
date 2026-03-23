import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

function resolveCoreModule(relativePath) {
  const candidates = [
    path.resolve(__dirname, `.deps/core/${relativePath}/src`),
    path.resolve(__dirname, `../../../core/${relativePath}/src`),
  ]
  return candidates.find((candidate) => fs.existsSync(candidate)) ?? candidates[0]
}

const coreHttpPath = resolveCoreModule('http/ts')
const coreBrowserPath = resolveCoreModule('browser/ts')
const coreAuthnPath = resolveCoreModule('authn/ts')

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: [
      { find: /^@devpablocristo\/core-http$/, replacement: path.join(coreHttpPath, 'index.ts') },
      { find: /^@devpablocristo\/core-http\/(.+)$/, replacement: `${coreHttpPath}/$1` },
      { find: /^@devpablocristo\/core-browser$/, replacement: path.join(coreBrowserPath, 'index.ts') },
      { find: /^@devpablocristo\/core-browser\/(.+)$/, replacement: `${coreBrowserPath}/$1` },
      { find: /^@devpablocristo\/core-authn$/, replacement: path.join(coreAuthnPath, 'index.ts') },
      { find: /^@devpablocristo\/core-authn\/(.+)$/, replacement: `${coreAuthnPath}/$1` },
    ],
  },
  server: {
    proxy: {
      '/v1': { target: 'http://review:8080', changeOrigin: true },
      '/companion': {
        target: 'http://companion:8080',
        changeOrigin: true,
        rewrite: (p) => p.replace(/^\/companion/, '') || '/',
      },
    },
    fs: {
      allow: [path.resolve(__dirname), coreHttpPath, coreBrowserPath, coreAuthnPath],
    },
  },
})
