<template>
  <div class="center-wrap">
    <q-icon name="lock" size="42px" class="li-muted" />
    <h1 class="li-display q-mt-md" style="font-size: 20px">{{ t('errors.forbidden') }}</h1>
    <p class="li-muted q-mt-xs text-center" style="font-size: 13px; max-width: 320px">
      {{ t('auth.signInSubtitle') }}
    </p>
    <q-btn
      unelevated
      no-caps
      color="primary"
      text-color="white"
      :label="t('nav.dashboard')"
      class="q-mt-lg"
      :to="{ name: 'dashboard' }"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>

<style scoped lang="scss">
.center-wrap {
  min-height: 70vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
</style>
