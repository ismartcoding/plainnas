import { ref, type Ref } from 'vue'
import type { ApolloCache, ApolloError } from '@apollo/client/core'
import { createCopyTaskGQL, createDirGQL, createMoveTaskGQL, deleteFilesGQL, initMutation, renameFileGQL } from '@/lib/api/mutation'
import { enrichFile, isAudio, isImage, isVideo, type IFile } from '@/lib/file'
import { initQuery, mountsGQL, pathStatGQL } from '@/lib/api/query'
import { useI18n } from 'vue-i18n'
import toast from '@/components/toaster'
import { download, encryptUrlParams, getFileId, getFileName, getFileUrl, getFileUrlByPath } from '@/lib/api/file'
import type { ISource } from '@/components/lightbox/types'
import { encodeBase64 } from '@/lib/strutil'
import { buildQuery, parseQuery, type IFilterField } from '@/lib/search'
import { findIndex, remove } from 'lodash-es'
import { getApiBaseUrl } from '@/lib/api/api'
import type { IApp, IFileFilter, IStorageMount } from '@/lib/interfaces'
import type sjcl from 'sjcl'
import { sortStorageVolumesByTitle } from '@/lib/volumes'
import { useTasksStore } from '@/stores/tasks'
import apollo from '@/plugins/apollo'
import { promptModal } from '@/components/modal'
import FileConflictModal, { type ConflictChoice, type ConflictMode } from '@/components/files/FileConflictModal.vue'

export const useCreateDir = (urlTokenKey: Ref<sjcl.BitArray | null>, items: Ref<IFile[]>) => {
  const createPath = ref('')

  return {
    createPath,
    createVariables(value: string) {
      return { path: createPath.value + '/' + value }
    },
    createMutation() {
      return initMutation({
        document: createDirGQL,
        options: {
          update: async (_: ApolloCache<any>, data: any) => {
            const d = data.data.createDir
            remove(items.value, (it: IFile) => it.path === d.path)
            items.value.unshift(enrichFile(d, urlTokenKey.value))
          },
        },
      })
    },
  }
}

export const useRename = (fetch: () => void) => {
  const renameItem = ref<IFile>()
  return {
    renameItem,
    renameDone(newName: string) {
      fetch()
    },
    renameMutation() {
      return initMutation({
        document: renameFileGQL,
      })
    },
    renameVariables(value: string) {
      return { path: renameItem.value?.path, name: value }
    },
  }
}

export const useVolumes = () => {
  const { t } = useI18n()
  const volumes = ref<IStorageMount[]>([])
  const { refetch } = initQuery({
    handle: (data: { mounts: IStorageMount[] }, error: string) => {
      if (!error) {
        const mountedOnly = (data.mounts ?? []).filter((m) => !String(m?.path ?? '').trim())

        const userVisible = mountedOnly.filter((m) => {
          const mp = String(m?.mountPoint ?? '').trim()
          const fs = String(m?.fsType ?? '').trim().toLowerCase()
          const total = Number(m?.totalBytes ?? 0)
          const label = String(m?.label ?? '').trim()
          const alias = String(m?.alias ?? '').trim()
          const uuid = String(m?.uuid ?? '').trim()
          const driveType = String(m?.driveType ?? '').trim().toLowerCase()
          const name = String(m?.name ?? '').trim()

          // Hide boot/EFI mounts and similar system paths.
          if (mp === '/boot' || mp === '/boot/efi' || mp === '/efi' || mp === '/boot/firmware') return false

          // Hide typical EFI partitions (vfat + very small).
          if (fs === 'vfat' && total > 0 && total < 2 * 1000 * 1000 * 1000 && !alias && !label) return false

          // Hide PlainNAS auto-mount placeholders like "usb1"/"usb2" unless the user named them.
          // This keeps the sidebar focused on human-meaningful storage (e.g. "Primary", "Backup").
          if (/^\/mnt\/usb\d+$/.test(mp) && /^usb\d+$/i.test(name) && !alias && !label) return false

          return true
        })

        volumes.value = sortStorageVolumesByTitle(userVisible, t)
      }
    },
    document: mountsGQL,
  })

  return { volumes, refetch }
}

export const useDownload = (urlTokenKey: Ref<sjcl.BitArray | null>) => {
  return {
    async downloadFile(path: string, fileName?: string) {
      const url = getFileUrlByPath(urlTokenKey.value, path)
      if (fileName) {
        download(url + `&dl=1&name=${fileName}`, fileName)
      } else {
        download(url + '&dl=1', getFileName(path))
      }
    },
    async downloadDir(path: string, fileName?: string) {
      const id = getFileId(urlTokenKey.value, path)
      const url = `${getApiBaseUrl()}/zip/dir?id=${encodeURIComponent(id)}`
      if (fileName) {
        download(url + `&name=${fileName}`, fileName)
      } else {
        download(url, getFileName(path))
      }
    },
    downloadFiles(key: string) {
      const id = encryptUrlParams(
        urlTokenKey.value,
        JSON.stringify({
          id: key,
          type: 'FILE',
          name: '',
        })
      )

      download(`${getApiBaseUrl()}/zip/files?id=${encodeURIComponent(id)}`, '')
    },
  }
}

