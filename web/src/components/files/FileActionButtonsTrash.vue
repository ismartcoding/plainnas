<template>
  <div class="actions">
    <v-icon-button v-tooltip="$t('delete')" class="sm" @click.stop="deleteItem(item)">
      <i-material-symbols:delete-forever-outline-rounded />
    </v-icon-button>
    <v-icon-button v-tooltip="$t('restore')" class="sm" @click.stop="restoreItem(item)">
      <i-material-symbols:restore-from-trash-outline-rounded />
    </v-icon-button>
    <template v-if="item.isDir">
      <v-icon-button v-tooltip="$t('download')" class="sm" @click.stop="downloadDir(item.path)">
          <i-material-symbols:download-rounded />
      </v-icon-button>
    </template>
    <template v-else>
      <v-icon-button v-tooltip="$t('download')" class="sm" @click.stop="downloadFile(item.path)">
          <i-material-symbols:download-rounded />
      </v-icon-button>
    </template>

    <v-dropdown v-model="infoMenuVisible">
      <template #trigger>
        <v-icon-button v-tooltip="$t('info')" class="sm">
            <i-material-symbols:info-outline-rounded />
        </v-icon-button>
      </template>
      <section class="card card-info">
        <div class="key-value vertical">
          <div class="key">{{ $t('path') }}</div>
          <div class="value">
            {{ item.path }}
          </div>
        </div>
      </section>
    </v-dropdown>

    <v-dropdown v-model="actionsMenuVisible">
      <template #trigger>
        <v-icon-button v-tooltip="$t('actions')" class="sm">
            <i-material-symbols:more-vert />
        </v-icon-button>
      </template>
      <div v-if="!item.isDir" class="dropdown-item" @click.stop="copyLink(item); actionsMenuVisible = false">
        {{ $t('copy_link') }}
      </div>
    </v-dropdown>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { IFile } from '@/lib/file'

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

function copyLink(item: IFile) {
  emit('copyLink', item)
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