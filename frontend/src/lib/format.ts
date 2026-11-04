export const planNames: Record<string, string> = {
  monthly: 'Месячный', quarterly: 'Квартальный', 'half-year': 'Полугодовой', yearly: 'Годовой',
}
export const planColors: Record<string, string> = {
  monthly: '#1d5fbf', quarterly: '#687a92', 'half-year': '#9199a5', yearly: '#292c32',
}
export const number = (value: number | bigint | null | undefined) => new Intl.NumberFormat('ru-RU').format(value ?? 0)

// Round fractional minor units and rubles with integer arithmetic, half away from zero.
export function money(value: number | string | bigint | null | undefined, decimals = 0): string {
  const raw = String(value ?? 0)
  if (!/^-?\d+(\.\d+)?$/.test(raw)) return '—'
  const negative = raw.startsWith('-')
  const [whole, fraction = ''] = raw.replace('-', '').split('.')
  const cents = BigInt(whole ?? '0') + (Number(fraction[0] ?? 0) >= 5 ? 1n : 0n)
  const rounded = decimals ? cents : (cents + 50n) / 100n
  const sign = negative && rounded !== 0n ? '−' : ''
  return decimals
    ? `${sign}${number(cents / 100n)},${String(cents % 100n).padStart(2, '0')} ₽`
    : `${sign}${number(rounded)} ₽`
}
export const compact = (value: number) => new Intl.NumberFormat('ru-RU', { notation: 'compact', maximumFractionDigits: 1 }).format(value || 0)
export function assetMoney(amount: number | string, currency: string): string {
  if (currency === 'RUB') return money(amount, 2)
  const scale = currency === 'BTC' ? 8 : currency === 'USDT' ? 6 : undefined
  if (scale == null || (typeof amount === 'number' && !Number.isSafeInteger(amount)) || !/^\d+$/.test(String(amount))) return '—'
  const units = BigInt(amount), divisor = 10n ** BigInt(scale)
  return `${number(units / divisor)},${String(units % divisor).padStart(scale, '0')} ${currency}`
}
export const compactMoney = (value: number) => compact(value / 100) + ' ₽'
export const percent = (value: number | null | undefined) => value == null ? '—' : new Intl.NumberFormat('ru-RU', { style: 'percent', maximumFractionDigits: 1 }).format(value)
export const date = (value: string | null | undefined) => value ? new Date(value).toLocaleDateString('ru-RU', { day: '2-digit', month: 'short', year: 'numeric', timeZone: 'UTC' }) : 'Без подписки'
export const monthName = (value: string) => new Date(value + '-01T00:00:00Z').toLocaleDateString('ru-RU', { month: 'short', year: 'numeric', timeZone: 'UTC' }).replace(' г.', '')
export const time = (value: string | null | undefined) => value ? new Date(value).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' }) : '—'
