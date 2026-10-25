<template>
  <q-page class="li-page">
    <!-- Profile -->
    <q-card flat class="li-card q-mb-md">
      <q-card-section class="q-pa-md">
        <div class="row items-center no-wrap">
          <q-avatar size="46px" color="grey-2" text-color="grey-9" style="font-size: 15px">
            {{ initials(auth.user?.full_name) }}
          </q-avatar>
          <div class="q-ml-md" style="flex: 1; min-width: 0">
            <div style="font-weight: 700; font-size: 15px">{{ auth.user?.full_name }}</div>
            <div class="li-muted" style="font-size: 12px">{{ auth.user?.email }}</div>
            <div class="li-badge li-badge--gold q-mt-xs">{{ roleLabel(auth.user?.role ?? '') }}</div>
          </div>
        </div>
      </q-card-section>
    </q-card>

    <!-- Language -->
    <q-card flat class="li-card q-mb-md">
      <q-card-section class="q-pa-md">
        <div class="li-eyebrow q-mb-sm">{{ t('settings.language') }}</div>
        <q-btn-toggle
          v-model="locale"
          :options="localeOptions"
          unelevated
          no-caps
          toggle-color="primary"
          color="grey-2"
          text-color="grey-9"
          class="full-width"
          @update:model-value="changeLocale"
        />
      </q-card-section>
    </q-card>

    <!-- Change password -->
    <q-card flat class="li-card q-mb-md">
      <q-card-section class="q-pa-md">
        <div class="li-eyebrow q-mb-sm">{{ t('settings.changePassword') }}</div>
        <q-input
          v-model="pw.current"
          type="password"
          outlined
          dense
          :label="t('settings.currentPassword')"
          class="q-mb-sm"
        />
        <q-input
          v-model="pw.next"
          type="password"
          outlined
          dense
          :label="t('settings.newPassword')"
          :hint="t('auth.passwordHint')"
          :error="!!pwError"
          :error-message="pwError"
          class="q-mb-sm"
          @update:model-value="pwError = ''"
        />
        <q-btn
          unelevated
          no-caps
          color="primary"
          text-color="white"
          :label="t('app.save')"
          size="sm"
          :loading="savingPw"
          @click="changePassword"
        />
      </q-card-section>
    </q-card>

    <!-- Privacy and data (UU PDP) -->
    <q-card flat class="li-card q-mb-md">
      <q-card-section class="q-pa-md">
        <div class="li-eyebrow q-mb-sm">{{ t('settings.privacyNotice') }}</div>
        <div class="li-muted" style="font-size: 11.5px; line-height: 1.5">
          {{ t('auth.privacyBody') }}
        </div>
        <div class="row items-center q-gutter-xs q-mt-sm">
          <q-icon
            :name="auth.user?.privacy_ack_at ? 'check_circle' : 'schedule'"
            size="14px"
            :color="auth.user?.privacy_ack_at ? 'positive' : 'grey'"
          />
          <span class="li-muted" style="font-size: 11px">
            {{ auth.user?.privacy_ack_at ? t('settings.privacyAcknowledged') : t('auth.privacyAck') }}
          </span>
        </div>
      </q-card-section>
    </q-card>

    <q-card flat class="li-card q-mb-md">
      <q-list>
        <q-item clickable v-ripple @click="exportData">
          <q-item-section avatar style="min-width: 36px">
            <q-icon name="download" size="19px" />
          </q-item-section>
          <q-item-section>
            <q-item__label style="font-size: 13px">{{ t('settings.exportData') }}</q-item-label>
            <q-item__label caption style="font-size: 11px">
              {{ t('settings.exportDataHint') }}
            </q-item__label>
          </q-item-section>
        </q-item>
        <q-separator />
        <q-item clickable v-ripple @click="confirmDelete">
          <q-item-section avatar style="min-width: 36px">
            <q-icon name="delete_outline" size="19px" color="negative" />
          </q-item-section>
          <q-item-section>
            <q-item__label style="font-size: 13px; color: #c10015">
              {{ t('settings.deleteAccount') }}
            </q-item__label>
            <q-item__label caption style="font-size: 11px">
              {{ t('settings.deleteAccountHint') }}
            </q-item__label>
          </q-item-section>
        </q-item>
      </q-list>
    </q-card>

    <!-- Install PWA -->
    <q-card flat class="li-card q-mb-md">
      <q-card-section class="q-pa-md">
        <div class="row items-center justify-between no-wrap">
          <div style="flex: 1; min-width: 0">
            <div class="li-eyebrow">{{ t('settings.installApp') }}</div>
            <div class="li-muted q-mt-xs" style="font-size: 11.5px; line-height: 1.45">
              {{ isIos ? t('settings.installIos') : t('settings.installHint') }}
            </div>
          </div>
          <q-btn
            v-if="canInstall"
            outline
            dense
            no-caps
            color="primary"
            :label="t('settings.installApp')"
            size="sm"
            @click="install"
          />
        </div>
      </q-card-section>
    </q-card>

    <!-- About / system -->
    <q-card flat class="li-card q-mb-md">
      <q-card-section class="q-pa-md">
        <div class="li-eyebrow q-mb-sm">{{ t('settings.about') }}</div>
        <div class="li-muted" style="font-size: 11.5px; line-height: 1.55">
          {{ t('settings.aboutBody') }}
        </div>
        <div class="li-divider q-my-md" />
        <div class="row justify-between" style="font-size: 11px">
          <span class="li-muted">{{ t('settings.appVersion') }}</span>
          <span class="li-tabular">v1.0.0</span>
        </div>
        <div v-if="meta" class="row justify-between q-mt-xs" style="font-size: 11px">
          <span class="li-muted">{{ t('settings.aiProvider') }}</span>
          <span class="li-tabular">
            {{ meta.ai_provider }}
            <span v-if="meta.offline_mode">({{ t('settings.offlineMode') }})</span>
          </span>
        </div>
        <div v-if="meta" class="row justify-between q-mt-xs" style="font-size: 11px">
          <span class="li-muted">API</span>
          <span class="li-tabular">{{ meta.api_version }} · {{ meta.environment }}</span>
        </div>
      </q-card-section>
    </q-card>

    <q-btn
      flat
      no-caps
      color="negative"
      icon="logout"
      :label="t('auth.signOut')"
      class="full-width q-mt-sm"
      @click="signOut"
    />
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Notify, Dialog } from 'quasar'
import api, { ApiErrorException } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { errorMessage } from '@/boot/axios'
import { initials, roleLabel } from '@/utils/format'
import { setLocale } from '@/i18n'
import type { MetaVersion } from '@/services/types'

