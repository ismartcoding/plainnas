<template>
  <v-modal @close="popModal">
    <template #headline>
       {{ $t('disk_manager') }}
        <v-icon-button v-tooltip="$t('refresh')" class="sm" :disabled="loading" @click.stop="refetch">
          <i-material-symbols:refresh-rounded />
        </v-icon-button>
    </template>

    <template #content>
      <div class="dm">
        <div v-if="errorText" class="dm-error">{{ errorText }}</div>
        <div v-else-if="loading" class="dm-loading">{{ $t('loading') }}</div>

        <template v-else>
          <div v-if="!sortedDisks.length" class="empty">{{ $t('no_disks') }}</div>

          <section v-for="(disk, idx) in sortedDisks" :key="disk.path" class="card border-card">
            <h5 class="card-title dm-card-title">
              <span class="dm-card-title-main">
                <span class="number"><field-id :id="idx + 1" :raw="disk" />.</span>
                <span class="dm-title">{{ diskTitle(disk, idx) }}</span>
                <span class="dm-size">({{ formatFileSize(Number(disk.sizeBytes || 0)) }})</span>
                <popper>
                  <v-icon-button class="btn-help sm">
                    <i-material-symbols:help-outline-rounded />
                  </v-icon-button>
                  <template #content>
                    <div class="dm-help-pop">
                      <div class="title">{{ diskSummaryText(disk) }}</div>
                      <dl class="meta">
                        <dt>{{ $t('unavailable_space') }}</dt>
                        <dd>
                          {{ formatFileSize(unavailableBytes(disk)) }}
                        </dd>
                        <template v-if="disk.model">
                          <dt>{{ $t('device_model') }}</dt>
                          <dd>{{ disk.model }}</dd>
                        </template>
                      </dl>
                      <div class="hint">{{ unavailableReasonText(disk) }}</div>
                    </div>
                  </template>
                </popper>

                <span v-if="disk.removable" class="dm-chip">{{ $t('removable') }}</span>
              </span>
              <v-outlined-button v-if="canFormatDisk(disk)" class="sm dm-format" :disabled="loading" @click.stop="formatDisk(disk)">
                {{ $t('format_disk') }}
              </v-outlined-button>
            </h5>

            <div class="card-body">
              <template v-if="diskVolumes(disk).length || diskPartitions(disk).length > 0">
                <div v-if="!diskVolumes(disk).length && diskPartitions(disk).length > 0" class="hint dm-hint">
                  {{ $t('no_mounted_volumes') }}
                </div>

                <div class="browser-volumes dm-volumes">
                  <div class="volumes">
                    <VolumeCard
                      v-for="v in diskVolumes(disk)"
                      :key="v.id"
                      :data="v"
                      :title="volumeTitleForDisk(v, disk)"
                      :drive-type="v.driveType"
                      :used-percent="volumeUsedPercent(v)"
                      :count="volumeCount(v)"
                      :show-progress="true"
                    />

                    <VolumeCard
                      v-for="p in diskUnmountedPartitions(disk)"
                      :key="p.id"
                      class="dm-disabled"
                      :data="p"
                      :title="partitionCardTitle(p)"
                      :drive-type="(p.fsType || '').toUpperCase()"
                      :show-progress="false"
                    />
                  </div>
                </div>
              </template>

              <div v-else class="hint dm-hint">
                {{ $t('disk_ready_hint') }}
              </div>
            </div>
          </section>
        </template>
      </div>
    </template>

    <template #actions>
      <v-outlined-button value="cancel" @click="popModal">{{ $t('close') }}</v-outlined-button>
    </template>
  </v-modal>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMainStore } from '@/stores/main'
import { initQuery, disksGQL, mountsGQL } from '@/lib/api/query'
import type { IStorageDisk, IStorageMount } from '@/lib/interfaces'
import { formatFileSize, formatUsedTotalBytes } from '@/lib/format'
import VolumeCard from '@/components/storage/VolumeCard.vue'
import { getStorageVolumeTitle } from '@/lib/volumes'
import { popModal, pushModal } from '@/components/modal'
import FormatDiskConfirm from '@/components/storage/FormatDiskConfirm.vue'

const { t, locale } = useI18n()
const mainStore = useMainStore()

const disks = ref<IStorageDisk[]>([])
const mounts = ref<IStorageMount[]>([])
const errorText = ref('')

