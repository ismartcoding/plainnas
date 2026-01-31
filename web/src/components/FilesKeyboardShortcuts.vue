<template>
  <keyboard-shortcuts :shortcuts="shortcuts">
    <template v-if="$slots.trigger" #trigger>
      <slot name="trigger" />
    </template>
  </keyboard-shortcuts>
</template>

<script setup lang="ts">
import { filesKeyboardShortcuts } from '@/lib/shortcuts/files'

const shortcuts = filesKeyboardShortcuts
</script> 