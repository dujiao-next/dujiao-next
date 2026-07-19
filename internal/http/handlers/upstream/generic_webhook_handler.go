package upstream

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/service"
	upstreamadapter "github.com/dujiao-next/internal/upstream"

	"github.com/gin-gonic/gin"
)

const genericWebhookCallbackBodyLimit = 1 << 20

type genericWebhookCallbackPayload struct {
	Event       string `json:"event"`
	OrderNo     string `json:"order_no"`
	Status      string `json:"status"`
	Fulfillment *struct {
		Type         string      `json:"type"`
		Payload      string      `json:"payload"`
		DeliveryData models.JSON `json:"delivery_data"`
		DeliveredAt  *time.Time  `json:"delivered_at"`
	} `json:"fulfillment,omitempty"`
}

// HandleGenericWebhookCallback receives status and fulfillment updates from generic webhook integrations.
func (h *Handler) HandleGenericWebhookCallback(c *gin.Context) {
	token, ok := bearerToken(c.GetHeader("Authorization"))
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error_code": "unauthorized", "error_message": "invalid bearer token"})
		return
	}
	conn, err := h.SiteConnectionService.VerifyGenericWebhookToken(token)
	if err != nil {
		if errors.Is(err, service.ErrConnectionTokenInvalid) {
			c.JSON(http.StatusUnauthorized, gin.H{"ok": false, "error_code": "unauthorized", "error_message": "invalid bearer token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error_code": "internal_error", "error_message": "failed to verify connection"})
		return
	}
	if conn.Status != constants.ConnectionStatusActive {
		c.JSON(http.StatusForbidden, gin.H{"ok": false, "error_code": "connection_disabled", "error_message": "connection is not active"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, genericWebhookCallbackBodyLimit)
	var payload genericWebhookCallbackPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error_code": "invalid_request", "error_message": "invalid request body"})
		return
	}
	payload.Event = strings.TrimSpace(payload.Event)
	payload.OrderNo = strings.TrimSpace(payload.OrderNo)
	payload.Status = mapCallbackStatus(payload.Status)
	if payload.Event == "" || payload.OrderNo == "" || payload.Status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error_code": "invalid_request", "error_message": "event, order_no and status are required"})
		return
	}
	if payload.Event != "order.fulfilled" && payload.Event != "order.status_changed" {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error_code": "invalid_event", "error_message": "unsupported callback event"})
		return
	}
	if !isSupportedGenericWebhookStatus(payload.Status) {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error_code": "invalid_status", "error_message": "unsupported callback status"})
		return
	}
	if payload.Status == "delivered" && payload.Fulfillment == nil {
		c.JSON(http.StatusBadRequest, gin.H{"ok": false, "error_code": "fulfillment_required", "error_message": "fulfillment is required for delivered status"})
		return
	}

	procOrder, err := h.ProcurementOrderService.GetByLocalOrderNo(payload.OrderNo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error_code": "internal_error", "error_message": "failed to load procurement order"})
		return
	}
	if procOrder == nil || procOrder.ConnectionID != conn.ID {
		c.JSON(http.StatusNotFound, gin.H{"ok": false, "error_code": "order_not_found", "error_message": "order not found"})
		return
	}

	var fulfillment *upstreamadapter.UpstreamFulfillment
	if payload.Fulfillment != nil {
		fulfillmentType := strings.TrimSpace(payload.Fulfillment.Type)
		if fulfillmentType == "" {
			fulfillmentType = constants.FulfillmentTypeUpstream
		}
		fulfillment = &upstreamadapter.UpstreamFulfillment{
			Type:         fulfillmentType,
			Status:       constants.FulfillmentStatusDelivered,
			Payload:      payload.Fulfillment.Payload,
			DeliveryData: payload.Fulfillment.DeliveryData,
			DeliveredAt:  payload.Fulfillment.DeliveredAt,
		}
	}
	if err := h.ProcurementOrderService.HandleUpstreamCallback(procOrder.ID, payload.Status, fulfillment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"ok": false, "error_code": "callback_processing_failed", "error_message": "failed to process callback"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func isSupportedGenericWebhookStatus(status string) bool {
	switch status {
	case "delivered", "canceled", "refunded", "partially_refunded":
		return true
	default:
		return false
	}
}
