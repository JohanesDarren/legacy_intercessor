<template>
  <q-page class="li-page">
    <!-- Service picker -->
    <div class="row q-col-gutter-sm q-mb-md">
      <div class="col-12 col-sm-7">
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
      <div class="col-12 col-sm-5">
        <div class="row q-gutter-sm">
          <q-btn
            v-if="auth.canManageRoster"
            outline
            no-caps
            dense
            color="primary"
            icon="add"
            class="col"
            style="height: 40px"
            :label="t('booth.createShift')"
            @click="openShiftForm"
          />
          <q-btn
            v-if="auth.canManageRoster"
            unelevated
            no-caps
            dense
            color="primary"
            text-color="white"
            icon="publish"
            class="col"
            style="height: 40px"
            :label="t('booth.publish')"
            :disable="shifts.length === 0"
            @click="publish"
          />
        </div>
      </div>
    </div>

    <!-- Pending swap requests (admin) -->
    <q-card v-if="auth.isAdmin && pendingSwaps.length > 0" flat class="swap-card q-mb-md">
      <q-card-section class="q-pa-md">
        <div class="li-eyebrow q-mb-sm">{{ t('booth.pendingSwaps') }}</div>
        <div v-for="sw in pendingSwaps" :key="sw.id" class="swap-row">
          <div style="flex: 1; min-width: 0">
            <div style="font-size: 12.5px; font-weight: 600">
              {{ sw.requester_name }} → {{ sw.target_name }}
            </div>
            <div class="li-muted" style="font-size: 11px">
              {{ sw.shift_label }} · {{ formatDate(sw.shift_starts_at) }}
            </div>
          </div>
          <q-btn
            flat
            dense
            round
            icon="check"
            color="positive"
            size="sm"
            @click="decide(sw.id, 'approved')"
          />
          <q-btn
            flat
            dense
            round
            icon="close"
            color="negative"
            size="sm"
            @click="decide(sw.id, 'declined')"
          />
        </div>
      </q-card-section>
    </q-card>

    <!-- Loading -->
    <div v-if="loading" class="column items-center q-py-xl">
      <q-spinner color="primary" size="28px" />
    </div>

    <!-- Empty -->
    <div v-else-if="shifts.length === 0" class="empty-state">
      <q-icon name="event_seat" size="30px" class="li-muted" />
      <p class="li-muted q-mt-sm" style="font-size: 13px">{{ t('booth.noShifts') }}</p>
      <q-btn
        v-if="auth.canManageRoster"
        unelevated
        no-caps
        color="primary"
        text-color="white"
        :label="t('booth.createShift')"
        class="q-mt-sm"
        @click="openShiftForm"
      />
    </div>

    <!-- Shifts -->
    <template v-else>
      <!-- Coverage summary -->
      <div class="coverage-bar q-mb-md">
        <div class="row items-center justify-between no-wrap">
          <span class="li-eyebrow" style="opacity: 0.6">
            {{ coveredCount }} / {{ shifts.length }} {{ t('booth.covered') }}
          </span>
          <span v-if="allPublished" class="li-badge li-badge--gold">
            <q-icon name="check_circle" size="11px" />
            {{ t('booth.published') }}
          </span>
          <span v-else class="li-badge li-badge--muted">{{ t('booth.unpublished') }}</span>
        </div>
      </div>

      <q-card v-for="shift in shifts" :key="shift.id" flat class="li-card q-mb-sm">
        <q-card-section class="q-pa-md">
          <!-- Shift header -->
          <div class="row items-start justify-between no-wrap q-mb-sm">
            <div style="min-width: 0">
              <div class="row items-center q-gutter-xs">
                <span style="font-weight: 700; font-size: 14px">{{ shift.label }}</span>
                <span
                  class="li-badge"
                  :class="shift.covered ? 'li-badge--gold' : 'li-badge--urgent'"
                >
                  {{ shift.covered ? t('booth.covered') : t('booth.uncovered') }}
                </span>
                <span v-if="shift.overbooked" class="li-badge li-badge--urgent">
                  {{ t('booth.overbooked') }}
                </span>
              </div>
              <div class="li-muted q-mt-xs" style="font-size: 11.5px">
                {{ formatTime(shift.starts_at) }}–{{ formatTime(shift.ends_at) }} WIB ·
                {{ shift.assignees.length }}/{{ shift.capacity }}
              </div>
            </div>
            <q-btn
              v-if="auth.canManageRoster && shift.assignees.length < shift.capacity"
              flat
              dense
              no-caps
              size="sm"
              color="primary"
              icon="person_add"
              :label="t('booth.assign')"
              @click="openAssign(shift)"
            />
          </div>

          <!-- Assignees -->
          <div v-if="shift.assignees.length === 0" class="li-muted" style="font-size: 12px">
            {{ t('booth.uncovered') }}
          </div>
          <div v-else class="row q-gutter-xs">
            <div
              v-for="a in shift.assignees"
              :key="a.assignment_id"
              class="assignee-chip"
              :class="{ 'assignee-chip--swapping': a.status === 'swap_requested' }"
            >
              <q-avatar size="22px" color="grey-3" text-color="grey-9" style="font-size: 10px">
                {{ initials(a.full_name) }}
              </q-avatar>
              <span class="assignee-chip__name">{{ a.full_name }}</span>
              <q-icon
                v-if="a.status === 'swap_requested'"
                name="swap_horiz"
                size="12px"
                class="q-ml-xs"
              />
              <q-btn
                v-if="auth.canManageRoster"
                flat
                dense
                round
                icon="close"
                size="xs"
                class="q-ml-xs"
                @click="unassign(a.assignment_id)"
              />
              <q-btn
                v-else-if="a.user_id === auth.user?.id && a.status === 'assigned'"
                flat
                dense
                round
                icon="swap_horiz"
                size="xs"
                class="q-ml-xs"
                @click="openSwap(a)"
              />
            </div>
          </div>
        </q-card-section>
      </q-card>

      <!-- Rotation helper (admin) -->
      <q-card v-if="auth.isAdmin && rotation.length > 0" flat class="li-card q-mt-md">
        <q-card-section class="q-pa-md">
          <div class="li-eyebrow">{{ t('booth.rotation') }}</div>
          <div class="li-muted q-mb-sm" style="font-size: 11px; line-height: 1.4">
            {{ t('booth.rotationHint') }}
          </div>
          <div v-for="(r, i) in rotation" :key="r.user_id" class="rotation-row">
            <span class="rotation-rank">{{ i + 1 }}</span>
            <span style="flex: 1; font-size: 12.5px">{{ r.full_name }}</span>
            <span class="li-muted li-tabular" style="font-size: 11px">
              {{ t('booth.shiftsLast4Weeks', { n: r.shifts_last_4_weeks }) }}
            </span>
          </div>
        </q-card-section>
      </q-card>
    </template>

    <!-- Create shift dialog -->
    <q-dialog v-model="showShiftForm" position="bottom">
      <q-card class="sheet">
        <q-card-section class="row items-center justify-between q-pb-sm">
          <div class="li-display" style="font-size: 16px">{{ t('booth.createShift') }}</div>
          <q-btn flat dense round icon="close" @click="showShiftForm = false" />
        </q-card-section>
        <q-card-section class="q-pt-none">
          <q-input
            v-model="shiftForm.label"
            outlined
            dense
            :label="t('booth.shiftLabel')"
            placeholder="Pre-service"
            class="q-mb-md"
          />
          <div class="row q-col-gutter-sm q-mb-md">
            <div class="col-6">
              <q-input v-model="shiftForm.date" type="date" outlined dense :label="t('services.startsAt')" />
            </div>
            <div class="col-3">
              <q-input v-model="shiftForm.start" type="time" outlined dense :label="t('booth.startsAt')" />
            </div>
            <div class="col-3">
              <q-input v-model="shiftForm.end" type="time" outlined dense :label="t('booth.endsAt')" />
            </div>
          </div>
          <div class="li-eyebrow q-mb-xs">{{ t('booth.capacity') }}</div>
          <div class="row q-gutter-xs">
            <q-chip
              v-for="n in [1, 2, 3, 4]"
              :key="n"
              :selected="shiftForm.capacity === n"
              clickable
              dense
              square
              class="cap-chip"
              :class="{ 'cap-chip--on': shiftForm.capacity === n }"
              @click="shiftForm.capacity = n"
            >
              {{ n }}
            </q-chip>
          </div>
        </q-card-section>
        <q-card-actions class="q-pa-md">
          <q-btn flat no-caps :label="t('app.cancel')" @click="showShiftForm = false" />
          <q-space />
          <q-btn
            unelevated
            no-caps
            color="primary"
            text-color="white"
            :label="t('app.save')"
            :loading="savingShift"
            @click="saveShift"
          />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Assign dialog -->
    <q-dialog v-model="showAssign" position="bottom">
      <q-card class="sheet">
        <q-card-section class="row items-center justify-between q-pb-sm">
          <div class="li-display" style="font-size: 16px">{{ t('booth.assign') }}</div>
          <q-btn flat dense round icon="close" @click="showAssign = false" />
        </q-card-section>
        <q-card-section class="q-pt-none">
          <div
            v-for="u in assignableUsers"
            :key="u.id"
            class="assign-row"
            @click="assign(u.id)"
          >
            <q-avatar size="28px" color="grey-3" text-color="grey-9" style="font-size: 11px">
              {{ initials(u.full_name) }}
            </q-avatar>
            <div style="flex: 1; min-width: 0">
              <div style="font-size: 13px; font-weight: 600">{{ u.full_name }}</div>
              <div class="li-muted" style="font-size: 11px">{{ roleLabel(u.role) }}</div>
            </div>
            <q-icon name="add_circle_outline" size="18px" color="primary" />
          </div>
          <div v-if="assignableUsers.length === 0" class="li-muted q-py-md text-center" style="font-size: 12.5px">
            {{ t('app.none') }}
          </div>
        </q-card-section>
      </q-card>
    </q-dialog>

    <!-- Swap dialog -->
    <q-dialog v-model="showSwap" position="bottom">
      <q-card class="sheet">
        <q-card-section class="row items-center justify-between q-pb-sm">
          <div class="li-display" style="font-size: 16px">{{ t('booth.requestSwap') }}</div>
          <q-btn flat dense round icon="close" @click="showSwap = false" />
        </q-card-section>
        <q-card-section class="q-pt-none">
          <div
            v-for="u in swapCandidates"
            :key="u.id"
            class="assign-row"
            @click="requestSwap(u.id)"
          >
            <q-avatar size="28px" color="grey-3" text-color="grey-9" style="font-size: 11px">
              {{ initials(u.full_name) }}
            </q-avatar>
            <div style="flex: 1; min-width: 0">
              <div style="font-size: 13px; font-weight: 600">{{ u.full_name }}</div>
            </div>
            <q-icon name="swap_horiz" size="18px" color="primary" />
          </div>
          <div v-if="swapCandidates.length === 0" class="li-muted q-py-md text-center" style="font-size: 12.5px">
            {{ t('app.none') }}
          </div>
        </q-card-section>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Notify } from 'quasar'
