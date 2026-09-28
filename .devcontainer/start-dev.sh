#!/usr/bin/env bash
# =============================================================================
# 开发环境启动脚本
# -----------------------------------------------------------------------------
# 由 devcontainer.json 的 postStartCommand 调用，负责拉起三个进程：
#   Go 后端（带轮询热重载）、用户前台、管理后台。
#
# 三个刻意的设计决策：
#
# 1) 项目根目录从「脚本自身位置」推导，而不是写死 /workspaces/dujiao-next。
#    写死路径在宿主目录被移动/重命名后会失效（挂载点变了，脚本还指向老位置）。
#
# 2) 本脚本始终以 0 退出，即便有子进程启动失败。
#    Dev Containers 把 postStartCommand 的非零退出视为致命错误：
#       postStartCommand from devcontainer.json failed with exit code N.
#       Skipping any further user-provided commands.
#    随后整个附加流程失败，容器直接打不开 —— 一个服务起不来会升级成
#    「连 shell 都进不去」，反而无法排查。因此这里改为
#    「尽力启动 + 明确汇报」：失败信息照常打印，但不阻断容器打开。
#
# 3) 三个进程全部拉起之后才探测后端就绪，而不是在启动前台之前同步等待。
#    容器每次重建都会丢掉 Go 构建缓存（~/.cache/go-build 在容器可写层里），
#    重建后的首次编译通常要 1~2 分钟。若在启动前台前同步等待，
#    前台会被这段编译时间白白卡住；而 60s 的窗口又短于冷编译，
#    会稳定打印一条误导性的「未就绪」告警。
# =============================================================================

set -u

