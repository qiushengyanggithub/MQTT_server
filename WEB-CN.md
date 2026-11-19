# Mochi MQTT Web 控制台说明书

本文档说明基于 Vue3 + Gin 新增的 Web 管理控制台，包括启动方式、页面功能、用户权限以及常见问题。

## 1. 技术组成

- 前端：Vue 3 + Vite + Ant Design Icons
- 后端：Gin（与 MQTT Broker 同进程运行）
- 前端源码：`web/src/`
- 前端构建产物：`web/dist/`
- API 实现：`api/`

## 2. 启动与访问

在项目根目录执行：

```bash
go run ./cmd
```

默认访问地址：

- Web 控制台：`http://localhost:8081`
- MQTT TCP：`1883`
- MQTT WebSocket：`1882`
- 原有状态接口：`8080`

如果需要修改端口：

```bash
go run ./cmd -tcp :18830 -ws :18831 -info :18832 -web-addr :8081
```

可用启动参数：

| 参数 | 默认值 | 说明 |
|---|---|---|
| `-web-addr` | `:8081` | Web 控制台与 API 监听地址，设为空可关闭 |
| `-web-dist` | `web/dist` | 前端构建产物目录 |
| `-web-user` | `admin` | Web 登录用户名 |
| `-web-password` | 由服务端配置 | Web 登录密码，请向管理员获取 |
| `-web-token` | 空 | 可选的静态 Bearer Token |
| `-users-file` | `users.json` | MQTT 用户数据文件，设为空则不持久化 |

## 3. 登录

1. 打开 `http://localhost:8081` 后会先进入登录页。
2. 输入管理员提供的用户名和密码。
3. 登录成功后，会话有效期为 8 小时。
4. 点击右上角「退出」可注销当前会话。

说明：

- 除 `/api/login` 和 `/api/health` 外，所有 API 都需要登录。
- 登录状态保存在浏览器 `localStorage` 中。
- 接口返回 401 时会自动退出到登录页。
- `-web-token` 静态 Token 与登录会话兼容。

## 4. 页面功能

### 4.1 概览

展示 Broker 的核心运行指标，数据来自 `system.Info` 与服务端运行时状态：

- 在线客户端、历史峰值
- 收到/发送消息数
- 订阅数、保留消息数、飞行消息数
- 运行时间、版本
- Goroutine 数与内存分配

页面每 5 秒自动刷新一次，也可点击右上角「刷新」立即更新。

说明：表格列数会随浏览器宽度自动调整。

### 4.2 客户端

显示当前连接到 Broker 的 MQTT 客户端：

- Client ID
- 远程地址
- 监听器（TCP / WS）
- 用户名
- MQTT 协议版本
- Keepalive
- 订阅数量
- 在线状态

操作功能：

- **踢出**：断开选中的单个客户端连接
- **禁用用户**：如果该连接使用了用户管理中的账号，可直接禁用该账号，并踢出该账号的所有连接；再次连接会被认证拒绝

### 4.3 订阅

展示所有客户端当前的 Topic Filter 订阅关系：

- 所属客户端
- Topic Filter
- QoS
- 共享订阅组

### 4.4 保留消息

展示 Broker 当前保存的 Retained 消息：

- Topic
- Payload（UTF-8 文本直接显示，二进制显示为 Base64）
- QoS
- 创建时间

### 4.5 用户管理

管理 MQTT 客户端的账号、密码与 Topic ACL。

功能：

- 新增用户（必须设置密码）
- 编辑用户（密码留空表示保持原密码不变）
- 删除用户
- 禁用/启用用户
- 配置 ACL 规则

用户列表字段：

- 用户名
- 账号状态（正常/已禁用）
- 连接状态（离线/在线及连接数）
- 密码（页面直接展示，适用于内网管理场景）
- ACL 规则
- 操作按钮

ACL 权限说明：

| 权限值 | 含义 |
|---|---|
| 拒绝 | 不允许访问该 Topic |
| 只读 | 允许订阅该 Topic |
| 只写 | 允许向该 Topic 发布 |
| 读写 | 允许订阅和发布 |

规则说明：

- 没有配置 ACL 时，该用户可访问所有 Topic
- 配置了 ACL 后，按白名单方式匹配
- 禁用用户会踢出该用户所有连接，并阻止再次登录

用户数据保存在 `users.json`，可通过 `-users-file` 指定其它文件。

### 4.6 发布消息

通过服务端内置客户端（Inline Client）向 Broker 发布消息：

- Topic
- Payload
- QoS（0 / 1 / 2）
- Retain

### 4.7 实时消息

通过 SSE（Server-Sent Events）实时展示 Broker 上发生的发布消息：

- Topic
- QoS
- 是否 Retain
- 来源客户端
- 时间与 Payload

最多保留最近 100 条，可手动清空。

## 5. REST API

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/login` | 登录，返回会话 Token |
| POST | `/api/logout` | 退出登录 |
| GET | `/api/health` | 健康检查（公开） |
| GET | `/api/overview` | Broker 概览数据 |
| GET | `/api/clients` | 客户端列表 |
| DELETE | `/api/clients/:id` | 踢出客户端 |
| GET | `/api/subscriptions` | 订阅列表 |
| GET | `/api/retained` | 保留消息 |
| POST | `/api/publish` | 发布消息 |
| GET | `/api/messages/stream` | SSE 实时消息流 |
| GET | `/api/users` | 用户列表 |
| POST | `/api/users` | 新建用户 |
| PUT | `/api/users/:username` | 修改用户 |
| DELETE | `/api/users/:username` | 删除用户 |
| POST | `/api/users/:username/disable` | 禁用用户并踢出连接 |
| POST | `/api/users/:username/enable` | 启用用户 |

除 `/api/login` 和 `/api/health` 外，均需要登录会话或有效 Token。

## 6. 用户认证行为

- 没有创建任何 MQTT 用户时：Broker 允许所有客户端连接（保持原有行为）
- 创建第一个用户后：只有用户名单中的账号可以连接
- 用户密码错误、用户被禁用、未知用户：认证失败
- ACL 未配置：允许访问所有 Topic
- ACL 已配置：仅允许匹配规则的 Topic 操作

## 7. 常见问题

### 端口 1883 被占用

使用其它端口启动：

```bash
go run ./cmd -tcp :18830 -ws :18831 -info :18832 -web-addr :8081
```

### 用户管理里看不到已有用户

默认读取 `users.json`。如果已有用户文件在其它位置：

```bash
go run ./cmd -users-file path/to/users.json
```

### 修改了用户，客户端需要重连吗

需要。禁用、修改密码或 ACL 后，请让客户端重新连接以使用新的权限。

### Docker 启动看不到控制台

当前 Dockerfile 使用 `cmd/docker` 配置文件入口，不启动 Web API 和控制台。请使用 `go run ./cmd` 启动带控制台的服务。

### 前端页面修改后如何生效

```bash
cd web
npm install
npm run build
```

构建产物输出到 `web/dist/`，Go 服务会直接读取该目录。

## 8. 安全建议

- 生产环境务必通过 `-web-user` 和 `-web-password` 修改默认登录凭据
- 建议配合 HTTPS 使用，避免密码和会话 Token 明文传输
- 用户密码当前会在页面明文显示，仅建议在内网或可信环境中使用
- 如需更严格的控制，可在网关层限制 `/api` 和控制台访问来源
