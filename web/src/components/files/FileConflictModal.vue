<template>
  <v-modal style="width: min(520px, 92vw);" @close="popModal">
    <template #content>
      <div class="title">{{ titleText }}</div>
      <div class="desc">{{ descText }}</div>

      <div v-if="details" class="details">{{ details }}</div>
    </template>

    <template #actions>
      <v-outlined-button @click="popModal">{{ $t('cancel') }}</v-outlined-button>

      <template v-if="mode === 'folder-folder'">
        <v-outlined-button @click="emitChoice('merge')">{{ $t('conflict.merge') }}</v-outlined-button>
        <v-filled-button @click="emitChoice('replace')">{{ $t('conflict.replace') }}</v-filled-button>
      </template>

      <template v-else-if="mode === 'file-file-single'">
        <v-outlined-button @click="emitChoice('keep_both')">{{ $t('conflict.keep_both') }}</v-outlined-button>
        <v-filled-button @click="emitChoice('replace')">{{ $t('conflict.replace') }}</v-filled-button>
      </template>

      <template v-else>
        <v-outlined-button @click="emitChoice('skip')">{{ $t('conflict.skip') }}</v-outlined-button>
        <v-outlined-button @click="emitChoice('keep_both')">{{ $t('conflict.keep_both') }}</v-outlined-button>
        <v-filled-button @click="emitChoice('replace')">{{ $t('conflict.replace') }}</v-filled-button>
      </template>
    </template>
  </v-modal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { popModal, Modal } from '@/components/modal'
import { useI18n } from 'vue-i18n'

export type ConflictMode = 'folder-folder' | 'file-file-single' | 'file-file-multiple'
export type ConflictChoice = 'merge' | 'replace' | 'keep_both' | 'skip'

const props = defineProps<{
  mode: ConflictMode
  details?: string
}>()

const emit = defineEmits<{
  (e: typeof Modal.EVENT_PROMPT, choice: ConflictChoice): void
}>()

const { t } = useI18n()

const titleText = computed(() => {
  if (props.mode === 'folder-folder') return t('conflict.folder_title')
  if (props.mode === 'file-file-single') return t('conflict.file_title')
  return t('conflict.files_title')
})

const descText = computed(() => {
  if (props.mode === 'folder-folder') return t('conflict.folder_desc')
  if (props.mode === 'file-file-single') return t('conflict.file_desc')
  return t('conflict.files_desc')
})

function emitChoice(choice: ConflictChoice) {
  emit(Modal.EVENT_PROMPT, choice)
}
</script>

<style scoped lang="scss">
.title {
  font-size: 1rem;
  font-weight: 700;
  margin-bottom: 6px;
}

.desc {
  color: var(--md-sys-color-on-surface-variant);
  margin-bottom: 10px;
}

.details {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
  font-size: 0.85rem;
  padding: 10px;
  background: color-mix(in srgb, var(--md-sys-color-surface) 85%, var(--md-sys-color-on-surface) 15%);
  border-radius: 10px;
  word-break: break-all;
}
</style>
