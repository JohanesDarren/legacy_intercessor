<template>
  <q-page class="li-page">
    <div v-if="loadingRecord" class="column items-center q-py-xl">
      <q-spinner color="primary" size="28px" />
    </div>

    <q-form v-else @submit="submit" greedy>
      <q-banner v-if="fromFollowUp" dense class="from-followup q-mb-md">
        <template #avatar><q-icon name="auto_awesome" size="17px" /></template>
        <span style="font-size: 11.5px; line-height: 1.45">
          {{ t('praise.createFromFollowUp') }}
        </span>
      </q-banner>

      <q-input
        v-model="form.title"
        outlined
        dense
        :label="t('praise.titleField')"
        :error="!!errors.title"
        :error-message="errors.title"
        maxlength="255"
        counter
        class="q-mb-md"
        @update:model-value="errors.title = ''"
      />

      <div class="row q-col-gutter-sm q-mb-md">
        <div class="col-12 col-sm-6">
          <q-select
            v-model="form.category_id"
            :options="categoryOptions"
            option-value="id"
            option-label="name"
            emit-value
            map-options
            outlined
            dense
            :label="t('praise.category')"
            :error="!!errors.category_id"
            :error-message="errors.category_id"
            @update:model-value="errors.category_id = ''"
          />
        </div>
        <div class="col-12 col-sm-6">
          <q-select
            v-model="form.reporter_gender"
            :options="genderOptions"
            emit-value
            map-options
            outlined
            dense
            :label="t('prayer.gender')"
          />
        </div>
      </div>

      <q-input
        v-model="form.reporter_name"
        outlined
        dense
        :label="`${t('praise.reporterName')} (${t('app.optional')})`"
        maxlength="100"
        class="q-mb-md"
      />

      <q-input
        v-model="form.content"
        type="textarea"
        outlined
        autogrow
        :label="t('praise.content')"
        :error="!!errors.content"
        :error-message="errors.content"
        maxlength="5000"
        counter
        input-style="min-height: 120px"
        class="q-mb-md"
        @update:model-value="errors.content = ''"
      />

      <!-- Featured (praise has no urgency flag) -->
      <q-card flat class="li-card q-mb-md">
        <q-card-section class="q-pa-md">
          <div class="row items-center justify-between no-wrap">
            <div style="flex: 1; min-width: 0">
              <div class="row items-center q-gutter-xs">
                <q-icon name="star" size="15px" style="color: #e5ad38" />
                <span style="font-size: 13px; font-weight: 600">{{ t('praise.featured') }}</span>
              </div>
              <div class="li-muted q-mt-xs" style="font-size: 11.5px; line-height: 1.4">
                {{ t('praise.featuredHint') }}
              </div>
            </div>
            <q-toggle v-model="form.is_featured" color="secondary" size="md" />
          </div>
        </q-card-section>
      </q-card>

      <!-- Consent: defaults to no -->
      <q-card flat class="li-card q-mb-md">
        <q-card-section class="q-pa-md">
          <div class="li-eyebrow q-mb-sm">{{ t('prayer.consent') }}</div>
          <q-option-group
            v-model="form.display_consent"
            :options="consentOptions"
            color="primary"
          />
          <div class="li-muted q-mt-sm" style="font-size: 11.5px; line-height: 1.45">
            <q-icon name="info" size="12px" class="q-mr-xs" />
            {{ t('prayer.consentHint') }}
          </div>
        </q-card-section>
      </q-card>

      <q-select
        v-model="form.service_id"
        :options="serviceOptions"
        option-value="id"
        option-label="label"
        emit-value
        map-options
        outlined
        dense
        clearable
        :label="t('prayer.service')"
        class="q-mb-md"
      />

      <div style="height: 80px" />
    </q-form>

    <div class="li-thumb-bar">
      <div class="row q-gutter-sm">
        <q-btn flat no-caps color="primary" :label="t('app.cancel')" class="col-4" @click="cancel" />
        <q-btn
          unelevated
          no-caps
          color="primary"
          text-color="white"
          :label="t('app.save')"
          :loading="saving"
          class="col"
          @click="submit"
        />
      </div>
    </div>
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

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const app = useAppStore()

