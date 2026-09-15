import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// Same-origin pilot deployment (see docs/admin-web-plan.md section 5.2):
// the SPA is served at "/" and "/api/*" goes to the Go backend. In dev,
// Vite's own server plays the "/" role and proxies "/api" to the backend.
// The backend listens on :8080 inside its container, but the local
// docker-compose maps that to host port 8090 (127.0.0.1:8090->8080) since
// :8080 on this machine is already taken by an unrelated project.
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8090',
        changeOrigin: false,
      },
    },
  },
})
