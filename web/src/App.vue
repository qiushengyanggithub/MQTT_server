<script setup>
import { computed, markRaw, onMounted, onUnmounted, ref } from 'vue'
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
  FilterOutlined,
  InboxOutlined,
  KeyOutlined,
  LoadingOutlined,
  MessageOutlined,
  ReloadOutlined,
  SendOutlined,
  SwapOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons-vue'
import {
  getOverview,
  getClients,
  getSubscriptions,
  getRetained,
  publishMessage,
  streamMessages,
  setToken,
} from './api'

const tabs = [
  { id: 'overview', label: '概览', icon: markRaw(DashboardOutlined), desc: 'Broker 运行状态与核心指标' },
  { id: 'clients', label: '客户端', icon: markRaw(DesktopOutlined), desc: '当前连接的 MQTT 客户端' },
  { id: 'subscriptions', label: '订阅', icon: markRaw(FilterOutlined), desc: '客户端订阅的 Topic Filter' },
  { id: 'retained', label: '保留消息', icon: markRaw(DatabaseOutlined), desc: 'Broker 当前保存的 Retained 消息' },
  { id: 'publish', label: '发布消息', icon: markRaw(SendOutlined), desc: '通过服务端 Inline Client 发布消息' },
  { id: 'messages', label: '实时消息', icon: markRaw(MessageOutlined), desc: 'SSE 实时推送的最新发布消息' },
]

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
    const [o, c, s, r] = await Promise.all([
      getOverview(),
      getClients(),
      getSubscriptions(),
      getRetained(),
    ])
    overview.value = o
    clients.value = c
    subscriptions.value = s
    retained.value = r
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

onMounted(() => {
  refresh()
  connectStream()
  refreshTimer = setInterval(refresh, 5000)
})

onUnmounted(() => {
  clearInterval(refreshTimer)
  if (eventSource) eventSource.close()
})
</script>

<template>
  <div class="shell">
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
              </tr>
              <tr v-if="!clients.length">
                <td colspan="8" class="empty"><InboxOutlined /> 暂无客户端</td>
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
