<template>
  <q-page class="li-page">
    <div class="row items-center justify-between q-mb-md">
      <div class="li-eyebrow">{{ t('users.subtitle') }}</div>
      <q-btn
        unelevated
        dense
        no-caps
        color="primary"
        text-color="white"
        icon="person_add"
        :label="t('users.invite')"
        @click="openInvite"
      />
    </div>

    <div v-if="loading" class="column items-center q-py-xl">
      <q-spinner color="primary" size="28px" />
    </div>

    <template v-else>
      <q-card v-for="u in users" :key="u.id" flat class="li-card q-mb-sm">
        <q-card-section class="q-pa-md">
          <div class="row items-center no-wrap">
            <q-avatar size="38px" color="grey-2" text-color="grey-9" style="font-size: 13px">
              {{ initials(u.full_name) }}
            </q-avatar>
            <div class="q-ml-sm" style="flex: 1; min-width: 0">
              <div class="row items-center q-gutter-xs">
                <span style="font-weight: 600; font-size: 13.5px">{{ u.full_name }}</span>
                <span class="li-badge" :class="roleBadge(u.role)">{{ roleLabel(u.role) }}</span>
                <span v-if="!u.is_active" class="li-badge li-badge--muted">
                  {{ t('users.inactive') }}
                </span>
              </div>
              <div class="li-muted li-truncate-1" style="font-size: 11.5px">{{ u.email }}</div>
              <div class="li-muted" style="font-size: 10.5px">
                {{ t('users.lastLogin') }}:
                {{ u.last_login_at ? formatDateTime(u.last_login_at) : t('users.never') }}
              </div>
            </div>
            <q-btn flat dense round icon="more_vert" size="sm">
              <q-menu>
                <q-list style="min-width: 160px">
                  <q-item
                    clickable
                    v-ripple
                    @click="toggleActive(u)"
                  >
                    <q-item-section avatar style="min-width: 28px">
                      <q-icon :name="u.is_active ? 'block' : 'check_circle'" size="17px" />
                    </q-item-section>
                    <q-item-section>
                      {{ u.is_active ? t('users.deactivate') : t('users.activate') }}
                    </q-item-section>
                  </q-item>
                  <q-item clickable v-ripple @click="resendInvite(u)">
                    <q-item-section avatar style="min-width: 28px">
                      <q-icon name="mail" size="17px" />
                    </q-item-section>
                    <q-item-section>{{ t('users.invite') }}</q-item-section>
                  </q-item>
                </q-list>
              </q-menu>
            </q-btn>
          </div>
        </q-card-section>
      </q-card>
    </template>

    <!-- Invite dialog -->
    <q-dialog v-model="showInvite">
      <q-card style="max-width: 460px; width: 92vw; border-radius: 16px">
        <q-card-section class="row items-center justify-between q-pb-sm">
          <div class="li-display" style="font-size: 16px">{{ t('users.invite') }}</div>
          <q-btn flat dense round icon="close" @click="closeInvite" />
        </q-card-section>

        <q-card-section v-if="!inviteLink" class="q-pt-none">
          <q-input
            v-model="inviteForm.email"
            type="email"
            outlined
            dense
            :label="t('users.email')"
            :error="!!inviteErrors.email"
            :error-message="inviteErrors.email"
            class="q-mb-md"
            @update:model-value="inviteErrors.email = ''"
          />
          <q-input
            v-model="inviteForm.full_name"
            outlined
            dense
            :label="t('users.fullName')"
            :error="!!inviteErrors.full_name"
            :error-message="inviteErrors.full_name"
            class="q-mb-md"
            @update:model-value="inviteErrors.full_name = ''"
          />
          <div class="li-eyebrow q-mb-xs">{{ t('users.role') }}</div>
          <q-option-group
            v-model="inviteForm.role"
            :options="roleOptions"
            color="primary"
          />
        </q-card-section>

        <q-card-section v-else class="q-pt-none">
          <div class="row items-center q-gutter-xs q-mb-sm">
            <q-icon name="check_circle" size="18px" color="positive" />
            <span style="font-size: 13px; font-weight: 600">{{ t('users.inviteCreated') }}</span>
          </div>
          <div class="li-muted q-mb-sm" style="font-size: 11.5px; line-height: 1.45">
            {{ t('users.inviteLinkHint') }}
          </div>
          <q-input :model-value="inviteLink" outlined dense readonly type="textarea" autogrow />
        </q-card-section>

        <q-card-actions class="q-pa-md">
          <template v-if="!inviteLink">
            <q-btn flat no-caps :label="t('app.cancel')" @click="closeInvite" />
            <q-space />
            <q-btn
              unelevated
              no-caps
              color="primary"
              text-color="white"
              :label="t('users.invite')"
              :loading="inviting"
              @click="sendInvite"
            />
          </template>
          <template v-else>
            <q-btn flat no-caps :label="t('app.close')" @click="closeInvite" />
            <q-space />
            <q-btn
              unelevated
              no-caps
              color="primary"
              text-color="white"
              icon="content_copy"
              :label="t('users.copyLink')"
              @click="copyLink"
            />
          </template>
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Notify } from 'quasar'
import api, { ApiErrorException } from '@/services/api'
import { errorMessage } from '@/boot/axios'
import { formatDateTime, initials, roleLabel } from '@/utils/format'
import type { User } from '@/services/types'

