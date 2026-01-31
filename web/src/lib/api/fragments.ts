import gql from 'graphql-tag'

export const tagFragment = gql`
  fragment TagFragment on Tag {
    id
    name
    count
  }
`

export const tagSubFragment = gql`
  fragment TagSubFragment on Tag {
    id
    name
  }
`

export const playlistAudioFragment = gql`
  fragment PlaylistAudioFragment on PlaylistAudio {
    title
    artist
    path
    duration
  }
`

export const appFragment = gql`
  fragment AppFragment on App {
    urlToken
    docPreviewAvailable
    httpPort
    httpsPort
    audios {
      ...PlaylistAudioFragment
    }
    audioCurrent
    audioMode
    dataDir
    scanProgress {
      indexed
      pending
      total
      state
    }
  }
  ${playlistAudioFragment}
`

export const fileFragment = gql`
  fragment FileFragment on File {
    path
    isDir
    createdAt
    updatedAt
    size
    childCount
  }
`

export const imageFragment = gql`
  fragment ImageFragment on Image {
    id
    title
    path
    size
    bucketId
    createdAt
    updatedAt
    tags {
      ...TagSubFragment
    }
  }
  ${tagSubFragment}
`

export const videoFragment = gql`
  fragment VideoFragment on Video {
    id
    title
    path
    duration
    size
    bucketId
    createdAt
    updatedAt
    tags {
      ...TagSubFragment
    }
  }
  ${tagSubFragment}
`

export const audioFragment = gql`
  fragment AudioFragment on Audio {
    id
    title
    artist
    path
    duration
    size
    bucketId
    albumFileId
    createdAt
    updatedAt
    tags {
      ...TagSubFragment
    }
  }
  ${tagSubFragment}
`

export const deviceInfoFragment = gql`
  fragment DeviceInfoFragment on DeviceInfo {
    hostname
    os
    kernelVersion
    arch
    uptime
    bootTime
    cpuModel
    cpuCores
    cpuThreads
    load1
    load5
    load15
    memoryTotalBytes
    memoryFreeBytes
    swapTotalBytes
    swapFreeBytes
    swapUsedBytes
    ips
    nics { name mac speedRate }
    appFullVersion
    model
  }
`