import api, { ApiErrorException } from '@/services/api'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { errorMessage } from '@/boot/axios'
import { formatDate, formatDateLong, formatTime, initials, roleLabel, toDateInput, toIso } from '@/utils/format'
import type { Assignee, Shift, SwapRequest, User } from '@/services/types'

const { t } = useI18n()
const route = useRoute()
const app = useAppStore()
const auth = useAuthStore()

const serviceId = ref('')
const shifts = ref<Shift[]>([])
const users = ref<User[]>([])
const swaps = ref<SwapRequest[]>([])
const rotation = ref<Array<{ user_id: string; full_name: string; shifts_last_4_weeks: number }>>([])
const loading = ref(true)

const showShiftForm = ref(false)
const showAssign = ref(false)
const showSwap = ref(false)
const savingShift = ref(false)
const activeShift = ref<Shift | null>(null)
const activeAssignment = ref<Assignee | null>(null)

const shiftForm = reactive({
  label: 'Pre-service',
  date: '',
  start: '16:00',
  end: '17:00',
  capacity: 2,
})

const serviceOptions = computed(() =>
  app.services.map((s) => ({ id: s.id, label: formatDateLong(s.starts_at) })),
)

const coveredCount = computed(() => shifts.value.filter((s) => s.covered).length)
const allPublished = computed(
  () => shifts.value.length > 0 && shifts.value.every((s) => s.is_published),
)
const pendingSwaps = computed(() => swaps.value.filter((s) => s.status === 'pending'))

