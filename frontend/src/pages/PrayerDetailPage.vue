<template>
  <q-page class="li-page">
    <div v-if="loading" class="column items-center q-py-xl">
      <q-spinner color="primary" size="28px" />
    </div>

    <template v-else-if="record">
      <!-- Header -->
      <div class="row items-start justify-between no-wrap q-mb-md">
        <div style="min-width: 0; flex: 1">
          <div class="row items-center q-gutter-xs q-mb-xs">
            <span class="li-eyebrow">{{ record.category_name }}</span>
            <span class="li-eyebrow" style="opacity: 0.32">·</span>
            <span class="li-eyebrow">{{ formatDate(record.received_at) }}</span>
          </div>
          <h1 class="li-display" style="font-size: 20px; line-height: 1.2">
            {{ record.title }}
          </h1>
        </div>
        <q-btn flat dense round icon="edit" :to="{ name: 'prayer-edit', params: { id: record.id } }" />
      </div>

      <!-- Badges -->
      <div class="row items-center q-gutter-xs q-mb-md">
        <UrgentBadge :is-urgent="record.is_urgent" />
        <span class="li-badge li-badge--gold">{{ statusLabel(record.status) }}</span>
        <span class="li-badge li-badge--neutral">
          <q-icon name="visibility" size="11px" />
          {{ consentLabel(record.display_consent) }}
        </span>
      </div>

      <!-- Content -->
      <q-card flat class="li-card q-mb-md">
        <q-card-section class="q-pa-md">
          <div class="li-eyebrow q-mb-xs">{{ t('prayer.content') }}</div>
          <p style="font-size: 14px; line-height: 1.62; white-space: pre-wrap">
            {{ record.content }}
          </p>

          <div class="li-divider q-my-md" />

          <div class="row q-col-gutter-sm">
            <div class="col-6">
              <div class="li-eyebrow">{{ t('prayer.requesterName') }}</div>
              <div style="font-size: 13px">{{ record.requester_name || '—' }}</div>
            </div>
            <div class="col-6">
              <div class="li-eyebrow">{{ t('prayer.gender') }}</div>
              <div style="font-size: 13px">{{ genderText(record.requester_gender) }}</div>
            </div>
          </div>
        </q-card-section>
      </q-card>

      <!-- Follow-up section (internal only) -->
      <div class="row items-center justify-between q-mb-sm">
        <div>
          <div class="li-eyebrow" style="opacity: 0.75">{{ t('prayer.followUps') }}</div>
          <div class="li-muted" style="font-size: 10.5px">
            <q-icon name="lock" size="10px" class="q-mr-xs" />
            {{ t('followUp.internalOnly') }}
          </div>
        </div>
        <q-btn
          unelevated
          dense
          no-caps
          color="primary"
          text-color="white"
          icon="add"
          size="sm"
          :label="t('prayer.logFollowUp')"
          @click="openFollowUpForm"
        />
      </div>

      <div v-if="followUps.length === 0" class="empty-followup">
        <q-icon name="history_toggle_off" size="24px" class="li-muted" />
        <p class="li-muted q-mt-sm" style="font-size: 12.5px">{{ t('prayer.noFollowUps') }}</p>
      </div>

      <!-- Timeline: newest first -->
      <div v-else class="timeline">
        <div v-for="(fu, idx) in followUps" :key="fu.id" class="timeline__item">
          <div class="timeline__dot" :class="dotClass(fu.condition_status)" />
          <div v-if="idx < followUps.length - 1" class="timeline__line" />

          <div class="timeline__body">
            <div class="row items-center justify-between no-wrap">
              <div class="row items-center q-gutter-xs">
                <span class="li-badge" :class="conditionBadge(fu.condition_status)">
                  {{ conditionLabel(fu.condition_status) }}
                </span>
                <span class="li-eyebrow">{{ channelLabel(fu.channel) }}</span>
              </div>
              <span class="li-muted li-tabular" style="font-size: 10.5px">
                {{ formatDate(fu.contact_date) }}
              </span>
            </div>

            <div class="li-eyebrow q-mt-sm" style="opacity: 0.45">
              {{ updateTypeLabel(fu.update_type) }}
            </div>
            <p class="q-mt-xs" style="font-size: 13px; line-height: 1.55; white-space: pre-wrap">
              {{ fu.notes }}
            </p>

            <div v-if="fu.next_action" class="next-action q-mt-sm">
              <q-icon name="arrow_forward" size="12px" class="q-mr-xs" />
              <span style="font-size: 11.5px">{{ fu.next_action }}</span>
              <span v-if="fu.next_action_date" class="li-muted" style="font-size: 11px">
                · {{ formatDate(fu.next_action_date) }}
              </span>
            </div>

            <div class="li-muted q-mt-xs" style="font-size: 10px">
              {{ fu.created_by_name || '—' }} · {{ formatDateTime(fu.created_at) }}
            </div>

            <!-- Answered -> create praise draft (PRD F-04 / US-17) -->
            <q-btn
              v-if="fu.condition_status === 'answered'"
              flat
              dense
              no-caps
              size="sm"
              color="secondary"
              icon="auto_awesome"
              :label="t('praise.createFromFollowUp')"
              class="q-mt-sm"
              @click="createPraiseFromFollowUp(fu)"
            />
          </div>
        </div>
      </div>

      <!-- Delete (admin only) -->
      <div v-if="auth.isAdmin" class="q-mt-lg">
        <q-btn
          flat
          no-caps
          color="negative"
          icon="delete_outline"
          :label="t('app.delete')"
          size="sm"
          @click="confirmDelete"
        />
      </div>

      <div style="height: 60px" />
    </template>

    <!-- Follow-up form dialog -->
    <q-dialog v-model="showFollowUp" position="bottom" persistent>
      <q-card class="followup-sheet">
        <q-card-section class="row items-center justify-between q-pb-sm">
          <div class="li-display" style="font-size: 16px">{{ t('followUp.title') }}</div>
          <q-btn flat dense round icon="close" @click="showFollowUp = false" />
        </q-card-section>

        <q-card-section class="q-pt-none scroll" style="max-height: 68vh">
          <q-banner dense class="followup-note q-mb-md">
            <template #avatar><q-icon name="lock" size="15px" /></template>
            <span style="font-size: 11px">{{ t('followUp.internalOnly') }}</span>
          </q-banner>

          <div class="row q-col-gutter-sm q-mb-md">
            <div class="col-6">
              <q-input v-model="fuForm.contact_date" type="date" outlined dense :label="t('followUp.contactDate')" />
            </div>
            <div class="col-6">
              <q-select
                v-model="fuForm.channel"
                :options="channelOptions"
                emit-value
                map-options
                outlined
                dense
                :label="t('followUp.channel')"
              />
            </div>
          </div>

          <q-select
            v-model="fuForm.update_type"
            :options="updateTypeOptions"
            emit-value
            map-options
            outlined
            dense
            :label="t('followUp.updateType')"
            class="q-mb-md"
          />

          <div class="li-eyebrow q-mb-xs">{{ t('followUp.condition') }}</div>
          <div class="row q-gutter-xs q-mb-md">
            <q-chip
              v-for="opt in conditionOptions"
              :key="opt.value"
              :selected="fuForm.condition_status === opt.value"
              clickable
              dense
              square
              class="cond-chip"
              :class="{ 'cond-chip--on': fuForm.condition_status === opt.value }"
              @click="fuForm.condition_status = opt.value"
            >
              {{ opt.label }}
            </q-chip>
          </div>

          <q-input
            v-model="fuForm.notes"
            type="textarea"
            outlined
            autogrow
            :label="t('followUp.notes')"
            :hint="t('followUp.notesHint')"
            :error="!!fuErrors.notes"
            :error-message="fuErrors.notes"
            input-style="min-height: 80px"
            class="q-mb-md"
            @update:model-value="fuErrors.notes = ''"
          />

          <q-input
            v-model="fuForm.next_action"
            outlined
            dense
            :label="`${t('followUp.nextAction')} (${t('app.optional')})`"
            maxlength="255"
            class="q-mb-md"
          />
          <q-input
            v-model="fuForm.next_action_date"
            type="date"
            outlined
            dense
            :label="`${t('followUp.nextActionDate')} (${t('app.optional')})`"
          />
        </q-card-section>

        <q-card-actions class="q-pa-md">
          <q-btn flat no-caps :label="t('app.cancel')" @click="showFollowUp = false" />
          <q-space />
          <q-btn
            unelevated
            no-caps
            color="primary"
            text-color="white"
            :label="t('app.save')"
            :loading="savingFollowUp"
            @click="saveFollowUp"
          />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Notify, Dialog } from 'quasar'
