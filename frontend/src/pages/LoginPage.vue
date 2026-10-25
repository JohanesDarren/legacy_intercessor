<template>
  <div class="login-wrap">
    <div class="login-card">
      <div class="row items-center no-wrap q-mb-lg">
        <span class="brand-dot" />
        <div class="q-ml-sm">
          <div class="li-display" style="font-size: 17px; letter-spacing: 3px">LEGACY</div>
          <div class="li-eyebrow" style="font-size: 9px">INTERCESSOR</div>
        </div>
      </div>

      <h1 class="li-display login-title">{{ t('auth.signInTitle') }}</h1>
      <p class="li-muted q-mb-lg" style="font-size: 13.5px; line-height: 1.55">
        {{ t('auth.signInSubtitle') }}
      </p>

      <q-form @submit="submit" greedy>
        <q-input
          v-model="email"
          type="email"
          outlined
          dense
          autocomplete="username"
          :label="t('auth.email')"
          :error="!!errors.email"
          :error-message="errors.email"
          class="q-mb-md"
          :disable="loading"
          @update:model-value="errors.email = ''"
        >
          <template #prepend><q-icon name="mail_outline" size="18px" /></template>
        </q-input>

        <q-input
          v-model="password"
          :type="showPassword ? 'text' : 'password'"
          outlined
          dense
          autocomplete="current-password"
          :label="t('auth.password')"
          :error="!!errors.password"
          :error-message="errors.password"
          :disable="loading"
          @update:model-value="errors.password = ''"
        >
          <template #prepend><q-icon name="lock_outline" size="18px" /></template>
          <template #append>
            <q-btn
              flat
              dense
              round
              :icon="showPassword ? 'visibility_off' : 'visibility'"
              size="sm"
              @click="showPassword = !showPassword"
            />
          </template>
        </q-input>

        <q-banner v-if="formError" dense class="bg-orange-1 text-orange-10 q-mt-md login-error">
          <template #avatar><q-icon name="error_outline" size="18px" /></template>
          {{ formError }}
        </q-banner>

        <q-btn
          type="submit"
          unelevated
          no-caps
          color="primary"
          text-color="white"
          class="full-width q-mt-lg"
          size="md"
          :loading="loading"
          :label="loading ? t('auth.signingIn') : t('auth.signIn')"
        />
      </q-form>

      <div class="li-divider q-my-lg" />

      <div class="row items-center justify-center q-gutter-x-xs li-muted" style="font-size: 11.5px">
        <q-icon name="church" size="13px" />
        <span>{{ t('app.service') }}</span>
      </div>
    </div>

    <p class="li-muted text-center q-mt-lg" style="font-size: 11px">
      {{ t('app.church') }} · v1.0
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { ApiErrorException } from '@/services/api'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const email = ref('')
const password = ref('')
const showPassword = ref(false)
const loading = ref(false)
const formError = ref('')
const errors = reactive({ email: '', password: '' })

function validate(): boolean {
  errors.email = ''
  errors.password = ''

  if (!email.value.trim()) {
    errors.email = t('auth.emailRequired')
  } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.value.trim())) {
    errors.email = t('auth.emailInvalid')
  }
  if (!password.value) {
    errors.password = t('auth.passwordRequired')
  }
  return !errors.email && !errors.password
}

async function submit() {
  formError.value = ''
  if (!validate()) return

  loading.value = true
  try {
    await auth.login(email.value.trim(), password.value)
    const next = (route.query.next as string) || '/'
    router.replace(next)
  } catch (err) {
    if (err instanceof ApiErrorException) {
      switch (err.code) {
        case 'ACCOUNT_LOCKED':
          formError.value = t('auth.locked')
          break
        case 'INVALID_CREDENTIALS':
          formError.value = t('auth.invalidCredentials')
          break
        case 'ACCOUNT_INACTIVE':
          formError.value = t('auth.inactive')
          break
        case 'FORBIDDEN':
          formError.value = t('auth.noPassword')
          break
        default:
          formError.value = err.message
      }
    } else {
      formError.value = t('errors.network')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.login-wrap {
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px 16px calc(24px + env(safe-area-inset-bottom, 0px));
  background: $brand-white;
}

.login-card {
  width: 100%;
  max-width: 400px;
  background: $brand-white;
  border: 1px solid rgba(0, 0, 0, 0.09);
  border-radius: 18px;
  padding: 32px 26px;
}

.brand-dot {
  width: 11px;
  height: 11px;
  border-radius: 50%;
  background: $brand-orange;
  display: inline-block;
}

.login-title {
  font-size: 24px;
  line-height: 1.15;
  margin-bottom: 8px;
  letter-spacing: -0.3px;
}

.login-error {
  border-radius: 9px;
  font-size: 12.5px;
  line-height: 1.5;
}
</style>
