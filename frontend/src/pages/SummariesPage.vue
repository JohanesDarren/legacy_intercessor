<template>
  <q-page class="li-page">
    <!-- Controls -->
    <div class="row q-col-gutter-sm q-mb-md">
      <div class="col-12 col-sm-5">
        <q-select
          v-model="serviceId"
          :options="serviceOptions"
          option-value="id"
          option-label="label"
          emit-value
          map-options
          outlined
          dense
          :label="t('prayer.service')"
          @update:model-value="loadSummaries"
        />
      </div>
      <div class="col-6 col-sm-3">
        <q-select
          v-model="kind"
          :options="kindOptions"
          emit-value
          map-options
          outlined
          dense
          :label="t('summaries.kind')"
          @update:model-value="loadSummaries"
        />
      </div>
      <div class="col-6 col-sm-2">
        <q-select
          v-model="language"
          :options="languageOptions"
          emit-value
          map-options
          outlined
          dense
          :label="t('summaries.language')"
        />
      </div>
      <div class="col-12 col-sm-2">
        <q-btn
          unelevated
          no-caps
          color="primary"
          text-color="white"
          icon="auto_awesome"
          class="full-width"
          style="height: 40px"
          :loading="generating"
          :label="summaries.length ? t('summaries.regenerate') : t('summaries.generate')"
          @click="generate"
        />
      </div>
    </div>

    <!-- Provider info -->
    <div v-if="aiInfo" class="provider-bar q-mb-md">
      <div class="row items-center no-wrap">
        <q-icon
          :name="aiInfo.offline_mode ? 'cloud_off' : 'auto_awesome'"
          size="15px"
          class="q-mr-xs"
        />
        <span style="font-size: 11px">
          {{ t('summaries.provider') }}: <strong>{{ aiInfo.provider }}</strong>
          · {{ t('summaries.model') }}: <strong>{{ aiInfo.model }}</strong>
          · {{ t('summaries.promptVersion') }}: {{ aiInfo.prompt_version }}
        </span>
      </div>
      <div v-if="aiInfo.offline_mode" class="q-mt-xs" style="font-size: 11px; line-height: 1.45">
        {{ t('summaries.offlineMode') }}
      </div>
    </div>

    <!-- Progress summary -->
    <div v-if="summaries.length > 0" class="row items-center justify-between q-mb-sm">
      <span class="li-eyebrow li-tabular" style="opacity: 0.55">
        {{ approvedCount }} / {{ summaries.length }} {{ t('summaries.approved') }}
      </span>
      <q-btn
        v-if="approvedCount < summaries.length"
        flat
        dense
        no-caps
        size="sm"
        color="primary"
        icon="done_all"
        label="Setujui semua"
        @click="approveAll"
      />
    </div>

    <!-- Loading -->
    <div v-if="loading || generating" class="column items-center q-py-xl">
      <q-spinner color="primary" size="28px" />
      <p class="li-muted q-mt-sm" style="font-size: 12.5px">
        {{ generating ? t('summaries.generating') : t('app.loading') }}
      </p>
    </div>

    <!-- Empty -->
    <div v-else-if="summaries.length === 0" class="empty-state">
      <q-icon name="auto_awesome" size="30px" class="li-muted" />
      <p class="li-muted q-mt-sm" style="font-size: 13px">{{ t('summaries.noneYet') }}</p>
      <q-btn
        unelevated
        no-caps
        color="primary"
        text-color="white"
        :label="t('summaries.generate')"
        class="q-mt-sm"
        @click="generate"
      />
    </div>

    <!-- Summary cards -->
    <template v-else>
      <q-card v-for="sum in summaries" :key="sum.id" flat class="li-card q-mb-md">
        <q-card-section class="q-pa-md">
          <!-- Header -->
          <div class="row items-center justify-between no-wrap q-mb-sm">
            <div class="row items-center q-gutter-xs">
              <span class="li-display" style="font-size: 15px">{{ sum.category_name }}</span>
              <span class="li-badge" :class="statusBadge(sum.status)">
                {{ statusText(sum.status) }}
              </span>
            </div>
            <span class="li-muted li-tabular" style="font-size: 10.5px">
              {{ t('summaries.sourceRecords', { n: sum.source_ids.length }) }}
            </span>
          </div>

          <!-- Editable text -->
          <q-input
            v-model="drafts[sum.id]"
            type="textarea"
            outlined
            autogrow
            dense
            :label="sum.status === 'approved' ? t('summaries.finalText') : t('summaries.aiText')"
            input-style="min-height: 70px; font-size: 13px; line-height: 1.55"
            :disable="sum.status === 'approved' && !editing[sum.id]"
            class="q-mb-sm"
          />

          <!-- Original AI text for audit -->
          <q-expansion-item
            v-if="sum.final_text && sum.final_text !== sum.ai_text"
            dense
            :label="t('summaries.aiText') + ' (asli)'"
            class="audit-expansion"
          >
            <div class="li-muted q-pa-sm" style="font-size: 11.5px; line-height: 1.5">
              {{ sum.ai_text }}
            </div>
          </q-expansion-item>

          <!-- Sources -->
          <q-expansion-item
            dense
            :label="t('summaries.showSources')"
            class="audit-expansion q-mt-xs"
          >
            <div class="q-pa-sm">
              <div
                v-for="sid in sum.source_ids"
                :key="sid"
                class="source-row"
                @click="openSource(sid)"
              >
                <q-icon name="description" size="12px" class="q-mr-xs li-muted" />
                <span class="li-muted" style="font-size: 10.5px; font-family: monospace">
                  {{ sid.slice(0, 8) }}…
                </span>
                <q-icon name="chevron_right" size="13px" class="li-muted q-ml-auto" />
              </div>
            </div>
          </q-expansion-item>

          <!-- Actions -->
          <div class="row q-gutter-sm q-mt-sm">
            <template v-if="sum.status !== 'approved'">
              <q-btn
                unelevated
                dense
                no-caps
                color="positive"
                text-color="white"
                icon="check"
                :label="t('summaries.approve')"
                size="sm"
                :loading="busy[sum.id]"
                @click="review(sum, 'approved')"
              />
              <q-btn
                flat
                dense
                no-caps
                color="negative"
                icon="close"
                :label="t('summaries.reject')"
                size="sm"
                :disable="busy[sum.id]"
                @click="review(sum, 'rejected')"
              />
            </template>
            <template v-else>
              <q-btn
                flat
                dense
                no-caps
                color="primary"
                icon="edit"
                :label="t('app.edit')"
                size="sm"
                @click="editing[sum.id] = !editing[sum.id]"
              />
              <q-btn
                v-if="editing[sum.id]"
                unelevated
                dense
                no-caps
                color="primary"
                text-color="white"
                :label="t('summaries.saveEdit')"
                size="sm"
                :loading="busy[sum.id]"
                @click="saveEdit(sum)"
              />
            </template>
          </div>
        </q-card-section>
      </q-card>

      <!-- Next step -->
      <q-card v-if="approvedCount === summaries.length" flat class="next-step q-mt-md">
        <q-card-section class="q-pa-md row items-center no-wrap">
          <q-icon name="check_circle" size="20px" color="positive" class="q-mr-sm" />
          <div style="flex: 1">
            <div style="font-weight: 600; font-size: 13px">
              {{ t('summaries.approved') }}
            </div>
            <div class="li-muted" style="font-size: 11.5px">
              {{ t('presentations.subtitle') }}
            </div>
          </div>
          <q-btn
            unelevated
            dense
            no-caps
            color="primary"
            text-color="white"
            icon="slideshow"
            :label="t('presentations.build')"
            size="sm"
            :to="{ name: 'presentations', query: { service_id: serviceId, kind } }"
          />
        </q-card-section>
      </q-card>
    </template>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Notify } from 'quasar'
