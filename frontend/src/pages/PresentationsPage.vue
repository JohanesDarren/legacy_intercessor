<template>
  <q-page class="li-page">
    <!-- Controls -->
    <div class="row q-col-gutter-sm q-mb-md">
      <div class="col-12 col-sm-6">
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
          @update:model-value="load"
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
          @update:model-value="load"
        />
      </div>
      <div class="col-6 col-sm-3">
        <q-btn
          v-if="auth.canManageSlides"
          unelevated
          no-caps
          color="primary"
          text-color="white"
          icon="add"
          class="full-width"
          style="height: 40px"
          :loading="building"
          :label="presentations.length ? t('presentations.rebuild') : t('presentations.build')"
          @click="build"
        />
      </div>
    </div>

    <!-- Latest approved export for the media team (US-15) -->
    <q-card v-if="latest" flat class="latest-card q-mb-md">
      <q-card-section class="q-pa-md row items-center no-wrap">
        <q-icon name="download_for_offline" size="20px" class="q-mr-sm" style="color: #1b7a3e" />
        <div style="flex: 1; min-width: 0">
          <div class="li-eyebrow" style="opacity: 0.6">{{ t('presentations.latest') }}</div>
          <div style="font-size: 12.5px; font-weight: 600">
            {{ latest.format.toUpperCase() }} · {{ formatDateTime(latest.created_at) }}
          </div>
        </div>
        <q-btn
          unelevated
          dense
          no-caps
          color="primary"
          text-color="white"
          icon="download"
          :label="t('app.download')"
          size="sm"
          @click="download(latest)"
        />
      </q-card-section>
    </q-card>

    <!-- Loading -->
    <div v-if="loading" class="column items-center q-py-xl">
      <q-spinner color="primary" size="28px" />
    </div>

    <!-- Empty -->
    <div v-else-if="presentations.length === 0" class="empty-state">
      <q-icon name="slideshow" size="30px" class="li-muted" />
      <p class="li-muted q-mt-sm" style="font-size: 13px; text-align: center">
        {{ t('presentations.blockedEmpty') }}
      </p>
      <q-btn
        v-if="auth.canManageSlides"
        unelevated
        no-caps
        color="primary"
        text-color="white"
        :label="t('presentations.build')"
        class="q-mt-sm"
        :loading="building"
        @click="build"
      />
    </div>

    <!-- List -->
    <template v-else>
      <q-card
        v-for="pres in presentations"
        :key="pres.id"
        flat
        class="li-card li-card--tap q-mb-sm"
        @click="open(pres.id)"
      >
        <q-card-section class="q-pa-md">
          <div class="row items-center no-wrap">
            <div style="flex: 1; min-width: 0">
              <div class="row items-center q-gutter-xs q-mb-xs">
                <span class="li-badge" :class="statusBadge(pres.status)">
                  {{ statusText(pres.status) }}
                </span>
                <span class="li-eyebrow">
                  {{ t('presentations.version', { n: pres.version }) }}
                </span>
              </div>
              <div style="font-weight: 600; font-size: 14px">
                {{ pres.kind === 'praise' ? t('presentations.praiseTemplate') : t('presentations.prayerTemplate') }}
              </div>
              <div class="li-muted q-mt-xs" style="font-size: 11.5px">
                {{ t('presentations.slides', { n: pres.slides.length }) }} ·
                {{ formatDateTime(pres.created_at) }}
              </div>
            </div>
            <q-icon name="chevron_right" size="20px" class="li-muted" style="opacity: 0.3" />
          </div>
        </q-card-section>
      </q-card>
    </template>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Notify } from 'quasar'
import api, { ApiErrorException } from '@/services/api'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { errorMessage } from '@/boot/axios'
import { formatDateLong, formatDateTime } from '@/utils/format'
import type { Export, Kind, Presentation } from '@/services/types'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const app = useAppStore()
const auth = useAuthStore()

const serviceId = ref('')
const kind = ref<Kind>('prayer')
const presentations = ref<Presentation[]>([])
const latest = ref<Export | null>(null)
const loading = ref(true)
const building = ref(false)

const kindOptions = computed(() => [
  { value: 'prayer', label: t('summaries.prayer') },
  { value: 'praise', label: t('summaries.praise') },
])

const serviceOptions = computed(() =>
  app.services.map((s) => ({ id: s.id, label: formatDateLong(s.starts_at) })),
)

function statusText(status: string) {
  if (status === 'exported') return t('presentations.statusExported')
  if (status === 'ready') return t('presentations.statusReady')
  return t('presentations.statusDraft')
}

function statusBadge(status: string) {
  if (status === 'exported') return 'li-badge--gold'
  if (status === 'ready') return 'li-badge--neutral'
  return 'li-badge--muted'
}

async function load() {
  if (!serviceId.value) return
  loading.value = true
  try {
    presentations.value = await api.listPresentations({
      service_id: serviceId.value,
      kind: kind.value,
    })
    try {
      latest.value = await api.latestExport(serviceId.value, kind.value)
    } catch {
      latest.value = null
    }
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
    presentations.value = []
  } finally {
    loading.value = false
  }
}

async function build() {
  if (!serviceId.value) return
  building.value = true
  try {
    const pres = await api.buildPresentation({ service_id: serviceId.value, kind: kind.value })
    Notify.create({ type: 'positive', message: t('app.saved'), timeout: 1600 })
    router.push({ name: 'presentation-detail', params: { id: pres.id } })
  } catch (err) {
    if (err instanceof ApiErrorException) {
      if (err.code === 'EMPTY_PRESENTATION') {
        Notify.create({ type: 'warning', message: t('summaries.noEligible'), timeout: 4000 })
      } else {
        Notify.create({ type: 'negative', message: err.message })
      }
    } else {
      Notify.create({ type: 'negative', message: errorMessage(err) })
    }
  } finally {
    building.value = false
  }
}

function open(id: string) {
  router.push({ name: 'presentation-detail', params: { id } })
}

function download(exp: Export) {
  if (!exp.download_url) return
  window.open(exp.download_url, '_blank')
}

onMounted(async () => {
  await app.bootstrap()
  serviceId.value = (route.query.service_id as string) || app.activeService?.id || ''
  if (route.query.kind === 'praise') kind.value = 'praise'
  await load()
})
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.latest-card {
  border: 1px solid rgba(27, 122, 62, 0.22);
  background: rgba(27, 122, 62, 0.04);
  border-radius: 12px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 40px 16px;
}
</style>
