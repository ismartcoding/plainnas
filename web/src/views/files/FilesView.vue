<template>
  <div class="top-app-bar">
    <v-checkbox touch-target="wrapper" :checked="allChecked" :indeterminate="!allChecked && checked"
      @change="toggleAllChecked" />
    <div class="title">
      <span v-if="selectedIds.length">{{ $t('x_selected', {
        count: realAllChecked ? displayTotal.toLocaleString() :
          selectedIds.length.toLocaleString()
      }) }}</span>
      <div v-else class="breadcrumb">
        <template v-for="(item, index) in breadcrumbPaths" :key="item.path">
          <template v-if="index === 0">
            <span v-if="index === breadcrumbPaths.length - 1 || item.path === breadcrumbCurrentDir"
              v-tooltip="getPageStats()">{{ item.name }} ({{ displayTotal }})</span>
            <a v-else v-tooltip="getPageStats()" href="#" @click.stop.prevent="navigateToDir(item.path)">{{ item.name
            }}</a>
          </template>
          <template v-else>
            <span v-if="index === breadcrumbPaths.length - 1 || item.path === breadcrumbCurrentDir">{{ item.name }} ({{
              displayTotal }})</span>
            <a v-else href="#" @click.stop.prevent="navigateToDir(item.path)">{{ item.name }}</a>
          </template>
        </template>
      </div>
      <template v-if="checked">
        <template v-if="uiMode === 'edit'">
          <v-icon-button v-tooltip="$t('copy')" @click.stop="copyItems">
            <i-material-symbols:content-copy-outline-rounded />
          </v-icon-button>
          <v-icon-button v-tooltip="$t('cut')" @click.stop="cutItems">
            <i-material-symbols:content-cut-rounded />
          </v-icon-button>
          <v-icon-button v-tooltip="$t('trash')" @click.stop="trashItems">
            <i-material-symbols:delete-outline-rounded />
          </v-icon-button>
        </template>
        <v-icon-button v-tooltip="$t('download')" :loading="downloadLoading" @click.stop="downloadItems">
          <i-material-symbols:download-rounded />
        </v-icon-button>
      </template>
    </div>

    <div v-if="!isPhone && !checked" class="actions">
      <FilesToolbarActions :ui-mode="uiMode" :current-dir="currentDir" :upload-menu-visible="uploadMenuVisible"
        :more-menu-visible="moreMenuVisible" :can-paste="canPaste()" :pasting="pasting" :refreshing="refreshing"
        :show-hidden="filter.showHidden" :sort-by="fileSortBy" :sort-items="sortItems" :on-toggle-ui-mode="toggleUIMode"
        :on-create-dir="createDir" :on-upload-files="uploadFilesClick" :on-upload-dir="uploadDirClick"
        :on-paste="pasteDir" :on-refresh="refreshCurrentDir" :on-open-keyboard-shortcuts="openKeyboardShortcuts"
        :on-toggle-show-hidden="onToggleShowHidden" :on-sort="sort"
        @update:uploadMenuVisible="(v) => uploadMenuVisible = v" @update:moreMenuVisible="(v) => moreMenuVisible = v" />
    </div>
  </div>

  <div v-if="isPhone && !checked" class="secondary-actions">
    <FilesToolbarActions :ui-mode="uiMode" :current-dir="currentDir" :upload-menu-visible="uploadMenuVisible"
      :more-menu-visible="moreMenuVisible" :can-paste="canPaste()" :pasting="pasting" :refreshing="refreshing"
      :show-hidden="filter.showHidden" :sort-by="fileSortBy" :sort-items="sortItems" :on-toggle-ui-mode="toggleUIMode"
      :on-create-dir="createDir" :on-upload-files="uploadFilesClick" :on-upload-dir="uploadDirClick"
      :on-paste="pasteDir" :on-refresh="refreshCurrentDir" :on-open-keyboard-shortcuts="openKeyboardShortcuts"
      :on-toggle-show-hidden="onToggleShowHidden" :on-sort="sort"
      @update:uploadMenuVisible="(v) => uploadMenuVisible = v" @update:moreMenuVisible="(v) => moreMenuVisible = v" />
  </div>

  <div v-if="loading && firstInit" class="scroller-wrapper">
    <div class="scroller main-list">
      <FileSkeletonItem v-for="i in 20" :key="i" :index="i" :is-phone="isPhone" />
    </div>
  </div>
  <div ref="listWrapperRef" class="scroller-wrapper" @dragover.stop.prevent="fileDragEnter">
    <div v-show="dropping" class="drag-mask" @drop.stop.prevent="dropFiles2" @dragleave.stop.prevent="fileDragLeave">{{
      $t('release_to_send_files') }}</div>
    <VirtualList v-if="items.length > 0" class="scroller main-list" :data-key="'id'" :data-sources="items"
      :estimate-size="80" :bottom-threshold="600" @tobottom="loadMore" :class="{ 'select-mode': checked }">
      <template #item="{ index, item }">
        <FileListItem :item="item" :index="index" :selected-ids="selectedIds" :shift-effecting-ids="shiftEffectingIds"
          :should-select="shouldSelect" :is-phone="isPhone" :image-error-ids="imageErrorIds"
          :extension-image-error-ids="extensionImageErrorIds" :can-paste="canPaste()" :edit-mode="uiMode === 'edit'"
          :handle-item-click="handleItemClick" :handle-mouse-over="handleMouseOverMode" :toggle-select="toggleSelect"
          :on-image-error="onImageError" :on-extension-image-error="onExtensionImageError" :view-item="viewItem"
          :click-item="clickItem" @download-dir="downloadDir" @download-file="downloadFile"
          @upload-files="uploadFilesClick" @upload-dir="uploadDirClick" @delete-item="deleteItem"
          @trash-item="deleteItem" @restore-item="restoreItem" @duplicate-item="duplicateItem" @cut-item="cutItem"
          @copy-item="copyItem" @paste-item="pasteItem" @copy-link="copyLinkItem" @rename-item="renameItemClick"
          @add-to-favorites="addToFavoritesClick" />
      </template>
      <template #footer>
        <div v-if="loadingMore || prefetching" class="list-footer">
          <v-circular-progress indeterminate class="sm" />
          <span>{{ $t('loading') }}</span>
        </div>
      </template>
    </VirtualList>
    <div v-if="!loading && items.length === 0" class="no-data-placeholder">
      {{ $t(noDataKey(loading)) }}
    </div>
    <input ref="fileInput" style="display: none" type="file" multiple @change="uploadChanged" />
    <input ref="dirFileInput" style="display: none" type="file" multiple webkitdirectory mozdirectory directory
      @change="dirUploadChanged" />
  </div>
