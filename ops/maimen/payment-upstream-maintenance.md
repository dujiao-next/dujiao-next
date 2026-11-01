# 固定 U 支付改造后的上游升级维护

更新：2026-10-09。适用于本 fork 的固定付款意图、独立链核验、结算与履约保护。当前官方基线仍为 `v1.4.8` / `d2e44618de03068cbbe874a23a6d7c6a249a79ec`，维护分支 `maimen-fullstack`。本文件不是升级脚本，也不代表已经试合并某个新上游版本。

## 结论与改动规模

商城源码确实有改动，不能把增强网关独立部署后继续用完全未修改的商城完成同等安全流程。商城必须自己固定付款条件、核实到账、去重结算，并让所有发货入口遵守保护；只加强网关仍会保留“网关报告成功即发货”的信任问题。

可以持续合并上游，但不能承诺零冲突、无感更新或只保留前端定制。核对时，相对商城原 HEAD `ad74853445bc820af5d1d6426a4fe03399fcbd06`：

- 62 个受跟踪文件有未提交修改，其中 51 个实现 / 配置文件、11 个测试文件；总计新增 3059 行、删除 307 行，包含测试和格式调整，不全是业务逻辑。
- 另有 42 个新增实现 / 配置文件、21 个新增测试文件、7 个已有新增说明文件；本维护手册不计在该快照内。
- 新的核验、结算、候选、收银和告警大部分放在独立模块 / 文件中，但接入了支付、发货、通知、会员等级、佣金、迁移、worker 装配和前台公共入口。共享通知发送与下游回调也有重构，不能声称其他支付完全不受源码影响。

当前全套测试通过只支持当前源码的回归结论，不能证明未知未来版本仍兼容。此次没有 fetch 新上游、merge/rebase、提交、推送或生产操作。

## 升级影响评估

| 上游变化 | 通常影响 | 必须检查 |
| --- | --- | --- |
| 文案、独立页面、无关商品展示 | 通常较低 | 两模板 / i18n 和构建；共享组件仍可能有文本冲突 |
| 支付页、API 返回、支付单模型、配置 | 中等 | fixed 标记、只读收银、旧字段清空、原授权 / 期限保存 |
| 支付回调、订单生命周期、发货、通知 / 重试 | 较高 | 不能仅以解决 Git 冲突完成；核对下面所有资金不变量 |
| 订单 / 钱包 / 库存事务、队列、迁移、模块架构大改 | 高 | 逐项迁移接入点，真实隔离 PostgreSQL / Redis 验收及回退策略；必要时单独安排适配版本 |

这些是按变化类型的工程判断，不是已测出的升级耗时。版本号“补丁更新”也可能包含高影响修改，必须看实际 diff。

## 接入点与不能丢失的行为

以下路径相对仓库根；具体代码地图以模块 README 为准。

| 接入面 | 主要路径 | 升级不变量 |
| --- | --- | --- |
| 创建 / 报价恢复 | `payment/application/payment_service_create.go`、`payment_service_provider.go`、`gateway/epusdt` | 原授权先持久化，丢响应重试原单；精确网络 / 合约 / 地址 / 整数金额 / 绝对期限，不能重建新单延长期限 |
| 回调与钱包 | `payment_service_callback.go`、`payment_service_channel_rules.go`、`domain/payment.go` | 普通 / 自定义 / 兼容回调都不能结算 fixed 单，含缺快照的损坏历史；会员普通订单范围，不能偷偷转钱包 |
| 持久候选与独立查链 | `payment/application/{candidates,chainverification}`、`infrastructure/chainrpc` | 落库后 ACK，历史来源保留，冲突不覆盖；两独立来源 / 主网锚点 / 最终性，无单源降级 |
| 结算事务 | `payment/application/settlement`、`infrastructure/gormstore` | 独立 allocation、唯一 event consumption、资金 / 订单 / outbox 原子提交；不得把 HTTP / RPC 放进长事务 |
| 发货与旧队列 | `fulfillment/application/service.go`、`fulfillment/infrastructure/gormstore/payment_guard.go`、`app/jobs/consumer` | 自动、人工和历史队列均执行 fixed guard；卡密落库才确认 effect，队列成功不是发货成功 |
| 通知和下游 | `notification/application`、`notification/infrastructure/smtp`、`downstreamcallback/application/service.go` | 同步发送接口 ACK、逐目标确认 / 目标集合、重试与冻结准入；防止旧队列重复派发固定支付消息，普通付款和退款原语义也需回归 |
| 会员等级 / 佣金 | `memberlevel/application/service.go`、`affiliate/application/commission.go` | 对应 SQL 变更与 effect 确认同事务；重放不重复累计，退款后旧升级不能复活 |
| 冻结 / 人工记录 | `infrastructure/gormstore/automation`、`cmd/payment-ops` | 缺控制行拒绝、默认 frozen、revision / operator audit 原子性；无强制成功后门，冻结不影响证据和告警 |
| 迁移 / 装配 | `bootstrap/database/migrations/registry.go`、`app/container`、`app/jobs/service.go`、`queue/tasks.go`、`config/config.go` | 新表 / 索引和订阅不丢、重复迁移不解冻；worker 注册完整，三个开关默认 false |
| 两套前台 | `FixedUsdtCashier.vue`、`usePayment.ts`、`views/Payment.vue`、`templates/vault/Payment.vue`、API 类型 / i18n | 商城同源受权数据，无网关展示回退；旧响应不能在换单 / 冻结 / 到期后恢复地址；金额保持字符串精度 |
| 构建 / 配置 | `go.mod`、`Dockerfile`、`.github/workflows/maimen-fullstack.yml`、`ops/maimen/upstream.json` | 工具链 / 锁文件一致，包含 `/app/payment-ops`，保持私有配置排除和镜像验证；不能直接用官方二进制覆盖 fork |