const intercessors = computed(() => users.value.filter((u) => u.role === 'intercessor' && u.is_active))

/** Intercessors not already on the active shift. */
const assignableUsers = computed(() => {
  if (!activeShift.value) return intercessors.value
  const taken = new Set(activeShift.value.assignees.map((a) => a.user_id))
  return intercessors.value.filter((u) => !taken.has(u.id))
})

const swapCandidates = computed(() =>
  intercessors.value.filter((u) => u.id !== auth.user?.id),
)

async function load() {
  if (!serviceId.value) return
  loading.value = true
  try {
    shifts.value = await api.listShifts(serviceId.value)
    if (auth.isAdmin) {
      try {
        swaps.value = await api.listSwaps('pending')
      } catch {
        swaps.value = []
      }
      try {
        rotation.value = await api.rotationSuggestion()
      } catch {
        rotation.value = []
      }
    }
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
    shifts.value = []
  } finally {
    loading.value = false
  }
}

function openShiftForm() {
  const svc = app.services.find((s) => s.id === serviceId.value)
  shiftForm.date = toDateInput(svc?.starts_at ?? new Date().toISOString())
  shiftForm.label = 'Pre-service'
  shiftForm.start = '16:00'
  shiftForm.end = '17:00'
  shiftForm.capacity = 2
  showShiftForm.value = true
}

