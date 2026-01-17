<template>
  <router-view />
  <Teleport to="body">
    <modal-container />
  </Teleport>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import emitter from './plugins/eventbus'
import toast from '@/components/toaster'
import { getWebSocketUrl } from './lib/api/api'
import { chachaDecrypt, chachaEncrypt, bitArrayToUint8Array } from './lib/api/crypto'
import { parseWebSocketData } from './lib/api/sjcl-arraybuffer'
import { applyDarkClass, changeColor, changeColorMode, getCurrentMode, getLastSavedAutoColorMode, isModeDark } from './lib/theme'
import { tokenToKey } from './lib/api/file'
import { useTasksStore } from '@/stores/tasks'
import { initQuery, getTasksGQL } from '@/lib/api/query'
let retryConnectTimeout = 0
const { t } = useI18n()
document.title = t('app_name')

let ws: WebSocket
let retryTime = 1000 // 1s

const EventType: { [key: number]: string } = {
  4: 'media_scan_progress',
  6: 'file_task_progress',
}

async function connect() {
  const clientId = localStorage.getItem('client_id')
  const token = localStorage.getItem('auth_token') ?? ''
  if (!token) {
    return
  }

  try {
    const key = tokenToKey(token)

    ws = new WebSocket(`${getWebSocketUrl()}?cid=${clientId}`)
    ws.onopen = async () => {
      emitter.emit('app_socket_connection_changed', true)
      console.log('WebSocket is connecting to app')
      retryTime = 1000 // reset retry time
      const enc = chachaEncrypt(key, new Date().getTime().toString())
      ws.send(bitArrayToUint8Array(enc))
    }

    ws.onmessage = async (event: MessageEvent) => {
      const buffer = await event.data.arrayBuffer()
      const plainTypes = [5]
      const r = parseWebSocketData(buffer, plainTypes)
      const type = EventType[r.type] ?? ''
      if (plainTypes.includes(r.type)) {
        if (type) {
          emitter.emit(type as any, new Blob([r.data], { type: 'application/octet-stream' }))
        }
        console.log(type)
      } else {
        try {
          const json = chachaDecrypt(key, r.data)
          if (type) {
            emitter.emit(type as any, json ? JSON.parse(json) : null)
          }
          console.log(`${type}, ${json}`)
        } catch (ex) {
          console.error(ex)
        }
      }
    }

    ws.onclose = (event: CloseEvent) => {
      console.error(event)
      retryConnect()
    }

    ws.onerror = (event: Event) => {
      console.error(event)
      ws.close()
      emitter.emit('app_socket_connection_changed', false)
    }
  } catch (ex) {
    console.error(ex)
    retryConnect()
  }
}

function retryConnect() {
  if (retryConnectTimeout) {
    clearTimeout(retryConnectTimeout)
  }
  retryConnectTimeout = setTimeout(
    () => {
      connect()
    },
    Math.min(5000, retryTime) // wait at most 5s
  )
  retryTime += 1000
}

function determinePageNavigationAutoMode() {
  if (getCurrentMode() !== 'auto') {
    return
  }

  const actualColorMode = isModeDark('auto', false) ? 'dark' : 'light'
  const lastSavedAutoColorMode = getLastSavedAutoColorMode()

  if (actualColorMode !== lastSavedAutoColorMode) {
    changeColorMode('auto')
  }
}

function initializeTheme() {
  applyDarkClass(isModeDark(getCurrentMode() || 'auto', false))
}

onMounted(() => {
  const tasksStore = useTasksStore()

  initQuery({
    document: getTasksGQL,
    handle: (data: any, error: string) => {
      if (error) return
      tasksStore.setFileTasks(data?.getTasks ?? [])
    },
  })

  emitter.on('toast', (r: string) => {
    toast(t(r), 'error')
  })

  emitter.on('file_task_progress', (payload: any) => {
    tasksStore.handleFileTaskProgress(payload)
  })

  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (getCurrentMode() !== 'auto') {
      return
    }

    changeColor()
  })

  try {
    initializeTheme()
    determinePageNavigationAutoMode()
  } catch (ex) {
    console.error(ex)
  }

  connect()
})
</script>