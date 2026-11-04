// The Go generator uses the same versioned, independent size stream. The API
// resolves the value again; this only previews it in the form.
export async function autoUsersCount(seed: number): Promise<number> {
  const bytes = new TextEncoder().encode(`users-count/v1/${seed}`)
  const digest = await crypto.subtle.digest('SHA-256', bytes)
  return 25000 + new DataView(digest).getUint32(0, false) % 75001
}
