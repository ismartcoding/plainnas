<template>
  <div class="quick-content-main">
    <div class="top-app-bar">
      <button v-tooltip="$t('close')" class="btn-icon" @click="store.quick = ''">
        <i-lucide:x />
      </button>
      <div class="title">{{ $t('header_actions.tasks') }}</div>
    </div>

    <div class="quick-content-body">
      <div class="filter-bar">
        <div class="button-group">
          <button v-for="type in types" :key="type" :class="{ 'selected': filterType === type }"
            @click="chooseFilterType(type)">
            {{ getLabel(type) }}
          </button>
        </div>
      </div>
      <VirtualList ref="listItemsRef" class="list-items" :data-key="'id'" :data-sources="visibleTasks"
        :estimate-size="80">
        <template #item="{ item }">
          <TaskItem v-if="item.kind === 'upload'" :item="item.upload" />
          <FileTaskItem v-else :item="item.task" />
        </template>
      </VirtualList>

      <div v-if="!visibleTasks.length" class="no-data">
        <div class="empty-content">
          <div class="empty-text">{{ $t('no_task') }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { addUploadTask } from '@/lib/upload/upload-queue'
import { useTempStore } from '@/stores/temp'
import { computed, ref, watch } from 'vue'
import { useMainStore } from '@/stores/main'
import { sortBy } from 'lodash-es'
import VirtualList from '@/components/virtualscroll'
import TaskItem from '@/components/TaskItem.vue'
import FileTaskItem from '@/components/FileTaskItem.vue'
import { useTasksStore, type IFileTask } from '@/stores/tasks'
import type { IUploadItem } from '@/stores/temp'
import { useI18n } from 'vue-i18n'

const tempStore = useTempStore()
const tasksStore = useTasksStore()
const store = useMainStore()
const { t } = useI18n()

const filterType = ref('in_progress')
const types = ['in_progress', 'completed']
const listItemsRef = ref()

function chooseFilterType(value: string) {
  filterType.value = value
  const scroller = listItemsRef.value
  if (scroller) {
    scroller.scrollTop = 0
  }
}

type TaskListItem =
  | { id: string; kind: 'upload'; upload: IUploadItem }
  | { id: string; kind: 'file'; task: IFileTask }

const visibleTasks = computed<TaskListItem[]>(() => {
  return filterType.value === 'in_progress' ? inProgressTasks() : completedTasks()
})

const inProgressTasks = (): TaskListItem[] => {
  const sortKeys: Map<string, number> = new Map()
  sortKeys.set('uploading', 0)
  sortKeys.set('saving', 1)
  sortKeys.set('pending', 2)
  sortKeys.set('paused', 3)
  sortKeys.set('created', 4)

  const uploadItems: TaskListItem[] = sortBy(
    tempStore.uploads.filter((it) => !['error', 'done'].includes(it.status)),
    (it) => sortKeys.get(it.status) ?? 5
  ).map((upload) => ({ id: upload.id, kind: 'upload', upload }))

  const fileItems: TaskListItem[] = tasksStore.fileTasks
    .filter((it) => !['DONE', 'ERROR'].includes(it.status))
    .sort((a, b) => (Date.parse(b.updatedAt || '') || 0) - (Date.parse(a.updatedAt || '') || 0))
    .map((task) => ({ id: task.id, kind: 'file', task }))

  return fileItems.concat(uploadItems)
}

const completedTasks = (): TaskListItem[] => {
  const uploadItems: TaskListItem[] = tempStore.uploads
    .filter((it) => ['error', 'done'].includes(it.status))
    .map((upload) => ({ id: upload.id, kind: 'upload', upload }))

  const fileItems: TaskListItem[] = tasksStore.fileTasks
    .filter((it) => ['DONE', 'ERROR'].includes(it.status))
    .sort((a, b) => (Date.parse(b.updatedAt || '') || 0) - (Date.parse(a.updatedAt || '') || 0))
    .map((task) => ({ id: task.id, kind: 'file', task }))

  return fileItems.concat(uploadItems)
}

const completedCount = computed(() => {
  return completedTasks().length
})

const totalCount = computed(() => {
  return tempStore.uploads.length + tasksStore.fileTasks.length
})

function getLabel(type: string) {
  const count = completedCount.value
  return t(type) + (type === 'completed' ? ` (${count})` : ` (${totalCount.value - count})`)
}

watch(
  () => tempStore.uploads,
  (newUploads, _) => {
    store.quick = 'task'
    const newItems = newUploads.filter((item) => item.status === 'created')
    newItems.forEach((item) => {
      addUploadTask(item, true)
      item.status = 'pending'
    })
  }
)
</script>

<style scoped lang="scss">
.filter-bar {
  padding: 8px 16px;

  .button-group {
    width: 100%;
  }
}

.list-items {
  padding-block: 8px;
  overflow-y: auto;
  overflow-x: hidden;
  height: calc(100vh - 100px);
}

.empty-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  color: var(--md-sys-color-on-surface-variant);
}

.empty-text {
  font-size: 1rem;
  opacity: 0.7;
}
</style>
