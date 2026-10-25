/**
 * The single HTTP client. Components never call axios directly
 * (PRD 7.3 frontend standards).
 *
 * Responsibilities:
 *  - attach the access token
 *  - transparently refresh once on 401 and replay the request
 *  - honour Retry-After on 429 instead of retrying immediately
 *  - surface the API error envelope as a typed ApiErrorException
 */

import axios, {
  type AxiosError,
  type AxiosInstance,
  type AxiosRequestConfig,
  type InternalAxiosRequestConfig,
} from 'axios'
import type {
  AiStatus,
  ApiEnvelope,
  ApiError,
  ApiListEnvelope,
  Category,
  DashboardSummary,
  Export,
  ExportFormat,
  FollowUp,
  Invite,
  InvitePreview,
  Kind,
  MetaVersion,
  Notification,
  PraiseReport,
  PrayerRequest,
  Presentation,
  Service,
  Shift,
  Summary,
  SwapRequest,
  TokenPair,
  User,
} from './types'

// Quasar injects build.env values as process.env.* at build time.
const BASE_URL = process.env.API_BASE_URL || '/api/v1'

const ACCESS_KEY = 'li.access'
const REFRESH_KEY = 'li.refresh'

/** ApiErrorException wraps the server's error envelope. */
export class ApiErrorException extends Error {
  code: string
  status: number
  fields?: Record<string, string[]>

  constructor(message: string, code: string, status: number, fields?: Record<string, string[]>) {
    super(message)
    this.name = 'ApiErrorException'
    this.code = code
    this.status = status
    this.fields = fields
  }
}

// ---------------------------------------------------------------- TOKEN STORE
export const tokenStore = {
  get access(): string | null {
    return localStorage.getItem(ACCESS_KEY)
  },
  get refresh(): string | null {
    return localStorage.getItem(REFRESH_KEY)
  },
  set(pair: { access_token: string; refresh_token: string }) {
    localStorage.setItem(ACCESS_KEY, pair.access_token)
    localStorage.setItem(REFRESH_KEY, pair.refresh_token)
  },
  clear() {
    localStorage.removeItem(ACCESS_KEY)
    localStorage.removeItem(REFRESH_KEY)
  },
}

