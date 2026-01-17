<template>
  <div class="file-action-buttons">
    <template v-if="isTrashed">
      <v-outlined-button @click.stop="$emit('delete-file')">
        <i-material-symbols:delete-forever-outline-rounded />
        {{ $t('delete') }}
      </v-outlined-button>
      <v-outlined-button :class="{ loading: isRestoreLoading }" @click.stop="restoreItem">
        <i-material-symbols:restore-from-trash-outline-rounded />
        {{ $t('restore') }}
      </v-outlined-button>
    </template>
    <template v-else>
      <v-outlined-button :class="{ loading: isTrashLoading }" @click.stop="trashItem">
        <i-material-symbols:delete-outline-rounded />
        {{ $t('move_to_trash') }}
      </v-outlined-button>
      <v-outlined-button @click.stop="$emit('rename-file')">
        <i-material-symbols:edit-outline-rounded />
        {{ $t('rename') }}
      </v-outlined-button>
    </template>
    <v-outlined-button class="download-btn" @click.stop="handleDownload">
      <i-material-symbols:download-rounded />
      {{ $t('download') }}
    </v-outlined-button>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive, computed } from 'vue'
import { getFileName } from '@/lib/api/file'
import { DataType } from '@/lib/data'
import { initMutation, trashFilesGQL, restoreFilesGQL } from '@/lib/api/mutation'
import { useMediaRestore, useMediaTrash, useFileTrashState } from '@/hooks/media-trash'
import emitter from '@/plugins/eventbus'
import type { IMediaItemsActionedEvent } from '@/lib/interfaces'
import type { ISource } from './types'

const props = defineProps({
  current: {
    type: Object as () => ISource | undefined,
    required: true,
  },
  downloadFile: {
    type: Function,
    required: true,
  },
})

const emit = defineEmits(['rename-file', 'delete-file', 'action-success'])

const { isTrashed } = useFileTrashState(() => props.current)

const isMediaItem = computed(() => {
  return Boolean(props.current?.data?.id && props.current?.type)
})

const trashKey = computed(() => {
  if (isMediaItem.value) return `ids:${props.current?.data?.id}`
  return props.current?.path || ''
})

function handleDownload() {
  if (props.current?.path) {
    props.downloadFile(props.current.path, getFileName(props.current.path).replace(' ', '-'))
  }
}

const { trash, trashLoading } = useMediaTrash()
const { restore, restoreLoading } = useMediaRestore()

const fileTrashLoading = reactive(new Map<string, boolean>())
const fileRestoreLoading = reactive(new Map<string, boolean>())

const { mutate: trashFilesMutation, onDone: onFilesTrashed } = initMutation({
  document: trashFilesGQL,
})
const { mutate: restoreFilesMutation, onDone: onFilesRestored } = initMutation({
  document: restoreFilesGQL,
})

function emitFilePathAction(action: 'trash' | 'restore') {
  const path = props.current?.path
  if (!path) return
  emitter.emit('media_items_actioned', { type: 'FILE', action, query: `path:${path}` })
  if (action === 'trash') {
    emitter.emit('file_trashed', { paths: [path] })
  } else {
    emitter.emit('file_restored', { paths: [path] })
  }
}

onFilesTrashed(() => {
  const key = trashKey.value
  if (key) fileTrashLoading.delete(key)
  if (!isMediaItem.value) {
    emitFilePathAction('trash')
  }
  emit('action-success', 'trash')
})

onFilesRestored(() => {
  const key = trashKey.value
  if (key) fileRestoreLoading.delete(key)
  if (!isMediaItem.value) {
    emitFilePathAction('restore')
  }
  emit('action-success', 'restore')
})

const isTrashLoading = computed(() => {
  const key = trashKey.value
  if (!key) return false
  if (isMediaItem.value) return trashLoading(key)
  return fileTrashLoading.get(key) ?? false
})

const isRestoreLoading = computed(() => {
  const key = trashKey.value
  if (!key) return false
  if (isMediaItem.value) return restoreLoading(key)
  return fileRestoreLoading.get(key) ?? false
})

function trashItem() {
  if (isMediaItem.value) {
    trash(props.current!.type as DataType, `ids:${props.current!.data!.id}`)
    return
  }
  const path = props.current?.path
  if (!path) return
  fileTrashLoading.set(path, true)
  trashFilesMutation({ paths: [path] })
}

function restoreItem() {
  if (isMediaItem.value) {
    restore(props.current!.type as DataType, `ids:${props.current!.data!.id}`)
    return
  }
  const path = props.current?.path
  if (!path) return
  fileRestoreLoading.set(path, true)
  restoreFilesMutation({ paths: [path] })
}

// Listen for media action events and emit action-success
const mediaItemsActionedHandler = (event: IMediaItemsActionedEvent) => {
  if (!isMediaItem.value) return
  const currentQuery = `ids:${props.current?.data?.id}`
  if (event.query === currentQuery && (event.action === 'trash' || event.action === 'restore')) {
    emit('action-success', event.action)
  }
}

onMounted(() => {
  emitter.on('media_items_actioned', mediaItemsActionedHandler)
})

onUnmounted(() => {
  emitter.off('media_items_actioned', mediaItemsActionedHandler)
})
</script>

<style lang="scss" scoped>
.file-action-buttons {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-template-rows: auto auto;
  gap: 8px;
}

.download-btn {
  grid-column: 1 / span 2;

  &:first-child {
    grid-column: 1 / span 2;
  }
}
</style>