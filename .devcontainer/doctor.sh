#!/usr/bin/env bash
# =============================================================================
# 店面模板体检脚本
# -----------------------------------------------------------------------------
# 回答一个高频问题：「我改了代码 / 换了模板，为什么页面还是旧的？」
#
# 逐层排查，把「代码没生效」和「模板没选中」这两类完全不同的原因区分开：
#   1. 服务是否在跑、端口是否指向正确的项目
#   2. dev server 是否已加载最新代码（直接向它请求源文件来验证）
#   3. 当前激活的是哪个模板（这才是「显示旧模板」最常见的真因）
#   4. 模板文件本身是否齐全
#
# 用法：
#   容器内： bash .devcontainer/doctor.sh
#   宿主上： bash .devcontainer/doctor.sh http://localhost
# =============================================================================

set -u

BASE="${1:-http://localhost}"
USER_URL="$BASE:5173"
ADMIN_URL="$BASE:5174"
API_URL="$BASE:8080"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

PASS=0
FAIL=0
WARN=0

ok()   { printf '  \033[32m✓\033[0m %s\n' "$*"; PASS=$((PASS + 1)); }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$*"; FAIL=$((FAIL + 1)); }
warn() { printf '  \033[33m!\033[0m %s\n' "$*"; WARN=$((WARN + 1)); }
head_() { printf '\n\033[1m%s\033[0m\n' "$*"; }

code() {
    local c
    # curl 连接失败时自身会打印 000 并非零退出，用 || true 避免重复输出
    c="$(curl -s -o /dev/null -w '%{http_code}' --max-time 6 "$1" 2>/dev/null || true)"
    printf '%s' "${c:-000}"
}

# -----------------------------------------------------------------------------
head_ "1. 服务可达性"
# -----------------------------------------------------------------------------
check_service() {
    local label="$1" url="$2" c
    c="$(code "$url")"
    if [ "$c" = "200" ]; then ok "$label  $url  → $c"
    else bad "$label  $url  → $c（服务未启动？）"; fi
}

check_service "用户前台" "$USER_URL/"
check_service "管理后台" "$ADMIN_URL/"
check_service "后端 API" "$API_URL/api/v1/public/config"

# Magpie 网关：容器内由 start-dev.sh 起一层 socat 中转（127.0.0.1:6789 → 宿主）。
# VS Code Copilot 的 BYOK 自定义模型把 base URL 存成 127.0.0.1:6789，而这份配置
# 在 Windows 侧、注入不了 remoteEnv —— 缺了中转就是 connect ECONNREFUSED
# 127.0.0.1:6789（VS Code 显示为「检查防火墙和网络连接」）。
#
# 分两级报：中转不在（无监听）与「中转在、宿主 Magpie 没开」是两回事，
# 前者要重启容器，后者只需在 Windows 上启动 Magpie。
if (echo > /dev/tcp/127.0.0.1/6789) >/dev/null 2>&1; then
    c="$(code "http://127.0.0.1:6789/v1/models")"
    if [ "$c" = "200" ]; then
        ok "Magpie 网关  127.0.0.1:6789 → $c"
    else
        warn "Magpie 网关 127.0.0.1:6789 在监听，但上游未就绪（→ $c）"
        printf '      宿主上的 Magpie 没在运行？在 Windows 上启动它即可，容器内无需操作。\n'
    fi
else
    warn "Magpie 网关 127.0.0.1:6789 无监听 —— 中转未运行"
    printf '      容器内跑 bash .devcontainer/start-dev.sh 可拉起；仍失败则重建容器。\n'
fi

# 后端根路径 404 是正常的：go run 不带 fullstack 标签时不挂载 SPA
c="$(code "$API_URL/")"
if [ "$c" = "404" ]; then
    ok "后端 / 返回 404 属正常（无 fullstack 标签时不挂载 SPA，UI 全由 Vite 提供）"
fi

# -----------------------------------------------------------------------------
head_ "2. 端口是否指向正确的项目"
# -----------------------------------------------------------------------------
user_html="$(curl -s --max-time 6 "$USER_URL/" 2>/dev/null)"
admin_html="$(curl -s --max-time 6 "$ADMIN_URL/" 2>/dev/null)"

if printf '%s' "$user_html" | grep -q '/@vite/client'; then
    ok ":5173 是 Vite dev server"
else
    bad ":5173 没有返回 Vite 页面"
fi

# 用「能不能取到该项目独有的源文件」来区分目录，比看 HTML 可靠
if [ "$(code "$USER_URL/src/templates/registry.ts")" = "200" ]; then
    ok ":5173 指向 frontend/user（能取到 templates/registry.ts）"
else
    bad ":5173 取不到 templates/registry.ts —— 可能指向了错误的目录"
fi

if [ "$(code "$ADMIN_URL/src/views/admin/Settings.vue")" = "200" ]; then
    ok ":5174 指向 frontend/admin（能取到 views/admin/Settings.vue）"
