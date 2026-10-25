<template>
  <q-page class="li-page">
    <div v-if="loadingRecord" class="column items-center q-py-xl">
      <q-spinner color="primary" size="28px" />
    </div>

    <q-form v-else @submit="submit" greedy>
      <!-- Title -->
      <q-input
        v-model="form.title"
        outlined
        dense
        :label="t('prayer.titleField')"
        :hint="t('prayer.titleHint')"
        :error="!!errors.title"
        :error-message="errors.title"
        maxlength="255"
        counter
        class="q-mb-md"
        @update:model-value="errors.title = ''"
      />

      <!-- Category + gender -->
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
            :label="t('prayer.category')"
            :error="!!errors.category_id"
            :error-message="errors.category_id"
            @update:model-value="errors.category_id = ''"
          />
        </div>
        <div class="col-12 col-sm-6">
          <q-select
            v-model="form.requester_gender"
            :options="genderOptions"
            emit-value
            map-options
            outlined
            dense
            :label="t('prayer.gender')"
          />
        </div>
      </div>

      <!-- Requester name -->
      <q-input
        v-model="form.requester_name"
        outlined
        dense
        :label="`${t('prayer.requesterName')} (${t('app.optional')})`"
        :hint="t('prayer.requesterNameHint')"
        maxlength="100"
        class="q-mb-md"
      />

      <!-- Content -->
      <q-input
        v-model="form.content"
        type="textarea"
        outlined
        autogrow
        :label="t('prayer.content')"
        :hint="t('prayer.contentHint')"
        :error="!!errors.content"
        :error-message="errors.content"
        maxlength="5000"
        counter
        input-style="min-height: 120px"
        class="q-mb-md"
        @update:model-value="errors.content = ''"
      />

      <!-- Urgent -->
      <q-card flat class="li-card q-mb-md">
        <q-card-section class="q-pa-md">
          <div class="row items-center justify-between no-wrap">
            <div style="flex: 1; min-width: 0">
              <div class="row items-center q-gutter-xs">
                <span class="li-badge li-badge--urgent">URGENT</span>
              </div>
              <div class="li-muted q-mt-xs" style="font-size: 11.5px; line-height: 1.4">
                {{ t('prayer.urgentHint') }}
              </div>
            </div>
            <q-toggle v-model="form.is_urgent" color="accent" size="md" />
          </div>
        </q-card-section>
      </q-card>

      <!-- Display consent: defaults to no -->
      <q-card flat class="li-card q-mb-md">
        <q-card-section class="q-pa-md">
          <div class="li-eyebrow q-mb-sm">{{ t('prayer.consent') }}</div>
          <q-option-group
            v-model="form.display_consent"
            :options="consentOptions"
            color="primary"
            class="consent-group"
          />
          <div class="li-muted q-mt-sm" style="font-size: 11.5px; line-height: 1.45">
            <q-icon name="info" size="12px" class="q-mr-xs" />
            {{ t('prayer.consentHint') }}
          </div>
          <q-banner
            v-if="form.display_consent === 'no'"
            dense
            class="q-mt-sm consent-note"
          >
            {{ t('errors.consentMissing') }}
          </q-banner>
        </q-card-section>
      </q-card>

      <!-- Service + status -->
      <div class="row q-col-gutter-sm q-mb-md">
        <div class="col-12 col-sm-6">
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
          />
        </div>
        <div class="col-12 col-sm-6">
          <q-select
            v-model="form.status"
            :options="statusOptions"
            emit-value
            map-options
            outlined
            dense
            :label="t('prayer.status')"
          />
        </div>
      </div>

      <div style="height: 80px" />
    </q-form>

    <!-- Thumb-reach action bar (PRD F-03) -->
    <div class="li-thumb-bar">
      <div class="row q-gutter-sm">
        <q-btn
          flat
          no-caps
          color="primary"
          :label="t('app.cancel')"
          class="col-4"
          @click="cancel"
        />
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

    <!-- Duplicate confirmation dialog -->
    <q-dialog v-model="showDuplicate">
      <q-card style="max-width: 420px; width: 92vw; border-radius: 16px">
        <q-card-section class="row items-start no-wrap">
          <q-icon name="content_copy" size="22px" color="warning" class="q-mr-sm" />
          <div>
            <div class="li-display" style="font-size: 16px">{{ t('errors.duplicate') }}</div>
            <p class="li-muted q-mt-xs" style="font-size: 13px; line-height: 1.5">
              {{ t('prayer.duplicateWarning') }}
            </p>
          </div>
        </q-card-section>
        <q-card-actions align="right" class="q-pa-md">
          <q-btn flat no-caps :label="t('app.cancel')" @click="showDuplicate = false" />
          <q-btn
            unelevated
            no-caps
            color="primary"
            text-color="white"
            :label="t('app.confirm')"
            @click="saveAnyway"
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
import { Notify } from 'quasar'
import api, { ApiErrorException } from '@/services/api'
import { useAppStore } from '@/stores/app'
import { errorMessage } from '@/boot/axios'
import { formatDateLong } from '@/utils/format'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const app = useAppStore()

