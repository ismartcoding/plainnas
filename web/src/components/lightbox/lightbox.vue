<template>
  <Teleport to="body">
    <div v-if="tempStore.lightbox.visible" class="lightbox" @touchmove="preventDefault" @wheel="onWheel">
      <div class="layout">
        <LightboxHeader
:current="current" @close="closeDialog" @view-origin="viewOrigin" @zoom-in="zoomIn"
          @zoom-out="zoomOut" @resize="resize" @rotate-left="rotateLeft" @rotate-right="rotateRight"
          @toggle-info="lightboxInfoVisible = !lightboxInfoVisible" />
        <section class="content" @click.self="closeDialog">
          <div v-if="tempStore.lightbox.sources.length > 1 && (loop || imgIndex > 0)" class="btn-prev" @click="onPrev">
            <i-material-symbols:chevron-left-rounded />
          </div>
          <div
            v-if="tempStore.lightbox.sources.length > 1 && (loop || imgIndex < tempStore.lightbox.sources.length - 1)"
            class="btn-next" @click="onNext">
            <i-material-symbols:chevron-right-rounded />
          </div>
          <div v-if="status.loading" class="loading">
            <v-circular-progress indeterminate />
          </div>
          <div v-else-if="status.loadError" class="v-on-error">
            {{ $t('load_failed', { name: current?.name }) }}
          </div>
          <div
v-if="current && isVideo(current.name)" v-show="!status.loading && !status.loadError"
            class="v-video-wrapper" @click.self="closeDialog">
            <video
ref="video" controls autoplay="true" :src="current.src" @error="onError" @canplay="onLoad"
              @playing="onPlaying" @pause="onPause" @volumechange="onVolumeChange" />
          </div>
          <div
v-else-if="current && isAudio(current.name)" v-show="!status.loading && !status.loadError"
            class="v-audio-wrapper" @click.self="closeDialog">
            <div style="padding: 50px">
              <audio controls autoplay="true" :src="current.src" @error="onError" @canplay="onLoad" />
            </div>
          </div>
          <div
v-else-if="current && isImage(current.name)" v-show="!status.loading && !status.loadError"
            class="v-img-wrapper" :style="imgWrapperStyle">
            <img
ref="imgRef" draggable="false" class="v-img"
              :style="isSvg(current.name) ? 'min-width: ' + imgState.width + 'px;' : ''"
              :src="current?.src + (current?.viewOriginImage ? '' : '&w=1024&h=1024&cc=false')" @mousedown="onMouseDown"
              @mouseup="onMouseUp" @mousemove="onMouseMove" @touchstart="onTouchStart" @touchmove="onTouchMove"
              @touchend="onTouchEnd" @load="onLoad" @error="onError" @dblclick="onDblclick" @dragstart="
                (e) => {
                  e.preventDefault()
                }
              " />
          </div>
        </section>

        <!-- Desktop info panel -->
        <LightboxInfo
v-if="lightboxInfoVisible && !isPhone && !isTablet" :current="current" :file-info="fileInfo"
          :url-token-key="urlTokenKey ? urlTokenKey.toString() : ''" :data-dir="app.dataDir" :tags-map="tagsMap"
          :download-file="downloadFile" @rename-file="renameFile" @delete-file="deleteFile" @add-to-tags="addToTags"
          @refetch-info="refetchInfo" />
      </div>

      <!-- Mobile info bottom sheet -->
      <BottomSheet v-if="isPhone || isTablet" v-model="lightboxInfoVisible" :title="$t('info')" show-footer>
        <!-- File Details Section -->
        <LightboxFileDetails :current="current" :file-info="fileInfo" :data-dir="app.dataDir" />

        <!-- File Tags Section -->
        <LightboxFileTags :current="current" :file-info="fileInfo" @add-to-tags="addToTags" />

        <!-- Action Buttons in Footer -->
        <template #footer>
          <LightboxFileActionButtons
:current="current" :download-file="downloadFile" @rename-file="renameFile"
            @delete-file="deleteFile" @action-success="handleActionSuccess" />
        </template>
      </BottomSheet>
    </div>
  </Teleport>
</template>
<script setup lang="ts">
import { computed, ref, reactive, watch, onMounted, onBeforeUnmount, inject } from 'vue'

