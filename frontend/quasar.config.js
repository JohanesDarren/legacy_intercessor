import { defineConfig } from '#q-app/wrappers'
import { fileURLToPath } from 'node:url'

export default defineConfig((/* ctx */) => {
  return {
    boot: ['pinia', 'axios', 'i18n'],

    css: ['app.scss'],

    extras: ['roboto-font', 'material-icons'],

    build: {
      target: {
        browser: ['es2020', 'firefox115', 'chrome115', 'safari14'],
        node: 'node20',
      },
      typescript: {
        strict: true,
        vueShim: true,
      },
      vueRouterMode: 'history',

      env: {
        // Relative path: the dev server proxies /api to the Go API, and in
        // production the Go binary serves the built PWA from the same origin.
        API_BASE_URL: process.env.API_BASE_URL || '/api/v1',
      },

      extendViteConf(viteConf) {
        viteConf.resolve = viteConf.resolve || {}
        viteConf.resolve.alias = {
          ...viteConf.resolve.alias,
          '@': fileURLToPath(new URL('./src', import.meta.url)),
        }
      },
    },

    devServer: {
      port: 9000,
      open: false,
      // Proxy API calls so the browser talks to one origin in development.
      // This avoids CORS entirely and mirrors the single-container production
      // deployment where the API serves the built PWA.
      proxy: {
        '/api': {
          target: 'http://localhost:8080',
          changeOrigin: true,
        },
      },
    },

    framework: {
      config: {
        brand: {
          primary: '#000000',
          secondary: '#E5AD38',
          accent: '#FF5500',
          dark: '#000000',
          positive: '#1B7A3E',
          negative: '#C10015',
          info: '#1F6FEB',
          warning: '#E5AD38',
        },
      },
      iconSet: 'material-icons',
      lang: 'en-US',
      plugins: ['Notify', 'Dialog', 'Loading', 'LocalStorage'],
    },

    animations: [],

    ssr: false,

    pwa: {
      workboxMode: 'GenerateSW',
      injectPwaMetaTags: true,
      swFilename: 'sw.js',
      manifestFilename: 'manifest.json',
      useCredentialsForManifestTag: false,
      extendGenerateSWOptions(cfg) {
        cfg.runtimeCaching = [
          ...(cfg.runtimeCaching || []),
          {
            urlPattern: /\/api\/v1\/dashboard\/summary$/,
            handler: 'NetworkFirst',
            options: {
              cacheName: 'dashboard-cache',
              expiration: { maxEntries: 1, maxAgeSeconds: 60 * 60 },
            },
          },
          {
            urlPattern: /\/api\/v1\/booth-shifts\/mine$/,
            handler: 'NetworkFirst',
            options: {
              cacheName: 'roster-cache',
              expiration: { maxEntries: 1, maxAgeSeconds: 60 * 60 * 24 },
            },
          },
        ]
      },
      manifest: {
        name: 'Legacy Intercessor',
        short_name: 'Intercessor',
        description: 'Prayer request and praise report management for Legacy Sukawarna intercessors.',
        display: 'standalone',
        orientation: 'portrait',
        background_color: '#FFFFFF',
        theme_color: '#000000',
        start_url: '/',
        scope: '/',
        icons: [
          { src: 'icons/icon-192x192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: 'icons/icon-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
          { src: 'icons/icon-maskable-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      metaVariables: {
        appleMobileWebAppCapable: 'yes',
        appleMobileWebAppStatusBarStyle: 'black',
        appleTouchIcon120: 'icons/apple-icon-120x120.png',
        appleTouchIcon180: 'icons/apple-icon-180x180.png',
        msapplicationTileColor: '#000000',
        themeColor: '#000000',
      },
    },

    electron: {},
  }
})
