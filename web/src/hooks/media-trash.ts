import { initMutation, trashMediaItemsGQL, restoreMediaItemsGQL } from '@/lib/api/mutation'
import { DataType } from '@/lib/data'
import { isInNasTrash } from '@/lib/trash'
import emitter from '@/plugins/eventbus'
import type { FetchResult } from '@apollo/client'
import { reactive, computed, type Ref } from 'vue'
import type { ISource } from '@/components/lightbox/types'

export const useMediaTrash = () => {
  const { mutate, onDone: onTrashed } = initMutation({
    document: trashMediaItemsGQL,
  })

  const loading = reactive(new Map())

  onTrashed((r: FetchResult<any, Record<string, any>, Record<string, any>>) => {
    const { type, query } = r.data.trashMediaItems
    loading.delete(query)
    emitter.emit('refetch_tags', type)
    emitter.emit('media_items_actioned', { type, action: 'trash', query })
  })

  return {
    trashLoading(query: string) {
      return loading.get(query) ?? false
    },
    trash(type: DataType, query: string) {
      loading.set(query, true)
      mutate({ query, type })
    },
  }
}

export const useMediaRestore = () => {
  const { mutate, onDone: onRestored } = initMutation({
    document: restoreMediaItemsGQL,
  })

  const loading = reactive(new Map())

  onRestored((r: FetchResult<any, Record<string, any>, Record<string, any>>) => {
    const { type, query } = r.data.restoreMediaItems
    loading.delete(query)
    emitter.emit('refetch_tags', type)
    emitter.emit('media_items_actioned', { type, action: 'restore', query })
  })

  return {
    restoreLoading(query: string) {
      return loading.get(query) ?? false
    },
    restore(type: DataType, query: string) {
      loading.set(query, true)
      mutate({ query, type })
    },
  }
}

export function useFileTrashState(
  current: (() => ISource | undefined) | Ref<ISource | undefined>, 
) {
  const isTrashed = computed(() => {
    const currentValue = typeof current === 'function' ? current() : current.value
    return isInNasTrash(currentValue?.path)
  })

  return {
    isTrashed,
  }
}
