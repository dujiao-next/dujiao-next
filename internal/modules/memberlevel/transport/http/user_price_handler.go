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

// resolveProductAndSKUMaps 按去重后的 ID 批量解析商品与 SKU 摘要。
// 查询失败不阻断调用方：对应的 map 为空时按"未找到"处理；
// skuOK 表示 SKU 查询成功，调用方据此区分"SKU 已删除"与"查询失败"。
func resolveProductAndSKUMaps(
	dir memberlevelcontract.ProductDirectory,
	productIDs, skuIDs []uint,
) (productMap map[uint]memberlevelcontract.ProductSummary, skuMap map[uint]memberlevelcontract.SKUSummary, skuOK bool) {
	productMap = map[uint]memberlevelcontract.ProductSummary{}
	skuMap = map[uint]memberlevelcontract.SKUSummary{}
	if dir == nil {
		return productMap, skuMap, false
	}
	if len(productIDs) > 0 {
		if resolved, err := dir.ResolveProducts(productIDs); err == nil && resolved != nil {
			productMap = resolved
		}
	}
	if len(skuIDs) == 0 {
		return productMap, skuMap, true
	}
	resolved, err := dir.ResolveSKUs(skuIDs)
	if err != nil || resolved == nil {
		return productMap, skuMap, false
	}
	return productMap, resolved, true
}

// collectUserPriceIDs 收集商品与 SKU 的去重 ID，供批量解析使用。
func collectUserPriceIDs(prices []memberleveldomain.UserProductPrice) (productIDs, skuIDs []uint) {
	productIDs = make([]uint, 0, len(prices))
	skuIDs = make([]uint, 0, len(prices))
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
	return productIDs, skuIDs
}

// attachProductSummaries 按当前页一次性批量解析商品与 SKU 摘要并写回视图。
// 解析失败不阻断列表：摘要是展示用的附加信息。
func (h *AdminHandler) attachProductSummaries(views []adminUserPriceView) {
	if h.products == nil || len(views) == 0 {
		return
	}
	prices := make([]memberleveldomain.UserProductPrice, 0, len(views))
	for _, v := range views {
		prices = append(prices, v.UserProductPrice)
	}
	productIDs, skuIDs := collectUserPriceIDs(prices)
	productMap, skuMap, _ := resolveProductAndSKUMaps(h.products, productIDs, skuIDs)
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
// sku_id > 0 但查不到对应 SKU 的行不返回（SKU 已被删除，下单时也不会生效）。
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
	productIDs, skuIDs := collectUserPriceIDs(prices)
	productMap, skuMap, skuOK := resolveProductAndSKUMaps(h.products, productIDs, skuIDs)
	views := make([]userPriceView, 0, len(prices))
	for _, p := range prices {
		// SKU 级专属价若 SKU 已确认被删除，则跳过（下单时不会生效，展示了反而让人困惑）。
		// 目录查询失败时 skuOK 为 false，此时保留该行，退化为只显示价格。
		if p.SKUID > 0 && skuOK {
			if _, found := skuMap[p.SKUID]; !found {
				continue
			}
		}
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
