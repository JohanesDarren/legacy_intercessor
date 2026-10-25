import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '@/services/api'
import type {
  Category,
  DashboardSummary,
  Kind,
  Service,
} from '@/services/types'

/**
 * Shared app data: the active service, the category lookup, and the dashboard
 * payload. Filters live in the URL (PRD F-05) so they are not stored here.
 */
export const useAppStore = defineStore('app', () => {
  const services = ref<Service[]>([])
  const activeServiceId = ref<string | null>(null)
  const categories = ref<Category[]>([])
  const dashboard = ref<DashboardSummary | null>(null)
  const loadingServices = ref(false)
  const loadingDashboard = ref(false)

  const activeService = computed(
    () => services.value.find((s) => s.id === activeServiceId.value) ?? services.value[0] ?? null,
  )

  const prayerCategories = computed(() => categories.value.filter((c) => c.kind === 'prayer'))
  const praiseCategories = computed(() => categories.value.filter((c) => c.kind === 'praise'))

  function categoriesFor(kind: Kind) {
    return kind === 'praise' ? praiseCategories.value : prayerCategories.value
  }

  function categoryName(id: string): string {
    return categories.value.find((c) => c.id === id)?.name ?? '—'
  }

  async function loadServices() {
    loadingServices.value = true
    try {
      services.value = await api.listServices()
      if (!activeServiceId.value && services.value.length > 0) {
        activeServiceId.value = services.value[0].id
      }
      return services.value
    } finally {
      loadingServices.value = false
    }
  }

  async function loadCategories(force = false) {
    if (categories.value.length > 0 && !force) return categories.value
    categories.value = await api.listCategories()
    return categories.value
  }

  async function loadDashboard() {
    loadingDashboard.value = true
    try {
      dashboard.value = await api.dashboard()
      return dashboard.value
    } finally {
      loadingDashboard.value = false
    }
  }

  function setActiveService(id: string) {
    activeServiceId.value = id
  }

  /** Loads the lookups that most screens need. */
  async function bootstrap() {
    await Promise.all([loadServices(), loadCategories()])
  }

  return {
    services,
    activeServiceId,
    activeService,
    categories,
    prayerCategories,
    praiseCategories,
    dashboard,
    loadingServices,
    loadingDashboard,
    categoriesFor,
    categoryName,
    loadServices,
    loadCategories,
    loadDashboard,
    setActiveService,
    bootstrap,
  }
})
