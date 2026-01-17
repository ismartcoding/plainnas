<template>
  <div class="top-app-bar">
    <div class="actions">
      <v-outlined-button v-if="selectedDirs.length" value="reset" @click="selectAll">{{ t('clear_selection')
        }}</v-outlined-button>
      <v-filled-button value="save" @click="save">{{ t('save') }}</v-filled-button>
    </div>
  </div>

  <div class="scroll-content settings-page settings-page--mobile-top">
    <p class="page-desc">{{ t('media_sources_desc') }}</p>

    <div class="picker">
      <div class="selected">
        <div class="selected-row">
          <div class="selected-chips">
            <v-filter-chip v-if="isAll" :label="`${t('all')} (/)`" :selected="true" @click="beginSelect" />
            <v-input-chip v-for="dir in selectedDirs" :key="dir" :label="dir" :remove-only="true"
              :aria-label-remove="t('remove')" @remove="removeDir(dir)" />
          </div>
        </div>
      </div>

      <DirectoryBrowser :volumes="volumes" :active-root="rootPath" :current-dir="currentDir" :can-go-up="canGoUp"
        :listing="listing" :dir-items="dirItems" :dir-name="dirName" :dir-disabled="isCovered"
        :volume-title="volumeTitle" :volume-used-percent="volumeUsedPercent"
        :volume-count="(v) => formatUsedTotalBytes(v.usedBytes, v.totalBytes)" :browser-min-height-px="260"
        :list-min-height-px="160" @select-root="selectRoot" @go-up="goUp" @enter-dir="enterDir">
        <template #toolbar-actions>
          <v-icon-button v-if="currentDir && !isCovered(currentDir)" v-tooltip="t('add')"
            @click.stop="addDir(currentDir)">
            <i-material-symbols:add-rounded />
          </v-icon-button>
          <v-icon-button v-if="currentDir && isSelected(currentDir)" disabled>
            <i-material-symbols:check-rounded />
          </v-icon-button>
        </template>

        <template #row-actions="{ dir }">
          <v-icon-button v-if="isSelected(dir)" disabled>
            <i-material-symbols:check-rounded />
          </v-icon-button>
          <v-icon-button v-else-if="!isCovered(dir)" v-tooltip="t('add')" @click.stop="addDir(dir)">
            <i-material-symbols:add-rounded />
          </v-icon-button>
        </template>
      </DirectoryBrowser>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { buildQuery, type IFilterField } from '@/lib/search'
import { filesGQL, initLazyQuery, initQuery, storageVolumesGQL } from '@/lib/api/query'
import type { IStorageVolume } from '@/lib/interfaces'
import { getFileName } from '@/lib/api/file'
import { formatUsedTotalBytes } from '@/lib/format'
import toast from '@/components/toaster'
import { useMediaSourceDirs } from '@/hooks/media'
import { getStorageVolumeTitle, sortStorageVolumesByTitle } from '@/lib/volumes'
import DirectoryBrowser from '@/components/DirectoryBrowser.vue'

const { t } = useI18n()
const { sourceDirs, save: saveDirs } = useMediaSourceDirs()

const selectedDirs = ref<string[]>([...(sourceDirs.value ?? [])])
const dirty = ref(false)
const isAll = computed(() => selectedDirs.value.length === 0)

const volumes = ref<IStorageVolume[]>([])
const rootPath = ref('')
const relativePath = ref('')
const dirItems = ref<string[]>([])

const currentDir = computed(() => {
  const root = rootPath.value || '/'
  const rel = (relativePath.value || '').replace(/^\/+/, '')
  if (!rel) return root
  if (root === '/') return `/${rel}`.replace(/\/+/g, '/').replace(/\/+$/g, '')
  return `${root}/${rel}`.replace(/\/+/g, '/').replace(/\/+$/g, '')
})

const canGoUp = computed(() => {
  return !!rootPath.value && (relativePath.value || '').trim() !== ''
})

function isSelected(dir: string) {
  return selectedDirs.value.includes(dir)
}

function isCovered(dir: string) {
  if (!dir) return false

  for (const s of selectedDirs.value) {
    if (s === dir) return true
    if (dir.startsWith(s + '/')) return true
  }
  return false
}

function beginSelect() {
  if (!rootPath.value) {
    rootPath.value = volumes.value[0]?.mountPoint || '/'
    relativePath.value = ''
  }
}

