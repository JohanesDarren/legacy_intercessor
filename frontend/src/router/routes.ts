import type { RouteRecordRaw } from 'vue-router'

/**
 * Routes follow PRD Section 11.1 screen list. `meta.roles` mirrors the
 * server-side RBAC so the UI never shows an action the API would reject.
 */
const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('pages/LoginPage.vue'),
    meta: { publicOnly: true },
  },
  {
    path: '/invite/:token',
    name: 'accept-invite',
    component: () => import('pages/AcceptInvitePage.vue'),
    meta: { publicOnly: true },
  },

  {
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        name: 'dashboard',
        component: () => import('pages/DashboardPage.vue'),
      },
      {
        path: 'prayer',
        name: 'prayer-list',
        component: () => import('pages/PrayerListPage.vue'),
        meta: { roles: ['admin', 'intercessor'] },
      },
      {
        path: 'prayer/new',
        name: 'prayer-new',
        component: () => import('pages/PrayerFormPage.vue'),
        meta: { roles: ['admin', 'intercessor'] },
      },
      {
        path: 'prayer/:id',
        name: 'prayer-detail',
        component: () => import('pages/PrayerDetailPage.vue'),
        meta: { roles: ['admin', 'intercessor'] },
      },
      {
        path: 'prayer/:id/edit',
        name: 'prayer-edit',
        component: () => import('pages/PrayerFormPage.vue'),
        meta: { roles: ['admin', 'intercessor'] },
      },
      {
        path: 'praise',
        name: 'praise-list',
        component: () => import('pages/PraiseListPage.vue'),
        meta: { roles: ['admin', 'intercessor'] },
      },
      {
        path: 'praise/new',
        name: 'praise-new',
        component: () => import('pages/PraiseFormPage.vue'),
        meta: { roles: ['admin', 'intercessor'] },
      },
      {
        path: 'praise/:id/edit',
        name: 'praise-edit',
        component: () => import('pages/PraiseFormPage.vue'),
        meta: { roles: ['admin', 'intercessor'] },
      },
      {
        path: 'summaries',
        name: 'summaries',
        component: () => import('pages/SummariesPage.vue'),
        meta: { roles: ['admin'] },
      },
      {
        path: 'presentations',
        name: 'presentations',
        component: () => import('pages/PresentationsPage.vue'),
      },
      {
        path: 'presentations/:id',
        name: 'presentation-detail',
        component: () => import('pages/PresentationDetailPage.vue'),
      },
      {
        path: 'booth',
        name: 'booth',
        component: () => import('pages/BoothPage.vue'),
        meta: { roles: ['admin', 'intercessor'] },
      },
      {
        path: 'services',
        name: 'services',
        component: () => import('pages/ServicesPage.vue'),
        meta: { roles: ['admin'] },
      },
      {
        path: 'users',
        name: 'users',
        component: () => import('pages/UsersPage.vue'),
        meta: { roles: ['admin'] },
      },
      {
        path: 'settings',
        name: 'settings',
        component: () => import('pages/SettingsPage.vue'),
      },
      {
        path: 'forbidden',
        name: 'forbidden',
        component: () => import('pages/ForbiddenPage.vue'),
      },
    ],
  },

  {
    path: '/:catchAll(.*)*',
    component: () => import('pages/NotFoundPage.vue'),
  },
]

export default routes
