package catalogwatchhttp

import (
	"errors"
	"strconv"

	siteconnection "github.com/dujiao-next/internal/modules/siteconnection/contract"
	"github.com/dujiao-next/internal/modules/upstreamwatch/application"
	"github.com/dujiao-next/internal/modules/upstreamwatch/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

func RegisterAdminRoutes(admin gin.IRoutes, service *application.Service) {
	admin.GET("/upstream-management/catalog/subscriptions/:id", func(c *gin.Context) {
		id, err := ginutil.ParseParamUint(c, "id")
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		settings, err := service.Settings(c.Request.Context(), id)
		if err != nil {
			respondScopeError(c, err)
			return
		}
		response.Success(c, settings)
	})
	admin.PUT("/upstream-management/catalog/subscriptions/:id", func(c *gin.Context) {
		id, err := ginutil.ParseParamUint(c, "id")
		var input struct {
			CategoryIDs *[]uint `json:"category_ids" binding:"required"`
		}
		if err != nil || c.ShouldBindJSON(&input) != nil || input.CategoryIDs == nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		if err := service.SaveSettings(c.Request.Context(), id, *input.CategoryIDs); err != nil {
			respondScopeError(c, err)
			return
		}
		response.Success(c, nil)
	})
	admin.GET("/upstream-management/catalog/status", func(c *gin.Context) {
		rows, err := service.Overview(c.Request.Context())
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.upstream_products_fetch_failed", err)
			return
		}
		response.Success(c, rows)
	})
	admin.GET("/upstream-management/catalog/products", func(c *gin.Context) {
		page, size := ginutil.ParsePaginationWithBounds(c, "page", "page_size", 20, 50)
		connectionID, connErr := ginutil.ParseQueryUint(c.Query("connection_id"), false)
		productID, productErr := ginutil.ParseQueryUint(c.Query("product_id"), false)
		view := c.Query("view")
		groupBy := c.Query("group_by")
		var categoryID *uint
		if raw, exists := c.GetQuery("category_id"); exists {
			value, err := strconv.ParseUint(raw, 10, strconv.IntSize)
			if err != nil || connectionID == 0 || productID != 0 {
				ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
				return
			}
			id := uint(value)
			categoryID = &id
		}
		if connErr != nil || productErr != nil || (productID != 0 && connectionID == 0) ||
			(view != "" && view != "new" && view != "historical") {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		if groupBy != "" {
			if groupBy != "category" || connectionID == 0 || productID != 0 || categoryID != nil {
				ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
				return
			}
			groups, err := service.CategoryGroups(c.Request.Context(), connectionID, view == "historical")
			if err != nil {
				ginutil.RespondError(c, response.CodeInternal, "error.upstream_products_fetch_failed", err)
				return
			}
			response.Success(c, groups)
			return
		}
		rows, total, err := service.List(c.Request.Context(), contract.ListFilter{
			ConnectionID: connectionID, ProductID: productID, Historical: view == "historical", Page: page, PageSize: size,
			CategoryID: categoryID,
		})
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.upstream_products_fetch_failed", err)
			return
		}
		response.SuccessWithPage(c, rows, response.BuildPagination(page, size, total))
	})
}

func respondScopeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, application.ErrScopeUnsupported):
		ginutil.RespondError(c, response.CodeBadRequest, "error.upstream_scope_unsupported", nil)
	case errors.Is(err, application.ErrScopeCategory):
		ginutil.RespondError(c, response.CodeBadRequest, "error.upstream_scope_category", nil)
	case errors.Is(err, siteconnection.ErrNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.connection_not_found", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.upstream_categories_fetch_failed", err)
	}
}
