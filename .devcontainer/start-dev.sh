#!/usr/bin/env bash

set -u

ROOT="/workspaces/dujiao-next"
LOG_DIR="/tmp/dujiao-next-dev"

mkdir -p "$LOG_DIR"

cd "$ROOT"

echo "======================================"
echo " Dujiao-Next development environment"
echo "======================================"

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
        echo "ERROR: Redis is not ready."
        exit 1
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

    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")

        if kill -0 "$PID" 2>/dev/null; then
            echo "$NAME is already running. PID=$PID"
            return
        fi
    fi

    rm -f "$PID_FILE"

    nohup bash -lc "$COMMAND" \
        > "$LOG_FILE" 2>&1 &

    PID=$!

    echo "$PID" > "$PID_FILE"

    echo "$NAME started. PID=$PID"
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
# 3. 等待 Go 后端启动
# --------------------------------------------------

echo "Waiting for Go backend..."

for i in $(seq 1 60); do

    if (echo > /dev/tcp/127.0.0.1/8080) >/dev/null 2>&1; then
        echo "Go backend is ready."
        break
    fi

    if [ "$i" -eq 60 ]; then
        echo "WARNING: Go backend did not become ready."
    fi

    sleep 1
done


# --------------------------------------------------
# 4. 启动用户前台
# --------------------------------------------------

echo "[3/4] Starting user frontend..."

start_process \
    "User frontend" \
    "$LOG_DIR/user.pid" \
    "$LOG_DIR/user.log" \
    "cd '$ROOT/frontend/user' && exec pnpm run dev --host 0.0.0.0"


# --------------------------------------------------
# 5. 启动管理后台
# --------------------------------------------------

echo "[4/4] Starting admin frontend..."

start_process \
    "Admin frontend" \
    "$LOG_DIR/admin.pid" \
    "$LOG_DIR/admin.log" \
    "cd '$ROOT/frontend/admin' && exec pnpm run dev --host 0.0.0.0"


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
echo "======================================"