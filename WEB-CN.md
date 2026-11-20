# MqttServer Web 控制台说明书

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
- 原有状态接口：`8082`

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
5. 点击右上角「重启后端」并确认后，会优雅关闭并重新启动 Broker 与 Web 服务；所有 MQTT 连接会断开，当前登录会话失效，服务恢复后页面自动跳回登录页。

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
- 在线客户端 / 累计消息 / 内存分配的简易趋势图（前端采样，最近 60 个点）

页面每 5 秒自动刷新一次，也可点击右上角「刷新」立即更新。趋势数据仅保存在浏览器内存中，刷新页面后重新累计。

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
- 支持按 Client ID / 用户名 / IP / 监听器搜索，按在线状态过滤
- 支持列排序、分页（10 / 20 / 50 条每页）
- 点击客户端行可打开详情抽屉，查看完整信息与订阅，并快捷踢出 / 禁用用户

### 4.3 订阅

展示所有客户端当前的 Topic Filter 订阅关系：

- 所属客户端
- Topic Filter
- QoS
- 共享订阅组

支持按 Client ID / Topic Filter / 共享组搜索，按 QoS 过滤，列排序与分页。

### 4.4 Topic 聚合视图

基于实时消息流在浏览器本地聚合，展示最近消息的 Topic 统计：

- Topic 消息数、QoS 0/1/2 分布、Retain 数量
- 最后消息时间与 Payload 预览
- 点击 Topic 可跳转到「实时消息」并自动按该 Topic 过滤

该视图只统计当前页面收到的实时消息，最多 100 条，刷新页面后清空，不依赖后端接口。

### 4.5 保留消息

展示 Broker 当前保存的 Retained 消息：

- Topic
- Payload（UTF-8 文本直接显示，二进制显示为 Base64）
- QoS
- 创建时间

支持按 Topic / Payload 搜索、列排序与分页。

### 4.6 用户管理

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

用户列表支持按用户名搜索、排序与分页。

用户数据保存在 `users.json`，可通过 `-users-file` 指定其它文件。

### 4.7 发布消息

通过服务端内置客户端（Inline Client）向 Broker 发布消息：

- Topic
- Payload
- QoS（0 / 1 / 2）
- Retain
- Payload 格式：文本（UTF-8）/ JSON（发布前校验）/ Base64 / Hex
- 发布历史：最近 20 条保存在浏览器本地，点击可回填表单

Base64 与 Hex 由服务端解码为原始字节后发布，接口字段为 `encoding`（`utf8` / `base64` / `hex`）。

### 4.8 实时消息

通过 SSE（Server-Sent Events）实时展示 Broker 上发生的发布消息：

- Topic
- QoS
- 是否 Retain
- 来源客户端
- 时间与 Payload

最多保留最近 100 条，可手动清空。

新增能力：

- 按 Topic 关键字、QoS、Retain 过滤
- Payload 显示格式切换：自动 / 文本 / JSON / Base64 / Hex
- 暂停 / 继续接收（暂停期间的消息缓存，继续后合并）
- 自动滚动开关

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
| POST | `/api/restart` | 重启后端服务（需登录） |
| GET | `/api/messages/stream` | SSE 实时消息流 |
| GET | `/api/users` | 用户列表 |
| POST | `/api/users` | 新建用户 |
| PUT | `/api/users/:username` | 修改用户 |
| DELETE | `/api/users/:username` | 删除用户 |
| POST | `/api/users/:username/disable` | 禁用用户并踢出连接 |
| POST | `/api/users/:username/enable` | 启用用户 |

`POST /api/publish` 的 JSON 字段：`topic`、`payload`、`qos`、`retain`、`encoding`。`encoding` 支持 `utf8`（默认）、`base64`、`hex`。

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

构建产物输出到 `web/dist/`，Go 服务默认直接读取该目录。如需编译成单二进制（前端内嵌），核心命令如下，详见 [BUILD-CN.md](BUILD-CN.md) 第 4.4 节：

```cmd
cd web && npm install && npm run build
cd ..
go build -tags embedweb -o mqtt-server-all-in-one.exe ./cmd
```

## 8. 安全建议

- 生产环境务必通过 `-web-user` 和 `-web-password` 修改默认登录凭据
- 建议配合 HTTPS 使用，避免密码和会话 Token 明文传输
- 用户密码当前会在页面明文显示，仅建议在内网或可信环境中使用
- 如需更严格的控制，可在网关层限制 `/api` 和控制台访问来源
- `/api/restart` 属于高权限操作，当前所有已登录 Web 用户均可调用，请谨慎保管管理员凭据
