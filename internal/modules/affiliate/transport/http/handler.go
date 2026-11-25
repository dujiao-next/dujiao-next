package affiliatehttp

import (
	"errors"
	"strings"

	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	affiliateapp "github.com/dujiao-next/internal/modules/affiliate/application"
	affiliatepresenter "github.com/dujiao-next/internal/modules/affiliate/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// Service 是前台推广返利端口。
type Service interface {
	TrackClick(input affiliateapp.TrackClickInput) error
	OpenAffiliate(userID uint) (*affiliatedomain.Profile, error)
	GetUserDashboard(userID uint) (affiliateapp.Dashboard, error)
	GetProductRates(userID, productID uint) (affiliateapp.ProductRates, error)
	ListUserCommissions(userID uint, page, pageSize int, status string) ([]affiliatedomain.Commission, int64, error)
	ListUserWithdraws(userID uint, page, pageSize int, status string) ([]affiliatedomain.WithdrawRequest, int64, error)
	ApplyWithdraw(userID uint, input affiliateapp.WithdrawApplyInput) (*affiliatedomain.WithdrawRequest, error)
}

type trackClickRequest struct {
	AffiliateCode string `json:"affiliate_code" binding:"required"`
	VisitorKey    string `json:"visitor_key"`
	LandingPath   string `json:"landing_path"`
	Referrer      string `json:"referrer"`
}

type withdrawApplyRequest struct {
	Amount  string `json:"amount" binding:"required"`
	Channel string `json:"channel" binding:"required"`
	Account string `json:"account" binding:"required"`
}

// Handler 处理前台推广返利请求。
type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	if svc == nil {
		panic("affiliate handler: service is nil")
	}
	return &Handler{svc: svc}
}

// TrackAffiliateClick 记录推广点击
func (h *Handler) TrackAffiliateClick(c *gin.Context) {
	var req trackClickRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.svc.TrackClick(affiliateapp.TrackClickInput{
		AffiliateCode: req.AffiliateCode,
		VisitorKey:    req.VisitorKey,
		LandingPath:   req.LandingPath,
		Referrer:      req.Referrer,
		ClientIP:      c.ClientIP(),
		UserAgent:     c.GetHeader("User-Agent"),
	}); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

// OpenAffiliate 开通推广返利
func (h *Handler) OpenAffiliate(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	profile, err := h.svc.OpenAffiliate(uid)
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrDisabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.forbidden", nil)
		case errors.Is(err, affiliateapp.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, affiliatepresenter.NewProfile(profile))
}

// GetAffiliateDashboard 获取推广返利看板
func (h *Handler) GetAffiliateDashboard(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	data, err := h.svc.GetUserDashboard(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, data)
}

// GetProductRates 查询当前推广员对某个商品及其规格生效的返利比例。
func (h *Handler) GetProductRates(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	productID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	data, err := h.svc.GetProductRates(uid, productID)
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrDisabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.forbidden", nil)
		case errors.Is(err, affiliateapp.ErrNotOpened):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.product_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		}
		return
	}
	response.Success(c, data)
}

// ListAffiliateCommissions 查询我的推广佣金记录
func (h *Handler) ListAffiliateCommissions(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))

	rows, total, err := h.svc.ListUserCommissions(uid, page, pageSize, status)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, affiliatepresenter.NewCommissionList(rows), response.BuildPagination(page, pageSize, total))
}

// ListAffiliateWithdraws 查询我的提现申请记录
func (h *Handler) ListAffiliateWithdraws(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	status := strings.TrimSpace(c.Query("status"))

	rows, total, err := h.svc.ListUserWithdraws(uid, page, pageSize, status)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, affiliatepresenter.NewWithdrawList(rows), response.BuildPagination(page, pageSize, total))
}

// ApplyAffiliateWithdraw 提交提现申请
func (h *Handler) ApplyAffiliateWithdraw(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req withdrawApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	row, err := h.svc.ApplyWithdraw(uid, affiliateapp.WithdrawApplyInput{
		Amount:  amount,
		Channel: req.Channel,
		Account: req.Account,
	})
	if err != nil {
		switch {
		case errors.Is(err, affiliateapp.ErrDisabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.forbidden", nil)
		case errors.Is(err, affiliateapp.ErrNotOpened):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrWithdrawAmountInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrWithdrawChannelInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		case errors.Is(err, affiliateapp.ErrWithdrawInsufficient):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}
	response.Success(c, affiliatepresenter.NewWithdraw(row))
}
