import { boot } from 'quasar/wrappers'
import { Notify } from 'quasar'
import { ApiErrorException } from '@/services/api'

/**
 * Registers global axios-free error handling helpers. The API client is a
 * module singleton, so this boot file only wires up user-facing notifications
 * for session expiry.
 */
export default boot(() => {
  window.addEventListener('li:session-expired', () => {
    Notify.create({
      type: 'warning',
      message: 'Sesi Anda berakhir. Silakan masuk lagi.',
      timeout: 4000,
    })
  })
})

/** Turns an unknown error into a user-facing message. */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiErrorException) {
    if (err.code === 'NETWORK_ERROR') {
      return 'Tidak dapat terhubung ke server. Periksa koneksi Anda.'
    }
    if (err.code === 'RATE_LIMITED') return err.message
    if (err.code === 'FORBIDDEN') return 'Anda tidak punya akses untuk tindakan ini.'
    return err.message
  }
  if (err instanceof Error) return err.message
  return 'Terjadi kesalahan. Coba lagi.'
}
