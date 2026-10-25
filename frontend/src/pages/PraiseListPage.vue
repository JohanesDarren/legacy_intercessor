<template>
  <q-page class="li-page">
    <div class="row items-center no-wrap q-gutter-sm q-mb-sm">
      <q-input
        v-model="search"
        dense
        outlined
        clearable
        :placeholder="t('praise.searchPlaceholder')"
        class="col"
        debounce="350"
        @update:model-value="applyFilters"
      >
        <template #prepend><q-icon name="search" size="18px" /></template>
      </q-input>
      <q-btn
        outline
        dense
        no-caps
        color="primary"
        icon="tune"
        class="filter-btn"
        @click="showFilters = true"
      >
        <q-badge v-if="activeFilterCount > 0" color="accent" text-color="black" floating rounded>
          {{ activeFilterCount }}
        </q-badge>
      </q-btn>
    </div>

    <FilterChips :chips="quickChips" class="q-mb-sm" @toggle="toggleQuick" @clear="clearFilters" />

    <div class="row items-center justify-between q-mb-sm">
      <span class="li-eyebrow li-tabular" style="opacity: 0.5">
        {{ t('praise.resultCount', { n: total }) }}
      </span>
      <q-btn flat dense no-caps size="sm" color="primary" @click="showSort = true">
        <q-icon name="sort" size="15px" class="q-mr-xs" />
        <span style="font-size: 11.5px">{{ sortLabel }}</span>
      </q-btn>
    </div>

    <div v-if="loading" class="column items-center q-py-xl">
      <q-spinner color="primary" size="28px" />
    </div>

    <div v-else-if="items.length === 0" class="empty-state">
      <q-icon name="auto_awesome" size="30px" class="li-muted" />
      <p class="li-muted q-mt-sm" style="font-size: 13px">{{ t('praise.noneFound') }}</p>
      <q-btn
        v-if="hasActiveFilters"
        flat
        no-caps
        color="primary"
        size="sm"
        :label="t('app.reset')"
        @click="clearFilters"
      />
    </div>

    <template v-else>
      <RecordCard
        v-for="item in items"
        :key="item.id"
        :title="item.title"
        :subtitle="item.content"
        :category-name="item.category_name"
        :date="item.reported_at"
        :consent="item.display_consent"
        :featured="item.is_featured"
        kind="praise"
        @click="open(item.id)"
      />

      <div v-if="totalPages > 1" class="row items-center justify-center q-gutter-sm q-mt-md">
        <q-btn flat dense round icon="chevron_left" :disable="page <= 1" @click="goPage(page - 1)" />
        <span class="li-muted li-tabular" style="font-size: 12.5px">{{ page }} / {{ totalPages }}</span>
        <q-btn
          flat
          dense
          round
          icon="chevron_right"
          :disable="page >= totalPages"
          @click="goPage(page + 1)"
        />
      </div>
    </template>

    <q-page-sticky position="bottom-right" :offset="[18, 18]">
      <q-btn
        fab
        unelevated
        color="primary"
        text-color="white"
        icon="add"
        :to="{ name: 'praise-new' }"
        aria-label="Tambah laporan pujian"
      />
    </q-page-sticky>

    <!-- Filters -->
    <q-dialog v-model="showFilters" position="bottom">
      <q-card class="filter-sheet">
        <q-card-section class="row items-center justify-between q-pb-sm">
          <div class="li-display" style="font-size: 16px">{{ t('app.filter') }}</div>
          <q-btn flat dense round icon="close" @click="showFilters = false" />
        </q-card-section>

        <q-card-section class="q-pt-none scroll" style="max-height: 60vh">
          <div class="li-eyebrow q-mb-xs">{{ t('prayer.filterConsent') }}</div>
          <div class="row q-gutter-xs q-mb-md">
            <q-chip
              v-for="opt in consentOptions"
              :key="opt.value"
              :selected="filters.consent === opt.value"
              clickable
              dense
              square
              class="li-chip"
              :class="{ 'li-chip--on': filters.consent === opt.value }"
              @click="setFilter('consent', opt.value)"
            >
              {{ opt.label }}
            </q-chip>
          </div>

          <div class="li-eyebrow q-mb-xs">{{ t('prayer.filterGender') }}</div>
          <div class="row q-gutter-xs q-mb-md">
            <q-chip
              v-for="opt in genderOptions"
              :key="opt.value"
              :selected="filters.gender === opt.value"
              clickable
              dense
              square
              class="li-chip"
              :class="{ 'li-chip--on': filters.gender === opt.value }"
              @click="setFilter('gender', opt.value)"
            >
              {{ opt.label }}
            </q-chip>
          </div>

          <div class="li-eyebrow q-mb-xs">{{ t('praise.category') }}</div>
          <div class="row q-gutter-xs">
            <q-chip
              v-for="cat in app.praiseCategories"
              :key="cat.id"
              :selected="filters.category_id === cat.id"
              clickable
              dense
              square
              class="li-chip"
              :class="{ 'li-chip--on': filters.category_id === cat.id }"
              @click="setFilter('category_id', cat.id)"
            >
              {{ cat.name }}
            </q-chip>
          </div>
        </q-card-section>

        <q-card-actions class="q-pa-md">
          <q-btn flat no-caps color="primary" :label="t('app.reset')" @click="clearFilters" />
          <q-space />
          <q-btn
            unelevated
            no-caps
            color="primary"
            text-color="white"
            :label="t('app.save')"
            @click="showFilters = false"
          />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Sort -->
    <q-dialog v-model="showSort" position="bottom">
      <q-card style="width: 100%; border-radius: 18px 18px 0 0">
        <q-list padding>
          <q-item v-for="opt in sortOptions" :key="opt.value" clickable v-ripple @click="setSort(opt.value)">
            <q-item-section>{{ opt.label }}</q-item-section>
            <q-item-section v-if="filters.sort === opt.value" side>
              <q-icon name="check" color="primary" size="18px" />
            </q-item-section>
          </q-item>
        </q-list>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Notify } from 'quasar'
