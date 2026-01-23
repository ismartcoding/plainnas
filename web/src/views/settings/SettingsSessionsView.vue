<template>
  <div class="top-app-bar">
    <div class="title">{{ t('sessions_desc') }}</div>
    <div class="actions">
        <v-icon-button v-tooltip="$t('refresh')" class="sm" :disabled="loading" @click.stop="refetch">
            <i-material-symbols:refresh-rounded />
        </v-icon-button>
    </div>
  </div>

  <div class="scroll-content settings-page">
    <section v-for="(s, idx) in sessions" :key="s.clientId" class="card border-card">
      <div class="card-title"> 
        <div class="title">
            <field-id :id="idx + 1" :raw="s" />
            <span>{{ s.clientName || '-' }}</span>
            <span v-if="isCurrent(s.clientId)" class="status-badge on">{{ t('session_this_device') }}</span>
        </div>
          <v-outlined-button v-if="!isCurrent(s.clientId)" value="revoke" @click="confirmRevoke(s)">{{ t('revoke') }}</v-outlined-button>
      </div>
      <div class="card-body">
        <div class="hint">
          <span>{{ t('active_at') }}: </span>
          <span v-tooltip="formatDateTime(s.lastActive)">
            {{ formatTimeAgo(s.lastActive) }}
          </span>
        </div>
      </div>
      </section>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import toast from '@/components/toaster'
import { initQuery, sessionsGQL } from '@/lib/api/query'
import { openModal } from '@/components/modal'
import SessionRevokeConfirm from '@/components/SessionRevokeConfirm.vue'
import { formatDateTime, formatTimeAgo } from '@/lib/format'

type SessionRow = {
  clientId: string
  clientName: string
  lastActive: string
  createdAt: string
  updatedAt: string
}

const { t } = useI18n()

const sessions = ref<SessionRow[]>([])
const error = ref('')
const currentClientId = localStorage.getItem('client_id') ?? ''

const { refetch, loading } = initQuery<{ sessions: SessionRow[] }>({
  document: sessionsGQL,
  handle: (data, err) => {
    error.value = err || ''
    sessions.value = (data?.sessions ?? []).slice()
  },
})

function refresh() {
  refetch()
}

function isCurrent(clientId: string) {
  return clientId === currentClientId
}

function confirmRevoke(s: SessionRow) {
  openModal(SessionRevokeConfirm, {
    clientId: s.clientId,
    clientName: s.clientName,
    done: async () => {
      toast(t('revoked'))
      await refetch()
    },
  })
}
</script>

