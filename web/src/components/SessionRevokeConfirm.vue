<template>
  <v-modal @close="popModal">
    <template #content>
      <div class="title">{{ $t('revoke_session_confirm') }}</div>
      <div class="meta mono">{{ clientId }}</div>
      <div v-if="clientName" class="meta">{{ clientName }}</div>
      <p v-if="isCurrent" class="hint">{{ $t('sessions_revoke_current_hint') }}</p>
    </template>
    <template #actions>
      <v-outlined-button @click="popModal">{{ $t('cancel') }}</v-outlined-button>
      <v-filled-button :loading="loading" @click="doRevoke">{{ $t('revoke') }}</v-filled-button>
    </template>
  </v-modal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { PropType } from 'vue'
import { initMutation, revokeSessionGQL } from '@/lib/api/mutation'
import { popModal } from './modal'

const props = defineProps({
  clientId: { type: String, required: true },
  clientName: { type: String, default: '' },
  done: {
    type: Function as PropType<() => void>,
    default: () => {},
  },
})

const { mutate, loading, onDone } = initMutation({
  document: revokeSessionGQL,
})

const isCurrent = computed(() => {
  return props.clientId === (localStorage.getItem('client_id') ?? '')
})

function doRevoke() {
  mutate({ clientId: props.clientId })
}

onDone(() => {
  props.done?.()
  popModal()
})
</script>

<style lang="scss" scoped>
.title {
  font-size: 1rem;
  font-weight: 700;
  margin-bottom: 0.5rem;
}

.meta {
  color: var(--md-sys-color-on-surface-variant);
  font-size: 0.9rem;
}

.hint {
  margin-top: 10px;
}
</style>
