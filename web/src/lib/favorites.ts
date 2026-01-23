import type { IFavoriteFolder, IStorageMount } from './interfaces'
import { getFullPath } from './file'

export function getFavoriteFolderFullPath(favoriteFolder: IFavoriteFolder): string {
    return getFullPath(favoriteFolder.rootPath, favoriteFolder.relativePath)
}

export function getFavoriteDisplayTitle(
    favoriteFolder: IFavoriteFolder,
    volumes: IStorageMount[],
    t: (key: string, ...args: any[]) => string
): string {
    const alias = (favoriteFolder.alias || '').trim()
    if (alias) {
        return alias
    }

    const full = getFavoriteFolderFullPath(favoriteFolder).replace(/\/+$/, '')
    const base = full.split('/').pop() || ''
    if (base) {
        return base
    }

    const volume = volumes.find((v) => v.mountPoint === favoriteFolder.rootPath)
    const volumeTitle = volume
        ? volume.name === '/'
            ? t('internal_storage')
            : (volume.name || volume.mountPoint || favoriteFolder.rootPath)
        : (favoriteFolder.rootPath || '').replace(/\/+$/, '')
    const rel = (favoriteFolder.relativePath || '').replace(/^\/+/, '')
    return rel ? `${volumeTitle}/${rel}` : volumeTitle
}