import api, { ApiErrorException } from '@/services/api'
import { useAppStore } from '@/stores/app'
import { errorMessage } from '@/boot/axios'
import { formatDateLong } from '@/utils/format'
import type { AiStatus, Kind, Summary } from '@/services/types'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const app = useAppStore()

const serviceId = ref('')
const kind = ref<Kind>('prayer')
const language = ref<'id' | 'en'>('id')

const summaries = ref<Summary[]>([])
const aiInfo = ref<AiStatus | null>(null)
const loading = ref(true)
const generating = ref(false)

/** Local editable copies, keyed by summary id. */
const drafts = reactive<Record<string, string>>({})
const busy = reactive<Record<string, boolean>>({})
const editing = reactive<Record<string, boolean>>({})

const kindOptions = computed(() => [
  { value: 'prayer', label: t('summaries.prayer') },
  { value: 'praise', label: t('summaries.praise') },
])

const languageOptions = computed(() => [
  { value: 'id', label: t('summaries.languageId') },
  { value: 'en', label: t('summaries.languageEn') },
])

const serviceOptions = computed(() =>
  app.services.map((s) => ({ id: s.id, label: formatDateLong(s.starts_at) })),
)

const approvedCount = computed(() => summaries.value.filter((s) => s.status === 'approved').length)

function statusText(status: string) {
  if (status === 'approved') return t('summaries.approved')
  if (status === 'rejected') return t('summaries.rejected')
  return t('summaries.draft')
}

function statusBadge(status: string) {
  if (status === 'approved') return 'li-badge--gold'
  if (status === 'rejected') return 'li-badge--urgent'
  return 'li-badge--neutral'
}

