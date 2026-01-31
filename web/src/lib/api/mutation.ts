import type { ApolloCache, DocumentNode } from '@apollo/client/core'
import gql from 'graphql-tag'
import { useMutation } from '@vue/apollo-composable'
import { fileFragment, playlistAudioFragment, tagFragment } from './fragments'
import { logErrorMessages } from '@vue/apollo-util'
import emitter from '@/plugins/eventbus'

export class InitMutationParams {
  document!: DocumentNode
  options?: any = {}
}
export function initMutation(params: InitMutationParams, handleError = true) {
  const r = useMutation(params.document, {
    clientId: 'a',
    ...params.options,
  })

  if (handleError) {
    r.onError((error) => {
      if (error.networkError?.message === 'connection_timeout') {
        emitter.emit('toast', 'connection_timeout')
      } else {
        emitter.emit('toast', error.message)
      }
      logErrorMessages(error)
    })
  }

  return r
}

export async function runMutation<TVariables extends Record<string, any> | undefined>(
  mutate: (variables?: TVariables) => Promise<any>,
  onDone: (fn: (...args: any[]) => void) => { off: () => void },
  onError: (fn: (...args: any[]) => void) => { off: () => void },
  variables?: TVariables
): Promise<boolean> {
  return await new Promise<boolean>((resolve) => {
    let doneSub: { off: () => void } | null = null
    let errorSub: { off: () => void } | null = null

    const cleanup = () => {
      doneSub?.off()
      errorSub?.off()
      doneSub = null
      errorSub = null
    }

    doneSub = onDone(() => {
      cleanup()
      resolve(true)
    })

    errorSub = onError(() => {
      cleanup()
      resolve(false)
    })

    mutate(variables).catch(() => {
      // Errors are handled via onError callbacks.
    })
  })
}

export function insertCache(cache: ApolloCache<any>, data: any, query: DocumentNode, variables?: any, reversed: boolean = false) {
  const q: any = cache.readQuery({ query, variables })
  const key = Object.keys(q)[0]
  const obj: Record<string, any> = {}
  if (key === 'files') {
    obj[key] = {
      ...q[key],
      items: reversed ? data.concat(q[key]['items']) : q[key]['items'].concat(data),
    }
  } else {
    const existing = Array.isArray(q[key]) ? q[key] : []
    const incoming = Array.isArray(data) ? data : [data]
    const combined = reversed ? incoming.concat(existing) : existing.concat(incoming)
    const seen = new Set<string>()
    const deduped = combined.filter((it: any) => {
      if (it && typeof it === 'object' && 'id' in it && it.id != null) {
        if (seen.has(it.id)) return false
        seen.add(it.id)
        return true
      }
      return true
    })
    obj[key] = deduped
  }
  cache.writeQuery({ query, variables, data: obj })
}

export const createDirGQL = gql`
  mutation createDir($path: String!) {
    createDir(path: $path) {
      ...FileFragment
    }
  }
  ${fileFragment}
`

export const writeTextFileGQL = gql`
  mutation writeTextFile($path: String!, $content: String!, $overwrite: Boolean!) {
    writeTextFile(path: $path, content: $content, overwrite: $overwrite) {
      ...FileFragment
    }
  }
  ${fileFragment}
`

export const renameFileGQL = gql`
  mutation renameFile($path: String!, $name: String!) {
    renameFile(path: $path, name: $name)
  }
`

export const setDeviceNameGQL = gql`
  mutation setDeviceName($name: String!) {
    setDeviceName(name: $name)
  }
`

export const revokeSessionGQL = gql`
  mutation revokeSession($clientId: String!) {
    revokeSession(clientId: $clientId)
  }
`

export const logoutGQL = gql`
  mutation logout {
    logout
  }
`

export const copyFileGQL = gql`
  mutation copyFile($src: String!, $dst: String!, $overwrite: Boolean!) {
    copyFile(src: $src, dst: $dst, overwrite: $overwrite)
  }
`

export const moveFileGQL = gql`
  mutation moveFile($src: String!, $dst: String!, $overwrite: Boolean!) {
    moveFile(src: $src, dst: $dst, overwrite: $overwrite)
  }
`

export const createCopyTaskGQL = gql`
  mutation createCopyTask($ops: [FileTaskOpInput!]!) {
    createCopyTask(ops: $ops) {
      id
      type
      title
      status
      error
      totalBytes
      doneBytes
      totalItems
      doneItems
      createdAt
      updatedAt
    }
  }
`

export const createMoveTaskGQL = gql`
  mutation createMoveTask($ops: [FileTaskOpInput!]!) {
    createMoveTask(ops: $ops) {
      id
      type
      title
      status
      error
      totalBytes
      doneBytes
      totalItems
      doneItems
      createdAt
      updatedAt
    }
  }
`

export const deleteFilesGQL = gql`
  mutation deleteFiles($paths: [String!]!) {
    deleteFiles(paths: $paths)
  }
`

export const trashFilesGQL = gql`
  mutation trashFiles($paths: [String!]!) {
    trashFiles(paths: $paths)
  }
`

export const restoreFilesGQL = gql`
  mutation restoreFiles($paths: [String!]!) {
    restoreFiles(paths: $paths)
  }
`

export const formatDiskGQL = gql`
  mutation formatDisk($path: String!) {
    formatDisk(path: $path)
  }
`

export const dlnaCastGQL = gql`
  mutation dlnaCast($rendererUdn: String!, $url: String!, $title: String!, $mime: String!, $type: DataType!) {
    dlnaCast(rendererUdn: $rendererUdn, url: $url, title: $title, mime: $mime, type: $type)
  }
`

