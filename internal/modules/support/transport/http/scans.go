package supporthttp

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

func (h *NotificationHandler) CreateScan(c *gin.Context) {
	id, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	v, err := h.service.CreateScan(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, contract.ErrNotificationNotConfigured) {
			message := "请先由主管保存 WxPusher 应用 App Token，再生成订阅二维码。"
			if strings.HasPrefix(c.GetHeader("X-Lang"), "en") {
				message = "Ask a supervisor to save the WxPusher app token before generating a subscription QR code."
			}
			if c.GetHeader("X-Lang") == "zh-TW" {
				message = "請先由主管儲存 WxPusher 應用 App Token，再產生訂閱二維碼。"
			}
			response.Error(c, response.CodeBadRequest, message)
			return
		}
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, v)
}
func (h *NotificationHandler) PollScan(c *gin.Context) {
	id, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	v, err := h.service.PollScan(c.Request.Context(), id, c.Param("scanID"))
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, v)
}
func (h *NotificationHandler) ConfirmScan(c *gin.Context) {
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
	if err := h.service.ConfirmScan(c.Request.Context(), id, c.Param("scanID"), in.Code); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
func (h *NotificationHandler) CancelScan(c *gin.Context) {
	id, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	if err := h.service.CancelScan(c.Request.Context(), id, c.Param("scanID")); err != nil {
		fail(c, err)
		return
	}
	response.Success(c, gin.H{})
}
