package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/logger"
	channelclientapp "github.com/dujiao-next/internal/modules/channelclient/application"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/upstream"

	"github.com/hibiken/asynq"
)

// handleBotNotify 处理 Telegram Bot 事件回调任务。
func (c *Consumer) handleBotNotify(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		return nil
	}
	var payload queue.BotNotifyPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_bot_notify_unmarshal_failed", "error", err)
		return fmt.Errorf("unmarshal bot notify payload: %w", err)
	}
	if fixed, err := c.usesFixedPaymentEffects(payload.OrderID); err != nil {
		return err
	} else if fixed {
		return nil
	}
	return c.sendBotNotify(ctx, payload, nil)
}

func (c *Consumer) sendBotNotify(ctx context.Context, payload queue.BotNotifyPayload, admit func(context.Context) error) error {
	if c == nil || c.Container == nil || c.ChannelClientService == nil {
		return fmt.Errorf("bot endpoint reader unavailable")
	}
	if strings.TrimSpace(payload.TelegramUserID) == "" {
		logger.Debugw("worker_bot_notify_skip_invalid", "event_type", payload.EventType, "order_id", payload.OrderID, "telegram_user_id", payload.TelegramUserID, "recharge_no", payload.RechargeNo)
		return nil
	}

	// 从 DB 读取 telegram_bot 类型的活跃 ChannelClient
	channelEndpoint, err := c.ChannelClientService.GetActiveEndpoint("telegram_bot")
	if err != nil {
		if errors.Is(err, channelclientapp.ErrNotFound) {
			logger.Debugw("worker_bot_notify_skip_no_channel_client")
			return nil
		}
		logger.Warnw("worker_bot_notify_find_channel_client_failed", "error", err)
		return fmt.Errorf("find telegram bot channel client: %w", err)
	}
	if channelEndpoint == nil || channelEndpoint.CallbackURL == "" {
		logger.Debugw("worker_bot_notify_skip_no_channel_client")
		return nil
	}

	timestamp := time.Now().Unix()
	path := "/internal/order-fulfilled"
	requestBody := map[string]interface{}{
		"order_id":         payload.OrderID,
		"telegram_user_id": payload.TelegramUserID,
	}
	switch payload.EventType {
	case queue.BotNotifyEventOrderPaid:
		if payload.OrderID == 0 {
			logger.Debugw("worker_bot_notify_skip_invalid", "event_type", payload.EventType, "order_id", payload.OrderID, "telegram_user_id", payload.TelegramUserID)
			return nil
		}
		path = "/internal/order-paid"
	case "", queue.BotNotifyEventOrderFulfilled:
		if payload.OrderID == 0 {
			logger.Debugw("worker_bot_notify_skip_invalid", "event_type", payload.EventType, "order_id", payload.OrderID, "telegram_user_id", payload.TelegramUserID)
			return nil
		}
	case queue.BotNotifyEventWalletRechargeSucceeded:
		if strings.TrimSpace(payload.RechargeNo) == "" {
			logger.Debugw("worker_bot_notify_skip_invalid", "event_type", payload.EventType, "telegram_user_id", payload.TelegramUserID, "recharge_no", payload.RechargeNo)
			return nil
		}
		path = "/internal/wallet-recharge-succeeded"
		requestBody = map[string]interface{}{
			"recharge_no":      payload.RechargeNo,
			"telegram_user_id": payload.TelegramUserID,
			"amount":           payload.Amount,
			"currency":         payload.Currency,
		}
	default:
		logger.Debugw("worker_bot_notify_skip_unknown_event", "event_type", payload.EventType)
		return nil
	}
	body, _ := json.Marshal(requestBody)
	signature := upstream.Sign(channelEndpoint.ChannelSecret, "POST", path, timestamp, body)

	requestURL, err := buildBotNotifyRequestURL(channelEndpoint.CallbackURL, path)
	if err != nil {
		return fmt.Errorf("build bot notify request url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", requestURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create bot notify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Dujiao-Next-Channel-Key", channelEndpoint.ChannelKey)
	req.Header.Set("Dujiao-Next-Channel-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("Dujiao-Next-Channel-Signature", signature)

	httpClient := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if admit != nil {
		if err := admit(ctx); err != nil {
			return err
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Warnw("worker_bot_notify_request_failed",
			"order_id", payload.OrderID, "telegram_user_id", payload.TelegramUserID, "error", err)
		return fmt.Errorf("bot notify request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		logger.Debugw("worker_bot_notify_sent",
			"order_id", payload.OrderID, "telegram_user_id", payload.TelegramUserID)
		return nil
	}

	// Durable sends retain retryable failures and surface permanent rejection.
	if admit != nil && resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 408 && resp.StatusCode != 429 {
		return paymentcontract.ErrEffectPermanent
	}
	if admit != nil {
		return fmt.Errorf("bot delivery failed status %d", resp.StatusCode)
	}
	// 4xx 客户端错误不重试
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		logger.Warnw("worker_bot_notify_client_error",
			"order_id", payload.OrderID, "status", resp.StatusCode)
		return nil
	}

	// 5xx 返回 error 触发 asynq 重试
	return fmt.Errorf("bot notify unexpected status: %d", resp.StatusCode)
}

// buildBotNotifyRequestURL 构建 Bot 回调请求URL并重置查询参数。
func buildBotNotifyRequestURL(rawURL string, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid callback url: %s", rawURL)
	}

	parsed.Path = path
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return parsed.String(), nil
}

// handleTelegramBroadcast 处理 Telegram 群发任务。
func (c *Consumer) handleTelegramBroadcast(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		return nil
	}
	var payload queue.TelegramBroadcastPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_telegram_broadcast_unmarshal_failed", "error", err)
		return err
	}
	if payload.BroadcastID == 0 {
		logger.Debugw("worker_telegram_broadcast_skip_invalid")
		return nil
	}
	if c.TelegramBroadcastService == nil {
		return nil
	}
	return c.TelegramBroadcastService.ProcessBroadcast(ctx, payload.BroadcastID)
}
