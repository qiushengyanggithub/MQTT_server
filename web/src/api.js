function token() {
  return localStorage.getItem('mqtt_web_token') || ''
}

export function setToken(value) {
  if (value) localStorage.setItem('mqtt_web_token', value)
  else localStorage.removeItem('mqtt_web_token')
}

async function request(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) }
  const current = token()
  if (current) headers.Authorization = `Bearer ${current}`

  const response = await fetch(`/api${path}`, { ...options, headers })
  if (!response.ok) {
    let detail = response.statusText
    try {
      const body = await response.json()
      detail = body.error || detail
    } catch (_) {
      detail = await response.text()
    }
    throw new Error(detail || '请求失败')
  }
  return response.json()
}

export const getOverview = () => request('/overview')
export const getClients = () => request('/clients')
export const getSubscriptions = () => request('/subscriptions')
export const getRetained = () => request('/retained')

export function publishMessage(payload) {
  return request('/publish', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function streamMessages(onMessage) {
  const url = new URL('/api/messages/stream', window.location.origin)
  const current = token()
  if (current) url.searchParams.set('token', current)

  const source = new EventSource(url)
  source.addEventListener('message', (event) => {
    try {
      onMessage(JSON.parse(event.data))
    } catch (_) {
      // ignore malformed events
    }
  })
  return source
}
