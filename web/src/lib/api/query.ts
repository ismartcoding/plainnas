import gql from 'graphql-tag'
import { useLazyQuery, useQuery } from '@vue/apollo-composable'
type DocumentParameter<TResult, TVariables> = any
type OptionsParameter<TResult, TVariables> = any
import {
  imageFragment,
  videoFragment,
  audioFragment,
  fileFragment,
  appFragment,
  tagFragment,
  tagSubFragment,
  deviceInfoFragment,
} from './fragments'
import type { ApolloQueryResult } from '@apollo/client'

export class InitQueryParams<TResult> {
  handle!: (data: TResult, error: string) => void
  document!: DocumentParameter<TResult, undefined>
  variables?: any = null
  options?: OptionsParameter<TResult, null> = {}
}

export function initQuery<TResult = any>(params: InitQueryParams<TResult>) {
  const { result, onResult, refetch, loading, variables } = useQuery(params.document, params.variables, () => ({
    clientId: 'a',
    ...(typeof params.options === 'function' ? params.options() : params.options),
  }))

  if (result.value) {
    params.handle(result.value, '')
  }

  onResult((r) => {
    let error = ''
    if (r.error) {
      if (r.error?.networkError) {
        error = 'network_error'
      } else {
        error = r.error?.message
      }
    }
    if (error || r.data) {
      params.handle(r.data, error)
    }
  })

  return { result, onResult, refetch, loading, variables }
}

export function initLazyQuery<TResult = any>(params: InitQueryParams<TResult>) {
  const { result, onResult, load, loading, variables, refetch } = useLazyQuery(params.document, params.variables, () => ({
    clientId: 'a',
    ...(typeof params.options === 'function' ? params.options() : params.options),
  }))

  // if (result.value) {
  //   params.handle(result.value, '')
  // }

  let first = true

  onResult((r: ApolloQueryResult<any>) => {
    let error = ''
    if (r.error) {
      if (r.error?.networkError) {
        error = 'network_error'
      } else {
        error = r.error?.message
      }
    }
    if (error || r.data) {
      params.handle(r.data, error)
    }
  })

  return {
    result,
    onResult,
    load,
    loading,
    variables,
    refetch,
    fetch: () => {
      if (first) {
        first = false
        load()
      } else {
        refetch()
      }
    },
  }
}

export const fileInfoGQL = gql`
  query ($id: ID!, $path: String!, $includeDirSize: Boolean = false) {
    fileInfo(id: $id, path: $path, includeDirSize: $includeDirSize) {
      ... on FileInfo {
        path
        updatedAt
        size
        tags {
          ...TagSubFragment
        }
      }
      data {
        ... on ImageFileInfo {
          width
          height
          location {
            latitude
            longitude
          }
        }
        ... on VideoFileInfo {
          duration
          width
          height
          location {
            latitude
            longitude
          }
        }
        ... on AudioFileInfo {
          duration
          location {
            latitude
            longitude
          }
        }
      }
    }
  }
  ${tagSubFragment}
`

export const pathStatGQL = gql`
  query pathStat($path: String!) {
    pathStat(path: $path) {
      exists
      isDir
    }
  }
`

export const pathStatsGQL = gql`
  query pathStats($paths: [String!]!) {
    pathStats(paths: $paths) {
      path
      exists
      isDir
    }
  }
`

export const homeStatsGQL = gql`
  query {
    imageCount(query: "")
    audioCount(query: "")
    videoCount(query: "")
    mounts {
      id
      path
      totalBytes
      freeBytes
    }
  }
`

export const dlnaRenderersGQL = gql`
  query {
    dlnaRenderers {
      udn
      name
      manufacturer
      modelName
      location
    }
  }
`


export const imagesGQL = gql`
  query images($offset: Int!, $limit: Int!, $query: String!, $sortBy: FileSortBy!) {
    images(offset: $offset, limit: $limit, query: $query, sortBy: $sortBy) {
      ...ImageFragment
    }
    imageCount(query: $query)
  }
  ${imageFragment}
`