import { on, off, isArray, preventDefault } from './utils/index'
import { useImage, useMouse, useTouch } from './utils/hooks'
import { createAroundPreloader } from './utils/preload'
import { useLightboxFileActions } from './utils/file_actions'
import type { ISource, IImgWrapperState, IndexChangeActions } from './types'
import { isVideo, isImage, isAudio, isSvg } from '@/lib/file'
import { getFileUrlByPath } from '@/lib/api/file'
import { useTempStore } from '@/stores/temp'
import { storeToRefs } from 'pinia'
import { useMainStore } from '@/stores/main'
import { fileInfoGQL, initLazyQuery, tagsGQL } from '@/lib/api/query'
import { openModal } from '@/components/modal'
import { useI18n } from 'vue-i18n'
import UpdateTagRelationsModal from '@/components/UpdateTagRelationsModal.vue'
import type { IItemTagsUpdatedEvent, IFileDeletedEvent, IFileRenamedEvent, ITag, IMediaItemsActionedEvent } from '@/lib/interfaces'
import emitter from '@/plugins/eventbus'
import { getFileName } from '@/lib/api/file'
import { remove } from 'lodash-es'

const props = defineProps({
  loop: {
    type: Boolean,
    default: true,
  },
})

const emit = defineEmits(['on-error', 'on-prev', 'on-next', 'on-prev-click', 'on-next-click', 'on-index-change'])

const { t } = useI18n()

const ORIGIN_IMAGE_MAX_BYTES = 1024 * 1024

function shouldUseOriginImageByDefault(s?: ISource): boolean {
  if (!s) return false
  if (!isImage(s.name)) return false
  if (isSvg(s.name)) return true
  return typeof s.size === 'number' && s.size > 0 && s.size < ORIGIN_IMAGE_MAX_BYTES
}

const viewOrigin = () => {
  const c = current.value
  if (c) {
    c.viewOriginImage = true
  }
  status.loading = true
}
const tempStore = useTempStore()
const { urlTokenKey, app } = storeToRefs(tempStore)
const video = ref<HTMLVideoElement>()
const { imgRef, imgState, setImgSize } = useImage()
const imgIndex = ref(0)
const { lightboxInfoVisible } = storeToRefs(useMainStore())

const imgWrapperState = reactive<IImgWrapperState>({
  scale: 1,
  lastScale: 1,
  rotateDeg: 0,
  top: 0,
  left: 0,
  initX: 0,
  initY: 0,
  lastX: 0,
  lastY: 0,
  touches: [] as TouchList | [],
})

const status = reactive({
  loadError: false,
  loading: false,
  dragging: false,
  gesturing: false,
  swipeToLeft: false,
  swipeToRight: false,
  wheeling: false,
})

const current = ref<ISource>()
const isPhone = inject('isPhone') as boolean
const isTablet = inject('isTablet') as boolean

const currCursor = () => {
  if (status.loadError) return 'default'
  return 'move'
}

const fileInfo = ref<any>(null)

const {
  loading: infoLoading,
  load: loadInfo,
  refetch: refetchInfo,
} = initLazyQuery({
  handle: (data: any, error: string) => {
    if (error) {
      //toast(t(error), 'error')
    } else {
      if (data) {
        fileInfo.value = data.fileInfo
        updateViewOriginImageState()
      }
    }
  },
  document: fileInfoGQL,
  variables: () => ({
    id: current.value?.data?.id ?? '',
    path: current.value?.path ?? '',
  }),
})

const { downloadFile, deleteFile, renameFile } = useLightboxFileActions({
  current,
  urlTokenKey,
  t,
  refetchInfo,
})

function updateViewOriginImageState() {
  if (current.value && fileInfo.value && isImage(current.value.name) && current.value.path === fileInfo.value.path) {
    if (shouldUseOriginImageByDefault(current.value)) {
      current.value.viewOriginImage = true
      return
    }

    const infoData = fileInfo.value.data
    if (!infoData) return

    const { width, height } = infoData
    if (typeof width !== 'number' || typeof height !== 'number') return

    // If the server reports the same dimensions as what we loaded, then we're already viewing the origin.
    if (width === imgState.naturalWidth && height === imgState.naturalHeight) {
      current.value.viewOriginImage = true
    }
  }
}

const tagsMap = new Map<string, ITag[]>()
const { loading: tagsLoading, load: loadTags } = initLazyQuery({
  handle: (data: any, error: string) => {
    if (data) {
      tagsMap.set(current.value?.type ?? '', data.tags)
    }
  },
  document: tagsGQL,
  variables: () => ({
    type: current.value?.type ?? '',
  }),
})

