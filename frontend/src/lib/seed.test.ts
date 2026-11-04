import { webcrypto } from 'node:crypto'
import { afterEach, expect, test, vi } from 'vitest'
import { autoUsersCount } from './seed'

afterEach(() => vi.unstubAllGlobals())
test('the independent size stream matches the Go seed42 vector and repeats exactly', async () => {
  vi.stubGlobal('crypto', webcrypto)
  expect(await autoUsersCount(42)).toBe(31419)
  expect(await autoUsersCount(731)).toBe(await autoUsersCount(731))
  expect(await autoUsersCount(731)).not.toBe(31419)
})
