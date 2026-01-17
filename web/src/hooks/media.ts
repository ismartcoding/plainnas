import DeleteItemsConfirm from '@/components/DeleteItemsConfirm.vue'
import { openModal } from '@/components/modal'
import { useI18n } from 'vue-i18n'
import toast from '@/components/toaster'
import { deleteMediaItemsGQL } from '@/lib/api/mutation'
import emitter from '@/plugins/eventbus'
import DeleteConfirm from '@/components/DeleteConfirm.vue'
import type { DataType } from '@/lib/data'
import { encodeBase64 } from '@/lib/strutil'
import type { MainState } from '@/stores/main'
import { buildQuery } from '@/lib/search'
import { replacePath } from '@/plugins/router'
import type { IAudio, IBucket, IImageItem, ITag, IVideoItem } from '@/lib/interfaces'
import { ref, type Ref } from 'vue'
import { bucketsTagsGQL, initLazyQuery } from '@/lib/api/query'
import { initMutation } from '@/lib/api/mutation'
import { mediaSourceDirsGQL } from '@/lib/api/query'
import { setMediaSourceDirsGQL } from '@/lib/api/mutation'

type BucketsTagsResult = {
  tags: Ref<ITag[]>
  buckets: Ref<IBucket[]>
  fetch: () => void
}

type MediaSourceDirsResult = {
  sourceDirs: Ref<string[]>
  loading: Ref<boolean>
  fetch: () => void
  saving: Ref<boolean>
  save: (dirs: string[]) => Promise<void>
}

const bucketsTagsCache = new Map<string, BucketsTagsResult>()
let mediaSourceDirsCache: MediaSourceDirsResult | null = null

export const useDeleteItems = () => {
  const { t } = useI18n()
  const typeNameMap = new Map<string, string>()
  typeNameMap.set('AUDIO', 'Audio')
  typeNameMap.set('VIDEO', 'Video')
  typeNameMap.set('IMAGE', 'Image')

  return {
    deleteItems: (type: string, ids: string[], realAllChecked: boolean, total: number, query: string) => {
      let q = query
      if (!realAllChecked) {
        if (ids.length === 0) {
          toast(t('select_first'), 'error')
          return
        }
        q = `ids:${ids.join(',')}`
      }

      openModal(DeleteItemsConfirm, {
        gql: deleteMediaItemsGQL,
        count: realAllChecked ? total : ids.length,
        variables: () => ({ type: type, query: q }),
        done: () => {
          emitter.emit('media_items_actioned', { type: type, action: 'delete', query: q })
        },
      })
    },

    deleteItem: (type: string, item: IImageItem | IVideoItem | IAudio) => {
      openModal(DeleteConfirm, {
        id: item.id,
        name: item.title,
        image: isIAudio(item) ? '' : item.fileId,
        gql: deleteMediaItemsGQL,
        variables: () => ({ type: type, query: `ids:${item.id}` }),
        typeName: typeNameMap.get(type) ?? '',
        done: () => {
          emitter.emit('media_items_actioned', { type: type, action: 'delete', id: item.id, query: `ids:${item.id}` })
        },
      })
    },
  }
}

function isIAudio(object: any): object is IAudio {
  return 'albumFileId' in object
}

export const useBuckets = (type: DataType) => {
  const path = {
    AUDIO: 'audios',
    IMAGE: 'images',
    VIDEO: 'videos',
  }[type]
  return {
    view(mainStore: MainState, id: string) {
      const q = buildQuery([
        {
          name: 'bucket_id',
          op: '',
          value: id,
        },
      ])
      replacePath(mainStore, `/${path}?q=${encodeBase64(q)}`)
    },
  }
}

export const useBucketsTags = (type: DataType) => {
  const key = String(type)
  const cached = bucketsTagsCache.get(key)
  if (cached) {
    return cached
  }

  const tags = ref<ITag[]>([])
  const buckets = ref<IBucket[]>([])
  const { t } = useI18n()
  const { fetch } = initLazyQuery({
    handle: async (data: { tags: ITag[]; mediaBuckets: IBucket[] }, error: string) => {
      if (error) {
        toast(t(error), 'error')
      } else {
        if (data) {
          tags.value = data.tags
          buckets.value = data.mediaBuckets
        }
      }
    },
    document: bucketsTagsGQL,
    variables: {
      type,
    },
  })

  const result: BucketsTagsResult = {
    tags,
    buckets,
    fetch,
  }

  bucketsTagsCache.set(key, result)
  return result
}

export const useMediaSourceDirs = () => {
  if (mediaSourceDirsCache) return mediaSourceDirsCache

  const { t } = useI18n()
  const sourceDirs = ref<string[]>([])

  const { fetch, loading } = initLazyQuery({
    document: mediaSourceDirsGQL,
    variables: () => ({}),
    handle: (data: { mediaSourceDirs: string[] }, error: string) => {
      if (error) {
        toast(t(error), 'error')
        return
      }
      sourceDirs.value = (data?.mediaSourceDirs ?? []).filter(Boolean)
    },
  })

  const { mutate: saveMutation, loading: saving, onDone: onSaveDone, onError: onSaveError } = initMutation({
    document: setMediaSourceDirsGQL,
  })

  const save = async (dirs: string[]) => {
    const normalized = (dirs ?? []).filter(Boolean)
    const ok = await new Promise<boolean>((resolve) => {
      let doneSub: { off: () => void } | null = null
      let errorSub: { off: () => void } | null = null

      const cleanup = () => {
        doneSub?.off()
        errorSub?.off()
        doneSub = null
        errorSub = null
      }

      doneSub = onSaveDone(() => {
        cleanup()
        resolve(true)
      })

      errorSub = onSaveError(() => {
        cleanup()
        resolve(false)
      })

      saveMutation({ dirs: normalized }).catch(() => {
        // Errors are handled via onError callbacks.
      })
    })

    if (!ok) return
    sourceDirs.value = normalized
  }

  mediaSourceDirsCache = {
    sourceDirs,
    loading,
    saving,
    fetch,
    save,
  }

  // Initial load.
  mediaSourceDirsCache.fetch()

  return mediaSourceDirsCache
}