const imgWrapperStyle = computed(() => {
  // On phone devices, adjust the top position to account for the two-row header
  const mobileOffset = isPhone ? -28 : 0 // Half of the extra height (56px/2 = 28px)

  return {
    cursor: currCursor(),
    top: `calc(50% + ${imgWrapperState.top + mobileOffset}px)`,
    left: `calc(50% + ${imgWrapperState.left}px)`,
    transition: status.dragging || status.gesturing ? 'none' : '',
    transform: `translate(-50%, -50%) scale(${imgWrapperState.scale}) rotate(${imgWrapperState.rotateDeg}deg)`,
  }
})

const closeDialog = () => {
  tempStore.lightbox.visible = false
  tempStore.lightbox.index = -1
  imgIndex.value = 0
  preloader.clear()
}

const reset = () => {
  imgWrapperState.scale = 1
  imgWrapperState.lastScale = 1
  imgWrapperState.rotateDeg = 0
  imgWrapperState.top = 0
  imgWrapperState.left = 0
  status.loadError = false
  status.dragging = false
  status.gesturing = false
  status.loading = true
}

function ensureSourceURL(s: ISource) {
  if (!s.src) {
    s.src = getFileUrlByPath(tempStore.urlTokenKey, s.path)
  }

  // Apply the same default-origin heuristic, but never override user choice.
  if (s.viewOriginImage === undefined && shouldUseOriginImageByDefault(s)) {
    s.viewOriginImage = true
  }
}

function imageDisplayURL(s: ISource): string | null {
  if (!s) return null
  if (!isImage(s.name)) return null
  ensureSourceURL(s)
  if (!s.src) return null
  return s.src + (s.viewOriginImage ? '' : '&w=1024&h=1024&cc=false')
}

const preloader = createAroundPreloader<ISource>({
  getItems: () => tempStore.lightbox.sources,
  getUrl: (s) => imageDisplayURL(s),
  loop: () => props.loop,
  ahead: 3,
  behind: 1,
  maxUrls: 60,
})

// switching imgs manually
const changeIndex = async (newIndex: number, actions?: IndexChangeActions) => {
  const oldIndex = imgIndex.value

  reset()

  const s = tempStore.lightbox.sources[newIndex]
  if (!s.src) {
    s.src = getFileUrlByPath(tempStore.urlTokenKey, s.path)
  }

  // If the original file is already small, skip thumbnail params and load it directly.
  // Do NOT force this to false for other files; preserve any prior user choice.
  if (shouldUseOriginImageByDefault(s)) {
    s.viewOriginImage = true
  }

  imgIndex.value = newIndex
  current.value = tempStore.lightbox.sources[imgIndex.value]

  // Preload neighbors ASAP (thumb generation can be slow on first hit).
  preloader.preloadAround(newIndex)

  setTimeout(() => {
    const type = current.value?.type ?? ''
    if (type && !tagsMap.has(type)) {
      loadTags()
    }
    loadInfo()
  }, 0) // Fix the bug that graphql send the {id: '', path: ''} query at first time.

  // No emit event when hidden or same index
  if (oldIndex === newIndex) return

  if (actions) {
    if (isArray(actions)) {
      actions.forEach((action) => {
        emit(action, oldIndex, newIndex)
      })
    } else {
      emit(actions, oldIndex, newIndex)
    }
  }
  emit('on-index-change', oldIndex, newIndex)
}

const onNext = () => {
  const oldIndex = imgIndex.value
  const newIndex = props.loop ? (oldIndex + 1) % tempStore.lightbox.sources.length : oldIndex + 1

  if (!props.loop && newIndex > tempStore.lightbox.sources.length - 1) return

  changeIndex(newIndex, ['on-next', 'on-next-click'])
}

const onPrev = () => {
  const oldIndex = imgIndex.value
  let newIndex = oldIndex - 1

  if (oldIndex === 0) {
    if (!props.loop) return
    newIndex = tempStore.lightbox.sources.length - 1
  }
  changeIndex(newIndex, ['on-prev', 'on-prev-click'])
}

// actions for changing img
const defaultScale = 1.5
const zoom = (newScale: number) => {
  if (Math.abs(1 - newScale) < 0.05) {
    newScale = 1
  } else if (Math.abs(imgState.maxScale - newScale) < 0.05) {
    newScale = imgState.maxScale
  }
  imgWrapperState.lastScale = imgWrapperState.scale
  imgWrapperState.scale = newScale
}

const zoomIn = () => {
  const newScale = imgWrapperState.scale * defaultScale
  if (newScale < imgState.maxScale * 100) {
    zoom(newScale)
  }
}

const zoomOut = () => {
  const newScale = imgWrapperState.scale / defaultScale
  if (newScale > 0.1) {
    zoom(newScale)
  }
}

