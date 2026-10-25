<template>
  <span v-if="isUrgent" class="li-badge li-badge--urgent">
    <q-icon name="priority_high" size="11px" />
    URGENT
  </span>
  <span v-else-if="consent === 'anonymous'" class="li-badge li-badge--neutral">
    <q-icon name="visibility_off" size="11px" />
    {{ t('prayer.consentAnonymous') }}
  </span>
  <span v-else-if="consent === 'no'" class="li-badge li-badge--muted">
    <q-icon name="block" size="11px" />
    {{ t('prayer.consentNo') }}
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  isUrgent?: boolean
  consent?: string
}>()

const { t } = useI18n()
</script>
