import { canOpenInBrowser, canPreviewAsPdf, canView, isTextFile, type IFile } from '@/lib/file'
import { getFileId, getFileUrlByPath, getPdfPreviewUrlByPath } from '@/lib/api/file'

import type sjcl from 'sjcl'
import type { Ref } from 'vue'

export function useFilesOpenItem(opts: {
    urlTokenKey: Ref<sjcl.BitArray | null>
    docPreviewAvailable: Ref<boolean>
    downloadFile: (path: string) => void
    view: (items: IFile[], item: IFile) => void
    items: Ref<IFile[]>
}) {
    function openFile(item: IFile) {
        if (isTextFile(item.name)) {
            // Open text files in new window with custom viewer
            const fileId = getFileId(opts.urlTokenKey.value, item.path)
            window.open(`/text-file?id=${encodeURIComponent(fileId)}`, '_blank')
            return
        }

        if (canPreviewAsPdf(item.name)) {
            if (opts.docPreviewAvailable.value) {
                window.open(getPdfPreviewUrlByPath(opts.urlTokenKey.value, item.path), '_blank')
            } else {
                opts.downloadFile(item.path)
            }
            return
        }

        if (canOpenInBrowser(item.name)) {
            window.open(getFileUrlByPath(opts.urlTokenKey.value, item.path), '_blank')
            return
        }

        if (canView(item.name)) {
            opts.view(opts.items.value, item)
            return
        }

        opts.downloadFile(item.path)
    }

    return { openFile }
}
