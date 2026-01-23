<template>
  <div class="top-app-bar">
    <v-checkbox
touch-target="wrapper" :checked="allChecked" :indeterminate="!allChecked && checked"
      @change="toggleAllChecked" />
    <div class="title">
      <span v-if="selectedIds.length">{{ $t('x_selected', {
        count: realAllChecked ? total.toLocaleString() :
          selectedIds.length.toLocaleString()
      }) }}</span>
      <div v-else class="breadcrumb">
        <template v-for="(item, index) in breadcrumbPaths" :key="item.path">
          <template v-if="index === 0">
            <span
v-if="index === breadcrumbPaths.length - 1 || item.path === breadcrumbCurrentDir"
              v-tooltip="getPageStats()">{{ item.name }} ({{ total }})</span>
            <a v-else v-tooltip="getPageStats()" href="#" @click.stop.prevent="navigateToDir(item.path)">{{ item.name
              }}</a>
          </template>
          <template v-else>
            <span v-if="index === breadcrumbPaths.length - 1 || item.path === breadcrumbCurrentDir">{{ item.name }} ({{
              total }})</span>
            <a v-else href="#" @click.stop.prevent="navigateToDir(item.path)">{{ item.name }}</a>
          </template>
        </template>
      </div>
      <template v-if="checked">
        <v-icon-button v-tooltip="$t('delete')" @click.stop="deleteItems">
          <i-material-symbols:delete-forever-outline-rounded />
        </v-icon-button>
        <v-icon-button v-tooltip="$t('restore')" @click.stop="restoreItems">
          <i-material-symbols:restore-from-trash-outline-rounded />
        </v-icon-button>
        <v-icon-button v-tooltip="$t('download')" :loading="downloadLoading" @click.stop="downloadItems">
          <i-material-symbols:download-rounded />
        </v-icon-button>
      </template>
    </div>

    <div v-if="!isPhone && !checked" class="actions">
      <FilesActionButtonsTrash
:current-dir="currentDir" :refreshing="refreshing" :sorting="sorting"
        :sort-items="sortItems" :file-sort-by="fileSortBy" @refresh-current-dir="refreshCurrentDir" @sort="sort" />
    </div>
  </div>

  <div v-if="isPhone && !checked" class="secondary-actions">
    <FilesActionButtonsTrash
:current-dir="currentDir" :refreshing="refreshing" :sorting="sorting"
      :sort-items="sortItems" :file-sort-by="fileSortBy" @refresh-current-dir="refreshCurrentDir" @sort="sort" />
  </div>

  <div v-if="loading && firstInit" class="scroller-wrapper">
    <div class="scroller main-list">
      <FileSkeletonItem v-for="i in 20" :key="i" :index="i" :is-phone="isPhone" />
    </div>
  </div>
  <div class="scroller-wrapper">
    <VirtualList
v-if="items.length > 0" class="scroller main-list" :data-key="'id'" :data-sources="items"
      :estimate-size="80">
      <template #item="{ index, item }">
        <FileListItemTrash
:item="item" :index="index" :selected-ids="selectedIds"
          :shift-effecting-ids="shiftEffectingIds" :should-select="shouldSelect" :is-phone="isPhone"
          :image-error-ids="imageErrorIds" :extension-image-error-ids="extensionImageErrorIds"
          :handle-item-click="handleItemClick" :handle-mouse-over="handleMouseOver" :toggle-select="toggleSelect"
          :on-image-error="onImageError" :on-extension-image-error="onExtensionImageError" :view-item="viewItem"
          :click-item="clickItem" @download-dir="downloadDir" @download-file="downloadFile" @delete-item="deleteItem"
          @restore-item="restoreItem" @copy-link="copyLinkItem" />
      </template>
    </VirtualList>
    <div v-if="!loading && items.length === 0" class="no-data-placeholder">
      {{ $t(noDataKey(loading)) }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onActivated, onDeactivated, reactive, ref } from 'vue'
