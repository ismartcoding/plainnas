<template>
  <div class="scroll-content settings-page">
  <section class="card border-card">
    <h5 class="card-title">{{ $t('device') }}</h5>
    <div class="card-body">
      <div v-for="(item, index) in basicInfos" :key="index" class="key-value">
        <div class="key">{{ $t(item.label) }}</div>
        <div class="value">
          <time v-if="item.isTime" v-tooltip="formatDateTimeFull(item.value)" class="time">{{
            formatDateTime(item.value) }} </time>
          <template v-else-if="Array.isArray(item.value)">
            <div v-for="(it, i) in item.value" :key="i">{{ it }}</div>
          </template>
          <template v-else>
            <div v-if="item.label === 'version'" class="version-row">
              <span>{{ item.value }}</span>
              <v-outlined-button
                v-if="appUpdate?.hasUpdate"
                class="btn-sm"
                v-tooltip="upgradeTooltip"
                @click="openUpgrade">
                {{ $t('upgrade') }}
              </v-outlined-button>
            </div>
            <template v-else>
              {{ item.value }}
            </template>
          </template>
        </div>
      </div>
    </div>
  </section>
  <section class="card border-card">
    <h5 class="card-title">{{ $t('system') }}</h5>
    <div class="card-body">
      <div v-for="(item, index) in systemInfos" :key="index" class="key-value">
        <div class="key">{{ $t(item.label) }}</div>
        <div class="value">
          <time v-if="item.isTime" v-tooltip="formatDateTimeFull(item.value)" class="time">{{
            formatDateTime(item.value)
          }}</time>
          <template v-else-if="Array.isArray(item.value)">
            <div v-for="(it, i) in item.value" :key="i">{{ it }}</div>
          </template>
          <template v-else>
            {{ item.value }}
          </template>
        </div>
      </div>
    </div>
  </section>
</div>
</template>

<script setup lang="ts">
import toast from '@/components/toaster'
import { initQuery, appUpdateGQL, deviceInfoGQL } from '@/lib/api/query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime, formatDateTimeFull, formatFileSize, formatUptime } from '@/lib/format'

const { t } = useI18n()

const basicInfos = ref<any[]>([])
const systemInfos = ref<any[]>([])
const appUpdate = ref<any | null>(null)

const upgradeTooltip = ref('')

function openUpgrade() {
  const url = appUpdate.value?.url || 'https://github.com/ismartcoding/plainnas/releases/latest'
  window.open(url, '_blank', 'noopener')
}

const { refetch } = initQuery({
  handle: (data: any, error: string) => {
    if (error) {
      toast(t(error), 'error')
    } else {
      const d = data.deviceInfo
      basicInfos.value = [
        { label: 'hostname', value: d.hostname },
        { label: 'version', value: d.appFullVersion },
        { label: 'os', value: `${d.os} ${d.kernelVersion} (${d.arch})` },
        { label: 'model', value: d.model },
        { label: 'cpu', value: d.cpuModel ? `${d.cpuModel} • ${d.cpuCores} Cores / ${d.cpuThreads} Threads` : `${d.cpuCores} Cores / ${d.cpuThreads} Threads` },
        { label: 'ip_addresses', value: d.ips },
        { label: 'nics', value: d.nics.map((n: any) => `${n.name}: ${n.mac.toUpperCase()} (${n.speedRate > 0 ? formatFileSize(n.speedRate) + '/s' : 'Unknown'})`) },
      ]

      systemInfos.value = [
        { label: 'load_average', value: `${d.load1.toFixed(2)}, ${d.load5.toFixed(2)}, ${d.load15.toFixed(2)}` },
        { label: 'uptime', value: formatUptime(d.uptime / 1000) },
        { label: 'memory_total', value: formatFileSize(d.memoryTotalBytes) },
        { label: 'memory_free', value: formatFileSize(d.memoryFreeBytes) },
        { label: 'swap', value: `${formatFileSize(d.swapUsedBytes)} / ${formatFileSize(d.swapTotalBytes)}` },
        { label: 'swap_free', value: formatFileSize(d.swapFreeBytes) },
      ]
    }
  },
  document: deviceInfoGQL,
})

initQuery({
  handle: (data: any, _error: string) => {
    const u = data?.appUpdate
    appUpdate.value = u || null
    if (u?.hasUpdate && u?.latestVersion) {
      upgradeTooltip.value = `${t('new_version_available')}: ${u.latestVersion}`
    } else {
      upgradeTooltip.value = ''
    }
  },
  document: appUpdateGQL,
})
</script>
<style lang="scss" scoped>
.scroll-content {
  padding-block-start: 16px;
}

.version-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
</style>
