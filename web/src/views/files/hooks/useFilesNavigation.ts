import type { ComputedRef, Ref } from 'vue'

import { decodeBase64 } from '@/lib/strutil'
import { replacePath } from '@/plugins/router'

import type { IFileFilter } from '@/lib/interfaces'

export function useFilesNavigation(opts: {
    filter: IFileFilter
    rootDir: ComputedRef<string>
    mainStore: any
    buildQ: (filter: IFileFilter) => string

    q: Ref<string>
    resetPaging: () => void
    clearSelection: () => void
    fetchCount: () => void
    fetch: () => void
}) {
    function getUrl(encodedQ: string) {
        const base = '/files'
        return encodedQ ? `${base}?q=${encodedQ}` : base
    }

    function applyEncodedQ(encodedQ: string) {
        replacePath(opts.mainStore, getUrl(encodedQ))

        // Update in-place (route may not re-activate this view).
        opts.resetPaging()
        opts.q.value = decodeBase64(encodedQ)
        opts.fetchCount()
        opts.fetch()
    }

    function syncFromRoute(encodedQParam: string | undefined, parseQ: (filter: IFileFilter, q: string) => void, showHidden: boolean) {
        opts.q.value = decodeBase64(encodedQParam ?? '')
        parseQ(opts.filter, opts.q.value)
        opts.filter.showHidden = showHidden

        opts.resetPaging()
        opts.fetchCount()
        opts.fetch()
    }

    function navigateToDir(dir: string) {
        opts.clearSelection()

        // Clear search text when navigating to avoid filtering the new directory.
        opts.filter.text = ''

        if (dir.startsWith(opts.rootDir.value)) {
            // Normal browsing uses (root_path + relative_path).
            const rel = dir.substring(opts.rootDir.value.length)
            opts.filter.relativePath = rel.startsWith('/') ? rel.substring(1) : rel
        } else {
            // Navigating outside current root.
            opts.filter.rootPath = dir
            opts.filter.relativePath = ''
        }

        applyEncodedQ(opts.buildQ(opts.filter))
    }

    function toggleShowHidden() {
        opts.filter.showHidden = !opts.filter.showHidden
        opts.mainStore.fileShowHidden = opts.filter.showHidden
        applyEncodedQ(opts.buildQ(opts.filter))
    }

    return {
        syncFromRoute,
        navigateToDir,
        toggleShowHidden,
    }
}
