<template>
  <div class="settings-view">
    <h1 class="title">{{ t('settings') }}</h1>
    <section>
      <h2 class="subtitle">{{ t('media_sources') }}</h2>
      <p class="page-desc">{{ t('media_sources_desc') }}</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
const { t } = useI18n()
</script>

<style scoped>
.title {
  font-size: 1.25rem;
  font-weight: 700;
  margin: 6px 0 12px;
}
.subtitle {
  font-size: 1rem;
  font-weight: 600;
  margin: 10px 0 6px;
}
</style>
