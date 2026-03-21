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

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: [
      { find: /^@devpablocristo\/core-http$/, replacement: path.join(coreHttpPath, 'index.ts') },
      { find: /^@devpablocristo\/core-http\/(.+)$/, replacement: `${coreHttpPath}/$1` },
      { find: /^@devpablocristo\/core-browser$/, replacement: path.join(coreBrowserPath, 'index.ts') },
      { find: /^@devpablocristo\/core-browser\/(.+)$/, replacement: `${coreBrowserPath}/$1` },
    ],
  },
  server: {
    fs: {
      allow: [path.resolve(__dirname), coreHttpPath, coreBrowserPath],
    },
  },
})
