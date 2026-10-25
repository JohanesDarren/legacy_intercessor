import { config } from '@vue/test-utils'
import { vi } from 'vitest'

// Quasar components read screen size and animations; jsdom lacks both.
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }),
})

// Silence Quasar's "not installed" warnings in shallow mounts.
config.global.stubs = {
  'q-layout': { template: '<div><slot /></div>' },
  'q-page-container': { template: '<div><slot /></div>' },
}