function buildFilesQuery() {
  const fields: IFilterField[] = []
  const root = (rootPath.value || '').trim()
  const rel = (relativePath.value || '').trim()
  if (root) fields.push({ name: 'root_path', op: '', value: root })
  if (rel) fields.push({ name: 'relative_path', op: '', value: rel })
  return buildQuery(fields)
}

function normalizeRelativeFromAbs(absPath: string) {
  const root = rootPath.value || ''
  if (!root) return ''
  if (root === '/') return absPath.replace(/^\/+/, '')
  if (!absPath.startsWith(root)) return ''
  return absPath.slice(root.length).replace(/^\/+/, '')
}

function dirName(path: string) {
  return getFileName(path) || path
}

function volumeUsedPercent(v: IStorageVolume) {
  const total = Number(v.totalBytes || 0)
  const used = Number(v.usedBytes || 0)
  if (!total) return 0
  const pct = (used / total) * 100
  if (!Number.isFinite(pct)) return 0
  return Math.max(0, Math.min(100, pct))
}

function volumeTitle(v: IStorageVolume) {
  return getStorageVolumeTitle(v, t)
}

function selectRoot(mountPoint: string) {
  rootPath.value = mountPoint || '/'
  relativePath.value = ''
}

function enterDir(absPath: string) {
  if (isCovered(absPath)) return
  relativePath.value = normalizeRelativeFromAbs(absPath)
}

function goUp() {
  const rel = (relativePath.value || '').replace(/\/+$/, '')
  const idx = rel.lastIndexOf('/')
  if (idx <= 0) {
    relativePath.value = ''
  } else {
    relativePath.value = rel.slice(0, idx)
  }
}

function selectAll() {
  dirty.value = true
  selectedDirs.value = []
}

function addDir(dir: string) {
  dirty.value = true
  const v = String(dir || '').trim()
  if (!v) return
  if (v === '/') {
    selectedDirs.value = []
    return
  }
  if (selectedDirs.value.includes(v)) return

  const prefix = v.replace(/\/+$/, '') + '/'
  selectedDirs.value = selectedDirs.value.filter((d) => !d.startsWith(prefix))

  selectedDirs.value.push(v)
}

function removeDir(dir: string) {
  dirty.value = true
  selectedDirs.value = selectedDirs.value.filter((d) => d !== dir)
}

function save() {
  saveDirs(selectedDirs.value)
    .then(() => {
      dirty.value = false
      toast(t('saved'))
    })
    .catch(() => {
      toast(t('error'), 'error')
    })
}

// Sync initial value from server once loaded, unless the user already modified it.
watch(sourceDirs, (dirs) => {
  if (dirty.value) return
  selectedDirs.value = [...(dirs ?? [])]
})

initQuery<{ storageVolumes: IStorageVolume[] }>({
  document: storageVolumesGQL,
  handle: (data, error) => {
    if (error) {
      toast(t(error), 'error')
      return
    }
    volumes.value = sortStorageVolumesByTitle(data?.storageVolumes ?? [], t)
    if (!rootPath.value) {
      rootPath.value = volumes.value[0]?.mountPoint || '/'
    }
  },
})

const { loading: listing, fetch: fetchDirs } = initLazyQuery<{ files: Array<{ path: string; isDir: boolean }> }>({
  document: filesGQL,
  variables: () => ({
    offset: 0,
    limit: 10000,
    query: buildFilesQuery(),
    sortBy: 'NAME_ASC',
  }),
  handle: (data, error) => {
    if (error) {
      dirItems.value = []
      toast(t(error), 'error')
      return
    }
    const files = (data?.files ?? []) as Array<{ path: string; isDir: boolean }>
    dirItems.value = files.filter((f) => f.isDir).map((f) => f.path)
  },
})

watch([rootPath, relativePath], () => {
  if (!rootPath.value) return
  fetchDirs()
}, { immediate: true })
</script>

<style scoped>
/* Prevent long paths/chips from forcing horizontal growth in flex layouts. */
.scroll-content {
  overflow-x: hidden;
}

.picker {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.selected-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.selected-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

@media (max-width: 720px) {
  .selected-row {
    flex-direction: column;
    align-items: stretch;
  }

  /* padding handled by global .settings-page */
}
</style>
