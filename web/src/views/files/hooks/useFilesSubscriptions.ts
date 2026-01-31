import { onActivated, onDeactivated } from 'vue'

import emitter from '@/plugins/eventbus'

import type { IFileRenamedEvent, IMediaItemsActionedEvent } from '@/lib/interfaces'
import type { IUploadItem } from '@/stores/temp'

export function useFilesSubscriptions(opts: {
    fetch: () => void
    refetchStats: () => void

    activatePaging: () => void
    unbindScrollFallback: () => void

    pageKeyDown: (e: KeyboardEvent) => void
    pageKeyUp: (e: KeyboardEvent) => void
}) {
    const uploadTaskDoneHandler = (r: IUploadItem) => {
        if (r.status === 'done') {
            // have to delay 1s to make sure the api return latest data.
            setTimeout(() => {
                opts.fetch()
                opts.refetchStats()
            }, 1000)
        }
    }

    const fileRenamedHandler = (_event: IFileRenamedEvent) => {
        // Refresh file list to show new filename
        opts.fetch()
    }

    const mediaItemsActionedHandler = (event: IMediaItemsActionedEvent) => {
        if (['delete', 'trash', 'restore'].includes(event.action)) {
            opts.fetch()
            opts.refetchStats()
        }
    }

    onActivated(() => {
        opts.activatePaging()
        emitter.on('upload_task_done', uploadTaskDoneHandler)
        emitter.on('file_renamed', fileRenamedHandler)
        emitter.on('media_items_actioned', mediaItemsActionedHandler)
        window.addEventListener('keydown', opts.pageKeyDown)
        window.addEventListener('keyup', opts.pageKeyUp)
    })

    onDeactivated(() => {
        opts.unbindScrollFallback()
        emitter.off('upload_task_done', uploadTaskDoneHandler)
        emitter.off('file_renamed', fileRenamedHandler)
        emitter.off('media_items_actioned', mediaItemsActionedHandler)
        window.removeEventListener('keydown', opts.pageKeyDown)
        window.removeEventListener('keyup', opts.pageKeyUp)
    })

    return {}
}
