# Dev Container 开发环境说明

## 一、服务与端口

| 端口 | 服务 | 启动命令 |
|---|---|---|
| **5173** | 用户前台（Vite dev server） | `pnpm run dev` in `frontend/user` |
| **5174** | **管理后台**（Vite dev server） | `pnpm run dev` in `frontend/admin` |
| **8080** | **Go 后端 API** | `go run ./cmd/server` |
| 6379 | Redis | docker-compose |
| 6789 | Magpie 中转（容器内 → 宿主） | `socat`，见 1.1 |

> ⚠️ 常见误解：`:5174` **不是后端**，它是管理后台的 Vite dev server。
> 真正的后端在 `:8080`，而它默认**不挂载任何前端**（`go run ./cmd/server` 不带 `fullstack` 构建标签）。

四个进程由 `start-dev.sh` 在容器启动时拉起，日志在 `/tmp/dujiao-next-dev/`：

```bash
tail -f /tmp/dujiao-next-dev/backend.log       # Go 后端
tail -f /tmp/dujiao-next-dev/user.log          # 用户前台
tail -f /tmp/dujiao-next-dev/admin.log         # 管理后台
tail -f /tmp/dujiao-next-dev/magpie-relay.log  # Magpie 中转
```

### 1.1 容器内的 Magpie 中转（127.0.0.1:6789）

容器有独立的网络命名空间：里面的 `127.0.0.1` 指向容器自身，**到不了 Windows 上的
Magpie**。两类客户端的处理方式不同：

| 客户端 | 地址来源 | 走哪条路 |
|---|---|---|
| Claude Code | 环境变量 `ANTHROPIC_BASE_URL` | `devcontainer.json` 的 `remoteEnv` 直接给 `host.docker.internal:6789` |
| Copilot 的 BYOK 自定义模型 | Windows 侧 profile 里的设置项 | **读不到 `remoteEnv`**，只会打 `127.0.0.1:6789` → 需要中转 |

于是 `start-dev.sh` 在容器内起一层 `socat`，把 `127.0.0.1:6789` 转发到
`host.docker.internal:6789`，让「配置在 Windows、地址写回环」的客户端也能用。

典型症状（Copilot 侧）：

```
TypeError: fetch failed
  Error: connect ECONNREFUSED 127.0.0.1:6789
```

VS Code 里显示为「请检查防火墙规则和网络连接」—— **与防火墙无关**，
是地址指向了容器自己。中转起来后即恢复。

排查：

```bash
ss -ltn | grep 6789                      # 应有 127.0.0.1:6789 在监听
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:6789/v1/models   # 期望 200
tail -f /tmp/dujiao-next-dev/magpie-relay.log
```

- 无监听 → 容器内 `bash .devcontainer/start-dev.sh` 可拉起；仍失败则重建容器
  （`socat` 由 Dockerfile 显式安装，旧镜像里可能没有）
- 有监听但 `curl` 不通 → 宿主上的 Magpie 没在运行，去 Windows 上启动它，**容器内无需操作**

> 曾考虑的两条替代路线，都不如中转：
> ① 把 BYOK 地址改成 `host.docker.internal` —— 那是 profile 级设置、全局共享，
> 会弄坏「不套容器直接开发」的场景；② 让 Magpie 改绑 `0.0.0.0` —— 要把网关
> 暴露到局域网并放开 Windows 防火墙入站，而 `host.docker.internal` 走 Docker
> Desktop 自己的通道，既到得了宿主回环，也不受入站规则约束。

---

## 二、为什么热重载在 Dev Container 里会失效

### 根因：inotify 事件无法穿过 Docker Desktop 的 bind mount

```yaml
# docker-compose.yml
volumes:
  - ..:/workspaces/dujiao-next:cached
```

项目源码在 **Windows 宿主**（`C:\Users\...\dujiao-next`），通过 Docker Desktop 的文件共享层
挂载进 **Linux 容器**（`/workspaces/dujiao-next`）。

| 机制 | 能否穿过共享层 |
|---|---|
| `inotify` 文件事件 | ❌ **不能** |
| 文件 `mtime`（修改时间） | ✅ 能 |

所有基于 inotify 的文件监听器（Vite 的 chokidar 默认模式、`air`、`reflex`、`entr`、
`fsnotify`）在容器内**都收不到宿主上的改动事件**。

结果就是：在 Windows 上保存文件 → 容器内毫无感知 → HMR 不触发、后端不重启。

