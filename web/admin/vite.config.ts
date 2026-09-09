import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// Same-origin pilot deployment (see docs/admin-web-plan.md section 5.2):
// the SPA is served at "/" and "/api/*" goes to the Go backend. In dev,
// Vite's own server plays the "/" role and proxies "/api" to the backend
// listening on :8080 (configs/config.yaml) so no CORS config is ever
// needed, in dev or in the pilot deployment.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: false,
      },
    },
  },
})