import { formatFileSize } from '@/lib/format'
import { useI18n } from 'vue-i18n'
import { useMainStore } from '@/stores/main'
import { storeToRefs } from 'pinia'
import { type IFile, canOpenInBrowser, canView, getSortItems, enrichFile, isTextFile } from '@/lib/file'
import { getFileName, getFileUrlByPath, getFileId } from '@/lib/api/file'
import { noDataKey } from '@/lib/list'
import emitter from '@/plugins/eventbus'
import { useDownload, useVolumes, useView, useSearch } from '@/hooks/files'
import { useTempStore } from '@/stores/temp'
import { openModal } from '@/components/modal'
import DownloadMethodModal from '@/components/DownloadMethodModal.vue'
import DeleteFileConfirm from '@/components/DeleteFileConfirm.vue'
import { useRoute } from 'vue-router'
import { decodeBase64, shortUUID } from '@/lib/strutil'
import { initMutation, setTempValueGQL, restoreFilesGQL } from '@/lib/api/mutation'
import type { ISource } from '@/components/lightbox/types'
import type { IFileDeletedEvent, IFileFilter, IBreadcrumbItem, IMediaItemsActionedEvent } from '@/lib/interfaces'
import { useSelectable } from '@/hooks/list'
import { useFilesKeyEvents } from '@/hooks/key-events'
import { filesGQL, initLazyQuery } from '@/lib/api/query'
import toast from '@/components/toaster'
import VirtualList from '@/components/virtualscroll'
import { replacePath } from '@/plugins/router'
import { remove } from 'lodash-es'
import { useFilesStore } from '@/stores/files'
import { buildQuery } from '@/lib/search'
import { getTrashDisplayName } from '@/lib/trash'
import { shouldHideSystemPath } from '@/lib/system-folders'

const isPhone = inject('isPhone') as boolean
const { t } = useI18n()
const sources = ref([])
const { parseQ, buildQ } = useSearch()
const filter = reactive<IFileFilter>({
  rootPath: '',
  showHidden: false,
  type: '',
  text: '',
  relativePath: '',
  trash: false,
})

const route = useRoute()
const query = route.query
const q = ref('')
const items = ref<IFile[]>([])
const { selectedIds, allChecked, realAllChecked, clearSelection, toggleAllChecked, toggleSelect, total, checked, shiftEffectingIds, handleItemClick, handleMouseOver, selectAll, shouldSelect } =
  useSelectable(items)
const { keyDown: pageKeyDown, keyUp: pageKeyUp } = useFilesKeyEvents(selectAll, clearSelection, () => {
  deleteItems()
})
const refreshing = ref(false)
const sorting = ref(false)

const imageErrorIds = ref<string[]>([])
const extensionImageErrorIds = ref<string[]>([])
const onImageError = (id: string) => {
  imageErrorIds.value.push(id)
}
const onExtensionImageError = (id: string) => {
  extensionImageErrorIds.value.push(id)
}

const sortItems = getSortItems()

const mainStore = useMainStore()
const { fileSortBy } = storeToRefs(mainStore)

const tempStore = useTempStore()
const { urlTokenKey } = storeToRefs(tempStore)
const rootDir = computed(() => filter.rootPath)
// TODO: migrate to volumes for per-root stats if needed
const { volumes, refetch: refetchStats } = useVolumes()
const { downloadFile, downloadDir, downloadFiles } = useDownload(urlTokenKey)
const { view } = useView(sources, (s: ISource[], index: number) => {
  tempStore.lightbox = {
    sources: s,
    index: index,
    visible: true,
  }
})

const page = ref(parseInt(query.page?.toString() ?? '1'))
const limit = 10000 // not paging for now

const currentDir = computed(() => {
  return ''
})

const breadcrumbCurrentDir = computed(() => {
  const v = filter.trash ? (filter.rootPath || '') : currentDir.value
  return v.replace(/\/+$/, '')
})

const breadcrumbPaths = computed(() => {
  if (filter.trash) {
    const paths: IBreadcrumbItem[] = [{ path: '', name: t('trash') }]
    const full = (filter.rootPath || '').replace(/\/+$/, '')
    if (!full) {
      return paths
    }

    // Build crumbs from the physical trash path, but hide internal segments.
    // Example: /mnt/d1/.nas-trash/data/2026/01/d_<id>__Images
    // Shows: Trash / 2026 / 01 / Images
    const marker = '/.nas-trash/'
    const idx = full.indexOf(marker)
    if (idx < 0) {
      // Fallback: just show the current folder name.
      paths.push({ path: full, name: getTrashDisplayName(getFileName(full)) })
      return paths
    }

    const base = `${full.substring(0, idx)}/.nas-trash`
    const after = full.substring(idx + marker.length)
    const parts = after.split('/').filter(Boolean)

    // Hide internal bucket layout segments from display, but keep them in the
    // accumulated path so navigation stays correct.
    const skip = new Set<number>()
    if (parts[0] === 'data') {
      skip.add(0)
      if (/^\d{4}$/.test(parts[1] || '') && /^\d{2}$/.test(parts[2] || '')) {
        skip.add(1)
        skip.add(2)
      }
    }

    let acc = base
    for (let i = 0; i < parts.length; i++) {
      const seg = parts[i]
      acc = `${acc}/${seg}`
      if (skip.has(i)) continue
      paths.push({ path: acc, name: getTrashDisplayName(seg) })
    }
    return paths
  }
  const paths: IBreadcrumbItem[] = []
  const root = rootDir.value
  let p = ''
  while (p && p !== root) {
    paths.unshift({ path: p, name: getFileName(p) })
    p = p.substring(0, p.lastIndexOf('/'))
  }
  if (root) {
    paths.unshift({ path: root, name: getPageTitle() })
  }
  return paths
})