> 附注：`:cached` 是 **macOS 专属**的 osxfs 一致性选项，在 Windows 上会被 Docker Desktop
> 静默忽略。它在这里不起任何作用。

### 三个独立问题，会同时表现为「改了没反应」

| # | 问题 | 影响面 |
|---|---|---|
| 1 | Vite 用原生 inotify 监听，收不到宿主改动 | 前台 + 后台 |
| 2 | `go run ./cmd/server` 编译一次就跑，**本来就没有文件监听** | 后端 |
| 3 | `import.meta.glob` 的结果在 transform 阶段求值并被缓存 | 新增模板目录后需重启 dev server |

第 2 条不是 bug —— 这是仓库原有的开发方式，只是没有热重载能力。

---

## 三、已做的修复

### 3.1 前端：开启 Vite 轮询监听

`frontend/user/vite.config.ts` 与 `frontend/admin/vite.config.ts` 的 `server` 段新增：

```ts
watch: {
  usePolling: process.env.VITE_USE_POLLING !== '0',
  interval: Number(process.env.VITE_POLL_INTERVAL || 300),
  ignored: ['**/node_modules/**', '**/.git/**', '**/dist/**', '**/coverage/**'],
},
```

轮询检测 `mtime`，因此能感知宿主上的改动。`ignored` 必须保留 —— 否则轮询会去扫
`node_modules`（几万个文件），CPU 直接拉满。

### 3.2 后端：新增轮询式热重载

新增 `.devcontainer/watch-backend.sh`，并把 `start-dev.sh` 的后端命令从
`go run ./cmd/server` 换成该脚本。

脚本行为：

1. 每 1 秒扫描 `**/*.go` + `go.mod` + `go.sum` 的最新 `mtime`
2. 发现变化 → `go build -o /tmp/dujiao-next-server ./cmd/server`
3. 编译成功 → 停旧进程，启动新二进制
4. **编译失败 → 保留正在跑的服务**，日志报错，等你改好自动重试
   （不会出现"改到一半服务没了、端口被占"的窘境）

> 为什么用 `go build` + 运行二进制，而不是 `go run`：
> `go run` 会 fork 出真正跑服务的子进程，`kill` 父进程后端口仍被占用；
> 运行独立二进制则 kill 谁就是谁。

### 3.3 启动链路与宿主路径解耦

把项目目录从桌面移到 `D:\GitHub项目\dujiao-next` 后，出现了「Dev Containers 打不开
容器或项目」。根因有两条，都是路径变更引起的：

**① 容器复用了迁移前的旧容器。** Dev Containers 用「工作区目录名 + `_devcontainer`」
推导 compose 项目名（扩展日志里可见 `--project-name dujiao-next_devcontainer`）。
目录改名前后项目名不变，于是扩展匹配到了迁移前创建的容器，而它的 bind 源仍指向
已不存在的旧路径。Docker 对「源不存在的 bind mount」不报错，而是在挂载点造一个
空目录 —— 容器里的 `/workspaces/dujiao-next` 是空的。

**② `postStartCommand` 用了相对路径。** `bash .devcontainer/start-dev.sh` 隐含依赖
「工作目录恰好等于 workspaceFolder」。挂载为空时它以 exit 127 失败：

```
bash: .devcontainer/start-dev.sh: No such file or directory
postStartCommand from devcontainer.json failed with exit code 127.
Skipping any further user-provided commands.
```

Dev Containers 把 `postStartCommand` 的非零退出视为致命错误，随后整个附加流程失败、
容器打不开 —— 于是一个「服务起不来」被放大成「连 shell 都进不去」。

对应改动：

| 位置 | 改动 |
|---|---|
| `devcontainer.json` | `postStartCommand` 改用 `${containerWorkspaceFolder}` 绝对路径 |
| `devcontainer.json` | `postCreateCommand` 增加前端 `pnpm install`（补回不受 git 跟踪的依赖） |
| `start-dev.sh` | `ROOT` 从脚本自身位置推导；新增「空挂载」前置自检；始终以 0 退出 |
| `watch-backend.sh` | `ROOT` 默认值同样改为按脚本位置推导 |
| 新增 `reset-devcontainer.sh` | 迁移/改名后一键清理陈旧容器 |

> 为什么 `start-dev.sh` 不再以非零码退出：bootstrap 脚本失败不应该把开发者挡在容器
> 外面。失败信息照常打印，结尾还会汇总哪些进程没起来并指向 `doctor.sh`，
> 但不阻断容器打开 —— 否则连排查的入口都没有。