</template>

<script setup lang="ts">
import { computed, inject, onActivated, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMainStore } from '@/stores/main'
import { storeToRefs } from 'pinia'
import { type IFile, getSortItems } from '@/lib/file'
import { getFileName } from '@/lib/api/file'
import { noDataKey } from '@/lib/list'
import { useCreateDir, useDownload, useVolumes, useView, useCopyPaste, useSearch } from '@/hooks/files'
import { useDragDropUpload, useFileUpload } from '@/hooks/upload'
import { useTempStore } from '@/stores/temp'
import { openModal } from '@/components/modal'
import EditValueModal from '@/components/EditValueModal.vue'
import { useRoute } from 'vue-router'
import { decodeBase64 } from '@/lib/strutil'
import type { ISource } from '@/components/lightbox/types'
import type { IFileFilter } from '@/lib/interfaces'
import { useSelectable } from '@/hooks/list'
import { useFilesKeyEvents } from '@/hooks/key-events'
import { useFilesPaging } from './hooks/useFilesPaging'
import { useFilesBreadcrumb } from './hooks/useFilesBreadcrumb'
import { useFilesActions } from './hooks/useFilesActions'
import { useFilesNavigation } from './hooks/useFilesNavigation'
import { useFilesSubscriptions } from './hooks/useFilesSubscriptions'
import { useFilesOpenItem } from './hooks/useFilesOpenItem'
import VirtualList from '@/components/virtualscroll'
import { useFilesStore } from '@/stores/files'
import { shouldHideSystemPath } from '@/lib/system-folders'
import KeyboardShortcutsModal from '@/components/KeyboardShortcutsModal.vue'
import { filesKeyboardShortcuts } from '@/lib/shortcuts/files'
import FilesToolbarActions from '@/components/files/FilesToolbarActions.vue'

const isPhone = inject('isPhone') as boolean
const { t } = useI18n()
const mainStore = useMainStore()
const { fileSortBy } = storeToRefs(mainStore)
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
const {
  items,
  total,
  dirTotal,
  displayTotal,
  loading,
  firstInit,
  loadingMore,
  prefetching,
  listWrapperRef,
  fetch,
  fetchCount,
  resetPaging: resetPagingInternal,
  loadMore,
  activate: activatePaging,
  unbindScrollFallback,
} = useFilesPaging({
  q,
  sortBy: storeToRefs(mainStore).fileSortBy,
  urlTokenKey: storeToRefs(useTempStore()).urlTokenKey,
  initialPage: parseInt(query.page?.toString() ?? '1'),
  limit: 1000,
  isBlocked: () => refreshing.value || sorting.value || pasting.value,
  shouldHideItem: (it) => shouldHideSystemPath(it?.path, !!it?.isDir),
  t,
})

const { selectedIds, allChecked, realAllChecked, clearSelection, toggleAllChecked, toggleSelect, checked, shiftEffectingIds, handleItemClick, handleMouseOver, selectAll, shouldSelect } =
  useSelectable(items)

const uiMode = computed<'view' | 'edit'>({
  get: () => mainStore.pageUIMode.files ?? 'view',
  set: (value) => {
    mainStore.pageUIMode.files = value
  },
})
const uploadMenuVisible = ref(false)
const moreMenuVisible = ref(false)

function toggleUIMode() {
  if (uiMode.value === 'edit') {
    uiMode.value = 'view'
    clearSelection()
  } else {
    uiMode.value = 'edit'
  }
}

