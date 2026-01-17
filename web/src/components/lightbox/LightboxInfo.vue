<template>
  <section class="info">
    <div class="info-header">
      <div class="info-title">
        <span>{{ $t('info') }}</span>
        <lightbox-keyboard-shortcuts class="info-keyboard-shortcuts" />
      </div>
      <div class="info-actions">
        <LightboxFileActionButtons
:current="current" :download-file="downloadFile" @rename-file="$emit('rename-file')"
          @delete-file="$emit('delete-file')" />
      </div>
    </div>
    <div class="info-content">
      <LightboxFileDetails :current="current" :file-info="fileInfo" :data-dir="dataDir" />

      <LightboxFileTags :current="current" :file-info="fileInfo" @add-to-tags="$emit('add-to-tags')" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { DataType } from '@/lib/data'
import { initMutation, trashFilesGQL } from '@/lib/api/mutation'
import { useMediaTrash, useFileTrashState } from '@/hooks/media-trash'
import emitter from '@/plugins/eventbus'
import type { ITag } from '@/lib/interfaces'
import type { ISource } from './types'

const props = defineProps({
  current: {
    type: Object as () => ISource | undefined,
    required: true,
  },
  fileInfo: {
    type: Object,
    default: null,
  },
  urlTokenKey: {
    type: String,
    required: true,
  },
  dataDir: {
    type: String,
    default: '',
  },
  tagsMap: {
    type: Object as () => Map<string, ITag[]>,
    required: true,
  },
  downloadFile: {
    type: Function,
    required: true,
  },
})

const emit = defineEmits(['rename-file', 'delete-file', 'add-to-tags', 'refetch-info'])

const { isTrashed } = useFileTrashState(() => props.current)
const { trash } = useMediaTrash()
const { mutate: trashFilesMutation, onDone: onFilesTrashed } = initMutation({
  document: trashFilesGQL,
})

onFilesTrashed(() => {
  const path = props.current?.path
  if (!path) return
  emitter.emit('media_items_actioned', { type: 'FILE', action: 'trash', query: `path:${path}` })
  emitter.emit('file_trashed', { paths: [path] })
})

function handleKeyDown(event: KeyboardEvent) {
  if (event.key === 'Delete' || ((event.ctrlKey || event.metaKey) && event.key === 'Backspace')) {
    event.preventDefault()

    if (isTrashed.value) {
      // Already in trash, permanently delete
      emit('delete-file')
      return
    }

    // Not in trash, move to trash (media items by id/type; file previews by path)
    if (props.current?.data?.id && props.current.type) {
      trash(props.current.type as DataType, `ids:${props.current.data.id}`)
      return
    }
    if (props.current?.path) {
      trashFilesMutation({ paths: [props.current.path] })
      return
    }
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeyDown)
})
</script>

<style lang="scss" scoped>
.info {
  grid-area: info;
  width: 350px;
  height: 100vh;
  box-sizing: border-box;
  background: var(--md-sys-color-surface-container);
  z-index: 1;
  display: flex;
  flex-direction: column;
}

.info-header {
  padding: 8px 16px;
}

.info-title {
  margin: 0 0 16px 0;
  display: flex;
  align-items: center;
  gap: 16px;

  span {
    font-size: 1.2rem;
    font-weight: bold;
  }
}

.info-content {
  padding: 16px;
  flex: 1;
  overflow-y: auto;
}

.info-keyboard-shortcuts {
  margin-inline-start: auto;
}
</style>
