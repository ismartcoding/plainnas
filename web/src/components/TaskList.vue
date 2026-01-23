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
          <button
v-for="type in types" :key="type" :class="{ 'selected': filterType === type }"
            @click="chooseFilterType(type)">
            {{ getLabel(type) }}
          </button>
        </div>
      </div>
      <VirtualList
ref="listItemsRef" class="list-items" :data-key="'id'" :data-sources="visibleTasks"
        :estimate-size="80">
        <template #item="{ item }">
          <UploadBatchTaskItem v-if="item.kind === 'upload_batch'" :batch-id="item.batchId" :uploads="item.uploads" />
          <TaskItem v-else-if="item.kind === 'upload'" :item="item.upload" />
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
import UploadBatchTaskItem from '@/components/UploadBatchTaskItem.vue'
import FileTaskItem from '@/components/FileTaskItem.vue'
import { useTasksStore, type IFileTask } from '@/stores/tasks'
import type { IUploadItem } from '@/stores/temp'
import { useI18n } from 'vue-i18n'
import apollo from '@/plugins/apollo'
import { pathStatsGQL } from '@/lib/api/query'
import { promptModal } from '@/components/modal'
import FileConflictModal, { type ConflictChoice, type ConflictMode } from '@/components/files/FileConflictModal.vue'
import { deleteFilesGQL, initMutation } from '@/lib/api/mutation'

const tempStore = useTempStore()
const tasksStore = useTasksStore()
const store = useMainStore()
const { t } = useI18n()

async function getPathStats(paths: string[]): Promise<Map<string, { exists: boolean; isDir: boolean }>> {
  const m = new Map<string, { exists: boolean; isDir: boolean }>()
  const unique = Array.from(new Set(paths.filter((p) => !!p)))
  if (unique.length === 0) return m

  // Avoid sending extremely large payloads in a single request.
  const CHUNK = 500
  for (let i = 0; i < unique.length; i += CHUNK) {
    const slice = unique.slice(i, i + CHUNK)
    try {
      const r = await apollo.a.query({
        query: pathStatsGQL,
        variables: { paths: slice },
        fetchPolicy: 'network-only',
      })
      const rows = r?.data?.pathStats ?? []
      for (const row of rows) {
        m.set(String(row.path), { exists: !!row.exists, isDir: !!row.isDir })
      }
    } catch {
      // Best-effort: leave missing entries as non-existent.
    }
  }

  return m
}

async function promptConflict(mode: ConflictMode, details?: string): Promise<ConflictChoice | undefined> {
  return promptModal<ConflictChoice>(FileConflictModal, { mode, details })
}

const { mutate: deleteFilesMutate } = initMutation({ document: deleteFilesGQL })

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
  | { id: string; kind: 'upload_batch'; batchId: string; uploads: IUploadItem[] }
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

  const completedStates = new Set(['done', 'error', 'canceled'])
  const keyOf = (it: IUploadItem) => it.batchId || it.id

  const batchMap: Map<string, IUploadItem[]> = new Map()
  for (const it of tempStore.uploads) {
    const k = keyOf(it)
    const list = batchMap.get(k)
    if (list) list.push(it)
    else batchMap.set(k, [it])
  }

  const batchStatus = (items: IUploadItem[]) => {
    const statuses = items.map((u) => u.status)
    const completedStates = new Set(['done', 'canceled'])
    if (statuses.includes('error')) return 'error'
    if (statuses.includes('uploading')) return 'uploading'
    if (statuses.includes('saving')) return 'saving'
    if (statuses.includes('pending')) return 'pending'
    if (statuses.every((s) => s === 'paused')) return 'paused'
    if (statuses.length > 0 && statuses.every((s) => completedStates.has(s))) return 'done'
    return 'created'
  }

  const batchCreatedAt = (items: IUploadItem[]) => {
    let min = Number.POSITIVE_INFINITY
    for (const it of items) {
      const v = typeof it.createdAt === 'number' ? it.createdAt : 0
      if (v < min) min = v
    }
    return min === Number.POSITIVE_INFINITY ? 0 : min
  }

  const uploadItems: TaskListItem[] = Array.from(batchMap.entries())
    .filter(([_, items]) => items.some((it) => !completedStates.has(it.status)))
    .sort((a, b) => {
      const sa = sortKeys.get(batchStatus(a[1])) ?? 5
      const sb = sortKeys.get(batchStatus(b[1])) ?? 5
      if (sa !== sb) return sa - sb
      return batchCreatedAt(b[1]) - batchCreatedAt(a[1])
    })
    .map(([batchId, uploads]) => ({ id: batchId, kind: 'upload_batch', batchId, uploads }))

  const fileItems: TaskListItem[] = tasksStore.fileTasks
    .filter((it) => !['DONE', 'ERROR'].includes(it.status))
    .sort((a, b) => (Date.parse(b.updatedAt || '') || 0) - (Date.parse(a.updatedAt || '') || 0))
    .map((task) => ({ id: task.id, kind: 'file', task }))

  return fileItems.concat(uploadItems)
}