---

## 四、让修复生效

配置类改动不会被热重载，需要重启对应进程：

```bash
# 在容器内执行

# 1) 前端：vite.config.ts 改动必须重启 dev server（配置不参与 HMR）
kill "$(cat /tmp/dujiao-next-dev/user.pid)"  2>/dev/null
kill "$(cat /tmp/dujiao-next-dev/admin.pid)" 2>/dev/null
cd /workspaces/dujiao-next/frontend/user  && nohup pnpm run dev --host 0.0.0.0 > /tmp/dujiao-next-dev/user.log  2>&1 &
cd /workspaces/dujiao-next/frontend/admin && nohup pnpm run dev --host 0.0.0.0 > /tmp/dujiao-next-dev/admin.log 2>&1 &

# 2) 后端：切换到带热重载的启动方式
kill "$(cat /tmp/dujiao-next-dev/backend.pid)" 2>/dev/null
cd /workspaces/dujiao-next && nohup bash .devcontainer/watch-backend.sh /workspaces/dujiao-next > /tmp/dujiao-next-dev/backend.log 2>&1 &
```

**最省事的做法：直接重建 / 重启 Dev Container**（VS Code 命令面板 → `Dev Containers: Rebuild Container`
或 `Dev Containers: Restart Container`），`postStartCommand` 会自动按新配置拉起三个进程。

---

## 五、排查手册

### 5.0 一键体检（先跑这个）

```bash
bash .devcontainer/doctor.sh
# 宿主上（端口已转发时）：bash .devcontainer/doctor.sh http://localhost
```

它会逐层验证：服务可达性 → 端口指向的项目 → dev server 是否已加载最新代码 →
**当前实际激活的模板** → 模板文件完整性 → git 状态。

### 5.1 先判断是「热重载坏了」还是「模板没选中」

**这是最高频的误判**：页面显示旧模板，绝大多数情况下不是 HMR 问题，而是
**站点配置里根本没选中那个模板**。先确认：

```bash
curl -s http://localhost:8080/api/v1/public/config | grep -o '"storefront_template":"[^"]*"'
```

- 输出 `"classic"` → 模板**没被选中**，与 HMR 无关。用 `?template=atlas` 预览，
  或到后台「设置 → 站点与主题 → 前台模板」切换
- 输出 `"atlas"` → 模板已选中，问题在渲染侧

### 5.2 确认 dev server 真的加载了新代码

不要靠"看起来是新的"来判断 —— **直接向 dev server 要源文件**，
它返回什么，浏览器就能拿到什么：

```bash
# 前台：registry.ts 里应能看到 atlas
curl -s http://localhost:5173/src/templates/registry.ts | grep -o atlas | head -3

# 前台：新增的模板文件应可被提供
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:5173/src/templates/atlas/Home.vue

# 后台：设置页应含 atlas 选项
curl -s http://localhost:5174/src/views/admin/Settings.vue | grep -o atlasMode
```

返回 200 / 有命中 ⇒ 代码是新的，别再折腾 HMR 了，问题在别处。

### 5.3 区分「文件监听失效」还是「HMR 通道断了」

在**容器内**（VS Code 集成终端）改一个文件，与在 Windows 上改，对比行为：

| 容器内改 | 宿主上改 | 结论 |
|---|---|---|
| ✅ 刷新 | ✅ 刷新 | 正常 |
| ✅ 刷新 | ❌ 不动 | **文件监听失效** → 检查 `usePolling` 是否生效 |
| ❌ 不动 | ❌ 不动 | **HMR 通道断了** → 看浏览器 Console |

HMR 通道断了的典型报错：

```
[vite] failed to connect to websocket
WebSocket connection to 'ws://[::1]:5173/' failed
```

处理：确认没有经过会改写 Host 的反向代理；必要时在 `server` 里显式指定
`hmr: { host: 'localhost', clientPort: 5173 }`。

### 5.4 确认轮询真的开了

```bash
# 容器内：确认 vite 进程看到了环境变量
tr '\0' '\n' < /proc/$(pgrep -f "vite" | head -1)/environ | grep VITE_
```

没输出说明环境变量没传进去；`usePolling` 的默认值仍是 `true`（因为判断是 `!== '0'`），
所以不传也能工作。

### 5.5 确认后端热重载在跑

