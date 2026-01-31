<template>

  <template v-if="uiMode === 'edit'">
    <v-icon-button v-tooltip="t('create_folder')" @click="onCreateDir">
      <i-material-symbols:create-new-folder-outline-rounded />
    </v-icon-button>

    <v-dropdown v-model="uploadMenu">
      <template #trigger>
        <v-icon-button v-tooltip="t('upload')">
          <i-material-symbols:upload-rounded />
        </v-icon-button>
      </template>
      <div class="dropdown-item" @click.stop="onUploadFilesClick(); uploadMenu = false">
        {{ t('upload_files') }}
      </div>
      <div class="dropdown-item" @click.stop="onUploadDirClick(); uploadMenu = false">
        {{ t('upload_folder') }}
      </div>
    </v-dropdown>

    <v-icon-button v-if="canPaste" v-tooltip="t('paste')" :loading="pasting" @click="onPaste">
      <i-material-symbols:content-paste-rounded />
    </v-icon-button>
  </template>

  <UIModeToggleButton :mode="uiMode" :tooltip="uiMode === 'edit' ? t('view') : t('edit')" @click="onToggleUiMode" />

  <v-icon-button v-tooltip="t('refresh')" :loading="refreshing" @click="onRefresh">
    <i-material-symbols:refresh-rounded />
  </v-icon-button>

  <v-dropdown v-model="moreMenu">
    <template #trigger>
      <v-icon-button v-tooltip="t('actions')">
        <i-material-symbols:sort-rounded />
      </v-icon-button>
    </template>

    <div class="dropdown-item" @click.stop="onOpenKeyboardShortcuts(); moreMenu = false">
      {{ t('keyboard_shortcuts') }}
    </div>

    <div class="dropdown-item" :class="{ 'selected': showHidden }" @click.stop="onToggleShowHidden(); moreMenu = false">
      {{ t('search_filter_show_hidden') }}
    </div>

    <div v-for="item in sortItems" :key="item.value" class="dropdown-item"
      :class="{ 'selected': item.value === sortBy }" @click.stop="onSort(item.value); moreMenu = false">
      {{ t(item.label) }}
    </div>
  </v-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import UIModeToggleButton from '@/components/UIModeToggleButton.vue'

type UIMode = 'view' | 'edit'

type SortItem = { label: string; value: string }

const props = defineProps<{
  uiMode: UIMode
  currentDir: string

  uploadMenuVisible: boolean
  moreMenuVisible: boolean

  canPaste: boolean
  pasting: boolean
  refreshing: boolean

  showHidden: boolean
  sortBy: string
  sortItems: SortItem[]

  onToggleUiMode: () => void
  onCreateDir: () => void
  onUploadFiles: (dir: string) => void
  onUploadDir: (dir: string) => void
  onPaste: () => void
  onRefresh: () => void
  onOpenKeyboardShortcuts: () => void
  onToggleShowHidden: () => void
  onSort: (value: string) => void
}>()

const emit = defineEmits<{
  (e: 'update:uploadMenuVisible', value: boolean): void
  (e: 'update:moreMenuVisible', value: boolean): void
}>()

const { t } = useI18n()

const uploadMenu = computed({
  get: () => props.uploadMenuVisible,
  set: (value: boolean) => emit('update:uploadMenuVisible', value),
})

const moreMenu = computed({
  get: () => props.moreMenuVisible,
  set: (value: boolean) => emit('update:moreMenuVisible', value),
})

function onUploadFilesClick() {
  props.onUploadFiles(props.currentDir)
}

function onUploadDirClick() {
  props.onUploadDir(props.currentDir)
}
</script>
