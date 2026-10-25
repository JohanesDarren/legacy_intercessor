import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

/**
 * The privacy dialog in MainLayout is driven entirely by
 * auth.needsPrivacyAck. These tests pin that contract, because the original bug
 * was a writable computed with an empty setter that swallowed the close event.
 */

vi.mock('@/services/api', () => {
  const user = {
    id: 'u1',
    full_name: 'Lead Intercessor',
    email: 'admin@legacy.sukawarna.id',
    role: 'admin',
    is_active: true,
    last_login_at: null,
    privacy_ack_at: null as string | null,
    created_at: '2026-01-01T00:00:00Z',
  }
  return {
    default: {
      me: vi.fn(async () => ({ ...user })),
      acknowledgePrivacy: vi.fn(async () => {
        user.privacy_ack_at = new Date().toISOString()
      }),
      logout: vi.fn(async () => {}),
      login: vi.fn(async () => ({ user: { ...user } })),
      acceptInvite: vi.fn(async () => ({ user: { ...user } })),
    },
    tokenStore: {
      get access() {
        return 'stub-token'
      },
      get refresh() {
        return 'stub-refresh'
      },
      set: vi.fn(),
      clear: vi.fn(),
    },
    ApiErrorException: class extends Error {},
  }
})

import { useAuthStore } from '@/stores/auth'

describe('auth store: privacy notice gate', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('starts with needsPrivacyAck false when there is no user', () => {
    const auth = useAuthStore()
    expect(auth.needsPrivacyAck).toBe(false)
  })

  it('reports needsPrivacyAck true for a user who has not acknowledged', async () => {
    const auth = useAuthStore()
    await auth.restore()
    expect(auth.user).not.toBeNull()
    expect(auth.user?.privacy_ack_at).toBeNull()
    expect(auth.needsPrivacyAck).toBe(true)
  })

  it('clears needsPrivacyAck after acknowledgePrivacy resolves', async () => {
    const auth = useAuthStore()
    await auth.restore()
    expect(auth.needsPrivacyAck).toBe(true)

    await auth.acknowledgePrivacy()

    // This is the exact regression: acknowledging must flip the flag, which is
    // what closes the dialog.
    expect(auth.user?.privacy_ack_at).not.toBeNull()
    expect(auth.needsPrivacyAck).toBe(false)
  })
})

describe('auth store: role gates', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('grants admin every capability', async () => {
    const auth = useAuthStore()
    await auth.restore()
    expect(auth.isAdmin).toBe(true)
    expect(auth.canReadRecords).toBe(true)
    expect(auth.canWriteRecords).toBe(true)
    expect(auth.canManageSlides).toBe(true)
    expect(auth.canManageRoster).toBe(true)
  })
})
