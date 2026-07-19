package upstream

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"
	upstreamadapter "github.com/dujiao-next/internal/upstream"

	"github.com/gin-gonic/gin"
)

// Handler 上游 API 处理器（本站作为 B 站暴露给下游 A 站的接口）
type Handler struct {
	*provider.Container
	downstreamRefRepo repository.DownstreamOrderRefRepository
}

// New 创建上游处理器
func New(c *provider.Container, downstreamRefRepo repository.DownstreamOrderRefRepository) *Handler {
	return &Handler{
		Container:         c,
		downstreamRefRepo: downstreamRefRepo,
	}
}

// ---- context helpers (避免循环引用 router 包) ----

const (
	upstreamUserIDKey       = "upstream_user_id"
	upstreamCredentialIDKey = "upstream_credential_id"
)

func getUpstreamUserID(c *gin.Context) uint {
	v, _ := c.Get(upstreamUserIDKey)
	if id, ok := v.(uint); ok {
		return id
	}
	return 0
}

func getUpstreamCredentialID(c *gin.Context) uint {
	v, _ := c.Get(upstreamCredentialIDKey)
	if id, ok := v.(uint); ok {
		return id
	}
	return 0
}

// ---- response helpers ----

func successResponse(c *gin.Context, data interface{}) {
	if data == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	c.JSON(http.StatusOK, data)
}

func errorResponse(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"ok":            false,
		"error_code":    code,
		"error_message": message,
	})
}

// ---- Ping ----

// Ping POST /api/v1/upstream/ping
func (h *Handler) Ping(c *gin.Context) {
	userID := getUpstreamUserID(c)
	if userID == 0 {
		errorResponse(c, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}

	// 站点名称
	siteName := ""
	siteConfig, err := h.SettingService.GetByKey(constants.SettingKeySiteConfig)
	if err == nil && siteConfig != nil {
		if name, ok := siteConfig["site_name"]; ok {
			if s, ok := name.(string); ok {
				siteName = s
			}
		}
	}

	// 用户钱包余额
	balanceStr := "0.00"
	account, err := h.WalletService.GetAccount(userID)
	if err == nil && account != nil {
		balanceStr = account.Balance.StringFixed(2)
	}

	// 币种
	currency, _ := h.SettingService.GetSiteCurrency("CNY")

	// 用户会员等级
	var memberLevel gin.H
	user, err := h.UserRepo.GetByID(userID)
	if err == nil && user != nil && user.MemberLevelID > 0 && h.MemberLevelService != nil {
		level, levelErr := h.MemberLevelService.GetByID(user.MemberLevelID)
		if levelErr == nil && level != nil {
			memberLevel = gin.H{
				"id":   level.ID,
				"name": level.NameJSON,
				"slug": level.Slug,
				"icon": level.Icon,
			}
		}
	}

	successResponse(c, gin.H{
		"ok":               true,
		"site_name":        siteName,
		"protocol_version": "1.0",
		"user_id":          userID,
		"balance":          balanceStr,
		"currency":         currency,
		"member_level":     memberLevel,
	})
}

// ---- ListCategories ----

// upstreamCategory 上游分类响应格式
type upstreamCategory struct {
	ID        uint        `json:"id"`
	ParentID  uint        `json:"parent_id"`
	Slug      string      `json:"slug"`
	Name      models.JSON `json:"name"`
	Icon      string      `json:"icon"`
	SortOrder int         `json:"sort_order"`
}

// ListCategories GET /api/v1/upstream/categories
func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.CategoryRepo.List()
	if err != nil {
		logger.Errorw("upstream_list_categories_failed", "error", err)
		errorResponse(c, http.StatusInternalServerError, "internal_error", "failed to list categories")
		return
	}

	items := make([]upstreamCategory, 0, len(categories))
	for _, cat := range categories {
		items = append(items, upstreamCategory{
			ID:        cat.ID,
			ParentID:  cat.ParentID,
			Slug:      cat.Slug,
			Name:      cat.NameJSON,
			Icon:      cat.Icon,
			SortOrder: cat.SortOrder,
		})
	}

	successResponse(c, gin.H{
		"ok":         true,
		"categories": items,
	})
}

// ---- ListProducts ----

