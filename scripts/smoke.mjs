import assert from 'node:assert/strict'

const base = process.env.BASE_URL || process.env.PUBLIC_BASE_URL || 'http://127.0.0.1:8080'
const password = process.env.OPS_PASSWORD
assert(password, 'Load .env before running the smoke check (node --env-file=.env scripts/smoke.mjs).')
const authorization = 'Basic ' + Buffer.from(`${process.env.OPS_USERNAME || 'ops'}:${password}`).toString('base64')
const pause = ms => new Promise(resolve => setTimeout(resolve, ms))

async function request(path, { ops = false, status = 200, headers = {} } = {}) {
  const response = await fetch(new URL(path, base), {
    headers: { ...(ops ? { Authorization: authorization } : {}), ...headers },
    signal: AbortSignal.timeout(15000),
  })
  const text = await response.text()
  assert.equal(response.status, status, `${path}: expected HTTP ${status}, got ${response.status}`)
  return { response, text, json: () => JSON.parse(text) }
}
function metadata(value) {
  assert(Object.hasOwn(value, 'data_as_of'), 'Missing data_as_of')
}

// Health is independent from the first snapshot; wait for data explicitly.
metadata((await request('/healthz')).json())
let ready = false
for (let attempt = 0; attempt < 60; attempt++) {
  const response = await fetch(new URL('/metrics/ltv/summary', base), { signal: AbortSignal.timeout(15000) })
  if (response.status === 200) { await response.arrayBuffer(); ready = true; break }
  assert.equal(response.status, 503)
  assert(response.headers.get('retry-after'))
  metadata(await response.json())
  await pause(1000)
}
assert(ready, 'First snapshot did not become available')
const status = (await request('/status')).json()
metadata(status)
assert(status.snapshot_id && status.last_success_at && status.next_sync_at)
assert.equal(status.last_error, null)

const summaryResponse = await request('/metrics/ltv/summary')
const summary = summaryResponse.json()
assert(summary.users_count >= 10000)
assert.equal(summary.currency, 'RUB')
for (const field of ['mean_ltv_minor', 'median_ltv_minor']) assert.match(summary[field], /^-?\d+\.\d{2}$/)
assert.equal(summary.data_as_of, status.data_as_of)
const tag = summaryResponse.response.headers.get('etag')
assert(tag)
assert(summaryResponse.response.headers.get('last-modified'))
assert.match(summaryResponse.response.headers.get('cache-control'), /max-age=\d+/)
for (const validator of [tag, 'W/' + tag, '"unrelated", ' + tag, '*']) {
  const conditional = await request('/metrics/ltv/summary', { status: 304, headers: { 'If-None-Match': validator } })
  assert.equal(conditional.text, '')
  assert.equal(conditional.response.headers.get('etag'), tag)
}
console.log('PASS snapshot, LTV and HTTP validators')

const plans = (await request('/api/plans')).json().plans
assert.deepEqual(plans.map(plan => plan.period_months).sort((a, b) => a - b), [1, 3, 6, 12])
const byPlan = (await request('/metrics/ltv/by-plan')).json()
assert.equal(byPlan.plans.reduce((sum, plan) => sum + plan.total_revenue_minor, 0), summary.total_revenue_minor)
for (const plan of plans) {
  const retention = (await request(`/metrics/retention?plan_id=${plan.id}&max_periods=24`)).json()
  metadata(retention)
  assert.equal(retention.periods.length, 24)
  for (const row of retention.periods) {
    assert(row.cumulative_eligible >= row.eligible && row.eligible >= row.renewed)
    assert.equal(row.rate === null, row.eligible === 0)
    assert.equal(row.cumulative_rate === null, row.cumulative_eligible === 0)
    if (row.rate !== null) assert(row.rate >= 0 && row.rate <= 1)
    if (row.cumulative_rate !== null) assert(row.cumulative_rate >= 0 && row.cumulative_rate <= 1)
  }
}
const empty = (await request('/metrics/ltv/summary?from=2099-01-01T00:00:00Z&to=2100-01-01T00:00:00Z')).json()
assert.equal(empty.users_count, 0)
assert.equal(empty.total_revenue_minor, 0)
assert.equal(empty.median_ltv_minor, '0.00')
for (const path of ['/metrics/retention?plan_id=absent', '/metrics/retention?plan_id=monthly&max_periods=25', '/metrics/ltv/summary?from=bad', '/metrics/ltv/summary?from=2026-01-01T00:00:00Z&to=2025-01-01T00:00:00Z']) {
  metadata((await request(path, { status: 400 })).json())
}
console.log('PASS tariff durations, maturity, empty cohorts and invalid filters')

