#!/usr/bin/env bash
# =============================================================================
# Dev Container 一键重置
# -----------------------------------------------------------------------------
# 解决什么问题
# ------------
# 把项目目录从 A 移到 B（或重命名）之后，Dev Containers 有可能复用「迁移前」
# 创建的旧容器。旧容器的 bind 源仍然指向 A，而 A 已经不存在，Docker 会在
# 挂载点造一个空目录 —— 于是容器里的工作区是空的：
#
#   bash: .devcontainer/start-dev.sh: No such file or directory
#   postStartCommand from devcontainer.json failed with exit code 127.
#   Skipping any further user-provided commands.
#
# Dev Containers 把 postStartCommand 的失败视为致命错误，随后整个附加流程
# 失败、容器打不开 —— 用户看到的就是「无法打开该容器或项目」。
#
# 本脚本把该项目的容器（含 bind 源已失效的陈旧容器）清干净，
# 让下次打开时按当前路径重建。
#
# 用法
# ----
#   bash .devcontainer/reset-devcontainer.sh                  # 交互确认后执行
#   bash .devcontainer/reset-devcontainer.sh --yes            # 跳过确认
#   bash .devcontainer/reset-devcontainer.sh --yes --purge-vscode-state
#
# --purge-vscode-state 额外清理 VS Code 生成的 docker-compose override 临时文件。
# 只删除「当前没有任何容器引用」的那些，因此不会影响其他 devcontainer 项目。
# 这些文件由扩展在每次运行时生成，删除后会自动重建，属安全操作。
# =============================================================================

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# VS Code 的 Dev Containers 扩展固定用 "<工作区目录名>_devcontainer" 作为
# compose 项目名（见扩展日志里的 --project-name 参数）。
PROJECT="$(basename "$ROOT")_devcontainer"

ASSUME_YES=0
PURGE_VSCODE_STATE=0

for arg in "$@"; do
    case "$arg" in
        --yes|-y)             ASSUME_YES=1 ;;
        --purge-vscode-state) PURGE_VSCODE_STATE=1 ;;
        -h|--help)            sed -n '2,32p' "$0"; exit 0 ;;
        *) echo "未知参数: $arg（用 --help 查看用法）" >&2; exit 2 ;;
    esac
done

echo "项目根目录   : $ROOT"
echo "compose 项目 : $PROJECT"
echo ""

if ! docker info >/dev/null 2>&1; then
    echo "错误：Docker 不可用。请先启动 Docker Desktop。" >&2
    exit 1
fi

# ---------------------------------------------------------------- 收集目标容器
TARGETS=""

# 1) 本项目 compose 名下的容器（不论运行与否）
while read -r id; do
    [ -n "$id" ] || continue
    TARGETS="$TARGETS $id"
done < <(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT" 2>/dev/null)

# 2) 带 devcontainer 标签、但 bind 源已不存在的容器 —— 迁移后的典型残留
while read -r id; do
    [ -n "$id" ] || continue
    src="$(docker inspect -f '{{range .Mounts}}{{if eq .Type "bind"}}{{.Source}}{{println}}{{end}}{{end}}' "$id" 2>/dev/null | head -1)"
    if [ -n "$src" ] && [ ! -e "$src" ]; then
        TARGETS="$TARGETS $id"
    fi
done < <(docker ps -aq --filter "label=devcontainer.local_folder" 2>/dev/null)

TARGETS="$(printf '%s\n' $TARGETS | sort -u | tr '\n' ' ')"
TARGETS="$(echo $TARGETS)"

if [ -z "$TARGETS" ]; then
    echo "没有找到需要清理的容器。"
else
    echo "将停止并删除以下容器："
    for id in $TARGETS; do
        name="$(docker inspect -f '{{.Name}}' "$id" 2>/dev/null | sed 's|^/||')"
        src="$(docker inspect -f '{{range .Mounts}}{{if eq .Type "bind"}}{{.Source}}{{println}}{{end}}{{end}}' "$id" 2>/dev/null | head -1)"
        state="$(docker inspect -f '{{.State.Status}}' "$id" 2>/dev/null)"
        printf '  %-14s %-42s %-9s bind源=%s\n' \
            "${id:0:12}" "${name:-?}" "${state:-?}" "${src:-<无>}"
    done
fi

# ---------------------------------------------------------------- 确认
if [ "$ASSUME_YES" -ne 1 ]; then
    echo ""
    printf '确认删除以上容器？[y/N] '
    read -r ans
    case "$ans" in
        y|Y|yes|YES) ;;
        *) echo "已取消，未做任何改动。"; exit 0 ;;
    esac
fi

# ---------------------------------------------------------------- 执行清理
echo ""
cd "$SCRIPT_DIR"

if [ -n "$TARGETS" ]; then
    echo "→ docker compose down"
    docker compose --project-name "$PROJECT" -f docker-compose.yml down --remove-orphans 2>&1 | sed 's/^/   /'

    for id in $TARGETS; do
        if docker rm -f "$id" >/dev/null 2>&1; then
            echo "   已删除容器 ${id:0:12}"
        fi
    done
fi

# ---------------------------------------------------------------- 清理 VS Code 临时文件
if [ "$PURGE_VSCODE_STATE" -eq 1 ]; then
    STATE_DIR="${APPDATA:-$HOME/AppData/Roaming}/Code/User/globalStorage/ms-vscode-remote.remote-containers/data/docker-compose"

    if [ -d "$STATE_DIR" ]; then
        echo ""
        echo "→ 清理 VS Code 生成的 compose override 临时文件"

        # 先收集「仍被现有容器引用」的文件，避免误删其他项目正在用的
        REFERENCED=""
        for id in $(docker ps -aq 2>/dev/null); do
            files="$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.config_files"}}' "$id" 2>/dev/null)"
            REFERENCED="$REFERENCED $files"
        done

        removed=0
        for f in "$STATE_DIR"/docker-compose.devcontainer.*.yml; do
            [ -f "$f" ] || continue
            if printf '%s' "$REFERENCED" | grep -qF "$(basename "$f")"; then
                continue
            fi
            rm -f "$f" && removed=$((removed + 1))
        done

        echo "   已删除 $removed 个未被引用的临时文件（保留其他项目正在使用的）"
    else
        echo ""
        echo "（未找到 VS Code 状态目录，跳过 --purge-vscode-state）"
    fi
fi

# ---------------------------------------------------------------- 后续步骤
cat <<'EOF'

======================================
 清理完成。接下来：
======================================

 1. 回到 VS Code，命令面板执行
      Dev Containers: Reopen in Container
    （或 Dev Containers: Rebuild Container）

 2. 容器打开后，在容器内确认工作区已就位：
      ls /workspaces/dujiao-next
      bash .devcontainer/doctor.sh

 3. 如果仍未恢复，执行带重建的重开：
      Dev Containers: Rebuild Container Without Cache

EOF
