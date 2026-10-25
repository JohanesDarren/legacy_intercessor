<template>
  <div class="li-slide-preview" :class="`li-slide-preview--${slide.template}`">
    <div class="prev-frame">
      <div class="prev-brand">
        <span class="prev-dot" />
        <span class="prev-brand__name">LEGACY</span>
        <span class="prev-brand__sub">SUKAWARNA</span>
      </div>

      <div class="prev-title">{{ slide.title }}</div>
      <div v-if="slide.note" class="prev-note">{{ slide.note }}</div>
      <div class="prev-rule" />

      <div class="prev-items">
        <div
          v-for="(item, idx) in slide.items"
          :key="idx"
          class="prev-item"
          :class="{ 'prev-item--urgent': item.is_urgent }"
        >
          <div class="prev-item__body">
            <div class="prev-item__title">{{ item.title }}</div>
            <div class="prev-item__text">{{ item.body }}</div>
          </div>
          <div class="prev-item__meta">
            <span v-if="item.name" class="prev-item__name">{{ item.name }}</span>
            <span v-if="item.is_urgent" class="prev-item__badge">URGENT</span>
          </div>
        </div>
      </div>

      <div class="prev-foot">
        <span>Legacy Sukawarna</span>
        <span>{{ pageLabel }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Slide } from '@/services/types'

const props = defineProps<{
  slide: Slide
  index?: number
  total?: number
}>()

const pageLabel = computed(() => {
  if (props.index === undefined || props.total === undefined) return ''
  return `${props.index + 1} / ${props.total}`
})
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

/* The preview scales the 1920x1080 layout down with container query units so it
   stays visually faithful to the exported file. */
.prev-frame {
  position: absolute;
  inset: 0;
  padding: 6.2%;
  display: flex;
  flex-direction: column;
}

.prev-brand {
  display: flex;
  align-items: baseline;
  gap: 0.6em;
  font-family: $typography-font-family-display;
  margin-bottom: 3%;
}

.prev-dot {
  width: 0.55em;
  height: 0.55em;
  border-radius: 50%;
  background: $brand-orange;
  display: inline-block;
}

.prev-brand__name {
  font-size: clamp(6px, 1.5cqw, 13px);
  font-weight: 700;
  letter-spacing: 2.5px;
  color: $identity-gold;
}

.prev-brand__sub {
  font-size: clamp(5px, 1.05cqw, 9px);
  font-weight: 700;
  letter-spacing: 3px;
  opacity: 0.5;
}

.prev-title {
  font-family: $typography-font-family-display;
  font-size: clamp(12px, 4.2cqw, 34px);
  font-weight: 700;
  line-height: 1.05;
  text-transform: uppercase;
  color: $identity-gold;
}

.prev-note {
  font-size: clamp(6px, 1.5cqw, 12px);
  line-height: 1.35;
  margin-top: 1.4%;
  opacity: 0.7;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
}

.prev-rule {
  width: 10%;
  height: 2px;
  background: $identity-gold;
  margin: 2% 0 3%;
}

.prev-items {
  display: flex;
  flex-direction: column;
  gap: 1.6%;
  flex: 1;
  overflow: hidden;
}

.prev-item {
  display: flex;
  justify-content: space-between;
  gap: 2%;
  padding: 1.5% 2%;
  border-left: 3px solid transparent;
  background: rgba(128, 128, 128, 0.12);
}

.li-slide-preview--praise .prev-item {
  background: rgba(0, 0, 0, 0.05);
}

.prev-item--urgent {
  border-left-color: $brand-orange;
}

.prev-item__body {
  flex: 1;
  min-width: 0;
}

.prev-item__title {
  font-family: $typography-font-family-display;
  font-size: clamp(7px, 1.9cqw, 15px);
  font-weight: 700;
  line-height: 1.16;
  margin-bottom: 0.4em;
}

.prev-item__text {
  font-size: clamp(6px, 1.5cqw, 12px);
  line-height: 1.32;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
}

.prev-item__meta {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 0.4em;
  flex-shrink: 0;
}

.prev-item__name {
  font-family: $typography-font-family-display;
  font-size: clamp(5px, 1.1cqw, 9px);
  font-weight: 700;
  letter-spacing: 0.5px;
  text-transform: uppercase;
  color: $identity-gold;
  white-space: nowrap;
}

.prev-item__badge {
  background: $brand-orange;
  color: $brand-black;
  font-family: $typography-font-family-display;
  font-size: clamp(4px, 0.95cqw, 8px);
  font-weight: 700;
  letter-spacing: 1px;
  padding: 0.3em 0.55em;
  white-space: nowrap;
}

.prev-foot {
  display: flex;
  justify-content: space-between;
  font-size: clamp(5px, 1.05cqw, 9px);
  letter-spacing: 1px;
  text-transform: uppercase;
  opacity: 0.45;
  margin-top: 2%;
}

.li-slide-preview {
  container-type: inline-size;
}
</style>
