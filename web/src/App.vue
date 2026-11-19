<script setup>
import { computed, markRaw, onMounted, onUnmounted, ref, watch } from 'vue'
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
  { id: 'retained', label: '保留消息', icon: markRaw(DatabaseOutlined), desc: 'Broker 当前保存的 Retained 消息' },
  { id: 'users', label: '用户管理', icon: markRaw(SafetyCertificateOutlined), desc: 'MQTT 连接账号与 Topic ACL 权限' },
  { id: 'publish', label: '发布消息', icon: markRaw(SendOutlined), desc: '通过服务端 Inline Client 发布消息' },
  { id: 'messages', label: '实时消息', icon: markRaw(MessageOutlined), desc: 'SSE 实时推送的最新发布消息' },
]

const loggedIn = ref(Boolean(localStorage.getItem('mqtt_web_token')))
const loginForm = ref({ username: 'admin', password: '' })
const loginError = ref('')
const loggingIn = ref(false)

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
const form = ref({ topic: 'test/topic', payload: 'hello mochi', qos: 0, retain: false })

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

function payloadText(item) {
  return item.payload || (item.payload_base64 ? `BASE64: ${item.payload_base64}` : '')
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
  } catch (e) {
    error.value = e.message || String(e)
  }
}

async function doPublish() {
  publishResult.value = ''
  try {
    await publishMessage({
      topic: form.value.topic,
      payload: form.value.payload,
      qos: Number(form.value.qos),
      retain: form.value.retain,
    })
    publishResult.value = '发布成功'
    await refresh()
  } catch (e) {
    publishResult.value = `发布失败: ${e.message || e}`
  }
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

function connectStream() {
  if (eventSource) eventSource.close()
  streamStatus.value = 'connecting'
  eventSource = streamMessages((message) => {
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
          <h1>Mochi MQTT 控制台</h1>
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
          <input v-model="loginForm.password" type="password" autocomplete="current-password" placeholder="请输入密码" />
        </label>
        <div v-if="loginError" class="error-banner"><CloseCircleOutlined /> {{ loginError }}</div>
        <button class="btn primary login-btn" type="submit" :disabled="loggingIn">
          <component :is="loggingIn ? LoadingOutlined : SafetyCertificateOutlined" />
          {{ loggingIn ? '登录中…' : '登录' }}
        </button>
      </form>

      <p class="login-hint">默认账号 admin / admin，可通过 -web-user 和 -web-password 修改</p>
    </div>
  </div>

  <div v-else class="shell">
    <header class="topbar">
      <div class="brand">
        <span class="logo"><AntDesignOutlined /></span>
        <div>
          <h1>Mochi MQTT 控制台</h1>
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
          <table>
            <thead>
              <tr>
                <th>Client ID</th>
                <th>远程地址</th>
                <th>监听器</th>
                <th>用户名</th>
                <th>MQTT 版本</th>
                <th>Keepalive</th>
                <th>订阅</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="cl in clients" :key="cl.id">
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
                  <button class="btn text" :disabled="!cl.connected" @click="kick(cl)">
                    <DisconnectOutlined /> 踢出
                  </button>
                  <button
                    v-if="cl.username && findUser(cl.username)"
                    class="btn text danger"
                    @click="toggleUserDisallow(cl)"
                  >
                    <StopOutlined /> {{ findUser(cl.username).disallow ? '启用用户' : '禁用用户' }}
                  </button>
                </td>
              </tr>
              <tr v-if="!clients.length">
                <td colspan="9" class="empty"><InboxOutlined /> 暂无客户端</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else-if="tab === 'subscriptions'">
          <table>
            <thead>
              <tr>
                <th>Client ID</th>
                <th>Topic Filter</th>
                <th>QoS</th>
                <th>共享组</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(sub, index) in subscriptions" :key="index">
                <td class="mono">{{ sub.client_id }}</td>
                <td class="mono">{{ sub.filter }}</td>
                <td>{{ sub.qos }}</td>
                <td>{{ sub.shared || '-' }}</td>
              </tr>
              <tr v-if="!subscriptions.length">
                <td colspan="4" class="empty"><InboxOutlined /> 暂无订阅</td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else-if="tab === 'retained'">
          <table>
            <thead>
              <tr>
                <th>Topic</th>
                <th>Payload</th>
                <th>QoS</th>
                <th>创建时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="item in retained" :key="item.topic">
                <td class="mono">{{ item.topic }}</td>
                <td class="mono payload">{{ payloadText(item) }}</td>
                <td>{{ item.qos }}</td>
                <td>{{ formatTime(item.created) }}</td>
              </tr>
              <tr v-if="!retained.length">
                <td colspan="4" class="empty"><InboxOutlined /> 暂无保留消息</td>
              </tr>
            </tbody>
          </table>
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

          <table class="users-table">
            <thead>
              <tr>
                <th>用户名</th>
                <th>状态</th>
                <th>连接状态</th>
                <th>密码</th>
                <th>ACL 规则</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="user in users" :key="user.username">
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
              <tr v-if="!users.length">
                <td colspan="6" class="empty"><InboxOutlined /> 暂无用户，请在上方创建，或用 -users-file 指定已有用户文件启动</td>
              </tr>
            </tbody>
          </table>
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
        </div>

        <div v-else class="messages">
          <div class="messages-head">
            <span>最近 {{ messages.length }} 条发布消息（SSE 实时推送，最多保留 100 条）</span>
            <button class="btn text" @click="messages = []"><DeleteOutlined /> 清空</button>
          </div>
          <div v-for="(msg, index) in messages" :key="index" class="message-item">
            <div class="message-meta">
              <span class="mono topic"><MessageOutlined /> {{ msg.topic }}</span>
              <span>QoS {{ msg.qos }}</span>
              <span v-if="msg.retain" class="pill ok">retain</span>
              <span>{{ msg.client_id }}</span>
              <span>{{ new Date(msg.at).toLocaleTimeString('zh-CN') }}</span>
            </div>
            <pre class="mono">{{ payloadText(msg) }}</pre>
          </div>
          <div v-if="!messages.length" class="empty large"><InboxOutlined /> 等待消息发布…</div>
        </div>
      </section>
    </main>
  </div>
</template>
