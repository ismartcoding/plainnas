<template>
  <left-sidebar class="settings-sidebar">
    <template #body>
      <ul class="nav">
        <li :class="{ active: isActive('basic') }" @click.prevent="go('basic')">
          <span class="icon" aria-hidden="true">
            <i-lucide:settings />
          </span>
          <span class="title">{{ t('basic_settings') }}</span>
        </li>

        <li :class="{ active: isActive('media-sources') }" @click.prevent="go('media-sources')">
          <span class="icon" aria-hidden="true">
            <i-lucide:folder-cog />
          </span>
          <span class="title">{{ t('media_sources') }}</span>
        </li>

        <li :class="{ active: isActive('lan-share') }" @click.prevent="go('lan-share')">
          <span class="icon" aria-hidden="true">
            <i-lucide:share2 />
          </span>
          <span class="title">{{ t('lan_share') }}</span>
        </li>

        <li :class="{ active: isActive('device-info') }" @click.prevent="go('device-info')">
          <span class="icon" aria-hidden="true">
            <i-lucide:smartphone />
          </span>
          <span class="title">{{ t('device_info') }}</span>
        </li>
      </ul>
    </template>
  </left-sidebar>
</template>

<script setup lang="ts">
import LeftSidebar from '@/components/LeftSidebar.vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { replacePath } from '@/plugins/router'
import { useMainStore } from '@/stores/main'

const { t } = useI18n()
const route = useRoute()
const mainStore = useMainStore()

function isActive(key: string) {
  const p = route.path
  return p === `/settings/${key}` || (key === 'basic' && p === '/settings')
}

function go(key: string) {
  replacePath(mainStore, `/settings/${key}`)
}
</script>
