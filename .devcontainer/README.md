# Dev Container 开发环境说明

## 一、服务与端口

| 端口 | 服务 | 启动命令 |
|---|---|---|
| **5173** | 用户前台（Vite dev server） | `pnpm run dev` in `frontend/user` |
| **5174** | **管理后台**（Vite dev server） | `pnpm run dev` in `frontend/admin` |
| **8080** | **Go 后端 API** | `go run ./cmd/server` |
| 6379 | Redis | docker-compose |

> ⚠️ 常见误解：`:5174` **不是后端**，它是管理后台的 Vite dev server。
> 真正的后端在 `:8080`，而它默认**不挂载任何前端**（`go run ./cmd/server` 不带 `fullstack` 构建标签）。

三个进程由 `start-dev.sh` 在容器启动时拉起，日志在 `/tmp/dujiao-next-dev/`：

```bash
tail -f /tmp/dujiao-next-dev/backend.log   # Go 后端
tail -f /tmp/dujiao-next-dev/user.log      # 用户前台
tail -f /tmp/dujiao-next-dev/admin.log     # 管理后台
```

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

- 输出 `"classic"` → 模板**没被选中**，与 HMR 无关。用 `?template=ember` 预览，
  或到后台「设置 → 站点与主题 → 前台模板」切换
- 输出 `"ember"` → 模板已选中，问题在渲染侧

### 5.2 确认 dev server 真的加载了新代码

不要靠"看起来是新的"来判断 —— **直接向 dev server 要源文件**，
它返回什么，浏览器就能拿到什么：

```bash
# 前台：registry.ts 里应能看到 ember
curl -s http://localhost:5173/src/templates/registry.ts | grep -o ember | head -3

# 前台：新增的模板文件应可被提供
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:5173/src/templates/ember/Home.vue

# 后台：设置页应含 ember 选项
curl -s http://localhost:5174/src/views/admin/Settings.vue | grep -o emberMode
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
pkill -f "dujiao-next-server"   # 杀掉热重载跑起来的后端二进制
pkill -f "go run ./cmd/server"  # 杀掉可能残留的 go run
```

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

在 Dev Container 里，默认值已经过权衡：前端 300ms 响应足够快，
后端 1s 只扫描 `.go` 文件（数量远少于前端），开销可忽略。

若你只在原生 Linux 上开发，可在 `.devcontainer/start-dev.sh` 里加
`export VITE_USE_POLLING=0`，或直接在 shell 里 `export` 后重启 dev server。
