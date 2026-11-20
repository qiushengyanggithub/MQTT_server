function token() {
  return localStorage.getItem('mqtt_web_token') || ''
}

export function setToken(value) {
  if (value) localStorage.setItem('mqtt_web_token', value)
  else localStorage.removeItem('mqtt_web_token')
}

export async function login(username, password) {
  const response = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (!response.ok) {
    let detail = response.statusText
    try {
      const body = await response.json()
      detail = body.error || detail
    } catch (_) {
      detail = await response.text()
    }
    throw new Error(detail || '登录失败')
  }
  return response.json()
}

export function logout() {
  return request('/logout', { method: 'POST' })
}

async function request(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) }
  const current = token()
  if (current) headers.Authorization = `Bearer ${current}`

  const response = await fetch(`/api${path}`, { ...options, headers })
  if (response.status === 401 && path !== '/login') {
    setToken('')
    window.dispatchEvent(new Event('auth-expired'))
  }
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
export const getUsers = () => request('/users')

export function createUser(payload) {
  return request('/users', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function updateUser(username, payload) {
  return request(`/users/${encodeURIComponent(username)}`, {
    method: 'PUT',
    body: JSON.stringify(payload),
  })
}

export function deleteUser(username) {
  return request(`/users/${encodeURIComponent(username)}`, { method: 'DELETE' })
}

export function kickClient(id) {
  return request(`/clients/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

export function setUserDisallowed(username, disallow) {
  const action = disallow ? 'disable' : 'enable'
  return request(`/users/${encodeURIComponent(username)}/${action}`, { method: 'POST' })
}

export function publishMessage(payload) {
  return request('/publish', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export const restartBackend = () => request('/restart', { method: 'POST' })

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