表中模块短路径均位于 `internal/modules/`，除非已写明 `internal/app` 等基础目录；检索实际文件时可用 `rg --files internal`。新文件降低同文件冲突概率，但接口 / 事务语义变化仍会影响它们。

## 推荐升级流程

1. 先把已经审核、验收的本轮改造保存为可追踪提交；当前有大量未提交修改，不能在其上直接 merge/rebase 或用 stash 掩盖。按授权发布，记录商城和网关的兼容版本，不仅记上游 tag。
2. 在独立任务分支 / worktree 获取明确的上游 tag/commit，确认官方基线是共同祖先，阅读 release notes、schema、队列与工具链 diff。不要在生产 checkout 自动合并。
3. 对新上游与旧基线的 diff，逐项对照上表。保留上游需要的安全修复和功能，同时迁移本 fork 的约束；不能批量选择 ours/theirs 或只看“没有冲突”。不把安全钩子删掉来通过编译。
4. 合并完成后对比“新上游 → fork”的差异，确认改造仍可解释、未意外恢复旧回调 / 官方自动覆盖更新入口。更新基线记录、版本号和本手册的接入点。
5. 完成下方验证，检查依赖可达漏洞；构建准确提交的全栈镜像并运行隔离 smoke。记录结果及兼容的网关版本。前次 196 包通过不代替新版本测试。
6. 生产升级先冻结固定 U 支付、按手册备份 / 明确迁移回退兼容，再部署。若新版本改变订单 / 支付事实模型，须检查历史未付款、已付款待发货、待通知和人工异常记录；不能为了升级清空队列或支付表。
7. 更新父层 HANDOFF / Task / MAINTENANCE / PROJECT_MAP 和商城私有交接。恢复时保留新数据库标记 / 消费记录，不能把不认识 fixed guard 的旧商城代码直接接回生产数据库。

通常继续采用已有 merge 流程保留上游历史；无需每个版本从头手工复制整套改造，也不建议重写已公开 fork 历史。按职责组织提交方便审查，但跨模块约束需要配套发布，不能假设每个提交都可独立启用。

## 验证要求与当前 CI 边界

`maimen-fullstack.yml` 保留 Go vet / 全套测试、前后台测试 / 构建、部署模板与发布前镜像 smoke。2026-10-09 工作区新增 `payment-security` job：GitHub 托管的隔离 PostgreSQL16.14 / Redis7.4.9、支付及关联模块竞态、两入口 govulncheck v1.8.0；image 必须等待该 job 成功。`ops/maimen/scripts/check_payment_test_report.py` 要求三个 TestPostgresPayment 场景和对应包完成通过，缺项、skip、截断或任何包 fail 都拒绝。普通 `go test ./...` 本身仍会跳过缺专用变量的 DB 测试。

测试使用专用 loopback payment_lab 和仅 runner 的 dujiao-payment-redis13 别名 / DB15，不放宽原隔离条件，不读取生产配置。镜像 smoke 新增实际执行 `/app/payment-ops -action status`，要求合成新库默认 frozen / revision1。上述 CI 和镜像检查尚未在 GitHub / 正式镜像运行；本轮仅 9 项脚本测试、YAML / Python 语法与模板检查通过，不将其标为完成远端验收。

| 验证 | 重点 |
| --- | --- |
| `go test ./...`、`go vet ./...` | 全站回归及架构规则，包含普通支付 / 钱包 / 下游通知，不只 fixed 分支 |
| `go test -race -count=1 ./internal/persistence/integrationtest/payment ./internal/modules/payment/... ./internal/app/jobs/consumer` | 冻结竞争、租约、重复结算、消息 / 发货、改快照与旧任务 |
| 专用 PG / Redis 的 `TestPostgresPayment` | 迁移、唯一事件 / 事务回滚 / 行锁、Redis 丢队列后恢复；必须确认未 skip，绝不填生产 DSN |
| 两前台测试 / 构建 | Classic / Vault 收银资格、期限、精确金额、旧字段不泄漏 |
| 入口 govulncheck | 商城 server / payment-ops，配套网关 secure；区分模块 / 包 / 可达调用链 |
| 镜像 / 兼容验证 | 新二进制和嵌入前端、payment-ops、配置 / schema，保持回滚到兼容版本 |

PG / Redis 专用变量、工作目录和完整命令见 [冻结手册](payment-automation-control.md) 及仓库私有维护指南。行为测试位于 `internal/persistence/integrationtest/payment/`，不是旧的 application/settlement 测试目录。

## 能否进一步减少耦合

后续可优先把 `payment_service_provider.go` / `payment_service_callback_dispatch.go` 的较长 fixed 专用编排搬进现有独立服务，保留短而明确的入口钩子；先完善自动 PG/Redis / race 回归，再做这种结构优化。迁移文件本身仍需一次验证，不能为“少改文件”集中成一个大文件。

把独立核验搬成另一个网络服务不会自动消除商城结算、冻结和发货约束，还会新增服务认证 / 故障一致性问题。直接去掉商城改造改回普通成功回调，会削弱本次安全目标，不作为低维护方案默认执行。
