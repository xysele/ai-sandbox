# AI Sandbox Gateway (Go)

为 AI agent 设计的轻量级沙箱网关，提供命令执行、文件操作、GUI 自动化、异步任务管理等能力。纯 Go 实现，无外部依赖，可一键部署到魔搭创空间等容器环境。

## 核心特性

- **无依赖**：仅用 Go 标准库，`go build` 即可得到单文件可执行程序
- **命令注入防护**：所有用户输入通过 argv 数组传参，不经过 shell 拼接
- **内存安全**：输出缓冲上限 4 MiB，文件读取上限 32 MiB，异步任务表限制 200 个
- **GUI 自动化**：基于 Xvfb + xdotool + scrot，支持无头截图、点击、键盘输入
- **异步任务**：长时命令（> 30 秒）走 `/task/create`，支持查询状态和主动取消
- **精简设计**：24 个端点，凡是一行 shell 能做的（git、进程管理、DNS）都交给 `/exec`

## 快速开始

### 本地运行

```bash
git clone <本仓库>
cd ai-sandbox-go
go build -o sandbox .
export GATEWAY_TOKEN=your_secret_token_here
./sandbox
```

服务启动在 `http://localhost:7860`。

### Docker 部署

```bash
docker build -t ai-sandbox-go .
docker run -d -p 7860:7860 \
  -e GATEWAY_TOKEN=your_secret_token_here \
  ai-sandbox-go
```

容器包含 Xvfb、fluxbox、xdotool、scrot、imagemagick、git、python3、nodejs 等工具。

### 魔搭创空间部署

1. 在创空间中新建 Dockerfile 应用，上传本仓库内容
2. 在「环境变量」面板设置 `GATEWAY_TOKEN`（必须设置，否则每次重启 token 都会变）
3. 端口**必须是 7860**（创空间强制要求）
4. 部署完成后通过 `https://<你的空间>.pai-megatron-x.com/` 访问

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `GATEWAY_TOKEN` | （随机生成） | 鉴权 token，**生产部署必须显式设置** |
| `PORT` | `7860` | 监听端口 |
| `DISPLAY` | `:99` | X server 显示编号（GUI 自动化） |
| `SCREENSHOT_DIR` | `/tmp/sandbox_screenshots` | 截图保存目录 |

**⚠️ 生产部署必须设置 `GATEWAY_TOKEN`！** 随机生成的 token 仅本进程知道，容器重启后会变化，导致调用方全部 401。

## API 端点（24 个）

### 认证

所有端点（除 `/` `/health` `/favicon.ico`）都需要在请求头中携带：

```
X-Gateway-Token: <你设置的 GATEWAY_TOKEN>
```

> 用自定义头而不是标准 `Authorization`，因为魔搭创空间的反向代理会剥掉 `Authorization` 头。

### 端点分类

#### 基础信息（2 个）
- `GET /` — 存活探针
- `GET /health` — 健康检查（返回 GUI 状态、可用运行时）

#### 命令执行（2 个）
- `POST /exec` — 执行 shell 命令（支持管道、重定向、后台模式）
- `POST /run_code` — 执行代码片段（python/bash/node/go）

#### 文件系统（7 个）
- `POST /read_file` — 读取文件（支持按行分段）
- `POST /write_file` — 写入文件（支持追加、base64 二进制）
- `POST /delete_file` — 删除文件或目录
- `POST /list_dir` — 列出目录内容
- `POST /search_files` — 递归搜索文件内容（grep）
- `POST /upload` — 上传文件（base64）
- `POST /download` — 下载文件（base64）

#### GUI 自动化（6 个）
- `POST /screenshot` — 截图（PNG 格式，base64 返回）
- `POST /click` — 模拟鼠标点击
- `POST /type_text` — 模拟键盘输入
- `POST /press_key` — 按特定按键（Return/Escape/ctrl+c 等）
- `POST /move_mouse` — 移动鼠标
- `GET /gui_status` — 查询 GUI 工具可用性

#### 异步任务（4 个）
- `POST /task/create` — 创建长时任务（超时上限 3600 秒）
- `POST /task/status` — 查询任务状态和输出
- `POST /task/cancel` — 取消运行中的任务
- `GET /task/list` — 列出所有任务

#### 系统与网络（2 个）
- `GET /system/info` — 系统信息（CPU/内存/磁盘/负载/任务数）
- `POST /http_fetch` — 发起 HTTP 请求（支持自定义头、超时）

## 安全设计

1. **命令注入防护**  
   所有用户输入（路径、pattern、环境变量值、代码片段）通过 `exec.CommandContext(argv[0], argv[1:]...)` 传递，不拼接进 shell 字符串，因此引号、反引号、`$()`、管道符都是字面量。

2. **时序攻击防护**  
   Token 比对用 `subtle.ConstantTimeCompare`，执行时间不依赖匹配位置，无法通过响应时延逐字节猜测。

3. **内存边界**  
   - 命令输出上限 4 MiB，超出部分截断
   - 文件读取/下载上限 32 MiB，超过返回 413
   - HTTP fetch 响应体上限 8 MiB
   - 任务表上限 200 个，超过后清理 30 分钟前的已结束任务

4. **路径安全**  
   拒绝删除 `/` 和 `.`；错误消息不暴露宿主机绝对路径（`statError()` 统一改写）。

5. **日志安全**  
   启动日志只报告 token **来源**（环境变量 / 随机生成），绝不打印 token 内容或前缀。

## 目录结构

```
ai-sandbox-go/
├── main.go                  # 入口
├── internal/
│   ├── config/
│   │   └── config.go        # 环境变量加载、随机 token 生成
│   ├── server/
│   │   └── server.go        # HTTP 服务器、路由、鉴权中间件
│   └── handlers/
│       ├── common.go        # 共用工具：runArgv、cappedBuffer、respondJSON
│       ├── exec.go          # /exec、/run_code
│       ├── filesystem.go    # /read_file、/write_file、/list_dir 等
│       ├── gui.go           # /screenshot、/click、/type_text 等
│       ├── tasks.go         # /task/*、/system/info
│       ├── network.go       # /http_fetch
│       └── basic.go         # /、/health、/gui_status
├── Dockerfile               # 多阶段构建（golang:1.21 → debian:bullseye-slim）
├── entrypoint.sh            # 启动 Xvfb + fluxbox + Go 服务
└── docs/
    ├── API.md               # 详细 API 文档
    ├── SKILLS.md            # Agent 技能速查（渐进式）
    └── DEPLOYMENT.md        # 部署指南
```

## 开发

```bash
# 构建
go build -o sandbox .

# 代码检查
go vet ./...
go fmt ./...
```

## 许可

MIT

