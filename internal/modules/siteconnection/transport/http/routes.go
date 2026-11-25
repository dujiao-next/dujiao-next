package siteconnectionhttp

import (
	"errors"

	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

// RegisterAdminRoutes 注册后台站点对接连接路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/site-connections", handler.GetSiteConnections)
	admin.GET("/site-connections/:id", handler.GetSiteConnection)
	admin.POST("/site-connections", handler.CreateSiteConnection)
	admin.PUT("/site-connections/:id", handler.UpdateSiteConnection)
	admin.DELETE("/site-connections/:id", handler.DeleteSiteConnection)
	admin.POST("/site-connections/:id/ping", handler.PingSiteConnection)
	admin.PUT("/site-connections/:id/status", handler.UpdateSiteConnectionStatus)
	admin.POST("/site-connections/:id/reapply-markup", handler.ReapplyConnectionMarkup)
}

func RegisterManagementRoutes(admin gin.IRoutes, service *siteconnectionapp.Management) {
	admin.GET("/upstream-management", func(c *gin.Context) {
		overview, err := service.Overview()
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.connection_fetch_failed", err)
			return
		}
		response.Success(c, overview)
	})
	admin.POST("/upstream-management/:id/refresh", func(c *gin.Context) {
		id, err := ginutil.ParseParamUint(c, "id")
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		balance, err := service.Refresh(c.Request.Context(), id)
		respondBalance(c, balance, err)
	})
	admin.PUT("/upstream-management/:id/threshold", func(c *gin.Context) {
		id, err := ginutil.ParseParamUint(c, "id")
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		var req struct {
			Amount   *string `json:"amount" binding:"required"`
			Currency string  `json:"currency" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		balance, err := service.SetThreshold(id, *req.Amount, req.Currency)
		respondBalance(c, balance, err)
	})
}

func respondBalance(c *gin.Context, balance siteconnectionapp.BalanceView, err error) {
	switch {
	case errors.Is(err, siteconnectioncontract.ErrNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
	case errors.Is(err, siteconnectioncontract.ErrInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
	case err != nil:
		ginutil.RespondError(c, response.CodeInternal, "error.connection_update_failed", err)
	default:
		response.Success(c, balance)
	}
}
