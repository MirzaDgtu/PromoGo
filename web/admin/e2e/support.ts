import AxeBuilder from '@axe-core/playwright'
import { expect, type Page } from '@playwright/test'

export interface Person {
  subject: string
  email: string
  name: string
}

// login runs the real OIDC flow: the app's "Войти через SSO" redirects to
// mock-oauth2-server, whose form takes any username as the sub claim plus
// optional extra claims, then back to /auth/callback.
export async function login(page: Page, who: Person) {
  await page.goto('/login')
  await page.getByRole('button', { name: 'Войти через SSO' }).click()

  await page.locator('input[name="username"]').fill(who.subject)
  const claims = page.locator('textarea[name="claims"]')
  if ((await claims.count()) > 0) {
    await claims.fill(JSON.stringify({ email: who.email, name: who.name }))
  }
  await page.locator('[type="submit"]').click()

  await page.waitForURL((url) => !url.pathname.startsWith('/auth/callback') && !url.pathname.startsWith('/login'))
}

// captureStaffToken records the PromoGo staff bearer token the app sends
// with its API calls. The app keeps it only in memory, so this is the one
// way a test can replay a request outside the UI — to prove the backend
// itself refuses what the UI merely hides.
export function captureStaffToken(page: Page): () => string | null {
  let token: string | null = null
  page.on('request', (req) => {
    const auth = req.headers()['authorization']
    if (req.url().includes('/api/v1/') && auth?.startsWith('Bearer ')) token = auth.slice('Bearer '.length)
  })
  return () => token
}

export async function openSection(page: Page, label: string) {
  await page.getByRole('navigation', { name: 'Основная навигация' }).getByRole('link', { name: label }).click()
}

export async function selectScope(page: Page, org: string, store: string | null) {
  await page.getByLabel('Организация', { exact: true }).selectOption({ label: org })
  await page.getByLabel('Магазин', { exact: true }).selectOption({ label: store ?? 'Все магазины организации' })
}

// expectAccessible fails on any WCAG 2.1 A/AA violation axe can detect on
// the current screen, listing each rule and the elements that break it.
export async function expectAccessible(page: Page, screen: string) {
  // Let loading skeletons and entrance animations settle, so axe checks
  // the real screen and not a half-faded frame (fades lower contrast).
  await expect(page.locator('.skeleton')).toHaveCount(0)
  await page.evaluate(() =>
    Promise.all(
      document
        .getAnimations()
        .filter((a) => a.effect?.getTiming().iterations !== Infinity)
        .map((a) => a.finished.catch(() => undefined)),
    ),
  )
  const results = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze()
  const report = results.violations.map(
    (v) =>
      `${v.id} (${v.impact ?? 'n/a'}): ${v.help}\n` +
      v.nodes.map((n) => `    ${n.target.join(' ')}: ${(n.failureSummary ?? '').replace(/\s+/g, ' ')}`).join('\n'),
  )
  expect(report, `accessibility violations on «${screen}»`).toEqual([])
}
