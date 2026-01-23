<template>
  <header class="header">
    <header-actions :logged-in="false" />
  </header>
  <h1>{{ $t('app_name') }}</h1>
  <form class="auth-form" @submit.prevent="onSubmit">
    <div v-show="showError" class="alert alert-danger show" role="alert">
      <i-material-symbols:error-outline-rounded />
      <div class="body">
        {{ error ? $t(error) : '' }}
      </div>
    </div>
    <v-text-field
v-model="password" :label="t('password')" type="password" class="form-control"
      :error="passwordError" autocomplete="current-password" :error-text="passwordError ? $t(passwordError) : ''"
      @keydown.enter="onSubmit" />
    <v-filled-button :disabled="isSubmitting" :loading="isSubmitting">
      {{ $t(isSubmitting ? 'logging_in' : 'log_in') }}
    </v-filled-button>
  </form>
  <div v-if="showWarning" class="tips">{{ $t('browser_warning') }}</div>
</template>
<script setup lang="ts">
import { ref } from 'vue'
import { useField, useForm } from 'vee-validate'
import { string } from 'yup'
import { useI18n } from 'vue-i18n'
import router from '@/plugins/router'
import { sha512, hashToKey, chachaEncrypt, chachaDecrypt, bitArrayToUint8Array, arrayBufferToBitArray } from '@/lib/api/crypto'
import { getApiBaseUrl, getApiHeaders } from '@/lib/api/api'
import { randomUUID } from '@/lib/strutil'
import { tokenToKey } from '@/lib/api/file'
const { handleSubmit, isSubmitting } = useForm()
const showError = ref(false)
const error = ref('')
const showWarning = window.location.protocol === 'http:' ? false : !(window.navigator as any).userAgentData
const { t } = useI18n()
const { value: password, errorMessage: passwordError } = useField('password', string().required())
import { getAccurateAgent } from '@/lib/agent/agent'

async function initRequest() {
  const token = localStorage.getItem('auth_token') ?? ''
  const options: RequestInit & { headers: Record<string, string> } = {
    method: 'POST',
    headers: getApiHeaders() as Record<string, string>,
  }

  if (token) {
    const uuid = randomUUID()
    const key = tokenToKey(token)
    const enc = chachaEncrypt(key, uuid)
    options.body = bitArrayToUint8Array(enc)
  }

  const r = await fetch(`${getApiBaseUrl()}/auth/status`, options)
  if (!r.ok) {
    return
  }
  const json = await r.json().catch(() => null)
  if (json?.needsSetup) {
    router.replace({
      path: '/setup-password',
      query: { redirect: router.currentRoute.value.query['redirect']?.toString() ?? '/' },
    })
    return
  }
  if (json?.authenticated) {
    window.location.href = router.currentRoute.value.query['redirect']?.toString() ?? '/'
  }
}

initRequest()

const onSubmit = handleSubmit(async () => {
  const pass = (password.value as string) ?? ''
  const hash = sha512(pass)
  const key = hashToKey(hash)
  error.value = ''
  showError.value = false
  isSubmitting.value = true
  try {
    const ua = await getAccurateAgent()
    const enc = chachaEncrypt(
      key,
      JSON.stringify({
        password: hash,
        browserName: ua.browser.name,
        browserVersion: ua.browser.version,
        osName: ua.os.name,
        osVersion: ua.os.version,
        isMobile: ua.isMobile,
      })
    )

    const r = await fetch(`${getApiBaseUrl()}/auth`, {
      method: 'POST',
      headers: {
        ...getApiHeaders(),
        'Content-Type': 'application/octet-stream',
      },
      body: bitArrayToUint8Array(enc),
    })
    if (r.ok) {
      const buf = await r.arrayBuffer()
      const decrypted = chachaDecrypt(key, arrayBufferToBitArray(buf))
      const json = JSON.parse(decrypted)
      if (json && json.token) {
        localStorage.setItem('auth_token', json.token)
        window.location.href = router.currentRoute.value.query['redirect']?.toString() ?? '/'
        return
      }
      showError.value = true
      error.value = 'login.failed'
    } else {
      showError.value = true
      error.value = 'login.failed'
    }
  } catch (ex) {
    showError.value = true
    error.value = 'login.failed'
  } finally {
    isSubmitting.value = false
  }
})

</script>

<style lang="scss" scoped>
@use '@/styles/auth-shared.scss' as *;

.auth-form {
  .tap-phone {
    text-align: center;
    padding-block-end: 1rem;

    *:is(svg) {
      width: 120px;
      margin-inline-start: 24px;
      fill: var(--md-sys-color-primary);
    }
  }

  .tap-phone-text {
    text-align: center;
  }
}
</style>
