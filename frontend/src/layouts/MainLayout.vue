<template>
  <q-layout view="hHh Lpr lFf">
    <!-- Desktop: permanent drawer -->
    <q-drawer
      v-if="!isMobile"
      v-model="drawer"
      show-if-above
      :width="248"
      class="li-drawer"
      bordered
    >
      <q-list padding class="q-py-md">
        <q-item class="q-mb-lg">
          <q-item-section>
            <div class="row items-center no-wrap">
              <span class="li-brand-dot" />
              <div class="q-ml-sm">
                <div class="li-display" style="font-size: 15px; letter-spacing: 2px">
                  LEGACY
                </div>
                <div class="li-eyebrow" style="font-size: 9px">INTERCESSOR</div>
              </div>
            </div>
          </q-item-section>
        </q-item>

        <q-item
          v-for="item in visibleNav"
          :key="item.name"
          :to="item.to"
          clickable
          v-ripple
          :exact="item.exact"
          active-class="li-nav-active"
          class="li-nav-item"
        >
          <q-item-section avatar style="min-width: 34px">
            <q-icon :name="item.icon" size="20px" />
          </q-item-section>
          <q-item-section>{{ t(item.label) }}</q-item-section>
          <q-item-section v-if="item.badge" side>
            <q-badge color="accent" text-color="black" :label="item.badge" />
          </q-item-section>
        </q-item>
      </q-list>

      <div class="absolute-bottom">
        <q-list padding>
          <q-separator class="q-mb-sm" />
          <q-item clickable v-ripple @click="signOut">
            <q-item-section avatar style="min-width: 34px">
              <q-icon name="logout" size="20px" />
            </q-item-section>
            <q-item-section>{{ t('auth.signOut') }}</q-item-section>
          </q-item>
        </q-list>
      </div>
    </q-drawer>

    <!-- Header -->
    <q-header class="li-header" :elevated="!isMobile">
      <q-toolbar class="q-px-md">
        <q-btn
          v-if="!isMobile"
          flat
          dense
          round
          icon="menu"
          @click="drawer = !drawer"
          class="q-mr-sm"
        />
        <q-toolbar-title class="row items-center no-wrap">
          <span class="li-brand-dot q-mr-sm" v-if="isMobile" />
          <span class="li-display" style="font-size: 16px; letter-spacing: 1.5px">
            {{ pageTitle }}
          </span>
        </q-toolbar-title>

        <q-btn
          flat
          dense
          round
          icon="notifications_none"
          :to="{ name: 'dashboard' }"
          class="q-mr-xs"
        >
          <q-badge
            v-if="unread > 0"
            color="accent"
            text-color="black"
            floating
            rounded
            :label="unread > 9 ? '9+' : unread"
          />
        </q-btn>
        <q-btn flat dense round icon="settings" :to="{ name: 'settings' }" />
      </q-toolbar>
    </q-header>

    <q-page-container>
      <router-view v-slot="{ Component }">
        <component :is="Component" />
      </router-view>
    </q-page-container>

    <!-- Mobile: bottom navigation within thumb reach -->
    <q-footer v-if="isMobile" class="li-bottom-bar">
      <div class="row no-wrap items-center justify-around">
        <button
          v-for="item in mobileNav"
          :key="item.name"
          class="li-tab"
          :class="{ 'li-tab--active': isActive(item) }"
          @click="router.push(item.to)"
        >
          <q-icon :name="item.icon" size="22px" />
          <span class="li-tab__label">{{ t(item.label) }}</span>
          <span v-if="item.name === 'dashboard' && unread > 0" class="li-tab__dot" />
        </button>
        <button class="li-tab" @click="showMore = true">
          <q-icon name="more_horiz" size="22px" />
          <span class="li-tab__label">{{ t('nav.more') }}</span>
        </button>
      </div>
    </q-footer>

    <!-- Mobile: overflow menu -->
    <q-dialog v-model="showMore" position="bottom">
      <q-card style="width: 100%; border-radius: 18px 18px 0 0">
        <q-list padding>
          <q-item
            v-for="item in moreNav"
            :key="item.name"
            clickable
            v-ripple
            @click="go(item.to)"
          >
            <q-item-section avatar>
              <q-icon :name="item.icon" />
            </q-item-section>
            <q-item-section>{{ t(item.label) }}</q-item-section>
          </q-item>
          <q-separator class="q-my-sm" />
          <q-item clickable v-ripple @click="signOut">
            <q-item-section avatar><q-icon name="logout" /></q-item-section>
            <q-item-section>{{ t('auth.signOut') }}</q-item-section>
          </q-item>
        </q-list>
      </q-card>
    </q-dialog>

    <!-- First-login privacy notice (PRD 10.5, not pre-checked) -->
    <q-dialog :model-value="showPrivacy" persistent @update:model-value="onPrivacyToggle">
      <q-card style="max-width: 520px; width: 92vw; border-radius: 16px">
        <q-card-section>
          <div class="li-display" style="font-size: 20px">
            {{ t('auth.privacyTitle') }}
          </div>
        </q-card-section>
        <q-card-section class="q-pt-none">
          <p class="li-muted" style="line-height: 1.6">{{ t('auth.privacyBody') }}</p>
        </q-card-section>
        <q-card-actions align="right" class="q-pa-md">
          <q-btn
            unelevated
            color="primary"
            text-color="white"
            no-caps
            :label="t('auth.privacyAck')"
            :loading="acking"
            @click="acknowledge"
          />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-layout>
</template>

<script setup lang="ts">
import { computed, ref, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useQuasar, Notify } from 'quasar'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { errorMessage } from '@/boot/axios'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const $q = useQuasar()
const auth = useAuthStore()
const app = useAppStore()

