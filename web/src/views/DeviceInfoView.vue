<template>
  <div class="scroll-content">
    <div class="grids">
      <div>
        <section class="card">
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
                  {{ item.value }}
                </template>
              </div>
            </div>
          </div>
        </section>
      </div>
      <div>
        <section class="card">
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
    </div>
  </div>
</template>

<script setup lang="ts">
import toast from '@/components/toaster'
import { initQuery, deviceInfoGQL } from '@/lib/api/query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime, formatDateTimeFull, formatSeconds, formatFileSize, formatUptime } from '@/lib/format'

const { t } = useI18n()

const basicInfos = ref<any[]>([])
const systemInfos = ref<any[]>([])

const { refetch } = initQuery({
  handle: (data: any, error: string) => {
    if (error) {
      toast(t(error), 'error')
    } else {
      const d = data.deviceInfo
      basicInfos.value = [
        { label: 'hostname', value: d.hostname },
        { label: 'os', value: `${d.os} ${d.kernelVersion} (${d.arch})` },
        { label: 'cpu', value: `${d.cpuModel} • ${d.cpuCores} Cores / ${d.cpuThreads} Threads` },
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
</script>
<style lang="scss" scoped>
.scroll-content {
  padding: 0 0 16px 0;
}

.grids {
  display: grid;
  gap: 16px;
  padding: 16px;
  grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
}

.card {
  height: 100%;
}

@media (max-width: 1200px) and (min-width: 769px) {
  .grids {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .grids {
    grid-template-columns: 1fr;
    gap: 12px;
    padding: 0 12px 12px 12px;
  }

  .scroll-content {
    padding: 0 0 12px 0;
  }
}
</style>