```bash
tail -f /tmp/dujiao-next-dev/backend.log
# 期望看到：
#   [watch-backend] 监视 /workspaces/dujiao-next 下的 .go / go.mod / go.sum（每 1s 轮询一次）
#   [watch-backend] ✓ 服务已启动 PID=12345

# 改一个 .go 文件保存，期望看到：
#   [watch-backend] 检测到源码变更，重新编译…
#   [watch-backend] ✓ 服务已启动 PID=12346
```

### 5.6 端口被占用 / 服务起不来

热重载脚本在编译失败时会保留旧服务，这是刻意的。若确实要强制清干净：

```bash
pkill -f "[d]ujiao-next-server"   # 杀掉热重载跑起来的后端二进制
pkill -f "[g]o run ./cmd/server"  # 杀掉可能残留的 go run
```

> 方括号是刻意的，别去掉。`pkill -f` 匹配的是完整命令行，而你自己敲的这条命令
> 命令行里就含有 `dujiao-next-server` 这串字面量 —— 不加方括号时它会把自己也匹配上，
> 表现为「命令执行到一半进程莫名消失、没有任何输出」。`[d]ujiao...` 作为正则仍然匹配
> `dujiao...`，但字面量 `[d]ujiao...` 不匹配它自己，从而避开这个坑。

---

## 六、可选：用 air 替代自研 watcher

