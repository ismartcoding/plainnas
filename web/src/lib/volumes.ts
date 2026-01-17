import type { IStorageVolume } from '@/lib/interfaces'

export type TranslateFn = (key: string) => string

export function getStorageVolumeBaseTitle(v: IStorageVolume, t: TranslateFn): string {
    const name = String(v?.name ?? '').trim()
    const mountPoint = String(v?.mountPoint ?? '').trim()

    // For the root mount we show a localized name.
    if (name === '/' || mountPoint === '/') return t('internal_storage')
    return name || mountPoint || '/'
}

export function getStorageVolumeTitle(v: IStorageVolume, t: TranslateFn): string {
    const alias = String(v?.alias ?? '').trim()
    return alias ? alias : getStorageVolumeBaseTitle(v, t)
}

function isInternalStorageVolume(v: IStorageVolume): boolean {
    const name = String(v?.name ?? '').trim()
    const mountPoint = String(v?.mountPoint ?? '').trim()
    return name === '/' || mountPoint === '/'
}

// Sort by what the UI shows (title), not raw fields.
export function sortStorageVolumesByTitle(volumes: IStorageVolume[], t: TranslateFn): IStorageVolume[] {
    return [...(volumes ?? [])].sort((a, b) => {
        const ai = isInternalStorageVolume(a)
        const bi = isInternalStorageVolume(b)
        if (ai !== bi) return ai ? -1 : 1

        const at = getStorageVolumeTitle(a, t)
        const bt = getStorageVolumeTitle(b, t)

        const byTitle = at.localeCompare(bt, undefined, { numeric: true, sensitivity: 'base' })
        if (byTitle !== 0) return byTitle

        const am = String(a?.mountPoint ?? '').trim()
        const bm = String(b?.mountPoint ?? '').trim()
        return am.localeCompare(bm, undefined, { numeric: true, sensitivity: 'base' })
    })
}
