import { expect, test, type Browser, type Page } from '@playwright/test'

import { ADMIN, API_URL } from './env.ts'
import { captureStaffToken, expectAccessible, login, openSection, selectScope, type Person } from './support.ts'

// The critical M1 scenarios from docs/admin-web-plan.md 8, run as one
// story: a platform admin onboards a new organization and store, sets up
// its loyalty program and register key, and gives a store manager access;
// the manager then sees only what the role allows, and the backend refuses
// the rest. Every run uses fresh names, so runs never collide with each
// other or with data a developer made by hand.
test.describe.configure({ mode: 'serial' })

const run = Date.now().toString(36)
const orgName = `E2E Сеть ${run}`
const storeName = `E2E Магазин ${run}`
const keyName = `E2E Касса ${run}`
const manager: Person = { subject: `e2e-manager-${run}`, email: `manager-${run}@promogo.test`, name: `Управляющий ${run}` }

let browser: Browser
let admin: Page

test.beforeAll(async ({ browser: b }) => {
  browser = b
  admin = await (await browser.newContext({ locale: 'ru-RU' })).newPage()
})

test.afterAll(async () => {
  await admin.context().close()
})

test('platform admin logs in and sees every section', async () => {
  await login(admin, ADMIN)

  const nav = admin.getByRole('navigation', { name: 'Основная навигация' })
  for (const section of ['Обзор', 'Организации и магазины', 'Сотрудники и доступ', 'Программа лояльности', 'Клиенты', 'API-ключи', 'Аудит']) {
    await expect(nav.getByRole('link', { name: section })).toBeVisible()
  }
  await expectAccessible(admin, 'Обзор')
})

test('onboards a new organization and its first store', async () => {
  await openSection(admin, 'Организации и магазины')
  await admin.getByRole('button', { name: '+ Организация' }).click()
  await admin.getByLabel('Название новой организации').fill(orgName)
  await admin.getByRole('button', { name: 'Создать', exact: true }).click()
  await expect(admin.getByRole('status').filter({ hasText: `Организация «${orgName}» создана и выбрана` })).toBeVisible()

  // A new organization opens straight into its first store form.
  const storeForm = admin.getByRole('form', { name: `Новый магазин в «${orgName}»` })
  await storeForm.getByLabel('Название').fill(storeName)
  await storeForm.getByLabel('Адрес').fill('г. Махачкала, ул. Тестовая, 1')
  await storeForm.getByRole('button', { name: 'Создать' }).click()
  await expect(admin.getByRole('status').filter({ hasText: `Магазин «${storeName}» создан` })).toBeVisible()

  await admin.getByRole('button', { name: `Сделать «${storeName}» текущим магазином` }).click()
  await expect(admin.getByLabel('Организация', { exact: true }).locator('option:checked')).toHaveText(orgName)
  await expect(admin.getByLabel('Магазин', { exact: true }).locator('option:checked')).toHaveText(storeName)
  await expectAccessible(admin, 'Организации и магазины')
})

test('switching organization and store changes what every screen shows', async () => {
  await openSection(admin, 'Обзор')
  await expect(admin.getByRole('heading', { level: 1, name: `Обзор — ${storeName}` })).toBeVisible()

  // Another organization: the store resets to "all stores" of that one.
  await admin.getByLabel('Организация', { exact: true }).selectOption({ label: ADMIN.homeOrg })
  await expect(admin.getByLabel('Магазин', { exact: true }).locator('option:checked')).toHaveText('Все магазины организации')
  await expect(admin.getByRole('heading', { level: 1, name: `Обзор — ${ADMIN.homeOrg}` })).toBeVisible()

  await selectScope(admin, orgName, null)
  await expect(admin.getByRole('heading', { level: 1, name: `Обзор — ${orgName}` })).toBeVisible()
  await expect(admin.getByRole('button', { name: `Открыть обзор магазина «${storeName}»` })).toBeVisible()
  await expectAccessible(admin, 'Обзор организации')

  await selectScope(admin, orgName, storeName)
  await expect(admin.getByRole('heading', { level: 1, name: `Обзор — ${storeName}` })).toBeVisible()
  await expect(admin.getByText('Не настроена: продажи записываются с 0 баллов, списание недоступно.')).toBeVisible()
  await expectAccessible(admin, 'Обзор магазина')
})

