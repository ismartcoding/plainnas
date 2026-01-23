<template>
  <v-modal style="width: min(500px, 90vw);" @close="close">
    <template #headline>
      {{ $t('info') }}
    </template>
    <template #content>
      <section class="card">
        <div class="key-value">
          <div class="key">{{ $t('path') }}</div>
          <div class="value">{{ item.path }}</div>
        </div>

        <div class="key-value">
          <div class="key">{{ $t('file_size') }}</div>
          <div class="value">
            <v-circular-progress v-if="loading" indeterminate class="sm" />
            <span v-else>{{ formatFileSize(Number(fileInfo?.size ?? 0)) }}</span>
          </div>
        </div>

        <div class="key-value">
          <div class="key">{{ $t('updated_at') }}</div>
          <div class="value">
            <time v-if="fileInfo?.updatedAt" v-tooltip="formatDateTimeFull(fileInfo.updatedAt)">{{ formatDateTime(fileInfo.updatedAt) }}</time>
          </div>
        </div>

        <div v-if="errorText" class="error-text">{{ $t(errorText) }}</div>
      </section>
    </template>
    <template #actions>
      <v-filled-button value="close" @click="close">{{ $t('close') }}</v-filled-button>
    </template>
  </v-modal>
</template>

<script setup lang="ts">
import { onMounted, ref, type PropType } from 'vue'
import type { IFile } from '@/lib/file'
import { initLazyQuery, fileInfoGQL } from '@/lib/api/query'
import { formatDateTime, formatDateTimeFull, formatFileSize } from '@/lib/format'
import { popModal } from '@/components/modal'

const props = defineProps({
  item: { type: Object as PropType<IFile>, required: true },
})

const item = props.item

const fileInfo = ref<any>(null)
const errorText = ref('')

const { loading, load } = initLazyQuery({
  handle: (data: any, error: string) => {
    if (error) {
      errorText.value = error
      return
    }
    fileInfo.value = data?.fileInfo ?? null
  },
  document: fileInfoGQL,
  variables: () => ({
    id: item.path,
    path: item.path,
    includeDirSize: !!item.isDir,
  }),
})

onMounted(() => {
  load()
})

function close() {
  popModal()
}
</script>

<style scoped lang="scss">
.error-text {
  margin-top: 12px;
  color: var(--md-sys-color-error, #b3261e);
}
</style>