const rotateLeft = () => {
  imgWrapperState.rotateDeg -= 90
}

const rotateRight = () => {
  imgWrapperState.rotateDeg += 90
}

const resize = () => {
  imgWrapperState.scale = 1
  imgWrapperState.top = 0
  imgWrapperState.left = 0
}

// check img moveable
const canMove = (button?: number) => {
  return button === 0
}

// mouse
const { onMouseDown, onMouseMove, onMouseUp } = useMouse(imgWrapperState, status, canMove)

const { onTouchStart, onTouchMove, onTouchEnd } = useTouch(imgState, imgWrapperState, status, canMove)

const onDblclick = () => {
  if (imgWrapperState.scale !== imgState.maxScale) {
    imgWrapperState.lastScale = imgWrapperState.scale
    imgWrapperState.scale = imgState.maxScale
  } else {
    imgWrapperState.scale = imgWrapperState.lastScale
  }
}

const onWheel = (e: WheelEvent) => {
  if (status.loadError || status.gesturing || status.loading || status.dragging || status.wheeling) {
    return
  }

  status.wheeling = true

  setTimeout(() => {
    status.wheeling = false
  }, 80)

  if (e.deltaY < 0) {
    zoomIn()
  } else {
    zoomOut()
  }
}

let isVideoPlaying = true
const onPlaying = () => {
  isVideoPlaying = true
  video.value?.blur() // make sure the keyboard event not eat up
}
const onPause = () => {
  isVideoPlaying = false
  video.value?.blur() // make sure the keyboard event not eat up
}

const onVolumeChange = () => {
  video.value?.blur() // make sure the keyboard event not eat up
}

// key press events handler
const onKeyPress = (e: Event) => {
  if (!tempStore.lightbox.visible) {
    return
  }
  const evt = e as KeyboardEvent
  if (evt.key === 'Escape') {
    // if has VueModal open should not close lightbox
    if (document.querySelector('.vue-modal')) {
      return
    }
    evt?.stopPropagation()
    closeDialog()
  } else if (evt.key === 'ArrowLeft') {
    evt?.stopPropagation()
    onPrev()
  } else if (evt.key === 'ArrowRight') {
    evt?.stopPropagation()
    onNext()
  } else if (evt.key === ' ') {
    const v = video.value
    if (v) {
      if (v.paused && !isVideoPlaying) {
        v.play()
      } else {
        v.pause()
      }
    }
  }
}

const onLoad = () => {
  status.loading = false
  if (current.value && isImage(current.value.name)) {
    setImgSize()
    updateViewOriginImageState()
  }
}

const onError = (e: Event) => {
  status.loading = false
  status.loadError = true
  emit('on-error', e)
}

const onWindowResize = () => {
  setImgSize()
}

watch(
  () => tempStore.lightbox.index,
  (newIndex) => {
    if (newIndex < 0 || newIndex >= tempStore.lightbox.sources.length) {
      return
    }
    changeIndex(newIndex)
  }
)

watch(
  () => tempStore.lightbox.sources,
  () => {
    preloader.clear()
  },
  { deep: false }
)

watch(
  () => status.dragging,
  (newStatus, oldStatus) => {
    const dragged = !newStatus && oldStatus
    if (!canMove() && dragged) {
      // if (status.swipeToLeft) {
      //   onNext()
      // } else if (status.swipeToRight) {
      //   onPrev()
      // }
    }
  }
)

function addToTags() {
  const type = current.value?.type ?? ''
  const tags = tagsMap.get(type) ?? []
  const item = current.value?.data ?? {}
  openModal(UpdateTagRelationsModal, {
    type,
    tags: tags,
    item: {
      key: item.id,
      title: item.title,
      size: item.size,
    },
    selected: tags.filter((it: ITag) => fileInfo.value?.tags.some((t: ITag) => t.id === it.id)),
  })
}

function handleActionSuccess(action: string) {
  // Close BottomSheet on mobile after successful trash/restore operations
  if (isPhone && (action === 'trash' || action === 'restore')) {
    lightboxInfoVisible.value = false
  }
}

const itemTagsUpdatedHandler = (event: IItemTagsUpdatedEvent) => {
  if (event.item.key === current.value?.data?.id) {
    refetchInfo()
  }
}

