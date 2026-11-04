export type DesignVariant = 'a' | 'b'

export function initialDesign(): DesignVariant {
  return new URLSearchParams(window.location.search).get('design') === 'b' ? 'b' : 'a'
}

export function saveDesign(variant: DesignVariant) {
  const url = new URL(window.location.href)
  url.searchParams.set('design', variant)
  window.history.replaceState(window.history.state, '', url.href)
}
