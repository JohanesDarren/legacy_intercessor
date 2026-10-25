import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api, { tokenStore, ApiErrorException } from '@/services/api'
import type { User } from '@/services/types'

/**
 * Session store. Holds the signed-in user and exposes the role checks that
 * every screen uses to gate actions (mirrors the server-side RBAC).
 */
export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const loading = ref(false)
  const initialised = ref(false)

  const isAuthenticated = computed(() => user.value !== null)
  const role = computed(() => user.value?.role ?? null)
  const isAdmin = computed(() => role.value === 'admin')
  const isIntercessor = computed(() => role.value === 'intercessor')
  const isViewer = computed(() => role.value === 'viewer')

  /** Roles allowed to read prayer content. */
  const canReadRecords = computed(
    () => role.value === 'admin' || role.value === 'intercessor',
  )
  /** Roles allowed to create and edit records. */
  const canWriteRecords = computed(
    () => role.value === 'admin' || role.value === 'intercessor',
  )
  /** Only admins approve summaries and export slides. */
  const canManageSlides = computed(() => role.value === 'admin')
  /** Only admins manage the roster. */
  const canManageRoster = computed(() => role.value === 'admin')

  const needsPrivacyAck = computed(
    () => user.value !== null && user.value.privacy_ack_at === null,
  )

  async function login(email: string, password: string) {
    loading.value = true
    try {
      const pair = await api.login(email, password)
      user.value = pair.user
      initialised.value = true
      return pair.user
    } finally {
      loading.value = false
    }
  }

  async function acceptInvite(token: string, password: string, fullName?: string) {
    loading.value = true
    try {
      const pair = await api.acceptInvite(token, password, fullName)
      user.value = pair.user
      initialised.value = true
      return pair.user
    } finally {
      loading.value = false
    }
  }

  /** Restores the session from a stored token on app start. */
  async function restore() {
    if (!tokenStore.access) {
      initialised.value = true
      return null
    }
    try {
      user.value = await api.me()
      return user.value
    } catch (err) {
      if (err instanceof ApiErrorException && err.status === 401) {
        tokenStore.clear()
      }
      user.value = null
      return null
    } finally {
      initialised.value = true
    }
  }

  async function logout() {
    try {
      await api.logout()
    } finally {
      user.value = null
    }
  }

  async function refreshProfile() {
    if (!tokenStore.access) return null
    user.value = await api.me()
    return user.value
  }

  async function acknowledgePrivacy() {
    await api.acknowledgePrivacy()
    if (user.value) {
      user.value = { ...user.value, privacy_ack_at: new Date().toISOString() }
    }
  }

  /** Clears local state when the refresh token is rejected. */
  function clear() {
    tokenStore.clear()
    user.value = null
  }

  return {
    user,
    loading,
    initialised,
    isAuthenticated,
    role,
    isAdmin,
    isIntercessor,
    isViewer,
    canReadRecords,
    canWriteRecords,
    canManageSlides,
    canManageRoster,
    needsPrivacyAck,
    login,
    acceptInvite,
    restore,
    logout,
    refreshProfile,
    acknowledgePrivacy,
    clear,
  }
})
