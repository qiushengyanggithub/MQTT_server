<script setup>
import { computed, markRaw, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import {
  AntDesignOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  ClusterOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  DeleteOutlined,
  DesktopOutlined,
  DisconnectOutlined,
  EditOutlined,
  EyeInvisibleOutlined,
  EyeOutlined,
  FilterOutlined,
  InboxOutlined,
  KeyOutlined,
  LoadingOutlined,
  LogoutOutlined,
  MessageOutlined,
  PlusOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  SendOutlined,
  StopOutlined,
  SwapOutlined,
  ThunderboltOutlined,
  UserAddOutlined,
  UserOutlined,
} from '@ant-design/icons-vue'
import {
  getOverview,
  getClients,
  getSubscriptions,
  getRetained,
  getUsers,
  createUser,
  updateUser,
  deleteUser,
  kickClient,
  setUserDisallowed,
  publishMessage,
  streamMessages,
  setToken,
  login,
  logout,
} from './api'

const tabs = [
  { id: 'overview', label: '概览', icon: markRaw(DashboardOutlined), desc: 'Broker 运行状态与核心指标' },
  { id: 'clients', label: '客户端', icon: markRaw(DesktopOutlined), desc: '当前连接的 MQTT 客户端' },
  { id: 'subscriptions', label: '订阅', icon: markRaw(FilterOutlined), desc: '客户端订阅的 Topic Filter' },
  { id: 'topics', label: 'Topic', icon: markRaw(ClusterOutlined), desc: '基于实时消息聚合的主题统计（本地采样）' },
  { id: 'retained', label: '保留消息', icon: markRaw(DatabaseOutlined), desc: 'Broker 当前保存的 Retained 消息' },
  { id: 'users', label: '用户管理', icon: markRaw(SafetyCertificateOutlined), desc: 'MQTT 连接账号与 Topic ACL 权限' },
  { id: 'publish', label: '发布消息', icon: markRaw(SendOutlined), desc: '通过服务端 Inline Client 发布消息' },
  { id: 'messages', label: '实时消息', icon: markRaw(MessageOutlined), desc: 'SSE 实时推送的最新发布消息' },
]

const loggedIn = ref(Boolean(localStorage.getItem('mqtt_web_token')))
const loginForm = ref({ username: 'admin', password: '' })
const loginError = ref('')
const loggingIn = ref(false)
const showPassword = ref(false)

const tab = ref('overview')
const overview = ref(null)
const clients = ref([])
const subscriptions = ref([])
const retained = ref([])
const messages = ref([])
const error = ref('')
const streamStatus = ref('connecting')
const tokenInput = ref(localStorage.getItem('mqtt_web_token') || '')
const publishResult = ref('')
const form = ref({ topic: 'test/topic', payload: 'hello mqtt', qos: 0, retain: false, format: 'text' })

const clientFilter = ref({ keyword: '', status: 'all' })
const subscriptionFilter = ref({ keyword: '', qos: 'all' })
const retainedFilter = ref({ keyword: '' })
const userFilter = ref({ keyword: '' })
const messageFilter = ref({ topic: '', qos: 'all', retain: 'all', format: 'auto' })
const messagesPaused = ref(false)
const pendingMessages = ref([])
const selectedClient = ref(null)
const messagesContainer = ref(null)
const autoScroll = ref(false)
const publishHistory = ref([])
const metricHistory = ref([])
const HISTORY_KEY = 'mqtt_publish_history'

const clientTable = reactive({ sortKey: 'id', asc: true, page: 1, pageSize: 10 })
const subscriptionTable = reactive({ sortKey: 'filter', asc: true, page: 1, pageSize: 10 })
const retainedTable = reactive({ sortKey: 'topic', asc: true, page: 1, pageSize: 10 })
const userTable = reactive({ sortKey: 'username', asc: true, page: 1, pageSize: 10 })

const users = ref([])
const editingUser = ref(null)
const userResult = ref('')
const clientResult = ref('')
const userForm = ref({ username: '', password: '', disallow: false, acl: [] })
const accessOptions = [
  { value: 0, label: '拒绝' },
  { value: 1, label: '只读' },
  { value: 2, label: '只写' },
  { value: 3, label: '读写' },
]

let refreshTimer = null
let eventSource = null

const activeTab = computed(() => tabs.find((item) => item.id === tab.value) || tabs[0])

const streamIcon = computed(() => {
  if (streamStatus.value === 'connected') return markRaw(CheckCircleOutlined)
  if (streamStatus.value === 'connecting') return markRaw(LoadingOutlined)
  return markRaw(CloseCircleOutlined)
})

const stats = computed(() => {
  const info = overview.value?.info || {}
  return [
    {
      label: '在线客户端',
      value: info.clients_connected ?? 0,
      hint: `峰值 ${info.clients_maximum ?? 0}`,
      icon: markRaw(DesktopOutlined),
      tone: 'blue',
    },
    {
      label: '收到消息',
      value: formatNumber(info.messages_received ?? 0),
      hint: `发送 ${formatNumber(info.messages_sent ?? 0)}`,
      icon: markRaw(SwapOutlined),
      tone: 'violet',
    },
    {
      label: '订阅数',
      value: info.subscriptions ?? 0,
      hint: `总客户端 ${info.clients_total ?? 0}`,
      icon: markRaw(ClusterOutlined),
      tone: 'cyan',
    },
    {
      label: '保留消息',
      value: info.retained ?? 0,
      hint: `飞行中 ${info.inflight ?? 0}`,
      icon: markRaw(DatabaseOutlined),
      tone: 'amber',
    },
    {
      label: '运行时间',
      value: formatDuration(info.uptime ?? 0),
      hint: `v${info.version || '-'}`,
      icon: markRaw(ClockCircleOutlined),
      tone: 'green',
    },
    {
      label: '协程数',
      value: info.threads ?? 0,
      hint: formatBytes(info.memory_alloc ?? 0),
      icon: markRaw(ThunderboltOutlined),
      tone: 'rose',
    },
  ]
})

const infoRows = computed(() => {
  const info = overview.value?.info
  if (!info) return []
  const labels = {
    version: '版本',
    started: '启动时间',
    uptime: '运行秒数',
    bytes_received: '接收字节',
    bytes_sent: '发送字节',
    clients_connected: '在线客户端',
    clients_disconnected: '离线持久会话',
    clients_maximum: '历史最大连接',
    clients_total: '客户端总数',
    messages_received: '收到消息',
    messages_sent: '发送消息',
    messages_dropped: '丢弃消息',
    retained: '保留消息',
    inflight: '飞行消息',
    inflight_dropped: '丢弃飞行消息',
    subscriptions: '订阅数',
    packets_received: '收到报文',
    packets_sent: '发送报文',
    memory_alloc: '内存分配',
    threads: '协程数',
  }
  return Object.keys(labels).map((key) => ({
    label: labels[key],
    value:
      key === 'started'
        ? formatTime(info[key])
        : key.includes('bytes') || key === 'memory_alloc'
          ? formatBytes(info[key])
          : formatNumber(info[key]),
  }))
})

const filteredClients = computed(() =>
  clients.value.filter(
    (cl) =>
      matchKeyword(clientFilter.value.keyword, cl.id, cl.username, cl.remote, cl.listener) &&
      (clientFilter.value.status === 'all' ||
        (clientFilter.value.status === 'online' ? cl.connected : !cl.connected)),
  ),
)

const filteredSubscriptions = computed(() =>
  subscriptions.value.filter(
    (sub) =>
      matchKeyword(subscriptionFilter.value.keyword, sub.client_id, sub.filter, sub.shared) &&
      (subscriptionFilter.value.qos === 'all' || String(sub.qos) === subscriptionFilter.value.qos),
  ),
)

const filteredRetained = computed(() =>
  retained.value.filter((item) =>
    matchKeyword(retainedFilter.value.keyword, item.topic, item.payload, item.payload_base64),
  ),
)

const filteredUsers = computed(() =>
  users.value.filter((user) => matchKeyword(userFilter.value.keyword, user.username)),
)

const filteredMessages = computed(() =>
  messages.value.filter(
    (msg) =>
      matchKeyword(messageFilter.value.topic, msg.topic) &&
      (messageFilter.value.qos === 'all' || String(msg.qos) === messageFilter.value.qos) &&
      (messageFilter.value.retain === 'all' ||
        (messageFilter.value.retain === 'yes' ? msg.retain : !msg.retain)),
  ),
)

const topicStats = computed(() => {
  const map = new Map()
  for (const msg of messages.value) {
    let item = map.get(msg.topic)
    if (!item) {
      item = { topic: msg.topic, count: 0, last: 0, qos: [0, 0, 0], retained: 0, lastPayload: '' }
      map.set(msg.topic, item)
    }
    item.count++
    if (msg.at > item.last) {
      item.last = msg.at
      item.lastPayload = payloadText(msg)
    }
    if (msg.qos >= 0 && msg.qos <= 2) item.qos[msg.qos]++
    if (msg.retain) item.retained++
  }
  return [...map.values()].sort((a, b) => b.count - a.count || b.last - a.last)
})

const pagedClients = computed(() => paginate(sortRows(filteredClients.value, clientTable), clientTable))
const pagedSubscriptions = computed(() =>
  paginate(sortRows(filteredSubscriptions.value, subscriptionTable), subscriptionTable),
)
const pagedRetained = computed(() => paginate(sortRows(filteredRetained.value, retainedTable), retainedTable))
const pagedUsers = computed(() => paginate(sortRows(filteredUsers.value, userTable), userTable))

const trend = computed(() => {
  const list = metricHistory.value
  const values = (key) => list.map((point) => point[key])
  return {
    count: list.length,
    clients: sparkline(values('clients')),
    messages: sparkline(values('messages')),
    memory: sparkline(values('memory')),
  }
})

watch(filteredClients, (rows) => resetPage(rows, clientTable))
watch(filteredSubscriptions, (rows) => resetPage(rows, subscriptionTable))
watch(filteredRetained, (rows) => resetPage(rows, retainedTable))
watch(filteredUsers, (rows) => resetPage(rows, userTable))
watch(filteredMessages, async () => {
  if (autoScroll.value && messagesContainer.value) {
    await nextTick()
    messagesContainer.value.scrollTop = 0
  }
})

function matchKeyword(keyword, ...values) {
  if (!keyword) return true
  const k = keyword.toLowerCase()
  return values.some((value) => String(value ?? '').toLowerCase().includes(k))
}

function sortRows(rows, state) {
  const key = state.sortKey
  const dir = state.asc ? 1 : -1
  return [...rows].sort((a, b) => {
    const av = a[key]
    const bv = b[key]
    if (av === bv) return 0
    if (av == null) return -1 * dir
    if (bv == null) return 1 * dir
    if (typeof av === 'number' && typeof bv === 'number') return (av - bv) * dir
    return String(av).localeCompare(String(bv), 'zh-CN') * dir
  })
}

function paginate(rows, state) {
  const start = (state.page - 1) * state.pageSize
  return rows.slice(start, start + state.pageSize)
}

function setSort(state, key) {
  if (state.sortKey === key) state.asc = !state.asc
  else {
    state.sortKey = key
    state.asc = true
  }
  state.page = 1
}

function sortMark(state, key) {
  if (state.sortKey !== key) return '↕'
  return state.asc ? '↑' : '↓'
}

function resetPage(rows, state) {
  const pages = Math.max(1, Math.ceil(rows.length / state.pageSize))
  if (state.page > pages) state.page = pages
}

function totalPages(rows, state) {
  return Math.max(1, Math.ceil(rows.length / state.pageSize))
}

function pageRange(rows, state) {
  if (!rows.length) return '0'
  const start = (state.page - 1) * state.pageSize + 1
  const end = Math.min(rows.length, state.page * state.pageSize)
  return `${start}-${end}`
}

function sparkline(values) {
  if (values.length < 2) return ''
  const width = 160
  const height = 40
  const min = Math.min(...values)
  const max = Math.max(...values)
  const span = max - min || 1
  return values
    .map((value, index) => {
      const x = (index / (values.length - 1)) * width
      const y = height - ((value - min) / span) * (height - 6) - 3
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}

function formatNumber(value) {
  return Number(value || 0).toLocaleString('zh-CN')
}

function formatBytes(value) {
  const n = Number(value || 0)
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = n / 1024
  let i = 0
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(1)} ${units[i]}`
}

function formatDuration(seconds) {
  const n = Number(seconds || 0)
  const days = Math.floor(n / 86400)
  const hours = Math.floor((n % 86400) / 3600)
  const minutes = Math.floor((n % 3600) / 60)
  return `${days}天 ${hours}小时 ${minutes}分`
}

function formatTime(unix) {
  if (!unix) return '-'
  return new Date(Number(unix) * 1000).toLocaleString('zh-CN')
}

function bytesFromText(text, encoding) {
  if (encoding === 'base64') {
    try {
      const binary = atob(text)
      return Uint8Array.from(binary, (char) => char.charCodeAt(0))
    } catch (_) {
      return new TextEncoder().encode(text)
    }
  }
  if (encoding === 'hex') {
    try {
      const clean = text.replace(/\s+/g, '')
      const bytes = new Uint8Array(Math.floor(clean.length / 2))
      for (let i = 0; i < bytes.length; i++) bytes[i] = parseInt(clean.slice(i * 2, i * 2 + 2), 16)
      return bytes
    } catch (_) {
      return new TextEncoder().encode(text)
    }
  }
  return new TextEncoder().encode(text || '')
}

function toHex(bytes) {
  return Array.from(bytes)
    .map((byte) => byte.toString(16).padStart(2, '0'))
    .join(' ')
}

function formatPayload(item, mode) {
  let bytes
  if (item.payload_base64) {
    try {
      const binary = atob(item.payload_base64)
      bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
    } catch (_) {
      bytes = new TextEncoder().encode(item.payload_base64)
    }
  } else {
    bytes = new TextEncoder().encode(item.payload || '')
  }

  const selected = mode || messageFilter.value.format || 'auto'
  if (selected === 'hex') return toHex(bytes)
  if (selected === 'base64') {
    let binary = ''
    bytes.forEach((byte) => {
      binary += String.fromCharCode(byte)
    })
    return btoa(binary)
  }
  const text = new TextDecoder().decode(bytes)
  if (selected === 'json') {
    try {
      return JSON.stringify(JSON.parse(text), null, 2)
    } catch (_) {
      return text
    }
  }
  return text
}

function payloadText(item) {
  return formatPayload(item, 'auto')
}

async function refresh() {
  try {
    error.value = ''
    const [o, c, s, r, u] = await Promise.all([
      getOverview(),
      getClients(),
      getSubscriptions(),
      getRetained(),
      getUsers(),
    ])
    overview.value = o
    clients.value = c
    subscriptions.value = s
    retained.value = r
    users.value = u
    const info = o?.info || {}
    metricHistory.value.push({
      at: Date.now(),
      clients: Number(info.clients_connected ?? 0),
      messages: Number(info.messages_received ?? 0),
      memory: Number(info.memory_alloc ?? 0),
    })
    if (metricHistory.value.length > 60) metricHistory.value.shift()
  } catch (e) {
    error.value = e.message || String(e)
  }
}

async function doPublish() {
  publishResult.value = ''
  const current = form.value
  if (current.format === 'json') {
    try {
      JSON.parse(current.payload)
    } catch (e) {
      publishResult.value = `发布失败: JSON 格式无效 - ${e.message}`
      return
    }
  }
  try {
    await publishMessage({
      topic: current.topic,
      payload: current.payload,
      qos: Number(current.qos),
      retain: current.retain,
      encoding: current.format === 'base64' || current.format === 'hex' ? current.format : 'utf8',
    })
    publishResult.value = '发布成功'
    publishHistory.value.unshift({ ...current, at: Date.now() })
    if (publishHistory.value.length > 20) publishHistory.value.pop()
    savePublishHistory()
    await refresh()
  } catch (e) {
    publishResult.value = `发布失败: ${e.message || e}`
  }
}

function loadPublishHistory() {
  try {
    publishHistory.value = JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]').slice(0, 20)
  } catch (_) {
    publishHistory.value = []
  }
}

function savePublishHistory() {
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(publishHistory.value.slice(0, 20)))
  } catch (_) {
    // ignore storage quota errors
  }
}

function applyPublishHistory(item) {
  form.value = { ...item }
  delete form.value.at
  if (!form.value.format) form.value.format = 'text'
}

function resetUserForm() {
  editingUser.value = null
  userResult.value = ''
  userForm.value = { username: '', password: '', disallow: false, acl: [] }
}

function editUser(user) {
  editingUser.value = user.username
  userResult.value = ''
  userForm.value = {
    username: user.username,
    password: '',
    disallow: user.disallow,
    acl: user.acl.map((item) => ({ filter: item.filter, access: item.access })),
  }
}

function addACLRule() {
  userForm.value.acl.push({ filter: '', access: 3 })
}

function removeACLRule(index) {
  userForm.value.acl.splice(index, 1)
}

async function saveUser() {
  userResult.value = ''
  const payload = {
    username: userForm.value.username.trim(),
    password: userForm.value.password,
    disallow: userForm.value.disallow,
    acl: userForm.value.acl.filter((item) => item.filter.trim() !== ''),
  }
  try {
    if (editingUser.value) {
      await updateUser(editingUser.value, payload)
      userResult.value = '用户更新成功'
    } else {
      await createUser(payload)
      userResult.value = '用户创建成功'
    }
    resetUserForm()
    await refresh()
  } catch (e) {
    userResult.value = `操作失败: ${e.message || e}`
  }
}

async function removeUser(user) {
  if (!window.confirm(`确定删除用户 ${user.username} 吗？`)) return
  userResult.value = ''
  try {
    await deleteUser(user.username)
    if (editingUser.value === user.username) resetUserForm()
    userResult.value = '用户已删除'
    await refresh()
  } catch (e) {
    userResult.value = `删除失败: ${e.message || e}`
  }
}

function findUser(username) {
  return users.value.find((item) => item.username === username) || null
}

async function kick(cl) {
  if (!window.confirm(`确定踢出客户端 ${cl.id} 吗？`)) return
  clientResult.value = ''
  try {
    await kickClient(cl.id)
    clientResult.value = `已踢出 ${cl.id}`
    await refresh()
  } catch (e) {
    clientResult.value = `踢出失败: ${e.message || e}`
  }
}

async function toggleUserDisallow(cl) {
  const user = findUser(cl.username)
  if (!user) {
    clientResult.value = '该连接没有对应的管理用户'
    return
  }
  const action = user.disallow ? '启用' : '禁用'
  if (!window.confirm(`确定${action}用户 ${user.username} 吗？禁用会同时踢出该用户所有连接。`)) return
  clientResult.value = ''
  try {
    await setUserDisallowed(user.username, !user.disallow)
    clientResult.value = `${action}用户 ${user.username} 成功`
    await refresh()
  } catch (e) {
    clientResult.value = `${action}失败: ${e.message || e}`
  }
}

function openClient(cl) {
  selectedClient.value = cl
}

function closeClient() {
  selectedClient.value = null
}

function toggleMessagesPause() {
  messagesPaused.value = !messagesPaused.value
  if (!messagesPaused.value && pendingMessages.value.length) {
    messages.value.unshift(...pendingMessages.value)
    messages.value = messages.value.slice(0, 100)
    pendingMessages.value = []
  }
}

function applyTopicFromStats(topic) {
  messageFilter.value.topic = topic
  tab.value = 'messages'
}

function connectStream() {
  if (eventSource) eventSource.close()
  streamStatus.value = 'connecting'
  eventSource = streamMessages((message) => {
    if (messagesPaused.value) {
      pendingMessages.value.unshift(message)
      if (pendingMessages.value.length > 100) pendingMessages.value.pop()
      return
    }
    messages.value.unshift(message)
    if (messages.value.length > 100) messages.value.pop()
  })
  eventSource.onopen = () => {
    streamStatus.value = 'connected'
  }
  eventSource.onerror = () => {
    streamStatus.value = 'disconnected'
  }
}

function applyToken() {
  setToken(tokenInput.value.trim())
  connectStream()
  refresh()
}

async function doLogin() {
  loginError.value = ''
  loggingIn.value = true
  try {
    const result = await login(loginForm.value.username.trim(), loginForm.value.password)
    setToken(result.token)
    loggedIn.value = true
    await refresh()
    connectStream()
  } catch (e) {
    loginError.value = e.message || String(e)
  } finally {
    loggingIn.value = false
  }
}

async function doLogout() {
  try {
    await logout()
  } catch (_) {
    // ignore network errors, clear local state anyway
  }
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  setToken('')
  loggedIn.value = false
  messages.value = []
}

function handleAuthExpired() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  loggedIn.value = false
}

watch(tab, (current) => {
  if (current === 'users' && loggedIn.value) refresh()
})

onMounted(() => {
  window.addEventListener('auth-expired', handleAuthExpired)
  loadPublishHistory()
  if (loggedIn.value) {
    refresh()
    connectStream()
  }
  refreshTimer = setInterval(() => {
    if (loggedIn.value) refresh()
  }, 5000)
})

onUnmounted(() => {
  clearInterval(refreshTimer)
  if (eventSource) eventSource.close()
  window.removeEventListener('auth-expired', handleAuthExpired)
})
</script>

<template>
  <div v-if="!loggedIn" class="login-page">
    <div class="login-card">
      <div class="login-brand">
        <span class="logo"><AntDesignOutlined /></span>
        <div>
          <h1>MqttServer 控制台</h1>
          <p>请登录后继续管理 Broker</p>
        </div>
      </div>

      <form class="login-form" @submit.prevent="doLogin">
        <label>
          <span class="field-label"><UserOutlined /> 用户名</span>
          <input v-model="loginForm.username" type="text" autocomplete="username" placeholder="请输入用户名" />
        </label>
        <label>
          <span class="field-label"><KeyOutlined /> 密码</span>
          <div class="password-field">
            <input
              v-model="loginForm.password"
              :type="showPassword ? 'text' : 'password'"
              autocomplete="current-password"
              placeholder="请输入密码"
            />
            <button
              type="button"
              class="password-toggle"
              :aria-label="showPassword ? '隐藏密码' : '显示密码'"
              @click="showPassword = !showPassword"
            >
              <component :is="showPassword ? EyeInvisibleOutlined : EyeOutlined" />
            </button>
          </div>
        </label>
        <div v-if="loginError" class="error-banner"><CloseCircleOutlined /> {{ loginError }}</div>
        <button class="btn primary login-btn" type="submit" :disabled="loggingIn">
          <component :is="loggingIn ? LoadingOutlined : SafetyCertificateOutlined" />
          {{ loggingIn ? '登录中…' : '登录' }}
        </button>
      </form>

    </div>
  </div>

  <div v-else class="shell">
    <header class="topbar">
      <div class="brand">
        <span class="logo"><AntDesignOutlined /></span>
        <div>
          <h1>MqttServer 控制台</h1>
          <p>MQTT Broker Web Dashboard</p>
        </div>
      </div>

      <div class="topbar-right">
        <div class="status" :class="streamStatus">
          <component :is="streamIcon" class="status-icon" />
          <span>实时通道：{{ streamStatus }}</span>
        </div>
        <div class="token-box">
          <KeyOutlined />
          <input
            v-model="tokenInput"
            type="password"
            placeholder="Bearer Token（可选）"
            @keyup.enter="applyToken"
          />
        </div>
        <button class="btn ghost" @click="applyToken">应用</button>
        <button class="btn primary" @click="refresh"><ReloadOutlined /> 刷新</button>
        <button class="btn ghost" @click="doLogout"><LogoutOutlined /> 退出</button>
      </div>
    </header>

    <div v-if="error" class="error-banner">
      <CloseCircleOutlined />
      <span>{{ error }}</span>
    </div>

    <main>
      <section class="stats">
        <div v-for="item in stats" :key="item.label" class="stat-card" :class="`tone-${item.tone}`">
          <span class="stat-icon"><component :is="item.icon" /></span>
          <div class="stat-body">
            <span class="stat-label">{{ item.label }}</span>
            <strong class="stat-value">{{ item.value }}</strong>
            <span class="stat-hint">{{ item.hint }}</span>
          </div>
        </div>
      </section>

      <section v-if="trend.count" class="trends">
        <div class="trend-card">
          <span class="trend-label">在线客户端 · 最近 {{ trend.count }} 次采样</span>
          <svg viewBox="0 0 160 40" preserveAspectRatio="none"><polyline :points="trend.clients" /></svg>
        </div>
        <div class="trend-card">
          <span class="trend-label">累计收到消息</span>
          <svg viewBox="0 0 160 40" preserveAspectRatio="none"><polyline :points="trend.messages" /></svg>
        </div>
        <div class="trend-card">
          <span class="trend-label">内存分配</span>
          <svg viewBox="0 0 160 40" preserveAspectRatio="none"><polyline :points="trend.memory" /></svg>
        </div>
      </section>

      <nav class="tabs">
        <button
          v-for="item in tabs"
          :key="item.id"
          :class="['tab', { active: tab === item.id }]"
          @click="tab = item.id"
        >
          <component :is="item.icon" />
          <span>{{ item.label }}</span>
          <span v-if="item.id === 'clients'" class="badge">{{ clients.length }}</span>
          <span v-if="item.id === 'subscriptions'" class="badge">{{ subscriptions.length }}</span>
          <span v-if="item.id === 'topics'" class="badge">{{ topicStats.length }}</span>
          <span v-if="item.id === 'users'" class="badge">{{ users.length }}</span>
          <span v-if="item.id === 'messages'" class="badge">{{ messages.length }}</span>
        </button>
      </nav>

      <section class="panel">
        <div class="panel-header">
          <div class="panel-title">
            <component :is="activeTab.icon" />
            <div>
              <h2>{{ activeTab.label }}</h2>
              <p>{{ activeTab.desc }}</p>
            </div>
          </div>
          <span v-if="clientResult" class="hint">{{ clientResult }}</span>
        </div>

        <div v-if="tab === 'overview'" class="info-grid">
          <div v-for="row in infoRows" :key="row.label" class="info-cell">
            <span>{{ row.label }}</span>
            <strong>{{ row.value }}</strong>
          </div>
        </div>

        <div v-else-if="tab === 'clients'">
          <div class="filter-bar">
            <input v-model="clientFilter.keyword" type="text" placeholder="搜索 Client ID / 用户名 / IP / 监听器" />
            <select v-model="clientFilter.status">
              <option value="all">全部状态</option>
              <option value="online">在线</option>
              <option value="offline">离线</option>
            </select>
            <span class="hint">共 {{ filteredClients.length }} 个客户端</span>
          </div>
          <table>
            <thead>
              <tr>
                <th class="sortable" @click="setSort(clientTable, 'id')">Client ID {{ sortMark(clientTable, 'id') }}</th>
                <th class="sortable" @click="setSort(clientTable, 'remote')">远程地址 {{ sortMark(clientTable, 'remote') }}</th>
                <th class="sortable" @click="setSort(clientTable, 'listener')">监听器 {{ sortMark(clientTable, 'listener') }}</th>
                <th class="sortable" @click="setSort(clientTable, 'username')">用户名 {{ sortMark(clientTable, 'username') }}</th>
                <th class="sortable" @click="setSort(clientTable, 'protocol_version')">MQTT 版本 {{ sortMark(clientTable, 'protocol_version') }}</th>
                <th class="sortable" @click="setSort(clientTable, 'keepalive')">Keepalive {{ sortMark(clientTable, 'keepalive') }}</th>
                <th>订阅</th>
                <th class="sortable" @click="setSort(clientTable, 'connected')">状态 {{ sortMark(clientTable, 'connected') }}</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="cl in pagedClients" :key="cl.id" class="clickable" @click="openClient(cl)">
                <td class="mono">{{ cl.id }}</td>
                <td>{{ cl.remote || '-' }}</td>
                <td>{{ cl.listener || '-' }}</td>
                <td>{{ cl.username || '-' }}</td>
                <td>v{{ cl.protocol_version }}</td>
                <td>{{ cl.keepalive }}s</td>
                <td>{{ cl.subscriptions.length }}</td>
                <td>
                  <span :class="['pill', cl.connected ? 'ok' : 'off']">
                    <component :is="cl.connected ? CheckCircleOutlined : CloseCircleOutlined" />
                    {{ cl.connected ? '在线' : '离线' }}
                  </span>
                </td>
                <td class="actions">
                  <button class="btn text" :disabled="!cl.connected" @click.stop="kick(cl)">
                    <DisconnectOutlined /> 踢出
                  </button>
                  <button
                    v-if="cl.username && findUser(cl.username)"
                    class="btn text danger"
                    @click.stop="toggleUserDisallow(cl)"
                  >
                    <StopOutlined /> {{ findUser(cl.username).disallow ? '启用用户' : '禁用用户' }}
                  </button>
                </td>
              </tr>
              <tr v-if="!pagedClients.length">
                <td colspan="9" class="empty"><InboxOutlined /> 暂无客户端</td>
              </tr>
            </tbody>
          </table>
          <div class="pagination">
            <button class="btn text" :disabled="clientTable.page <= 1" @click="clientTable.page--">上一页</button>
            <span>第 {{ clientTable.page }} / {{ totalPages(filteredClients, clientTable) }} 页 · {{ pageRange(filteredClients, clientTable) }} / {{ filteredClients.length }} 条</span>
            <button class="btn text" :disabled="clientTable.page >= totalPages(filteredClients, clientTable)" @click="clientTable.page++">下一页</button>
            <select v-model.number="clientTable.pageSize" @change="clientTable.page = 1">
              <option :value="10">10 条/页</option>
              <option :value="20">20 条/页</option>
              <option :value="50">50 条/页</option>
            </select>
          </div>
        </div>

        <div v-else-if="tab === 'subscriptions'">
          <div class="filter-bar">
            <input v-model="subscriptionFilter.keyword" type="text" placeholder="搜索 Client ID / Topic Filter / 共享组" />
            <select v-model="subscriptionFilter.qos">
              <option value="all">全部 QoS</option>
              <option value="0">QoS 0</option>
              <option value="1">QoS 1</option>
              <option value="2">QoS 2</option>
            </select>
            <span class="hint">共 {{ filteredSubscriptions.length }} 条订阅</span>
          </div>
          <table>
            <thead>
              <tr>
                <th class="sortable" @click="setSort(subscriptionTable, 'client_id')">Client ID {{ sortMark(subscriptionTable, 'client_id') }}</th>
                <th class="sortable" @click="setSort(subscriptionTable, 'filter')">Topic Filter {{ sortMark(subscriptionTable, 'filter') }}</th>
                <th class="sortable" @click="setSort(subscriptionTable, 'qos')">QoS {{ sortMark(subscriptionTable, 'qos') }}</th>
                <th class="sortable" @click="setSort(subscriptionTable, 'shared')">共享组 {{ sortMark(subscriptionTable, 'shared') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(sub, index) in pagedSubscriptions" :key="index">
                <td class="mono">{{ sub.client_id }}</td>
                <td class="mono">{{ sub.filter }}</td>
                <td>{{ sub.qos }}</td>
                <td>{{ sub.shared || '-' }}</td>
              </tr>
              <tr v-if="!pagedSubscriptions.length">
                <td colspan="4" class="empty"><InboxOutlined /> 暂无订阅</td>
              </tr>
            </tbody>
          </table>
          <div class="pagination">
            <button class="btn text" :disabled="subscriptionTable.page <= 1" @click="subscriptionTable.page--">上一页</button>
            <span>第 {{ subscriptionTable.page }} / {{ totalPages(filteredSubscriptions, subscriptionTable) }} 页 · {{ pageRange(filteredSubscriptions, subscriptionTable) }} / {{ filteredSubscriptions.length }} 条</span>
            <button class="btn text" :disabled="subscriptionTable.page >= totalPages(filteredSubscriptions, subscriptionTable)" @click="subscriptionTable.page++">下一页</button>
            <select v-model.number="subscriptionTable.pageSize" @change="subscriptionTable.page = 1">
              <option :value="10">10 条/页</option>
              <option :value="20">20 条/页</option>
              <option :value="50">50 条/页</option>
            </select>
          </div>
        </div>

        <div v-else-if="tab === 'topics'">
          <div class="filter-bar">
            <input v-model="messageFilter.topic" type="text" placeholder="搜索 Topic（同时过滤实时消息）" />
            <span class="hint">基于最近 {{ messages.length }} 条实时消息本地聚合，刷新页面后清空</span>
          </div>
          <table>
            <thead>
              <tr>
                <th>Topic</th>
                <th>消息数</th>
                <th>QoS 0/1/2</th>
                <th>Retain</th>
                <th>最后时间</th>
                <th>最后 Payload</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in topicStats" :key="item.topic" class="clickable" @click="applyTopicFromStats(item.topic)">
                <td class="mono">{{ item.topic }}</td>
                <td>{{ item.count }}</td>
                <td>{{ item.qos[0] }} / {{ item.qos[1] }} / {{ item.qos[2] }}</td>
                <td>{{ item.retained }}</td>
                <td>{{ formatTime(item.last / 1000) }}</td>
                <td class="mono payload">{{ item.lastPayload }}</td>
              </tr>
              <tr v-if="!topicStats.length">
                <td colspan="6" class="empty"><InboxOutlined /> 暂无消息数据</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else-if="tab === 'retained'">
          <div class="filter-bar">
            <input v-model="retainedFilter.keyword" type="text" placeholder="搜索 Topic / Payload" />
            <span class="hint">共 {{ filteredRetained.length }} 条保留消息</span>
          </div>
          <table>
            <thead>
              <tr>
                <th class="sortable" @click="setSort(retainedTable, 'topic')">Topic {{ sortMark(retainedTable, 'topic') }}</th>
                <th>Payload</th>
                <th class="sortable" @click="setSort(retainedTable, 'qos')">QoS {{ sortMark(retainedTable, 'qos') }}</th>
                <th class="sortable" @click="setSort(retainedTable, 'created')">创建时间 {{ sortMark(retainedTable, 'created') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in pagedRetained" :key="item.topic">
                <td class="mono">{{ item.topic }}</td>
                <td class="mono payload">{{ payloadText(item) }}</td>
                <td>{{ item.qos }}</td>
                <td>{{ formatTime(item.created) }}</td>
              </tr>
              <tr v-if="!pagedRetained.length">
                <td colspan="4" class="empty"><InboxOutlined /> 暂无保留消息</td>
              </tr>
            </tbody>
          </table>
          <div class="pagination">
            <button class="btn text" :disabled="retainedTable.page <= 1" @click="retainedTable.page--">上一页</button>
            <span>第 {{ retainedTable.page }} / {{ totalPages(filteredRetained, retainedTable) }} 页 · {{ pageRange(filteredRetained, retainedTable) }} / {{ filteredRetained.length }} 条</span>
            <button class="btn text" :disabled="retainedTable.page >= totalPages(filteredRetained, retainedTable)" @click="retainedTable.page++">下一页</button>
            <select v-model.number="retainedTable.pageSize" @change="retainedTable.page = 1">
              <option :value="10">10 条/页</option>
              <option :value="20">20 条/页</option>
              <option :value="50">50 条/页</option>
            </select>
          </div>
        </div>

        <div v-else-if="tab === 'users'" class="users-view">
          <div class="user-form-card">
            <div class="user-form-title">
              <component :is="editingUser ? EditOutlined : UserAddOutlined" />
              <strong>{{ editingUser ? `编辑用户 ${editingUser}` : '新增用户' }}</strong>
            </div>
            <p class="hint">
              数据来源：启动参数 <code>-users-file</code> 指定的 JSON 文件（默认 <code>users.json</code>），进入本页会自动加载已有用户。
            </p>

            <div class="user-form-grid">
              <label>
                <span class="field-label"><UserOutlined /> 用户名</span>
                <input v-model="userForm.username" type="text" placeholder="例如 device_01" />
              </label>
              <label>
                <span class="field-label"><KeyOutlined /> 密码</span>
                <input
                  v-model="userForm.password"
                  type="password"
                  :placeholder="editingUser ? '留空则保持不变' : '必填'"
                />
              </label>
              <label class="checkbox">
                <input v-model="userForm.disallow" type="checkbox" />
                禁用该用户
              </label>
            </div>

            <div class="acl-editor">
              <div class="acl-editor-head">
                <span class="field-label"><SafetyCertificateOutlined /> Topic ACL</span>
                <button class="btn text" @click="addACLRule"><PlusOutlined /> 添加规则</button>
              </div>
              <div v-for="(rule, index) in userForm.acl" :key="index" class="acl-row">
                <input v-model="rule.filter" type="text" placeholder="例如 devices/#" />
                <select v-model.number="rule.access">
                  <option v-for="option in accessOptions" :key="option.value" :value="option.value">
                    {{ option.label }}
                  </option>
                </select>
                <button class="btn text danger" @click="removeACLRule(index)"><DeleteOutlined /></button>
              </div>
              <p v-if="!userForm.acl.length" class="hint">
                未配置 ACL 时，该用户可访问所有 Topic。
              </p>
            </div>

            <div class="form-actions">
              <button class="btn primary" @click="saveUser">
                <component :is="editingUser ? EditOutlined : PlusOutlined" />
                {{ editingUser ? '保存修改' : '创建用户' }}
              </button>
              <button v-if="editingUser" class="btn" @click="resetUserForm">取消</button>
              <span
                :class="[
                  'result',
                  userResult.startsWith('操作失败') || userResult.startsWith('删除失败') ? 'failed' : '',
                ]"
              >{{ userResult }}</span>
            </div>
          </div>

          <div class="filter-bar">
            <input v-model="userFilter.keyword" type="text" placeholder="搜索用户名" />
            <span class="hint">共 {{ filteredUsers.length }} 个用户</span>
          </div>
          <table class="users-table">
            <thead>
              <tr>
                <th class="sortable" @click="setSort(userTable, 'username')">用户名 {{ sortMark(userTable, 'username') }}</th>
                <th>状态</th>
                <th>连接状态</th>
                <th>密码</th>
                <th>ACL 规则</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="user in pagedUsers" :key="user.username">
                <td class="mono">{{ user.username }}</td>
                <td>
                  <span :class="['pill', user.disallow ? 'off' : 'ok']">
                    <component :is="user.disallow ? CloseCircleOutlined : CheckCircleOutlined" />
                    {{ user.disallow ? '已禁用' : '正常' }}
                  </span>
                </td>
                <td>
                  <span :class="['pill', user.online ? 'ok' : 'off']">
                    <component :is="user.online ? CheckCircleOutlined : CloseCircleOutlined" />
                    {{ user.online ? `在线 · ${user.connections} 个连接` : '离线' }}
                  </span>
                </td>
                <td class="mono">{{ user.password || '未设置' }}</td>
                <td>
                  <div v-if="user.acl.length" class="acl-tags">
                    <span v-for="(rule, i) in user.acl" :key="i" class="acl-tag">
                      {{ rule.filter }} · {{ rule.accessName }}
                    </span>
                  </div>
                  <span v-else class="hint">全部 Topic</span>
                </td>
                <td class="actions">
                  <button class="btn text" @click="editUser(user)"><EditOutlined /> 编辑</button>
                  <button class="btn text danger" @click="removeUser(user)"><DeleteOutlined /> 删除</button>
                </td>
              </tr>
              <tr v-if="!pagedUsers.length">
                <td colspan="6" class="empty"><InboxOutlined /> 暂无用户，请在上方创建，或用 -users-file 指定已有用户文件启动</td>
              </tr>
            </tbody>
          </table>
          <div class="pagination">
            <button class="btn text" :disabled="userTable.page <= 1" @click="userTable.page--">上一页</button>
            <span>第 {{ userTable.page }} / {{ totalPages(filteredUsers, userTable) }} 页 · {{ pageRange(filteredUsers, userTable) }} / {{ filteredUsers.length }} 条</span>
            <button class="btn text" :disabled="userTable.page >= totalPages(filteredUsers, userTable)" @click="userTable.page++">下一页</button>
            <select v-model.number="userTable.pageSize" @change="userTable.page = 1">
              <option :value="10">10 条/页</option>
              <option :value="20">20 条/页</option>
              <option :value="50">50 条/页</option>
            </select>
          </div>
        </div>

        <div v-else-if="tab === 'publish'" class="publish-form">
          <label>
            <span class="field-label"><SendOutlined /> Topic</span>
            <input v-model="form.topic" type="text" placeholder="例如 devices/1/temperature" />
          </label>
          <label>
            <span class="field-label"><MessageOutlined /> Payload</span>
            <textarea v-model="form.payload" rows="5" />
          </label>
          <div class="form-row">
            <label>
              <span class="field-label"><ThunderboltOutlined /> QoS</span>
              <select v-model.number="form.qos">
                <option :value="0">0 - 最多一次</option>
                <option :value="1">1 - 至少一次</option>
                <option :value="2">2 - 恰好一次</option>
              </select>
            </label>
            <label>
              <span class="field-label">Payload 格式</span>
              <select v-model="form.format">
                <option value="text">文本（UTF-8）</option>
                <option value="json">JSON（会校验格式）</option>
                <option value="base64">Base64</option>
                <option value="hex">Hex</option>
              </select>
            </label>
            <label class="checkbox">
              <input v-model="form.retain" type="checkbox" />
              Retain（保留消息）
            </label>
          </div>
          <div class="form-actions">
            <button class="btn primary" @click="doPublish"><SendOutlined /> 发布</button>
            <span :class="['result', publishResult.startsWith('发布失败') ? 'failed' : '']">
              <CheckCircleOutlined v-if="publishResult === '发布成功'" />
              <CloseCircleOutlined v-else-if="publishResult" />
              {{ publishResult }}
            </span>
          </div>
          <div v-if="publishHistory.length" class="publish-history">
            <div class="history-head">
              <strong>发布历史（本地保存，点击回填）</strong>
              <button class="btn text danger" @click="publishHistory = []; savePublishHistory()"><DeleteOutlined /> 清空</button>
            </div>
            <div
              v-for="(item, index) in publishHistory"
              :key="index"
              class="history-item"
              @click="applyPublishHistory(item)"
            >
              <span class="mono">{{ item.topic }}</span>
              <span class="mono">{{ (item.payload || '').slice(0, 60) }}{{ (item.payload || '').length > 60 ? '…' : '' }}</span>
              <span>QoS {{ item.qos }}</span>
              <span>{{ item.format || 'text' }}</span>
              <span>{{ new Date(item.at).toLocaleString('zh-CN') }}</span>
            </div>
          </div>
        </div>

        <div v-else class="messages">
          <div class="filter-bar">
            <input v-model="messageFilter.topic" type="text" placeholder="过滤 Topic" />
            <select v-model="messageFilter.qos">
              <option value="all">全部 QoS</option>
              <option value="0">QoS 0</option>
              <option value="1">QoS 1</option>
              <option value="2">QoS 2</option>
            </select>
            <select v-model="messageFilter.retain">
              <option value="all">全部消息</option>
              <option value="yes">仅 Retain</option>
              <option value="no">非 Retain</option>
            </select>
            <select v-model="messageFilter.format">
              <option value="auto">Payload 自动</option>
              <option value="text">Payload 文本</option>
              <option value="json">Payload JSON</option>
              <option value="base64">Payload Base64</option>
              <option value="hex">Payload Hex</option>
            </select>
            <button class="btn" @click="toggleMessagesPause">{{ messagesPaused ? '继续接收' : '暂停接收' }}</button>
            <label class="checkbox">
              <input v-model="autoScroll" type="checkbox" />
              自动滚动
            </label>
            <span class="hint">显示 {{ filteredMessages.length }} / {{ messages.length }} 条{{ messagesPaused ? ` · 已缓存 ${pendingMessages.length} 条` : '' }}</span>
            <button class="btn text danger" @click="messages = []; pendingMessages = []"><DeleteOutlined /> 清空</button>
          </div>
          <div class="messages-head">
            <span>SSE 实时推送，最多保留 100 条</span>
          </div>
          <div ref="messagesContainer" class="messages-list">
            <div v-for="(msg, index) in filteredMessages" :key="index" class="message-item">
              <div class="message-meta">
                <span class="mono topic"><MessageOutlined /> {{ msg.topic }}</span>
                <span>QoS {{ msg.qos }}</span>
                <span v-if="msg.retain" class="pill ok">retain</span>
                <span>{{ msg.client_id }}</span>
                <span>{{ new Date(msg.at).toLocaleTimeString('zh-CN') }}</span>
              </div>
              <pre class="mono">{{ formatPayload(msg, messageFilter.format) }}</pre>
            </div>
            <div v-if="!filteredMessages.length" class="empty large"><InboxOutlined /> 等待消息发布…</div>
          </div>
        </div>
      </section>
    </main>

    <div v-if="selectedClient" class="drawer-mask" @click="closeClient">
      <aside class="drawer" @click.stop>
        <div class="drawer-head">
          <div>
            <h3>{{ selectedClient.id }}</h3>
            <p>客户端详情</p>
          </div>
          <button class="btn text" @click="closeClient">关闭</button>
        </div>
        <div class="drawer-body">
          <div class="info-grid">
            <div class="info-cell"><span>状态</span><strong>{{ selectedClient.connected ? '在线' : '离线' }}</strong></div>
            <div class="info-cell"><span>远程地址</span><strong>{{ selectedClient.remote || '-' }}</strong></div>
            <div class="info-cell"><span>监听器</span><strong>{{ selectedClient.listener || '-' }}</strong></div>
            <div class="info-cell"><span>用户名</span><strong>{{ selectedClient.username || '-' }}</strong></div>
            <div class="info-cell"><span>MQTT 版本</span><strong>v{{ selectedClient.protocol_version }}</strong></div>
            <div class="info-cell"><span>Keepalive</span><strong>{{ selectedClient.keepalive }}s</strong></div>
            <div class="info-cell"><span>Clean Session</span><strong>{{ selectedClient.clean ? '是' : '否' }}</strong></div>
            <div class="info-cell"><span>Inflight</span><strong>{{ selectedClient.inflight }}</strong></div>
          </div>
          <h4 class="drawer-subtitle">订阅（{{ selectedClient.subscriptions.length }}）</h4>
          <table>
            <thead>
              <tr><th>Topic Filter</th><th>QoS</th><th>共享组</th></tr>
            </thead>
            <tbody>
              <tr v-for="(sub, index) in selectedClient.subscriptions" :key="index">
                <td class="mono">{{ sub.filter }}</td>
                <td>{{ sub.qos }}</td>
                <td>{{ sub.shared || '-' }}</td>
              </tr>
              <tr v-if="!selectedClient.subscriptions.length">
                <td colspan="3" class="empty"><InboxOutlined /> 暂无订阅</td>
              </tr>
            </tbody>
          </table>
          <div class="form-actions">
            <button class="btn text danger" :disabled="!selectedClient.connected" @click="kick(selectedClient)">
              <DisconnectOutlined /> 踢出客户端
            </button>
            <button
              v-if="selectedClient.username && findUser(selectedClient.username)"
              class="btn"
              @click="toggleUserDisallow(selectedClient)"
            >
              <StopOutlined /> {{ findUser(selectedClient.username).disallow ? '启用用户' : '禁用用户' }}
            </button>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>