const { t, locale } = useI18n()
const router = useRouter()
const auth = useAuthStore()

const meta = ref<MetaVersion | null>(null)
const savingPw = ref(false)
const pwError = ref('')
const pw = reactive({ current: '', next: '' })

const localeOptions = [
  { label: 'Bahasa Indonesia', value: 'id-ID' },
  { label: 'English', value: 'en-US' },
]

const isIos = computed(
  () => /iPad|iPhone|iPod/.test(navigator.userAgent) && !('MSStream' in window),
)

const canInstall = ref(false)
let deferredPrompt: Event | null = null

function changeLocale(value: string | number | boolean | null) {
  if (typeof value === 'string') {
    setLocale(value)
  }
}

async function changePassword() {
  pwError.value = ''
  if (pw.next.length < 10) {
    pwError.value = t('auth.passwordTooShort')
    return
  }
  savingPw.value = true
  try {
    await api.updateMe({ current_password: pw.current, new_password: pw.next })
    pw.current = ''
    pw.next = ''
    Notify.create({ type: 'positive', message: t('settings.passwordChanged'), timeout: 2000 })
  } catch (err) {
    if (err instanceof ApiErrorException) pwError.value = err.message
    else pwError.value = errorMessage(err)
  } finally {
    savingPw.value = false
  }
}

async function exportData() {
  try {
    const data = await api.exportMyData()
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'legacy-intercessor-my-data.json'
    a.click()
    URL.revokeObjectURL(url)
    Notify.create({ type: 'positive', message: t('settings.exportStarted'), timeout: 2000 })
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  }
}

function confirmDelete() {
  Dialog.create({
    title: t('settings.deleteAccount'),
    message: t('settings.deleteConfirm'),
    cancel: true,
    persistent: true,
    ok: { label: t('app.confirm'), color: 'negative', unelevated: true, noCaps: true },
  }).onOk(async () => {
    try {
      await api.deleteMe()
      await auth.logout()
      router.replace({ name: 'login' })
    } catch (err) {
      Notify.create({ type: 'negative', message: errorMessage(err) })
    }
  })
}

async function install() {
  if (!deferredPrompt) return
  ;(deferredPrompt as Event & { prompt?: () => Promise<void> }).prompt?.()
  deferredPrompt = null
  canInstall.value = false
}

async function signOut() {
  await auth.logout()
  router.replace({ name: 'login' })
}

onMounted(async () => {
  try {
    meta.value = await api.metaVersion()
  } catch {
    meta.value = null
  }

  window.addEventListener('beforeinstallprompt', (e) => {
    e.preventDefault()
    deferredPrompt = e
    canInstall.value = true
  })
})
</script>
