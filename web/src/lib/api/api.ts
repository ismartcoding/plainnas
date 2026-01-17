export function getApiHost() {
  return import.meta.env.VITE_APP_API_HOST || window.location.host
}

export function getApiHeaders() {
  return {
    'Content-Type': 'multipart/form-data',
    'c-id': localStorage.getItem('client_id') ?? '',
  }
}

export function getWebSocketUrl() {
  const p = window.location.protocol === 'http:' ? 'ws' : 'wss'
  return `${p}://${getApiHost()}/ws`
}

export function getApiBaseUrl() {
  return `${window.location.protocol}\/\/${getApiHost()}`
}