test('configures the loyalty program, changes it and rolls it back', async () => {
  await openSection(admin, 'Программа лояльности')
  await expect(admin.getByRole('heading', { level: 1, name: `Программа лояльности — ${storeName}` })).toBeVisible()
  await expect(admin.getByText('Программа лояльности ещё не настроена')).toBeVisible()
  await expectAccessible(admin, 'Программа лояльности (не настроена)')

  const accrual = admin.getByLabel('Начислять, % от суммы покупки')
  await accrual.fill('7')
  await admin.getByRole('button', { name: 'Сохранить и включить' }).click()
  await expect(admin.getByRole('status').filter({ hasText: 'Сохранено. Действует версия 1.' })).toBeVisible()

  await accrual.fill('10')
  await admin.getByRole('button', { name: 'Сохранить', exact: true }).click()
  await expect(admin.getByRole('status').filter({ hasText: 'Сохранено. Действует версия 2.' })).toBeVisible()
  await expect(accrual).toHaveValue('10')

  await admin.getByRole('button', { name: 'Вернуть версию 1' }).click()
  await admin.getByRole('alertdialog', { name: 'Вернуть версию 1' }).getByRole('button', { name: 'Вернуть версию' }).click()
  await expect(admin.getByRole('status').filter({ hasText: 'Значения версии 1 возвращены как версия 3.' })).toBeVisible()
  await expect(accrual).toHaveValue('7')
  await expect(admin.getByRole('row').filter({ hasText: 'Действует' })).toContainText('7 %')
  await expectAccessible(admin, 'Программа лояльности')
})

test('issues a register key shown once, which works until it is revoked', async ({ request }) => {
  await openSection(admin, 'API-ключи')
  await admin.getByRole('button', { name: '+ Выдать ключ' }).click()

  const form = admin.getByRole('form', { name: 'Новый API-ключ' })
  await form.getByLabel('Название').fill(keyName)
  await form.getByRole('checkbox', { name: /Операции/ }).check()
  await form.getByRole('checkbox', { name: /Поиск клиента/ }).check()
  await expectAccessible(admin, 'Форма нового API-ключа')
  await form.getByRole('button', { name: 'Выдать ключ' }).click()

  const dialog = admin.getByRole('dialog', { name: 'Ключ выдан' })
  const plaintext = await dialog.getByLabel('API-ключ').inputValue()
  expect(plaintext).toMatch(/^\S+\.\S+$/)
  await expectAccessible(admin, 'Окно нового ключа')

  // Closing needs an explicit "I saved it"; afterwards the key is gone.
  const close = dialog.getByRole('button', { name: 'Закрыть' })
  await expect(close).toBeDisabled()
  await dialog.getByRole('checkbox', { name: 'Я сохранил ключ в надёжном месте' }).check()
  await close.click()
  await expect(dialog).toBeHidden()
  expect(await admin.content()).not.toContain(plaintext)

  const row = admin.getByRole('row').filter({ hasText: keyName })
  await expect(row).toContainText('Активен')

  // The key really authenticates the register: an unknown phone is a 404
  // from an authorized caller, not a 401.
  const lookup = () =>
    request.get(`${API_URL}/api/v1/clients/lookup?phone=%2B79990000999`, { headers: { Authorization: `Bearer ${plaintext}` } })
  expect((await lookup()).status()).toBe(404)

  await row.getByRole('button', { name: `Отозвать ключ «${keyName}»` }).click()
  await admin.getByRole('alertdialog', { name: `Отозвать ключ «${keyName}»` }).getByRole('button', { name: 'Отозвать ключ' }).click()
  await expect(admin.getByRole('status').filter({ hasText: `Ключ «${keyName}» отозван` })).toBeVisible()
  await expect(admin.getByRole('row').filter({ hasText: keyName })).toContainText('Отозван')

  expect((await lookup()).status()).toBe(401)
  await expectAccessible(admin, 'API-ключи')
})

