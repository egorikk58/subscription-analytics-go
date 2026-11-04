import { expect, test } from '@playwright/test'

test('reviewer explores actual data, filters, Telegram lookup, seed preview and HTTP cache', async ({ page, request }) => {
  const errors: string[] = []
  page.on('pageerror', error => errors.push(error.message))
  const initial = await request.get('/api/dashboard')
  expect(initial.status()).toBe(200)
  const live = await initial.json()
  expect(live.mode).toBe('live')
  expect(live.summary.users_count).toBeGreaterThanOrEqual(10000)
  for (const view of ['overview', 'revenue', 'retention', 'cohorts', 'users', 'generator', 'sources']) {
    await page.goto('/#/' + view)
    await expect(page.locator('[data-frontend-stack]')).toBeVisible()
    await expect(page.locator('#mode-badge')).toContainText('Снимок')
    await expect(page.locator('#content')).not.toHaveAttribute('aria-busy', 'true')
    await expect(page.getByRole('status', { name: 'Загрузка аналитики' })).toHaveCount(0)
  }
  await page.goto('/#/overview')
  await page.getByRole('button', { name: 'Подробнее', exact: true }).click()
  await expect(page.getByRole('dialog', { name: 'Сравнение источников' })).toBeVisible()
  const source = live.utm_groups[0].utm_link
  await page.getByRole('dialog').getByRole('button', { name: source === 'unknown' ? 'Без UTM' : source, exact: true }).click()
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await expect(page.getByLabel('Источник трафика UTM')).toHaveValue(source)
  await page.goto('/#/users')
  await page.locator('#filter-from').fill('2099-01-01')
  await expect(page.locator('#selection-count')).toHaveText('0')
  await page.locator('#reset-filters').click()
  await expect(page.locator('#selection-count')).not.toHaveText('0')
  const telegram = String(live.users.items[0].telegram_id)
  await page.getByLabel('Telegram ID для LTV').fill(telegram)
  await page.getByRole('button', { name: 'Получить LTV', exact: true }).click()
  await expect(page.locator('pre').filter({ hasText: 'payments_count' })).toContainText(telegram)

  await page.goto('/#/generator')
  await page.locator('#seed-value').fill('731')
  await page.locator('#seed-users').fill('10000')
  await page.getByRole('button', { name: 'Сгенерировать набор', exact: true }).click()
  await expect(page.locator('#mode-badge')).toContainText('731')
  expect((await (await request.get('/status')).json()).snapshot_id).toBe(live.snapshot_id)
  await page.locator('#return-live').click()
  await expect(page.locator('#mode-badge')).toContainText('Снимок')

  await page.goto('/#/sources')
  await page.getByRole('button', { name: 'Выполнить GET', exact: true }).click()
  await expect(page.getByText(/^HTTP 200\b/)).toBeVisible()
  await page.getByRole('button', { name: 'Проверить 304', exact: true }).click()
  await expect(page.getByText(/^HTTP 304\b/)).toBeVisible()
  await expect(page.getByText('Тело ответа отсутствует')).toBeVisible()
  expect(errors).toEqual([])
})

test('reviewer opens Swagger and the provisioned Grafana overview', async ({ browser }) => {
  const password = process.env.OPS_PASSWORD
  expect(password, 'Load the environment containing OPS_PASSWORD').toBeTruthy()
  const context = await browser.newContext({
    baseURL: process.env.BASE_URL || process.env.PUBLIC_BASE_URL || 'http://127.0.0.1:8080',
    httpCredentials: { username: process.env.OPS_USERNAME || 'ops', password: password! },
  })
  const page = await context.newPage()
  await page.goto('/docs/')
  await expect(page.locator('.swagger-ui').first()).toBeVisible()
  await expect(page.getByText('/metrics/ltv/summary', { exact: true }).first()).toBeVisible()
  await page.goto('/grafana/', { waitUntil: 'networkidle' })
  await expect(page.getByText('Go-сервисы доступны', { exact: true })).toBeVisible()
  await expect(page.getByText('4 из 4', { exact: true })).toBeVisible()
  await context.close()
})
