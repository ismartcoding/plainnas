import { type ComputedRef, type Ref } from 'vue'

import { openModal } from '@/components/modal'
import toast from '@/components/toaster'

import { initMutation, addFavoriteFolderGQL, restoreFilesGQL, setTempValueGQL, trashFilesGQL } from '@/lib/api/mutation'
import { shortUUID } from '@/lib/strutil'
import { getFileUrlByPath, getPdfPreviewUrlByPath } from '@/lib/api/file'

import DownloadMethodModal from '@/components/DownloadMethodModal.vue'
import EditValueModal from '@/components/EditValueModal.vue'

import emitter from '@/plugins/eventbus'

import { useDownload, useRename } from '@/hooks/files'

import type sjcl from 'sjcl'
import { canPreviewAsPdf, type IFile } from '@/lib/file'

export function useFilesActions(opts: {
    items: Ref<IFile[]>
    total: Ref<number>
    selectedIds: Ref<string[]>
    clearSelection: () => void

    currentDir: ComputedRef<string>
    rootDir: ComputedRef<string>

    urlTokenKey: Ref<sjcl.BitArray | null>
    docPreviewAvailable: Ref<boolean>

    t: (key: string, params?: any) => string

    fetch: () => void
    refetchStats: () => void

    copy: (ids: string[]) => void
    cut: (ids: string[]) => void
    paste: (dir: string) => void
}) {
    const { downloadFile, downloadDir, downloadFiles } = useDownload(opts.urlTokenKey)

    const { renameItem, renameDone, renameMutation, renameVariables } = useRename(() => {
        opts.fetch()
    })

    const { mutate: setTempValue, onDone: setTempValueDone, loading: downloadLoading } = initMutation({
        document: setTempValueGQL,
    })

    setTempValueDone((r: any) => {
        downloadFiles(r.data.setTempValue.key)
        opts.clearSelection()
    })

    const { mutate: trashFilesMutation } = initMutation({
        document: trashFilesGQL,
    })

    const { mutate: restoreFilesMutation } = initMutation({
        document: restoreFilesGQL,
    })

    const { mutate: addFavoriteFolderMutation, loading: addingFavorite } = initMutation({
        document: addFavoriteFolderGQL,
        options: {
            update: () => {
                emitter.emit('refetch_favorite_folders')
            },
        },
    })

    function onTrashed(paths: string[]) {
        paths.forEach((p) => {
            const idx = opts.items.value.findIndex((it) => it.id === p)
            if (idx >= 0) opts.items.value.splice(idx, 1)
        })
        opts.total.value = opts.items.value.length
        opts.clearSelection()
        opts.refetchStats()
    }

    function onRestored(paths: string[]) {
        paths.forEach((p) => {
            const idx = opts.items.value.findIndex((it) => it.id === p)
            if (idx >= 0) opts.items.value.splice(idx, 1)
        })
        opts.total.value = opts.items.value.length
        opts.clearSelection()
        opts.refetchStats()
    }

    function downloadItems() {
        const selected = opts.items.value.filter((it) => opts.selectedIds.value.includes(it.id))
        if (selected.length === 0) {
            toast(opts.t('select_first'), 'error')
            return
        }

        if (selected.length === 1) {
            const item = selected[0]
            if (item.isDir) {
                downloadDir(item.path)
            } else {
                downloadFile(item.path)
            }
            opts.clearSelection()
            return
        }

        openModal(DownloadMethodModal, {
            onEach: async () => {
                for (const it of selected) {
                    if (it.isDir) {
                        downloadDir(it.path)
                    } else {
                        downloadFile(it.path)
                    }
                    await new Promise((resolve) => setTimeout(resolve, 250))
                }
                opts.clearSelection()
            },
            onZip: () => {
                setTempValue({
                    key: shortUUID(),
                    value: JSON.stringify(
                        opts.selectedIds.value.map((it: string) => ({
                            path: it,
                        }))
                    ),
                })
            },
        })
    }

    function trashItems() {
        const selected = opts.items.value.filter((it) => opts.selectedIds.value.includes(it.id))
        if (selected.length === 0) {
            toast(opts.t('select_first'), 'error')
            return
        }

        const paths = selected.map((it) => it.path)
        trashFilesMutation({ paths }).then(() => {
            onTrashed(paths)
            emitter.emit('file_trashed', { paths })
            opts.fetch()
        })
    }

    function deleteItem(item: IFile) {
        trashFilesMutation({ paths: [item.path] }).then(() => {
            onTrashed([item.path])
            emitter.emit('file_trashed', { paths: [item.path] })
            opts.fetch()
        })
    }

    function restoreItem(item: IFile) {
        restoreFilesMutation({ paths: [item.path] }).then(() => {
            onRestored([item.path])
            emitter.emit('file_restored', { paths: [item.path] })
            opts.fetch()
        })
    }

    function renameItemClick(item: IFile) {
        renameItem.value = item
        openModal(EditValueModal, {
            title: opts.t('rename'),
            placeholder: opts.t('name'),
            value: item.name,
            mutation: renameMutation,
            getVariables: renameVariables,
            done: renameDone,
        })
    }

    function copyItems() {
        opts.copy(opts.selectedIds.value)
        opts.clearSelection()
    }

    function cutItems() {
        opts.cut(opts.selectedIds.value)
        opts.clearSelection()
    }

    function pasteDir() {
        opts.paste(opts.currentDir.value)
    }

    function duplicateItem(item: IFile) {
        opts.copy([item.id])
        opts.paste(opts.currentDir.value)
    }

    function cutItem(item: IFile) {
        opts.cut([item.id])
    }

    function copyItem(item: IFile) {
        opts.copy([item.id])
    }

    function pasteItem(item: IFile) {
        opts.paste(item.path)
    }

    function fallbackCopyToClipboard(text: string) {
        try {
            const textArea = document.createElement('textarea')
            textArea.value = text
            textArea.style.position = 'fixed'
            textArea.style.left = '-999999px'
            textArea.style.top = '-999999px'
            document.body.appendChild(textArea)
            textArea.focus()
            textArea.select()

            const successful = document.execCommand('copy')
            document.body.removeChild(textArea)

            if (successful) {
                toast(opts.t('link_copied'))
            } else {
                toast(opts.t('copy_failed'), 'error')
            }
        } catch (err) {
            console.error('Failed to copy text: ', err)
            toast(opts.t('copy_failed'), 'error')
        }
    }

    function copyLinkItem(item: IFile) {
        const url = canPreviewAsPdf(item.name) && opts.docPreviewAvailable.value
            ? getPdfPreviewUrlByPath(opts.urlTokenKey.value, item.path)
            : getFileUrlByPath(opts.urlTokenKey.value, item.path)

        if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard
                .writeText(url)
                .then(() => {
                    toast(opts.t('link_copied'))
                })
                .catch(() => {
                    fallbackCopyToClipboard(url)
                })
        } else {
            fallbackCopyToClipboard(url)
        }
    }

    function addToFavoritesClick(item: IFile) {
        if (!item.isDir) return

        const rootPath = opts.rootDir.value
        const relativePath = item.path.startsWith(rootPath) ? item.path.substring(rootPath.length).replace(/^\/+/, '') : ''

        const finalRoot = item.path.startsWith(rootPath) ? rootPath : item.path
        const finalRelative = item.path.startsWith(rootPath) ? relativePath : ''

        addFavoriteFolderMutation({
            rootPath: finalRoot,
            relativePath: finalRelative,
        })
            .then(() => {
                toast(opts.t('added'))
            })
            .catch((error) => {
                console.error('Error adding favorite folder:', error)
                toast(opts.t('error'), 'error')
            })
    }

    return {
        downloadLoading,
        addingFavorite,

        downloadItems,
        trashItems,
        deleteItem,
        restoreItem,
        renameItemClick,

        copyItems,
        cutItems,
        pasteDir,
        duplicateItem,
        cutItem,
        copyItem,
        pasteItem,

        copyLinkItem,
        addToFavoritesClick,
    }
}
