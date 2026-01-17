import { defineStore } from 'pinia'
import { useMainStore } from '@/stores/main'

export type FileTaskStatus = 'QUEUED' | 'RUNNING' | 'DONE' | 'ERROR'
export type FileTaskType = 'COPY' | 'MOVE'

export interface IFileTask {
  id: string
  type: FileTaskType
  title: string
  status: FileTaskStatus
  error: string
  totalBytes: number
  doneBytes: number
  totalItems: number
  doneItems: number
  createdAt: string
  updatedAt: string
}

type TasksState = {
  fileTasks: IFileTask[]
}

export const useTasksStore = defineStore('tasks', {
  state: (): TasksState => ({
    fileTasks: [],
  }),
  actions: {
    setFileTasks(tasks: IFileTask[]) {
      // newest first
      this.fileTasks = [...tasks].sort((a, b) => {
        const ta = Date.parse(a.updatedAt || '') || 0
        const tb = Date.parse(b.updatedAt || '') || 0
        return tb - ta
      })
    },

    upsertFileTask(task: IFileTask) {
      const idx = this.fileTasks.findIndex((t) => t.id === task.id)
      if (idx >= 0) {
        const merged = { ...this.fileTasks[idx], ...task }
        this.fileTasks.splice(idx, 1)
        this.fileTasks.unshift(merged)
        return
      }

      this.fileTasks.unshift(task)
      // Open TaskList when a new task is created
      const main = useMainStore()
      main.quick = 'task'
    },

    removeFileTask(id: string) {
      const idx = this.fileTasks.findIndex((t) => t.id === id)
      if (idx >= 0) {
        this.fileTasks.splice(idx, 1)
      }
    },

    handleFileTaskProgress(payload: any) {
      if (!payload || typeof payload !== 'object') return
      // Payload is a flat task snapshot from websocket
      this.upsertFileTask(payload as IFileTask)
    },
  },
})
