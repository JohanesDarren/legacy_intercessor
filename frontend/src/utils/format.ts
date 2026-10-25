/**
 * Date and formatting helpers. All dates display in WIB (Asia/Jakarta) through
 * these functions only (PRD 7.3: a single formatting utility).
 */

const TZ = 'Asia/Jakarta'

const dateFmt = new Intl.DateTimeFormat('id-ID', {
  timeZone: TZ,
  day: '2-digit',
  month: 'short',
  year: 'numeric',
})

const dateLongFmt = new Intl.DateTimeFormat('id-ID', {
  timeZone: TZ,
  weekday: 'long',
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

const timeFmt = new Intl.DateTimeFormat('id-ID', {
  timeZone: TZ,
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
})

const dateTimeFmt = new Intl.DateTimeFormat('id-ID', {
  timeZone: TZ,
  day: '2-digit',
  month: 'short',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
})

/** "12 Des 2026" */
export function formatDate(iso?: string | null): string {
  if (!iso) return '—'
  return dateFmt.format(new Date(iso))
}

/** "Sabtu, 12 Desember 2026" */
export function formatDateLong(iso?: string | null): string {
  if (!iso) return '—'
  return dateLongFmt.format(new Date(iso))
}

/** "17:00" */
export function formatTime(iso?: string | null): string {
  if (!iso) return '—'
  return timeFmt.format(new Date(iso))
}

/** "12 Des 2026, 17:00" */
export function formatDateTime(iso?: string | null): string {
  if (!iso) return '—'
  return dateTimeFmt.format(new Date(iso))
}

/** "17:00 WIB" */
export function formatTimeWib(iso?: string | null): string {
  if (!iso) return '—'
  return `${timeFmt.format(new Date(iso))} WIB`
}

/** Relative time such as "3 hari lalu" or "in 2 hours". */
export function relativeTime(iso?: string | null): string {
  if (!iso) return '—'
  const then = new Date(iso).getTime()
  const diffMs = then - Date.now()
  const abs = Math.abs(diffMs)
  const mins = Math.round(abs / 60000)
  const hours = Math.round(abs / 3600000)
  const days = Math.round(abs / 86400000)

  const suffix = diffMs < 0 ? ' lalu' : ' lagi'
  if (mins < 1) return 'baru saja'
  if (mins < 60) return `${mins} menit${suffix}`
  if (hours < 24) return `${hours} jam${suffix}`
  if (days < 30) return `${days} hari${suffix}`
  return formatDate(iso)
}

/**
 * Countdown text for the slide deadline. Negative values mean the deadline
 * has passed.
 */
export function deadlineCountdown(iso?: string | null): {
  text: string
  passed: boolean
  urgent: boolean
} {
  if (!iso) return { text: 'Belum diatur', passed: false, urgent: false }
  const ms = new Date(iso).getTime() - Date.now()
  const passed = ms < 0
  const abs = Math.abs(ms)
  const hours = Math.floor(abs / 3600000)
  const mins = Math.floor((abs % 3600000) / 60000)

  let text: string
  if (hours >= 48) {
    text = `${Math.floor(hours / 24)} hari`
  } else if (hours >= 1) {
    text = `${hours} jam ${mins} menit`
  } else {
    text = `${mins} menit`
  }
  if (passed) text = `Lewat ${text}`

  return { text, passed, urgent: !passed && hours < 24 }
}

/** Local date as YYYY-MM-DD in WIB, for date inputs. */
export function toDateInput(iso?: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: TZ,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(d)
  return parts
}

/** Builds an ISO string at a given WIB date and time. */
export function toIso(dateStr: string, timeStr: string): string {
  return new Date(`${dateStr}T${timeStr}:00+07:00`).toISOString()
}

/** Initials for an avatar, max two letters. */
export function initials(name?: string | null): string {
  if (!name) return '?'
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((p) => p[0]?.toUpperCase() ?? '')
    .join('')
}

/** Indonesian label for a consent value. */
export function consentLabel(consent: string): string {
  switch (consent) {
    case 'yes':
      return 'Boleh tampil'
    case 'anonymous':
      return 'Anonim'
    default:
      return 'Tidak tampil'
  }
}

/** Indonesian label for a prayer status. */
export function statusLabel(status: string): string {
  switch (status) {
    case 'open':
      return 'Baru'
    case 'in_prayer':
      return 'Didalam doa'
    case 'answered':
      return 'Dijawab'
    case 'closed':
      return 'Selesai'
    default:
      return status
  }
}

/** Indonesian label for a condition. */
export function conditionLabel(condition: string): string {
  switch (condition) {
    case 'improving':
      return 'Membaik'
    case 'stable':
      return 'Stabil'
    case 'worsening':
      return 'Memburuk'
    case 'answered':
      return 'Dijawab'
    case 'unreachable':
      return 'Tidak terjangkau'
    default:
      return condition
  }
}

/** Indonesian label for a contact channel. */
export function channelLabel(channel: string): string {
  switch (channel) {
    case 'visit':
      return 'Kunjungan'
    case 'phone':
      return 'Telepon'
    case 'whatsapp':
      return 'WhatsApp'
    default:
      return 'Lainnya'
  }
}

/** Indonesian label for an update type. */
export function updateTypeLabel(type: string): string {
  switch (type) {
    case 'documentation':
      return 'Dokumentasi'
    case 'life_update':
      return 'Kabar hidup'
    case 'current_condition':
      return 'Kondisi saat ini'
    default:
      return 'Lainnya'
  }
}

/** Indonesian label for a role. */
export function roleLabel(role: string): string {
  switch (role) {
    case 'admin':
      return 'Admin'
    case 'intercessor':
      return 'Intercessor'
    case 'viewer':
      return 'Viewer'
    default:
      return role
  }
}