import UrgentBadge from '@/components/UrgentBadge.vue'
import api from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { errorMessage } from '@/boot/axios'
import {
  channelLabel,
  conditionLabel,
  consentLabel,
  formatDate,
  formatDateTime,
  statusLabel,
  updateTypeLabel,
  toDateInput,
} from '@/utils/format'
import type { FollowUp, PrayerRequest } from '@/services/types'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const recordId = route.params.id as string
const record = ref<PrayerRequest | null>(null)
const followUps = ref<FollowUp[]>([])
const loading = ref(true)

const showFollowUp = ref(false)
const savingFollowUp = ref(false)
const fuErrors = reactive({ notes: '' })

const fuForm = reactive({
  contact_date: toDateInput(new Date().toISOString()),
  channel: 'visit',
  update_type: 'current_condition',
  condition_status: 'stable',
  notes: '',
  next_action: '',
  next_action_date: '',
})

const channelOptions = computed(() => [
  { value: 'visit', label: t('followUp.channelVisit') },
  { value: 'phone', label: t('followUp.channelPhone') },
  { value: 'whatsapp', label: t('followUp.channelWhatsapp') },
  { value: 'other', label: t('followUp.channelOther') },
])

const updateTypeOptions = computed(() => [
  { value: 'current_condition', label: t('followUp.typeCurrentCondition') },
  { value: 'life_update', label: t('followUp.typeLifeUpdate') },
  { value: 'documentation', label: t('followUp.typeDocumentation') },
  { value: 'other', label: t('followUp.typeOther') },
])

