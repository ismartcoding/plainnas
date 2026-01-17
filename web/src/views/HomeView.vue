<template>
  <div class="grids">
    <div class="card feature-card" @click="openTab('/audios')">
      <div class="card-icon">
        <i-lucide:music />
      </div>
      <div class="card-content">
        <div class="card-title-row">
          <span v-if="counter.audios !== undefined && counter.audios >= 0" class="count">{{
            counter.audios.toLocaleString() }}</span>
          <span class="title">{{ $t('audios') }}</span>
        </div>
      </div>
    </div>
    <div class="card feature-card" @click="openTab('/images')">
      <div class="card-icon">
        <i-lucide:image />
      </div>
      <div class="card-content">
        <div class="card-title-row">
          <span v-if="counter.images !== undefined && counter.images >= 0" class="count">{{
            counter.images.toLocaleString() }}</span>
          <span class="title">{{ $t('images') }}</span>
        </div>
      </div>
    </div>
    <div class="card feature-card" @click="openTab('/videos')">
      <div class="card-icon">
        <i-lucide:video />
      </div>
      <div class="card-content">
        <div class="card-title-row">
          <span v-if="counter.videos !== undefined && counter.videos >= 0" class="count">{{
            counter.videos.toLocaleString() }}</span>
          <span class="title">{{ $t('videos') }}</span>
        </div>
      </div>
    </div>

    <div class="card feature-card" @click="openFilesInternalStorage">
      <div class="card-icon">
        <i-lucide:folder />
      </div>
      <div class="card-content">
        <div class="card-title-row">
          <span class="title">{{ $t('files') }}</span>
        </div>
        <div v-if="counter.total >= 0" class="storage-info">
          {{ $t('storage_free_total', { free: formatFileSize(counter.free), total: formatFileSize(counter.total) }) }}
        </div>

        <div class="scan-panel">
          <div v-if="['running', 'paused'].includes(app.scanProgress.state || '')" class="scan-row">
            <span v-if="app.scanProgress.state">{{ stateLabel }} </span>
            <span v-if="app.scanProgress.root" class="muted">{{ app.scanProgress.root }}</span>
            <span v-if="app.scanProgress.total > 0" class="muted">{{ percent }}%</span>
          </div>
          <div
v-if="['running', 'paused'].includes(app.scanProgress.state || '') && app.scanProgress.total > 0"
            class="progress">
            <div class="bar" :style="{ width: percent + '%' }"></div>
          </div>
          <div
v-if="['running', 'paused'].includes(app.scanProgress.state || '') && app.scanProgress.total > 0"
            class="muted">
            {{ app.scanProgress.indexed.toLocaleString() }} / {{ app.scanProgress.total.toLocaleString() }}
            <span v-if="app.scanProgress.pending > 0"> · {{ $t('pending') }} {{
              app.scanProgress.pending.toLocaleString() }}</span>
          </div>

          <div class="action-row">
            <v-filled-button v-if="showPause" @click.stop="pauseScan">{{ $t('pause') }}</v-filled-button>
            <v-filled-button v-if="showResume" @click.stop="resumeScan">{{ $t('resume') }}</v-filled-button>
            <v-outlined-button v-if="showStop" @click.stop="stopScan">{{ $t('stop') }}</v-outlined-button>
            <v-outlined-button
v-if="showRebuild || rebuildIndexLoading" :loading="rebuildIndexLoading"
              @click.stop="rebuildIndex">{{
                $t('rebuild_index')
              }}</v-outlined-button>
          </div>
        </div>
      </div>
    </div>

  </div>
</template>

<script setup lang="ts">
import toast from '@/components/toaster'
import { homeStatsGQL, initQuery } from '@/lib/api/query'
import { formatFileSize } from '@/lib/format'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useTempStore } from '@/stores/temp'
import { storeToRefs } from 'pinia'
import { sumBy } from 'lodash-es'
import { useMainStore } from '@/stores/main'
import { initMutation, pauseMediaScanGQL, resumeMediaScanGQL, stopMediaScanGQL, rebuildMediaIndexGQL } from '@/lib/api/mutation'
import { replacePath } from '@/plugins/router'
import type { IHomeStats } from '@/lib/interfaces'
import { buildQuery } from '@/lib/search'
import { encodeBase64 } from '@/lib/strutil'
import VFilledButton from '@/components/base/VFilledButton.vue'
import VOutlinedButton from '@/components/base/VOutlinedButton.vue'