export const useView = (sources: Ref<ISource[]>, ivView: (sources: ISource[], i: number) => void) => {
  return {
    view(items: IFile[], f: IFile) {
      sources.value = items
        .filter((it) => isImage(it.name) || isVideo(it.name) || isAudio(it.name))
        .map((it) => ({
          path: it.path,
          src: it.fileId ? getFileUrl(it.fileId) : '',
          name: getFileName(it.path),
          size: it.size,
          duration: 0,
          data: f,
        }))
      const index = findIndex(sources.value, (it: ISource) => it.path === f.path)
      ivView(sources.value, index)
    },
  }
}

export const useCopyPaste = (items: Ref<IFile[]>, isCut: Ref<boolean>, selectedFiles: Ref<IFile[]>, refetchFiles: () => void, refetchStats: () => void) => {
  const dstDir = ref<string>()

  const tasksStore = useTasksStore()

  const {
    mutate: copyMutate,
    loading: copyLoading,
    onDone: copyDone,
    onError: copyError,
  } = initMutation({
    document: createCopyTaskGQL,
  })

  const {
    mutate: cutMutate,
    loading: cutLoading,
    onDone: cutDone,
    onError: cutError,
  } = initMutation({
    document: createMoveTaskGQL,
  })

  const { t } = useI18n()

  async function getPathInfo(path: string): Promise<{ exists: boolean; isDir?: boolean }> {
    try {
      const r = await apollo.a.query({
        query: pathStatGQL,
        variables: { path },
        fetchPolicy: 'network-only',
      })
      const st = r?.data?.pathStat
      return { exists: !!st?.exists, isDir: !!st?.isDir }
    } catch {
      return { exists: false }
    }
  }

  async function promptConflict(mode: ConflictMode, details?: string): Promise<ConflictChoice | undefined> {
    return promptModal<ConflictChoice>(FileConflictModal, { mode, details })
  }

  const onError = (error: ApolloError) => {
    toast(t(error.message))
  }

  copyError(onError)
  cutError(onError)

  const onDone = (result: any) => {
    const task = result?.data?.createCopyTask ?? result?.data?.createMoveTask
    if (task) {
      tasksStore.upsertFileTask(task)
    }

    selectedFiles.value = []

    // Keep existing behavior: refetch soon, and websocket-driven task progress handles UI feedback.
    setTimeout(() => {
      refetchFiles()
      refetchStats()
    }, 500)
  }

  copyDone(onDone)
  cutDone(onDone)

  const { mutate: deleteFilesMutate } = initMutation({ document: deleteFilesGQL })

  return {
    loading: copyLoading || cutLoading,
    canPaste() {
      return selectedFiles.value.length > 0
    },
    copy(ids: string[]) {
      selectedFiles.value = items.value.filter((it) => ids.includes(it.id))
      isCut.value = false
    },
    cut(ids: string[]) {
      selectedFiles.value = items.value.filter((it) => ids.includes(it.id))
      isCut.value = true
    },
    async paste(dir: string) {
      dstDir.value = dir

      const selection = [...selectedFiles.value]
      if (selection.length === 0) return

      const targets = selection.map((file) => ({
        file,
        src: file.path,
        dst: (dir.endsWith('/') ? dir + file.name : dir + '/' + file.name).replace(/\/+?/g, '/'),
      }))

      // Detect conflicts by checking destination existence.
      const dirConflicts: Array<{ file: IFile; dst: string }> = []
      const fileConflicts: Array<{ file: IFile; dst: string }> = []
      for (const t0 of targets) {
        const info = await getPathInfo(t0.dst)
        if (!info.exists) continue
        if (t0.file.isDir) dirConflicts.push({ file: t0.file, dst: t0.dst })
        else fileConflicts.push({ file: t0.file, dst: t0.dst })
      }

      const dirConflictByDst = new Set(dirConflicts.map((c) => c.dst))
      const fileConflictByDst = new Set(fileConflicts.map((c) => c.dst))

      let dirChoice: ConflictChoice | undefined
      let fileChoice: ConflictChoice | undefined

      // Folder -> Folder conflicts
      if (dirConflicts.length > 0) {
        const details = dirConflicts.length === 1 ? dirConflicts[0].dst : `${dirConflicts.length} folders`
        dirChoice = await promptConflict('folder-folder', details)
        if (!dirChoice) return

        if (dirChoice === 'replace') {
          try {
            await deleteFilesMutate({ paths: dirConflicts.map((c) => c.dst) })
          } catch {
            return
          }
        }
      }

      // File -> File conflicts
      if (fileConflicts.length > 0) {
        const hasOnlyOneFile = selection.filter((s) => !s.isDir).length === 1
        const mode: ConflictMode = hasOnlyOneFile ? 'file-file-single' : 'file-file-multiple'
        const details = fileConflicts.length === 1 ? fileConflicts[0].dst : `${fileConflicts.length} files`
        fileChoice = await promptConflict(mode, details)
        if (!fileChoice) return

        if (fileChoice === 'skip') {
          // Remove conflicting files only.
          const skipSet = new Set(fileConflicts.map((c) => c.file.path))
          for (let i = selection.length - 1; i >= 0; i--) {
            if (!selection[i].isDir && skipSet.has(selection[i].path)) {
              selection.splice(i, 1)
            }
          }
        }
      }

      // Recompute targets after possible skip.
      const finalTargets = selection.map((file) => ({
        file,
        src: file.path,
        dst: (dir.endsWith('/') ? dir + file.name : dir + '/' + file.name).replace(/\/+?/g, '/'),
      }))

      const ops = finalTargets.map((tgt) => {
        const dst = tgt.dst
        const isDir = tgt.file.isDir
        const dstConflictsDir = dirConflictByDst.has(dst)
        const dstConflictsFile = fileConflictByDst.has(dst)

        // Default behavior: keep both (backend will uniquify when overwrite=false).
        let overwrite = false

        if (isDir && dstConflictsDir) {
          overwrite = dirChoice === 'merge' || dirChoice === 'replace'
        }

        if (!isDir && dstConflictsFile) {
          overwrite = fileChoice === 'replace'
        }

        return { src: tgt.src, dst, overwrite }
      })

      if (isCut.value) {
        cutMutate({ ops })
      } else {
        copyMutate({ ops })
      }
    },
  }
}