const conditionOptions = computed(() => [
  { value: 'improving', label: t('followUp.conditionImproving') },
  { value: 'stable', label: t('followUp.conditionStable') },
  { value: 'worsening', label: t('followUp.conditionWorsening') },
  { value: 'answered', label: t('followUp.conditionAnswered') },
  { value: 'unreachable', label: t('followUp.conditionUnreachable') },
])

function genderText(g: string) {
  if (g === 'male') return t('prayer.male')
  if (g === 'female') return t('prayer.female')
  return t('prayer.unspecified')
}

function dotClass(condition: string) {
  if (condition === 'answered') return 'timeline__dot--ok'
  if (condition === 'worsening') return 'timeline__dot--bad'
  if (condition === 'unreachable') return 'timeline__dot--muted'
  return 'timeline__dot--neutral'
}

function conditionBadge(condition: string) {
  if (condition === 'answered') return 'li-badge--gold'
  if (condition === 'worsening') return 'li-badge--urgent'
  return 'li-badge--neutral'
}

function openFollowUpForm() {
  fuForm.contact_date = toDateInput(new Date().toISOString())
  fuForm.channel = 'visit'
  fuForm.update_type = 'current_condition'
  fuForm.condition_status = 'stable'
  fuForm.notes = ''
  fuForm.next_action = ''
  fuForm.next_action_date = ''
  fuErrors.notes = ''
  showFollowUp.value = true
}