const mediaItemsActionedHandler = (event: IMediaItemsActionedEvent) => {
  const isMedia = Boolean(current.value?.data?.id && current.value?.type)
  const query = isMedia ? `ids:${current.value?.data?.id}` : `path:${current.value?.path ?? ''}`
  if (!query || !['delete', 'trash', 'restore'].includes(event.action) || event.query !== query) {
    return
  }
  if (isMedia) {
    remove(tempStore.lightbox.sources, (it: ISource) => `ids:${it.data?.id}` === event.query)
  } else {
    remove(tempStore.lightbox.sources, (it: ISource) => `path:${it.path}` === event.query)
  }
  if (tempStore.lightbox.sources.length) {
    onNext()
  } else {
    closeDialog()
  }
}

const fileDeletedHandler = (event: IFileDeletedEvent) => {
  if (event.paths.includes(current.value?.data?.path)) {
    remove(tempStore.lightbox.sources, (it: ISource) => event.paths.includes(it.path))
    if (tempStore.lightbox.sources.length) {
      onNext()
    } else {
      closeDialog()
    }
  }
}

const fileRenamedHandler = (event: IFileRenamedEvent) => {
  tempStore.lightbox.sources.forEach((source: ISource) => {
    if (source.path === event.oldPath) {
      source.path = event.newPath
      source.name = getFileName(event.newPath)
      if (source.data) {
        source.data.path = event.newPath
        source.data.name = getFileName(event.newPath)
      }
    }
  })

  // If the currently displayed file was renamed, update the display
  if (current.value && current.value.path === event.oldPath) {
    current.value.path = event.newPath
    current.value.name = getFileName(event.newPath)
    if (current.value.data) {
      current.value.data.path = event.newPath
      current.value.data.name = getFileName(event.newPath)
    }
  }
}

onMounted(() => {
  on(window, 'keydown', onKeyPress)
  on(window, 'resize', onWindowResize)
  emitter.on('item_tags_updated', itemTagsUpdatedHandler)
  emitter.on('media_items_actioned', mediaItemsActionedHandler)
  emitter.on('file_deleted', fileDeletedHandler)
  emitter.on('file_renamed', fileRenamedHandler)
})

onBeforeUnmount(() => {
  off(window, 'keydown', onKeyPress)
  off(window, 'resize', onWindowResize)
  emitter.off('item_tags_updated', itemTagsUpdatedHandler)
  emitter.off('media_items_actioned', mediaItemsActionedHandler)
  emitter.off('file_deleted', fileDeletedHandler)
  emitter.off('file_renamed', fileRenamedHandler)
})
</script>
<style lang="scss" scoped>
.v-on-error {
  position: absolute;
  top: 50%;
  left: 50%;
}

.loading {
  position: absolute;
  top: 50%;
  left: 50%;
  opacity: 0;
  animation: showDiv 0.5s ease-in-out 0.5s forwards;
}

.content {
  grid-area: content;
  position: relative;
  height: calc(100vh - 56px);

  /* Mobile layout adjustment */
  @media (max-width: 480px) {
    height: calc(100vh - 112px);
    /* Account for two-row header on mobile */
  }
}

.lightbox {
  background: var(--md-sys-color-surface);
  overflow: hidden;
}

.layout {
  display: grid;
  grid-template-areas:
    'toolbar info'
    'content info';
  grid-template-columns: 1fr auto;
  grid-template-rows: auto 1fr;
}

/* Mobile BottomSheet styles */
.lightbox :deep(.bottom-sheet-content) {
  padding-inline: 24px;
  padding-block: 0;
  max-height: 70vh;
  overflow-y: auto;
}

.lightbox :deep(.bottom-sheet-footer) {
  padding: 16px 24px 24px 24px;
  border-top: 1px solid var(--md-sys-color-outline-variant);
}

.v-img-wrapper {
  user-select: none;
  margin: 0;
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50% -50%);
  transition: 0.3s linear;
  will-change: transform opacity;

  img {
    user-select: none;
    user-select: none;
    max-width: 90vw;
    max-height: 90vh;
    display: block;
    position: relative;

    @media (max-width: 750px) {
      max-width: 95vw;
      max-height: 95vh;
    }
  }
}

.v-video-wrapper,
.v-audio-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  flex-direction: column;
  height: 100%;

  audio {
    width: 400px;
  }

  video {
    height: 95%;
    max-width: 88%;
  }
}

.btn-prev,
.btn-next {
  user-select: none;
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  cursor: pointer;
  opacity: 0.6;
  font-size: 4rem;
  transition: 0.15s linear;
  outline: none;
  z-index: 1;

  &:hover {
    opacity: 1;
  }
}

.btn-next {
  right: 12px;
}

.btn-prev {
  left: 12px;
}
</style>