async function saveShift() {
  if (!shiftForm.label.trim() || !shiftForm.date) {
    Notify.create({ type: 'warning', message: t('errors.validation') })
    return
  }
  savingShift.value = true
  try {
    await api.createShift(serviceId.value, {
      label: shiftForm.label.trim(),
      starts_at: toIso(shiftForm.date, shiftForm.start),
      ends_at: toIso(shiftForm.date, shiftForm.end),
      capacity: shiftForm.capacity,
    })
    showShiftForm.value = false
    Notify.create({ type: 'positive', message: t('app.saved'), timeout: 1600 })
    await load()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  } finally {
    savingShift.value = false
  }
}

function openAssign(shift: Shift) {
  activeShift.value = shift
  showAssign.value = true
}

async function assign(userId: string) {
  if (!activeShift.value) return
  try {
    await api.assignToShift(activeShift.value.id, userId)
    showAssign.value = false
    Notify.create({ type: 'positive', message: t('booth.assigned'), timeout: 1600 })
    await load()
  } catch (err) {
    if (err instanceof ApiErrorException && err.code === 'SHIFT_OVERBOOKED') {
      Notify.create({ type: 'negative', message: t('errors.capacity'), timeout: 4000 })
    } else {
      Notify.create({ type: 'negative', message: errorMessage(err) })
    }
  }
}

async function unassign(assignmentId: string) {
  try {
    await api.unassign(assignmentId)
    await load()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  }
}

async function publish() {
  try {
    const res = await api.publishRoster(serviceId.value)
    Notify.create({
      type: 'positive',
      message: `${t('booth.published')} (${res.shifts_published})`,
      timeout: 2000,
    })
    await load()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  }
}

function openSwap(a: Assignee) {
  activeAssignment.value = a
  showSwap.value = true
}

async function requestSwap(targetUserId: string) {
  if (!activeAssignment.value) return
  try {
    await api.requestSwap(activeAssignment.value.assignment_id, targetUserId)
    showSwap.value = false
    Notify.create({ type: 'positive', message: t('booth.swapRequested'), timeout: 2000 })
    await load()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  }
}

async function decide(swapId: string, status: 'approved' | 'declined') {
  try {
    await api.decideSwap(swapId, status)
    Notify.create({
      type: 'positive',
      message: status === 'approved' ? t('booth.swapApproved') : t('booth.swapDeclined'),
      timeout: 1800,
    })
    await load()
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
  }
}

onMounted(async () => {
  await app.bootstrap()
  serviceId.value = (route.query.service_id as string) || app.activeService?.id || ''

  if (auth.isAdmin) {
    try {
      users.value = await api.listUsers()
    } catch {
      users.value = []
    }
  } else {
    // Intercessors only need the list to pick a swap target.
    try {
      users.value = await api.listUsers()
    } catch {
      users.value = []
    }
  }

  await load()
})
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.swap-card {
  border: 1px solid rgba(255, 85, 0, 0.24);
  background: rgba(255, 85, 0, 0.04);
  border-radius: 12px;
}

.swap-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 0;
  border-top: 1px solid rgba(0, 0, 0, 0.06);

  &:first-of-type {
    border-top: 0;
  }
}

.coverage-bar {
  background: rgba(0, 0, 0, 0.03);
  border-radius: 9px;
  padding: 9px 12px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 40px 16px;
}

.assignee-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 20px;
  padding: 3px 6px 3px 3px;
}

.assignee-chip--swapping {
  background: rgba(255, 85, 0, 0.12);
}

.assignee-chip__name {
  font-size: 11.5px;
  font-weight: 600;
  white-space: nowrap;
}

.rotation-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 0;
}

.rotation-rank {
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.06);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 9.5px;
  font-weight: 700;
}

.sheet {
  width: 100%;
  border-radius: 18px 18px 0 0;
  max-width: 560px;
  margin: 0 auto;
}

.cap-chip {
  font-size: 12px;
  background: rgba(0, 0, 0, 0.045);
  min-height: 32px;
  min-width: 36px;
  justify-content: center;
}

.cap-chip--on {
  background: $brand-black;
  color: $brand-white;
}

.assign-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 4px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
  cursor: pointer;

  &:active {
    background: rgba(0, 0, 0, 0.04);
  }
}
</style>
