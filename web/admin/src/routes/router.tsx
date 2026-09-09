import { Navigate, Route, Routes } from 'react-router-dom'

import { CallbackPage } from '../auth/CallbackPage'
import { LoginPage } from '../auth/LoginPage'
import { AppShell } from '../components/AppShell'
import { NotFoundState } from '../components/states'
import { strings } from '../i18n/strings'
import { ComingSoonPage } from '../pages/ComingSoonPage'
import { RequireAuth } from './RequireAuth'

export function AppRouter() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/auth/callback" element={<CallbackPage />} />

      <Route
        element={
          <RequireAuth>
            <AppShell />
          </RequireAuth>
        }
      >
        <Route index element={<ComingSoonPage title={strings.nav.dashboard} />} />
        <Route path="/organizations" element={<ComingSoonPage title={strings.nav.organizations} />} />
        <Route path="/staff" element={<ComingSoonPage title={strings.nav.staff} />} />
        <Route path="/loyalty-config" element={<ComingSoonPage title={strings.nav.loyaltyConfig} />} />
        <Route path="/clients" element={<ComingSoonPage title={strings.nav.clients} />} />
        <Route path="/api-keys" element={<ComingSoonPage title={strings.nav.apiKeys} />} />
        <Route path="/audit" element={<ComingSoonPage title={strings.nav.audit} />} />
      </Route>

      <Route path="/404" element={<NotFoundState />} />
      <Route path="*" element={<Navigate to="/404" replace />} />
    </Routes>
  )
}