export function getFileDir(fileName: string) {
  let dir = 'Documents'
  if (isImage(fileName)) {
    dir = 'Pictures'
  } else if (isVideo(fileName)) {
    dir = 'Movies'
  } else if (isAudio(fileName)) {
    dir = 'Music'
  }
  return dir
}

export const useDownloadItems = (urlTokenKey: Ref<sjcl.BitArray | null>, type: string, clearSelection: () => void, fileName: string | (() => string)) => {
  const { t } = useI18n()

  return {
    downloadItems: (realAllChecked: boolean, ids: string[], query: string) => {
      let q = query
      if (!realAllChecked) {
        if (ids.length === 0) {
          toast(t('select_first'), 'error')
          return
        }
        q = `ids:${ids.join(',')}`
      }

      // Generate dynamic filename if function is provided
      const finalFileName = typeof fileName === 'function' ? fileName() : fileName

      const id = encryptUrlParams(
        urlTokenKey.value,
        JSON.stringify({
          query: q,
          type: type,
          name: finalFileName,
        })
      )
      download(`${getApiBaseUrl()}/zip/files?id=${encodeURIComponent(id)}`, finalFileName)
      clearSelection()
    },
  }
}

export const useSearch = () => {
  return {
    parseQ: (filter: IFileFilter, q: string) => {
      const fields = parseQuery(q)
      filter.showHidden = false
      filter.text = ''
      filter.rootPath = ''
      filter.relativePath = ''
      filter.trash = false
      filter.fileSize = undefined
      fields.forEach((it) => {
        if (it.name === 'text') {
          filter.text = it.value
        } else if (it.name === 'type') {
          filter.type = it.value
        } else if (it.name === 'root_path') {
          filter.rootPath = it.value
        } else if (it.name === 'relative_path') {
          filter.relativePath = it.value
        } else if (it.name === 'show_hidden') {
          filter.showHidden = it.value === 'true'
        } else if (it.name === 'trash') {
          filter.trash = it.value === 'true'
        } else if (it.name === 'file_size') {
          filter.fileSize = it.op + it.value
        }
      })
    },
    buildQ: (filter: IFileFilter): string => {
      const fields: IFilterField[] = []

      if (filter.text !== '') {
        fields.push({
          name: 'text',
          op: '',
          value: filter.text,
        })
      }

      if (filter.rootPath !== '') {
        fields.push({
          name: 'root_path',
          op: '',
          value: filter.rootPath,
        })
      }

      if (filter.relativePath !== undefined && filter.relativePath !== '') {
        fields.push({
          name: 'relative_path',
          op: '',
          value: filter.relativePath,
        })
      }

      if (filter.showHidden) {
        fields.push({
          name: 'show_hidden',
          op: '',
          value: filter.showHidden ? 'true' : 'false',
        })
      }

      if (filter.trash) {
        fields.push({
          name: 'trash',
          op: '',
          value: 'true',
        })
      }

      if (filter.fileSize !== undefined && filter.fileSize !== '') {
        const match = filter.fileSize.match(/^([><=!]+)?(.+)$/)
        if (match) {
          const op = match[1] || ''
          const value = match[2]
          fields.push({
            name: 'file_size',
            op: op,
            value: value,
          })
        }
      }

      return encodeBase64(buildQuery(fields))
    },
  }
}