const firstInit = ref(true)

const { mutate: restoreFilesMutation } = initMutation({
  document: restoreFilesGQL,
})
const { loading, fetch } = initLazyQuery({
  handle: async (data: any, error: string) => {
    firstInit.value = false
    refreshing.value = false
    sorting.value = false
    if (error) {
      toast(t(error), 'error')
    } else {
      const list: IFile[] = []
      for (const item of data.files) {
        if (shouldHideSystemPath(item?.path, !!item?.isDir)) continue
        list.push(enrichFile(item, urlTokenKey.value))
      }
      items.value = list
      total.value = list.length
    }
  },
  document: filesGQL,
  variables: () => ({
    offset: (page.value - 1) * limit,
    limit,
    query: q.value,
    sortBy: fileSortBy.value,
  }),
  options: {
    fetchPolicy: 'cache-and-network',
  },
})

const {
  loading: downloadLoading,
  mutate: setTempValue,
  onDone: setTempValueDone,
} = initMutation({
  document: setTempValueGQL,
})

setTempValueDone((r: any) => {
  downloadFiles(r.data.setTempValue.key)
  clearSelection()
})

const downloadItems = () => {
  const selected = items.value.filter((it) => selectedIds.value.includes(it.id))
  if (selected.length === 0) {
    toast(t('select_first'), 'error')
    return
  }

  if (selected.length === 1) {
    const item = selected[0]
    if (item.isDir) {
      downloadDir(item.path)
    } else {
      downloadFile(item.path)
    }
    clearSelection()
    return
  }

  openModal(DownloadMethodModal, {
    onEach: async () => {
      for (const it of selected) {
        if (it.isDir) {
          downloadDir(it.path)
        } else {
          downloadFile(it.path)
        }
        await new Promise((resolve) => setTimeout(resolve, 250))
      }
      clearSelection()
    },
    onZip: () => {
      setTempValue({
        key: shortUUID(),
        value: JSON.stringify(
          selectedIds.value.map((it: string) => ({
            path: it,
          }))
        ),
      })
    },
  })
}

const onDeleted = (paths: string[]) => {
  paths.forEach((p) => {
    remove(items.value, (it: IFile) => it.path === p)
  })
  total.value = items.value.length
  clearSelection()
  emitter.emit('file_deleted', { paths: paths })
  refetchStats()
}

const onRestored = (paths: string[]) => {
  paths.forEach((p) => {
    remove(items.value, (it: IFile) => it.id === p)
  })
  total.value = items.value.length
  clearSelection()
  refetchStats()
}

const deleteItems = () => {
  openModal(DeleteFileConfirm, {
    files: items.value.filter((it) => selectedIds.value.includes(it.id)),
    onDone: (files: IFile[]) => {
      onDeleted(files.map((it) => it.path))
    },
  })
}

const restoreItems = () => {
  const selected = items.value.filter((it) => selectedIds.value.includes(it.id))
  if (selected.length === 0) {
    toast(t('select_first'), 'error')
    return
  }
  const paths = selected.map((it) => it.path)
  restoreFilesMutation({ paths }).then(() => {
    onRestored(paths)
    emitter.emit('file_restored', { paths })
    // Ensure listing reflects actual filesystem state after restoring.
    fetch()
  })
}

function getPageTitle() {
  return t('trash')
}

function getPageStats() {
  const v = volumes.value.find((v) => v.mountPoint === rootDir.value)
  if (v) {
    return `${t('storage_free_total', {
      free: formatFileSize(v.freeBytes ?? 0),
      total: formatFileSize(v.totalBytes ?? 0),
    })}`
  }
  return ''
}

function navigateToDir(dir: string) {
  clearSelection()

  // Clear search text when navigating to avoid filtering the new directory.
  filter.text = ''

  // Trash browsing uses the physical `.nas-trash` directory path as root_path.
  filter.rootPath = dir
  filter.relativePath = ''

  const q = buildQ(filter)
  replacePath(mainStore, getUrl(q))
}

