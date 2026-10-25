<template>
  <q-page class="li-page">
    <div v-if="loading" class="column items-center q-py-xl">
      <q-spinner color="primary" size="28px" />
    </div>

    <template v-else-if="pres">
      <!-- Header -->
      <div class="row items-start justify-between no-wrap q-mb-md">
        <div style="min-width: 0">
          <div class="row items-center q-gutter-xs q-mb-xs">
            <span class="li-badge" :class="pres.status === 'exported' ? 'li-badge--gold' : 'li-badge--muted'">
              {{ statusText(pres.status) }}
            </span>
            <span class="li-eyebrow">{{ t('presentations.version', { n: pres.version }) }}</span>
          </div>
          <h1 class="li-display" style="font-size: 19px; line-height: 1.2">
            {{ pres.kind === 'praise' ? t('presentations.praiseTemplate') : t('presentations.prayerTemplate') }}
          </h1>
          <div class="li-muted" style="font-size: 12px">
            {{ t('presentations.slides', { n: pres.slides.length }) }}
          </div>
        </div>
      </div>

      <!-- Export actions -->
      <div v-if="auth.canManageSlides" class="row q-col-gutter-sm q-mb-md">
        <div class="col-6">
          <q-btn
            unelevated
            no-caps
            color="primary"
            text-color="white"
            icon="picture_as_pdf"
            class="full-width"
            style="height: 44px"
            :loading="exporting === 'pdf'"
            :label="t('presentations.exportPdf')"
            @click="runExport('pdf')"
          />
        </div>
        <div class="col-6">
          <q-btn
            outline
            no-caps
            color="primary"
            icon="image"
            class="full-width"
            style="height: 44px"
            :loading="exporting === 'png'"
            :label="t('presentations.exportPng')"
            @click="runExport('png')"
          />
        </div>
      </div>

      <!-- Export progress -->
      <q-banner v-if="exporting" dense class="export-progress q-mb-md">
        <template #avatar><q-spinner size="16px" color="primary" /></template>
        <span style="font-size: 12px">{{ t('presentations.exporting') }}</span>
      </q-banner>

      <!-- Export result -->
      <q-card v-if="lastExport && lastExport.status === 'done'" flat class="export-done q-mb-md">
        <q-card-section class="q-pa-md row items-center no-wrap">
          <q-icon name="check_circle" size="20px" color="positive" class="q-mr-sm" />
          <div style="flex: 1; min-width: 0">
            <div style="font-weight: 600; font-size: 13px">{{ t('presentations.exportDone') }}</div>
            <div class="li-muted" style="font-size: 11px">
              {{ lastExport.format.toUpperCase() }} · {{ formatDateTime(lastExport.created_at) }}
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
            @click="download"
          />
        </q-card-section>
      </q-card>

      <q-banner v-if="lastExport && lastExport.status === 'failed'" dense class="export-failed q-mb-md">
        <template #avatar><q-icon name="error_outline" size="17px" /></template>
        <span style="font-size: 12px">{{ lastExport.error_message || t('presentations.exportFailed') }}</span>
      </q-banner>

      <!-- Slides -->
      <div class="li-eyebrow q-mb-sm" style="opacity: 0.6">{{ t('presentations.preview') }}</div>

      <div v-for="(slide, idx) in pres.slides" :key="slide.id" class="slide-block q-mb-md">
        <div class="row items-center justify-between no-wrap q-mb-xs">
          <span class="li-eyebrow li-tabular">
            {{ t('presentations.slideOf', { n: idx + 1, total: pres.slides.length }) }}
          </span>
          <div v-if="auth.canManageSlides" class="row q-gutter-xs">
            <q-btn
              flat
              dense
              round
              size="sm"
              icon="arrow_upward"
              :disable="idx === 0"
              @click="move(idx, -1)"
            />
            <q-btn
              flat
              dense
              round
              size="sm"
              icon="arrow_downward"
              :disable="idx === pres.slides.length - 1"
              @click="move(idx, 1)"
            />
            <q-btn
              flat
              dense
              round
              size="sm"
              icon="delete_outline"
              color="negative"
              @click="removeSlide(idx)"
            />
          </div>
        </div>

        <SlidePreview :slide="slide" :index="idx" :total="pres.slides.length" />
      </div>

      <!-- Export history -->
      <div v-if="exports.length > 0" class="q-mt-lg">
        <div class="li-eyebrow q-mb-sm" style="opacity: 0.6">{{ t('presentations.history') }}</div>
        <q-card flat class="li-card">
          <q-list separator>
            <q-item v-for="exp in exports" :key="exp.id">
              <q-item-section avatar style="min-width: 32px">
                <q-icon
                  :name="exp.format === 'pdf' ? 'picture_as_pdf' : 'image'"
                  size="17px"
                  class="li-muted"
                />
              </q-item-section>
              <q-item-section>
                <q-item__label style="font-size: 12.5px; font-weight: 600">
                  {{ exp.format.toUpperCase() }}
                  <span class="li-muted" style="font-weight: 400">
                    · {{ formatDateTime(exp.created_at) }}
                  </span>
                </q-item__label>
                <q-item__label caption style="font-size: 11px">
                  {{ exp.status }}
                </q-item__label>
              </q-item-section>
              <q-item-section v-if="exp.download_url" side>
                <q-btn
                  flat
                  dense
                  round
                  icon="download"
                  size="sm"
                  @click="downloadExport(exp)"
                />
              </q-item-section>
            </q-item>
          </q-list>
        </q-card>
      </div>
    </template>
  </q-page>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Notify, Dialog } from 'quasar'
