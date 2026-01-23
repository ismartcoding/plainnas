<template>
  <div class="top-app-bar">
    <div class="title">{{ t('events_desc') }}</div>
    <div class="actions">
      <v-chip-set>
        <v-filter-chip label="50" :selected="limit === 50" @click="setLimit(50)" />
        <v-filter-chip label="100" :selected="limit === 100" @click="setLimit(100)" />
        <v-filter-chip label="200" :selected="limit === 200" @click="setLimit(200)" />
        <v-filter-chip label="1000" :selected="limit === 1000" @click="setLimit(1000)" />
      </v-chip-set>

      <v-icon-button v-tooltip="t('refresh')" class="sm" :disabled="loading" @click.stop="refresh">
        <i-material-symbols:refresh-rounded />
      </v-icon-button>
    </div>
  </div>

  <div class="scroll-content settings-page">
    <section v-if="error" class="card border-card">
      <div class="card-body">
        <div class="error">{{ t(error) }}</div>
      </div>
    </section>

    <section v-else-if="events.length === 0" class="card border-card">
      <div class="card-body">
        <div class="empty">{{ t('no_data') }}</div>
      </div>
    </section>

    <section v-for="(e, idx) in events" :key="e.id" class="card border-card">
      <div class="card-title">
        <div class="title">
            <field-id :id="idx + 1" :raw="e" />
            <span class="status-badge" :class="badgeClass(e.type)" :title="e.type">{{ typeLabel(e.type) }}</span>
            <span v-tooltip="formatDateTime(e.createdAt)">
            {{ formatTimeAgo(e.createdAt) }}
            </span>
        </div>
      </div>
      <div class="card-body">
        <div v-if="eventParts(e).length" class="hint event-hint">
          <template v-for="(p, idx) in eventParts(e)" :key="idx">
            <span>{{ p }}</span>
            <span v-if="idx < eventParts(e).length - 1">·</span>
          </template>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { initQuery, eventsGQL } from '@/lib/api/query'
import { formatDateTime, formatTimeAgo } from '@/lib/format'

type EventRow = {
  id: string
  type: string
  message: string
  clientId: string
  createdAt: string
}

const { t } = useI18n()

const limit = ref(200)
const events = ref<EventRow[]>([])
const error = ref('')

const { refetch, loading } = initQuery<{ events: EventRow[] }>({
  document: eventsGQL,
  variables: { limit: limit.value },
  handle: (data, err) => {
    error.value = err || ''
    events.value = (data?.events ?? []).slice()
  },
})

function typeLabel(type: string) {
  const key = `event_type.${type}`
  const v = t(key)
  return v === key ? type : v
}

function badgeClass(type: string) {
  if (type === 'login_failed' || type.endsWith('_failed')) return 'bad'
  return 'on'
}

function shortClientID(clientId: string) {
  const s = (clientId || '').trim()
  if (!s) return ''
  if (s.length <= 14) return s
  return `${s.slice(0, 6)}…${s.slice(-4)}`
}

function simplifyReasonText(msg: string) {
  const s = (msg || '').trim()
  const lower = s.toLowerCase()
  if (!s) return ''
  if (lower.includes('device or resource busy')) return t('event_reason_device_busy')
  if (lower.includes('permission denied')) return t('event_reason_permission_denied')
  if (lower.includes('no such file') || lower.includes('not found')) return t('event_reason_not_found')
  if (lower.includes('timeout')) return t('event_reason_timeout')
  return t('event_reason_failed')
}

function parseFormatMessage(message: string) {
  const s = (message || '').trim()
  if (!s.startsWith('/dev/')) return { disk: '', reason: '' }
  const idx = s.indexOf(':')
  if (idx === -1) return { disk: s, reason: '' }
  const disk = s.slice(0, idx).trim()
  const reason = s.slice(idx + 1).trim()
  return { disk, reason }
}

function parseMountMessage(message: string) {
  const s = (message || '').trim()
  const m = /^mounted\s+UUID\s+(\S+)\s+at\s+(\S+)$/i.exec(s)
  if (!m) return { mountpoint: '' }
  return { mountpoint: m[2] }
}

function parseMountFailedMessage(message: string) {
  const s = (message || '').trim()
  const m1 = /^UUID\s+(\S+)\s+->\s+(\S+):\s*(.+)$/i.exec(s)
  if (m1) return { target: m1[2], reason: m1[3] }
  const m2 = /^(mkdir|cleanup)\s+(\S+):\s*(.+)$/i.exec(s)
  if (m2) return { target: m2[2], reason: m2[3] }
  return { target: '', reason: s }
}

function eventParts(e: EventRow) {
  const parts: string[] = []
  const type = (e.type || '').trim()
  const message = (e.message || '').trim()

  if (type === 'login' || type === 'logout') {
    if (message) parts.push(`${t('event_field_client')}: ${message}`)
    return parts
  }

  if (type === 'revoke') {
    if (message) parts.push(`${t('event_field_client')}: ${message}`)
    if (!message && e.clientId) parts.push(`${t('event_field_session')}: ${shortClientID(e.clientId)}`)
    return parts
  }

  if (type === 'login_failed') {
    const reasonKey = `event_login_failed_reason.${message}`
    const reason = message ? t(reasonKey) : ''
    parts.push(`${t('event_field_reason')}: ${reason === reasonKey ? t('event_reason_failed') : reason}`)
    return parts
  }

  if (type === 'mount') {
    const { mountpoint } = parseMountMessage(message)
    if (mountpoint) parts.push(`${t('event_field_mountpoint')}: ${mountpoint}`)
    return parts
  }

  if (type === 'unmount') {
    if (message) parts.push(`${t('event_field_mountpoint')}: ${message}`)
    return parts
  }

  if (type === 'format_disk') {
    if (message) parts.push(`${t('event_field_disk')}: ${message}`)
    return parts
  }

  if (type === 'format_disk_failed') {
    const { disk, reason } = parseFormatMessage(message)
    if (disk) parts.push(`${t('event_field_disk')}: ${disk}`)
    const simple = simplifyReasonText(reason)
    if (simple) parts.push(`${t('event_field_reason')}: ${simple}`)
    return parts
  }

  if (type === 'mount_failed') {
    const { target, reason } = parseMountFailedMessage(message)
    if (target) parts.push(`${t('event_field_target')}: ${target}`)
    const simple = simplifyReasonText(reason)
    if (simple) parts.push(`${t('event_field_reason')}: ${simple}`)
    return parts
  }

  // Fallback: show message, but keep it compact.
  if (message) parts.push(message)
  return parts
}

function refresh() {
  ;(refetch as any)({ limit: limit.value })
}

function setLimit(v: number) {
  if (limit.value === v) return
  limit.value = v
  refresh()
}
</script>

<style scoped lang="scss">
.empty,
.error {
  color: var(--md-sys-color-on-surface-variant);
}
</style>
