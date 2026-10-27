import { Navigate, Route, Routes } from 'react-router-dom'

import { CallbackPage } from '../auth/CallbackPage'
import { LoginPage } from '../auth/LoginPage'
import { AppShell } from '../components/AppShell'
import { NotFoundState } from '../components/states'
import { ApiKeysPage } from '../pages/apiKeys/ApiKeysPage'
import { AuditPage } from '../pages/audit/AuditPage'
import { ClientsPage } from '../pages/clients/ClientsPage'
import { LoyaltyConfigPage } from '../pages/loyalty/LoyaltyConfigPage'
import { OrganizationsPage } from '../pages/OrganizationsPage'
import { OverviewPage } from '../pages/overview/OverviewPage'
import { StaffPage } from '../pages/staff/StaffPage'
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
        <Route index element={<OverviewPage />} />
        <Route path="/organizations" element={<OrganizationsPage />} />
        <Route path="/staff" element={<StaffPage />} />
        <Route path="/loyalty-config" element={<LoyaltyConfigPage />} />
        <Route path="/clients" element={<ClientsPage />} />
        <Route path="/api-keys" element={<ApiKeysPage />} />
        <Route path="/audit" element={<AuditPage />} />
      </Route>

      <Route path="/404" element={<NotFoundState />} />
      <Route path="*" element={<Navigate to="/404" replace />} />
    </Routes>
  )
}
