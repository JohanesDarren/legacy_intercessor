import { boot } from 'quasar/wrappers'
import { i18n } from '@/i18n'

/** Installs vue-i18n on the app instance. */
export default boot(({ app }) => {
  app.use(i18n)
})
