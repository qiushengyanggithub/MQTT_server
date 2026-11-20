# MqttServer Web 控制台编译说明

本文档说明如何编译前端、后端，以及生成 Debian 12 / AMD64 架构的可运行文件。

## 1. 环境要求

| 组件 | 版本要求 | 说明 |
|---|---|---|
| Go | 1.21 及以上 | `go.mod` 中声明的最低版本 |
| Node.js | 18 / 20 / 22 | 用于构建 Vue3 前端 |
| npm | 随 Node.js 安装 | 前端包管理 |
| Docker | 可选 | 仅构建镜像时需要 |

检查版本：

```bash
go version
node -v
npm -v
```

当前项目已验证环境：

- Go 1.26.x
- Node.js 21.x
- npm 10.x

## 2. 目录结构

```text
MQTT_server/
├── cmd/
│   ├── main.go              # 带 Web 控制台的服务入口
│   └── docker/main.go       # Docker 配置文件入口，不启动 Web 控制台
├── api/                     # Gin API、用户管理、认证 Hook
├── web/
│   ├── src/                 # Vue3 前端源码
│   └── dist/                # 前端构建产物（可被 Go 内嵌，也可运行时读取）
├── WEB-CN.md                # Web 控制台说明书
├── BUILD-CN.md              # 本文档
└── go.mod
```

> 注意：默认构建不会把前端打包进去，部署时需同时携带 `web/dist/`；使用 `-tags embedweb` 可编译成单文件。

## 3. 编译前端

在 `web/` 目录执行：

```bash
cd web
npm install
npm run build
```

构建结果：

```text
web/dist/
├── index.html
└── assets/
    ├── index-*.css
    └── index-*.js
```

开发模式（Vite 热更新，API 自动代理到 8081）：

```bash
cd web
npm run dev
```

前端开发服务默认地址：`http://localhost:5173`

## 4. 本地编译与运行

### 4.1 Windows 原生编译

```cmd
go build -o mqtt-server-windows-amd64.exe ./cmd
```

运行：

```cmd
mqtt-server-windows-amd64.exe
```

### 4.2 Linux / macOS 原生编译

```bash
go build -o mqtt-server ./cmd
chmod +x mqtt-server
./mqtt-server
```

### 4.3 直接运行源码

```bash
go run ./cmd
```

### 4.4 单二进制构建（前端内嵌）

先按第 3 节构建前端（`-tags embedweb` 编译时要求 `web/dist/` 已存在），再编译后端。

完整核心命令（Windows CMD，最常用）：

```cmd
cd web && npm install && npm run build
cd ..
go build -tags embedweb -o mqtt-server-all-in-one.exe ./cmd
```

如果前端已经构建好，只需重新编译后端：

```cmd
go build -tags embedweb -o mqtt-server-all-in-one.exe ./cmd
```

或 Linux / macOS：

```bash
go build -tags embedweb -o mqtt-server-all-in-one ./cmd
```

原理：启用 `embedweb` 构建标签后，`web/dist` 会通过 `go:embed` 编译进二进制；运行时优先使用内嵌文件，`-web-dist` 参数在该模式下被忽略。这样部署时只需要一个可执行文件（用户数据文件 `users.json` 等仍按需携带）。

交叉编译成 Debian 12 / AMD64 单文件（先 `npm run build`）：

```cmd
cd /d d:\GO\MQTT_server&&set GOOS=linux&&set GOARCH=amd64&&set CGO_ENABLED=0&&go build -tags embedweb -trimpath -o mqtt-server-linux-amd64 ./cmd
```

> Windows CMD 里如果写成 `set GOOS=linux && ...`（`&&` 前留空格），变量值会带上尾随空格，导致 `unsupported GOOS/GOARCH pair linux /amd64`。请像上面一样紧贴 `&&`，或者分行写。

## 5. 交叉编译 Debian 12 / AMD64

在 Windows 上生成 Linux AMD64 运行文件。默认生成的是**前端内嵌的单文件**（`-tags embedweb`），部署时无需携带 `web/dist/`。

### 5.1 CMD 写法

```cmd
cd /d d:\GO\MQTT_server&&set GOOS=linux&&set GOARCH=amd64&&set CGO_ENABLED=0&&go build -tags embedweb -trimpath -o mqtt-server-linux-amd64 ./cmd
```

> `set GOOS=linux` 后不要留空格再接 `&&`，否则 GOOS 会变成 `linux `（带尾随空格），报 `unsupported GOOS/GOARCH pair linux /amd64`。

