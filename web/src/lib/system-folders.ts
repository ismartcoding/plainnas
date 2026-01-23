import { getFileName } from '@/lib/api/file'

const HIDDEN_SYSTEM_DIR_NAMES = new Set(['lost+found'])

export function isHiddenSystemDirName(name: string): boolean {
    return HIDDEN_SYSTEM_DIR_NAMES.has(String(name || '').trim())
}

export function shouldHideSystemPath(path: string, isDir?: boolean): boolean {
    if (isDir === false) return false
    const base = getFileName(path)
    return isHiddenSystemDirName(base)
}
