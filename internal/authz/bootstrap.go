package authz

import "fmt"

// RoleSeed 预置角色定义
type RoleSeed struct {
	Role      string
	Inherits  []string
	Policies  []Policy
	Immutable bool
}

// BuiltinRoleSeeds 系统预置角色矩阵
func BuiltinRoleSeeds() []RoleSeed {
	return []RoleSeed{
		{
			Role: "readonly_auditor",
			Policies: []Policy{
				{Object: "/admin/*", Action: "GET"},
				{Object: "/admin/password", Action: "PUT"},                       // 所有管理员均可修改自己密码
				{Object: "/admin/ads/impression", Action: "POST"},                // 广告曝光埋点，所有管理员可触发
				{Object: "/admin/2fa/setup", Action: "POST"},                     // 自助绑定 2FA
				{Object: "/admin/2fa/enable", Action: "POST"},                    // 自助启用 2FA
				{Object: "/admin/2fa/disable", Action: "POST"},                   // 自助关闭 2FA
				{Object: "/admin/2fa/recovery-codes/regenerate", Action: "POST"}, // 重新生成恢复码
			},
			Immutable: true,
		},
		{
			Role:     "operations",
			Inherits: []string{"readonly_auditor"},
			Policies: []Policy{
				{Object: "/admin/products", Action: "*"},
				{Object: "/admin/products/:id", Action: "*"},
				{Object: "/admin/products/:id/wholesale-prices", Action: "PATCH"},
				{Object: "/admin/categories", Action: "*"},
				{Object: "/admin/categories/:id", Action: "*"},
				{Object: "/admin/categories/:id/active", Action: "PATCH"},
				{Object: "/admin/posts", Action: "*"},
				{Object: "/admin/posts/:id", Action: "*"},
				{Object: "/admin/banners", Action: "*"},
				{Object: "/admin/banners/:id", Action: "*"},
				{Object: "/admin/coupons", Action: "*"},
				{Object: "/admin/coupons/:id", Action: "*"},
				{Object: "/admin/promotions", Action: "*"},
				{Object: "/admin/promotions/:id", Action: "*"},
				{Object: "/admin/card-secrets", Action: "*"},
				{Object: "/admin/card-secrets/:id", Action: "*"},
				{Object: "/admin/card-secrets/batch", Action: "POST"},
				{Object: "/admin/card-secrets/import", Action: "POST"},
				{Object: "/admin/card-secrets/batch-status", Action: "PATCH"},
				{Object: "/admin/card-secrets/batch-delete", Action: "POST"},
				{Object: "/admin/card-secrets/export", Action: "POST"},
				{Object: "/admin/card-secrets/export-available", Action: "POST"},
				{Object: "/admin/card-secrets/stats", Action: "GET"},
				{Object: "/admin/card-secrets/batches", Action: "GET"},
				{Object: "/admin/card-secrets/template", Action: "GET"},
				{Object: "/admin/gift-cards", Action: "*"},
				{Object: "/admin/gift-cards/:id", Action: "*"},
				{Object: "/admin/gift-cards/generate", Action: "POST"},
				{Object: "/admin/gift-cards/batch-status", Action: "PATCH"},
				{Object: "/admin/gift-cards/export", Action: "POST"},
				{Object: "/admin/upload", Action: "POST"},
				{Object: "/admin/media/batch-delete", Action: "POST"},
				{Object: "/admin/media/:id", Action: "PUT"},
				{Object: "/admin/media/:id", Action: "DELETE"},
				{Object: "/admin/affiliates/users", Action: "GET"},
				{Object: "/admin/affiliates/users/:id/status", Action: "PATCH"},
				{Object: "/admin/affiliates/users/batch-status", Action: "PATCH"},
				// 会员等级管理
				{Object: "/admin/member-levels", Action: "*"},
				{Object: "/admin/member-levels/:id", Action: "*"},
				{Object: "/admin/member-levels/backfill", Action: "POST"},
				{Object: "/admin/member-level-prices", Action: "*"},
				{Object: "/admin/member-level-prices/batch", Action: "POST"},
				{Object: "/admin/member-level-prices/:id", Action: "DELETE"},
				{Object: "/admin/post-categories", Action: "GET"},
				{Object: "/admin/post-categories", Action: "POST"},
				{Object: "/admin/post-categories/:id", Action: "PUT"},
				{Object: "/admin/post-categories/:id", Action: "DELETE"},
				{Object: "/admin/post-categories/:id/status", Action: "PATCH"},
			},
			Immutable: true,
		},
		{
			Role:     "support",
			Inherits: []string{"readonly_auditor"},
			Policies: []Policy{
				{Object: "/admin/orders", Action: "GET"},
				{Object: "/admin/orders/:id", Action: "GET"},
				{Object: "/admin/orders/:id", Action: "PATCH"},
				{Object: "/admin/orders/:id/fulfillment/download", Action: "GET"},
				{Object: "/admin/orders/:id/refund-to-wallet", Action: "POST"},
				{Object: "/admin/orders/:id/manual-refund", Action: "POST"},
				{Object: "/admin/order-refunds", Action: "GET"},
				{Object: "/admin/order-refunds/:id", Action: "GET"},
				{Object: "/admin/fulfillments", Action: "POST"},
				{Object: "/admin/users", Action: "GET"},
				{Object: "/admin/users/:id", Action: "GET"},
				{Object: "/admin/users/:id", Action: "PUT"},
				{Object: "/admin/users/batch-status", Action: "PUT"},
				{Object: "/admin/users/:id/coupon-usages", Action: "GET"},
				{Object: "/admin/users/:id/wallet", Action: "GET"},
				{Object: "/admin/users/:id/wallet/transactions", Action: "GET"},
				{Object: "/admin/users/:id/wallet/adjust", Action: "POST"},
				{Object: "/admin/users/:id/member-level", Action: "PUT"},
				{Object: "/admin/users/:id/oauth/telegram", Action: "DELETE"},
				{Object: "/admin/users/:id/2fa", Action: "DELETE"}, // 客服协助用户重置丢失 TOTP+恢复码 的 2FA
				{Object: "/admin/user-login-logs", Action: "GET"},
				{Object: "/admin/wallet/recharges", Action: "GET"},
				{Object: "/admin/payments", Action: "GET"},
				{Object: "/admin/payments/:id", Action: "GET"},
				{Object: "/admin/gift-cards", Action: "GET"},
			},
			Immutable: true,
		},
		{
			Role:     "integration",
			Inherits: []string{"readonly_auditor"},
			Policies: []Policy{
				{Object: "/admin/site-connections", Action: "*"},
				{Object: "/admin/site-connections/:id", Action: "*"},
				{Object: "/admin/site-connections/:id/ping", Action: "POST"},
				{Object: "/admin/site-connections/:id/status", Action: "PUT"},
				{Object: "/admin/site-connections/:id/reapply-markup", Action: "POST"},
				{Object: "/admin/product-mappings", Action: "*"},
				{Object: "/admin/product-mappings/:id", Action: "*"},
				{Object: "/admin/product-mappings/:id/sync", Action: "POST"},
				{Object: "/admin/product-mappings/:id/status", Action: "PUT"},
				{Object: "/admin/product-mappings/import", Action: "POST"},
				{Object: "/admin/product-mappings/batch-import", Action: "POST"},
				{Object: "/admin/product-mappings/batch-sync", Action: "POST"},
				{Object: "/admin/product-mappings/batch-status", Action: "POST"},
				{Object: "/admin/product-mappings/batch-delete", Action: "POST"},
				{Object: "/admin/procurement-orders", Action: "GET"},
				{Object: "/admin/procurement-orders/stats", Action: "GET"},
				{Object: "/admin/procurement-orders/:id", Action: "GET"},
				{Object: "/admin/procurement-orders/:id/upstream-payload/download", Action: "GET"},
				{Object: "/admin/procurement-orders/:id/retry", Action: "POST"},
				{Object: "/admin/procurement-orders/:id/cancel", Action: "POST"},
				{Object: "/admin/reconciliation/run", Action: "POST"},
				{Object: "/admin/reconciliation/jobs", Action: "GET"},
				{Object: "/admin/reconciliation/jobs/:id", Action: "GET"},
				{Object: "/admin/reconciliation/items/:id/resolve", Action: "PUT"},
				{Object: "/admin/api-credentials", Action: "*"},
				{Object: "/admin/api-credentials/:id", Action: "*"},
				{Object: "/admin/api-credentials/:id/approve", Action: "POST"},
				{Object: "/admin/api-credentials/:id/reject", Action: "POST"},
				{Object: "/admin/api-credentials/:id/status", Action: "PUT"},
				{Object: "/admin/upstream-products", Action: "GET"},
				{Object: "/admin/resellers/operations/overview", Action: "GET"},
				{Object: "/admin/resellers/profiles", Action: "GET"},
				{Object: "/admin/resellers/profiles/:id", Action: "GET"},
				{Object: "/admin/resellers/profiles/:id", Action: "PUT"},
				{Object: "/admin/resellers/profiles/:id/system-domain", Action: "PUT"},
				{Object: "/admin/resellers/profiles/:id/approve", Action: "POST"},
				{Object: "/admin/resellers/profiles/:id/reject", Action: "POST"},
				{Object: "/admin/resellers/domains", Action: "GET"},
				{Object: "/admin/resellers/domains/:id/approve", Action: "POST"},
				{Object: "/admin/resellers/domains/:id/set-primary", Action: "POST"},
				{Object: "/admin/resellers/site-configs", Action: "GET"},
				{Object: "/admin/resellers/site-configs/:reseller_id", Action: "GET"},
				{Object: "/admin/resellers/site-configs/:reseller_id", Action: "PUT"},
				{Object: "/admin/resellers/site-configs/:reseller_id/reset", Action: "POST"},
				{Object: "/admin/resellers/product-settings", Action: "GET"},
				{Object: "/admin/resellers/product-settings/:reseller_id/:product_id", Action: "GET"},
				{Object: "/admin/resellers/product-settings/:reseller_id/:product_id/preview", Action: "POST"},
				{Object: "/admin/resellers/product-settings/:reseller_id/:product_id", Action: "PUT"},
				{Object: "/admin/resellers/product-settings/:reseller_id/:product_id", Action: "DELETE"},
			},
			Immutable: true,
		},
		{
			Role:     "finance",
			Inherits: []string{"readonly_auditor"},
			Policies: []Policy{
				{Object: "/admin/payments", Action: "GET"},
				{Object: "/admin/payments/:id", Action: "GET"},
				{Object: "/admin/payments/export", Action: "GET"},
				{Object: "/admin/payment-channels", Action: "*"},
				{Object: "/admin/payment-channels/:id", Action: "*"},
				{Object: "/admin/orders", Action: "GET"},
				{Object: "/admin/orders/:id", Action: "GET"},
				{Object: "/admin/orders/:id/refund-to-wallet", Action: "POST"},
				{Object: "/admin/orders/:id/manual-refund", Action: "POST"},
				{Object: "/admin/order-refunds", Action: "GET"},
				{Object: "/admin/order-refunds/:id", Action: "GET"},
				{Object: "/admin/affiliates/commissions", Action: "GET"},
				{Object: "/admin/affiliates/withdraws", Action: "GET"},
				{Object: "/admin/affiliates/withdraws/:id/reject", Action: "POST"},
				{Object: "/admin/affiliates/withdraws/:id/pay", Action: "POST"},
				{Object: "/admin/resellers/operations/finance", Action: "GET"},
				{Object: "/admin/resellers/ledger-entries", Action: "GET"},
				{Object: "/admin/resellers/balance-accounts", Action: "GET"},
				{Object: "/admin/resellers/withdraws", Action: "GET"},
				{Object: "/admin/resellers/withdraws/:id/reject", Action: "POST"},
				{Object: "/admin/resellers/withdraws/:id/pay", Action: "POST"},
				{Object: "/admin/gift-cards", Action: "GET"},
				{Object: "/admin/gift-cards/export", Action: "POST"},
				{Object: "/admin/wallet/recharges", Action: "GET"},
			},
			Immutable: true,
		},
		{
			Role:     "system_admin",
			Inherits: []string{"readonly_auditor"},
			Policies: []Policy{
				// 系统设置
				{Object: "/admin/settings", Action: "*"},
				{Object: "/admin/settings/smtp", Action: "*"},
				{Object: "/admin/settings/smtp/test", Action: "POST"},
				{Object: "/admin/settings/captcha", Action: "*"},
				{Object: "/admin/settings/telegram-auth", Action: "*"},
				{Object: "/admin/settings/notification-center", Action: "*"},
				{Object: "/admin/settings/notification-center/logs", Action: "GET"},
				{Object: "/admin/settings/notification-center/test", Action: "POST"},
				{Object: "/admin/settings/notifications", Action: "*"},
				{Object: "/admin/settings/notifications/logs", Action: "GET"},
				{Object: "/admin/settings/notifications/test", Action: "POST"},
				{Object: "/admin/settings/order-email-template", Action: "*"},
				{Object: "/admin/settings/order-email-template/reset", Action: "POST"},
				{Object: "/admin/settings/affiliate", Action: "*"},
				{Object: "/admin/settings/telegram-bot", Action: "*"},
				{Object: "/admin/settings/telegram-bot/runtime-status", Action: "GET"},
				// 权限管理（仅 system_admin 可操作）
				{Object: "/admin/authz/me", Action: "GET"},
				{Object: "/admin/authz/roles", Action: "*"},
				{Object: "/admin/authz/roles/:role", Action: "*"},
				{Object: "/admin/authz/roles/:role/policies", Action: "GET"},
				{Object: "/admin/authz/admins", Action: "*"},
				{Object: "/admin/authz/admins/:id", Action: "*"},
				{Object: "/admin/authz/admins/:id/roles", Action: "*"},
				{Object: "/admin/authz/admins/:id/2fa/reset", Action: "POST"}, // 超管重置目标管理员 2FA（handler 仍二次校验 isSuper）
				{Object: "/admin/authz/policies", Action: "*"},
				{Object: "/admin/authz/permissions/catalog", Action: "GET"},
				{Object: "/admin/authz/audit-logs", Action: "GET"},
				// 系统信息与版本检测
				{Object: "/admin/system/version/check", Action: "GET"},
				// 渠道客户端管理
				{Object: "/admin/channel-clients", Action: "*"},
				{Object: "/admin/channel-clients/:id", Action: "*"},
				{Object: "/admin/channel-clients/:id/status", Action: "PUT"},
				{Object: "/admin/channel-clients/:id/reset-secret", Action: "POST"},
				// Telegram Bot 群发
				{Object: "/admin/telegram-bot/broadcasts", Action: "*"},
				{Object: "/admin/telegram-bot/users", Action: "GET"},
				// 合规声明（GET 已由 readonly_auditor 的 /admin/* GET 通配覆盖）
				{Object: "/admin/compliance/acknowledge", Action: "POST"},
				{Object: "/admin/resellers/profiles/:id/disable", Action: "POST"},
				{Object: "/admin/resellers/profiles/:id/restore", Action: "POST"},
				{Object: "/admin/resellers/domains/:id/disable", Action: "POST"},
			},
			Immutable: true,
		},
	}
}