// upstreamProduct 上游商品响应格式
type upstreamProduct struct {
	ID               uint                       `json:"id"`
	Slug             string                     `json:"slug"`
	SeoMeta          models.JSON                `json:"seo_meta"`
	Title            models.JSON                `json:"title"`
	Description      models.JSON                `json:"description"`
	Content          models.JSON                `json:"content"`
	Images           models.StringArray         `json:"images"`
	Tags             models.StringArray         `json:"tags"`
	PriceAmount      string                     `json:"price_amount"`
	OriginalPrice    string                     `json:"original_price,omitempty"`
	MemberPrice      string                     `json:"member_price,omitempty"`
	WholesalePrices  models.WholesalePriceTiers `json:"wholesale_prices,omitempty"`
	FulfillmentType  string                     `json:"fulfillment_type"`
	ManualFormSchema models.JSON                `json:"manual_form_schema"`
	IsActive         bool                       `json:"is_active"`
	CategoryID       uint                       `json:"category_id"`
	SKUs             []upstreamSKU              `json:"skus"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

type upstreamSKU struct {
	ID            uint        `json:"id"`
	SKUCode       string      `json:"sku_code"`
	SpecValues    models.JSON `json:"spec_values"`
	PriceAmount   string      `json:"price_amount"`
	OriginalPrice string      `json:"original_price,omitempty"`
	MemberPrice   string      `json:"member_price,omitempty"`
	StockStatus   string      `json:"stock_status"`
	StockQuantity int         `json:"stock_quantity"`
	IsActive      bool        `json:"is_active"`
}

// ListProducts GET /api/v1/upstream/products
func (h *Handler) ListProducts(c *gin.Context) {
	page, pageSize := shared.ParsePaginationWithBounds(c, "page", "page_size", 50, 50)

	// 是否包含下架商品：下游同步任务用此参数识别上游下架/删除状态
	includeInactive := c.Query("include_inactive") == "true"

	// 解析增量同步时间
	var updatedAfter *time.Time
	if updatedAfterStr := c.Query("updated_after"); updatedAfterStr != "" {
		if t, parseErr := time.Parse(time.RFC3339, updatedAfterStr); parseErr == nil {
			updatedAfter = &t
		}
	}

	products, total, err := h.ProductService.ListForUpstreamSync(updatedAfter, includeInactive, page, pageSize)
	if err != nil {
		logger.Errorw("upstream_list_products_failed", "error", err)
		errorResponse(c, http.StatusInternalServerError, "internal_error", "failed to list products")
		return
	}

	// 补充自动发货库存计数
	if err := h.ProductService.ApplyAutoStockCounts(products); err != nil {
		logger.Warnw("upstream_apply_stock_counts_failed", "error", err)
	}

	// 补充上游对接商品的 SKU 库存
	h.applyUpstreamStockToProducts(products)

	// 获取下游用户的会员等级
	userID := getUpstreamUserID(c)
	var memberLevelID uint
	if userID > 0 {
		user, err := h.UserRepo.GetByID(userID)
		if err == nil && user != nil {
			memberLevelID = user.MemberLevelID
		}
	}

	// 批量解析映射商品的真实交付类型
	fulfillmentTypeMap := h.resolveEffectiveFulfillmentTypes(products)

	items := make([]upstreamProduct, 0, len(products))
	for _, p := range products {
		items = append(items, h.toUpstreamProductWithMemberPrice(p, memberLevelID, fulfillmentTypeMap))
	}

	successResponse(c, gin.H{
		"ok":                true,
		"items":             items,
		"total":             total,
		"page":              page,
		"page_size":         pageSize,
		"includes_inactive": includeInactive, // 回声告诉下游本次响应是否包含下架商品
	})
}

// GetProduct GET /api/v1/upstream/products/:id
func (h *Handler) GetProduct(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		errorResponse(c, http.StatusBadRequest, "bad_request", "product id is required")
		return
	}

	product, err := h.ProductService.GetAdminByID(id)
	if err != nil {
		// 商品被软删除（数据库不存在）→ 保留 product_not_found 向后兼容旧版下游 adapter
		if errors.Is(err, service.ErrNotFound) {
			errorResponse(c, http.StatusNotFound, "product_not_found", "product not found")
			return
		}
		logger.Errorw("upstream_get_product_failed", "id", id, "error", err)
		errorResponse(c, http.StatusInternalServerError, "internal_error", "failed to get product")
		return
	}

	// 商品下架（IsActive=false）：仍返回 200，由下游根据 is_active 字段判断
	// 这样下游可在不下单的前提下感知下架状态、自动同步本地下架

	// 补充自动发货库存计数
	products := []models.Product{*product}
	if err := h.ProductService.ApplyAutoStockCounts(products); err != nil {
		logger.Warnw("upstream_apply_stock_counts_failed", "error", err)
	}

	// 补充上游对接商品的 SKU 库存
	h.applyUpstreamStockToProducts(products)

	// 获取下游用户的会员等级
	userID := getUpstreamUserID(c)
	var memberLevelID uint
	if userID > 0 {
		user, err := h.UserRepo.GetByID(userID)
		if err == nil && user != nil {
			memberLevelID = user.MemberLevelID
		}
	}

	// 解析映射商品的真实交付类型
	fulfillmentTypeMap := h.resolveEffectiveFulfillmentTypes(products)

	successResponse(c, gin.H{
		"ok":      true,
		"product": h.toUpstreamProductWithMemberPrice(products[0], memberLevelID, fulfillmentTypeMap),
	})
}

// ---- CreateOrder ----

type createOrderRequest struct {
	SKUID             uint        `json:"sku_id" binding:"required"`
	Quantity          int         `json:"quantity" binding:"required,min=1"`
	ManualFormData    models.JSON `json:"manual_form_data"`
	DownstreamOrderNo string      `json:"downstream_order_no"`
	TraceID           string      `json:"trace_id"`
	CallbackURL       string      `json:"callback_url"`
}

// CreateOrder POST /api/v1/upstream/orders
func (h *Handler) CreateOrder(c *gin.Context) {
	userID := getUpstreamUserID(c)
	credentialID := getUpstreamCredentialID(c)
	if userID == 0 || credentialID == 0 {
		errorResponse(c, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}

	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorResponse(c, http.StatusBadRequest, "bad_request", "invalid request body: "+err.Error())
		return
	}

	// 验证 callback URL（防止 SSRF）
	if req.CallbackURL != "" {
		if err := validateCallbackURL(req.CallbackURL); err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid_callback_url", err.Error())
			return
		}
	}

	// 幂等性检查：同一 credential 的相同 downstream_order_no 不允许重复创建
	if strings.TrimSpace(req.DownstreamOrderNo) != "" {
		existingRef, _ := h.downstreamRefRepo.GetByCredentialAndDownstreamNo(credentialID, req.DownstreamOrderNo)
		if existingRef != nil {
			// 已有相同下游订单号的记录，返回已有订单信息
			existingOrder, orderErr := h.OrderService.GetOrderByUser(existingRef.OrderID, userID)
			if orderErr == nil && existingOrder != nil {
				currency, _ := h.SettingService.GetSiteCurrency("CNY")
				successResponse(c, gin.H{
					"ok":       true,
					"order_id": existingOrder.ID,
					"order_no": existingOrder.OrderNo,
					"status":   existingOrder.Status,
					"amount":   existingOrder.TotalAmount.StringFixed(2),
					"currency": currency,
				})
				return
			}
		}
	}

	// 查找 SKU 获取所属商品 ID
	sku, err := h.ProductSKURepo.GetByID(req.SKUID)
	if err != nil || sku == nil {
		errorResponse(c, http.StatusBadRequest, "sku_unavailable", "sku not found")
		return
	}
	if !sku.IsActive {
		errorResponse(c, http.StatusBadRequest, "sku_unavailable", "sku is not active")
		return
	}

	// 验证商品是否上架
	product, err := h.ProductRepo.GetByID(fmt.Sprintf("%d", sku.ProductID))
	if err != nil || product == nil || !product.IsActive {
		errorResponse(c, http.StatusBadRequest, "product_unavailable", "product is not available")
		return
	}

	// 构建手动表单数据
	var manualFormData map[string]models.JSON
	if req.ManualFormData != nil && product.FulfillmentType == constants.FulfillmentTypeManual {
		manualFormData = map[string]models.JSON{
			fmt.Sprintf("%d", sku.ProductID): req.ManualFormData,
		}
	}

	// 创建订单（下游订单跳过风控：已通过 API 签名认证，自动钱包扣款，不存在占库存问题）
	input := service.CreateOrderInput{
		UserID: userID,
		Items: []service.CreateOrderItem{
			{
				ProductID:       sku.ProductID,
				SKUID:           req.SKUID,
				Quantity:        req.Quantity,
				FulfillmentType: product.FulfillmentType,
			},
		},
		ClientIP:        c.ClientIP(),
		ManualFormData:  manualFormData,
		SkipRiskControl: true,
	}

	order, err := h.OrderService.CreateOrder(input)
	if err != nil {
		mapOrderErrorToResponse(c, err)
		return
	}

	// 创建下游订单引用记录（用于回调通知下游）
	ref := &models.DownstreamOrderRef{
		OrderID:           order.ID,
		ApiCredentialID:   credentialID,
		DownstreamOrderNo: req.DownstreamOrderNo,
		CallbackURL:       req.CallbackURL,
		TraceID:           req.TraceID,
		CallbackStatus:    "pending",
	}
	if createErr := h.downstreamRefRepo.Create(ref); createErr != nil {
		logger.Errorw("upstream_create_downstream_ref_failed",
			"order_id", order.ID,
			"credential_id", credentialID,
			"downstream_order_no", req.DownstreamOrderNo,
			"callback_url", req.CallbackURL,
			"trace_id", req.TraceID,
			"error", createErr,
		)
	} else {
		logger.Infow("upstream_downstream_ref_created",
			"ref_id", ref.ID,
			"order_id", order.ID,
			"callback_url", req.CallbackURL,
		)
	}

	// 自动使用钱包余额支付（上游 API 订单默认钱包扣款）
	payResult, payErr := h.PaymentService.CreatePayment(service.CreatePaymentInput{
		OrderID:    order.ID,
		UseBalance: true,
		ClientIP:   c.ClientIP(),
	})
	if payErr != nil {
		logger.Errorw("upstream_auto_wallet_pay_failed",
			"order_id", order.ID,
			"error", payErr,
		)
		// 支付失败，自动取消订单避免遗留未支付订单
		if _, cancelErr := h.OrderService.CancelOrder(order.ID, userID); cancelErr != nil {
			logger.Warnw("upstream_cancel_unpaid_order_failed", "order_id", order.ID, "error", cancelErr)
		}
		// 钱包余额不足或支付失败，返回 200 + ok:false 让 A 站正确解析错误码
		c.JSON(http.StatusOK, gin.H{
			"ok":            false,
			"order_id":      order.ID,
			"order_no":      order.OrderNo,
			"status":        constants.OrderStatusCanceled,
			"error_code":    "payment_failed",
			"error_message": fmt.Sprintf("wallet payment failed: %s", payErr.Error()),
		})
		return
	}

	// 刷新订单状态
	finalStatus := order.Status
	if payResult != nil && payResult.OrderPaid {
		finalStatus = constants.OrderStatusPaid
	}

	// 币种
	currency, _ := h.SettingService.GetSiteCurrency("CNY")

	successResponse(c, gin.H{
		"ok":       true,
		"order_id": order.ID,
		"order_no": order.OrderNo,
		"status":   finalStatus,
		"amount":   order.TotalAmount.StringFixed(2),
		"currency": currency,
	})
}

// ---- GetOrder ----

// GetOrder GET /api/v1/upstream/orders/:id
func (h *Handler) GetOrder(c *gin.Context) {
	userID := getUpstreamUserID(c)
	if userID == 0 {
		errorResponse(c, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}

	orderID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "bad_request", "invalid order id")
		return
	}

	order, err := h.OrderService.GetOrderByUser(orderID, userID)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			errorResponse(c, http.StatusNotFound, "order_not_found", "order not found")
			return
		}
		logger.Errorw("upstream_get_order_failed", "order_id", orderID, "error", err)
		errorResponse(c, http.StatusInternalServerError, "internal_error", "failed to get order")
		return
	}

	status := strings.ToLower(strings.TrimSpace(order.Status))
	localRefundRecords := make([]models.JSON, 0)

	// 优先使用采购单视角的上游退款信息，避免订单状态与上游退款状态不一致。
	if h.ProcurementOrderService != nil {
		procOrder, procErr := h.ProcurementOrderService.GetByLocalOrderNo(order.OrderNo)
		if procErr == nil && procOrder != nil {
			switch strings.ToLower(strings.TrimSpace(procOrder.Status)) {
			case constants.ProcurementStatusPartiallyRefunded, constants.ProcurementStatusRefunded:
				status = strings.ToLower(strings.TrimSpace(procOrder.Status))
			}
		}
	}

	// refund_records 固定返回本地退款记录（与更上游无关），由服务层统一处理。
	if records, recordsErr := h.OrderService.BuildLocalRefundRecordsForOrder(order); recordsErr != nil {
		logger.Warnw("upstream_get_order_refund_records_failed", "order_id", order.ID, "error", recordsErr)
	} else {
		localRefundRecords = records
	}

	resp := gin.H{
		"ok":              true,
		"order_id":        order.ID,
		"order_no":        order.OrderNo,
		"status":          status,
		"amount":          order.TotalAmount.StringFixed(2),
		"refunded_amount": order.RefundedAmount.StringFixed(2),
		"currency":        order.Currency,
		"refund_records":  localRefundRecords,
	}

	// 若已交付，返回交付信息（优先使用订单自身的 fulfillment，否则从子订单获取）
	sourceFulfillment := order.Fulfillment
	if sourceFulfillment == nil && len(order.Children) > 0 {
		for i := range order.Children {
			if order.Children[i].Fulfillment != nil {
				sourceFulfillment = order.Children[i].Fulfillment
				break
			}
		}
	}
	if sourceFulfillment != nil && sourceFulfillment.Status == constants.FulfillmentStatusDelivered {
		resp["fulfillment"] = gin.H{
			"type":          sourceFulfillment.Type,
			"status":        sourceFulfillment.Status,
			"payload":       sourceFulfillment.Payload,
			"delivery_data": sourceFulfillment.LogisticsJSON,
			"delivered_at":  sourceFulfillment.DeliveredAt,
		}
	}

	// 订单项信息
	if len(order.Items) > 0 {
		items := make([]gin.H, 0, len(order.Items))
		for _, item := range order.Items {
			items = append(items, gin.H{
				"product_id":           item.ProductID,
				"sku_id":               item.SKUID,
				"title":                item.TitleJSON,
				"quantity":             item.Quantity,
				"original_unit_price":  item.OriginalUnitPrice.StringFixed(2),
				"unit_price":           item.UnitPrice.StringFixed(2),
				"original_total_price": item.OriginalTotalPrice.StringFixed(2),
				"total_price":          item.TotalPrice.StringFixed(2),
				"fulfillment_type":     item.FulfillmentType,
			})
		}
		resp["items"] = items
	}

	successResponse(c, resp)
}

// ---- CancelOrder ----

// CancelOrder POST /api/v1/upstream/orders/:id/cancel
func (h *Handler) CancelOrder(c *gin.Context) {
	userID := getUpstreamUserID(c)
	if userID == 0 {
		errorResponse(c, http.StatusUnauthorized, "unauthorized", "invalid credentials")
		return
	}

	orderID, err := shared.ParseParamUint(c, "id")
	if err != nil {
		errorResponse(c, http.StatusBadRequest, "bad_request", "invalid order id")
		return
	}

	order, err := h.OrderService.CancelOrder(orderID, userID)
	if err != nil {
		if errors.Is(err, service.ErrOrderNotFound) {
			errorResponse(c, http.StatusNotFound, "order_not_found", "order not found")
			return
		}
		if errors.Is(err, service.ErrOrderCancelNotAllowed) {
			errorResponse(c, http.StatusConflict, "cancel_not_allowed", "order cannot be canceled in current status")
			return
		}
		logger.Errorw("upstream_cancel_order_failed", "order_id", orderID, "error", err)
		errorResponse(c, http.StatusInternalServerError, "internal_error", "failed to cancel order")
		return
	}

	successResponse(c, gin.H{
		"ok":       true,
		"order_id": order.ID,
		"order_no": order.OrderNo,
		"status":   order.Status,
	})
}

// ---- HandleCallback (A 站接收 B 站回调) ----

type callbackPayload struct {
	Event             string `json:"event"`
	OrderID           uint   `json:"order_id"`
	OrderNo           string `json:"order_no"`
	DownstreamOrderNo string `json:"downstream_order_no"`
	Status            string `json:"status"`
	Fulfillment       *struct {
		Type         string      `json:"type"`
		Status       string      `json:"status"`
		Payload      string      `json:"payload"`
		DeliveryData models.JSON `json:"delivery_data"`
		DeliveredAt  *time.Time  `json:"delivered_at"`
	} `json:"fulfillment,omitempty"`
	Timestamp int64 `json:"timestamp"`
}

// HandleCallback POST /api/v1/upstream/callback (A 站点接收 B 站回调)
func (h *Handler) HandleCallback(c *gin.Context) {
	// ---- 签名验证 ----
	apiKey := c.GetHeader(upstreamadapter.HeaderApiKey)
	timestampStr := c.GetHeader(upstreamadapter.HeaderTimestamp)
	signature := c.GetHeader(upstreamadapter.HeaderSignature)

	if apiKey == "" || timestampStr == "" || signature == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "missing authentication headers"})
		return
	}

	timestamp, err := upstreamadapter.ParseTimestamp(timestampStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "invalid timestamp"})
		return
	}

	if !upstreamadapter.IsTimestampValid(timestamp) {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "timestamp expired"})
		return
	}

	// 根据 api_key 查找对应的站点连接
	conn, err := h.SiteConnectionRepo.GetByApiKey(apiKey)
	if err != nil {
		logger.Errorw("upstream_callback_lookup_connection_failed", "api_key", apiKey, "error", err)
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "internal error"})
		return
	}
	if conn == nil || conn.Status != "active" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "invalid api key"})
		return
	}

	// 读取 body 用于签名验证
	var body []byte
	if c.Request.Body != nil {
		body, err = io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "message": "failed to read request body"})
			return
		}
		c.Request.Body = io.NopCloser(&bodyBuf{data: body})
	}

	// 解密 api_secret 并验证签名
	apiSecret := conn.ApiSecret
	if h.SiteConnectionService != nil {
		if decrypted, decErr := h.SiteConnectionService.DecryptSecret(apiSecret); decErr == nil {
			apiSecret = decrypted
		}
	}

	if !upstreamadapter.Verify(apiSecret, "POST", "/api/v1/upstream/callback", signature, timestamp, body) {
		logger.Warnw("upstream_callback_signature_invalid", "api_key", apiKey)
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "signature verification failed"})
		return
	}

	// ---- 解析 payload ----
	var payload callbackPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "invalid request body"})
		return
	}

	if payload.DownstreamOrderNo == "" || payload.Status == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "missing required fields"})
		return
	}

	if h.ProcurementOrderService == nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "service not available"})
		return
	}

	// 根据 downstream_order_no（即本站的 local_order_no）查找对应的采购单
	procOrder, err := h.ProcurementOrderService.GetByLocalOrderNo(payload.DownstreamOrderNo)
	if err != nil || procOrder == nil {
		logger.Warnw("upstream_callback_procurement_not_found",
			"downstream_order_no", payload.DownstreamOrderNo,
			"upstream_order_id", payload.OrderID,
		)
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "procurement order not found"})
		return
	}

	// 转换状态并处理回调
	var uf *upstreamadapter.UpstreamFulfillment
	if payload.Fulfillment != nil {
		uf = &upstreamadapter.UpstreamFulfillment{
			Type:         payload.Fulfillment.Type,
			Status:       payload.Fulfillment.Status,
			Payload:      payload.Fulfillment.Payload,
			DeliveryData: payload.Fulfillment.DeliveryData,
			DeliveredAt:  payload.Fulfillment.DeliveredAt,
		}
	}

	upstreamStatus := mapCallbackStatus(payload.Status)
	if err := h.ProcurementOrderService.HandleUpstreamCallback(procOrder.ID, upstreamStatus, uf); err != nil {
		logger.Warnw("upstream_callback_handle_failed",
			"procurement_order_id", procOrder.ID,
			"upstream_status", upstreamStatus,
			"error", err,
		)
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "callback processing failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "received"})
}

// bodyBuf 用于重置 body
type bodyBuf struct {
	data   []byte
	offset int
}

func (b *bodyBuf) Read(p []byte) (n int, err error) {
	if b.offset >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.offset:])
	b.offset += n
	return n, nil
}

// mapCallbackStatus 规范化上游订单状态；成功状态保持原值，避免丢失 delivered/completed/fulfilled 的语义。
func mapCallbackStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "canceled", "cancelled":
		return "canceled"
	default:
		return normalized
	}
}

// ---- helpers ----

// applyUpstreamStockToProducts 为 upstream 类型商品的 SKU 填充上游库存数据
// 从 SKU 映射中读取 UpstreamStock，写入 ProductSKU 的虚拟字段，供 computeSKUStock 使用
func (h *Handler) applyUpstreamStockToProducts(products []models.Product) {
	for i := range products {
		p := &products[i]
		if p.FulfillmentType != constants.FulfillmentTypeUpstream {
			continue
		}
		for j := range p.SKUs {
			sku := &p.SKUs[j]
			mapping, err := h.SKUMappingRepo.GetByLocalSKUID(sku.ID)
			if err != nil || mapping == nil {
				sku.UpstreamStock = 0
				continue
			}
			sku.UpstreamStock = mapping.UpstreamStock
		}
	}
}

// resolveEffectiveFulfillmentTypes 批量解析映射商品的真实交付类型
// 对于映射商品（FulfillmentType="upstream"），返回 ProductMapping 中保存的原始交付类型
func (h *Handler) resolveEffectiveFulfillmentTypes(products []models.Product) map[uint]string {
	result := make(map[uint]string)
	var mappedIDs []uint
	for _, p := range products {
		if p.IsMapped && p.FulfillmentType == constants.FulfillmentTypeUpstream {
			mappedIDs = append(mappedIDs, p.ID)
		}
	}
	if len(mappedIDs) == 0 {
		return result
	}
	mappings, err := h.ProductMappingRepo.ListByLocalProductIDs(mappedIDs)
	if err != nil {
		logger.Warnw("resolve_effective_fulfillment_types_failed", "error", err)
		return result
	}
	for _, m := range mappings {
		ft := m.UpstreamFulfillmentType
		if ft != constants.FulfillmentTypeAuto {
			ft = constants.FulfillmentTypeManual
		}
		result[m.LocalProductID] = ft
	}
	return result
}

func (h *Handler) toUpstreamProductWithMemberPrice(p models.Product, memberLevelID uint, fulfillmentTypeMap map[uint]string) upstreamProduct {
	skus := make([]upstreamSKU, 0, len(p.SKUs))
	for _, s := range p.SKUs {
		if !s.IsActive {
			continue
		}
		stockStatus, stockQuantity := computeSKUStock(p, s)
		si := upstreamSKU{
			ID:            s.ID,
			SKUCode:       s.SKUCode,
			SpecValues:    s.SpecValuesJSON,
			PriceAmount:   s.PriceAmount.StringFixed(2),
			StockStatus:   stockStatus,
			StockQuantity: stockQuantity,
			IsActive:      s.IsActive,
		}
		if memberLevelID > 0 && h.MemberLevelService != nil {
			mp, _ := h.MemberLevelService.ResolveMemberPrice(memberLevelID, p.ID, s.ID, s.PriceAmount.Decimal)
			if mp.LessThan(s.PriceAmount.Decimal) {
				si.OriginalPrice = si.PriceAmount
				si.MemberPrice = models.NewMoneyFromDecimal(mp).StringFixed(2)
				si.PriceAmount = si.MemberPrice // price_amount 是实际售价（会员价）
			}
		}
		skus = append(skus, si)
	}

	// 映射商品返回原始交付类型，非映射商品返回自身交付类型
	effectiveFulfillmentType := p.FulfillmentType
	if ft, ok := fulfillmentTypeMap[p.ID]; ok {
		effectiveFulfillmentType = ft
	}

	result := upstreamProduct{
		ID:               p.ID,
		Slug:             p.Slug,
		SeoMeta:          p.SeoMetaJSON,
		Title:            p.TitleJSON,
		Description:      p.DescriptionJSON,
		Content:          p.ContentJSON,
		Images:           p.Images,
		Tags:             p.Tags,
		PriceAmount:      p.PriceAmount.StringFixed(2),
		WholesalePrices:  p.WholesalePrices,
		FulfillmentType:  effectiveFulfillmentType,
		ManualFormSchema: p.ManualFormSchemaJSON,
		IsActive:         p.IsActive,
		CategoryID:       p.CategoryID,
		SKUs:             skus,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
	}

	if memberLevelID > 0 && h.MemberLevelService != nil {
		mp, _ := h.MemberLevelService.ResolveMemberPrice(memberLevelID, p.ID, 0, p.PriceAmount.Decimal)
		if mp.LessThan(p.PriceAmount.Decimal) {
			result.OriginalPrice = result.PriceAmount
			result.MemberPrice = models.NewMoneyFromDecimal(mp).StringFixed(2)
			result.PriceAmount = result.MemberPrice
		}
	}

	return result
}

// computeSKUStock 计算 SKU 的库存状态和实际可用量
func computeSKUStock(p models.Product, s models.ProductSKU) (status string, quantity int) {
	if p.FulfillmentType == constants.FulfillmentTypeManual {
		// 手动交付：根据 SKU 级别手动库存判断
		skuTotal := s.ManualStockTotal
		if skuTotal == constants.ManualStockUnlimited {
			return constants.ProductStockStatusUnlimited, -1
		}
		available := skuTotal - s.ManualStockLocked
		if available <= 0 {
			return constants.ProductStockStatusOutOfStock, 0
		}
		if available <= 20 {
			return constants.ProductStockStatusLowStock, available
		}
		return constants.ProductStockStatusInStock, available
	}

	if p.FulfillmentType == constants.FulfillmentTypeUpstream {
		// 上游对接商品：使用 SKU 映射中的上游库存（通过虚拟字段 UpstreamStock 传入）
		available := s.UpstreamStock
		if available < 0 {
			return constants.ProductStockStatusUnlimited, -1
		}
		if available == 0 {
			return constants.ProductStockStatusOutOfStock, 0
		}
		if available <= 20 {
			return constants.ProductStockStatusLowStock, available
		}
		return constants.ProductStockStatusInStock, available
	}

	// 自动发货：根据卡密库存判断
	available := int(s.AutoStockAvailable)
	if available <= 0 {
		return constants.ProductStockStatusOutOfStock, 0
	}
	if available <= 20 {
		return constants.ProductStockStatusLowStock, available
	}
	return constants.ProductStockStatusInStock, available
}

// mapOrderErrorToResponse 将订单创建错误映射为上游 API 错误响应
// validateCallbackURL 验证回调 URL 的安全性（防止 SSRF）
func validateCallbackURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url format")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("callback url must use http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("callback url must have a host")
	}
	// 禁止 localhost 和回环地址
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return fmt.Errorf("callback url must not point to localhost")
	}
	// 检查是否是内网 IP
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("callback url must not point to private network")
		}
	}
	return nil
}

func mapOrderErrorToResponse(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrWalletInsufficientBalance):
		errorResponse(c, http.StatusPaymentRequired, "insufficient_balance", "wallet balance is insufficient")
	case errors.Is(err, service.ErrCardSecretInsufficient),
		errors.Is(err, service.ErrManualStockInsufficient):
		errorResponse(c, http.StatusConflict, "insufficient_stock", "product stock is insufficient")
	case errors.Is(err, service.ErrProductNotAvailable),
		errors.Is(err, service.ErrProductNotFound):
		errorResponse(c, http.StatusBadRequest, "product_unavailable", "product is not available")
	case errors.Is(err, service.ErrProductSKUInvalid),
		errors.Is(err, service.ErrProductSKURequired):
		errorResponse(c, http.StatusBadRequest, "sku_unavailable", "sku is invalid or not available")
	case errors.Is(err, service.ErrInvalidOrderItem):
		errorResponse(c, http.StatusBadRequest, "bad_request", "invalid order parameters")
	case errors.Is(err, service.ErrManualFormRequiredMissing),
		errors.Is(err, service.ErrManualFormFieldInvalid),
		errors.Is(err, service.ErrManualFormTypeInvalid),
		errors.Is(err, service.ErrManualFormOptionInvalid):
		errorResponse(c, http.StatusBadRequest, "bad_request", "manual form data is invalid: "+err.Error())
	default:
		logger.Errorw("upstream_create_order_failed", "error", err)
		errorResponse(c, http.StatusInternalServerError, "internal_error", "failed to create order")
	}
}