const disksQuery = initQuery<{ disks: IStorageDisk[] }>({
  document: disksGQL,
  handle: (data, error) => {
    if (error) {
      errorText.value = t(error)
      disks.value = []
      return
    }
    errorText.value = ''
    disks.value = (data?.disks ?? []) as IStorageDisk[]
  },
})

const mountsQuery = initQuery<{ mounts: IStorageMount[] }>({
  document: mountsGQL,
  handle: (data, error) => {
    if (error) {
      // Keep disks visible even if volumes fail.
      return
    }
    mounts.value = (data?.mounts ?? []) as IStorageMount[]
  },
})

const loading = computed(() => !!disksQuery.loading.value || !!mountsQuery.loading.value)

const diskTitleCollator = computed(() => new Intl.Collator(locale.value, { numeric: true, sensitivity: 'base' }))

const sortedDisks = computed(() => {
  const collator = diskTitleCollator.value
  return [...(disks.value || [])].sort((a, b) => {
    const aTitle = diskTitle(a).trim()
    const bTitle = diskTitle(b).trim()
    const cmp = collator.compare(aTitle, bTitle)
    if (cmp) return cmp
    return String(a.path || a.name || '').localeCompare(String(b.path || b.name || ''))
  })
})

function refetch() {
  disksQuery.refetch()
  mountsQuery.refetch()
}

function volumeUsedPercent(v: IStorageMount) {
  const total = Number(v.totalBytes || 0)
  const used = Number(v.usedBytes || 0)
  if (!total) return 0
  const pct = (used / total) * 100
  if (!Number.isFinite(pct)) return 0
  return Math.max(0, Math.min(100, pct))
}

function volumeCount(v: IStorageMount) {
  const total = Number(v.totalBytes || 0)
  const used = Number(v.usedBytes || 0)
  if (!total) return ''
  return formatUsedTotalBytes(used, total)
}

function diskVolumes(d: IStorageDisk): IStorageMount[] {
  const name = String(d?.name || '').trim()
  if (!name) return []
  return (mounts.value || []).filter((v) => !String(v?.path ?? '').trim() && !v.remote && (v.parentDevice || '').trim() === name)
}

function diskPartitions(d: IStorageDisk): IStorageMount[] {
  const name = String(d?.name || '').trim()
  if (!name) return []
  return (mounts.value || []).filter((m) => !!String(m?.path ?? '').trim() && !m.remote && (m.parentDevice || '').trim() === name)
}

function diskUnmountedPartitions(d: IStorageDisk): IStorageMount[] {
  const mounted = new Set(
    diskVolumes(d)
      .map((v) => String(v.mountPoint || '').trim())
      .filter(Boolean)
  )
  return diskPartitions(d).filter((p) => {
    const mp = String(p.mountPoint || '').trim()
    if (!mp) return true
    return !mounted.has(mp)
  })
}

function diskSummaryKind(d: IStorageDisk): 'partitioned' | 'whole-disk' | 'unpartitioned' {
  const partitions = Number(diskPartitions(d).length || 0)
  if (partitions > 0) return 'partitioned'
  if (diskVolumes(d).length > 0) return 'whole-disk'
  return 'unpartitioned'
}

function diskSummaryText(d: IStorageDisk) {
  const kind = diskSummaryKind(d)
  if (kind === 'partitioned') return t('partition_count_x', { n: diskPartitions(d).length })
  if (kind === 'whole-disk') return t('whole_disk_volume')
  return t('unpartitioned_disk')
}

function unavailableBytes(d: IStorageDisk) {
  const diskSize = Number(d?.sizeBytes || 0)
  if (!diskSize) return 0

  const vols = diskVolumes(d)
  const parts = diskPartitions(d)

  // Estimate how much space the user can "use":
  // - for mounted volumes, use filesystem total bytes (closer to user-visible capacity)
  // - for unmounted partitions, use raw partition size (best available)
  // Remaining is typically formatting metadata, reserved space, or unallocated gaps.
  let usable = 0
  const volByMount = new Map<string, IStorageMount>()
  for (const v of vols) {
    const mp = String(v.mountPoint || '').trim()
    if (mp) volByMount.set(mp, v)
  }

  if (parts.length) {
    for (const p of parts) {
      const mp = String(p.mountPoint || '').trim()
      const v = mp ? volByMount.get(mp) : undefined
      if (v) {
        usable += Number(v.totalBytes || 0)
        continue
      }
      usable += Number(p.totalBytes || 0)
    }
  } else {
    for (const v of vols) {
      usable += Number(v.totalBytes || 0)
    }
  }

  const gap = diskSize - usable
  if (!Number.isFinite(gap) || gap <= 0) return 0
  return gap
}