const drawer = ref(true)
const showMore = ref(false)
const acking = ref(false)

const isMobile = computed(() => $q.screen.lt.md)
const unread = computed(() => app.dashboard?.unread_notifications ?? 0)

interface NavItem {
  name: string
  label: string
  icon: string
  to: { name: string }
  roles?: string[]
  exact?: boolean
  badge?: number
}

const navItems = computed<NavItem[]>(() => [
  { name: 'dashboard', label: 'nav.dashboard', icon: 'space_dashboard', to: { name: 'dashboard' }, exact: true },
  { name: 'prayer', label: 'nav.prayer', icon: 'volunteer_activism', to: { name: 'prayer-list' }, roles: ['admin', 'intercessor'] },
  { name: 'praise', label: 'nav.praise', icon: 'auto_awesome', to: { name: 'praise-list' }, roles: ['admin', 'intercessor'] },
  { name: 'summaries', label: 'nav.summaries', icon: 'summarize', to: { name: 'summaries' }, roles: ['admin'] },
  { name: 'presentations', label: 'nav.presentations', icon: 'slideshow', to: { name: 'presentations' } },
  { name: 'booth', label: 'nav.booth', icon: 'event_seat', to: { name: 'booth' }, roles: ['admin', 'intercessor'] },
  { name: 'services', label: 'nav.services', icon: 'church', to: { name: 'services' }, roles: ['admin'] },
  { name: 'users', label: 'nav.users', icon: 'group', to: { name: 'users' }, roles: ['admin'] },
  { name: 'settings', label: 'nav.settings', icon: 'settings', to: { name: 'settings' } },
])

const visibleNav = computed(() =>
  navItems.value.filter((i) => !i.roles || (auth.role && i.roles.includes(auth.role))),
)

/** The four primary destinations that fit in the mobile tab bar. */
const mobileNav = computed(() =>
  visibleNav.value.filter((i) =>
    ['dashboard', 'prayer', 'presentations', 'booth'].includes(i.name),
  ),
)

const moreNav = computed(() =>
  visibleNav.value.filter(
    (i) => !['dashboard', 'prayer', 'presentations', 'booth'].includes(i.name),
  ),
)

const pageTitle = computed(() => {
  const name = route.name as string
  const map: Record<string, string> = {
    dashboard: 'nav.dashboard',
    'prayer-list': 'nav.prayer',
    'prayer-new': 'prayer.new',
    'prayer-detail': 'prayer.detail',
    'prayer-edit': 'prayer.edit',
    'praise-list': 'nav.praise',
    'praise-new': 'praise.new',
    'praise-edit': 'praise.edit',
    summaries: 'nav.summaries',
    presentations: 'nav.presentations',
    'presentation-detail': 'nav.presentations',
    booth: 'nav.booth',
    services: 'nav.services',
    users: 'nav.users',
    settings: 'nav.settings',
    forbidden: 'errors.forbidden',
  }
  return t(map[name] ?? 'app.name')
})

// The dialog is controlled by the store, not by a local flag, so acknowledging
// it actually closes it. A writable computed with an empty setter silently
// swallowed the close event.
const showPrivacy = computed(() => auth.needsPrivacyAck)

/** Quasar asks to close the dialog; only the ack action may close it. */
function onPrivacyToggle() {
  // Intentionally ignored: the notice is persistent until acknowledged.
}

function isActive(item: NavItem) {
  if (item.exact) return route.name === item.name
  return route.matched.some((m) => m.name === item.to.name) || route.name === item.name
}

function go(to: { name: string }) {
  showMore.value = false
  router.push(to)
}

async function acknowledge() {
  acking.value = true
  try {
    await auth.acknowledgePrivacy()
    Notify.create({ type: 'positive', message: t('app.saved'), timeout: 1800 })
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  } finally {
    acking.value = false
  }
}

async function signOut() {
  await auth.logout()
  router.replace({ name: 'login' })
}

watch(isMobile, (mobile) => {
  drawer.value = !mobile
})

onMounted(async () => {
  drawer.value = !isMobile.value
  try {
    await app.bootstrap()
    await app.loadDashboard()
  } catch {
    /* the dashboard is optional chrome; pages load their own data */
  }
})
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.li-brand-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: $brand-orange;
}

.li-header {
  background: $brand-black;
  color: $brand-white;
}

.li-drawer {
  background: $brand-white;
}

.li-nav-item {
  border-radius: 9px;
  margin: 2px 10px;
  min-height: 44px;
  color: rgba(0, 0, 0, 0.76);
  font-size: 13.5px;
}

.li-nav-active {
  background: rgba(0, 0, 0, 0.06);
  color: $brand-black;
  font-weight: 600;

  :deep(.q-icon) {
    color: $identity-gold;
  }
}

.li-bottom-bar {
  background: rgba(255, 255, 255, 0.97);
  backdrop-filter: blur(14px);
  border-top: 1px solid rgba(0, 0, 0, 0.09);
  padding-bottom: env(safe-area-inset-bottom, 0px);
}

.li-tab {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
  padding: 9px 2px 8px;
  min-height: 52px;
  background: transparent;
  border: 0;
  color: rgba(0, 0, 0, 0.48);
  position: relative;
  cursor: pointer;
  -webkit-tap-highlight-color: transparent;
}

.li-tab--active {
  color: $brand-black;

  .li-tab__label {
    font-weight: 700;
  }

  :deep(.q-icon) {
    color: $identity-gold;
  }
}

.li-tab__label {
  font-size: 9.5px;
  letter-spacing: 0.4px;
  text-transform: uppercase;
  line-height: 1;
}

.li-tab__dot {
  position: absolute;
  top: 7px;
  right: 50%;
  margin-right: -14px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: $brand-orange;
}
</style>
