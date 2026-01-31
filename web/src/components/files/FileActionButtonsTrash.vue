<template>
  <div class="actions">
    <v-icon-button v-tooltip="$t('delete')" class="sm" @click.stop="deleteItem(item)">
      <i-material-symbols:delete-forever-outline-rounded />
    </v-icon-button>
    <v-icon-button v-tooltip="$t('restore')" class="sm" @click.stop="restoreItem(item)">
      <i-material-symbols:restore-from-trash-outline-rounded />
    </v-icon-button>
    <v-icon-button v-tooltip="$t('download')" class="sm" @click.stop="downloadItem(item)">
        <i-material-symbols:download-rounded />
    </v-icon-button>
    <v-icon-button v-tooltip="$t('info')" class="sm" @click.stop="openInfo(item)">
        <i-material-symbols:info-outline-rounded />
    </v-icon-button>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { IFile } from '@/lib/file'
import { openModal } from '@/components/modal'
import FileInfoModal from '@/components/files/FileInfoModal.vue'

interface Props {
  item: IFile
}

const props = defineProps<Props>()

const emit = defineEmits<{
  downloadDir: [path: string]
  downloadFile: [path: string]
  deleteItem: [item: IFile]
  restoreItem: [item: IFile]
  copyLink: [item: IFile]
}>()

const infoMenuVisible = ref(false)
const actionsMenuVisible = ref(false)

function downloadDir(path: string) {
  emit('downloadDir', path)
}

function downloadFile(path: string) {
  emit('downloadFile', path)
}

function deleteItem(item: IFile) {
  emit('deleteItem', item)
}

function restoreItem(item: IFile) {
  emit('restoreItem', item)
}

function downloadItem(item: IFile) {
  if (item.isDir) {
    downloadDir(item.path)
  } else {
    downloadFile(item.path)
  }
}

function openInfo(item: IFile) {
  openModal(FileInfoModal, { item })
}

</script>

<style scoped lang="scss">
.actions {
  display: flex;
  gap: 4px;
  align-items: center;
  
  &.mobile {
    flex-wrap: wrap;
  }
}
</style> 