const { t } = useI18n()

const users = ref<User[]>([])
const loading = ref(true)
const showInvite = ref(false)
const inviting = ref(false)
const inviteLink = ref('')

const inviteForm = reactive({ email: '', full_name: '', role: 'intercessor' })
const inviteErrors = reactive({ email: '', full_name: '' })

const roleOptions = computed(() => [
  { value: 'intercessor', label: t('users.roleIntercessor') },
  { value: 'admin', label: t('users.roleAdmin') },
  { value: 'viewer', label: t('users.roleViewer') },
])

function roleBadge(role: string) {
  if (role === 'admin') return 'li-badge--gold'
  if (role === 'viewer') return 'li-badge--muted'
  return 'li-badge--neutral'
}

function openInvite() {
  inviteForm.email = ''
  inviteForm.full_name = ''
  inviteForm.role = 'intercessor'
  inviteErrors.email = ''
  inviteErrors.full_name = ''
  inviteLink.value = ''
  showInvite.value = true
}

function closeInvite() {
  showInvite.value = false
  inviteLink.value = ''
}

async function sendInvite() {
  inviteErrors.email = ''
  inviteErrors.full_name = ''

  if (!inviteForm.email.trim()) {
    inviteErrors.email = t('app.required')
    return
  }
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(inviteForm.email.trim())) {
    inviteErrors.email = t('auth.emailInvalid')
    return
  }
  if (!inviteForm.full_name.trim()) {
    inviteErrors.full_name = t('app.required')
    return
  }

  inviting.value = true
  try {
    const inv = await api.createInvite(
      inviteForm.email.trim(),
      inviteForm.full_name.trim(),
      inviteForm.role,
    )
    inviteLink.value = `${window.location.origin}/#/invite/${inv.invite_token}`
    await load()
  } catch (err) {
    if (err instanceof ApiErrorException) {
      if (err.code === 'CONFLICT') inviteErrors.email = t('users.emailExists')
      else if (err.fields) {
        for (const [f, m] of Object.entries(err.fields)) {
          if (f in inviteErrors) (inviteErrors as Record<string, string>)[f] = m[0] ?? ''
        }
      } else {
        Notify.create({ type: 'negative', message: err.message })
      }
    } else {
      Notify.create({ type: 'negative', message: errorMessage(err) })
    }
  } finally {
    inviting.value = false
  }
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(inviteLink.value)
    Notify.create({ type: 'positive', message: t('app.copied'), timeout: 1600 })
  } catch {
    Notify.create({ type: 'warning', message: t('users.inviteLink') })
  }
}

async function toggleActive(u: User) {
  try {
    await api.setUserActive(u.id, !u.is_active)
    await load()
    Notify.create({ type: 'positive', message: t('app.saved'), timeout: 1500 })
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  }
}

function resendInvite(u: User) {
  openInvite()
  inviteForm.email = u.email
  inviteForm.full_name = u.full_name
  inviteForm.role = u.role
}

async function load() {
  loading.value = true
  try {
    users.value = await api.listUsers()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
    users.value = []
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped lang="scss">
.li-truncate-1 {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