function openKeyboardShortcuts() {
  openModal(KeyboardShortcutsModal, {
    title: t('keyboard_shortcuts'),
    shortcuts: filesKeyboardShortcuts,
  })
}

function handleMouseOverMode(event: MouseEvent, index: number) {
  if (uiMode.value !== 'edit') return
  handleMouseOver(event, index)
}

const selectAllInEditMode = () => {
  if (uiMode.value !== 'edit') return
  selectAll()
}
const clearSelectionInEditMode = () => {
  if (uiMode.value !== 'edit') return
  clearSelection()
}
const trashInEditMode = () => {
  if (uiMode.value !== 'edit') return
  trashItems()
}

const { keyDown: pageKeyDown, keyUp: pageKeyUp } = useFilesKeyEvents(
  selectAllInEditMode,
  clearSelectionInEditMode,
  trashInEditMode,
)
const refreshing = ref(false)
const sorting = ref(false)

function resetPaging() {
  resetPagingInternal()
  clearSelection()
}

const imageErrorIds = ref<string[]>([])
const extensionImageErrorIds = ref<string[]>([])
const onImageError = (id: string) => {
  imageErrorIds.value.push(id)
}
const onExtensionImageError = (id: string) => {
  extensionImageErrorIds.value.push(id)
}

const sortItems = getSortItems()

const tempStore = useTempStore()
const { app, urlTokenKey, uploads } = storeToRefs(tempStore)
const { selectedFiles, isCut } = storeToRefs(useFilesStore())
const { dropping, fileDragEnter, fileDragLeave, dropFiles } = useDragDropUpload(uploads)
// TODO: migrate to volumes for per-root stats if needed
const { createPath, createVariables, createMutation } = useCreateDir(urlTokenKey, items)
const { volumes, refetch: refetchStats } = useVolumes()
const { downloadFile, downloadDir } = useDownload(urlTokenKey)
const { view } = useView(sources, (s: ISource[], index: number) => {
  tempStore.lightbox = {
    sources: s,
    index: index,
    visible: true,
  }
})

const { openFile } = useFilesOpenItem({
  urlTokenKey,
  docPreviewAvailable: computed(() => !!app.value?.docPreviewAvailable),
  downloadFile,
  view,
  items,
})

const { rootDir, currentDir, breadcrumbCurrentDir, breadcrumbPaths, getPageTitle, getPageStats } = useFilesBreadcrumb({
  filter,
  volumes,
  t,
})

const { syncFromRoute, navigateToDir, toggleShowHidden: onToggleShowHidden } = useFilesNavigation({
  filter,
  rootDir,
  mainStore,
  buildQ,
  q,
  resetPaging,
  clearSelection,
  fetchCount,
  fetch,
})

useFilesSubscriptions({
  fetch,
  refetchStats,
  activatePaging,
  unbindScrollFallback,
  pageKeyDown,
  pageKeyUp,
})

watch(
  () => loading.value,
  (v) => {
    // Keep existing flags in sync with query loading state.
    if (!v) {
      refreshing.value = false
      sorting.value = false
    }
  }
)
const { loading: pasting, canPaste, copy, cut, paste } = useCopyPaste(items, isCut, selectedFiles, fetch, refetchStats)
const { input: fileInput, upload: uploadFiles, uploadChanged } = useFileUpload(uploads)
const { input: dirFileInput, upload: uploadDir, uploadChanged: dirUploadChanged } = useFileUpload(uploads)

const {
  downloadLoading,
  downloadItems,
  trashItems,
  deleteItem,
  restoreItem,
  renameItemClick,
  copyItems,
  cutItems,
  pasteDir,
  duplicateItem,
  cutItem,
  copyItem,
  pasteItem,
  copyLinkItem,
  addToFavoritesClick,
} = useFilesActions({
  items,
  total,
  selectedIds,
  clearSelection,
  currentDir,
  rootDir,
  urlTokenKey,
  docPreviewAvailable: computed(() => !!app.value?.docPreviewAvailable),
  t,
  fetch,
  refetchStats,
  copy,
  cut,
  paste,
})

function clickItem(item: IFile) {
  if (item.isDir) {
    navigateToDir(item.path)
    return
  }

  openFile(item)
}

function viewItem(event: Event, item: IFile) {
  if (item.isDir) {
    return
  }

  event.stopPropagation()

  openFile(item)
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
  resetPaging()
  fetchCount()
  fetch()
}

const createDir = () => {
  createPath.value = currentDir.value
  openModal(EditValueModal, {
    title: t('create_folder'),
    placeholder: t('name'),
    mutation: createMutation,
    getVariables: createVariables,
  })
}

function uploadFilesClick(dir: string) {
  uploadFiles(dir)
}

function uploadDirClick(dir: string) {
  uploadDir(dir)
}

function dropFiles2(e: DragEvent) {
  dropFiles(e, currentDir.value, () => true)
}

onActivated(() => {
  syncFromRoute(query.q?.toString(), parseQ, mainStore.fileShowHidden)
})

watch(
  () => fileSortBy.value,
  () => {
    resetPaging()
    fetchCount()
    fetch()
  }
)

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

.list-footer {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px 0;
}
</style>