else
    bad ":5174 取不到 views/admin/Settings.vue —— 可能指向了错误的目录"
fi

# -----------------------------------------------------------------------------
head_ "3. dev server 是否已加载最新代码"
# -----------------------------------------------------------------------------
# 直接向 dev server 要源文件：它返回什么，浏览器就能拿到什么。
registry_js="$(curl -s --max-time 10 "$USER_URL/src/templates/registry.ts" 2>/dev/null)"

if printf '%s' "$registry_js" | grep -q 'atlas'; then
    ok "registry.ts 已包含 atlas（dev server 已加载最新代码）"
else
    bad "registry.ts 里没有 atlas —— dev server 还在跑旧代码，请重启它"
fi

if [ "$(code "$USER_URL/src/templates/atlas/Home.vue")" = "200" ]; then
    ok "atlas/Home.vue 可被 dev server 提供（HTTP 200）"
else
    bad "atlas/Home.vue 取不到 —— 文件缺失，或需重启 dev server 以重建 import.meta.glob"
fi

admin_settings="$(curl -s --max-time 10 "$ADMIN_URL/src/views/admin/Settings.vue" 2>/dev/null)"
if printf '%s' "$admin_settings" | grep -q "atlasMode\|'atlas'"; then
    ok "后台设置页已含 atlas 模板选项"
else
    bad "后台设置页没有 atlas 选项 —— 后台 dev server 还在跑旧代码"
fi

# -----------------------------------------------------------------------------
head_ "4. 当前实际激活的模板（「显示旧模板」的头号真因）"
# -----------------------------------------------------------------------------
config="$(curl -s --max-time 8 "$API_URL/api/v1/public/config" 2>/dev/null)"
active="$(printf '%s' "$config" | grep -o '"storefront_template":"[^"]*"' | head -1 | cut -d'"' -f4)"

if [ -z "$active" ]; then
    bad "取不到 storefront_template（后端未就绪？）"
elif [ "$active" = "atlas" ]; then
    ok "站点配置已激活 atlas —— 若页面仍是旧样式，问题在渲染侧"
else
    warn "站点配置当前激活的是「$active」，不是 atlas"
    printf '      这不是 bug：页面按配置渲染是正确行为。\n'
    printf '      要看到 atlas，二选一：\n'
    printf '        · 临时预览  %s/?template=atlas\n' "$USER_URL"
    printf '        · 正式启用  后台 %s → 设置 → 站点与主题 → 前台模板 → 选「浅灰工具风」\n' "$ADMIN_URL"
fi

# -----------------------------------------------------------------------------
head_ "5. 模板文件完整性"
# -----------------------------------------------------------------------------
# 逐一核对仍在库中的模板目录（classic 复用 views/，没有独立目录，不在此列）
for tpl in vault atlas; do
    if [ -d "$ROOT/frontend/user/src/templates/$tpl" ]; then
        n="$(find "$ROOT/frontend/user/src/templates/$tpl" -type f 2>/dev/null | wc -l | tr -d ' ')"
        ok "模板 $tpl：$n 个文件"
    else
        bad "找不到 frontend/user/src/templates/$tpl 目录"
    fi
done

if grep -q "StorefrontTemplateVault\|StorefrontTemplateAtlas" "$ROOT/internal/constants/constants.go" 2>/dev/null; then
    ok "后端模板常量已定义（vault / atlas）"
else
    bad "后端缺少模板常量"
fi

if grep -q "StorefrontTemplateVault\|StorefrontTemplateAtlas" "$ROOT/internal/modules/settings/application/site_normalize.go" 2>/dev/null; then
    ok "后端归一化白名单已放行 vault / atlas"
else
    warn "后端归一化未放行模板 —— 后台保存会被静默改回 classic（需重启后端）"
fi

# -----------------------------------------------------------------------------
head_ "6. Git 状态"
# -----------------------------------------------------------------------------
if git -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1; then
    printf '  分支 %s · HEAD %s\n' \
        "$(git -C "$ROOT" branch --show-current)" \
        "$(git -C "$ROOT" rev-parse --short HEAD)"
    dirty="$(git -C "$ROOT" status --porcelain | wc -l | tr -d ' ')"
    if [ "$dirty" = "0" ]; then ok "工作区干净"
    else warn "工作区有 $dirty 处未提交改动"; fi
else
    warn "不是 git 仓库"
fi

# -----------------------------------------------------------------------------
printf '\n\033[1m结果\033[0m  通过 %d，警告 %d，失败 %d\n\n' "$PASS" "$WARN" "$FAIL"
if [ "$FAIL" -gt 0 ]; then
    printf '存在失败项，按上面的提示逐条处理后重跑本脚本。\n\n'
    exit 1
fi
printf '基础项全部通过。若页面仍不是预期模板，请对照第 4 节的结论。\n\n'
