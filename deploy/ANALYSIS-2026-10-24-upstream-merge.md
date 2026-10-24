# 上游合并分析与记录（2026-10-24）

## 结论

- fork `main`（`d63a6602`）已合入上游 `dujiao-next/dujiao-next` main 的全部 60 个提交
  （合并基点 `d2e44618`，2026-09-16 → 上游最新 `41b86260`，2026-10-11）。
- 合并分支：`kano/merge-upstream-20261024`，合并提交 `56a50045`。
- 20 个文件冲突全部解决；Go 测试、go vet、gofmt、前端（user/admin）测试与构建全部通过。
- 前置事实：`kano/remove-oidc-redirect` 已通过 PR #21 合入 `origin/main` 并发布
  （镜像标签已到 v0.1.0-kano.19），本地 main 本次同步到 `d63a6602`（含 PR #22）。

## 上游主要内容（60 提交）

- 新功能：邀请码注册（`invite_only_enabled` 设置、`invite_registration.go`、
  前端 `?invite=` 链接透传、`InviteCodeDialog`）。
- 安全：上游图片下载防 SSRF 并限制大小/扩展名；上传目录禁执行脚本防同源 XSS；
  启用 2FA / 重置管理员密码后清理鉴权缓存。
- 限流：注册/忘记密码/发送验证码限流独立为 `authRule`（20 次/5 分钟，按 IP）。
- 采购单/分销：状态并发回退、重复交付通知、上游拒单重试策略、子订单退款限额等十余项修复。
- 重构：identity/user 与 memberlevel 重复的用户存储方法删除（只在 memberlevel 保留）；
  分页辅助统一到 gormutil；登录设置处理器合并。
- 其余：佣金重试、超卖防护（编辑商品保留实时库存）、验证码先落库后发信等。

## 冲突解决要点（fork 定制 vs 上游）

| 区域 | 处理 |
| --- | --- |
| AGENTS.md | 两边各自新增：上游 AI 协作规则在前，fork 部署工作流附后 |
| 路由/装配 | 同时保留 fork 的 `SSOAuth` 处理器与上游的 `Invite` 处理器及 `authRateLimit` |
| SendVerifyCode/Register | 采用上游新签名（含 `inviteCode` 参数），保留 fork 的 `requireLocalIdentityManagement()` 守卫（SSO-only 时拒绝本地身份流程） |
| google/telegram 登录 | 上游拆出带 inviteCode 的私有方法，fork 守卫移入私有方法顶部，覆盖全部调用方 |
| identity/user Store | 跟随上游删除 `IncrementTotalRecharged/IncrementTotalSpent/UpdateMemberLevelIfCurrent`（memberlevel 契约保留同名方法）；保留 fork 的 `UpdateFields`（admin/google/profile 4 处调用） |
| 注册前端 | SSO-only 分支保留 fork 内嵌 Passport 流程；本地注册分支带上游 `invite_code` |
| i18n | `ssoMfa`（fork）与 `inviteOnlyHint`（上游）三语并存 |
| Users.vue | nickname 同时保留 fork 的 `:disabled="!identityEditable"` 与上游 `maxlength="32"` |
| 架构守卫 | `userauth/application` 生产文件预算 13→14（上游新增 invite_registration.go），注释已更新 |
| sso_policy_test | Register/SendVerifyCode 调用补 inviteCode 空参 |

## 验证

- `go build ./...`、`go vet ./...`、`gofmt -l`（无输出）、`go test ./...` 全部通过。
- `frontend/user`：4 个测试文件 11+ 用例通过，构建成功。
- `frontend/admin`：4 个测试文件 30 用例通过，构建成功。
- 构建产物已清理；`internal/web/` 无改动，无需本地 fullstack 构建（CI 会构建）。

## 下一步

1. 推送分支并发起 PR，等待 `ci` 工作流通过后合入 fork `main`。
2. 合并后打新标签（沿用 `vX.Y.Z-N` 命名，当前基线见 RELEASE-2026-10-08-version-naming.md），发布镜像。
3. VPS 更新前按 deploy/README.md 备份；本次上游含数据库无关变更，但仍按完整流程执行。
4. 部署后重点回归：登录（SSO-only 与本地降级）、注册（含邀请码开关关闭时）、
   采购单状态流转、佣金创建。线上 SSO-only 模式下邀请码注册不生效（本地注册被守卫拒绝），属预期。
5. 后续可评估：是否把上游邀请码与 Casdoor 统一账号体系打通（当前为独立能力）。
