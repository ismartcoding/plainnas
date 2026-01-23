import type { Ref } from 'vue'
import type { IBucket, IFilter } from '@/lib/interfaces'
import type { IUploadItem } from '@/stores/temp'
import { getDirFromPath } from '@/lib/file'
import { pickUploadDir } from '@/lib/upload/pick-upload-dir'
import { extractClipboardFiles, queueFilesToUpload } from '@/hooks/upload'
import { isEditableTarget } from '@/lib/dom'

export function createBucketUploadTarget(options: {
    filter: Pick<IFilter, 'bucketId' | 'trash'>
    buckets: Ref<IBucket[]>
    picker: {
        title: string
        description: string
        initialPath?: string
        modalId: string
        storageKey: string
    }
}) {
    const getSelectedBucketDir = () => {
        const bucketId = options.filter.bucketId
        if (!bucketId) return ''

        const bucket = options.buckets.value.find((it) => it.id === bucketId)
        const top = bucket?.topItems?.[0]
        if (!top) return ''
        return getDirFromPath(top)
    }

    const resolveTargetDir = async (): Promise<string | undefined> => {
        const bucketDir = getSelectedBucketDir()
        if (bucketDir) return bucketDir

        return pickUploadDir({
            title: options.picker.title,
            description: options.picker.description,
            initialPath: options.picker.initialPath || '',
            modalId: options.picker.modalId,
            storageKey: options.picker.storageKey,
        })
    }

    const createPasteUploadHandler = (uploads: Ref<IUploadItem[]>, typePrefix: string) => {
        return (e: ClipboardEvent) => {
            void (async () => {
                if (options.filter.trash) return
                if (isEditableTarget(e.target)) return

                const files = extractClipboardFiles(e, typePrefix)
                if (!files.length) return

                e.preventDefault()

                const dir = await resolveTargetDir()
                if (!dir) return

                queueFilesToUpload(files, dir, uploads)
            })()
        }
    }

    return {
        getSelectedBucketDir,
        resolveTargetDir,
        createPasteUploadHandler,
    }
}
