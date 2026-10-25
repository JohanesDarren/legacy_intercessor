<template>
  <q-page class="li-page">
    <div class="row items-center justify-between q-mb-md">
      <div>
        <div class="li-eyebrow">{{ t('services.subtitle') }}</div>
      </div>
      <q-btn
        v-if="auth.isAdmin"
        unelevated
        dense
        no-caps
        color="primary"
        text-color="white"
        icon="add"
        :label="t('services.create')"
        @click="openForm"
      />
    </div>

    <div v-if="loading" class="column items-center q-py-xl">
      <q-spinner color="primary" size="28px" />
    </div>

    <div v-else-if="services.length === 0" class="empty-state">
      <q-icon name="church" size="30px" class="li-muted" />
      <p class="li-muted q-mt-sm" style="font-size: 13px">{{ t('app.empty') }}</p>
    </div>

    <q-card v-for="svc in services" :key="svc.id" flat class="li-card q-mb-sm">
      <q-card-section class="q-pa-md">
        <div class="row items-start justify-between no-wrap">
          <div style="min-width: 0">
            <div class="row items-center q-gutter-xs q-mb-xs">
              <span class="li-badge" :class="isPast(svc) ? 'li-badge--muted' : 'li-badge--gold'">
                {{ isPast(svc) ? t('services.past') : t('services.upcoming') }}
              </span>
            </div>
            <div style="font-weight: 600; font-size: 14px">{{ formatDateLong(svc.starts_at) }}</div>
            <div class="li-muted q-mt-xs" style="font-size: 11.5px">
              <q-icon name="schedule" size="12px" class="q-mr-xs" />{{ formatTimeWib(svc.starts_at) }}
              · {{ svc.location }}
            </div>
            <div v-if="svc.slide_deadline" class="li-muted q-mt-xs" style="font-size: 11.5px">
              <q-icon name="timer" size="12px" class="q-mr-xs" />
              {{ t('services.slideDeadline') }}: {{ formatDateTime(svc.slide_deadline) }} WIB
            </div>
          </div>
          <q-btn
            v-if="auth.isAdmin"
            flat
            dense
            round
            icon="edit"
            size="sm"
            @click="openEdit(svc)"
          />
        </div>
      </q-card-section>
    </q-card>

    <!-- Form dialog -->
    <q-dialog v-model="showForm" position="bottom">
      <q-card class="sheet">
        <q-card-section class="row items-center justify-between q-pb-sm">
          <div class="li-display" style="font-size: 16px">
            {{ editing ? t('services.title') : t('services.create') }}
          </div>
          <q-btn flat dense round icon="close" @click="showForm = false" />
        </q-card-section>
        <q-card-section class="q-pt-none">
          <div class="row q-col-gutter-sm q-mb-md">
            <div class="col-7">
              <q-input v-model="form.date" type="date" outlined dense :label="t('services.startsAt')" />
            </div>
            <div class="col-5">
              <q-input v-model="form.time" type="time" outlined dense label="WIB" />
            </div>
          </div>
          <q-input
            v-model="form.location"
            outlined
            dense
            :label="t('services.location')"
            class="q-mb-md"
          />
          <div class="row q-col-gutter-sm">
            <div class="col-7">
              <q-input
                v-model="form.deadlineDate"
                type="date"
                outlined
                dense
                :label="t('services.slideDeadline')"
              />
            </div>
            <div class="col-5">
              <q-input v-model="form.deadlineTime" type="time" outlined dense label="WIB" />
            </div>
          </div>
          <div class="li-muted q-mt-xs" style="font-size: 11px">
            {{ t('services.deadlineHint') }}
          </div>
        </q-card-section>
        <q-card-actions class="q-pa-md">
          <q-btn flat no-caps :label="t('app.cancel')" @click="showForm = false" />
          <q-space />
          <q-btn
            unelevated
            no-caps
            color="primary"
            text-color="white"
            :label="t('app.save')"
            :loading="saving"
            @click="save"
          />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Notify } from 'quasar'
import api from '@/services/api'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { errorMessage } from '@/boot/axios'
import { formatDateLong, formatDateTime, formatTimeWib, toDateInput, toIso } from '@/utils/format'
import type { Service } from '@/services/types'

const { t } = useI18n()
const app = useAppStore()
const auth = useAuthStore()

const services = ref<Service[]>([])
const loading = ref(true)
const showForm = ref(false)
const saving = ref(false)
const editing = ref<Service | null>(null)

const form = reactive({
  date: '',
  time: '17:00',
  location: 'GBI Aruna, Bandung',
  deadlineDate: '',
  deadlineTime: '12:00',
})

function isPast(svc: Service) {
  return new Date(svc.starts_at).getTime() < Date.now()
}

function openForm() {
  editing.value = null
  const now = new Date()
  form.date = toDateInput(now.toISOString())
  form.time = '17:00'
  form.location = 'GBI Aruna, Bandung'
  form.deadlineDate = toDateInput(now.toISOString())
  form.deadlineTime = '12:00'
  showForm.value = true
}

function openEdit(svc: Service) {
  editing.value = svc
  form.date = toDateInput(svc.starts_at)
  form.time = new Intl.DateTimeFormat('en-GB', {
    timeZone: 'Asia/Jakarta',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(svc.starts_at))
  form.location = svc.location
  if (svc.slide_deadline) {
    form.deadlineDate = toDateInput(svc.slide_deadline)
    form.deadlineTime = new Intl.DateTimeFormat('en-GB', {
      timeZone: 'Asia/Jakarta',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }).format(new Date(svc.slide_deadline))
  }
  showForm.value = true
}

async function save() {
  if (!form.date) {
    Notify.create({ type: 'warning', message: t('errors.validation') })
    return
  }
  saving.value = true
  try {
    const deadline = form.deadlineDate
      ? toIso(form.deadlineDate, form.deadlineTime || '12:00')
      : undefined

    if (editing.value) {
      await api.updateService(editing.value.id, {
        location: form.location,
        slide_deadline: deadline,
      })
    } else {
      await api.createService({
        starts_at: toIso(form.date, form.time || '17:00'),
        location: form.location,
        slide_deadline: deadline,
      })
    }
    showForm.value = false
    Notify.create({ type: 'positive', message: t('services.savedOk'), timeout: 1600 })
    await load()
    await app.loadServices()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  } finally {
    saving.value = false
  }
}

async function load() {
  loading.value = true
  try {
    services.value = await api.listServices(true)
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
    services.value = []
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>

<style scoped lang="scss">
.sheet {
  width: 100%;
  border-radius: 18px 18px 0 0;
  max-width: 560px;
  margin: 0 auto;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 40px 16px;
}
</style>
