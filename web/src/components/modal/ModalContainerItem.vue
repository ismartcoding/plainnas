<template>
  <div ref="containerRef" style="position: relative; z-index: 2">
    <component
      :is="modal?.component"
      v-bind="modal?.props.value"
      ref="modalRef"
      :modal-id="resolvedModalId"
      v-on="modal?.events"
    />
  </div>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { saveInstance } from './utils/instances'
import { modalQueue } from './utils/state'
import type Modal from './utils/Modal'

const modalRef = ref(null)
const containerRef = ref<HTMLDivElement>()

const props = defineProps({
  id: { type: Number, required: true },
})

const modal = getModalById(props.id)

const resolvedModalId = computed(() => {
  const provided = (modal?.props.value as any)?.modalId ?? (modal?.props.value as any)?.['modal-id'] ?? ''
  const tokens = `${provided || ''} _modal_${props.id}`
    .trim()
    .split(/\s+/)
    .filter(Boolean)
  return Array.from(new Set(tokens)).join(' ')
})
function getModalById(id: number | undefined): Modal | undefined {
  return modalQueue.value.find((elem) => elem.id === id)
}

watch(
  () => modalRef.value,
  (newValue) => {
    saveInstance(props.id!, newValue!)
  }
)
</script>
