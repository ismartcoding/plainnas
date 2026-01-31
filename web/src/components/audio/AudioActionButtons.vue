<template>
  <div class="actions">
    <template v-if="filter.trash">
      <v-icon-button v-if="editMode" v-tooltip="$t('delete')" @click.stop="deleteItem(dataType, item)">
        <i-material-symbols:delete-forever-outline-rounded />
      </v-icon-button>
      <v-icon-button v-tooltip="$t('restore')" :loading="restoreLoading(`ids:${item.id}`)"
        @click.stop="restore(dataType, `ids:${item.id}`)">
        <i-material-symbols:restore-from-trash-outline-rounded />
      </v-icon-button>
      <v-icon-button v-tooltip="$t('download')"
        @click.stop="downloadFile(item.path, getFileName(item.path).replace(' ', '-'))">
        <i-material-symbols:download-rounded />
      </v-icon-button>
    </template>
    <template v-else>
      <template v-if="editMode">
        <v-icon-button v-tooltip="$t('download')"
          @click.stop="downloadFile(item.path, getFileName(item.path).replace(' ', '-'))">
          <i-material-symbols:download-rounded />
        </v-icon-button>
        <v-icon-button v-tooltip="$t('move_to_trash')" :loading="trashLoading(`ids:${item.id}`)"
          @click.stop="trash(dataType, `ids:${item.id}`)">
          <i-material-symbols:delete-outline-rounded />
        </v-icon-button>
        <v-icon-button v-tooltip="$t('add_to_tags')" @click.stop="addItemToTags(item)">
          <i-material-symbols:label-outline-rounded />
        </v-icon-button>
      </template>
      <template v-else>
        <v-icon-button v-tooltip="$t('download')"
          @click.stop="downloadFile(item.path, getFileName(item.path).replace(' ', '-'))">
          <i-material-symbols:download-rounded />
        </v-icon-button>
        <v-icon-button v-tooltip="$t('cast')"
          @click.stop="openDlnaCastModal({ url: getFileUrl(getFileId(urlTokenKey, item.path)), title: item.title || getFileName(item.path), mime: guessDlnaMimeByName(getFileName(item.path), dataType), type: dataType })">
          <i-material-symbols:airplay-outline-rounded />
        </v-icon-button>
        <v-icon-button v-if="isInPlaylist(item) && !animatingIds.includes(item.id)"
          v-tooltip="$t('remove_from_playlist')" @click.stop.prevent="handleRemoveFromPlaylist($event, item)">
          <i-material-symbols:playlist-remove class="playlist-remove-icon" />
        </v-icon-button>
        <v-icon-button v-else-if="isInPlaylist(item) && animatingIds.includes(item.id)" :disabled="true">
          <i-material-symbols:playlist-remove class="playlist-remove-icon rotating" />
        </v-icon-button>
        <v-icon-button v-else-if="!isInPlaylist(item) && !animatingIds.includes(item.id)"
          v-tooltip="$t('add_to_playlist')" @click.stop.prevent="addToPlaylist($event, item)">
          <i-material-symbols:playlist-add class="playlist-add-icon" />
        </v-icon-button>
        <v-icon-button v-else-if="!isInPlaylist(item) && animatingIds.includes(item.id)" :disabled="true">
          <i-material-symbols:playlist-add class="playlist-add-icon rotating" />
        </v-icon-button>
        <v-circular-progress v-if="playLoading && item.path === playPath" indeterminate />
        <v-icon-button v-else-if="isAudioPlaying(item)" v-tooltip="$t('pause')" @click.stop="pause()">
          <i-material-symbols:pause-circle-outline-rounded />
        </v-icon-button>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import type { IAudio, IFilter } from '@/lib/interfaces'
import { DataType } from '@/lib/data'
import { getFileId, getFileName, getFileUrl } from '@/lib/api/file'
import { guessDlnaMimeByName, openDlnaCastModal } from '@/lib/dlna'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { useTempStore } from '@/stores/temp'

const { t: $t } = useI18n()

const { urlTokenKey } = storeToRefs(useTempStore())

interface Props {
  item: IAudio
  filter: IFilter
  dataType: DataType
  editMode: boolean
  animatingIds: string[]
  playLoading: boolean
  playPath: string
  app: any
  // Functions passed from parent
  deleteItem: (dataType: DataType, item: IAudio) => void
  restore: (dataType: DataType, query: string) => void
  downloadFile: (path: string, fileName: string) => void
  trash: (dataType: DataType, query: string) => void
  handleRemoveFromPlaylist: (event: MouseEvent, item: IAudio) => void
  addToPlaylist: (event: MouseEvent, item: IAudio) => void
  addItemToTags: (item: IAudio) => void
  pause: () => void
  isAudioPlaying: (item: IAudio) => boolean
  isInPlaylist: (item: IAudio) => boolean
  restoreLoading: (query: string) => boolean
  trashLoading: (query: string) => boolean
}

defineProps<Props>()
</script>