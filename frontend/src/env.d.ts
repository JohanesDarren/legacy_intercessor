/// <reference types="quasar" />

/**
 * Ambient declarations for the Quasar app-vite environment.
 * `#q-app/wrappers` is provided by the Quasar build; TypeScript needs it
 * declared so router and boot files type-check.
 */
declare module '#q-app/wrappers' {
  import type { App } from 'vue'
  import type { Router } from 'vue-router'
  import type { Pinia } from 'pinia'

  interface BootContext {
    app: App
    router: Router
    store?: Pinia
    ssrContext?: unknown
    urlPath?: string
    redirect: (url: string) => void
  }

  type BootCallback = (ctx: BootContext) => void | Promise<void>
  export function boot(cb: BootCallback): BootCallback
  export function defineBoot(cb: BootCallback): BootCallback

  interface RouterContext {
    store?: Pinia
    ssrContext?: unknown
  }
  export function defineRouter(factory: (ctx: RouterContext) => Router): () => Router
  export function route(cb: unknown): unknown
  export function defineConfig(cb: (ctx: Record<string, unknown>) => unknown): unknown
}

interface ImportMetaEnv {
  readonly SSR: boolean
  readonly VUE_ROUTER_MODE: 'history' | 'hash'
  readonly VUE_ROUTER_BASE: string
  readonly API_BASE_URL: string
  readonly MODE: string
  readonly DEV: boolean
  readonly PROD: boolean
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