export const playAudioGQL = gql`
  mutation playAudio($path: String!) {
    playAudio(path: $path) {
      ...PlaylistAudioFragment
    }
  }
  ${playlistAudioFragment}
`

export const updateAudioPlayModeGQL = gql`
  mutation updateAudioPlayMode($mode: MediaPlayMode!) {
    updateAudioPlayMode(mode: $mode)
  }
`

export const deletePlaylistAudioGQL = gql`
  mutation deletePlaylistAudio($path: String!) {
    deletePlaylistAudio(path: $path)
  }
`

export const addPlaylistAudiosGQL = gql`
  mutation addPlaylistAudios($query: String!) {
    addPlaylistAudios(query: $query)
  }
`

export const clearAudioPlaylistGQL = gql`
  mutation clearAudioPlaylist {
    clearAudioPlaylist
  }
`

export const reorderPlaylistAudiosGQL = gql`
  mutation reorderPlaylistAudios($paths: [String!]!) {
    reorderPlaylistAudios(paths: $paths)
  }
`

export const deleteMediaItemsGQL = gql`
  mutation deleteMediaItems($type: DataType!, $query: String!) {
    deleteMediaItems(type: $type, query: $query) {
      type
      query
    }
  }
`

export const trashMediaItemsGQL = gql`
  mutation trashMediaItems($type: DataType!, $query: String!) {
    trashMediaItems(type: $type, query: $query) {
      type
      query
    }
  }
`

export const restoreMediaItemsGQL = gql`
  mutation restoreMediaItems($type: DataType!, $query: String!) {
    restoreMediaItems(type: $type, query: $query) {
      type
      query
    }
  }
`

export const removeFromTagsGQL = gql`
  mutation removeFromTags($type: DataType!, $tagIds: [ID!]!, $query: String!) {
    removeFromTags(type: $type, tagIds: $tagIds, query: $query)
  }
`

export const addToTagsGQL = gql`
  mutation addToTags($type: DataType!, $tagIds: [ID!]!, $query: String!) {
    addToTags(type: $type, tagIds: $tagIds, query: $query)
  }
`

export const updateTagRelationsGQL = gql`
  mutation updateTagRelations($type: DataType!, $item: TagRelationStub!, $addTagIds: [ID!]!, $removeTagIds: [ID!]!) {
    updateTagRelations(type: $type, item: $item, addTagIds: $addTagIds, removeTagIds: $removeTagIds)
  }
`

export const createTagGQL = gql`
  mutation createTag($type: DataType!, $name: String!) {
    createTag(type: $type, name: $name) {
      ...TagFragment
    }
  }
  ${tagFragment}
`

export const updateTagGQL = gql`
  mutation updateTag($id: ID!, $name: String!) {
    updateTag(id: $id, name: $name) {
      ...TagFragment
    }
  }
  ${tagFragment}
`

export const deleteTagGQL = gql`
  mutation deleteTag($id: ID!) {
    deleteTag(id: $id)
  }
`

export const addFavoriteFolderGQL = gql`
  mutation addFavoriteFolder($rootPath: String!, $relativePath: String!) {
    addFavoriteFolder(rootPath: $rootPath, relativePath: $relativePath) {
      rootPath
      relativePath
      alias
    }
  }
`

export const removeFavoriteFolderGQL = gql`
  mutation removeFavoriteFolder($rootPath: String!, $relativePath: String!) {
    removeFavoriteFolder(rootPath: $rootPath, relativePath: $relativePath) {
      rootPath
      relativePath
      alias
    }
  }
`

export const setFavoriteFolderAliasGQL = gql`
  mutation setFavoriteFolderAlias($rootPath: String!, $relativePath: String!, $alias: String!) {
    setFavoriteFolderAlias(rootPath: $rootPath, relativePath: $relativePath, alias: $alias)
  }
`

export const setTempValueGQL = gql`
  mutation setTempValue($key: String!, $value: String!) {
    setTempValue(key: $key, value: $value) {
      key
      value
    }
  }
`

export const mergeChunksGQL = gql`
  mutation mergeChunks($fileId: String!, $totalChunks: Int!, $path: String!, $replace: Boolean!) {
    mergeChunks(fileId: $fileId, totalChunks: $totalChunks, path: $path, replace: $replace)
  }
`

// Media scan control
export const startMediaScanGQL = gql`
  mutation startMediaScan($root: String!) { startMediaScan(root: $root) }
`
export const pauseMediaScanGQL = gql`
  mutation pauseMediaScan { pauseMediaScan }
`
export const resumeMediaScanGQL = gql`
  mutation resumeMediaScan { resumeMediaScan }
`
export const stopMediaScanGQL = gql`
  mutation stopMediaScan { stopMediaScan }
`
export const rebuildMediaIndexGQL = gql`
  mutation rebuildMediaIndex($root: String!) { rebuildMediaIndex(root: $root) }
`

// Storage volume alias
export const setMountAliasGQL = gql`
  mutation setMountAlias($id: String!, $alias: String!) {
    setMountAlias(id: $id, alias: $alias)
  }
`

export const setMediaSourceDirsGQL = gql`
  mutation setMediaSourceDirs($dirs: [String!]!) {
    setMediaSourceDirs(dirs: $dirs)
  }
`

export const setSambaSettingsGQL = gql`
  mutation setSambaSettings($input: SambaSettingsInput!) {
    setSambaSettings(input: $input)
  }
`

export const setSambaUserPasswordGQL = gql`
  mutation setSambaUserPassword($password: String!) {
    setSambaUserPassword(password: $password)
  }
`

