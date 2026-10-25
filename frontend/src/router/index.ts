import { defineRouter } from '#q-app/wrappers'
import {
  createMemoryHistory,
  createRouter,
  createWebHashHistory,
  createWebHistory,
} from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import routes from './routes'

export default defineRouter((/* { store, ssrContext } */) => {
  const createHistory = import.meta.env.SSR
    ? createMemoryHistory
    : import.meta.env.VUE_ROUTER_MODE === 'history'
      ? createWebHistory
      : createWebHashHistory

  const Router = createRouter({
    scrollBehavior: () => ({ left: 0, top: 0 }),
    routes,
    history: createHistory(import.meta.env.VUE_ROUTER_BASE),
  })

  // Route guard: restore the session once, then enforce auth and roles.
  Router.beforeEach(async (to) => {
    const auth = useAuthStore()

    if (!auth.initialised) {
      await auth.restore()
    }

    const requiresAuth = to.matched.some((r) => r.meta?.requiresAuth)
    const publicOnly = to.matched.some((r) => r.meta?.publicOnly)

    if (requiresAuth && !auth.isAuthenticated) {
      return { name: 'login', query: { next: to.fullPath } }
    }
    if (publicOnly && auth.isAuthenticated) {
      return { name: 'dashboard' }
    }

    const roles = to.matched.flatMap((r) => (r.meta?.roles as string[] | undefined) ?? [])
    if (roles.length > 0 && auth.role && !roles.includes(auth.role)) {
      return { name: 'forbidden' }
    }

    return true
  })

  return Router
})