function getUrl(q: string) {
  const base = '/files/trash'
  return q ? `${base}?q=${q}` : base
}

function clickItem(item: IFile) {
  if (item.isDir) {
    navigateToDir(item.path)
    return
  }
  if (isTextFile(item.name)) {
    // Open text files in new window with custom viewer
    const fileId = getFileId(urlTokenKey.value, item.path)
    window.open(`/text-file?id=${encodeURIComponent(fileId)}`, '_blank')
  } else if (canOpenInBrowser(item.name)) {
    window.open(getFileUrlByPath(urlTokenKey.value, item.path), '_blank')
  } else if (canView(item.name)) {
    view(items.value, item)
  } else {
    downloadFile(item.path)
  }
}

function viewItem(event: Event, item: IFile) {
  if (item.isDir) {
    return
  }

  event.stopPropagation()
  if (isTextFile(item.name)) {
    // Open text files in new window with custom viewer
    const fileId = getFileId(urlTokenKey.value, item.path)
    window.open(`/text-file?id=${encodeURIComponent(fileId)}`, '_blank')
  } else if (canOpenInBrowser(item.name)) {
    window.open(getFileUrlByPath(urlTokenKey.value, item.path), '_blank')
  } else if (canView(item.name)) {
    view(items.value, item)
  } else {
    downloadFile(item.path)
  }
}

function sort(value: string) {
  if (fileSortBy.value === value) {
    return
  }
  sorting.value = true
  fileSortBy.value = value
}

function refreshCurrentDir() {
  refreshing.value = true
  fetch()
}

function copyLinkItem(item: IFile) {
  const url = getFileUrlByPath(urlTokenKey.value, item.path)

  // Try modern clipboard API first
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard
      .writeText(url)
      .then(() => {
        toast(t('link_copied'))
      })
      .catch(() => {
        fallbackCopyToClipboard(url)
      })
  } else {
    // Fallback for older browsers or non-HTTPS environments
    fallbackCopyToClipboard(url)
  }
}

function fallbackCopyToClipboard(text: string) {
  try {
    // Create a temporary textarea element
    const textArea = document.createElement('textarea')
    textArea.value = text
    textArea.style.position = 'fixed'
    textArea.style.left = '-999999px'
    textArea.style.top = '-999999px'
    document.body.appendChild(textArea)
    textArea.focus()
    textArea.select()

    // Try to copy using execCommand
    const successful = document.execCommand('copy')
    document.body.removeChild(textArea)

    if (successful) {
      toast(t('link_copied'))
    } else {
      toast(t('copy_failed'), 'error')
    }
  } catch (err) {
    console.error('Failed to copy text: ', err)
    toast(t('copy_failed'), 'error')
  }
}

function deleteItem(item: IFile) {
  openModal(DeleteFileConfirm, {
    files: [item],
    onDone: () => {
      onDeleted([item.path])
    },
  })
}

function restoreItem(item: IFile) {
  restoreFilesMutation({ paths: [item.path] }).then(() => {
    onRestored([item.path])
    emitter.emit('file_restored', { paths: [item.path] })
    // Ensure listing reflects actual filesystem state after restoring.
    fetch()
  })
}

const mediaItemsActionedHandler = (event: IMediaItemsActionedEvent) => {
  if (['delete', 'restore'].includes(event.action)) {
    fetch()
    refetchStats()
  }
}

onActivated(() => {
  q.value = decodeBase64(query.q?.toString() ?? '')
  parseQ(filter, q.value)
  filter.trash = true
  filter.showHidden = true
  if (!q.value.includes('trash:')) {
    const trashQ = buildQuery([{ name: 'trash', op: '', value: 'true' }])
    q.value = q.value ? `${q.value} ${trashQ}` : trashQ
  }
  fetch()
  emitter.on('media_items_actioned', mediaItemsActionedHandler)
  window.addEventListener('keydown', pageKeyDown)
  window.addEventListener('keyup', pageKeyUp)
})

onDeactivated(() => {
  emitter.off('media_items_actioned', mediaItemsActionedHandler)
  window.removeEventListener('keydown', pageKeyDown)
  window.removeEventListener('keyup', pageKeyUp)
})
</script>
<style lang="scss" scoped>
.breadcrumb {
  a {
    &:not(:last-child) {
      &::after {
        content: '/';
        margin-inline: 4px;
      }
    }
  }
}

.main-files {
  .scroller-wrapper {
    position: relative;
    height: 100%;

    .drag-mask {
      left: 16px;
      right: 16px;
    }
  }
}
</style>
