<template>
  <teleport to="body">
    <div class="v-modal-backdrop" @click="handleBackdropClick">
      <div class="v-modal-container" :class="modalClasses" :data-animate-height="animateHeight ? '1' : '0'" @click.stop>
        <div class="v-modal-sizer" :style="sizerStyle">
          <div ref="contentEl" class="v-modal-content">
            <!-- Headline slot -->
            <div v-if="$slots.headline" class="v-modal-headline">
              <slot name="headline"></slot>
            </div>

            <!-- Content slot -->
            <div v-if="$slots.content" class="v-modal-body">
              <slot name="content"></slot>
            </div>

            <!-- Actions slot -->
            <div v-if="$slots.actions" class="v-modal-actions">
              <slot name="actions"></slot>
            </div>
          </div>
        </div>
      </div>
    </div>
  </teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, onUpdated, ref } from 'vue'

interface Props {
  backgroundClose?: boolean
  modalId?: string
}

const props = withDefaults(defineProps<Props>(), {
  backgroundClose: true,
  modalId: ''
})

const modalClasses = computed(() => {
  return (props.modalId || '')
    .split(/\s+/)
    .filter(Boolean)
    .map((id) => `modal-${id}`)
})

const contentEl = ref<HTMLElement | null>(null)
const animateHeight = ref(false)
const sizerHeightPx = ref<number | null>(null)
let ro: ResizeObserver | null = null

const measuredEls = {
  headline: null as HTMLElement | null,
  body: null as HTMLElement | null,
  actions: null as HTMLElement | null,
}

const sizerStyle = computed(() => {
  if (sizerHeightPx.value == null) return {}
  return { height: `${sizerHeightPx.value}px` }
})

const measureHeight = () => {
  const el = contentEl.value
  if (!el) return

  if (!measuredEls.headline || !measuredEls.body || !measuredEls.actions) {
    measuredEls.headline = el.querySelector('.v-modal-headline') as HTMLElement | null
    measuredEls.body = el.querySelector('.v-modal-body') as HTMLElement | null
    measuredEls.actions = el.querySelector('.v-modal-actions') as HTMLElement | null
  }

  // Use intrinsic heights so we can grow/shrink even when the modal has a fixed height.
  const headlineH = measuredEls.headline?.offsetHeight ?? 0
  const actionsH = measuredEls.actions?.offsetHeight ?? 0
  const bodyH = measuredEls.body ? measuredEls.body.scrollHeight : 0

  const maxH = Math.floor(window.innerHeight * 0.9)
  const nextH = Math.min(headlineH + actionsH + bodyH, maxH)

  if (nextH <= 0) return
  if (sizerHeightPx.value === nextH) return
  sizerHeightPx.value = nextH
}

// 禁用属性自动继承，因为teleport会导致警告
defineOptions({
  inheritAttrs: false
})

const emit = defineEmits<{
  close: []
  cancel: []
}>()

// 处理背景点击
const handleBackdropClick = () => {
  if (props.backgroundClose) {
    emit('close')
  }
}

// 处理ESC键
const handleEscapeKey = (event: KeyboardEvent) => {
  if (event.key === 'Escape') {
    emit('close')
  }
}

// 阻止背景滚动
const disableBodyScroll = () => {
  document.body.style.overflow = 'hidden'
}

const enableBodyScroll = () => {
  document.body.style.overflow = ''
}

onMounted(() => {
  document.addEventListener('keydown', handleEscapeKey)
  disableBodyScroll()

    // Animate height changes by measuring intrinsic content height.
    // We clamp to 90vh; beyond that the body scrolls.
    ; (async () => {
      await nextTick()
      measuredEls.headline = null
      measuredEls.body = null
      measuredEls.actions = null
      measureHeight()
      await nextTick()
      animateHeight.value = true

      ro = new ResizeObserver(measureHeight)
      if (contentEl.value) ro.observe(contentEl.value)
      window.addEventListener('resize', measureHeight)
    })()
})

onUpdated(() => {
  // Slot content can change without resizing the observed border-box (e.g. body overflow),
  // but we still want the modal to expand/shrink to fit up to 90vh.
  measuredEls.headline = null
  measuredEls.body = null
  measuredEls.actions = null
  nextTick(measureHeight)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleEscapeKey)
  enableBodyScroll()

  window.removeEventListener('resize', measureHeight)
  if (ro && contentEl.value) ro.unobserve(contentEl.value)
  ro?.disconnect()
  ro = null
})

// 提供show方法以兼容MD Dialog的API
defineExpose({
  show: () => {
    // Vue Modal默认已经显示，这里为了API兼容性
  }
})
</script>

<style lang="scss" scoped>
.v-modal-backdrop {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  animation: fade-in 0.15s ease-out;
}

.v-modal-container {
  position: relative;
  min-width: 360px;
  max-width: 90vw;
  max-height: 90vh;
  background-color: var(--md-sys-color-surface-container-high, var(--md-sys-color-surface-variant));
  border-radius: 28px;
  box-shadow:
    0px 11px 15px -7px rgba(0, 0, 0, 0.2),
    0px 24px 38px 3px rgba(0, 0, 0, 0.14),
    0px 9px 46px 8px rgba(0, 0, 0, 0.12);
  overflow: hidden;
  animation: modal-enter 0.15s ease-out;
}

.v-modal-sizer {
  width: 100%;
  max-height: 90vh;
  overflow: hidden;
}

.v-modal-container[data-animate-height='1'] .v-modal-sizer {
  transition: height 180ms cubic-bezier(0.2, 0, 0, 1);
  will-change: height;
}

.v-modal-content {
  display: flex;
  flex-direction: column;
  height: 100%;
  max-height: 90vh;
  --outlined-field-bg: var(--md-sys-color-surface-container-high);
}

.v-modal-headline {
  padding: 24px 24px 0 24px;
  font-size: 1.375rem;
  font-weight: 500;
  line-height: 1.6;
  color: var(--md-sys-color-on-surface);
}

.v-modal-body {
  padding: 24px;
  color: var(--md-sys-color-on-surface-variant);
  line-height: 1.5;
  overflow-y: auto;
  flex: 1;
  min-height: 0;
}

.v-modal-actions {
  padding: 0 24px 24px 24px;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.v-modal-container:has(.v-modal-content[data-type="alert"]) {
  .v-modal-actions {
    justify-content: center;
  }
}

@keyframes fade-in {
  from {
    opacity: 0;
  }

  to {
    opacity: 1;
  }
}

@keyframes modal-enter {
  from {
    opacity: 0;
    transform: scale(0.8);
  }

  to {
    opacity: 1;
    transform: scale(1);
  }
}

.dark {
  .v-modal-container {
    --md-sys-color-surface-container-high: var(--md-sys-color-surface-variant);
  }
}
</style>