import RecordCard from '@/components/RecordCard.vue'
import FilterChips, { type ChipDef } from '@/components/FilterChips.vue'
import api from '@/services/api'
import { useAppStore } from '@/stores/app'
import { errorMessage } from '@/boot/axios'
import type { PraiseReport } from '@/services/types'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const app = useAppStore()

const items = ref<PraiseReport[]>([])
const total = ref(0)
const totalPages = ref(1)
const loading = ref(true)
const showFilters = ref(false)
const showSort = ref(false)

const filters = reactive({
  q: '',
  consent: '',
  gender: '',
  category_id: '',
  featured: '' as '' | 'true',
  sort: 'reported_at_desc',
})

const search = ref('')

const sortOptions = computed(() => [
  { value: 'reported_at_desc', label: t('prayer.sortNewest') },
  { value: 'reported_at_asc', label: t('prayer.sortOldest') },
  { value: 'is_featured_desc', label: t('praise.filterFeatured') },
  { value: 'category_asc', label: t('prayer.sortCategory') },
])

const consentOptions = computed(() => [
  { value: '', label: t('app.all') },
  { value: 'yes', label: t('prayer.consentYes') },
  { value: 'anonymous', label: t('prayer.consentAnonymous') },
  { value: 'no', label: t('prayer.consentNo') },
])

const genderOptions = computed(() => [
  { value: '', label: t('app.all') },
  { value: 'male', label: t('prayer.male') },
  { value: 'female', label: t('prayer.female') },
  { value: 'unspecified', label: t('prayer.unspecified') },
])

const sortLabel = computed(() => sortOptions.value.find((o) => o.value === filters.sort)?.label ?? '')
const page = computed(() => Number(route.query.page ?? 1))

const activeFilterCount = computed(() => {
  let n = 0
  if (filters.consent) n++
  if (filters.gender) n++
  if (filters.category_id) n++
  if (filters.featured) n++
  return n
})

