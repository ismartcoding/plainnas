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
        <v-icon-button v-tooltip="$t('add_to_tags')" class="sm" @click.stop="addItemToTags(item)">
          <i-material-symbols:label-outline-rounded />
        </v-icon-button>
      </template>
      <template v-else>
        <v-icon-button v-tooltip="$t('download')"
          @click.stop="downloadFile(item.path, getFileName(item.path).replace(' ', '-'))">
          <i-material-symbols:download-rounded />
        </v-icon-button>
        <v-icon-button v-tooltip="$t('cast')"
          @click.stop="openDlnaCastModal({ url: getFileUrl(item.fileId), title: item.title || getFileName(item.path), mime: guessDlnaMimeByName(getFileName(item.path), dataType), type: dataType })">
          <i-material-symbols:airplay-outline-rounded />
        </v-icon-button>
      </template>
    </template>
  </div>
</template>

<script setup lang="ts">
import type { IVideoItem, IFilter } from '@/lib/interfaces'
import { DataType } from '@/lib/data'
import { getFileName, getFileUrl } from '@/lib/api/file'
import { guessDlnaMimeByName, openDlnaCastModal } from '@/lib/dlna'
import { useI18n } from 'vue-i18n'

const { t: $t } = useI18n()

interface Props {
  item: IVideoItem
  filter: IFilter
  dataType: DataType
  editMode: boolean
  app: any
  // Functions passed from parent
  deleteItem: (dataType: DataType, item: IVideoItem) => void
  restore: (dataType: DataType, query: string) => void
  downloadFile: (path: string, fileName: string) => void
  trash: (dataType: DataType, query: string) => void
  addItemToTags: (item: IVideoItem) => void
  restoreLoading: (query: string) => boolean
  trashLoading: (query: string) => boolean
}

defineProps<Props>()
</script>