<template>
  <q-card
    flat
    class="li-card li-card--tap q-mb-sm"
    :class="cardClass"
    @click="$emit('click')"
  >
    <q-card-section class="q-pa-md">
      <div class="row items-start no-wrap">
        <div class="col" style="min-width: 0">
          <div class="row items-center q-gutter-xs q-mb-xs">
            <span class="li-eyebrow">{{ categoryName }}</span>
            <span class="li-eyebrow" style="opacity: 0.32">·</span>
            <span class="li-eyebrow">{{ formatDate(date) }}</span>
          </div>

          <div
            class="li-truncate-2"
            style="font-weight: 600; font-size: 15px; line-height: 1.32"
          >
            {{ title }}
          </div>

          <div
            v-if="subtitle"
            class="li-muted li-truncate-2 q-mt-xs"
            style="font-size: 13px; line-height: 1.45"
          >
            {{ subtitle }}
          </div>

          <div class="row items-center q-gutter-xs q-mt-sm">
            <UrgentBadge :is-urgent="isUrgent" :consent="consent" />
            <span v-if="statusLabel" class="li-badge li-badge--gold">{{ statusLabel }}</span>
            <span v-if="featured" class="li-badge li-badge--gold">
              <q-icon name="star" size="11px" />
              {{ t('praise.featured') }}
            </span>
            <span v-if="followUpHint" class="li-badge li-badge--neutral">
              <q-icon name="history" size="11px" />
              {{ followUpHint }}
            </span>
          </div>
        </div>

        <q-icon name="chevron_right" size="20px" class="li-muted q-ml-sm" style="opacity: 0.3" />
      </div>
    </q-card-section>
  </q-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import UrgentBadge from './UrgentBadge.vue'
import { formatDate, statusLabel as statusText } from '@/utils/format'

const props = defineProps<{
  title: string
  subtitle?: string | null
  categoryName: string
  date: string
  isUrgent?: boolean
  consent?: string
  status?: string
  featured?: boolean
  followUpCount?: number
  kind?: 'prayer' | 'praise'
}>()

defineEmits<{ click: [] }>()

const { t } = useI18n()

const cardClass = computed(() => {
  if (props.isUrgent) return 'li-card--urgent'
  return props.kind === 'praise' ? 'li-card--praise' : 'li-card--prayer'
})

const statusLabel = computed(() => {
  if (!props.status) return ''
  return statusText(props.status)
})

const followUpHint = computed(() => {
  if (props.followUpCount === undefined) return ''
  if (props.followUpCount === 0) return t('dashboard.neverFollowedUp')
  return `${props.followUpCount}×`
})
</script>
