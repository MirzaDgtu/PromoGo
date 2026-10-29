import { execFileSync } from 'node:child_process'

import { ADMIN, API_URL, APP_CONTAINER, OIDC_ISSUER } from './env.ts'

async function reachable(url: string): Promise<boolean> {
  try {
    const res = await fetch(url)
    return res.ok
  } catch {
    return false
  }
}

// globalSetup fails fast with a readable message when the stack isn't up,
// then makes sure the E2E platform admin exists. bootstrap-admin is
// idempotent (creates or promotes), so every run can call it.
export default async function globalSetup() {
  const missing: string[] = []
  if (!(await reachable(`${API_URL}/readyz`))) missing.push(`PromoGo API (${API_URL}/readyz)`)
  if (!(await reachable(`${OIDC_ISSUER}/.well-known/openid-configuration`))) missing.push(`mock OIDC (${OIDC_ISSUER})`)
  if (missing.length > 0) {
    throw new Error(`E2E stack is not running: ${missing.join(', ')}. See web/admin/e2e/README.md.`)
  }

  execFileSync(
    'docker',
    [
      'exec', APP_CONTAINER, 'promogo', 'bootstrap-admin',
      '--subject', ADMIN.subject, '--email', ADMIN.email, '--name', ADMIN.name, '--org-name', ADMIN.homeOrg,
      // execFileSync passes arguments as-is, so the spaces need no quoting.
    ],
    { stdio: 'pipe' },
  )
}
