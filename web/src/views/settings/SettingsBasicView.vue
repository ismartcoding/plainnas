<template>
  <div class="top-app-bar">
    <div class="actions">
      <v-filled-button value="save" :loading="saving" @click="save">{{ t('save') }}</v-filled-button>
    </div>
  </div>

  <div class="scroll-content settings-page">
    <section class="card border-card">
      <h5 class="card-title">{{ t('device_name') }}</h5>
      <div class="card-body">
        <p class="subtle">{{ t('device_name_desc') }}</p>

        <v-text-field
v-model="deviceName" class="form-control" :label="t('device_name')" autocomplete="off"
          :placeholder="t('device_name_placeholder')" :error="(submitAttempted || dirty) && !!deviceNameError"
          :error-text="(submitAttempted || dirty) && deviceNameError ? t(deviceNameError) : ''" @keyup.enter="save" />
        <div class="settings-kv">
          <div class="settings-kv__label">{{ t('discovery_name') }}</div>
          <div class="settings-kv__value mono">{{ discoveryName || '-' }}</div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useField, useForm } from 'vee-validate'
import { string } from 'yup'
import toast from '@/components/toaster'
import { initMutation, setDeviceNameGQL } from '@/lib/api/mutation'
import { initQuery, deviceInfoGQL } from '@/lib/api/query'

const { t } = useI18n()

const { handleSubmit } = useForm()

const dirty = ref(false)
const submitAttempted = ref(false)

const { value: deviceName, errorMessage: deviceNameError, validate: validateDeviceName } = useField<string>(
  'deviceName',
  string()
    .required('device_name_invalid')
    .max(63, 'device_name_rules')
    .matches(/^[a-z0-9]+(?:-[a-z0-9]+)*$/, 'device_name_rules')
)
const currentHostname = ref('')
const syncing = ref(false)

const discoveryName = computed(() => {
  const h = (currentHostname.value || '').trim()
  if (!h) return ''
  return `${h}.local`
})

function normalizeHostnameInput(v: string) {
  // Keep frontend behavior aligned with backend validation: only allow hostname-friendly characters.
  return String(v || '')
    .trim()
    .toLowerCase()
    .replace(/[\s._]+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/-+/g, '-')
    .replace(/^-+/, '')
    .replace(/-+$/, '')
    .slice(0, 63)
}

watch(deviceName, (v) => {
  if (syncing.value) return
  dirty.value = true
  if (submitAttempted.value) validateDeviceName()
})

const { refetch: refetchDeviceInfo } = initQuery<{ deviceInfo: { hostname: string } }>({
  document: deviceInfoGQL,
  handle: (data, error) => {
    if (error) return
    const h = String(data?.deviceInfo?.hostname || '').trim()
    currentHostname.value = h
    if (!dirty.value) {
      syncing.value = true
      deviceName.value = h
      syncing.value = false
    }
  },
})

const { mutate, loading: saving } = initMutation({
  document: setDeviceNameGQL,
}, false)

function extractGraphQLErrorMessage(e: unknown) {
  const raw = String((e as any)?.message || '').trim()
  return raw.replace(/^GraphQL error:\s*/i, '').trim()
}

function extractDeviceNameErrorKey(message: string) {
  // Backend may send "device_name_xxx: details".
  const first = String(message || '').split(':', 1)[0].trim()
  return first
}

const doSave = handleSubmit(async () => {
  const name = normalizeHostnameInput(deviceName.value)
  if (!name) {
    toast(t('device_name_invalid'), 'error')
    return
  }

  try {
    await mutate({ name })
    dirty.value = false
    await refetchDeviceInfo()
    toast(t('saved'))
  } catch (e) {
    const msg = extractGraphQLErrorMessage(e)
    const code = extractDeviceNameErrorKey(msg)
    const key = code.startsWith('device_name_') ? code : 'device_name_apply_failed'
    toast(t(key), 'error')
  }
})

async function save() {
  submitAttempted.value = true
  await validateDeviceName()
  await doSave()
}
</script>