function syncDrafts() {
  for (const s of summaries.value) {
    if (drafts[s.id] === undefined) {
      drafts[s.id] = s.final_text ?? s.ai_text
    }
  }
}

async function loadSummaries() {
  if (!serviceId.value) return
  loading.value = true
  try {
    summaries.value = await api.listSummaries(serviceId.value, kind.value)
    for (const key of Object.keys(drafts)) delete drafts[key]
    for (const key of Object.keys(editing)) delete editing[key]
    syncDrafts()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
    summaries.value = []
  } finally {
    loading.value = false
  }
}

async function generate() {
  if (!serviceId.value) {
    Notify.create({ type: 'warning', message: t('prayer.service') })
    return
  }
  generating.value = true
  try {
    summaries.value = await api.generateSummaries({
      service_id: serviceId.value,
      kind: kind.value,
      language: language.value,
      max_words_per_item: 40,
    })
    for (const key of Object.keys(drafts)) delete drafts[key]
    syncDrafts()
    Notify.create({
      type: 'positive',
      message: `${summaries.value.length} ${t('summaries.draft')}`,
      timeout: 1800,
    })
  } catch (err) {
    if (err instanceof ApiErrorException) {
      if (err.code === 'NO_ELIGIBLE_RECORDS') {
        Notify.create({ type: 'warning', message: t('summaries.noEligible'), timeout: 4000 })
      } else if (err.code === 'AI_PROVIDER_TIMEOUT' || err.code === 'AI_PROVIDER_UNAVAILABLE') {
        Notify.create({ type: 'negative', message: t('summaries.failed'), timeout: 4000 })
      } else {
        Notify.create({ type: 'negative', message: err.message })
      }
    } else {
      Notify.create({ type: 'negative', message: errorMessage(err) })
    }
  } finally {
    generating.value = false
  }
}

async function review(sum: Summary, status: 'approved' | 'rejected') {
  busy[sum.id] = true
  try {
    const text = drafts[sum.id]?.trim()
    const updated = await api.reviewSummary(sum.id, {
      status,
      final_text: status === 'approved' && text ? text : undefined,
    })
    const idx = summaries.value.findIndex((s) => s.id === sum.id)
    if (idx >= 0) summaries.value[idx] = updated
    drafts[sum.id] = updated.final_text ?? updated.ai_text
    editing[sum.id] = false
    Notify.create({
      type: 'positive',
      message: status === 'approved' ? t('summaries.approved') : t('summaries.rejected'),
      timeout: 1600,
    })
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  } finally {
    busy[sum.id] = false
  }
}

async function saveEdit(sum: Summary) {
  busy[sum.id] = true
  try {
    const updated = await api.reviewSummary(sum.id, {
      final_text: drafts[sum.id]?.trim(),
      status: 'approved',
    })
    const idx = summaries.value.findIndex((s) => s.id === sum.id)
    if (idx >= 0) summaries.value[idx] = updated
    drafts[sum.id] = updated.final_text ?? updated.ai_text
    editing[sum.id] = false
    Notify.create({ type: 'positive', message: t('app.saved'), timeout: 1600 })
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  } finally {
    busy[sum.id] = false
  }
}

async function approveAll() {
  for (const sum of summaries.value) {
    if (sum.status !== 'approved') {
      await review(sum, 'approved')
    }
  }
}

function openSource(id: string) {
  if (kind.value === 'prayer') {
    router.push({ name: 'prayer-detail', params: { id } })
  } else {
    router.push({ name: 'praise-edit', params: { id } })
  }
}

onMounted(async () => {
  await app.bootstrap()

  serviceId.value = (route.query.service_id as string) || app.activeService?.id || ''
  if (route.query.kind === 'praise') kind.value = 'praise'

  try {
    aiInfo.value = await api.aiStatus()
  } catch {
    aiInfo.value = null
  }

  await loadSummaries()
})
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.provider-bar {
  background: rgba(0, 0, 0, 0.035);
  border-radius: 9px;
  padding: 9px 12px;
  color: rgba(0, 0, 0, 0.66);
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 40px 16px;
}

.audit-expansion {
  border-top: 1px solid rgba(0, 0, 0, 0.06);
  font-size: 12px;

  :deep(.q-item) {
    min-height: 34px;
    padding: 0;
  }

  :deep(.q-item__label) {
    font-size: 11.5px;
    color: rgba(0, 0, 0, 0.55);
  }
}

.source-row {
  display: flex;
  align-items: center;
  padding: 6px 4px;
  border-radius: 5px;
  cursor: pointer;

  &:active {
    background: rgba(0, 0, 0, 0.05);
  }
}

.next-step {
  border: 1px solid rgba(27, 122, 62, 0.24);
  background: rgba(27, 122, 62, 0.045);
  border-radius: 12px;
}
</style>
