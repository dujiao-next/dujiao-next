# AGENTS.md — Viva小铺 Dujiao-Next 源码定制项目

> 本项目是 `faka.bfsmlt.com` 生产站点的源码定制仓库。
> 任何 AI/开发者操作前请先阅读本文档。

---

## 1. 项目目标

基于 Dujiao-Next 定制 Viva小铺自动发卡平台。

当前生产站点：

```text
https://faka.bfsmlt.com
```

服务器：

```text
VMISS 香港 BGP
IP: 38.47.108.70
SSH: ssh vmiss
运行目录: /srv/faka
```

当前业务：

```text
超市 A 优惠券
超市 B 优惠券
虚拟优惠码 / 卡密 / 自动发货
```

---

## 2. 当前仓库说明

当前仓库 fork 自：

```text
https://github.com/dujiao-next/dujiao-next
```

本地目录：

```text
/Users/lutao/vibeCodingProjects/viva/viva-faka-custom
```

当前远程：

```text
origin   git@github.com:LTXWorld/viva-faka-custom.git
upstream https://github.com/dujiao-next/dujiao-next.git 或 git@github.com:dujiao-next/dujiao-next.git
```

当前定制分支：

```text
viva-custom
```

---

## 3. 重要架构提醒

Dujiao-Next fullstack release 由三部分组成：

```text
1. dujiao-next/dujiao-next  后端 API + fullstack embed 壳
2. dujiao-next/user         前台用户端 SPA
3. dujiao-next/admin        后台管理端 SPA
```

本仓库主要是后端 API 仓库。

如果要改前台页面，例如：

```text
游客购买邮箱/密码字段
首页样式
商品详情页
订单详情页
支付页
```

通常需要修改 `dujiao-next/user` 前台仓库，而不是只改本仓库。

如果要改后台管理页面，通常需要修改 `dujiao-next/admin` 仓库。

---

## 4. 当前生产环境已完成配置

生产站点当前已完成：

```text
站点名：Viva小铺
博客导航：关闭
支付渠道：KPay 易支付 / 支付宝 / redirect
支付成功回跳：https://faka.bfsmlt.com/pay
游客购买字段提示：通过 site_config.scripts 注入 JS/CSS 临时实现
```

KPay：

```text
网关：https://api.kpay.cc/epay
商户 ID：152
异步回调：https://faka.bfsmlt.com/api/v1/payments/callback
同步跳转：https://faka.bfsmlt.com/pay
```

> KPay EPay Key 是敏感信息，禁止写入 Git 仓库。

---

## 5. 需要源码化的第一批改动

优先把当前生产环境的热修正规化：

```text
1. 前台游客购买邮箱字段添加原生 label：邮箱：
2. 前台游客购买密码字段添加原生 label：密码：
3. 给邮箱/密码添加说明文字
4. 暗色模式适配
5. 不再依赖 site_config.scripts 注入 JS/CSS
```

对应位置大概率在 Dujiao-Next `user` 前台仓库的 checkout 页面。

当前从生产二进制中观察到相关前端逻辑在：

```text
Checkout 页面
checkout.guestEmailPlaceholder
checkout.guestPasswordPlaceholder
checkout.guestInstructions
```

---

## 6. 部署原则

生产环境部署前必须：

```bash
ssh vmiss "/usr/local/bin/faka-backup.sh"
```

部署时不得覆盖：

```text
/srv/faka/app/config.yml
/srv/faka/data/db/dujiao.db
/srv/faka/data/uploads/
```

只允许替换：

```text
/srv/faka/app/dujiao-server
```

部署后检查：

```bash
ssh vmiss "systemctl is-active dujiao-next nginx redis-server"
curl -I https://faka.bfsmlt.com/
```

人工检查：

```text
首页
商品详情
游客下单
支付跳转
支付成功回订单详情
卡密显示
交付使用说明显示
后台登录
```

---

## 7. GitHub Actions 目标

最终目标：

```text
push/main 或 workflow_dispatch
→ GitHub Actions 构建 fullstack Linux x86_64 dujiao-server
→ 上传到 /srv/faka/releases
→ 备份当前版本
→ 替换 /srv/faka/app/dujiao-server
→ 重启 dujiao-next
→ 健康检查
```

初期建议只使用手动触发：

```yaml
workflow_dispatch
```

不要一开始就每次 push 自动部署生产。

---

## 8. 禁止提交到 Git 的内容

禁止提交：

```text
KPay EPay Key
后台密码
服务器 SSH 私钥
数据库文件
Cloudflare Token
.env / config.yml 生产配置
```

敏感信息只放：

```text
GitHub Actions Secrets
服务器 /srv/faka/app/config.yml
```

---

## 9. 相关文档

原始项目上下文：

```text
/Users/lutao/vibeCodingProjects/faka/AGENTS.md
```

源码定制规划：

```text
/Users/lutao/vibeCodingProjects/faka/CUSTOM_DEVELOPMENT_PLAN.md
```

---

*最后更新：2026-06-05*
