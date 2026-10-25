import { describe, it, expect } from 'vitest'
import { deadlineCountdown, consentLabel, statusLabel, toIso, initials } from './format'

describe('deadlineCountdown', () => {
  it('flags a passed deadline', () => {
    const past = new Date(Date.now() - 3 * 3600_000).toISOString()
    const r = deadlineCountdown(past)
    expect(r.passed).toBe(true)
    expect(r.text).toMatch(/^Lewat /)
    expect(r.urgent).toBe(false)
  })

  it('flags urgency inside 24 hours', () => {
    const soon = new Date(Date.now() + 5 * 3600_000).toISOString()
    const r = deadlineCountdown(soon)
    expect(r.passed).toBe(false)
    expect(r.urgent).toBe(true)
  })

  it('is not urgent beyond 24 hours', () => {
    const later = new Date(Date.now() + 72 * 3600_000).toISOString()
    const r = deadlineCountdown(later)
    expect(r.urgent).toBe(false)
  })

  it('handles a missing deadline', () => {
    expect(deadlineCountdown(null).text).toBe('Belum diatur')
  })
})

describe('consentLabel', () => {
  it('maps every consent value', () => {
    expect(consentLabel('yes')).toBe('Boleh tampil')
    expect(consentLabel('anonymous')).toBe('Anonim')
    expect(consentLabel('no')).toBe('Tidak tampil')
  })

  it('falls back to "no" wording for unknown input', () => {
    expect(consentLabel('garbage')).toBe('Tidak tampil')
  })
})

describe('statusLabel', () => {
  it('maps prayer statuses', () => {
    expect(statusLabel('open')).toBe('Baru')
    expect(statusLabel('in_prayer')).toBe('Didalam doa')
    expect(statusLabel('answered')).toBe('Dijawab')
    expect(statusLabel('closed')).toBe('Selesai')
  })
})

describe('toIso', () => {
  it('builds a WIB (+07:00) instant', () => {
    expect(toIso('2026-10-31', '17:00')).toBe('2026-10-31T10:00:00.000Z')
  })
})

describe('initials', () => {
  it('takes at most two initials', () => {
    expect(initials('Lead Intercessor')).toBe('LI')
    expect(initials('Sari')).toBe('S')
    expect(initials(null)).toBe('?')
  })
})
