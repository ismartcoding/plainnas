import { format } from './timeago'

const FILE_SIZE_UNITS_SI = ['kB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'] as const
const FILE_SIZE_UNITS_IEC = ['KiB', 'MiB', 'GiB', 'TiB', 'PiB', 'EiB', 'ZiB', 'YiB'] as const

function getFileSizeUnits(si: boolean) {
  return si ? FILE_SIZE_UNITS_SI : FILE_SIZE_UNITS_IEC
}

function getLocale() {
  return localStorage.getItem('locale') ?? navigator.language ?? 'en-US'
}

export function formatDateTime(str: string, options?: Intl.DateTimeFormatOptions | undefined) {
  if (str === '1970-01-01T00:00:00Z') {
    return ''
  }
  return new Intl.DateTimeFormat(getLocale(), {
    hour12: false,
    dateStyle: 'medium',
    timeStyle: 'short',
    ...options,
  }).format(new Date(str))
}

export function formatDateTimeFull(str: string) {
  if (str === '1970-01-01T00:00:00Z') {
    return ''
  }
  return formatDateTime(str, { dateStyle: 'long', timeStyle: 'long' })
}

export function formatTimeAgo(str: string) {
  return format(new Date(str), getLocale().replace('-', '_'))
}

export function formatDate(str: string) {
  return new Intl.DateTimeFormat(getLocale()).format(new Date(str))
}

export function formatTime(str: string) {
  return new Intl.DateTimeFormat(getLocale(), { hour12: false, timeStyle: 'short' }).format(new Date(str))
}

export function formatSeconds(seconds: number) {
  const hours = Math.floor(seconds / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const remainingSeconds = Math.floor(seconds % 60)

  const formattedHours = hours < 10 ? `0${hours}` : hours
  const formattedMinutes = minutes < 10 ? `0${minutes}` : minutes
  const formattedSeconds = remainingSeconds < 10 ? `0${remainingSeconds}` : remainingSeconds

  if (hours > 0) {
    return `${formattedHours}:${formattedMinutes}:${formattedSeconds}`
  } else {
    return `${formattedMinutes}:${formattedSeconds}`
  }
}

export function formatUptime(seconds: number) {
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const secs = Math.floor(seconds % 60)
  const pad = (n: number) => (n < 10 ? `0${n}` : `${n}`)
  if (days > 0) {
    return `${days}d ${pad(hours)}:${pad(minutes)}:${pad(secs)}`
  }
  if (hours > 0) {
    return `${pad(hours)}:${pad(minutes)}:${pad(secs)}`
  }
  return `${pad(minutes)}:${pad(secs)}`
}

export function formatFileSize(bytes: number, si = true, dp = 1, forcedUnitIndex?: number) {
  const thresh = si ? 1000 : 1024
  const units = getFileSizeUnits(si)

  if (forcedUnitIndex !== undefined) {
    const u = Math.max(0, Math.min(forcedUnitIndex, units.length - 1))
    const factor = thresh ** (u + 1)
    return (bytes / factor).toFixed(dp) + ' ' + units[u]
  }

  if (Math.abs(bytes) < thresh) {
    return bytes + ' B'
  }

  let u = -1
  const r = 10 ** dp

  do {
    bytes /= thresh
    ++u
  } while (Math.round(Math.abs(bytes) * r) / r >= thresh && u < units.length - 1)

  return bytes.toFixed(dp) + ' ' + units[u]
}

// Formats used/total bytes using the same unit for clarity, e.g. "24.4 GiB / 62.2 GiB".
export function formatUsedTotalBytes(usedBytes: number, totalBytes: number, si = true) {
  if (totalBytes <= 0) return ''

  const thresh = si ? 1000 : 1024
  const maxUnitIndex = getFileSizeUnits(si).length - 1

  if (Math.abs(totalBytes) < thresh) {
    return `${usedBytes} B / ${totalBytes} B`
  }

  // Pick a unit based on total bytes, then force the same unit for used bytes.
  let u = -1
  let scaledTotal = totalBytes
  do {
    scaledTotal /= thresh
    ++u
  } while (Math.abs(scaledTotal) >= thresh && u < maxUnitIndex)

  // Short but readable in the sidebar.
  const dp = Math.abs(scaledTotal) >= 100 ? 0 : 1
  return `${formatFileSize(usedBytes, si, dp, u)} / ${formatFileSize(totalBytes, si, dp, u)}`
}

export function generateDownloadFileName(prefix: string) {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  const hours = String(now.getHours()).padStart(2, '0')
  const minutes = String(now.getMinutes()).padStart(2, '0')
  const seconds = String(now.getSeconds()).padStart(2, '0')

  return `${prefix}_${year}${month}${day}_${hours}${minutes}${seconds}.zip`
}