const completedTasks = (): TaskListItem[] => {
  const completedStates = new Set(['done', 'error', 'canceled'])
  const keyOf = (it: IUploadItem) => it.batchId || it.id

  const batchMap: Map<string, IUploadItem[]> = new Map()
  for (const it of tempStore.uploads) {
    const k = keyOf(it)
    const list = batchMap.get(k)
    if (list) list.push(it)
    else batchMap.set(k, [it])
  }

  const uploadItems: TaskListItem[] = Array.from(batchMap.entries())
    .filter(([_, items]) => items.length > 0 && items.every((it) => completedStates.has(it.status)))
    .map(([batchId, uploads]) => ({ id: batchId, kind: 'upload_batch', batchId, uploads }))

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
  return inProgressTasks().length + completedTasks().length
})

function getLabel(type: string) {
  const count = completedCount.value
  return t(type) + (type === 'completed' ? ` (${count})` : ` (${totalCount.value - count})`)
}

watch(
  () => tempStore.uploads,
  async (newUploads, _) => {
    store.quick = 'task'

    const created = newUploads.filter((item) => item.status === 'created')
    if (created.length === 0) return

    const keyOf = (it: IUploadItem) => it.batchId || it.id
    const batches: Map<string, IUploadItem[]> = new Map()
    for (const it of created) {
      const k = keyOf(it)
      const list = batches.get(k)
      if (list) list.push(it)
      else batches.set(k, [it])
    }

    const orderedBatches = Array.from(batches.entries()).sort((a, b) => {
      const ta = Math.min(...a[1].map((x) => x.createdAt || 0))
      const tb = Math.min(...b[1].map((x) => x.createdAt || 0))
      return ta - tb
    })

    for (const [_, newItems] of orderedBatches) {
      // 1) Folder -> Folder conflicts (only when we have relativePath with folders)
      const folderRoots: Map<string, { baseDir: string; folder: string }> = new Map()
      for (const item of newItems) {
        const rel = (item.relativePath || '').trim()
        const baseDir = (item.baseDir || item.dir || '').trim()
        if (!rel || !rel.includes('/')) continue
        const top = rel.split('/').filter(Boolean)[0]
        if (!top) continue
        const key = `${baseDir}::${top}`
        folderRoots.set(key, { baseDir, folder: top })
      }

      const folderTargets = Array.from(folderRoots.values()).map((v) => `${v.baseDir.replace(/\/+$/g, '')}/${v.folder}`.replace(/\/+?/g, '/'))
      const folderStats = await getPathStats(folderTargets)
      const folderConflicts: string[] = folderTargets.filter((p) => {
        const st = folderStats.get(p)
        return !!st?.exists && !!st?.isDir
      })

      if (folderConflicts.length > 0) {
        const details = folderConflicts.length === 1 ? folderConflicts[0] : `${folderConflicts.length} folders`
        const choice = await promptConflict('folder-folder', details)
        if (!choice) {
          newItems.forEach((it) => (it.status = 'canceled'))
          continue
        }

        if (choice === 'replace') {
          try {
            await deleteFilesMutate({ paths: folderConflicts })
          } catch {
            newItems.forEach((it) => (it.status = 'error'))
            continue
          }
        }
      }

      // 2) File -> File conflicts (check all new upload items)
      const uploadFilePaths = newItems.map((item) => (item.dir.endsWith('/') ? item.dir + item.file.name : item.dir + '/' + item.file.name).replace(/\/+?/g, '/'))
      const fileStats = await getPathStats(uploadFilePaths)
      const fileConflicts: IUploadItem[] = newItems.filter((item, idx) => {
        const p = uploadFilePaths[idx]
        const st = fileStats.get(p)
        return !!st?.exists && !st?.isDir
      })

      let replace = true
      if (fileConflicts.length > 0) {
        const mode: ConflictMode = newItems.length <= 1 ? 'file-file-single' : 'file-file-multiple'
        const details = fileConflicts.length === 1
          ? (fileConflicts[0].dir.endsWith('/') ? fileConflicts[0].dir + fileConflicts[0].file.name : fileConflicts[0].dir + '/' + fileConflicts[0].file.name)
          : `${fileConflicts.length} files`
        const choice = await promptConflict(mode, details)
        if (!choice) {
          newItems.forEach((it) => (it.status = 'canceled'))
          continue
        }

        if (choice === 'skip') {
          const conflictIds = new Set(fileConflicts.map((it) => it.id))
          newItems.forEach((it) => {
            if (conflictIds.has(it.id)) it.status = 'canceled'
          })
        } else if (choice === 'keep_both') {
          replace = false
        } else if (choice === 'replace') {
          replace = true
        }
      }

      // 3) Enqueue uploads
      for (const item of newItems) {
        if (item.status !== 'created') continue
        addUploadTask(item, replace)
        item.status = 'pending'
      }
    }
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
