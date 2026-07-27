# AI Sandbox Gateway

AI Sandbox Gateway 是一个面向自动化程序和 Agent 的 HTTP 网关。它把容器里的命令执行、文件读写、长任务、网络请求、浏览器和虚拟桌面操作封装成 JSON API。

服务端使用 Go 标准库实现，没有第三方 Go 依赖。完整运行环境由 Docker 镜像提供，其中包含 Python、Node.js、Playwright、Xvfb、xdotool、git、curl 等常用工具。

这个项目提供的是“进入容器执行操作的入口”，不是虚拟机或容器隔离方案。拿到 `GATEWAY_TOKEN` 的调用方，实际上拥有容器内的远程代码执行权限。生产部署前应先阅读[安全边界](#安全边界)。

## 提供的能力

- 执行 shell 命令，支持管道、重定向、环境变量和工作目录
- 运行 Python、Bash、Node.js 或 Go 代码片段
- 读取、写入、搜索、上传和下载文件
- 创建可查询、可取消的后台任务
- 从容器内部发起 HTTP 请求
- 通过 Xvfb 和 xdotool 操作虚拟桌面
- 通过 Playwright 执行单次网页操作
- 将多个固定的 API 操作合并为一次请求

## 快速开始

### 直接运行

需要 Go 1.21 或更高版本：

```bash
go build -o ai-sandbox-go .
GATEWAY_TOKEN='replace-with-a-long-random-token' ./ai-sandbox-go
```

服务默认监听 `0.0.0.0:7860`。直接运行 Go 程序只会启动网关；GUI、浏览器和代码运行能力取决于本机是否安装了对应工具。

先检查服务状态：

```bash
curl -sS http://127.0.0.1:7860/health
```

再执行一个需要鉴权的请求：

```bash
curl -sS http://127.0.0.1:7860/exec \
  -H 'X-Gateway-Token: replace-with-a-long-random-token' \
  -H 'Content-Type: application/json' \
  --data '{"command":"printf \"hello from sandbox\\n\""}'
```

正常响应如下：

```json
{
  "exit_code": 0,
  "stderr": "",
  "stdout": "hello from sandbox\n",
  "success": true
}
```

### Docker

Docker 镜像包含项目预期的完整工具集，包括 Go 工具链和支持 CGO 的基础编译工具：

```bash
docker build -t ai-sandbox-go .

docker run --name ai-sandbox-go --rm \
  -p 7860:7860 \
  -e GATEWAY_TOKEN='replace-with-a-long-random-token' \
  ai-sandbox-go
```

如需保留工作文件，可以显式挂载一个目录：

```bash
mkdir -p workspace

docker run --name ai-sandbox-go --rm \
  -p 7860:7860 \
  -e GATEWAY_TOKEN='replace-with-a-long-random-token' \
  -v "$PWD/workspace:/workspace" \
  ai-sandbox-go
```

挂载后，网关可以修改或删除 `workspace` 中的宿主机文件。不要挂载整个主目录、仓库根目录或其他不需要暴露的路径。

## 配置

| 环境变量 | 程序默认值 | 说明 |
| --- | --- | --- |
| `GATEWAY_TOKEN` | 启动时随机生成 | API 鉴权令牌。实际部署必须显式设置 |
| `PORT` | `7860` | HTTP 监听端口 |
| `DISPLAY` | `:99` | GUI 操作使用的 X display |
| `SCREENSHOT_DIR` | `/tmp/sandbox_screenshots` | 虚拟桌面截图的保存目录 |

Docker 镜像将 `SCREENSHOT_DIR` 设置为 `/tmp/cs_screenshots`。`entrypoint.sh` 会启动分辨率为 `1280x800` 的 Xvfb，并固定使用 `DISPLAY=:99`。

如果没有设置 `GATEWAY_TOKEN`，程序会生成一个临时令牌，但不会把令牌内容写入日志。调用方无法得知该令牌，而且容器重启后令牌会变化。这个行为只用于避免服务无鉴权启动，不适合作为正常配置。

## 鉴权和请求约定

`/` 和 `/health` 是公开端点。其他 API 请求都要携带：

```http
X-Gateway-Token: <GATEWAY_TOKEN>
```

项目使用自定义请求头，是因为部分托管平台的反向代理不会向应用转发 `Authorization`。Token 通过常量时间比较进行校验。

JSON 请求还应设置：

```http
Content-Type: application/json
```

多数响应都有 `success` 字段。需要同时检查 HTTP 状态码和响应体，因为命令退出码非零时，`/exec` 仍会返回 HTTP 200，但响应中的 `success` 为 `false`。`/http_fetch` 的 `success` 表示网关成功完成请求，不代表目标站点返回了 2xx；目标状态码在 `status_code` 字段中。

## 常用调用

下面的示例统一使用两个环境变量：

```bash
export SANDBOX_URL='http://127.0.0.1:7860'
export GATEWAY_TOKEN='replace-with-a-long-random-token'
```

### 执行命令

`/exec` 通过 `sh -lc` 运行命令，支持完整 shell 语法：

```bash
curl -sS "$SANDBOX_URL/exec" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{
    "command": "find . -type f | sort | head",
    "cwd": "/workspace",
    "env": {"LANG": "C.UTF-8"},
    "timeout": 30
  }'
```

请求字段：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `command` | 是 | 交给 `sh -lc` 的命令 |
| `cwd` | 否 | 工作目录 |
| `env` | 否 | 追加到进程环境中的键值对 |
| `timeout` | 否 | 超时秒数，默认 30 |
| `background` | 否 | 立即返回 PID；默认 `false` |

后台模式的输出统一追加到 `/tmp/sandbox_bg.log`。需要最终状态、独立输出或取消能力时，应使用异步任务，而不是 `background: true`。

### 运行代码片段

`/run_code` 会先把代码写入临时文件，再调用对应解释器：

```bash
curl -sS "$SANDBOX_URL/run_code" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{
    "language": "python",
    "code": "items = [2, 3, 5]\nprint(sum(items))",
    "timeout": 30
  }'
```

支持的 `language` 值为 `python`、`bash`、`node` 和 `go`。默认 Docker 镜像包含这四种运行时，以及编译常见 CGO 项目所需的基础 C/C++ 工具。自定义镜像可以裁剪运行时，实际可用性仍应以 `/health` 返回的 `runtimes` 为准。

### 长任务

预计运行超过 30 秒的命令通过 `/task/create` 提交：

```bash
curl -sS "$SANDBOX_URL/task/create" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{
    "command": "npm ci && npm test",
    "cwd": "/workspace/project",
    "timeout": 600
  }'
```

接口会立即返回 `task_id`。查询状态：

```bash
curl -sS "$SANDBOX_URL/task/status?task_id=task_1&stream=true" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN"
```

任务状态包括 `pending`、`running`、`completed`、`failed` 和 `cancelled`。`stream=true` 会在任务运行期间返回当前已捕获的 `stdout` 和 `stderr`。

取消任务：

```bash
curl -sS "$SANDBOX_URL/task/cancel" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"task_id":"task_1"}'
```

任务只保存在进程内存中，重启后不会恢复。已结束任务保留约 30 分钟；任务表最多保存 200 条记录。

### 文件操作

写入文本文件：

```bash
curl -sS "$SANDBOX_URL/write_file" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"path":"/workspace/example.txt","content":"first line\n"}'
```

按行读取文件：

```bash
curl -sS "$SANDBOX_URL/read_file" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"path":"/workspace/example.txt","offset":0,"limit":100}'
```

批量写入文件：

```bash
curl -sS "$SANDBOX_URL/write_files" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{
    "files": [
      {"path": "/workspace/app/main.py", "content": "print(42)\n"},
      {"path": "/workspace/app/config.json", "content": "{\"debug\":false}\n"}
    ]
  }'
```

二进制内容使用 base64 放在 `content_b64` 中。`/upload` 和 `/download` 是同一套语义下便于工具封装的独立端点。

### 批量请求

`/batch` 可以减少多次 HTTP 往返：

```bash
curl -sS "$SANDBOX_URL/batch" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{
    "operations": [
      {
        "endpoint": "/write_file",
        "body": {"path": "/tmp/batch.txt", "content": "ready\n"}
      },
      {
        "endpoint": "/read_file",
        "body": {"path": "/tmp/batch.txt"}
      }
    ],
    "stop_on_error": true
  }'
```

一次最多执行 50 个子操作。每个子操作都按 POST 处理，不能调用 `/batch` 本身，也不能把前一个操作的返回值自动填入后一个请求。

### 从容器发起 HTTP 请求

```bash
curl -sS "$SANDBOX_URL/http_fetch" \
  -H "X-Gateway-Token: $GATEWAY_TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{
    "url": "https://example.com/",
    "method": "GET",
    "timeout": 20
  }'
```

响应包含目标站点的 `status_code`、`headers`、`body`、`size` 和 `truncated`。这个接口可以访问容器网络所能到达的地址，包括内网地址，因此部署时要把它视为具备 SSRF 能力的受信任接口。

## API 一览

### 服务状态

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/` | 最小存活探针，无需鉴权 |
| `GET` | `/health` | 运行时和 GUI 可用性，无需鉴权 |
| `GET` / `POST` | `/system/info` | CPU、内存、磁盘和任务统计 |

### 命令和任务

| 方法 | 路径 | 主要请求字段 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/exec` | `command`, `cwd`, `env`, `timeout`, `background` | 执行 shell 命令 |
| `POST` | `/run_code` | `language`, `code`, `timeout` | 执行代码片段 |
| `POST` | `/task/create` | `command`, `cwd`, `env`, `timeout` | 创建异步任务 |
| `GET` / `POST` | `/task/status` | `task_id`; GET 可加 `stream=true` | 查询任务和实时输出 |
| `POST` | `/task/cancel` | `task_id` | 取消任务 |
| `GET` / `POST` | `/task/list` | 无 | 列出任务 |

### 文件系统

| 方法 | 路径 | 主要请求字段 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/read_file` | `path`, `offset`, `limit` | 读取文本，可按行切片 |
| `POST` | `/write_file` | `path`, `content` / `content_b64`, `append` | 写入或追加文件 |
| `POST` | `/write_files` | `files` | 批量写入文件 |
| `POST` | `/delete_file` | `path` | 递归删除文件或目录 |
| `POST` | `/list_dir` | `path`, `show_hidden` | 列出目录 |
| `POST` | `/search_files` | `pattern`, `path`, `glob`, `before_context`, `after_context`, `max_results` | 递归搜索文本 |
| `POST` | `/upload` | `path`, `content_b64` | 上传二进制文件 |
| `POST` | `/download` | `path` | 以 base64 下载文件 |

`/write_files` 一次最多写入 100 个文件，总内容上限约 16 MiB。中途失败时会尝试删除本次已经写入的路径，但这不是严格事务；不要用它批量覆盖无法恢复的重要文件。

### 虚拟桌面

| 方法 | 路径 | 主要请求字段 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/gui_status` | 无 | 检查 X server 和 GUI 工具 |
| `POST` | `/screenshot` | `path` | 截取整个虚拟桌面并返回 PNG base64 |
| `POST` | `/click` | `x`, `y`, `button`, `double` | 点击屏幕坐标 |
| `POST` | `/type_text` | `text` | 向当前焦点窗口输入文本 |
| `POST` | `/press_key` | `key` | 发送按键或组合键 |
| `POST` | `/move_mouse` | `x`, `y` | 移动鼠标 |

调用这些端点前先检查 `/health` 的 `gui_ready`。坐标操作依赖当前桌面状态，点击前后都应重新截图确认。

### Playwright 浏览器

| 方法 | 路径 | 主要请求字段 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/browser/status` | 无 | 检查 Playwright 和 Chromium |
| `POST` | `/browser/navigate` | `url`, `timeout` | 访问 URL 并返回状态 |
| `POST` | `/browser/screenshot` | `url`, `selector`, `full_page`, `timeout` | 截取网页或元素 |
| `POST` | `/browser/click` | `url`, `selector`, `timeout` | 点击 CSS 选择器 |
| `POST` | `/browser/type` | `url`, `selector`, `text`, `timeout` | 填写输入框 |
| `POST` | `/browser/evaluate` | `url`, `script`, `timeout` | 在页面上下文执行 JavaScript |
| `POST` | `/browser/wait_for` | `url`, `selector`, `timeout` | 等待选择器出现 |

浏览器端点是无状态的。每次请求都会启动一个新的 Chromium、打开请求中的 URL、执行一次操作，然后关闭浏览器。Cookie、点击结果和输入内容不会延续到下一次请求。多步骤流程应写成一个 Playwright 脚本通过 `/run_code` 或 `/exec` 执行，或者改用共享的虚拟桌面。

### 网络和批处理

| 方法 | 路径 | 主要请求字段 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/http_fetch` | `url`, `method`, `headers`, `body`, `timeout` | 从容器发起 HTTP 请求 |
| `POST` | `/batch` | `operations`, `stop_on_error` | 批量执行固定的 POST 操作 |

## 资源限制

| 项目 | 限制 |
| --- | --- |
| 单条命令的 stdout | 4 MiB，超出部分截断 |
| 单条命令的 stderr | 4 MiB，超出部分截断 |
| `/read_file` 和 `/download` | 单文件 32 MiB |
| `/http_fetch` 响应体 | 8 MiB，超出部分截断 |
| `/write_files` | 100 个文件，总内容约 16 MiB |
| `/batch` | 50 个子操作 |
| 异步任务表 | 200 条记录 |
| 已结束任务保留时间 | 约 30 分钟 |
| 异步任务默认超时 | 300 秒 |
| 普通命令默认超时 | 30 秒 |

大文件应先在容器内筛选、压缩或拆分，再取回结果。不要依赖网关保存长期状态；任务表和未挂载目录中的文件都可能随进程或容器重启而丢失。

## Agent 使用指南

仓库内提供了项目级 skill：

```text
.agents/skills/ai-sandbox-gateway/
├── SKILL.md
├── agents/openai.yaml
├── references/api.md
└── scripts/gateway_call.py
```

支持 skill 的 Agent 可以通过 `$ai-sandbox-gateway` 调用。配套脚本只依赖 Python 标准库：

```bash
export AI_SANDBOX_URL='https://runtime.example'
export AI_SANDBOX_CREDENTIALS="$PWD/cs_token.txt"

python3 .agents/skills/ai-sandbox-gateway/scripts/gateway_call.py health

python3 .agents/skills/ai-sandbox-gateway/scripts/gateway_call.py \
  call /exec --data '{"command":"uname -a"}'

python3 .agents/skills/ai-sandbox-gateway/scripts/gateway_call.py \
  task --timeout 600 'npm ci && npm test'
```

脚本从凭据文件中读取 `gateway_token` 和 `studio_token`：前者只通过 `X-Gateway-Token` 请求头发送，后者会自动合并到每个 ModelScope 请求的查询参数。不要用 shell 变量拼接或解析该文件。脚本还提供 `upload` 和 `download` 子命令，适合在本地文件与远程容器之间传输二进制内容。

## 部署到 ModelScope Studio

`deploy.py` 使用 `modelscope.ai` 当前的 Studio API。脚本会完成以下操作：

1. 通过 `POST /api/v1/login` 登录并保存 Cookie 和 CSRF Token。
2. 创建或复用 Docker Studio。
3. 将当前 Git 提交推送到 Studio 的 `master` 分支。
4. 新增或更新 Studio 环境变量 `GATEWAY_TOKEN`。
5. 将 Gateway Token 和 ModelScope Studio Token 写入本地凭据文件。
6. 调用 `reset_restart` 并等待实例进入 `Running`。
7. 输出 Studio 页面、运行实例的基础 URL 和凭据文件路径。

先执行公开能力检查。这个命令不需要 Access Token，也不会修改远端：

```bash
python3 deploy.py --check
```

部署时优先使用 `MODELSCOPE_ACCESS_KEY`。旧名称 `MODELSCOPE_API_KEY` 仍然兼容：

```bash
export MODELSCOPE_ACCESS_KEY='ms-your-access-token'
python3 deploy.py
```

命名空间默认从登录结果中读取，不必手工设置。需要部署到组织或其他命名空间时再指定：

```bash
python3 deploy.py --namespace your-namespace --studio-name ai-sandbox-go
```

脚本默认创建公开 Studio（`visibility=1`），因为网关本身已经使用 `GATEWAY_TOKEN` 鉴权。公开 Studio 的仓库源码也可能公开可见，不要把凭据写进仓库；运行时凭据应通过 Studio 环境变量注入。若账户支持其他可见性值，可通过 `--visibility` 显式覆盖。

部署前需要了解以下行为：

- 只会推送已经提交的内容；工作区不干净时脚本会停止，避免漏掉本地改动。
- 推送前会读取远端 `master`。如果本地与远端历史分叉，脚本会保留远端提交并生成一个部署合并提交；该提交的文件内容与本地 `HEAD` 完全一致。
- ModelScope 的 `master` 是受保护分支，脚本始终使用普通快进推送，不会强制覆盖远端历史。
- 优先使用本地环境变量 `GATEWAY_TOKEN`，其次复用 `cs_token.txt` 中的 `gateway_token`，都不存在时才生成新 Token。首次升级时也能读取旧版纯文本文件。
- 部署成功后，Gateway Token 和 Studio Token 会以 JSON 写入权限为 `0600` 的 `cs_token.txt`，两者都不会打印到终端。
- Git 认证通过临时 credential helper 从环境读取 Access Token，不会把 Token 写进 remote URL。

常用选项：

```bash
# 触发重启后立即返回，不等待 Running
python3 deploy.py --no-wait

# 调整等待时间和轮询间隔
python3 deploy.py --wait-timeout 1200 --poll-interval 15
```

ModelScope.ai 的 Docker Studio 由仓库中的 `Dockerfile` 构建，因此平台 SDK 版本为空是正常情况。免费实例类型由 `/api/v1/studios/free_instance` 动态获取。

运行实例的域名由 ModelScope 分配，不能在部署前可靠推导。应使用脚本最后输出的 `Runtime base URL`，不要拼接旧的 `/api/v1/studios/<namespace>/<name>/proxy/7860` 地址。

把服务交给 Agent 时，只需提供 `Runtime base URL` 和 `cs_token.txt` 的绝对路径。凭据文件的结构如下，实际值不应出现在对话、日志或仓库中：

```json
{
  "gateway_token": "...",
  "studio_token": "..."
}
```

### 手动部署

也可以在 `https://www.modelscope.ai/studios/create` 手工创建 Docker Studio，然后：

1. 在 Studio 设置中新增环境变量 `GATEWAY_TOKEN`。
2. 将本仓库已提交的代码推送到 Studio Git 地址的 `master` 分支。
3. 在 Studio 页面执行重启。
4. 等待状态进入 `Running` 后，从 Studio 详情获取运行实例 URL。
5. 先请求 `/health`，再用带 `X-Gateway-Token` 的 `/system/info` 检查网关鉴权。

## 安全边界

部署前至少确认以下几点：

1. `/exec`、`/run_code` 和 `/task/create` 提供任意命令执行能力。不要把 Token 交给不受信任的调用方。
2. 服务本身不提供 HTTPS。公网部署应放在支持 TLS 的反向代理或托管平台后面。
3. 服务没有用户隔离、权限分级、速率限制或配额系统。一个 Token 对应整个实例的权限。
4. 文件接口接受绝对路径。`/delete_file` 会递归删除目标，只对清理后的 `/` 和 `.` 做了硬性拒绝。
5. `/http_fetch` 可以访问容器网络中的地址；`/browser/evaluate` 可以在目标页面执行 JavaScript。
6. 不要把任何 Token 写进仓库、命令输出或可公开访问的日志。Gateway Token 绝不能放进 URL；ModelScope Studio Token 由 helper 按平台要求加入请求查询参数。
7. 容器应使用独立、最小权限的运行环境。只挂载任务需要的目录，不要挂载 Docker socket、宿主机根目录或敏感凭据目录。

结构化端点在调用外部程序时尽量使用 argv 传参，避免路径、搜索词和输入文本被 shell 再解释；但 `/exec` 和异步任务本来就是 shell 接口，传入的 `command` 会按 shell 语义执行。调用方仍需对命令来源负责。

## 常见问题

### 请求返回 401

检查请求头名称是否为 `X-Gateway-Token`，并确认 Token 与当前进程使用的值一致。未显式配置 Token 时，服务重启会生成新的临时值。

### `/health` 正常，但其他接口返回 404

托管平台的基础 URL 往往包含代理路径。不要只保留域名；应在完整基础 URL 后追加 `/exec`、`/task/status` 等端点。

### `gui_ready` 为 false

直接运行 Go 程序不会自动启动 Xvfb。使用项目 Docker 镜像，或者自行安装并启动 X server、窗口管理器、xdotool 和截图工具。可调用 `/gui_status` 查看缺失项。

### 浏览器接口提示 Playwright 不可用

先调用 `/browser/status`。默认 Docker 镜像会安装 `playwright-chromium` 和 Chromium；本地直接运行时需要自行安装。

### 命令成功，但输出不完整

stdout 和 stderr 分别限制为 4 MiB。让命令在容器内把完整结果写入文件，再通过筛选、压缩或分片读取获取需要的部分。

### 任务查询不到

任务状态只存在内存中。进程重启后任务表会清空，已结束任务也会在约 30 分钟后回收。

## 项目结构

```text
.
├── main.go                         # 程序入口
├── internal
│   ├── config/config.go            # 环境变量和 Token
│   ├── server/server.go            # 路由、HTTP 服务和鉴权
│   └── handlers
│       ├── basic.go                # 存活、健康和 GUI 状态
│       ├── exec.go                 # 命令和代码执行
│       ├── tasks.go                # 异步任务和系统信息
│       ├── filesystem.go           # 文件操作
│       ├── batch.go                # 批量请求
│       ├── network.go              # HTTP fetch
│       ├── gui.go                  # 虚拟桌面操作
│       └── browser.go              # Playwright 操作
├── .agents/skills/ai-sandbox-gateway
│   ├── SKILL.md                    # Agent 使用流程
│   ├── references/api.md           # 完整 API 参考
│   └── scripts/gateway_call.py     # 命令行调用工具
├── Dockerfile                      # 完整运行镜像
├── entrypoint.sh                   # 启动 Xvfb、fluxbox 和网关
├── deploy.py                       # ModelScope 部署脚本
└── go.mod
```

## 开发与检查

```bash
go fmt ./...
go vet ./...
go test ./...
go build ./...
```

项目当前没有外部 Go 模块。修改 API 时，应同步检查路由、请求结构、README 和 `.agents/skills/ai-sandbox-gateway/references/api.md`，避免调用文档与实际处理器不一致。