# 项目根目录：优先取环境变量 DUJIAO_ROOT，其次按「脚本所在目录的上一级」推导
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${DUJIAO_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"
LOG_DIR="/tmp/dujiao-next-dev"

mkdir -p "$LOG_DIR"

# 目录不可达时不在这里抛 bash 原生错误，交给下面的前置自检统一给出可操作提示
cd "$ROOT" 2>/dev/null || true

echo "======================================"
echo " Dujiao-Next development environment"
echo "======================================"
echo "ROOT=$ROOT"
echo ""


# --------------------------------------------------
# 0. 前置自检：确认挂载进来的确实是完整的项目根目录
# --------------------------------------------------
# 迁移/重命名宿主目录后最常踩的坑：容器复用了迁移前创建的旧容器，
# 其 bind 源仍指向已不存在的旧路径，于是 /workspaces/dujiao-next 是个空目录。
# 此时后续每一步都会以 "No such file or directory" 失败，且报错位置离真因很远。
# 这里提前识别，并给出可操作的修复指引。
if [ ! -f "$ROOT/go.mod" ] || [ ! -d "$ROOT/.devcontainer" ] || [ ! -d "$ROOT/cmd" ]; then
    echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
    echo " 错误：$ROOT 不是完整的项目根目录"
    echo "       （缺少 go.mod / .devcontainer / cmd 之一）"
    echo ""
    echo " 当前实际内容："
    ls -A "$ROOT" 2>/dev/null | sed 's/^/   /'
    echo ""
    echo " 最可能的原因：宿主上的项目目录被移动或重命名后，容器仍复用了旧容器，"
    echo " 它的 bind 源指向已不存在的旧路径，挂载进来的是一个空目录。"
    echo ""
    echo " 修复方式（任选其一）："
    echo "   · VS Code 命令面板 → Dev Containers: Rebuild Container"
    echo "   · 或先在宿主上执行 bash .devcontainer/reset-devcontainer.sh，"
    echo "     再重新打开容器"
    echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
    echo ""
    exit 0
fi


# --------------------------------------------------
# 1. 等待 Redis
# --------------------------------------------------

echo "[1/4] Waiting for Redis..."

for i in $(seq 1 60); do
    if (echo > /dev/tcp/redis/6379) >/dev/null 2>&1; then
        echo "Redis is ready."
        break
    fi

    if [ "$i" -eq 60 ]; then
        echo "WARNING: Redis 未就绪，仍继续启动（后端可能连接失败）。"
        break
    fi

    sleep 1
done


# --------------------------------------------------
# 启动函数
# --------------------------------------------------

start_process() {
    NAME="$1"
    PID_FILE="$2"
    LOG_FILE="$3"
    COMMAND="$4"

    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE" 2>/dev/null)" 2>/dev/null; then
        echo "$NAME is already running. PID=$(cat "$PID_FILE")"
        return
    fi

    rm -f "$PID_FILE"

    # 用 setsid 让服务进入独立的会话与进程组，而不是只靠 nohup。
    #
    # 本脚本由 docker exec 以 postStartCommand 的身份运行，而脚本里用
    # `nohup ... &` 起的后台进程**仍然留在调用方的进程组里**。实测：
    #     PID    PPID    PGID     SID
    #   11365   11357   11357   11357     ← 与父 shell 同 PGID/SID
    # 一旦这个 exec 会话被回收、或调用方按进程组发信号，后台服务会被一起带走。
    # nohup 只忽略 SIGHUP，挡不住 SIGTERM —— 表现就是「服务起来过一会儿又没了，
    # 而日志里没有任何崩溃信息」。
    #
    # setsid 之后服务自成会话（SID = 自身 PID），不再受调用方进程组影响。
    #
    # PID 文件由子进程自己写：setsid 在必要时会 fork，父进程拿到的 $! 未必是真实 PID。
    setsid bash -c '
        echo $$ > "$1"
        exec bash -lc "$2"
    ' _ "$PID_FILE" "$COMMAND" > "$LOG_FILE" 2>&1 < /dev/null &

    # 等子进程把 PID 写出来（最多 2s）
    for _ in 1 2 3 4 5 6 7 8 9 10; do
        [ -s "$PID_FILE" ] && break
        sleep 0.2
    done

    echo "$NAME started. PID=$(cat "$PID_FILE" 2>/dev/null || echo '?')"
}


# --------------------------------------------------
# 2. 启动 Go 后端（带热重载）
# --------------------------------------------------
# 走 watch-backend.sh 而不是直接 go run：
#   Docker Desktop 的 bind mount 不转发 inotify，改 .go 文件后必须手动重启，
#   该脚本用轮询检测 mtime 变化并自动重编译重启（零依赖）。
#   如需退回「只跑不重载」，把下面的命令换成 "cd '$ROOT' && exec go run ./cmd/server"。

echo "[2/4] Starting Go backend..."

start_process \
    "Go backend" \
    "$LOG_DIR/backend.pid" \
    "$LOG_DIR/backend.log" \
    "cd '$ROOT' && exec bash '$ROOT/.devcontainer/watch-backend.sh' '$ROOT'"


# --------------------------------------------------
# 3. 启动用户前台
# --------------------------------------------------

echo "[3/4] Starting user frontend..."

start_process \
    "User frontend" \
    "$LOG_DIR/user.pid" \
    "$LOG_DIR/user.log" \
    "cd '$ROOT/frontend/user' && exec pnpm run dev --host 0.0.0.0"


# --------------------------------------------------
# 4. 启动管理后台
# --------------------------------------------------

echo "[4/4] Starting admin frontend..."

start_process \
    "Admin frontend" \
    "$LOG_DIR/admin.pid" \
    "$LOG_DIR/admin.log" \
    "cd '$ROOT/frontend/admin' && exec pnpm run dev --host 0.0.0.0"


# --------------------------------------------------
# 5. 等待后端就绪（放在最后，见文件头第 3 条）
# --------------------------------------------------
# 窗口按实测值设定，不要凭感觉调小：
#   容器每次重建都会丢掉 Go 构建缓存（GOCACHE=/home/vscode/.cache/go-build
#   在容器可写层里），而源码在 Windows bind mount 上，I/O 很慢。
#   实测：冷编译约 210s；缓存命中约 50s（其中 CPU 只占 ~15s，其余是 I/O 等待）。
#   窗口短于冷编译就会稳定打印一条误导性的「未就绪」告警 —— 见 README 8.6。

BACKEND_WAIT="${BACKEND_WAIT:-300}"
backend_ready=0

echo ""
echo "Waiting for Go backend (最多 ${BACKEND_WAIT}s)..."

for i in $(seq 1 "$BACKEND_WAIT"); do

    if (echo > /dev/tcp/127.0.0.1/8080) >/dev/null 2>&1; then
        echo "Go backend is ready. (${i}s)"
        backend_ready=1
        break
    fi

    # 每 30s 报一次进度，避免长时间静默
    if [ $((i % 30)) -eq 0 ]; then
        echo "  ... 已等待 ${i}s（编译中，进度见 $LOG_DIR/backend.log）"
    fi

    sleep 1
done

if [ "$backend_ready" -ne 1 ]; then
    echo "WARNING: Go backend 在 ${BACKEND_WAIT}s 内未就绪。"
    echo "         首次冷编译较慢属正常，可能稍后自行起来；"
    echo "         请查 $LOG_DIR/backend.log 确认是否编译报错。"
fi


# --------------------------------------------------
# 6. 结果自检（只汇报，不改退出码）
# --------------------------------------------------

DEAD=""

for item in "Go backend:$LOG_DIR/backend.pid" \
            "User frontend:$LOG_DIR/user.pid" \
            "Admin frontend:$LOG_DIR/admin.pid"; do
    NAME="${item%%:*}"
    PID_FILE="${item#*:}"

    if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
        :
    else
        DEAD="$DEAD $NAME"
    fi
done


echo ""
echo "======================================"
echo " Dujiao-Next services started"
echo "======================================"
echo ""
echo "User:"
echo "http://localhost:5173"
echo ""
echo "Admin:"
echo "http://localhost:5174"
echo ""
echo "Backend:"
echo "http://localhost:8080"
echo ""
echo "Logs:"
echo "$LOG_DIR"

if [ -n "$DEAD" ]; then
    echo ""
    echo "--------------------------------------"
    echo " 注意：以下进程未存活:$DEAD"
    echo ""
    echo " 最近日志："
    tail -n 10 "$LOG_DIR"/*.log 2>/dev/null | sed 's/^/   /'
    echo ""
    echo " 一键体检： bash $ROOT/.devcontainer/doctor.sh"
    echo "--------------------------------------"
fi

echo "======================================"