function unavailableReasonText(d: IStorageDisk) {
  if (diskPartitions(d).length) return t('unavailable_reason_partitioned')
  if (diskVolumes(d).length) return t('unavailable_reason_whole_disk')
  return t('unavailable_reason_unset')
}

function diskTitle(d: IStorageDisk, idx = 0) {
  const n = (d.name || '').trim()
  // Prefer model when present; fallback to a friendlier label than raw device name.
  const model = (d.model || '').trim()
  if (model) return model
  const num = Number.isFinite(idx) ? idx + 1 : 1
  if (n.startsWith('nvme')) return `${t('disk')} (NVMe)`
  if (n.startsWith('mmcblk0')) return t('internal_storage')
  if (n.startsWith('mmcblk')) return t('sdcard')
  if (d.removable) return t('usb_storage')
  return t('disk')
}

function partitionCardTitle(p: IStorageMount) {
  const title = getStorageVolumeTitle(p, t)
  const mp = String(p.mountPoint || '').trim()
  if (!mp) return title
  return `${title} - ${mp}`
}

function volumeTitleForDisk(v: IStorageMount, d: IStorageDisk) {
  const mp = String(v.mountPoint || '').trim()
  const part = diskPartitions(d).find((p) => String(p.mountPoint || '').trim() === mp)
  if (part) return getStorageVolumeTitle(part, t)
  return getStorageVolumeTitle(v, t)
}

function isSystemDisk(d: IStorageDisk) {
  const parts = diskPartitions(d)
  if (parts.some((p) => String(p.mountPoint || '').trim() === '/')) return true
  if (diskVolumes(d).some((v) => String(v.mountPoint || '').trim() === '/')) return true

  const rootVol = (mounts.value || []).find((v) => !String(v?.path ?? '').trim() && String(v.mountPoint || '').trim() === '/')
  if (!rootVol) return false
  const parent = String(rootVol.parentDevice || '').trim()
  if (!parent) return false
  if (parent === String(d.name || '').trim()) return true
  if (parts.some((p) => String(p.name || '').trim() === parent)) return true
  return false
}

function canFormatDisk(d: IStorageDisk) {
  return !isSystemDisk(d)
}

function formatDisk(d: IStorageDisk) {
  pushModal(FormatDiskConfirm, {
    path: d.path,
    label: diskTitle(d),
    onDone: () => refetch(),
  })
}
</script>

<style scoped>
.dm {
  min-width: min(960px, 92vw);
}

.dm-error {
  color: var(--md-sys-color-error);
}

.dm-loading {
  color: var(--md-sys-color-on-surface-variant);
}

.dm-card-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dm-card-title-main {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex-wrap: wrap;
}

.dm-title {
  font-weight: 650;
  overflow: hidden;
}

.dm-size {
  color: var(--md-sys-color-on-surface-variant);
}

.dm-chip {
  font-size: 12px;
  padding: 2px 8px;
  border-radius: 999px;
  border: 1px solid var(--md-sys-color-outline-variant);
  color: var(--md-sys-color-on-surface-variant);
  white-space: nowrap;
}

.dm-volumes.browser-volumes {
  margin-top: 8px;
  padding: 0;
}

.dm-disabled {
  opacity: 0.55;
  filter: grayscale(0.35);
  cursor: default;
}

.dm-hint {
  margin-top: 6px;
}

.dm-format {
  white-space: nowrap;
}

.dm-help-pop {
  max-width: min(520px, 80vw);
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-weight: normal;

  .title {
    font-weight: 650;
    color: var(--md-sys-color-on-surface);
  }

  .meta {
    margin: 0;
    display: grid;
    grid-template-columns: 140px 1fr;
    gap: 4px 12px;

    dt { color: var(--md-sys-color-on-surface-variant); }
    dd { margin: 0; color: var(--md-sys-color-on-surface); }
  }
}
</style>