// ---------------------------------------------------------------- CLIENT
const http: AxiosInstance = axios.create({
  baseURL: BASE_URL,
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

http.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = tokenStore.access
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// A single in-flight refresh shared by concurrent 401s, so a burst of requests
// does not rotate the refresh token several times and revoke itself.
let refreshing: Promise<string> | null = null

async function refreshAccessToken(): Promise<string> {
  const refresh = tokenStore.refresh
  if (!refresh) throw new ApiErrorException('Session expired', 'UNAUTHORIZED', 401)

  const { data } = await axios.post<ApiEnvelope<TokenPair>>(
    `${BASE_URL}/auth/refresh`,
    { refresh_token: refresh },
    { headers: { 'Content-Type': 'application/json' }, timeout: 15000 },
  )
  tokenStore.set(data.data)
  return data.data.access_token
}

http.interceptors.response.use(
  (response) => response,
  async (error: AxiosError<ApiError>) => {
    const original = error.config as (AxiosRequestConfig & { _retried?: boolean }) | undefined
    const status = error.response?.status
    const payload = error.response?.data

    // 401: try exactly one silent refresh, then replay.
    if (status === 401 && original && !original._retried && tokenStore.refresh) {
      original._retried = true
      try {
        refreshing = refreshing ?? refreshAccessToken()
        const token = await refreshing
        refreshing = null
        original.headers = { ...(original.headers ?? {}), Authorization: `Bearer ${token}` }
        return http.request(original)
      } catch {
        refreshing = null
        tokenStore.clear()
        window.dispatchEvent(new CustomEvent('li:session-expired'))
        throw new ApiErrorException('Your session expired. Please sign in again.', 'UNAUTHORIZED', 401)
      }
    }

    // 429: read Retry-After so the UI can wait instead of hammering (PRD 6.4).
    if (status === 429) {
      const retryAfter = Number(error.response?.headers?.['retry-after'] ?? 60)
      throw new ApiErrorException(
        `Too many requests. Please wait ${retryAfter} seconds.`,
        'RATE_LIMITED',
        429,
      )
    }

    if (payload?.status === 'error') {
      throw new ApiErrorException(
        payload.message,
        payload.error_code,
        status ?? 500,
        payload.errors,
      )
    }

    if (error.code === 'ECONNABORTED') {
      throw new ApiErrorException('The request timed out. Check your connection.', 'TIMEOUT', 0)
    }

    throw new ApiErrorException(
      'Cannot reach the server. Check your connection and try again.',
      'NETWORK_ERROR',
      status ?? 0,
    )
  },
)

// ---------------------------------------------------------------- HELPERS
type Query = Record<string, string | number | boolean | undefined | null>

function clean(params?: Query): Query | undefined {
  if (!params) return undefined
  const out: Query = {}
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') out[k] = v
  }
  return out
}

// ---------------------------------------------------------------- API
export const api = {
  // ---- meta
  async health() {
    const { data } = await http.get<ApiEnvelope<{ status: string; time: string }>>('/health')
    return data.data
  },
  async metaVersion() {
    const { data } = await http.get<ApiEnvelope<MetaVersion>>('/meta/version')
    return data.data
  },

  // ---- auth
  async login(email: string, password: string) {
    const { data } = await http.post<ApiEnvelope<TokenPair>>('/auth/login', { email, password })
    tokenStore.set(data.data)
    return data.data
  },
  async logout() {
    const refresh = tokenStore.refresh
    try {
      if (refresh) await http.post('/auth/logout', { refresh_token: refresh })
    } finally {
      tokenStore.clear()
    }
  },
  async previewInvite(token: string) {
    const { data } = await http.get<ApiEnvelope<InvitePreview>>(`/auth/invites/${token}`)
    return data.data
  },
  async acceptInvite(token: string, password: string, fullName?: string) {
    const { data } = await http.post<ApiEnvelope<TokenPair>>('/auth/accept-invite', {
      token,
      password,
      full_name: fullName,
    })
    tokenStore.set(data.data)
    return data.data
  },
  async createInvite(email: string, fullName: string, role: string) {
    const { data } = await http.post<ApiEnvelope<Invite>>('/auth/invites', {
      email,
      full_name: fullName,
      role,
    })
    return data.data
  },

  // ---- users
  async me() {
    const { data } = await http.get<ApiEnvelope<User>>('/users/me')
    return data.data
  },
  async updateMe(payload: { full_name?: string; current_password?: string; new_password?: string }) {
    const { data } = await http.patch<ApiEnvelope<User>>('/users/me', payload)
    return data.data
  },
  async acknowledgePrivacy() {
    await http.post('/users/me/privacy-ack')
  },
  async exportMyData() {
    const { data } = await http.get<ApiEnvelope<Record<string, unknown>>>('/users/me/export')
    return data.data
  },
  async deleteMe() {
    await http.delete('/users/me')
  },
  async listUsers() {
    const { data } = await http.get<ApiEnvelope<User[]>>('/users')
    return data.data
  },
  async setUserActive(id: string, isActive: boolean) {
    const { data } = await http.patch<ApiEnvelope<{ id: string; is_active: boolean }>>(
      `/users/${id}/active`,
      { is_active: isActive },
    )
    return data.data
  },

  // ---- services & categories
  async listServices(includePast = false) {
    const { data } = await http.get<ApiEnvelope<Service[]>>('/services', {
      params: clean({ include_past: includePast }),
    })
    return data.data
  },
  async nextService() {
    const { data } = await http.get<ApiEnvelope<Service>>('/services/next')
    return data.data
  },
  async createService(payload: { starts_at: string; location?: string; slide_deadline?: string }) {
    const { data } = await http.post<ApiEnvelope<Service>>('/services', payload)
    return data.data
  },
  async updateService(
    id: string,
    payload: { location?: string; slide_deadline?: string; notes?: string },
  ) {
    const { data } = await http.patch<ApiEnvelope<Service>>(`/services/${id}`, payload)
    return data.data
  },
  async listCategories(kind?: Kind) {
    const { data } = await http.get<ApiEnvelope<Category[]>>('/categories', {
      params: clean({ kind }),
    })
    return data.data
  },

  // ---- prayer requests
  async listPrayerRequests(params?: Query) {
    const { data } = await http.get<ApiListEnvelope<PrayerRequest>>('/prayer-requests', {
      params: clean(params),
    })
    return data
  },
  async getPrayerRequest(id: string) {
    const { data } = await http.get<ApiEnvelope<PrayerRequest>>(`/prayer-requests/${id}`)
    return data.data
  },
  async createPrayerRequest(payload: Record<string, unknown>) {
    const { data } = await http.post<ApiEnvelope<PrayerRequest>>('/prayer-requests', payload)
    return data.data
  },
  async updatePrayerRequest(id: string, payload: Record<string, unknown>) {
    const { data } = await http.patch<ApiEnvelope<PrayerRequest>>(`/prayer-requests/${id}`, payload)
    return data.data
  },
  async deletePrayerRequest(id: string) {
    await http.delete(`/prayer-requests/${id}`)
  },

  // ---- follow-ups
  async listFollowUps(requestId: string) {
    const { data } = await http.get<ApiEnvelope<FollowUp[]>>(
      `/prayer-requests/${requestId}/follow-ups`,
    )
    return data.data
  },
  async createFollowUp(requestId: string, payload: Record<string, unknown>) {
    const { data } = await http.post<ApiEnvelope<FollowUp>>(
      `/prayer-requests/${requestId}/follow-ups`,
      payload,
    )
    return data.data
  },
  async recentFollowUps(limit = 20) {
    const { data } = await http.get<ApiEnvelope<FollowUp[]>>('/follow-ups', {
      params: clean({ limit }),
    })
    return data.data
  },

  // ---- praise reports
  async listPraiseReports(params?: Query) {
    const { data } = await http.get<ApiListEnvelope<PraiseReport>>('/praise-reports', {
      params: clean(params),
    })
    return data
  },
  async getPraiseReport(id: string) {
    const { data } = await http.get<ApiEnvelope<PraiseReport>>(`/praise-reports/${id}`)
    return data.data
  },
  async createPraiseReport(payload: Record<string, unknown>) {
    const { data } = await http.post<ApiEnvelope<PraiseReport>>('/praise-reports', payload)
    return data.data
  },
  async updatePraiseReport(id: string, payload: Record<string, unknown>) {
    const { data } = await http.patch<ApiEnvelope<PraiseReport>>(`/praise-reports/${id}`, payload)
    return data.data
  },
  async deletePraiseReport(id: string) {
    await http.delete(`/praise-reports/${id}`)
  },

  // ---- AI summaries
  async generateSummaries(payload: {
    service_id: string
    kind: Kind
    language?: 'id' | 'en'
    max_words_per_item?: number
  }) {
    const { data } = await http.post<ApiEnvelope<Summary[]>>('/ai/summaries', payload)
    return data.data
  },
  async listSummaries(serviceId: string, kind?: Kind) {
    const { data } = await http.get<ApiEnvelope<Summary[]>>('/ai/summaries', {
      params: clean({ service_id: serviceId, kind }),
    })
    return data.data
  },
  async reviewSummary(id: string, payload: { final_text?: string; status?: string }) {
    const { data } = await http.patch<ApiEnvelope<Summary>>(`/ai/summaries/${id}`, payload)
    return data.data
  },
  async aiStatus() {
    const { data } = await http.get<ApiEnvelope<AiStatus>>('/ai/status')
    return data.data
  },

  // ---- presentations & exports
  async listPresentations(params?: { service_id?: string; kind?: Kind }) {
    const { data } = await http.get<ApiEnvelope<Presentation[]>>('/presentations', {
      params: clean(params),
    })
    return data.data
  },
  async getPresentation(id: string) {
    const { data } = await http.get<ApiEnvelope<Presentation>>(`/presentations/${id}`)
    return data.data
  },
  async buildPresentation(payload: { service_id: string; kind: Kind; note?: string }) {
    const { data } = await http.post<ApiEnvelope<Presentation>>('/presentations', payload)
    return data.data
  },
  async updatePresentation(id: string, slides: Presentation['slides']) {
    const { data } = await http.patch<ApiEnvelope<Presentation>>(`/presentations/${id}`, { slides })
    return data.data
  },
  async startExport(presentationId: string, format: ExportFormat) {
    const { data } = await http.post<ApiEnvelope<Export>>(
      `/presentations/${presentationId}/exports`,
      { format },
    )
    return data.data
  },
  async getExport(presentationId: string, exportId: string) {
    const { data } = await http.get<ApiEnvelope<Export>>(
      `/presentations/${presentationId}/exports/${exportId}`,
    )
    return data.data
  },
  async listExports(presentationId: string) {
    const { data } = await http.get<ApiEnvelope<Export[]>>(
      `/presentations/${presentationId}/exports`,
    )
    return data.data
  },
  async latestExport(serviceId: string, kind: Kind) {
    const { data } = await http.get<ApiEnvelope<Export>>('/exports/latest', {
      params: clean({ service_id: serviceId, kind }),
    })
    return data.data
  },

  /** Polls an export job until it finishes or the deadline passes. */
  async waitForExport(
    presentationId: string,
    exportId: string,
    onTick?: (e: Export) => void,
    maxSeconds = 60,
  ): Promise<Export> {
    const deadline = Date.now() + maxSeconds * 1000
    let delay = 700
    for (;;) {
      const job = await api.getExport(presentationId, exportId)
      onTick?.(job)
      if (job.status === 'done' || job.status === 'failed') return job
      if (Date.now() > deadline) return job
      await new Promise((r) => setTimeout(r, delay))
      delay = Math.min(delay + 400, 2500)
    }
  },

  // ---- booth
  async listShifts(serviceId: string) {
    const { data } = await http.get<ApiEnvelope<Shift[]>>(`/services/${serviceId}/booth-shifts`)
    return data.data
  },
  async createShift(
    serviceId: string,
    payload: { label: string; starts_at: string; ends_at: string; capacity: number },
  ) {
    const { data } = await http.post<ApiEnvelope<Shift>>(
      `/services/${serviceId}/booth-shifts`,
      payload,
    )
    return data.data
  },
  async publishRoster(serviceId: string) {
    const { data } = await http.post<ApiEnvelope<{ shifts_published: number }>>(
      `/services/${serviceId}/booth-roster/publish`,
    )
    return data.data
  },
  async assignToShift(shiftId: string, userId: string) {
    const { data } = await http.post<ApiEnvelope<Record<string, unknown>>>(
      `/booth-shifts/${shiftId}/assignments`,
      { user_id: userId },
    )
    return data.data
  },
  async unassign(assignmentId: string) {
    await http.delete(`/booth-assignments/${assignmentId}`)
  },
  async requestSwap(assignmentId: string, targetUserId: string) {
    const { data } = await http.post<ApiEnvelope<SwapRequest>>(
      `/booth-assignments/${assignmentId}/swap-requests`,
      { target_user_id: targetUserId },
    )
    return data.data
  },
  async decideSwap(swapId: string, status: 'approved' | 'declined') {
    const { data } = await http.patch<ApiEnvelope<SwapRequest>>(`/swap-requests/${swapId}`, {
      status,
    })
    return data.data
  },
  async listSwaps(status?: string) {
    const { data } = await http.get<ApiEnvelope<SwapRequest[]>>('/swap-requests', {
      params: clean({ status }),
    })
    return data.data
  },
  async myShifts() {
    const { data } = await http.get<ApiEnvelope<Shift[]>>('/booth-shifts/mine')
    return data.data
  },
  async rotationSuggestion() {
    const { data } = await http.get<
      ApiEnvelope<Array<{ user_id: string; full_name: string; shifts_last_4_weeks: number }>>
    >('/booth-shifts/rotation')
    return data.data
  },

  // ---- dashboard
  async dashboard() {
    const { data } = await http.get<ApiEnvelope<DashboardSummary>>('/dashboard/summary')
    return data.data
  },
  async notifications() {
    const { data } = await http.get<ApiEnvelope<Notification[]>>('/notifications')
    return data.data
  },
  async markNotificationRead(id: string) {
    await http.post(`/notifications/${id}/read`)
  },
}

export default api