const dashboard = (await request('/api/dashboard')).json()
assert.equal(dashboard.mode, 'live')
assert.equal(dashboard.summary.users_count, summary.users_count)
for (const currency of ['RUB', 'BTC', 'USDT']) assert(dashboard.dataset.currencies[currency] > 0)
const utm = (await request('/metrics/ltv/by-utm')).json()
assert.equal(utm.groups.reduce((sum, group) => sum + group.users_count, 0), summary.users_count)
assert.equal(utm.groups.reduce((sum, group) => sum + group.total_revenue_minor, 0), summary.total_revenue_minor)
const unknown = (await request('/metrics/ltv/by-utm?utm_link=unknown')).json()
assert.equal(unknown.groups.length, 1)
assert.equal(unknown.groups[0].utm_link, 'unknown')
const user = dashboard.users.items[0]
const profile = (await request(`/users/by-telegram/${user.telegram_id}/ltv`)).json()
for (const field of ['user_id', 'telegram_id', 'first_plan_id', 'ltv', 'payments_count', 'current_status', 'data_as_of']) assert(Object.hasOwn(profile, field))
assert.equal(profile.user_id, user.id)
assert.equal(profile.ltv, user.ltv_minor)
metadata((await request('/users/by-telegram/9223372036854775807/ltv', { status: 404 })).json())
metadata((await request('/users/by-telegram/0/ltv', { status: 400 })).json())
const unpaid = (await request('/users/by-telegram/1000000000/ltv')).json()
assert.equal(unpaid.ltv, 0)
assert.equal(unpaid.payments_count, 0)
assert.equal(unpaid.first_plan_id, null)
assert.equal(unpaid.current_status, 'none')
console.log('PASS RUB/BTC/USDT, UTM grouping and Telegram LTV')

const html = (await request('/')).text
const entry = html.match(/src="(\/assets\/[^\"]+\.js)"/)?.[1]
assert(entry, 'Embedded React entrypoint is missing')
const asset = await request(entry)
assert.match(asset.response.headers.get('content-type'), /javascript/)
assert(asset.text.length > 100)
const specification = (await request('/openapi.json')).json()
assert(specification.paths['/metrics/ltv/summary'])
assert(specification.paths['/users/by-telegram/{telegram_id}/ltv'])
await request('/docs/')
for (const path of ['/db/', '/grafana/', '/prometheus/', '/metrics']) await request(path, { status: 401 })
assert((await request('/db/', { ops: true })).text.includes('pgAdmin'))
assert.equal((await request('/grafana/api/health', { ops: true })).json().database, 'ok')
const dashboards = (await request('/grafana/api/search?type=dash-db', { ops: true })).json()
assert.deepEqual(dashboards.map(item => item.uid).sort(), ['subscope-http', 'subscope-overview', 'subscope-storage', 'subscope-sync'])
assert.equal((await request('/grafana/api/datasources/uid/subscope-prometheus/health', { ops: true })).json().status, 'OK')
let targetsReady = false
for (let attempt = 0; attempt < 30; attempt++) {
  const targets = (await request('/prometheus/api/v1/targets', { ops: true })).json().data.activeTargets
  if (targets.length === 5 && targets.every(target => target.health === 'up')) { targetsReady = true; break }
  await pause(1000)
}
assert(targetsReady, 'Prometheus targets did not become 5/5 UP')
assert((await request('/metrics', { ops: true })).text.includes('subscope_http_requests_total'))
console.log('PASS embedded frontend, OpenAPI, pgAdmin, Grafana dashboards and Prometheus')
