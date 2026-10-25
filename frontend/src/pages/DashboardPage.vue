<template>
  <q-page class="li-page">
    <!-- Greeting -->
    <div class="row items-end justify-between q-mb-md">
      <div>
        <div class="li-eyebrow">{{ t('dashboard.greeting') }}</div>
        <div class="li-display" style="font-size: 21px; line-height: 1.15">
          {{ firstName }}
        </div>
      </div>
      <div class="li-eyebrow li-tabular">{{ todayLabel }}</div>
    </div>

    <div v-if="loading && !dash" class="column items-center q-py-xl">
      <q-spinner color="primary" size="30px" />
    </div>

    <template v-else-if="dash">
      <!-- Deadline warning banner (F-02 acceptance criteria) -->
      <q-banner
        v-if="dash.deadline_status.warning"
        dense
        class="warning-banner q-mb-md"
        :class="{ 'warning-banner--critical': dash.next_service?.deadline_passed }"
      >
        <template #avatar>
          <q-icon
            :name="dash.next_service?.deadline_passed ? 'error_outline' : 'schedule'"
            size="19px"
          />
        </template>
        <div class="row items-center no-wrap full-width">
          <span style="font-size: 12.5px; line-height: 1.4; flex: 1">
            {{ dash.deadline_status.warning }}
          </span>
          <q-btn
            v-if="auth.canManageSlides"
            flat
            dense
            no-caps
            size="sm"
            class="q-ml-sm"
            :label="dash.deadline_status.presentation_exists ? t('dashboard.reviewSummaries') : t('dashboard.buildSlides')"
            :to="{ name: dash.deadline_status.presentation_exists ? 'presentations' : 'summaries' }"
          />
        </div>
      </q-banner>

      <!-- Next service + deadline -->
      <q-card v-if="dash.next_service" flat class="li-card q-mb-md">
        <q-card-section class="q-pa-md">
          <div class="row items-start justify-between no-wrap">
            <div style="min-width: 0">
              <div class="li-eyebrow">{{ t('dashboard.nextService') }}</div>
              <div class="li-display q-mt-xs" style="font-size: 17px; line-height: 1.2">
                {{ formatDateLong(dash.next_service.starts_at) }}
              </div>
              <div class="row items-center q-gutter-xs q-mt-xs li-muted" style="font-size: 12.5px">
                <q-icon name="schedule" size="13px" />
                <span>{{ formatTimeWib(dash.next_service.starts_at) }}</span>
                <q-icon name="place" size="13px" class="q-ml-xs" />
                <span>{{ dash.next_service.location }}</span>
              </div>
            </div>
            <div
              class="status-pill"
              :class="dash.deadline_status.exported ? 'status-pill--ok' : 'status-pill--warn'"
            >
              <q-icon
                :name="dash.deadline_status.exported ? 'check_circle' : 'pending'"
                size="12px"
              />
              {{ dash.deadline_status.exported ? t('dashboard.exported') : t('dashboard.notExported') }}
            </div>
          </div>

          <!-- Deadline countdown -->
          <div v-if="dash.next_service.slide_deadline" class="deadline-row q-mt-md">
            <div class="row items-center justify-between no-wrap">
              <div class="row items-center q-gutter-xs">
                <q-icon name="timer" size="15px" class="li-muted" />
                <span class="li-eyebrow" style="opacity: 0.5">
                  {{ t('dashboard.slideDeadline') }}
                </span>
              </div>
              <span
                class="li-display li-tabular"
                :class="{ 'text-negative': countdown.urgent || countdown.passed }"
                style="font-size: 14px"
              >
                {{ countdown.text }}
              </span>
            </div>
            <div class="row items-center q-gutter-xs q-mt-xs">
              <q-icon name="event" size="12px" class="li-muted" />
              <span class="li-muted" style="font-size: 11.5px">
                {{ formatDateTime(dash.next_service.slide_deadline) }} WIB
              </span>
            </div>
          </div>
        </q-card-section>
      </q-card>

      <!-- KPI row -->
      <div class="row q-col-gutter-sm q-mb-md">
        <div class="col-6 col-md-3">
          <q-card flat class="li-card kpi-card kpi-card--urgent">
            <q-card-section class="q-pa-md">
              <div class="li-kpi" :style="{ color: dash.urgent_count > 0 ? '#FF5500' : undefined }">
                {{ dash.urgent_count }}
              </div>
              <div class="li-eyebrow q-mt-xs">{{ t('dashboard.urgentCount') }}</div>
            </q-card-section>
          </q-card>
        </div>
        <div class="col-6 col-md-3">
          <q-card flat class="li-card kpi-card">
            <q-card-section class="q-pa-md">
              <div class="li-kpi">{{ dash.prayer_this_week }}</div>
              <div class="li-eyebrow q-mt-xs">{{ t('dashboard.prayerThisWeek') }}</div>
            </q-card-section>
          </q-card>
        </div>
        <div class="col-6 col-md-3">
          <q-card flat class="li-card kpi-card">
            <q-card-section class="q-pa-md">
              <div class="li-kpi">{{ dash.praise_this_week }}</div>
              <div class="li-eyebrow q-mt-xs">{{ t('dashboard.praiseThisWeek') }}</div>
            </q-card-section>
          </q-card>
        </div>
        <div class="col-6 col-md-3">
          <q-card flat class="li-card kpi-card">
            <q-card-section class="q-pa-md">
              <div class="li-kpi">{{ dash.draft_summary_count }}</div>
              <div class="li-eyebrow q-mt-xs">{{ t('dashboard.draftSummaries') }}</div>
            </q-card-section>
          </q-card>
        </div>
      </div>

      <!-- My next booth shift -->
      <q-card v-if="auth.canReadRecords" flat class="li-card q-mb-md">
        <q-card-section class="q-pa-md">
          <div class="row items-center justify-between no-wrap">
            <div class="li-eyebrow">{{ t('dashboard.myNextShift') }}</div>
            <q-btn
              flat
              dense
              no-caps
              size="sm"
              color="primary"
              :label="t('nav.booth')"
              :to="{ name: 'booth' }"
            />
          </div>

          <div v-if="dash.my_next_shift" class="q-mt-sm">
            <div class="row items-center q-gutter-sm">
              <q-icon name="event_seat" size="18px" style="color: #e5ad38" />
              <div>
                <div style="font-weight: 600; font-size: 14px">
                  {{ dash.my_next_shift.label }}
                </div>
                <div class="li-muted" style="font-size: 12px">
                  {{ formatDateLong(dash.my_next_shift.starts_at) }} ·
                  {{ formatTime(dash.my_next_shift.starts_at) }}–{{
                    formatTime(dash.my_next_shift.ends_at)
                  }}
                  WIB
                </div>
              </div>
            </div>
          </div>
          <div v-else class="li-muted q-mt-xs" style="font-size: 12.5px">
            {{ t('dashboard.noShift') }}
          </div>
        </q-card-section>
      </q-card>

      <!-- Urgent requests needing follow-up -->
      <div v-if="auth.canReadRecords">
        <div class="row items-center justify-between q-mb-sm">
          <div class="li-eyebrow" style="opacity: 0.75">
            {{ t('dashboard.urgentNeedFollowUp') }}
          </div>
          <q-btn
            flat
            dense
            no-caps
            size="sm"
            color="primary"
            :label="t('nav.prayer')"
            :to="{ name: 'prayer-list', query: { urgent: 'true' } }"
          />
        </div>

        <div v-if="dash.urgent_requests.length === 0" class="empty-state">
          <q-icon name="check_circle_outline" size="26px" class="li-muted" />
          <p class="li-muted q-mt-sm" style="font-size: 12.5px">
            {{ t('dashboard.allClear') }}
          </p>
        </div>

        <q-card
          v-for="item in dash.urgent_requests"
          :key="item.id"
          flat
          class="li-card li-card--tap li-card--urgent q-mb-sm"
          @click="openRequest(item.id)"
        >
          <q-card-section class="q-pa-md">
            <div class="row items-start no-wrap">
              <div class="col" style="min-width: 0">
                <div class="row items-center q-gutter-xs q-mb-xs">
                  <span class="li-badge li-badge--urgent">URGENT</span>
                  <span class="li-eyebrow">{{ item.category_name }}</span>
                </div>
                <div
                  class="li-truncate-2"
                  style="font-weight: 600; font-size: 14px; line-height: 1.3"
                >
                  {{ item.title }}
                </div>
                <div class="row items-center q-gutter-xs q-mt-xs">
                  <q-icon name="history" size="12px" class="li-muted" />
                  <span class="li-muted" style="font-size: 11.5px">
                    {{
                      item.days_since_follow_up >= 999
                        ? t('dashboard.neverFollowedUp')
                        : t('dashboard.daysAgo', { n: item.days_since_follow_up })
                    }}
                  </span>
                  <span v-if="item.requester_name" class="li-muted" style="font-size: 11.5px">
                    · {{ item.requester_name }}
                  </span>
                </div>
              </div>
              <q-icon name="chevron_right" size="19px" class="li-muted q-ml-sm" style="opacity: 0.3" />
            </div>
          </q-card-section>
        </q-card>
      </div>

      <!-- Quick actions -->
      <div v-if="auth.canWriteRecords" class="row q-col-gutter-sm q-mt-md">
        <div class="col-6">
          <q-btn
            unelevated
            no-caps
            color="primary"
            text-color="white"
            icon="add"
            class="full-width"
            :label="t('prayer.new')"
            :to="{ name: 'prayer-new' }"
            style="height: 46px"
          />
        </div>
        <div class="col-6">
          <q-btn
            outline
            no-caps
            color="primary"
            icon="auto_awesome"
            class="full-width"
            :label="t('praise.new')"
            :to="{ name: 'praise-new' }"
            style="height: 46px"
          />
        </div>
      </div>
    </template>

    <!-- Error state -->
    <div v-else class="column items-center q-py-xl">
      <q-icon name="cloud_off" size="34px" class="li-muted" />
      <p class="li-muted q-mt-md" style="font-size: 13px">{{ errorMsg }}</p>
      <q-btn flat no-caps color="primary" :label="t('app.retry')" @click="load" />
    </div>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { errorMessage } from '@/boot/axios'