const isEdit = computed(() => route.name === 'prayer-edit')
const recordId = computed(() => route.params.id as string)

const loadingRecord = ref(false)
const saving = ref(false)
const showDuplicate = ref(false)

const form = reactive({
  title: '',
  category_id: '',
  requester_name: '',
  requester_gender: 'unspecified',
  content: '',
  is_urgent: false,
  display_consent: 'no',
  service_id: null as string | null,
  status: 'open',
})

const errors = reactive({ title: '', content: '', category_id: '' })

const categoryOptions = computed(() => app.prayerCategories)

const serviceOptions = computed(() =>
  app.services.map((s) => ({
    id: s.id,
    label: formatDateLong(s.starts_at),
  })),
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

const statusOptions = computed(() => [
  { value: 'open', label: 'Baru' },
  { value: 'in_prayer', label: 'Didalam doa' },
  { value: 'answered', label: 'Dijawab' },
  { value: 'closed', label: 'Selesai' },
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

function payload(confirmDuplicate = false) {
  return {
    service_id: form.service_id || undefined,
    category_id: form.category_id,
    requester_name: form.requester_name.trim() || undefined,
    requester_gender: form.requester_gender,
    title: form.title.trim(),
    content: form.content.trim(),
    is_urgent: form.is_urgent,
    display_consent: form.display_consent,
    status: form.status,
    confirm_duplicate: confirmDuplicate,
  }
}

async function submit() {
  if (!validate()) return

  saving.value = true
  try {
    if (isEdit.value) {
      await api.updatePrayerRequest(recordId.value, payload(true))
    } else {
      await api.createPrayerRequest(payload(false))
    }
    Notify.create({ type: 'positive', message: t('prayer.savedOk'), timeout: 1800 })
    router.back()
  } catch (err) {
    if (err instanceof ApiErrorException && err.code === 'DUPLICATE_REQUEST') {
      showDuplicate.value = true
    } else if (err instanceof ApiErrorException && err.fields) {
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

async function saveAnyway() {
  showDuplicate.value = false
  saving.value = true
  try {
    await api.createPrayerRequest(payload(true))
    Notify.create({ type: 'positive', message: t('prayer.savedOk'), timeout: 1800 })
    router.back()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  } finally {
    saving.value = false
  }
}

function cancel() {
  router.back()
}

onMounted(async () => {
  await app.bootstrap()

  if (!form.service_id && app.activeService) {
    form.service_id = app.activeService.id
  }
  if (!form.category_id && app.prayerCategories.length > 0) {
    form.category_id = app.prayerCategories[0].id
  }

  if (isEdit.value) {
    loadingRecord.value = true
    try {
      const rec = await api.getPrayerRequest(recordId.value)
      form.title = rec.title
      form.category_id = rec.category_id
      form.requester_name = rec.requester_name ?? ''
      form.requester_gender = rec.requester_gender
      form.content = rec.content
      form.is_urgent = rec.is_urgent
      form.display_consent = rec.display_consent
      form.service_id = rec.service_id
      form.status = rec.status
    } catch (err) {
      Notify.create({ type: 'negative', message: errorMessage(err) })
      router.back()
    } finally {
      loadingRecord.value = false
    }
  }
})
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.consent-group {
  :deep(.q-radio__label) {
    font-size: 13px;
  }
}

.consent-note {
  background: rgba(255, 85, 0, 0.08);
  border-left: 3px solid $brand-orange;
  border-radius: 8px;
  font-size: 11.5px;
  color: #b33c00;
  line-height: 1.45;
}
</style>
