export function heatAppearance(value: number | null) {
  if (value == null) return { backgroundColor: 'var(--muted)', color: 'var(--muted-foreground)' }
  const level = Math.min(5, Math.floor(Math.max(0, Math.min(1, value)) * 5))
  return { backgroundColor: `var(--heat-${level})`, color: `var(--heat-ink-${level})` }
}