`watch-backend.sh` 是**零依赖**方案，不需要重建容器。若你更想要成熟工具的特性
（彩色日志、更细的忽略规则、构建错误高亮），可以换成 [air](https://github.com/air-verse/air)：

```dockerfile
# .devcontainer/Dockerfile
FROM mcr.microsoft.com/devcontainers/go:1.26-bookworm
RUN go install github.com/air-verse/air@latest
```

```toml
# .air.toml（放仓库根目录）
root = "."
tmp_dir = "/tmp/air"

[build]
  cmd = "go build -o /tmp/air/main ./cmd/server"
  bin = "/tmp/air/main"
  include_ext = ["go", "tpl", "tmpl"]
  exclude_dir = ["frontend", "node_modules", ".git", "dist", "tmp"]
  delay = 800
  stop_on_error = false      # 编译失败保留旧进程

[log]
  time = true
```

然后把 `start-dev.sh` 里的命令改成 `exec air -c .air.toml`。

> 注意：air 默认基于 `fsnotify`（inotify），**在 Windows/macOS 宿主的 bind mount 下同样收不到事件**。
> 需要额外设置 `poll = true` 和 `poll_interval = 1000` 才会走轮询。
> 这正是本仓库选择自研轮询脚本的原因之一 —— 少一层踩坑。

---

## 七、性能与可调项

轮询会带来持续的文件扫描开销。当前配置下的可调项：

| 环境变量 | 默认 | 说明 |
|---|---|---|
| `VITE_USE_POLLING` | 开启 | 设为 `0` 关闭（**原生 Linux 开发**时建议关闭） |
| `VITE_POLL_INTERVAL` | `300`（毫秒） | 越小越灵敏、越吃 CPU |
| `WATCH_INTERVAL` | `1`（秒） | 后端轮询间隔 |

**后端扫描并不是「开销可忽略」** —— 实测（Windows bind mount，本仓库）：

| 项 | 单次耗时 |
|---|---|
| `stamp()`，未排除 `.pnpm-store`（修复前） | ~5.9s |
| `stamp()`，已排除 `.pnpm-store`（当前） | ~3.4s |

其中绝大部分是 `internal/` 下 1151 个 `.go` 文件的逐文件 `stat`。
瓶颈是 bind mount 上每次 `stat` 约 2~3ms 的往返延迟，不是文件数量本身 ——
所以只扫 Go 目录（3.2s）相比扫全仓库（5.9s）并没有质变。

`.pnpm-store` 必须排除：本仓库把它纳入了 git 跟踪，有 1.8 万个文件，
却不可能有 Go 源码，不排除就是纯白扫（这正是修复前那 5.9s 里多出来的 2.5s）。

> 于是 `WATCH_INTERVAL=1` 配合 3.4s 的单次扫描，等于监视器几乎在持续遍历
> 文件系统，会和 `go build` 抢 I/O。嫌吵就把 `WATCH_INTERVAL` 调到 5，
> 代价是热重载延迟从 ~4s 变成 ~8s。

若你只在原生 Linux 上开发，可在 `.devcontainer/start-dev.sh` 里加
`export VITE_USE_POLLING=0`，或直接在 shell 里 `export` 后重启 dev server。

---

## 八、宿主目录迁移 / 重命名后打不开容器

### 8.1 症状

- 打开项目时 VS Code 报错，或提示
  `postStartCommand ... failed with exit code 127. Skipping any further user-provided commands.`
- 容器里 `/workspaces/dujiao-next` 是空的（`ls` 看不到 `cmd/`、`frontend/`、`internal/`）
- 后端日志：`stat /workspaces/dujiao-next/cmd/server: directory not found`
- 前台日志：`cd: /workspaces/dujiao-next/frontend/user: No such file or directory`

### 8.2 原因

Dev Containers 按「工作区目录名 + `_devcontainer`」推导 compose 项目名。目录改名前后
项目名不变 → 扩展匹配到迁移前创建的旧容器 → 旧容器的 bind 源指向已不存在的旧路径
→ 挂载点是个空目录。详见 3.3。

### 8.3 处理

```bash
# 在宿主上执行（会先列出待删容器并要求确认；加 --yes 跳过确认）
bash .devcontainer/reset-devcontainer.sh --yes
```

然后回到 VS Code：命令面板 → `Dev Containers: Reopen in Container`。

若仍不正常，用更彻底的一档：

```
Dev Containers: Rebuild Container Without Cache
```

### 8.4 确认已恢复

```bash
# 容器内
ls /workspaces/dujiao-next        # 应看到 cmd/ frontend/ internal/
bash .devcontainer/doctor.sh      # 一键体检
```

### 8.5 迁移前建议

1. **先关容器再搬目录**：命令面板 → `Dev Containers: Close Remote Connection`，
   再移动文件夹，最后重新打开。这样不会留下指向旧路径的容器。
2. **搬家后不要指望「打开即恢复」**：即便 VS Code 没报错，也执行一次
   `Dev Containers: Rebuild Container`，确保容器按新路径重建。
3. `node_modules/`、`dist/` 等**不受 git 跟踪**的目录不会随 git 恢复。
   `postCreateCommand` 已包含前端 `pnpm install`，重建容器时会自动补装；
   若只是想在现有容器里补，手动执行：

   ```bash
   cd frontend/user && pnpm install
   cd ../admin      && pnpm install
   ```

### 8.6 重建后后端要编译几分钟（正常现象，不是卡死）

容器每次重建都会丢掉 Go 构建缓存 —— `GOCACHE` 默认是 `/home/vscode/.cache/go-build`，
位于容器可写层，容器一重建就没了。再加上源码在 Windows bind mount 上，I/O 很慢。实测：

| 场景 | `go build ./cmd/server` 耗时 |
|---|---|
| 冷缓存（刚重建容器） | ~210s |
| 热缓存 | ~50s（其中 CPU 仅 ~15s，其余全是 bind mount 的 I/O 等待） |

所以重建容器后 `postStartCommand` 会在 `Waiting for Go backend` 处停留几分钟 ——
那是编译，不是卡死。等待窗口是环境变量 `BACKEND_WAIT`（默认 300s），
等待期间每 30s 打印一次进度。想确认是否真在编译：

```bash
tail -f /tmp/dujiao-next-dev/backend.log
```

> 教训：这个窗口**不要凭感觉调小**。曾把它设成 60s，短于冷编译的 210s，
> 于是每次重建都稳定打印一条「Go backend did not become ready」的告警，
> 让人误以为后端坏了。

**可选优化：把构建缓存放进具名卷，重建后不再冷编译**

`docker-compose.yml` 加一个具名卷（`docker compose down` 不删具名卷，
只有 `down -v` 才删），重建后缓存仍在，后端编译回到 ~50s：

```yaml
services:
  app:
    volumes:
      - ..:/workspaces/dujiao-next:cached
      - go-build-cache:/home/vscode/.cache/go-build

volumes:
  go-build-cache:
```

> ⚠️ 这条**尚未在本仓库实测**。具名卷首次创建由 Docker 初始化，属主可能是 root，
> 而生命周期命令以 `vscode` 运行。挂载后必须先确认可写：
> `docker exec -u vscode <容器> touch /home/vscode/.cache/go-build/.probe`。
> 不可写的话需要在镜像里预先建好该目录（并保证属主为 vscode）再挂载。
> 改完请重建容器实测一次再长期使用。

