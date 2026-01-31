<template>
  <keyboard-shortcuts :shortcuts="shortcuts">
    <template v-if="$slots.trigger" #trigger>
      <slot name="trigger" />
    </template>
  </keyboard-shortcuts>
</template>

<script setup lang="ts">
import { mediaKeyboardShortcuts } from '@/lib/shortcuts/media'

const shortcuts = mediaKeyboardShortcuts
</script>
