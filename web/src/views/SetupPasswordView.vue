<template>
  <header class="header">
    <header-actions :logged-in="false" />
  </header>
  <h1>{{ $t('app_name') }}</h1>
    <form  class="auth-form" @submit.prevent="onSubmit">
      <div v-show="showError" class="alert alert-danger show" role="alert">
        <i-material-symbols:error-outline-rounded />
        <div class="body">
          {{ error ? $t(error) : '' }}
        </div>
      </div>
      <v-text-field
        v-model="password"
        :label="t('password')"
        type="password"
        class="form-control"
        :error="passwordError"
        autocomplete="new-password"
        :error-text="passwordError ? $t(passwordError) : ''"
        @keydown.enter="onSubmit" />
      <v-text-field
        v-model="confirmPassword"
        :label="t('samba_password_confirm')"
        type="password"
        class="form-control"
        :error="confirmError"
        autocomplete="new-password"
        :error-text="confirmError ? $t(confirmError) : ''"
        @keydown.enter="onSubmit" />
      <v-filled-button :disabled="isSubmitting" :loading="isSubmitting">
        {{ $t('save') }}
      </v-filled-button>
    </form>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useField, useForm } from 'vee-validate'
import { string } from 'yup'
import { useI18n } from 'vue-i18n'
import router from '@/plugins/router'
import { sha512, hashToKey, chachaEncrypt, chachaDecrypt, bitArrayToUint8Array, arrayBufferToBitArray } from '@/lib/api/crypto'
import { getApiBaseUrl, getApiHeaders } from '@/lib/api/api'
import { getAccurateAgent } from '@/lib/agent/agent'

const { handleSubmit, isSubmitting } = useForm()
const { t } = useI18n()
const showError = ref(false)
const error = ref('')

const { value: password, errorMessage: passwordError } = useField('password', string().required())
const { value: confirmPassword } = useField('confirmPassword', string().required())
const confirmError = ref('')

async function initSetup() {
  try {
    const r = await fetch(`${getApiBaseUrl()}/auth/status`, {
      method: 'POST',
      headers: getApiHeaders() as Record<string, string>,
    })
    if (!r.ok) return
    const json = await r.json().catch(() => null)
    if (json?.authenticated) {
      window.location.href = router.currentRoute.value.query['redirect']?.toString() ?? '/'
      return
    }
    if (!json?.needsSetup) {
      router.replace({ path: '/login', query: { redirect: router.currentRoute.value.query['redirect']?.toString() ?? '/' } })
    }
  } catch {
    // ignore
  }
}

initSetup()

const onSubmit = handleSubmit(async () => {
  const pass = String(password.value || '')
  const confirm = String(confirmPassword.value || '')
  confirmError.value = ''
  error.value = ''
  showError.value = false

  if (pass !== confirm) {
    confirmError.value = 'password_not_match'
    return
  }

  const hash = sha512(pass)

  try {
    const setupResp = await fetch(`${getApiBaseUrl()}/auth/setup`, {
      method: 'POST',
      headers: {
        ...getApiHeaders(),
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ password: hash }),
    })

    if (!setupResp.ok) {
      const body = await setupResp.json().catch(() => null)
      const msg = body?.errors?.[0]?.message
      showError.value = true
      error.value = typeof msg === 'string' && msg ? msg : 'login.failed'
      return
    }

    // Auto-login after setup.
    const key = hashToKey(hash)
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

    if (!r.ok) {
      showError.value = true
      error.value = 'login.failed'
      return
    }

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
  } catch {
    showError.value = true
    error.value = 'connection_timeout'
  }
})
</script>

<style lang="scss" scoped>
@use '@/styles/auth-shared.scss' as *;
</style>
