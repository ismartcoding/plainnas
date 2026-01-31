<template>
  <v-icon-button v-tooltip="tooltip" @click.stop="emit('click')">
    <i-material-symbols:chrome-reader-mode-outline-rounded v-if="mode === 'edit'" />
    <i-material-symbols:dashboard-2-edit-outline-rounded v-else />
  </v-icon-button>
</template>

<script setup lang="ts">
type UIMode = 'view' | 'edit'

defineProps<{ mode: UIMode; tooltip: string }>()

const emit = defineEmits<{ (e: 'click'): void }>()
</script>