const { t } = useI18n()

const mainStore = useMainStore()

const { app, counter } = storeToRefs(useTempStore())

const percent = computed(() => {
  const { indexed, total } = app.value.scanProgress || { indexed: 0, total: 0 }
  if (!total) return 0
  return Math.min(100, Math.max(0, Math.round((indexed / total) * 100)))
})

const stateLabel = computed(() => {
  if (app.value.scanProgress?.state === 'running') return 'Building file index…'
  if (app.value.scanProgress?.state === 'paused') return t('paused')
  if (app.value.scanProgress?.state === 'stopped') return t('stopped')
  return ''
})

const showPause = computed(() => app.value.scanProgress?.state === 'running')
const showResume = computed(() => app.value.scanProgress?.state === 'paused')
const showStop = computed(() => ['running', 'paused'].includes(app.value.scanProgress?.state || ''))
const showRebuild = computed(() => ['idle', 'stopped'].includes(app.value.scanProgress?.state || ''))

function openTab(fullPath: string) { replacePath(mainStore, fullPath) }
function openFilesInternalStorage() {
  const q = buildQuery([{ name: 'root_path', op: '', value: '/' }])
  replacePath(mainStore, `/files?q=${encodeBase64(q)}`)
}

const { mutate: pauseScanMutation } = initMutation({ document: pauseMediaScanGQL })
const { mutate: resumeScanMutation } = initMutation({ document: resumeMediaScanGQL })
const { mutate: stopScanMutation } = initMutation({ document: stopMediaScanGQL })
const { mutate: rebuildIndexMutation, loading: rebuildIndexLoading } = initMutation({ document: rebuildMediaIndexGQL })

async function pauseScan() { await pauseScanMutation(); }
async function resumeScan() { await resumeScanMutation(); }
async function stopScan() { await stopScanMutation(); }
async function rebuildIndex() {
  await rebuildIndexMutation({ root: '/' })
}

initQuery({
  handle: (data: IHomeStats, error: string) => {
    if (error) { toast(t(error), 'error') }
    else if (data) {
      counter.value.videos = data.videoCount
      counter.value.images = data.imageCount
      counter.value.audios = data.audioCount
      const vols = data.storageVolumes
      const totalBytes = sumBy(vols, (it: any) => it.totalBytes)
      const freeBytes = sumBy(vols, (it: any) => it.freeBytes)
      counter.value.total = totalBytes
      counter.value.free = freeBytes
    }
  },
  document: homeStatsGQL,
  variables: null,
})
</script>

<style lang="scss" scoped>
.grids {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
  overflow-y: auto;
  padding: 16px;
}

.feature-card {
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
  min-height: 120px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
}

.feature-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.card-icon {
  display: flex;
  justify-content: center;
  align-items: center;
  margin-bottom: 8px;
}

.card-icon svg {
  width: 32px;
  height: 32px;
  color: var(--md-sys-color-primary);
}

.card-content {
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
}

.card-title-row {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 6px;
  margin: 0;
}

.card-title-row .count {
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--md-sys-color-primary);
}

.card-title-row .title {
  font-size: 0.875rem;
  text-transform: capitalize;
  color: var(--md-sys-color-on-surface);
}

.storage-info {
  font-size: 0.75rem;
  color: var(--md-sys-color-on-surface-variant);
  margin-top: 4px;
}

.muted {
  margin-top: 8px;
  color: var(--md-sys-color-on-surface-variant);
  font-size: 0.875rem;
}

.scan-panel {
  width: 100%;
  max-width: 320px;
  margin-top: 10px;
}

.scan-row {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.progress {
  position: relative;
  height: 6px;
  background: rgba(0, 0, 0, 0.08);
  border-radius: 999px;
  overflow: hidden;
}

.progress .bar {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 0;
  background: var(--md-sys-color-primary);
  border-radius: 999px;
  transition: width 0.25s ease;
}

.action-row {
  display: flex;
  justify-content: center;
  gap: 8px;
  margin-top: 10px;
  flex-wrap: wrap;
}
</style>
