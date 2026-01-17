import { describe, expect, it } from 'vitest'

import { formatFileSize, formatUsedTotalBytes } from './format'

describe('formatFileSize', () => {
  it('formats bytes below threshold as B', () => {
    expect(formatFileSize(999, true)).toBe('999 B')
    expect(formatFileSize(1023, false)).toBe('1023 B')
  })

  it('formats exact threshold into first unit', () => {
    expect(formatFileSize(1000, true, 1)).toBe('1.0 kB')
    expect(formatFileSize(1024, false, 1)).toBe('1.0 KiB')
  })

  it('supports forcing a unit index', () => {
    const gib = 1024 ** 3
    expect(formatFileSize(5 * gib, false, 1, 2)).toBe('5.0 GiB')

    // Clamp negative unit index to 0 (kB / KiB)
    expect(formatFileSize(1000, true, 1, -1)).toBe('1.0 kB')
  })
})

describe('formatUsedTotalBytes', () => {
  it('uses B for totals below threshold', () => {
    expect(formatUsedTotalBytes(50, 999, true)).toBe('50 B / 999 B')
  })

  it('uses a single unit picked from total', () => {
    const gib = 1024 ** 3
    const used = Math.round(24.4 * gib)
    const total = Math.round(62.2 * gib)

    expect(formatUsedTotalBytes(used, total, false)).toBe('24.4 GiB / 62.2 GiB')
  })
})
