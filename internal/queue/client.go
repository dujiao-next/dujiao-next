package queue

import (
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"

	"github.com/hibiken/asynq"
)

const (
	// DefaultQueue 默认队列名称
	DefaultQueue = constants.QueueDefault
)

// Client 队列客户端封装
type Client struct {
	client       *asynq.Client
	enabled      bool
	defaultQueue string
}

// NewClient 创建队列客户端
func NewClient(cfg *config.QueueConfig) (*Client, error) {
	if cfg == nil || !cfg.Enabled {
		return &Client{enabled: false, defaultQueue: DefaultQueue}, nil
	}
	opt := buildRedisOpt(cfg)
	client := asynq.NewClient(opt)
	return &Client{
		client:       client,
		enabled:      true,
		defaultQueue: DefaultQueue,
	}, nil
}

// Enabled 判断是否启用
func (c *Client) Enabled() bool {
	return c != nil && c.enabled && c.client != nil
}

// Close 关闭客户端
func (c *Client) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}

// EnqueueOrderStatusEmail 推送订单状态邮件任务
func (c *Client) EnqueueOrderStatusEmail(payload OrderStatusEmailPayload, opts ...asynq.Option) error {
	if !c.Enabled() {
		return nil
	}
	task, err := NewOrderStatusEmailTask(payload)
	if err != nil {
		return err
	}
	// 限制重试次数与保留期，避免 SMTP 长时间不可用时邮件任务堆积成雪崩；
	// 调用方可通过 opts 覆盖。
	options := append([]asynq.Option{
		asynq.Queue(c.defaultQueue),
		asynq.MaxRetry(3),
		asynq.Retention(24 * time.Hour),
	}, opts...)
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueOrderAutoFulfill 推送自动交付任务
func (c *Client) EnqueueOrderAutoFulfill(payload OrderAutoFulfillPayload, opts ...asynq.Option) error {
	if !c.Enabled() {
		return nil
	}
	task, err := NewOrderAutoFulfillTask(payload)
	if err != nil {
		return err
	}
	options := append([]asynq.Option{asynq.Queue(c.defaultQueue)}, opts...)
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueAffiliateOrderPaid 推送佣金创建重试任务
func (c *Client) EnqueueAffiliateOrderPaid(payload AffiliateOrderPaidPayload, opts ...asynq.Option) error {
	if !c.Enabled() {
		return nil
	}
	task, err := NewAffiliateOrderPaidTask(payload)
	if err != nil {
		return err
	}
	options := append([]asynq.Option{asynq.Queue(c.defaultQueue)}, opts...)
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueOrderTimeoutCancel 推送订单超时取消任务
func (c *Client) EnqueueOrderTimeoutCancel(payload OrderTimeoutCancelPayload, delay time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	if delay < 0 {
		delay = 0
	}
	task, err := NewOrderTimeoutCancelTask(payload)
	if err != nil {
		return err
	}
	options := []asynq.Option{asynq.Queue(c.defaultQueue), asynq.ProcessIn(delay)}
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueWalletRechargeExpire 推送钱包充值超时过期任务
func (c *Client) EnqueueWalletRechargeExpire(payload WalletRechargeExpirePayload, delay time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	if delay < 0 {
		delay = 0
	}
	task, err := NewWalletRechargeExpireTask(payload)
	if err != nil {
		return err
	}
	options := []asynq.Option{asynq.Queue(c.defaultQueue), asynq.ProcessIn(delay)}
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueNotificationDispatch 推送通知中心分发任务
func (c *Client) EnqueueNotificationDispatch(payload NotificationDispatchPayload, opts ...asynq.Option) error {
	if !c.Enabled() {
		return nil
	}
	task, err := NewNotificationDispatchTask(payload)
	if err != nil {
		return err
	}
	options := append([]asynq.Option{asynq.Queue(c.defaultQueue)}, opts...)
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueProcurementSubmit 推送采购提交任务
func (c *Client) EnqueueProcurementSubmit(payload ProcurementSubmitPayload, opts ...asynq.Option) error {
	if !c.Enabled() {
		return nil
	}
	task, err := NewProcurementSubmitTask(payload)
	if err != nil {
		return err
	}
	// 采购单服务自行管理重试逻辑，asynq 仅处理瞬态错误（DB/Redis 不可达等）
	options := append([]asynq.Option{asynq.Queue(c.defaultQueue), asynq.MaxRetry(3)}, opts...)
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueProcurementPollStatus 推送采购状态轮询任务
func (c *Client) EnqueueProcurementPollStatus(payload ProcurementPollStatusPayload, delay time.Duration) error {
	if !c.Enabled() {
		return nil
	}
	if delay < 0 {
		delay = 0
	}
	task, err := NewProcurementPollStatusTask(payload)
	if err != nil {
		return err
	}
	options := []asynq.Option{asynq.Queue(c.defaultQueue), asynq.ProcessIn(delay)}
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueDownstreamCallback 推送下游回调通知任务
func (c *Client) EnqueueDownstreamCallback(payload DownstreamCallbackPayload, opts ...asynq.Option) error {
	if !c.Enabled() {
		return nil
	}
	task, err := NewDownstreamCallbackTask(payload)
	if err != nil {
		return err
	}
	options := append([]asynq.Option{asynq.Queue(c.defaultQueue)}, opts...)
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueReconciliationRun 入队对账执行任务
func (c *Client) EnqueueReconciliationRun(payload ReconciliationRunPayload, opts ...asynq.Option) error {
	if !c.Enabled() {
		return nil
	}
	task, err := NewReconciliationRunTask(payload)
	if err != nil {
		return err
	}
	options := append([]asynq.Option{asynq.Queue(c.defaultQueue)}, opts...)
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueBotNotify 入队 Bot 交付通知任务
func (c *Client) EnqueueBotNotify(payload BotNotifyPayload, opts ...asynq.Option) error {
	if !c.Enabled() {
		return nil
	}
	task, err := NewBotNotifyTask(payload)
	if err != nil {
		return err
	}
	options := append([]asynq.Option{asynq.Queue(c.defaultQueue), asynq.MaxRetry(5)}, opts...)
	_, err = c.client.Enqueue(task, options...)
	return err
}

// EnqueueTelegramBroadcast 入队 Telegram 群发任务。
func (c *Client) EnqueueTelegramBroadcast(payload TelegramBroadcastPayload, opts ...asynq.Option) error {
	if !c.Enabled() {
		return nil
	}
	task, err := NewTelegramBroadcastTask(payload)
	if err != nil {
		return err
	}
	options := append([]asynq.Option{asynq.Queue(c.defaultQueue), asynq.MaxRetry(3)}, opts...)
	_, err = c.client.Enqueue(task, options...)
	return err
}

// BuildServerConfig 生成队列服务配置
func BuildServerConfig(cfg *config.QueueConfig) (asynq.RedisClientOpt, asynq.Config) {
	opt := buildRedisOpt(cfg)
	concurrency := 10
	if cfg != nil && cfg.Concurrency > 0 {
		concurrency = cfg.Concurrency
	}
	queues := map[string]int{DefaultQueue: 1}
	if cfg != nil && len(cfg.Queues) > 0 {
		queues = cfg.Queues
	}
	return opt, asynq.Config{
		Concurrency: concurrency,
		Queues:      queues,
	}
}

func buildRedisOpt(cfg *config.QueueConfig) asynq.RedisClientOpt {
	host := "127.0.0.1"
	port := 6379
	password := ""
	db := 0
	if cfg != nil {
		if strings.TrimSpace(cfg.Host) != "" {
			host = strings.TrimSpace(cfg.Host)
		}
		if cfg.Port > 0 {
			port = cfg.Port
		}
		password = cfg.Password
		db = cfg.DB
	}
	return asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       db,
	}
}