const hasActiveFilters = computed(() => activeFilterCount.value > 0 || !!filters.q)

const quickChips = computed<ChipDef[]>(() => [
  {
    key: 'featured',
    value: 'true',
    label: t('praise.filterFeatured'),
    active: filters.featured === 'true',
  },
  {
    key: 'consent_yes',
    value: 'yes',
    label: t('prayer.consentYes'),
    active: filters.consent === 'yes',
  },
])

function readFromUrl() {
  const q = route.query
  filters.q = (q.q as string) ?? ''
  filters.consent = (q.consent as string) ?? ''
  filters.gender = (q.gender as string) ?? ''
  filters.category_id = (q.category_id as string) ?? ''
  filters.featured = (q.featured as string) === 'true' ? 'true' : ''
  filters.sort = (q.sort as string) ?? 'reported_at_desc'
  search.value = filters.q
}

function applyFilters() {
  const query: Record<string, string> = {}
  if (search.value) query.q = search.value
  if (filters.consent) query.consent = filters.consent
  if (filters.gender) query.gender = filters.gender
  if (filters.category_id) query.category_id = filters.category_id
  if (filters.featured) query.featured = filters.featured
  if (filters.sort && filters.sort !== 'reported_at_desc') query.sort = filters.sort
  router.replace({ query }).catch(() => {
    /* noop */
  })
}

function setFilter(key: 'consent' | 'gender' | 'category_id', value: string) {
  filters[key] = filters[key] === value ? '' : value
  applyFilters()
}

function toggleQuick(key: string, value: string | boolean) {
  const v = String(value)
  if (key === 'featured') {
    filters.featured = filters.featured === 'true' ? '' : 'true'
  } else if (key === 'consent_yes') {
    filters.consent = filters.consent === v ? '' : v
  }
  applyFilters()
}

function setSort(value: string) {
  filters.sort = value
  showSort.value = false
  applyFilters()
}

function clearFilters() {
  filters.consent = ''
  filters.gender = ''
  filters.category_id = ''
  filters.featured = ''
  filters.sort = 'reported_at_desc'
  search.value = ''
  router.replace({ query: {} }).catch(() => {
    /* noop */
  })
}

function goPage(n: number) {
  router.replace({ query: { ...route.query, page: String(n) } })
}

async function load() {
  loading.value = true
  try {
    const [sortBy, sortOrder] = filters.sort.split('_')
    const res = await api.listPraiseReports({
      page: page.value,
      limit: 20,
      q: filters.q || undefined,
      consent: filters.consent || undefined,
      gender: filters.gender || undefined,
      category_id: filters.category_id || undefined,
      featured: filters.featured === 'true' ? 'true' : undefined,
      sort_by: sortBy === 'reported' ? 'reported_at' : sortBy,
      sort_order: sortOrder,
    })
    items.value = res.data
    total.value = res.meta.total
    totalPages.value = res.meta.total_pages
  } catch (err) {
    Notify.create({ type: 'negative', message: errorMessage(err) })
    items.value = []
  } finally {
    loading.value = false
  }
}

function open(id: string) {
  router.push({ name: 'praise-edit', params: { id } })
}

watch(
  () => route.query,
  () => {
    readFromUrl()
    load()
  },
)

onMounted(async () => {
  readFromUrl()
  await app.loadCategories()
  await load()
})
</script>

<style scoped lang="scss">
@import '@/css/quasar.variables.scss';

.filter-btn {
  min-width: 44px;
  height: 40px;
}

.filter-sheet {
  width: 100%;
  border-radius: 18px 18px 0 0;
  max-width: 560px;
  margin: 0 auto;
}

.li-chip {
  font-size: 11.5px;
  background: rgba(0, 0, 0, 0.045);
  color: rgba(0, 0, 0, 0.72);
  min-height: 30px;
}

.li-chip--on {
  background: $brand-black;
  color: $brand-white;
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 40px 16px;
}
</style>
