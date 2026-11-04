import { expect, test } from '@playwright/test'
import { makeDashboard } from '../src/test/fixtures'

test.beforeEach(async ({page}) => {
  await page.route('**/status', route => route.fulfill({json:{snapshot_id:1,data_as_of:'2026-11-01T00:00:00Z',last_success_at:'2026-11-01T00:00:00Z',next_sync_at:null,last_error:null,sync_in_progress:false}}))
  await page.route('**/api/dashboard?**', route => route.fulfill({json:makeDashboard()}))
})
test('all seven React sections fit desktop, tablet and phone with fixture API', async ({page}) => {
  const errors:string[]=[];page.on('pageerror', error => errors.push(error.message))
  for(const width of [1440,768,390]) {
    await page.setViewportSize({width,height:1000})
    for(const view of ['overview','revenue','retention','cohorts','users','generator','sources']) {
      await page.goto('/#/'+view)
      await expect(page.locator('[data-frontend-stack]')).toBeVisible()
      await expect(page.locator('#mode-badge')).toContainText('seed 42')
      await expect(page.locator('#content')).not.toHaveAttribute('aria-busy','true')
      await expect(page.getByRole('status',{name:'Загрузка аналитики'})).toHaveCount(0)
      expect(await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth), `${view} at ${width}`).toBeLessThanOrEqual(1)
    }
  }
  expect(errors).toEqual([])
})
test('new filters do not relabel old data while a new cohort is loading', async ({page}) => {
  let release:()=>void=()=>{}
  const gate = new Promise<void>(resolve=>{release=resolve})
  await page.route('**/api/dashboard?**', async route => {
    if(new URL(route.request().url()).searchParams.get('from')?.startsWith('2025')) await gate
    await route.fulfill({json:makeDashboard()})
  })
  await page.goto('/#/overview');await expect(page.locator('#mode-badge')).toBeVisible()
  await page.locator('#filter-from').fill('2025-01-01')
  await expect(page.getByText(/Показаны данные:/)).toContainText('начало истории')
  await expect(page.getByText(/Показаны данные:/)).toContainText('Загружается новая выборка')
  release()
  await expect(page.getByText(/Показаны данные:/)).toContainText('2025-01-01')
})

test('a pending tariff cannot relabel or activate the previous cohort matrix', async ({page}) => {
  let release:()=>void=()=>{}
  const gate=new Promise<void>(resolve=>{release=resolve})
  await page.route('**/api/dashboard?**',async route=>{
    if(new URL(route.request().url()).searchParams.get('plan_id')==='yearly') await gate
    await route.fulfill({json:makeDashboard()})
  })
  await page.goto('/#/cohorts')
  await expect(page.getByText('Месячный · кумулятивное удержание по периодам',{exact:true})).toBeVisible()
  await page.locator('#filter-plan').selectOption('yearly')
  await expect(page.locator('#content')).toHaveAttribute('aria-busy','true')
  await expect(page.locator('#content')).toHaveAttribute('inert','')
  await expect(page.getByText('Месячный · кумулятивное удержание по периодам',{exact:true})).toBeVisible()
  await expect(page.getByText('Годовой · кумулятивное удержание по периодам',{exact:true})).toHaveCount(0)
  release()
  await expect(page.getByText('Годовой · кумулятивное удержание по периодам',{exact:true})).toBeVisible()
  await expect(page.locator('#content')).not.toHaveAttribute('inert','')
})
test('API inspector exposes real HTTP validators and an empty 304 response', async ({page}) => {
  await page.route('**/metrics/ltv/summary',route=>route.request().headers()['if-none-match'] ? route.fulfill({status:304,headers:{etag:'"1"'}}) : route.fulfill({status:200,headers:{etag:'"1"','cache-control':'public, max-age=100'},json:{data_as_of:'2026-11-01T00:00:00Z',users_count:10000}}))
  await page.goto('/#/sources')
  await page.getByRole('button',{name:'Выполнить GET',exact:true}).click()
  await expect(page.getByText('HTTP 200')).toBeVisible()
  await page.getByRole('button',{name:'Проверить 304',exact:true}).click()
  await expect(page.getByText('HTTP 304')).toBeVisible()
  await expect(page.getByText('Тело ответа отсутствует')).toBeVisible()
})

test('late preview cannot replace live data after return, and CSV follows selected sample tab', async ({page}) => {
  let release:()=>void=()=>{}
  const gate=new Promise<void>(resolve=>{release=resolve})
  await page.route('**/api/demo/preview?**', async route => {
    const config=route.request().postDataJSON() as {seed:number}
    if(config.seed===732) await gate
    const report=makeDashboard();report.mode='preview';report.snapshot_id=null;report.dataset.seed=config.seed
    await route.fulfill({json:report}).catch(()=>{})
  })
  await page.goto('/#/generator')
  await page.locator('#seed-value').fill('731')
  await page.getByRole('button',{name:'Сгенерировать набор',exact:true}).click()
  await expect(page.locator('#mode-badge')).toContainText('731')
  await page.getByRole('tab',{name:'Пользователи',exact:true}).click()
  const download=page.waitForEvent('download');await page.locator('#export-button').click()
  const stream=await (await download).createReadStream();let csv='';for await(const chunk of stream!) csv+=chunk.toString()
  expect(csv).toContain('telegram_id');expect(csv).not.toContain('subscription_id')
  await page.locator('#seed-value').fill('732')
  await page.getByRole('button',{name:'Сгенерировать набор',exact:true}).click()
  await expect(page.locator('#generate-button')).toBeDisabled()
  await page.locator('#return-live').click()
  release()
  await expect(page.locator('#mode-badge')).toContainText('Снимок')
  await expect(page.locator('#mode-badge')).toContainText('42')
})

test('quick navigation supports keyboard search and dismisses with Escape', async ({page}) => {
  await page.goto('/#/overview')
  await expect(page.locator('#mode-badge')).toBeVisible()
  await page.keyboard.press('Control+k')
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.getByRole('combobox', {name:'Найти раздел'}).fill('Когорты')
  await page.keyboard.press('Enter')
  await expect(page).toHaveURL(/#\/cohorts$/)
  await expect(page.getByRole('dialog')).toHaveCount(0)
  await page.keyboard.press('Control+k')
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(page.getByRole('dialog')).toHaveCount(0)
})

test('phone filters disclose controls and a shared selection survives a reload', async ({page}) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator, 'clipboard', {value: {writeText: async (value:string) => {
      (window as unknown as {copiedSelection:string}).copiedSelection = value
    }}})
  })
  await page.setViewportSize({width:390,height:844})
  await page.goto('/#/overview')
  await expect(page.locator('#mode-badge')).toBeVisible()
  await expect(page.locator('#filter-plan')).toBeHidden()
  await page.getByRole('button',{name:/^Фильтры/}).click()
  await page.locator('#filter-plan').selectOption('yearly')
  await page.locator('#filter-from').fill('2025-01-01')
  await expect(page.locator('#content')).not.toHaveAttribute('aria-busy','true')
  await page.getByRole('button',{name:'Скопировать ссылку на выборку'}).click()
  const copied=await page.evaluate(() => (window as unknown as {copiedSelection:string}).copiedSelection)
  expect(copied).toContain('plan_id=yearly')
  expect(copied).toContain('from=2025-01-01')
  await page.goto(copied)
  await page.reload()
  await page.getByRole('button',{name:/^Фильтры/}).click()
  await expect(page.locator('#filter-plan')).toHaveValue('yearly')
  await expect(page.locator('#filter-from')).toHaveValue('2025-01-01')
})
