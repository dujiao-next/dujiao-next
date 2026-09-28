#!/usr/bin/env bash
# =============================================================================
# 后端热重载（零依赖，仅用 coreutils / findutils）
# -----------------------------------------------------------------------------
# 为什么不用 inotify 类工具（air / reflex / entr）：
#   Docker Desktop 在 Windows / macOS 宿主上通过文件共享层挂载源码，
#   **inotify 事件不会从宿主传到容器**，所有基于 inotify 的监听器都会失效。
#   而 mtime 是正常同步的，因此轮询是这里唯一可靠且零依赖的做法。
#
# 为什么先 go build 再运行二进制（而不是直接 go run）：
#   1. go run 会 fork 出真正跑服务的子进程，kill 父进程后端口仍被占用；
#      运行独立二进制则 kill 谁就是谁。
#   2. 编译失败时不会杀掉正在跑的服务，你可以继续改代码，
#      修好后自动重新编译启动（与 air 的行为一致），不会出现"改一半服务没了"。
#
# 用法：bash .devcontainer/watch-backend.sh [项目根目录]
# 环境变量：
#   WATCH_INTERVAL   轮询间隔秒数，默认 1
#   SERVER_PKG       要构建的包，默认 ./cmd/server
# =============================================================================

set -u

# 项目根目录：优先取位置参数，其次按「脚本所在目录的上一级」推导。
# 不写死 /workspaces/dujiao-next，宿主目录被移动/重命名后依然可用。
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="${1:-$(cd "$SCRIPT_DIR/.." && pwd)}"
INTERVAL="${WATCH_INTERVAL:-1}"
SERVER_PKG="${SERVER_PKG:-./cmd/server}"
BIN="/tmp/dujiao-next-server"
SERVER_PID=""

log() { printf '[watch-backend] %s\n' "$*"; }

# ---------------------------------------------------------------- 进程清理
cleanup() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        log "停止服务 PID=$SERVER_PID"
        kill -TERM "$SERVER_PID" 2>/dev/null
        wait "$SERVER_PID" 2>/dev/null
    fi
    exit 0
}
trap cleanup INT TERM EXIT

# ---------------------------------------------------------------- 变更指纹
# 取所有 .go / go.mod / go.sum 中最新的 mtime（秒级 + 纳秒）。
# 只要有任何一个文件被改动，这个值就会变。
#
# prune 列表必须包含 .pnpm-store：本仓库把它纳入了 git 跟踪，里面有 1.8 万个文件，
# 却不可能有 Go 源码。不排除的话每次轮询都要白 stat 它们一遍。
# 实测（Windows bind mount，本仓库）单次 stamp()：
#   未排除 .pnpm-store  ≈ 5.9s
#   已排除              ≈ 3.3s（剩下的是 internal/ 下 1151 个 .go 文件的逐文件 stat）
#
# 注意：轮询间隔 WATCH_INTERVAL 默认 1s，而单次扫描就要 3s 以上 ——
# 也就是说这个监视器几乎在持续遍历文件系统，会和 go build 抢 I/O。
# 若嫌吵，把 WATCH_INTERVAL 调到 5，代价是热重载延迟变长。
stamp() {
    find "$ROOT" \
        -type d \( \
            -name node_modules -o \
            -name .git -o \
            -name dist -o \
            -name .devcontainer -o \
            -name .pnpm-store -o \
            -name vendor -o \
            -name coverage \
        \) -prune -o \
        -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -printf '%T@\n' \
        2>/dev/null | sort -rn | head -1
}

# ---------------------------------------------------------------- 构建并启动
build_and_start() {
    log "编译中…"
    if ! ( cd "$ROOT" && go build -o "$BIN" "$SERVER_PKG" ); then
        log "✗ 编译失败，保留当前运行中的服务；修复后会自动重试"
        return 1
    fi

    # 新二进制就绪后才停旧服务，尽量缩短中断窗口
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        kill -TERM "$SERVER_PID" 2>/dev/null
        wait "$SERVER_PID" 2>/dev/null
    fi

    "$BIN" &
    SERVER_PID=$!
    log "✓ 服务已启动 PID=$SERVER_PID"
    return 0
}

# ---------------------------------------------------------------- 主循环
log "监视 $ROOT 下的 .go / go.mod / go.sum（每 ${INTERVAL}s 轮询一次）"

LAST_STAMP="$(stamp)"
build_and_start || true

while true; do
    sleep "$INTERVAL"

    NOW_STAMP="$(stamp)"
    if [ "$NOW_STAMP" != "$LAST_STAMP" ]; then
        LAST_STAMP="$NOW_STAMP"
        log "检测到源码变更，重新编译…"
        build_and_start || true
    fi
done
