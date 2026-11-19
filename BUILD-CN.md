# Mochi MQTT Web 控制台编译说明

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
│   └── dist/                # 前端构建产物（Go 服务运行时读取）
├── WEB-CN.md                # Web 控制台说明书
├── BUILD-CN.md              # 本文档
└── go.mod
```

> 注意：Go 二进制不会把前端打包进去，部署时需要同时携带 `web/dist/`。

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

## 5. 交叉编译 Debian 12 / AMD64

在 Windows 上生成 Linux AMD64 运行文件。

### 5.1 CMD 写法

```cmd
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -trimpath -o mqtt-server-linux-amd64 ./cmd
```

### 5.2 PowerShell 写法

```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -trimpath -o mqtt-server-linux-amd64 ./cmd
```

### 5.3 可选：减小二进制体积

```bash
go build -trimpath -ldflags="-s -w" -o mqtt-server-linux-amd64 ./cmd
```

### 5.4 参数说明

| 参数 | 说明 |
|---|---|
| `GOOS=linux` | 目标操作系统为 Linux |
| `GOARCH=amd64` | 目标架构为 x86_64 |
| `CGO_ENABLED=0` | 禁用 CGO，避免依赖目标机器的 glibc / gcc |
| `-trimpath` | 去除本地路径信息 |
| `-ldflags="-s -w"` | 去掉符号表和调试信息，缩小体积 |

输出文件：

```text
mqtt-server-linux-amd64
```

## 6. Debian 12 部署

### 6.1 需要复制的文件

```text
mqtt-server-linux-amd64
web/dist/
```

建议目录结构：

```text
/opt/mqtt/
├── mqtt-server-linux-amd64
└── web/
    └── dist/
        ├── index.html
        └── assets/
```

### 6.2 启动

```bash
cd /opt/mqtt
chmod +x mqtt-server-linux-amd64
./mqtt-server-linux-amd64 -web-dist ./web/dist
```

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
Description=Mochi MQTT Server with Web Console
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/mqtt
ExecStart=/opt/mqtt/mqtt-server-linux-amd64 -web-dist /opt/mqtt/web/dist -users-file /opt/mqtt/users.json
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

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
- 如需在 Docker 中使用控制台，需要把构建目标改为 `./cmd`，并把前端构建产物打入镜像

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

说明 Go 服务没找到 `web/dist`。请确认：

1. 已执行 `npm run build`
2. 启动时 `-web-dist` 指向正确目录
3. `web/dist/index.html` 确实存在

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

然后重启 Go 服务即可，后端会直接从 `web/dist` 读取最新文件。
