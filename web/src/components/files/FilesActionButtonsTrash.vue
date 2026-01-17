<template>
  <files-keyboard-shortcuts />
  <v-icon-button v-tooltip="$t('refresh')" :loading="refreshing" @click="refreshCurrentDir">
      <i-material-symbols:refresh-rounded />
  </v-icon-button>
  <v-dropdown v-model="sortMenuVisible">
    <template #trigger>
      <v-icon-button v-tooltip="$t('sort')" :loading="sorting">
          <i-material-symbols:sort-rounded />
      </v-icon-button>
    </template>
    <div v-for="item in sortItems" :key="item.value" class="dropdown-item" :class="{ 'selected': item.value === fileSortBy }" @click="sort(item.value); sortMenuVisible = false">
      {{ $t(item.label) }}
    </div>
  </v-dropdown>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps({
  currentDir: {
    type: String,
    required: true,
  },
  refreshing: {
    type: Boolean,
    required: true,
  },
  sorting: {
    type: Boolean,
    required: true,
  },
  sortItems: {
    type: Array as () => { value: string; label: string }[],
    required: true,
  },
  fileSortBy: {
    type: String,
    required: true,
  },
})

const emit = defineEmits<{
  refreshCurrentDir: []
  sort: [value: string]
}>()

const sortMenuVisible = ref(false)

function refreshCurrentDir() {
  emit('refreshCurrentDir')
}

function sort(value: string) {
  emit('sort', value)
}
</script> 