### 5.2 PowerShell 写法

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -tags embedweb -trimpath -o mqtt-server-linux-amd64 ./cmd
```

### 5.3 可选：减小二进制体积

```bash
go build -tags embedweb -trimpath -ldflags="-s -w" -o mqtt-server-linux-amd64 ./cmd
```

### 5.4 参数说明

| 参数 | 说明 |
|---|---|
| `GOOS=linux` | 目标操作系统为 Linux |
| `GOARCH=amd64` | 目标架构为 x86_64 |
| `CGO_ENABLED=0` | 禁用 CGO，避免依赖目标机器的 glibc / gcc |
| `-tags embedweb` | 把 `web/dist` 前端编进二进制，生成单文件 |
| `-trimpath` | 去除本地路径信息 |
| `-ldflags="-s -w"` | 去掉符号表和调试信息，缩小体积 |

输出文件：

```text
mqtt-server-linux-amd64
```

## 6. Debian 12 部署

### 6.1 需要复制的文件

默认构建（运行时读取前端）：

```text
mqtt-server-linux-amd64
web/dist/
```

单二进制构建（`-tags embedweb`）：

```text
mqtt-server-linux-amd64
```

建议目录结构（单二进制）：

```text
/opt/mqtt/
├── mqtt-server-linux-amd64
└── users.json          # 按需携带
```

### 6.2 启动

```bash
cd /opt/mqtt
chmod +x mqtt-server-linux-amd64
./mqtt-server-linux-amd64
```

> 单二进制模式无需 `-web-dist`；默认构建仍需指定 `-web-dist ./web/dist`（或使用默认的相对路径且工作目录正确）。

默认端口：

| 端口 | 用途 |
|---|---|
| 1883 | MQTT TCP |
| 1882 | MQTT WebSocket |
| 8082 | 状态接口 |
| 8081 | Web 控制台与 REST API |

端口冲突时：

```bash
./mqtt-server-linux-amd64 \
  -tcp :18830 \
  -ws :18831 \
  -info :18832 \
  -web-addr :8081
```

### 6.3 systemd 服务示例

创建 `/etc/systemd/system/mqtt-server.service`：

```ini
[Unit]
Description=MqttServer with Web Console
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/mqtt
ExecStart=/opt/mqtt/mqtt-server-linux-amd64 -users-file /opt/mqtt/users.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

> 上面示例对应单二进制构建；若使用默认构建，请把 `ExecStart` 改为：
> `ExecStart=/opt/mqtt/mqtt-server-linux-amd64 -web-dist /opt/mqtt/web/dist -users-file /opt/mqtt/users.json`

启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable mqtt-server
sudo systemctl start mqtt-server
sudo systemctl status mqtt-server
```

## 7. Docker 镜像

当前 Dockerfile 使用的是 `cmd/docker` 配置入口：

```bash
docker build -t mochi:latest .
docker run -p 1883:1883 -p 1882:1882 -p 8082:8082 -v $(pwd)/config.yaml:/config.yaml mochi:latest
```

注意：

- 该入口按 `config.yaml` 启动
- 当前 Dockerfile **不启动 Web 控制台**
- 如需在 Docker 中使用控制台，可把构建目标改为 `./cmd` 并加上 `-tags embedweb`（需先执行 `npm run build`），这样前端会编译进二进制，无需在镜像中携带 `web/dist`

## 8. 常用启动参数

| 参数 | 默认值 | 说明 |
|---|---|---|
| `-tcp` | `:1883` | MQTT TCP 监听地址 |
| `-ws` | `:1882` | MQTT WebSocket 监听地址 |
| `-info` | `:8082` | 状态接口监听地址 |
| `-web-addr` | `:8081` | Web 控制台地址，设为空可关闭 |
| `-web-dist` | `web/dist` | 前端构建产物目录 |
| `-web-user` | `admin` | Web 登录用户名 |
| `-web-password` | 服务端配置 | Web 登录密码，请向管理员获取 |
| `-web-token` | 空 | 可选静态 Bearer Token |
| `-users-file` | `users.json` | MQTT 用户数据文件，设为空则不持久化 |

查看全部参数：

```bash
./mqtt-server-linux-amd64 -h
```

## 9. 常见问题

### 9.1 打开页面提示“Frontend not built”

说明 Go 服务没找到前端文件。请确认：

1. 默认构建：已执行 `npm run build`，且 `-web-dist` 指向正确目录，`web/dist/index.html` 确实存在
2. 单二进制构建：编译前已执行 `npm run build`，且使用了 `-tags embedweb`（编译时会检查 `web/dist` 是否存在）

### 9.2 Debian 上权限不足

```bash
chmod +x mqtt-server-linux-amd64
```

### 9.3 端口被占用

```bash
ss -tulnp | grep -E '1883|1882|8081|8082'
```

然后用对应参数换端口。

### 9.4 交叉编译报 CGO 错误

确认设置了：

```bash
set CGO_ENABLED=0
```

本项目使用纯 Go 实现依赖，禁用 CGO 后可生成无需系统 C 库的可执行文件。

### 9.5 修改前端后如何生效

```bash
cd web
npm run build
```

如果使用默认构建，重启 Go 服务即可，后端会直接从 `web/dist` 读取最新文件；如果使用单二进制模式，需要重新执行 `go build -tags embedweb ...` 生成新的可执行文件。

### 9.6 报 `unsupported GOOS/GOARCH pair linux /amd64`

这是 Windows CMD 中 `set GOOS=linux && ...` 的空格问题：`&&` 前留空格会把尾随空格写进变量，变成 `linux `。改成紧贴写法：

```cmd
set GOOS=linux&&set GOARCH=amd64&&set CGO_ENABLED=0&&go build -tags embedweb -trimpath -o mqtt-server-linux-amd64 ./cmd
```

或者把每个 `set` 单独写一行。
