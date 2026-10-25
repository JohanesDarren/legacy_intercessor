<template>
  <div class="invite-wrap">
    <div class="invite-card">
      <div class="row items-center no-wrap q-mb-lg">
        <span class="brand-dot" />
        <div class="q-ml-sm">
          <div class="li-display" style="font-size: 17px; letter-spacing: 3px">LEGACY</div>
          <div class="li-eyebrow" style="font-size: 9px">INTERCESSOR</div>
        </div>
      </div>

      <!-- Loading the invite -->
      <div v-if="loading" class="column items-center q-py-xl">
        <q-spinner color="primary" size="32px" />
        <p class="li-muted q-mt-md" style="font-size: 13px">{{ t('app.loading') }}</p>
      </div>

      <!-- Invalid / expired invite -->
      <div v-else-if="inviteError" class="column items-center q-py-lg text-center">
        <q-icon name="link_off" size="40px" class="li-muted q-mb-md" />
        <div class="li-display q-mb-sm" style="font-size: 18px">
          {{ t('auth.inviteInvalid') }}
        </div>
        <p class="li-muted" style="font-size: 13px; line-height: 1.55">{{ inviteError }}</p>
        <q-btn
          flat
          no-caps
          color="primary"
          class="q-mt-lg"
          :label="t('auth.signIn')"
          :to="{ name: 'login' }"
        />
      </div>

      <!-- Valid invite -->
      <div v-else>
        <h1 class="li-display invite-title">{{ t('auth.acceptInviteTitle') }}</h1>
        <p class="li-muted q-mb-md" style="font-size: 13.5px; line-height: 1.55">
          {{ t('auth.acceptInviteSubtitle') }}
        </p>

        <div class="invite-identity q-mb-lg">
          <div class="li-eyebrow">{{ t('auth.email') }}</div>
          <div style="font-weight: 600; font-size: 14px">{{ preview?.email }}</div>
          <div class="li-eyebrow q-mt-sm">{{ t('users.role') }}</div>
          <div style="font-size: 13px">{{ roleLabel(preview?.role ?? '') }}</div>
        </div>

        <q-form @submit="submit" greedy>
          <q-input
            v-model="fullName"
            outlined
            dense
            :label="`${t('auth.fullName')} (${t('app.optional')})`"
            class="q-mb-md"
            :disable="submitting"
          />

          <q-input
            v-model="password"
            :type="showPassword ? 'text' : 'password'"
            outlined
            dense
            :label="t('auth.newPassword')"
            :hint="t('auth.passwordHint')"
            :error="!!errors.password"
            :error-message="errors.password"
            :disable="submitting"
            class="q-mb-md"
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

          <q-input
            v-model="confirm"
            :type="showPassword ? 'text' : 'password'"
            outlined
            dense
            :label="t('auth.confirmPassword')"
            :error="!!errors.confirm"
            :error-message="errors.confirm"
            :disable="submitting"
            @update:model-value="errors.confirm = ''"
          >
            <template #prepend><q-icon name="lock_outline" size="18px" /></template>
          </q-input>

          <q-banner v-if="formError" dense class="bg-orange-1 text-orange-10 q-mt-md invite-error">
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
            :loading="submitting"
            :label="t('auth.activate')"
          />
        </q-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import api, { ApiErrorException } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import type { InvitePreview } from '@/services/types'
import { roleLabel } from '@/utils/format'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const token = route.params.token as string
const preview = ref<InvitePreview | null>(null)
const loading = ref(true)
const submitting = ref(false)
const inviteError = ref('')
const formError = ref('')

const fullName = ref('')
const password = ref('')
const confirm = ref('')
const showPassword = ref(false)
const errors = reactive({ password: '', confirm: '' })

onMounted(async () => {
  try {
    preview.value = await api.previewInvite(token)
    fullName.value = preview.value.full_name || ''
  } catch (err) {
    if (err instanceof ApiErrorException) {
      if (err.code === 'INVITE_EXPIRED') inviteError.value = t('auth.inviteExpired')
      else if (err.code === 'INVITE_ALREADY_USED') inviteError.value = t('auth.inviteUsed')
      else inviteError.value = err.message
    } else {
      inviteError.value = t('errors.network')
    }
  } finally {
    loading.value = false
  }
})

function validate(): boolean {
  errors.password = ''
  errors.confirm = ''

  if (password.value.length < 10) {
    errors.password = t('auth.passwordTooShort')
  }
  if (password.value !== confirm.value) {
    errors.confirm = t('auth.passwordMismatch')
  }
  return !errors.password && !errors.confirm
}

async function submit() {
  formError.value = ''
  if (!validate()) return

  submitting.value = true
  try {
    await auth.acceptInvite(token, password.value, fullName.value.trim() || undefined)
    router.replace({ name: 'dashboard' })
  } catch (err) {
    formError.value =
      err instanceof ApiErrorException ? err.message : t('errors.network')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.invite-wrap {
  min-height: 100vh;
  min-height: 100dvh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px 16px calc(24px + env(safe-area-inset-bottom, 0px));
  background: $brand-white;
}

.invite-card {
  width: 100%;
  max-width: 400px;
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

.invite-title {
  font-size: 22px;
  line-height: 1.18;
  margin-bottom: 8px;
}

.invite-identity {
  background: rgba(0, 0, 0, 0.035);
  border-radius: 10px;
  padding: 12px 14px;
}

.invite-error {
  border-radius: 9px;
  font-size: 12.5px;
  line-height: 1.5;
}
</style>
