import { boot } from 'quasar/wrappers'
import { createPinia } from 'pinia'

/**
 * Installs Pinia. This must run before the router is used, because the router
 * guard reads the auth store to decide whether a route is allowed.
 */
export default boot(({ app }) => {
  app.use(createPinia())
})
