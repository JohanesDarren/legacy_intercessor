<template>
  <div class="row items-center no-wrap q-gutter-xs" style="overflow-x: auto; padding-bottom: 4px">
    <q-chip
      v-for="chip in chips"
      :key="chip.key"
      :selected="chip.active"
      :removable="chip.active && chip.removable !== false"
      clickable
      dense
      square
      class="li-chip"
      :class="{ 'li-chip--on': chip.active }"
      @click="$emit('toggle', chip.key, chip.value)"
      @remove="$emit('toggle', chip.key, chip.value)"
    >
      {{ chip.label }}
    </q-chip>

    <q-chip
      v-if="hasActive"
      dense
      square
      clickable
      class="li-chip li-chip--reset"
      @click="$emit('clear')"
    >
      <q-icon name="close" size="13px" class="q-mr-xs" />
      {{ t('app.reset') }}
    </q-chip>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

export interface ChipDef {
  key: string
  value: string | boolean
  label: string
  active: boolean
  removable?: boolean
}

const props = defineProps<{
  chips: ChipDef[]
}>()

defineEmits<{
  toggle: [key: string, value: string | boolean]
  clear: []
}>()

const { t } = useI18n()
const hasActive = computed(() => props.chips.some((c) => c.active))
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.li-chip {
  font-size: 11.5px;
  letter-spacing: 0.3px;
  background: rgba(0, 0, 0, 0.045);
  color: rgba(0, 0, 0, 0.72);
  border: 1px solid transparent;
  min-height: 30px;
  flex-shrink: 0;
}

.li-chip--on {
  background: $brand-black;
  color: $brand-white;
  border-color: $brand-black;
}

.li-chip--reset {
  background: transparent;
  border-color: rgba(0, 0, 0, 0.2);
  color: rgba(0, 0, 0, 0.6);
}
</style>
