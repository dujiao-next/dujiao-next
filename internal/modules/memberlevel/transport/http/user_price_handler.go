package memberlevelhttp

import (
	"errors"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// UserPriceInput 用户专属价输入
type UserPriceInput struct {
	UserID      uint    `json:"user_id" binding:"required"`
	ProductID   uint    `json:"product_id" binding:"required"`
	SKUID       uint    `json:"sku_id"`
	PriceAmount float64 `json:"price_amount" binding:"required"`
}

// BatchUpsertUserPricesRequest 批量设置用户专属价请求
type BatchUpsertUserPricesRequest struct {
	Prices []UserPriceInput `json:"prices" binding:"required,min=1"`
}

// adminUserPriceView 是后台列表的一行：专属价 + 商品/SKU 摘要。
// 摘要随列表一起返回，后台不必再为每一行单独请求商品详情（一页最多 20 次）。
type adminUserPriceView struct {
	memberleveldomain.UserProductPrice
	ProductTitle       jsonmap.JSON `json:"product_title,omitempty"`
	ProductSlug        string       `json:"product_slug,omitempty"`
	ProductActive      bool         `json:"product_active"`
	ProductPriceAmount money.Amount `json:"product_price_amount"`
	SKUCode            string       `json:"sku_code,omitempty"`
	SKUSpecValues      jsonmap.JSON `json:"sku_spec_values,omitempty"`
	SKUPriceAmount     money.Amount `json:"sku_price_amount,omitempty"`
}

// GetUserPrices GET /admin/member-user-prices?user_id= 分页获取用户的专属价
func (h *AdminHandler) GetUserPrices(c *gin.Context) {
	userID, err := ginutil.ParseQueryUint(c.Query("user_id"), false)
	if err != nil || userID == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	page, pageSize := ginutil.ParsePaginationWithKeys(c, "page", "page_size", 50)

	prices, total, err := h.service.ListUserPrices(userID, page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_product_price_fetch_failed", err)
		return
	}

	views := make([]adminUserPriceView, 0, len(prices))
	for _, p := range prices {
		views = append(views, adminUserPriceView{UserProductPrice: p})
	}
	h.attachProductSummaries(views)

	response.SuccessWithPage(c, views, response.BuildPagination(page, pageSize, total))
}

// attachProductSummaries 按当前页一次性批量解析商品与 SKU 摘要并写回视图。
// 解析失败不阻断列表：摘要是展示用的附加信息。
func (h *AdminHandler) attachProductSummaries(views []adminUserPriceView) {
	if h.products == nil || len(views) == 0 {
		return
	}
	productIDs := make([]uint, 0, len(views))
	skuIDs := make([]uint, 0, len(views))
	seenProduct := map[uint]struct{}{}
	seenSKU := map[uint]struct{}{}
	for _, v := range views {
		if _, dup := seenProduct[v.ProductID]; !dup {
			seenProduct[v.ProductID] = struct{}{}
			productIDs = append(productIDs, v.ProductID)
		}
		if v.SKUID > 0 {
			if _, dup := seenSKU[v.SKUID]; !dup {
				seenSKU[v.SKUID] = struct{}{}
				skuIDs = append(skuIDs, v.SKUID)
			}
		}
	}

	productMap, err := h.products.ResolveProducts(productIDs)
	if err != nil {
		return
	}
	skuMap, err := h.products.ResolveSKUs(skuIDs)
	if err != nil {
		skuMap = nil
	}

	for i := range views {
		if product, found := productMap[views[i].ProductID]; found {
			views[i].ProductTitle = product.Title
			views[i].ProductSlug = product.Slug
			views[i].ProductActive = product.IsActive
			views[i].ProductPriceAmount = product.PriceAmount
		}
		if sku, found := skuMap[views[i].SKUID]; found && views[i].SKUID > 0 {
			views[i].SKUCode = sku.SKUCode
			views[i].SKUSpecValues = sku.SpecValues
			views[i].SKUPriceAmount = sku.PriceAmount
		}
	}
}

// BatchUpsertUserPrices POST /admin/member-user-prices/batch 批量设置用户专属价
func (h *AdminHandler) BatchUpsertUserPrices(c *gin.Context) {
	var req BatchUpsertUserPricesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	prices := make([]memberleveldomain.UserProductPrice, 0, len(req.Prices))
	for _, p := range req.Prices {
		prices = append(prices, memberleveldomain.UserProductPrice{
			UserID:      p.UserID,
			ProductID:   p.ProductID,
			SKUID:       p.SKUID,
			PriceAmount: money.FromDecimal(decimal.NewFromFloat(p.PriceAmount)),
		})
	}

	if err := h.service.BatchUpsertUserPrices(prices); err != nil {
		switch {
		case errors.Is(err, memberlevelcontract.ErrUserPriceInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.user_product_price_invalid", nil)
		case errors.Is(err, memberlevelcontract.ErrUserNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_product_price_save_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"saved": true})
}

// DeleteUserPrice DELETE /admin/member-user-prices/:id 删除用户专属价
func (h *AdminHandler) DeleteUserPrice(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.service.DeleteUserPrice(id); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_product_price_delete_failed", err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// UserPriceReader 是当前登录用户读取自己专属价的最小端口。
type UserPriceReader interface {
	ListUserPrices(userID uint, page, pageSize int) ([]memberleveldomain.UserProductPrice, int64, error)
}

// UserHandler 处理登录用户查看自己专属价的请求。
type UserHandler struct {
	prices   UserPriceReader
	products memberlevelcontract.ProductDirectory
}

// NewUserHandler 创建用户专属价 handler。products 可为 nil，此时响应只带 ID 与价格。
func NewUserHandler(prices UserPriceReader, products memberlevelcontract.ProductDirectory) *UserHandler {
	if prices == nil {
		panic("memberlevel user handler: prices is nil")
	}
	return &UserHandler{prices: prices, products: products}
}

type userPriceView struct {
	ProductID     uint         `json:"product_id"`
	SKUID         uint         `json:"sku_id"`
	PriceAmount   money.Amount `json:"price_amount"`
	ProductSlug   string       `json:"product_slug,omitempty"`
	ProductTitle  jsonmap.JSON `json:"product_title,omitempty"`
	ProductActive bool         `json:"product_active"`
	SKUCode       string       `json:"sku_code,omitempty"`
	SKUSpecValues jsonmap.JSON `json:"sku_spec_values,omitempty"`
}

// ListMyPrices GET /me/product-prices 返回当前用户的全部专属价，前台据此展示一口价。
// 商品标题、slug 与 SKU 规格仅用于个人中心展示；解析失败不阻断主体返回。
func (h *UserHandler) ListMyPrices(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	prices, _, err := h.prices.ListUserPrices(userID, 0, 0)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_product_price_fetch_failed", err)
		return
	}
	productMap, skuMap := h.resolveDirectory(prices)
	views := make([]userPriceView, 0, len(prices))
	for _, p := range prices {
		view := userPriceView{ProductID: p.ProductID, SKUID: p.SKUID, PriceAmount: p.PriceAmount}
		if product, found := productMap[p.ProductID]; found {
			view.ProductSlug = product.Slug
			view.ProductTitle = product.Title
			view.ProductActive = product.IsActive
		}
		if sku, found := skuMap[p.SKUID]; found && p.SKUID > 0 {
			view.SKUCode = sku.SKUCode
			view.SKUSpecValues = sku.SpecValues
		}
		views = append(views, view)
	}
	response.Success(c, views)
}

func (h *UserHandler) resolveDirectory(prices []memberleveldomain.UserProductPrice) (map[uint]memberlevelcontract.ProductSummary, map[uint]memberlevelcontract.SKUSummary) {
	productMap := map[uint]memberlevelcontract.ProductSummary{}
	skuMap := map[uint]memberlevelcontract.SKUSummary{}
	if h.products == nil || len(prices) == 0 {
		return productMap, skuMap
	}
	productIDs := make([]uint, 0, len(prices))
	skuIDs := make([]uint, 0, len(prices))
	seenProduct := map[uint]struct{}{}
	seenSKU := map[uint]struct{}{}
	for _, p := range prices {
		if _, dup := seenProduct[p.ProductID]; !dup {
			seenProduct[p.ProductID] = struct{}{}
			productIDs = append(productIDs, p.ProductID)
		}
		if p.SKUID > 0 {
			if _, dup := seenSKU[p.SKUID]; !dup {
				seenSKU[p.SKUID] = struct{}{}
				skuIDs = append(skuIDs, p.SKUID)
			}
		}
	}
	if resolved, err := h.products.ResolveProducts(productIDs); err == nil && resolved != nil {
		productMap = resolved
	}
	if len(skuIDs) > 0 {
		if resolved, err := h.products.ResolveSKUs(skuIDs); err == nil && resolved != nil {
			skuMap = resolved
		}
	}
	return productMap, skuMap
}
