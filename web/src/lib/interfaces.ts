import type { IFile } from './file'

export interface IData {
  id: string
}

export interface ITag extends IData {
  id: string
  name: string
  count: number
}

export interface IType extends IData {
  id: string
  name: string
}

export interface IBucket extends IData {
  id: string
  name: string
  itemCount: number
  topItems: string[]
}

export interface IMessage extends IData {
  id: string
  body: string
  address: string
  serviceCenter: string
  date: string
  type: number
  tags: ITag[]
}

export interface IMedia extends IData {
  id: string
  title: string
  path: string
  size: number
  bucketId: string
  tags: ITag[]
  createdAt: string
  updatedAt: string
}

export interface IAudio extends IMedia {
  artist: string
  albumFileId: string
  duration: number
}

export type IImage = IMedia

export interface IVideo extends IMedia {
  duration: number
}

export interface IPlaylistAudio {
  title: string
  artist: string
  path: string
  fileId: string
  duration: number
  size: number
}

export interface IFilter {
  tagIds: string[]
  text?: string
  bucketId?: string
  type?: string
  trash?: boolean
}

export interface IFileFilter {
  showHidden: boolean
  type: string
  rootPath: string
  text: string
  // relativePath replaces parent for composing final directory with rootPath
  relativePath?: string
  trash?: boolean
}

export interface IDropdownItem {
  text: string
  click: () => void
}

export interface ITagRelationStub {
  key: string
  title: string
  size: number
}

export interface IImageItem extends IImage {
  fileId: string
}
export interface IVideoItem extends IVideo {
  fileId: string
}

export interface IStorageVolume {
  id: string
  name: string
  alias?: string
  mountPoint: string
  fsType: string
  totalBytes: number
  usedBytes: number
  freeBytes: number
  removable: boolean
  remote: boolean
  driveType: string
}

// deleted, trashed, restored
export interface IMediaItemsActionedEvent {
  type: string
  action: string
  query: string
  id?: string
}

export interface IFileDeletedEvent {
  paths: string[]
}

export interface IFileRenamedEvent {
  oldPath: string
  newPath: string
  item: IFile
}

export interface IFileTrashedEvent {
  paths: string[]
}

export interface IFileRestoredEvent {
  paths: string[]
}

export interface IItemTagsUpdatedEvent {
  item: ITagRelationStub
  type: string
}

export interface IItemsTagsUpdatedEvent {
  type: string
}

export interface IHomeStats {
  mediaCount: number
  videoCount: number
  audioCount: number
  imageCount: number
  storageVolumes: IStorageVolume[]
}

export interface IFavoriteFolder {
  rootPath: string
  relativePath: string
  alias?: string | null
}

export interface IScanProgress {
  indexed: number
  pending: number
  total: number
  state: string
  root: string
}

export interface IApp {
  urlToken: string
  httpPort: number
  httpsPort: number
  deviceName: string
  appVersion: string
  audios: IPlaylistAudio[]
  audioCurrent: string
  audioMode: string
  dataDir: string
  scanProgress: IScanProgress
}

export interface IBreadcrumbItem {
  path: string
  name: string
}
