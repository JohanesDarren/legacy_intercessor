/**
 * Typed API models. These mirror the Go DTOs and the PRD Section 6.4 contract.
 */

export type Role = 'admin' | 'intercessor' | 'viewer'
export type Gender = 'male' | 'female' | 'unspecified'
export type Consent = 'yes' | 'anonymous' | 'no'
export type PrayerStatus = 'open' | 'in_prayer' | 'answered' | 'closed'
export type Kind = 'prayer' | 'praise'
export type SummaryStatus = 'draft' | 'approved' | 'rejected'
export type ExportFormat = 'png' | 'pdf'
export type ExportStatus = 'queued' | 'running' | 'done' | 'failed'
export type ConditionStatus =
  | 'improving'
  | 'stable'
  | 'worsening'
  | 'answered'
  | 'unreachable'
export type ContactChannel = 'visit' | 'phone' | 'whatsapp' | 'other'
export type UpdateType =
  | 'documentation'
  | 'life_update'
  | 'current_condition'
  | 'other'

export interface PageMeta {
  page: number
  limit: number
  total: number
  total_pages: number
  has_next: boolean
  has_prev: boolean
}

export interface ApiEnvelope<T> {
  status: 'success'
  message: string
  data: T
}

export interface ApiListEnvelope<T> {
  status: 'success'
  message: string
  data: T[]
  meta: PageMeta
}

export interface ApiError {
  status: 'error'
  message: string
  error_code: string
  errors?: Record<string, string[]>
}

export interface User {
  id: string
  full_name: string
  email: string
  role: Role
  is_active: boolean
  last_login_at: string | null
  privacy_ack_at: string | null
  created_at: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
  token_type: string
  expires_in: number
  expires_at: string
  user: User
}

export interface Service {
  id: string
  starts_at: string
  location: string
  slide_deadline: string | null
  notes: string | null
  created_at: string
  updated_at: string
}

export interface Category {
  id: string
  kind: Kind
  name: string
  sort_order: number
  created_at: string
}

export interface PrayerRequest {
  id: string
  service_id: string | null
  category_id: string
  category_name: string
  requester_name: string | null
  requester_gender: Gender
  title: string
  content: string
  is_urgent: boolean
  status: PrayerStatus
  display_consent: Consent
  received_at: string
  created_at: string
  updated_at: string
  created_by: string | null
  follow_up_count: number
  last_follow_up_at: string | null
}

export interface PraiseReport {
  id: string
  service_id: string | null
  category_id: string
  category_name: string
  source_follow_up_id: string | null
  reporter_name: string | null
  reporter_gender: Gender
  title: string
  content: string
  is_featured: boolean
  display_consent: Consent
  reported_at: string
  created_at: string
  updated_at: string
  created_by: string | null
}

export interface FollowUp {
  id: string
  prayer_request_id: string
  contact_date: string
  channel: ContactChannel
  update_type: UpdateType
  condition_status: ConditionStatus
  notes: string
  next_action: string | null
  next_action_date: string | null
  created_at: string
  created_by: string | null
  created_by_name: string | null
  request_title?: string
  is_urgent?: boolean
}

export interface Summary {
  id: string
  service_id: string
  category_id: string
  category_name: string
  kind: Kind
  source_ids: string[]
  ai_text: string
  final_text: string | null
  display_text: string
  status: SummaryStatus
  model_name: string
  prompt_version: string
  approved_by: string | null
  approved_at: string | null
  created_at: string
  updated_at: string
}

export interface SlideItem {
  ref_id: string
  kind: Kind
  title: string
  body: string
  name?: string
  is_urgent?: boolean
  source_summary_id?: string
}

export interface Slide {
  id: string
  template: Kind
  title: string
  items: SlideItem[]
  note?: string
}

export interface Presentation {
  id: string
  service_id: string
  kind: Kind
  version: number
  status: 'draft' | 'ready' | 'exported'
  slides: Slide[]
  created_at: string
  updated_at: string
  created_by: string | null
}

export interface Export {
  id: string
  presentation_id: string
  format: ExportFormat
  status: ExportStatus
  storage_key: string | null
  download_url: string | null
  error_message: string | null
  created_at: string
  completed_at: string | null
}

export interface Assignee {
  assignment_id: string
  user_id: string
  full_name: string
  status: string
  swap_with_user_id: string | null
}

export interface Shift {
  id: string
  service_id: string
  label: string
  starts_at: string
  ends_at: string
  capacity: number
  is_published: boolean
  assignees: Assignee[]
  covered: boolean
  overbooked: boolean
}

export interface SwapRequest {
  id: string
  assignment_id: string
  requested_by: string
  requester_name: string
  target_user_id: string
  target_name: string
  status: 'pending' | 'approved' | 'declined' | 'cancelled'
  decided_by: string | null
  decided_at: string | null
  created_at: string
  shift_label: string
  shift_starts_at: string
}

export interface UrgentItem {
  id: string
  title: string
  category_name: string
  requester_name: string | null
  received_at: string
  last_follow_up_at: string | null
  follow_up_count: number
  days_since_follow_up: number
}

export interface DeadlineStatus {
  presentation_exists: boolean
  exported: boolean
  warning?: string
}

export interface NextService {
  id: string
  starts_at: string
  location: string
  slide_deadline: string | null
  deadline_hours_left: number
  deadline_passed: boolean
}

export interface DashboardSummary {
  next_service: NextService | null
  deadline_status: DeadlineStatus
  urgent_count: number
  urgent_requests: UrgentItem[]
  prayer_this_week: number
  praise_this_week: number
  my_next_shift: Shift | null
  draft_summary_count: number
  unread_notifications: number
  server_time: string
  timezone: string
}

export interface Notification {
  id: string
  title: string
  body: string
  link: string | null
  read_at: string | null
  created_at: string
}

export interface Invite {
  id: string
  email: string
  full_name: string
  role: Role
  expires_at: string
  accepted_at: string | null
  created_at: string
  invite_token?: string
}

export interface InvitePreview {
  email: string
  full_name: string
  role: Role
  expires_at: string
}

export interface AiStatus {
  provider: string
  model: string
  prompt_version: string
  offline_mode: boolean
}

export interface MetaVersion {
  api_version: string
  min_app_version: string
  ai_provider: string
  offline_mode: boolean
  environment: string
  export_driver: string
  timezone: string
  rate_limit_per_minute: number
}
