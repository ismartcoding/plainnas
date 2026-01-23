<template>
  <div class="item task-item" :class="`item-${statusStyleKey}`">
    <div class="title">{{ item.title }}</div>
    <div class="subtitle">
      <span class="status" :class="`status-${statusStyleKey}`">
        {{ statusLabel }}
      </span>
      <span v-if="item.totalBytes > 0" class="size">{{ formatFileSize(item.totalBytes) }}</span>

      <div class="icon task-actions">
        <button v-tooltip="$t('remove')" class="btn-icon remove-btn" @click="remove">
          <i-material-symbols:close-rounded />
        </button>
      </div>
    </div>

    <div v-if="showProgress || item.error" class="body">
      <div v-if="showProgress" class="progress-info">
        <div class="progress-text">
          {{ formatFileSize(item.doneBytes) }} / {{ formatFileSize(item.totalBytes) }}
          <span v-if="item.totalItems > 0"> ({{ item.doneItems }}/{{ item.totalItems }})</span>
        </div>
        <div class="progress-bar">
          <div class="progress-fill" :style="{ width: progressPercent + '%' }"></div>
        </div>
      </div>

      <div v-if="item.error" class="error-message">
        {{ item.error }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { formatFileSize } from '@/lib/format'
import { useTasksStore, type IFileTask } from '@/stores/tasks'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ item: IFileTask }>()
const tasksStore = useTasksStore()
const { t } = useI18n()

const statusStyleKey = computed(() => {
  switch (props.item.status) {
    case 'QUEUED':
      return 'pending'
    case 'RUNNING':
      return 'uploading'
    case 'DONE':
      return 'done'
    case 'ERROR':
      return 'error'
    default:
      return 'pending'
  }
})

const statusLabel = computed(() => {
  switch (props.item.status) {
    case 'QUEUED':
      return t('pending')
    case 'RUNNING':
      return t('running')
    case 'DONE':
      return t('completed')
    case 'ERROR':
      return t('upload_status.error')
    default:
      return t('pending')
  }
})

const showProgress = computed(() => {
  return ['QUEUED', 'RUNNING'].includes(props.item.status) && props.item.totalBytes > 0
})

const progressPercent = computed(() => {
  if (!props.item.totalBytes || props.item.totalBytes <= 0) return 0
  return Math.max(0, Math.min(100, Math.round((props.item.doneBytes / props.item.totalBytes) * 100)))
})

function remove() {
  tasksStore.removeFileTask(props.item.id)
}
</script>

<style scoped lang="scss">
@use '@/styles/task-item.scss' as *;
</style>
