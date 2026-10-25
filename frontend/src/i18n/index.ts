import { createI18n } from 'vue-i18n'
import id from './id-ID'
import en from './en-US'

/**
 * Bahasa Indonesia is the default locale (PRD Q-04 assumption), with English
 * available from Settings. Every user-visible string comes from these files.
 */
const stored = typeof localStorage !== 'undefined' ? localStorage.getItem('li.locale') : null

export const i18n = createI18n({
  legacy: false,
  locale: stored || 'id-ID',
  fallbackLocale: 'en-US',
  messages: { 'id-ID': id, 'en-US': en },
})

export function setLocale(locale: string) {
  i18n.global.locale.value = locale as 'id-ID' | 'en-US'
  localStorage.setItem('li.locale', locale)
  document.documentElement.setAttribute('lang', locale)
}

export default i18n
