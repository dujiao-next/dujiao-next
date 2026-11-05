package invitecodehttp

import (
	"errors"
	"strings"
	"time"

	invitecodeapp "github.com/dujiao-next/internal/modules/identity/invitecode/application"
	invitecodedomain "github.com/dujiao-next/internal/modules/identity/invitecode/domain"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminService 是后台邀请码管理端口。
type AdminService interface {
	Generate(input invitecodeapp.GenerateInput) ([]invitecodedomain.InviteCode, error)
	List(code, status string, page, pageSize int) ([]invitecodedomain.InviteCode, int64, error)
	Update(id uint, input invitecodeapp.UpdateInput) (*invitecodedomain.InviteCode, error)
	Delete(id uint) error
}

// UserLister 查询凭某个邀请码注册的用户。
type UserLister interface {
	List(filter usercontract.ListFilter) ([]userdomain.User, int64, error)
}

// AdminHandler 处理后台邀请码请求。
type AdminHandler struct {
	codes AdminService
	users UserLister
}

func NewAdminHandler(codes AdminService, users UserLister) *AdminHandler {
	if codes == nil || users == nil {
		panic("invite code admin handler: required dependency is nil")
	}
	return &AdminHandler{codes: codes, users: users}
}

type generateRequest struct {
	Quantity  int    `json:"quantity"`
	Code      string `json:"code"`
	MaxUses   int    `json:"max_uses"`
	ExpiresAt string `json:"expires_at"`
	Note      string `json:"note"`
}

type updateRequest struct {
	MaxUses   *int    `json:"max_uses"`
	ExpiresAt *string `json:"expires_at"` // 传空字符串表示清除过期时间
	Status    *string `json:"status"`
	Note      *string `json:"note"`
}

type adminInviteCodeItem struct {
	invitecodedomain.InviteCode
	Usable bool `json:"usable"`
}

type adminInviteCodeUser struct {
	ID          uint      `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

func respondServiceError(c *gin.Context, err error, fallbackKey string) {
	switch {
	case errors.Is(err, invitecodeapp.ErrInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.invite_code_params_invalid", nil)
	case errors.Is(err, invitecodeapp.ErrExists):
		ginutil.RespondError(c, response.CodeBadRequest, "error.invite_code_exists", nil)
	case errors.Is(err, invitecodeapp.ErrNotFound):
		ginutil.RespondError(c, response.CodeNotFound, "error.invite_code_not_found", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, fallbackKey, err)
	}
}

// Generate POST /admin/invite-codes/generate
func (h *AdminHandler) Generate(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	var req generateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	expiresAt, err := ginutil.ParseTimeNullable(strings.TrimSpace(req.ExpiresAt))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	codes, err := h.codes.Generate(invitecodeapp.GenerateInput{
		Quantity:  req.Quantity,
		Code:      req.Code,
		MaxUses:   req.MaxUses,
		ExpiresAt: expiresAt,
		Note:      req.Note,
		CreatedBy: &adminID,
	})
	if err != nil {
		respondServiceError(c, err, "error.invite_code_create_failed")
		return
	}
	response.Success(c, gin.H{"codes": codes, "created": len(codes)})
}

// List GET /admin/invite-codes
func (h *AdminHandler) List(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	codes, total, err := h.codes.List(c.Query("code"), c.Query("status"), page, pageSize)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.invite_code_fetch_failed", err)
		return
	}
	now := time.Now()
	items := make([]adminInviteCodeItem, 0, len(codes))
	for i := range codes {
		items = append(items, adminInviteCodeItem{InviteCode: codes[i], Usable: codes[i].Usable(now)})
	}
	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}

// Update PUT /admin/invite-codes/:id
func (h *AdminHandler) Update(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	input := invitecodeapp.UpdateInput{MaxUses: req.MaxUses, Status: req.Status, Note: req.Note}
	if req.ExpiresAt != nil {
		raw := strings.TrimSpace(*req.ExpiresAt)
		if raw == "" {
			input.ClearExpires = true
		} else if input.ExpiresAt, err = ginutil.ParseTimeNullable(raw); err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
	}
	code, err := h.codes.Update(id, input)
	if err != nil {
		respondServiceError(c, err, "error.invite_code_update_failed")
		return
	}
	response.Success(c, adminInviteCodeItem{InviteCode: *code, Usable: code.Usable(time.Now())})
}

// Delete DELETE /admin/invite-codes/:id
func (h *AdminHandler) Delete(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.codes.Delete(id); err != nil {
		respondServiceError(c, err, "error.invite_code_delete_failed")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// Users GET /admin/invite-codes/:id/users
func (h *AdminHandler) Users(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	page, pageSize := ginutil.ParsePagination(c)
	users, total, err := h.users.List(usercontract.ListFilter{InviteCodeID: id, Page: page, PageSize: pageSize})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.invite_code_fetch_failed", err)
		return
	}
	items := make([]adminInviteCodeUser, 0, len(users))
	for _, user := range users {
		items = append(items, adminInviteCodeUser{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			Status:      user.Status,
			CreatedAt:   user.CreatedAt,
		})
	}
	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}
