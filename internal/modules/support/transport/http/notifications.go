package supporthttp

import (
	"github.com/dujiao-next/internal/modules/support/application"
	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct{ service *application.Notifications }

func NewNotificationHandler(s *application.Notifications) *NotificationHandler {
	return &NotificationHandler{s}
}
func (h *NotificationHandler) Settings(c *gin.Context) {
	v, err := h.service.Settings(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, v)
}
func (h *NotificationHandler) SaveSettings(c *gin.Context) {
	var in application.NotificationSettings
	if c.ShouldBindJSON(&in) != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	if err := h.service.SaveSettings(c.Request.Context(), in); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *NotificationHandler) Binding(c *gin.Context) {
	id, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	v, err := h.service.Binding(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{"uid": v.UID, "enabled": v.Enabled, "pending": v.PendingHash != "", "mode": v.Mode, "spt_configured": v.SPTCipher != ""})
}
func (h *NotificationHandler) Begin(c *gin.Context) {
	id, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	var in struct {
		UID  string `json:"uid"`
		Mode string `json:"mode"`
		SPT  string `json:"spt"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	var err error
	if in.Mode == "simple" {
		err = h.service.BeginSimpleBinding(c.Request.Context(), id, in.SPT)
	} else if in.Mode == "" || in.Mode == "standard" {
		err = h.service.BeginBinding(c.Request.Context(), id, in.UID)
	} else {
		err = contract.ErrInvalid
	}
	if err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *NotificationHandler) Confirm(c *gin.Context) {
	id, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	var in struct {
		Code string `json:"code"`
	}
	if c.ShouldBindJSON(&in) != nil {
		fail(c, contract.ErrInvalid)
		return
	}
	if err := h.service.ConfirmBinding(c.Request.Context(), id, in.Code); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *NotificationHandler) Unbind(c *gin.Context) {
	id, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	if err := h.service.Unbind(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *NotificationHandler) Test(c *gin.Context) {
	id, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	if err := h.service.Test(c.Request.Context(), id); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func RegisterNotificationRoutes(admin gin.IRoutes, h *NotificationHandler) {
	admin.POST("/support/notification/scans", h.CreateScan)
	admin.GET("/support/notification/scans/:scanID", h.PollScan)
	admin.POST("/support/notification/scans/:scanID/confirm", h.ConfirmScan)
	admin.DELETE("/support/notification/scans/:scanID", h.CancelScan)
	admin.GET("/support/notification/settings", h.Settings)
	admin.PUT("/support/notification/settings", h.SaveSettings)
	admin.GET("/support/notification/binding", h.Binding)
	admin.POST("/support/notification/binding", h.Begin)
	admin.POST("/support/notification/binding/confirm", h.Confirm)
	admin.DELETE("/support/notification/binding", h.Unbind)
	admin.POST("/support/notification/test", h.Test)
}