export const videosGQL = gql`
  query videos($offset: Int!, $limit: Int!, $query: String!, $sortBy: FileSortBy!) {
    videos(offset: $offset, limit: $limit, query: $query, sortBy: $sortBy) {
      ...VideoFragment
    }
    videoCount(query: $query)
  }
  ${videoFragment}
`

export const audiosGQL = gql`
  query audios($offset: Int!, $limit: Int!, $query: String!, $sortBy: FileSortBy!) {
    items: audios(offset: $offset, limit: $limit, query: $query, sortBy: $sortBy) {
      ...AudioFragment
    }
    total: audioCount(query: $query)
  }
  ${audioFragment}
`

export const filesGQL = gql`
  query files($offset: Int!, $limit: Int!, $query: String!, $sortBy: FileSortBy!) {
    files(offset: $offset, limit: $limit, query: $query, sortBy: $sortBy) {
      ...FileFragment
    }
  }
  ${fileFragment}
`

export const filesCountGQL = gql`
  query filesCount($query: String!) {
    filesCount(query: $query)
  }
`

export const recentFilesGQL = gql`
  query recentFiles {
    recentFiles {
      ...FileFragment
    }
  }
  ${fileFragment}
`

export const filesSidebarCountsGQL = gql`
  query filesSidebarCounts {
    recentFilesCount
    trashCount
  }
`

export const mountsGQL = gql`
  query {
    mounts {
      id
      name
      alias
      label
      mountPoint
      fsType
      totalBytes
      usedBytes
      freeBytes
      remote
      driveType

      diskID
      path
      partitionNum
      uuid
    }
  }
`

export const disksGQL = gql`
  query {
    disks {
      id
      name
      path
      sizeBytes
      removable
      model
    }
  }
`

export const mediaSourceDirsGQL = gql`
  query {
    mediaSourceDirs
  }
`

export const sessionsGQL = gql`
  query {
    sessions {
      clientId
      clientName
      lastActive
      createdAt
      updatedAt
    }
  }
`

export const eventsGQL = gql`
  query events($limit: Int!) {
    events(limit: $limit) {
      id
      type
      message
      clientId
      createdAt
    }
  }
`

export const sambaSettingsGQL = gql`
  query {
    sambaSettings {
      enabled
      username
      hasPassword
      shares {
        name
        sharePath
        auth
        readOnly
      }
      serviceName
      serviceActive
      serviceEnabled
    }
  }
`

export const appGQL = gql`
  query {
    app {
      ...AppFragment
    }
  }
  ${appFragment}
`

export const favoriteFoldersGQL = gql`
  query {
    favoriteFolders {
      rootPath
      relativePath
      alias
    }
  }
`

export const getTasksGQL = gql`
  query {
    getTasks {
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

export const tagsGQL = gql`
  query tags($type: DataType!) {
    tags(type: $type) {
      ...TagFragment
    }
  }
  ${tagFragment}
`

export const mediaBucketsGQL = gql`
  query mediaBuckets($type: DataType!) {
    mediaBuckets(type: $type) {
      id
      name
      itemCount
      topItems
    }
  }
`

export const bucketsTagsGQL = gql`
  query bucketsTags($type: DataType!) {
    tags(type: $type) {
      ...TagFragment
    }
    mediaBuckets(type: $type) {
      id
      name
      itemCount
      topItems
    }
  }
  ${tagFragment}
`

export const imageCountGQL = gql`
  query {
    total: imageCount(query: "")
    trash: imageCount(query: "trash:true")
  }
`

export const audioCountGQL = gql`
  query {
    total: audioCount(query: "")
    trash: audioCount(query: "trash:true")
  }
`

export const videoCountGQL = gql`
  query {
    total: videoCount(query: "")
    trash: videoCount(query: "trash:true")
  }
`

export const deviceInfoGQL = gql`
  query {
    deviceInfo {
      ...DeviceInfoFragment
    }
  }
  ${deviceInfoFragment}
`

export const appUpdateGQL = gql`
  query {
    appUpdate {
      currentVersion
      latestVersion
      hasUpdate
      url
    }
  }
`

export const uploadedChunksGQL = gql`
  query uploadedChunks($fileId: String!) {
    uploadedChunks(fileId: $fileId)
  }
`