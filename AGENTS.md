# AGENTS.md

本文件约束所有在本仓库工作的 AI 编码助手（Claude Code、Codex、Cursor、Copilot、Gemini 等）。
架构、目录、构建方式的完整说明见 [README.md](README.md)；这里只列 **必须遵守的规则** 和 **最容易踩的坑**。
与 README 冲突时以代码和测试为准，并顺手修正文档。

## 项目速览

- Go 1.26 后端（Gin + GORM，SQLite / PostgreSQL）+ 两个 Vue 3 SPA：`frontend/user/`（前台 :5173）、`frontend/admin/`（后台 :5174）。
- 生产是 **单二进制**：前端产物经 `go:embed`（`-tags fullstack`）打进后端，一个进程一个端口。不带 tag 编译只有 API，本地开发默认走这条。
- 后端是模块化单体：`internal/modules/<name>/{domain,application,infrastructure,transport,contract}`，装配在 `internal/bootstrap/`，跨模块用例在 `internal/workflows/`。

## 工作方式

- 动手前先读相关代码，沿真实调用链走一遍；修 bug 找根因，改共享函数前先查全部调用方。
- **只做任务要求的改动**：不顺手重构、不改无关格式、不升级依赖、不新增依赖（确需新增先说明理由）。
- 优先复用仓库里已有的模块、工具函数和组件，不要重复造轮子。
- 不确定的事实（接口、字段、配置项）去代码里查，不要猜，不要编造不存在的函数或路径。
- 不要创建设计文档、审计报告、计划、AI 过程记录等文件提交进仓库；这类内容放在 PR 描述里。

## 后端硬性规则

1. **分层由测试强制**：`internal/architecture/` 会解析全部 import，违规直接让测试失败。要点：
   - 只有模块的 `infrastructure/gormstore` 可以 import GORM；`application` 不得 import Gin / asynq。
   - 模块之间只能通过对方的 `contract/` 交互，禁止 import 其他模块内部包。
   - `internal/shared` 不依赖任何模块 / GORM / Gin；`internal/platform` 不依赖业务模块。
   - 测试失败时修代码，**不要改 architecture 测试去放行**。
2. **SQLite 自锁死锁（极其重要）**：SQLite 运行在 `MaxOpenConns=1`。事务闭包（`WithinTransaction` / `WithTx(tx)`）内：
   - 所有查询必须走事务句柄或绑定到它的 store，禁止触碰全局 DB 连接——包括间接调用会自己查库的 service（如读取设置）。
   - 需要的配置、设置 **在开启事务前读好**，以变量传入闭包。
   - 禁止在事务内发外部 HTTP 请求（支付网关等），提交后再调用。
   - 同时兼容 SQLite 和 PostgreSQL：不要写只有一种数据库支持的 SQL，注意 SQLite 不支持并发写。
3. **后台 RBAC**：所有 `/api/v1/admin/...` 路由经过 Casbin。新增 / 删除 / 重命名 admin 路由时，必须同步更新
   `internal/authz/bootstrap.go` 的 `BuiltinRoleSeeds()`，否则只有超级管理员能访问。按职责归入角色：
   - 商品 / 分类 / 卡密 / 会员等级 → `operations`
   - 订单 / 退款 / 用户 / 钱包 / 客服 → `support`
   - 上游对接 / 产品映射 / 采购 / 对账 / 凭证 → `integration`
   - 支付 / 财务 / 分销提现 → `finance`
   - 系统设置 / 权限 / 渠道客户端 / Telegram Bot → `system_admin`
   - 全员可读、自助接口（改自己密码、自己 2FA）→ `readonly_auditor`

   策略用 `keyMatch2`，`/admin/products/:id` 会匹配 `/admin/products/batch-status`，加策略前确认是否已被通配覆盖。
   `internal/app/httpserver/rbac_coverage_test.go` 会校验覆盖情况。
4. **i18n**：API 返回给用户的文案走 `internal/i18n/messages.go` 的消息 key，三种语言（`zh-CN`、`zh-TW`、`en-US`）同时补齐。
5. **新增顶层路由前缀**：`/api`、`/uploads`、`/health` 是保留前缀，未命中返回 404 而非 SPA 首页。新增后端顶层前缀时同步更新 `internal/web/handler.go` 的 `reservedPaths`。
6. Go 代码必须 `gofmt`，并通过 `go vet ./...`。

## 前端硬性规则

1. **i18n**：所有用户可见文案不得硬编码，三语同步补齐。
   - user：`frontend/user/src/i18n/locales/{zh-CN,zh-TW,en-US}.json`
   - admin：`frontend/admin/src/i18n/index.ts`
2. **admin 运行时路径**：admin 可挂在任意 `web.admin_path` 下，禁止用构建期常量拼站点前缀。
   - 原生 `<a href>`、`window.location` 跳转必须用 `src/utils/adminBase.ts` 的 `adminUrl()`。
   - `<router-link :to>`、`router.push()` **不要**加前缀（vue-router 已带 base，再加会变成 `/admin/admin/...`）。
3. **user 店面模板**：模板页面在 `src/templates/<name>/`，缺省回落到 `src/views/`，见 `src/templates/registry.ts`。改页面时确认改的是对的那一份。
4. 组件文件 `PascalCase.vue`，工具函数 `camelCase.ts`；admin UI 基于 shadcn-vue / reka-ui，优先用已有组件。
5. 包管理器只用 **pnpm 10.34.3**（`corepack enable`）。不要用 `pnpm --dir X`（读不到子目录 `packageManager`，会选错版本），一律 `cd` 进目录执行。不要提交 npm / yarn 的 lockfile。

## 交付前验证

按改动范围执行，**全部通过才算完成**；无法执行的要在交付说明里写明原因：

| 改动范围 | 必跑命令 |
| --- | --- |
| Go 代码 | `gofmt -l $(git ls-files '*.go')`（应无输出）、`go vet ./...`、`go test ./...` |
| `internal/web/` 或 embed 相关 | 额外：`go build -tags release,fullstack ./cmd/server`（需先构建两个前端并拷到 `internal/web/dist/`，见 README） |
| `frontend/admin/` | `cd frontend/admin && pnpm run test && pnpm run build` |
| `frontend/user/` | `cd frontend/user && pnpm run test && pnpm run build` |
| `scripts/dujiao-next-manager.sh` | `bash -n` + `shellcheck -x -S warning`，以及 `scripts/tests/` 下的测试 |

验证结束后：删除产生的构建产物（`frontend/*/dist/`、`internal/web/dist/`、根目录二进制等），关闭启动过的 dev server / 后端进程。

## 安全与数据

- 不要提交 `config.yml`、`.env`、`db/`、`uploads/`、`logs/` 或任何密钥、Token、真实用户数据；示例配置只改 `config.yml.example`。
- 不要削弱鉴权、签名校验、限流、输入校验来"让功能跑通"。支付回调、金额计算、钱包余额等资金路径改动要格外谨慎，并补测试。
- 不要执行破坏性操作（删库、`git reset --hard`、`git push --force`、改写历史）除非人类明确要求。

## Git 提交

- 只在人类要求时 commit，**不要自行 push**。
- commit message 只写一行 subject，祈使语气，带前缀：`feat:` / `fix:` / `refactor:` / `chore:` / `docs:` / `test:`，中英文均可。例：`fix: 修复商品无法关闭 SKU 规格配置`。
- **不要**在 commit message 中添加 `Co-Authored-By: <AI>`、`Generated with ...` 等任何 AI 署名行。
- 一个 commit / PR 只做一件事；PR 描述写清改动范围、验证结果和已知风险，涉及界面改动附截图。