const isEdit = computed(() => route.name === 'praise-edit')
const recordId = computed(() => route.params.id as string)
const fromFollowUp = computed(() => !!route.query.follow_up_id)

const loadingRecord = ref(false)
const saving = ref(false)

const form = reactive({
  title: '',
  category_id: '',
  reporter_name: '',
  reporter_gender: 'unspecified',
  content: '',
  is_featured: false,
  display_consent: 'no',
  service_id: null as string | null,
  source_follow_up_id: null as string | null,
})

const errors = reactive({ title: '', content: '', category_id: '' })

const categoryOptions = computed(() => app.praiseCategories)
const serviceOptions = computed(() =>
  app.services.map((s) => ({ id: s.id, label: formatDateLong(s.starts_at) })),
)

const genderOptions = computed(() => [
  { value: 'unspecified', label: t('prayer.unspecified') },
  { value: 'male', label: t('prayer.male') },
  { value: 'female', label: t('prayer.female') },
])

const consentOptions = computed(() => [
  { value: 'no', label: t('prayer.consentNo') },
  { value: 'anonymous', label: t('prayer.consentAnonymous') },
  { value: 'yes', label: t('prayer.consentYes') },
])

function validate(): boolean {
  errors.title = ''
  errors.content = ''
  errors.category_id = ''

  if (!form.title.trim()) errors.title = t('app.required')
  if (!form.content.trim()) errors.content = t('app.required')
  if (!form.category_id) errors.category_id = t('app.required')

  return !errors.title && !errors.content && !errors.category_id
}

function payload() {
  return {
    service_id: form.service_id || undefined,
    category_id: form.category_id,
    source_follow_up_id: form.source_follow_up_id || undefined,
    reporter_name: form.reporter_name.trim() || undefined,
    reporter_gender: form.reporter_gender,
    title: form.title.trim(),
    content: form.content.trim(),
    is_featured: form.is_featured,
    display_consent: form.display_consent,
  }
}

async function submit() {
  if (!validate()) return

  saving.value = true
  try {
    if (isEdit.value) {
      await api.updatePraiseReport(recordId.value, payload())
    } else {
      await api.createPraiseReport(payload())
    }
    Notify.create({ type: 'positive', message: t('praise.savedOk'), timeout: 1800 })
    router.replace({ name: 'praise-list' })
  } catch (err) {
    if (err instanceof ApiErrorException && err.fields) {
      for (const [field, msgs] of Object.entries(err.fields)) {
        if (field in errors) {
          ;(errors as Record<string, string>)[field] = msgs[0] ?? ''
        }
      }
      Notify.create({ type: 'negative', message: t('errors.validation') })
    } else {
      Notify.create({ type: 'negative', message: errorMessage(err) })
    }
  } finally {
    saving.value = false
  }
}

function cancel() {
  router.back()
}

onMounted(async () => {
  await app.bootstrap()

  if (!form.service_id && app.activeService) form.service_id = app.activeService.id
  if (!form.category_id && app.praiseCategories.length > 0) {
    form.category_id = app.praiseCategories[0].id
  }

  // Pre-fill from an answered follow-up (PRD F-04 acceptance criteria).
  if (fromFollowUp.value) {
    form.source_follow_up_id = route.query.follow_up_id as string
    if (route.query.title) form.title = route.query.title as string
    // Consent stays 'no' until the intercessor confirms.
    form.display_consent = 'no'
  }

  if (isEdit.value) {
    loadingRecord.value = true
    try {
      const rec = await api.getPraiseReport(recordId.value)
      form.title = rec.title
      form.category_id = rec.category_id
      form.reporter_name = rec.reporter_name ?? ''
      form.reporter_gender = rec.reporter_gender
      form.content = rec.content
      form.is_featured = rec.is_featured
      form.display_consent = rec.display_consent
      form.service_id = rec.service_id
      form.source_follow_up_id = rec.source_follow_up_id
    } catch (err) {
      Notify.create({ type: 'negative', message: errorMessage(err) })
      router.replace({ name: 'praise-list' })
    } finally {
      loadingRecord.value = false
    }
  }
})
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.from-followup {
  background: rgba(229, 173, 56, 0.13);
  border-left: 3px solid $identity-gold;
  border-radius: 9px;
  color: rgba(0, 0, 0, 0.75);
}
</style>
