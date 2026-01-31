<template>
  <v-modal @close="popModal">
    <template #headline>
      {{ $t('cast') }}
      <div class="actions">
        <v-icon-button v-tooltip="$t('refresh')" class="sm" :disabled="isLoading" @click.stop="refresh">
          <i-material-symbols:refresh-rounded />
        </v-icon-button>
      </div>
    </template>

    <template #content>
      <div class="content">
        <div class="meta">
          <div class="title" v-tooltip="title">{{ title }}</div>
        </div>

        <v-select v-model="selectedUdn" :label="$t('cast_device')"
          :placeholder="isLoading ? $t('loading') : $t('select')" :disabled="isLoading" :options="rendererOptions" />

        <div v-if="castHint" class="empty">{{ castHint }}</div>

        <div v-if="!isLoading && renderers.length === 0" class="empty">
          {{ $t('no_cast_devices') }}
        </div>
      </div>
    </template>

    <template #actions>
      <v-outlined-button @click="popModal">{{ $t('cancel') }}</v-outlined-button>
      <v-filled-button :disabled="!selectedUdn" :loading="castLoading" @click="cast">
        {{ $t('cast') }}
      </v-filled-button>
    </template>
  </v-modal>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { popModal } from '@/components/modal'
import toast from '@/components/toaster'
import { dlnaRenderersGQL, initLazyQuery } from '@/lib/api/query'
import { dlnaCastGQL, initMutation } from '@/lib/api/mutation'
import emitter from '@/plugins/eventbus'
import type { IDlnaRenderer } from '@/lib/interfaces'
import type { DataType } from '@/lib/data'

const { t } = useI18n()

const props = defineProps<{
  url: string
  title: string
  mime: string
  type: DataType
}>()

const renderers = ref<IDlnaRenderer[]>([])
const selectedUdn = ref<string>('')
const discovering = ref(false)
const submitAttempted = ref(false)

const rendererOptions = computed(() => {
  return renderers.value.map((r) => ({ value: r.udn, label: r.name }))
})

const selectedRenderer = computed(() => {
  return renderers.value.find((r) => r.udn === selectedUdn.value)
})

const castHint = computed(() => {
  if (!submitAttempted.value) return ''
  if (!castLoading.value) return ''
  const name = selectedRenderer.value?.name
  return name ? `${t('sending')} ${name}` : t('sending')
})

const { loading, fetch } = initLazyQuery<{ dlnaRenderers: IDlnaRenderer[] }>({
  document: dlnaRenderersGQL,
  variables: null,
  handle: (data, error) => {
    if (error) return
    if (!data?.dlnaRenderers) return
    mergeRenderers(data.dlnaRenderers)
  },
})

const isLoading = computed(() => loading.value || discovering.value)

function refresh() {
  startDiscovery()
}

function mergeRenderers(list: IDlnaRenderer[]) {
  if (!list || list.length === 0) return
  const byUdn = new Map(renderers.value.map((r) => [r.udn, r]))
  for (const r of list) {
    if (!r?.udn) continue
    byUdn.set(r.udn, { ...byUdn.get(r.udn), ...r })
  }
  renderers.value = [...byUdn.values()].sort((a, b) => a.name.localeCompare(b.name))
  if (!selectedUdn.value && renderers.value.length > 0) {
    selectedUdn.value = renderers.value[0].udn
  }
}

let discoveryStopTimer: ReturnType<typeof setTimeout> | 0 = 0
function startDiscovery() {
  discovering.value = true
  fetch()
  if (discoveryStopTimer) clearTimeout(discoveryStopTimer)
  discoveryStopTimer = setTimeout(() => {
    discovering.value = false
  }, 65_000)
}

function onRendererFound(r: IDlnaRenderer) {
  mergeRenderers([r])
}

function onDiscoveryDone() {
  discovering.value = false
  if (discoveryStopTimer) {
    clearTimeout(discoveryStopTimer)
    discoveryStopTimer = 0
  }
}

const { mutate: doCast, loading: castLoading, onDone } = initMutation({
  document: dlnaCastGQL,
})

onDone(() => {
  toast(t('cast_started'))
  popModal()
})

function cast() {
  if (!selectedUdn.value) return
  submitAttempted.value = true
  doCast({
    rendererUdn: selectedUdn.value,
    url: props.url,
    title: props.title,
    mime: props.mime,
    type: props.type,
  })
}

onMounted(() => {
  emitter.on('dlna_renderer_found', onRendererFound)
  emitter.on('dlna_discovery_done', onDiscoveryDone)
  startDiscovery()
})

onBeforeUnmount(() => {
  emitter.off('dlna_renderer_found', onRendererFound)
  emitter.off('dlna_discovery_done', onDiscoveryDone)
  if (discoveryStopTimer) {
    clearTimeout(discoveryStopTimer)
    discoveryStopTimer = 0
  }
})
</script>

<style scoped lang="scss">
.content {
  display: grid;
  gap: 12px;
}

.meta {
  .title {
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}

.empty {
  opacity: 0.75;
  font-size: 13px;
}
</style>
