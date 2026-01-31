import { computed, type ComputedRef, type Ref } from 'vue'

import { formatFileSize } from '@/lib/format'
import { getFileName } from '@/lib/api/file'
import { getStorageVolumeTitle } from '@/lib/volumes'

import type { IBreadcrumbItem, IFileFilter, IStorageMount } from '@/lib/interfaces'

export function useFilesBreadcrumb(opts: {
    filter: IFileFilter
    volumes: Ref<IStorageMount[]>
    t: (key: string, params?: any) => string
}): {
    rootDir: ComputedRef<string>
    currentDir: ComputedRef<string>
    breadcrumbCurrentDir: ComputedRef<string>
    breadcrumbPaths: ComputedRef<IBreadcrumbItem[]>
    getPageTitle: () => string
    getPageStats: () => string
} {
    const rootDir = computed(() => opts.filter.rootPath)

    const currentDir = computed(() => {
        if (!opts.filter.rootPath) return ''
        if (!opts.filter.relativePath) return opts.filter.rootPath
        const rel = opts.filter.relativePath.startsWith('/') ? opts.filter.relativePath.substring(1) : opts.filter.relativePath
        return `${opts.filter.rootPath}/${rel}`.replace(/\/+/, '/').replace(/\/+/, '/')
    })

    const breadcrumbCurrentDir = computed(() => currentDir.value.replace(/\/+$/, ''))

    function getPageTitle() {
        const v = opts.volumes.value.find((v) => v.mountPoint === rootDir.value)
        if (v) {
            return getStorageVolumeTitle(v, opts.t)
        }

        if (rootDir.value) {
            return getFileName(rootDir.value) || opts.t('internal_storage')
        }

        return opts.t('internal_storage')
    }

    function getPageStats() {
        const v = opts.volumes.value.find((v) => v.mountPoint === rootDir.value)
        if (!v) return ''

        return `${opts.t('storage_free_total', {
            free: formatFileSize(v.freeBytes ?? 0),
            total: formatFileSize(v.totalBytes ?? 0),
        })}`
    }

    const breadcrumbPaths = computed(() => {
        const paths: IBreadcrumbItem[] = []
        const root = rootDir.value
        let p = currentDir.value

        while (p && p !== root) {
            paths.unshift({ path: p, name: getFileName(p) })
            p = p.substring(0, p.lastIndexOf('/'))
        }

        if (root) {
            paths.unshift({ path: root, name: getPageTitle() })
        }

        return paths
    })

    return {
        rootDir,
        currentDir,
        breadcrumbCurrentDir,
        breadcrumbPaths,
        getPageTitle,
        getPageStats,
    }
}