import {
  deadlineCountdown,
  formatDateLong,
  formatDateTime,
  formatTime,
  formatTimeWib,
} from '@/utils/format'

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const app = useAppStore()

const loading = ref(true)
const errorMsg = ref('')

const dash = computed(() => app.dashboard)
const firstName = computed(() => auth.user?.full_name?.split(' ')[0] ?? '')

const countdown = computed(() =>
  deadlineCountdown(dash.value?.next_service?.slide_deadline ?? null),
)

const todayLabel = computed(() =>
  new Intl.DateTimeFormat('id-ID', {
    timeZone: 'Asia/Jakarta',
    weekday: 'long',
    day: 'numeric',
    month: 'long',
  }).format(new Date()),
)

async function load() {
  loading.value = true
  errorMsg.value = ''
  try {
    await app.loadDashboard()
  } catch (err) {
    errorMsg.value = errorMessage(err)
  } finally {
    loading.value = false
  }
}

function openRequest(id: string) {
  router.push({ name: 'prayer-detail', params: { id } })
}

onMounted(load)
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.warning-banner {
  background: rgba(229, 173, 56, 0.14);
  border-left: 3px solid $identity-gold;
  border-radius: 10px;
  color: rgba(0, 0, 0, 0.82);
}

.warning-banner--critical {
  background: rgba(255, 85, 0, 0.12);
  border-left-color: $brand-orange;
}

.kpi-card {
  height: 100%;
}

.kpi-card--urgent {
  border-color: rgba(255, 85, 0, 0.2);
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.6px;
  text-transform: uppercase;
  padding: 5px 8px;
  border-radius: 5px;
  white-space: nowrap;
  flex-shrink: 0;
}

.status-pill--ok {
  background: rgba(27, 122, 62, 0.12);
  color: #1b7a3e;
}

.status-pill--warn {
  background: rgba(255, 85, 0, 0.13);
  color: #b33c00;
}

.deadline-row {
  border-top: 1px solid rgba(0, 0, 0, 0.07);
  padding-top: 12px;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 26px 16px;
  background: rgba(0, 0, 0, 0.02);
  border-radius: 12px;
}
</style>