async function saveFollowUp() {
  fuErrors.notes = ''
  if (!fuForm.notes.trim()) {
    fuErrors.notes = t('app.required')
    return
  }

  savingFollowUp.value = true
  try {
    await api.createFollowUp(recordId, {
      contact_date: fuForm.contact_date || undefined,
      channel: fuForm.channel,
      update_type: fuForm.update_type,
      condition_status: fuForm.condition_status,
      notes: fuForm.notes.trim(),
      next_action: fuForm.next_action.trim() || undefined,
      next_action_date: fuForm.next_action_date || undefined,
    })
    Notify.create({ type: 'positive', message: t('followUp.savedOk'), timeout: 1800 })
    showFollowUp.value = false
    await loadFollowUps()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  } finally {
    savingFollowUp.value = false
  }
}

/** Pre-fills the praise form from an answered follow-up (PRD F-04). */
function createPraiseFromFollowUp(fu: FollowUp) {
  router.push({
    name: 'praise-new',
    query: {
      follow_up_id: fu.id,
      title: record.value?.title ?? '',
      request_id: recordId,
    },
  })
}

function confirmDelete() {
  Dialog.create({
    title: t('app.delete'),
    message: t('prayer.deleteConfirm'),
    cancel: true,
    persistent: true,
    ok: { label: t('app.confirm'), color: 'negative', unelevated: true, noCaps: true },
  }).onOk(async () => {
    try {
      await api.deletePrayerRequest(recordId)
      Notify.create({ type: 'positive', message: t('app.deleted'), timeout: 1600 })
      router.replace({ name: 'prayer-list' })
    } catch (err) {
      Notify.create({ type: 'negative', message: errorMessage(err) })
    }
  })
}

async function loadFollowUps() {
  try {
    followUps.value = await api.listFollowUps(recordId)
  } catch {
    followUps.value = []
  }
}

onMounted(async () => {
  try {
    record.value = await api.getPrayerRequest(recordId)
    await loadFollowUps()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
    router.replace({ name: 'prayer-list' })
  } finally {
    loading.value = false
  }
})
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.empty-followup {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 28px 16px;
  background: rgba(0, 0, 0, 0.02);
  border-radius: 12px;
}

.timeline {
  position: relative;
  padding-left: 4px;
}

.timeline__item {
  position: relative;
  padding-left: 26px;
  padding-bottom: 20px;
}

.timeline__dot {
  position: absolute;
  left: 0;
  top: 4px;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  border: 2px solid $brand-white;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.14);
}

.timeline__dot--ok { background: #1b7a3e; }
.timeline__dot--bad { background: $brand-orange; }
.timeline__dot--muted { background: #9e9e9e; }
.timeline__dot--neutral { background: $identity-gold; }

.timeline__line {
  position: absolute;
  left: 4px;
  top: 16px;
  bottom: 0;
  width: 1px;
  background: rgba(0, 0, 0, 0.1);
}

.timeline__body {
  background: rgba(0, 0, 0, 0.022);
  border-radius: 10px;
  padding: 12px 14px;
}

.next-action {
  display: flex;
  align-items: center;
  color: rgba(0, 0, 0, 0.68);
  background: rgba(229, 173, 56, 0.11);
  border-radius: 6px;
  padding: 6px 9px;
}

.followup-sheet {
  width: 100%;
  border-radius: 18px 18px 0 0;
  max-width: 560px;
  margin: 0 auto;
}

.followup-note {
  background: rgba(0, 0, 0, 0.04);
  border-radius: 8px;
  color: rgba(0, 0, 0, 0.6);
}

.cond-chip {
  font-size: 11.5px;
  background: rgba(0, 0, 0, 0.045);
  color: rgba(0, 0, 0, 0.72);
  min-height: 30px;
}

.cond-chip--on {
  background: $brand-black;
  color: $brand-white;
}
</style>
