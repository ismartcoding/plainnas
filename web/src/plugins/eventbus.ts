import type {
  IItemTagsUpdatedEvent,
  IItemsTagsUpdatedEvent,
  IFileDeletedEvent,
  IFileRenamedEvent,
  IFileRestoredEvent,
  IFileTrashedEvent,
  IMediaItemsActionedEvent,
} from '@/lib/interfaces'
import type { IUploadItem } from '@/stores/temp'
import mitt, { type Emitter } from 'mitt'
import type { IDlnaRenderer } from '@/lib/interfaces'

type Events = {
  upload_task_done: IUploadItem
  upload_progress: IUploadItem
  refetch_app: undefined
  refetch_favorite_folders: undefined
  play_audio: undefined
  do_play_audio: undefined
  pause_audio: undefined
  media_scan_progress: any
  file_task_progress: any
  dlna_renderer_found: IDlnaRenderer
  dlna_discovery_done: any
  item_tags_updated: IItemTagsUpdatedEvent
  items_tags_updated: IItemsTagsUpdatedEvent
  refetch_tags: string
  media_items_actioned: IMediaItemsActionedEvent
  file_deleted: IFileDeletedEvent
  file_trashed: IFileTrashedEvent
  file_restored: IFileRestoredEvent
  file_renamed: IFileRenamedEvent
  toast: string
  color_mode_changed: undefined
  app_socket_connection_changed: boolean
}

const emitter: Emitter<Events> = mitt<Events>()

export default emitter
