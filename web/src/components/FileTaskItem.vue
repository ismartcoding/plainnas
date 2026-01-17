<template>
  <div class="item task-item" :class="`item-${statusKey}`">
    <div class="title">{{ item.title }}</div>
    <div class="subtitle">
      <span class="status" :class="`status-${statusKey}`">
        {{ $t(`upload_status.${statusKey}`) }}
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

const props = defineProps<{ item: IFileTask }>()
const tasksStore = useTasksStore()

const statusKey = computed(() => {
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
.subtitle {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 4px;
  font-size: 0.875rem;
  color: var(--md-sys-color-on-surface-variant);
}

.task-actions {
  margin-left: auto;
  display: flex;
  gap: 4px;
}

.remove-btn {
  color: var(--md-sys-color-error);
}

.progress-info {
  margin-top: 8px;
}

.progress-text {
  font-size: 0.75rem;
  color: var(--md-sys-color-on-surface-variant);
  margin-bottom: 4px;
}

.progress-bar {
  height: 4px;
  background: var(--md-sys-color-surface-variant);
  border-radius: 2px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: var(--md-sys-color-primary);
  transition: width 0.3s ease;
}

.error-message {
  margin-top: 8px;
  font-size: 0.75rem;
  color: var(--md-sys-color-error);
  padding: 8px;
  background: var(--md-sys-color-error-container);
  border-radius: 8px;
}
</style>