// BootstrapBuiltinRoles 初始化预置角色与默认策略
func (s *Service) BootstrapBuiltinRoles() error {
	if s == nil || s.enforcer == nil {
		return fmt.Errorf("authz service unavailable")
	}

	changed := false
	for _, seed := range BuiltinRoleSeeds() {
		role, err := NormalizeRole(seed.Role)
		if err != nil {
			return err
		}

		exists, err := s.enforcer.HasNamedGroupingPolicy("g", role, roleAnchor)
		if err != nil {
			return fmt.Errorf("check builtin role failed: %w", err)
		}
		if !exists {
			added, err := s.enforcer.AddNamedGroupingPolicy("g", role, roleAnchor)
			if err != nil {
				return fmt.Errorf("create builtin role failed: %w", err)
			}
			if added {
				changed = true
			}
		}

		for _, parent := range seed.Inherits {
			parentRole, err := NormalizeRole(parent)
			if err != nil {
				return err
			}
			added, err := s.enforcer.AddNamedGroupingPolicy("g", role, parentRole)
			if err != nil {
				return fmt.Errorf("link role inheritance failed: %w", err)
			}
			if added {
				changed = true
			}
		}

		for _, policy := range seed.Policies {
			action := NormalizeAction(policy.Action)
			if action == "" {
				return fmt.Errorf("builtin policy action is required")
			}
			added, err := s.enforcer.AddPolicy(role, NormalizeObject(policy.Object), action)
			if err != nil {
				return fmt.Errorf("add builtin policy failed: %w", err)
			}
			if added {
				changed = true
			}
		}
	}

	if changed {
		return s.saveAndReload()
	}
	return nil
}
