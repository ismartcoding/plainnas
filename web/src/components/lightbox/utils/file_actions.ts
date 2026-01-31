import type { Ref } from 'vue'
import type { Composer } from 'vue-i18n'
import type sjcl from 'sjcl'

import { openModal } from '@/components/modal'
import DeleteFileConfirm from '@/components/DeleteFileConfirm.vue'
import EditValueModal from '@/components/EditValueModal.vue'

import { useDownload, useRename } from '@/hooks/files'
import { useDeleteItems } from '@/hooks/media'
import { getFileName } from '@/lib/api/file'
import { DataType } from '@/lib/data'
import emitter from '@/plugins/eventbus'

import type { IFile } from '@/lib/file'
import type { ISource } from '../types'

export function useLightboxFileActions(opts: {
    current: Ref<ISource | undefined>
    urlTokenKey: Ref<sjcl.BitArray | null>
    t: Composer['t']
    refetchInfo: () => void
}) {
    const { downloadFile } = useDownload(opts.urlTokenKey)
    const { deleteItem } = useDeleteItems()

    const { renameItem, renameDone, renameMutation, renameVariables } = useRename(() => {
        opts.refetchInfo()
    })

    function deleteFile() {
        const mediaTypes = [DataType.VIDEO, DataType.AUDIO, DataType.IMAGE]
        const type = opts.current.value?.type
        const item = opts.current.value?.data

        if (type && mediaTypes.includes(type as any)) {
            deleteItem(type, item)
            return
        }

        if (!item) return

        openModal(DeleteFileConfirm, {
            files: [item],
            onDone: () => {
                emitter.emit('file_deleted', { paths: [item.path] })
            },
        })
    }

    function renameFile() {
        const current = opts.current.value
        const item = current?.data
        if (!item || !current?.path) return

        renameItem.value = {
            id: item.id,
            path: current.path,
            name: getFileName(current.path),
            size: current.size || 0,
            isDir: false,
            extension: '',
            fileId: '',
            updatedAt: '',
            createdAt: '',
        } as IFile

        openModal(EditValueModal, {
            title: opts.t('rename'),
            placeholder: opts.t('name'),
            value: getFileName(current.path),
            mutation: renameMutation,
            getVariables: renameVariables,
            done: (newName: string) => {
                renameDone(newName)

                const oldPath = current.path
                const newPath = oldPath.substring(0, oldPath.lastIndexOf('/') + 1) + newName

                emitter.emit('file_renamed', {
                    oldPath,
                    newPath,
                    item: {
                        ...current.data,
                        path: newPath,
                        name: newName,
                    },
                })

                // Keep the currently displayed state in sync.
                current.path = newPath
                current.name = newName
            },
        })
    }

    return {
        downloadFile,
        deleteFile,
        renameFile,
    }
}
