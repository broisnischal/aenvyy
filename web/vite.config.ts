import { defineConfig } from 'vite'
import { devtools } from '@tanstack/devtools-vite'

import { tanstackStart } from '@tanstack/react-start/plugin/vite'

import viteReact from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// envvar's web UI is a zero-knowledge SPA: all secret data is fetched from the
// Go API at /v1 and en/decrypted in the browser, so we build a static client
// bundle (SPA mode) that the Go binary embeds and serves as a single artifact.
const config = defineConfig({
  resolve: { tsconfigPaths: true },
  plugins: [
    devtools(),
    tailwindcss(),
    tanstackStart({ spa: { enabled: true } }),
    viteReact(),
  ],
  server: {
    port: 3000,
    // During local dev, proxy API calls to the Go server (`envvar server`).
    proxy: {
      '/v1': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
    },
  },
})

export default config