test('a store manager gets access by code and sees only what the role allows', async () => {
  const context = await browser.newContext({ locale: 'ru-RU' })
  const page = await context.newPage()
  const token = captureStaffToken(page)

  // Before anyone grants access: the "no access" screen with the code.
  await login(page, manager)
  await expect(page.getByRole('heading', { name: 'Нет доступа к организациям' })).toBeVisible()
  await expect(page.getByLabel('Ваш код доступа')).toHaveValue(manager.subject)
  await expectAccessible(page, 'Нет доступа')

  // The admin grants store_manager on the new store with that code.
  await openSection(admin, 'Сотрудники и доступ')
  await admin.getByRole('button', { name: '+ Добавить сотрудника' }).click()
  const form = admin.getByRole('form', { name: 'Новый сотрудник' })
  await form.getByLabel('Код доступа').fill(manager.subject)
  await form.getByLabel('Роль', { exact: true }).selectOption({ label: 'Управляющий магазином' })
  await form.getByLabel('Доступ', { exact: true }).selectOption({ label: `Магазин «${storeName}»` })
  await expectAccessible(admin, 'Форма нового сотрудника')
  await form.getByRole('button', { name: 'Выдать доступ' }).click()
  // Named by profile if the manager already logged in, by code otherwise.
  await expect(admin.getByRole('status').filter({ hasText: /^Доступ выдан: / })).toBeVisible()
  await expectAccessible(admin, 'Сотрудники и доступ')

  // The manager checks again and lands in their only store.
  await page.getByRole('button', { name: 'Проверить доступ' }).click()
  await expect(page.getByRole('heading', { level: 1, name: `Обзор — ${storeName}` })).toBeVisible()

  const nav = page.getByRole('navigation', { name: 'Основная навигация' })
  for (const allowed of ['Обзор', 'Организации и магазины', 'Программа лояльности', 'Клиенты']) {
    await expect(nav.getByRole('link', { name: allowed })).toBeVisible()
  }
  for (const hidden of ['Сотрудники и доступ', 'API-ключи', 'Аудит']) {
    await expect(nav.getByRole('link', { name: hidden })).toHaveCount(0)
  }
  // Store managers don't see the register's key status either.
  await expect(page.getByText('Касса или 1С')).toHaveCount(0)
  await expectAccessible(page, 'Обзор управляющего')

  // A hidden section opened by URL says "no rights", not a broken page.
  for (const path of ['/api-keys', '/staff']) {
    await page.goto(path)
    await expect(page.getByRole('alert').filter({ hasText: 'Недостаточно прав' })).toBeVisible()
  }
  await expectAccessible(page, 'Недостаточно прав')

  // Hiding is not the access control: the API itself refuses the manager.
  const staffToken = token()
  expect(staffToken, 'the app should have called the API with a staff token').not.toBeNull()
  const orgID = await admin.getByLabel('Организация', { exact: true }).inputValue()
  const storeID = await page.getByLabel('Магазин', { exact: true }).inputValue()
  const asManager = { headers: { Authorization: `Bearer ${staffToken}` } }
  const keys = await page.request.get(`${API_URL}/api/v1/admin/organizations/${orgID}/stores/${storeID}/api-keys`, asManager)
  expect(keys.status()).toBe(403)
  const issue = await page.request.post(`${API_URL}/api/v1/admin/organizations/${orgID}/stores/${storeID}/api-keys`, {
    ...asManager,
    data: { name: 'не должен выдаться', scopes: ['transactions.write'] },
  })
  expect(issue.status()).toBe(403)
  const staff = await page.request.get(`${API_URL}/api/v1/admin/organizations/${orgID}/staff`, asManager)
  expect(staff.status()).toBe(403)

  await context.close()
})

test('the audit log records what the admin just did', async () => {
  await openSection(admin, 'Аудит')
  await expect(admin.getByRole('heading', { level: 1, name: `Аудит — ${orgName}` })).toBeVisible()
  for (const event of [
    `Создан магазин «${storeName}»`,
    'Сохранена программа лояльности (версия 1)',
    'Программа лояльности возвращена к версии 1 (стала версией 3)',
    `Выдан API-ключ «${keyName}»`,
    `Отозван API-ключ «${keyName}»`,
  ]) {
    await expect(admin.getByText(event)).toBeVisible()
  }
  await expectAccessible(admin, 'Аудит')
})
