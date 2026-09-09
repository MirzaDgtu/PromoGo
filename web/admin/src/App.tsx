import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useState } from 'react'
import { BrowserRouter } from 'react-router-dom'

import { AuthProvider, useAuth } from './auth/AuthProvider'
import { ErrorBoundary } from './components/ErrorBoundary'
import { AppRouter } from './routes/router'
import { OrgStoreProvider } from './session/OrgStoreContext'

function AppProviders({ children }: { children: React.ReactNode }) {
  const { status } = useAuth()
  return <OrgStoreProvider enabled={status === 'authenticated'}>{children}</OrgStoreProvider>
}

export function App() {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            // A 401 already triggers AuthProvider's controlled re-auth
            // (api/client.ts's onResponse hook); retrying here on top of
            // that would just duplicate the same silent-renewal flow.
            retry: (failureCount, error) => {
              const status = (error as { response?: Response })?.response?.status
              if (status === 401 || status === 403 || status === 404) return false
              return failureCount < 2
            },
          },
        },
      }),
  )

  return (
    <ErrorBoundary>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <AuthProvider queryClient={queryClient}>
            <AppProviders>
              <AppRouter />
            </AppProviders>
          </AuthProvider>
        </BrowserRouter>
      </QueryClientProvider>
    </ErrorBoundary>
  )
}
