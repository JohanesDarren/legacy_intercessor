<template>
  <div class="center-wrap">
    <q-icon name="search_off" size="42px" class="li-muted" />
    <h1 class="li-display q-mt-md" style="font-size: 20px">{{ t('errors.notFound') }}</h1>
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
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 24px;
}
</style>
