package supporthttp

import (
	"encoding/json"
	"errors"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/modules/support/application"
	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/modules/support/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strconv"
)

type Supervisor func(uint) bool
type Handler struct {
	service    *application.Service
	mode       string
	supervisor Supervisor
}

func New(service *application.Service, mode string, supervisor Supervisor) *Handler {
	return &Handler{service, mode, supervisor}
}
func (h *Handler) actor(c *gin.Context) (contract.Actor, bool) {
	a := contract.Actor{}
	if h.mode == "admin" {
		id, ok := ginutil.GetAdminID(c)
		if !ok || id == 0 {
			return a, false
		}
		a.AdminID = id
		a.Supervisor = ginutil.IsSuperAdmin(c) || (h.supervisor != nil && h.supervisor(id))
		return a, true
	}
	tenant, ok := resellercontract.TenantFromContext(c.Request.Context())
	if !ok || tenant.Unavailable {
		ginutil.RespondError(c, response.CodeForbidden, "error.forbidden", nil)
		return a, false
	}
	if tenant.ResellerID != nil {
		a.TenantID = *tenant.ResellerID
	}
	if h.mode == "user" {
		id, ok := ginutil.GetUserID(c)
		if !ok || id == 0 {
			return a, false
		}
		a.UserID = id
	} else {
		a.GuestToken = c.GetHeader("X-Support-Token")
	}
	return a, true
}
func fail(c *gin.Context, err error) {
	code := response.CodeInternal
	key := "error.internal_error"
	switch {
	case errors.Is(err, contract.ErrNotFound):
		code = response.CodeNotFound
		key = "error.not_found"
	case errors.Is(err, contract.ErrForbidden):
		code = response.CodeForbidden
		key = "error.forbidden"
	case errors.Is(err, contract.ErrInvalid), errors.Is(err, contract.ErrConflict):
		code = response.CodeBadRequest
		key = "error.bad_request"
	}
	ginutil.RespondError(c, code, key, nil)
}
func idParam(c *gin.Context) (uint, bool) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil || id == 0 {
		fail(c, contract.ErrInvalid)
		return 0, false
	}
	return id, true
}

type conversationView struct {
	domain.Conversation
	UnreadCount int64                  `json:"unread_count"`
	Summary     *contract.OrderSummary `json:"order_summary,omitempty"`
}

func (h *Handler) view(c *gin.Context, v domain.Conversation, a contract.Actor) conversationView {
	n, _ := h.service.Store.Unread(c.Request.Context(), &v, a.AdminID > 0)
	out := conversationView{Conversation: v, UnreadCount: n}
	if v.OrderSummary != "" {
		var summary contract.OrderSummary
		if json.Unmarshal([]byte(v.OrderSummary), &summary) == nil {
			out.Summary = &summary
		}
	}
	return out
}
func (h *Handler) List(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	page, limit := ginutil.ParsePagination(c)
	if limit > 100 {
		limit = 100
	}
	items, total, err := h.service.List(c.Request.Context(), a, c.Query("status"), page, limit)
	if err != nil {
		fail(c, err)
		return
	}
	out := []conversationView{}
	for _, v := range items {
		out = append(out, h.view(c, v, a))
	}
	response.SuccessWithPage(c, out, response.BuildPagination(page, limit, total))
}
func (h *Handler) Create(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024)
	var in application.CreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	email, password, _ := ginutil.GetGuestCredentials(c)
	v, token, err := h.service.Create(c.Request.Context(), a, in, email, password)
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{"conversation": h.view(c, *v, a), "token": token})
}
func (h *Handler) Get(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	v, err := h.service.Get(c.Request.Context(), a, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, h.view(c, *v, a))
}

type messageView struct {
	domain.Message
	Attachments []domain.Attachment `json:"attachments"`
}

func (h *Handler) Messages(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	after, _ := strconv.ParseUint(c.Query("after_id"), 10, 32)
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := h.service.Messages(c.Request.Context(), a, id, uint(after), limit)
	if err != nil {
		fail(c, err)
		return
	}
	ids := []uint{}
	for _, m := range items {
		ids = append(ids, m.ID)
	}
	attachments, err := h.service.Store.Attachments(c.Request.Context(), id, ids)
	if err != nil {
		fail(c, err)
		return
	}
	out := []messageView{}
	for _, m := range items {
		v := messageView{Message: m, Attachments: []domain.Attachment{}}
		for _, f := range attachments {
			if f.MessageID == m.ID {
				v.Attachments = append(v.Attachments, f)
			}
		}
		out = append(out, v)
	}
	response.Success(c, out)
}
func (h *Handler) Send(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32*1024)
	var in application.MessageInput
	if c.ShouldBindJSON(&in) != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	v, err := h.service.Append(c.Request.Context(), a, id, in)
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, v)
}
func (h *Handler) Read(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in struct {
		MessageID uint `json:"message_id"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	if err := h.service.Read(c.Request.Context(), a, id, in.MessageID); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *Handler) Link(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in struct {
		OrderNo string `json:"order_no"`
	}
	if c.ShouldBindJSON(&in) != nil || len(in.OrderNo) > 128 {
		fail(c, contract.ErrInvalid)
		return
	}
	email, password, _ := ginutil.GetGuestCredentials(c)
	if err := h.service.LinkOrder(c.Request.Context(), a, id, in.OrderNo, email, password); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *Handler) Claim(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	if err := h.service.Claim(c.Request.Context(), a, id); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *Handler) Assign(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in struct {
		AdminID uint `json:"admin_id"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	if err := h.service.Assign(c.Request.Context(), a, id, in.AdminID); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *Handler) Status(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	var in struct {
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	if err := h.service.Status(c.Request.Context(), a, id, in.Status); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *Handler) Agents(c *gin.Context) {
	v, err := h.service.Agents(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, v)
}
func (h *Handler) Upload(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 5*1024*1024+64*1024)
	if err := c.Request.ParseMultipartForm(5 * 1024 * 1024); err != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
	if err != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	v, err := h.service.Upload(c.Request.Context(), a, id, http.DetectContentType(data), data)
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, v)
}
func (h *Handler) Download(c *gin.Context) {
	a, ok := h.actor(c)
	if !ok {
		return
	}
	id, ok := idParam(c)
	if !ok {
		return
	}
	aid, err := ginutil.ParseParamUint(c, "attachmentID")
	if err != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	v, err := h.service.Attachment(c.Request.Context(), a, id, aid)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, v.MIME, v.Data)
}
