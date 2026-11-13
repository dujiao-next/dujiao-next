package adminapitokenbootstrap

import (
	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/logger"
	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	tokenhttp "github.com/dujiao-next/internal/modules/identity/adminapitoken/transport/http"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func NewHandler(c *container.Container) *tokenhttp.Handler {
	return tokenhttp.NewHandler(c.AdminAPITokenService, func(event tokenhttp.AuditEvent) {
		if err := c.AuthzAuditService.Record(auditlogapp.AuthzRecord{
			OperatorAdminID: event.OperatorID, OperatorUsername: event.Username,
			TargetAdminID: &event.AdminID, Action: event.Action, RequestID: event.RequestID,
			Detail: jsonmap.JSON{"token_id": event.TokenID},
		}); err != nil {
			logger.Warnw("admin_api_token_audit_failed", "token_id", event.TokenID, "error", err)
		}
	})
}