import SlidePreview from '@/components/SlidePreview.vue'
import api, { ApiErrorException } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { errorMessage } from '@/boot/axios'
import { formatDateTime } from '@/utils/format'
import type { Export, ExportFormat, Presentation } from '@/services/types'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const presId = route.params.id as string
const pres = ref<Presentation | null>(null)
const exports = ref<Export[]>([])
const lastExport = ref<Export | null>(null)
const loading = ref(true)
const exporting = ref<ExportFormat | null>(null)

function statusText(status: string) {
  if (status === 'exported') return t('presentations.statusExported')
  if (status === 'ready') return t('presentations.statusReady')
  return t('presentations.statusDraft')
}

async function load() {
  loading.value = true
  try {
    pres.value = await api.getPresentation(presId)
    try {
      exports.value = await api.listExports(presId)
    } catch {
      exports.value = []
    }
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
    router.replace({ name: 'presentations' })
  } finally {
    loading.value = false
  }
}

async function runExport(format: ExportFormat) {
  if (!pres.value) return
  exporting.value = format
  lastExport.value = null
  try {
    const job = await api.startExport(pres.value.id, format)
    const done = await api.waitForExport(pres.value.id, job.id, (e) => {
      lastExport.value = e
    })
    lastExport.value = done
    if (done.status === 'done') {
      Notify.create({ type: 'positive', message: t('presentations.exportDone'), timeout: 2000 })
      exports.value = await api.listExports(pres.value.id)
      if (pres.value) pres.value.status = 'exported'
    } else if (done.status === 'failed') {
      Notify.create({ type: 'negative', message: done.error_message || t('presentations.exportFailed') })
    }
  } catch (err) {
    if (err instanceof ApiErrorException) {
      if (err.code === 'CONSENT_MISSING') {
        Notify.create({ type: 'negative', message: t('presentations.blockedConsent'), timeout: 5000 })
      } else if (err.code === 'SUMMARY_NOT_APPROVED') {
        Notify.create({ type: 'negative', message: t('presentations.blockedDraft'), timeout: 5000 })
      } else if (err.code === 'EMPTY_PRESENTATION') {
        Notify.create({ type: 'warning', message: t('presentations.blockedEmpty') })
      } else {
        Notify.create({ type: 'negative', message: err.message })
      }
    } else {
      Notify.create({ type: 'negative', message: errorMessage(err) })
    }
  } finally {
    exporting.value = null
  }
}

function download() {
  if (lastExport.value?.download_url) {
    window.open(lastExport.value.download_url, '_blank')
  }
}

/** Opens a stored export in a new tab (used from the history list). */
function downloadExport(exp: Export) {
  if (exp.download_url) {
    window.open(exp.download_url, '_blank')
  }
}

/** Reorder slides locally and persist (PRD F-07). */
async function move(idx: number, delta: number) {
  if (!pres.value) return
  const slides = [...pres.value.slides]
  const target = idx + delta
  if (target < 0 || target >= slides.length) return
  ;[slides[idx], slides[target]] = [slides[target], slides[idx]]
  pres.value.slides = slides
  try {
    await api.updatePresentation(pres.value.id, slides)
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
    await load()
  }
}

function removeSlide(idx: number) {
  if (!pres.value) return
  const slide = pres.value.slides[idx]
  Dialog.create({
    title: t('presentations.removeSlide'),
    message: `${slide.title} · ${slide.items.length}`,
    cancel: true,
    ok: { label: t('app.delete'), color: 'negative', unelevated: true, noCaps: true },
  }).onOk(async () => {
    if (!pres.value) return
    const slides = pres.value.slides.filter((_, i) => i !== idx)
    pres.value.slides = slides
    try {
      await api.updatePresentation(pres.value.id, slides)
      Notify.create({ type: 'positive', message: t('app.deleted'), timeout: 1500 })
    } catch (err) {
      Notify.create({ type: 'negative', message: errorMessage(err) })
      await load()
    }
  })
}

onMounted(load)
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.export-progress {
  background: rgba(0, 0, 0, 0.04);
  border-radius: 9px;
}

.export-done {
  border: 1px solid rgba(27, 122, 62, 0.24);
  background: rgba(27, 122, 62, 0.045);
  border-radius: 11px;
}

.export-failed {
  background: rgba(255, 85, 0, 0.1);
  border-left: 3px solid $brand-orange;
  border-radius: 9px;
  color: #b33c00;
}

.slide-block {
  border-radius: 12px;
}
</style>
