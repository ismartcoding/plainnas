<template>
  <section
    v-if="!isPhone"
    class="file-item selectable-card"
    :class="{ selected: selectedIds.includes(item.id), selecting: shiftEffectingIds.includes(item.id) }"
    @click.stop="handleItemClick($event, item, index, () => clickItem(item))"
    @mouseenter.stop="handleMouseOver($event, index)"
  >
    <div class="start">
      <v-checkbox v-if="shiftEffectingIds.includes(item.id)" class="checkbox" touch-target="wrapper" :checked="shouldSelect" @click.stop="toggleSelect($event, item, index)" />
      <v-checkbox v-else class="checkbox" touch-target="wrapper" :checked="selectedIds.includes(item.id)" @click.stop="toggleSelect($event, item, index)" />
      <span class="number"><field-id :id="index + 1" :raw="item" /></span>
    </div>
    
    <div class="image" @click="viewItem($event, item)">
      <img v-if="item.isDir" :src="`/ficons/folder.svg`" class="svg" />
      <template v-else>
        <img v-if="extensionImageErrorIds.includes(item.id)" class="svg" src="/ficons/default.svg" />
        <img v-else-if="!imageErrorIds.includes(item.id) && item.fileId" class="image-thumb" :src="getFileUrl(item.fileId, '&w=50&h=50')" @error="onImageError(item.id)" />
        <img v-else-if="item.extension" :src="`/ficons/${item.extension}.svg`" class="svg" @error="onExtensionImageError(item.id)" />
        <img v-else class="svg" src="/ficons/default.svg" />
      </template>
    </div>
    
    <div class="title">
      {{ displayName }}
    </div>
    
    <div class="subtitle">
      <span v-if="item.isDir">{{ $t('x_items', item.childCount || 0) }}</span>
      <span v-else>{{ formatFileSize(item.size) }}</span>
      <span v-tooltip="formatDateTime(item.updatedAt)">{{ formatTimeAgo(item.updatedAt) }}</span>
    </div>
    
    <FileActionButtonsTrash
          :item="item"
          @download-dir="downloadDir"
          @download-file="downloadFile"
          @delete-item="deleteItem"
          @restore-item="restoreItem"
          @copy-link="copyLink"
        />
  </section>

  <!-- Phone Layout -->
  <ListItemPhone
    v-else
    :is-selected="selectedIds.includes(item.id)"
    :is-selecting="shiftEffectingIds.includes(item.id)"
    :checkbox-checked="shiftEffectingIds.includes(item.id) ? shouldSelect : selectedIds.includes(item.id)"
    @click="handleItemClick($event, item, index, () => clickItem(item))"
    @mouseenter.stop="handleMouseOver($event, index)"
    @checkbox-click="(event: MouseEvent) => toggleSelect(event, item, index)"
  >
    <template #image>
      <div class="image" @click="viewItem($event, item)">
        <img v-if="item.isDir" :src="`/ficons/folder.svg`" class="svg" />
        <template v-else>
          <img v-if="extensionImageErrorIds.includes(item.id)" class="svg" src="/ficons/default.svg" />
          <img v-else-if="!imageErrorIds.includes(item.id) && item.fileId" class="image-thumb" :src="getFileUrl(item.fileId, '&w=50&h=50')" @error="onImageError(item.id)" />
          <img v-else-if="item.extension" :src="`/ficons/${item.extension}.svg`" class="svg" @error="onExtensionImageError(item.id)" />
          <img v-else class="svg" src="/ficons/default.svg" />
        </template>
      </div>
    </template>
    
    <template #title>{{ displayName }}</template>
    
    <template #subtitle>
      <span v-if="item.isDir">{{ $t('x_items', item.childCount || 0) }}</span>
      <span v-else>{{ formatFileSize(item.size) }}</span>
      <span v-tooltip="formatDateTime(item.updatedAt)">{{ formatTimeAgo(item.updatedAt) }}</span>
    </template>
    
    <template #actions>
      <FileActionButtons
        :item="item"
        @download-dir="downloadDir"
        @download-file="downloadFile"
        @delete-item="deleteItem"
        @restore-item="restoreItem"
        @copy-link="copyLink"
      />
    </template>
  </ListItemPhone>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { IFile } from '@/lib/file'
import { getTrashDisplayName } from '@/lib/trash'

// Extend IFile to include childCount property for directories
interface IFileWithChildCount extends IFile {
  childCount?: number
}
import { formatFileSize, formatDateTime, formatTimeAgo } from '@/lib/format'
import { getFileUrl } from '@/lib/api/file'

interface Props {
  item: IFileWithChildCount
  index: number
  selectedIds: string[]
  shiftEffectingIds: string[]
  shouldSelect: boolean
  isPhone: boolean
  imageErrorIds: string[]
  extensionImageErrorIds: string[]
  // Functions passed from parent
  handleItemClick: (event: MouseEvent, item: IFile, index: number, callback: () => void) => void
  handleMouseOver: (event: MouseEvent, index: number) => void
  toggleSelect: (event: MouseEvent, item: IFile, index: number) => void
  onImageError: (id: string) => void
  onExtensionImageError: (id: string) => void
  viewItem: (event: Event, item: IFile) => void
  clickItem: (item: IFile) => void
}

const props = defineProps<Props>()

const displayName = computed(() => {
  return getTrashDisplayName(props.item.name)
})

const emit = defineEmits<{
  downloadDir: [path: string]
  downloadFile: [path: string]
  deleteItem: [item: IFile]
  restoreItem: [item: IFile]
  copyLink: [item: IFile]
}>()

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

function copyLink(item: IFile) {
  emit('copyLink', item)
}

</script>

<style scoped lang="scss">
.list-item-phone {
  margin-block-end: 8px;
}
</style> 