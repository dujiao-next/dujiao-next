package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/crypto"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/upstream"

	"github.com/shopspring/decimal"
)

var (
	ErrConnectionNotFound     = errors.New("site connection not found")
	ErrConnectionInvalid      = errors.New("site connection is invalid")
	ErrConnectionTokenInvalid = errors.New("site connection token is invalid")
)

type ConnectionProtocolCapabilities struct {
	CatalogSync   bool `json:"catalog_sync"`
	OrderQuery    bool `json:"order_query"`
	OrderCancel   bool `json:"order_cancel"`
	AsyncCallback bool `json:"async_callback"`
}

type ConnectionProtocolInfo struct {
	Value        string                         `json:"value"`
	Label        string                         `json:"label"`
	Capabilities ConnectionProtocolCapabilities `json:"capabilities"`
}

func SupportedConnectionProtocols() []ConnectionProtocolInfo {
	return []ConnectionProtocolInfo{
		{
			Value: constants.ConnectionProtocolDujiaoNext,
			Label: "Dujiao Next",
			Capabilities: ConnectionProtocolCapabilities{
				CatalogSync: true, OrderQuery: true, OrderCancel: true, AsyncCallback: true,
			},
		},
		{
			Value: constants.ConnectionProtocolGenericWebhook,
			Label: "通用 WebHook",
			Capabilities: ConnectionProtocolCapabilities{
				AsyncCallback: true,
			},
		},
	}
}

// MarkupReapplier 在连接定价配置（汇率/加价/取整）变更后，按新配置重算该连接已映射商品的本地售价。
// 由 ProductMappingService 实现，通过 setter 注入以避免与本服务的循环依赖。
type MarkupReapplier interface {
	ReapplyMarkup(connectionID uint) (int, error)
}

// SiteConnectionService 对接连接服务
type SiteConnectionService struct {
	connRepo        repository.SiteConnectionRepository
	encryptKey      []byte
	uploadsDir      string
	markupReapplier MarkupReapplier
}

// NewSiteConnectionService 创建连接服务
func NewSiteConnectionService(connRepo repository.SiteConnectionRepository, appSecretKey, uploadsDir string) *SiteConnectionService {
	return &SiteConnectionService{
		connRepo:   connRepo,
		encryptKey: crypto.DeriveKey(appSecretKey),
		uploadsDir: uploadsDir,
	}
}

// SetMarkupReapplier 注入定价重算器（容器装配时调用）。
func (s *SiteConnectionService) SetMarkupReapplier(r MarkupReapplier) {
	s.markupReapplier = r
}

// CreateConnectionInput 创建连接输入
type CreateConnectionInput struct {
	Name               string  `json:"name"`
	BaseURL            string  `json:"base_url"`
	ApiKey             string  `json:"api_key"`
	ApiSecret          string  `json:"api_secret"`
	Protocol           string  `json:"protocol"`
	CallbackURL        string  `json:"callback_url"`
	RetryMax           int     `json:"retry_max"`
	RetryIntervals     string  `json:"retry_intervals"`
	ExchangeRate       float64 `json:"exchange_rate"`
	PriceMarkupPercent float64 `json:"price_markup_percent"`
	PriceRoundingMode  string  `json:"price_rounding_mode"`
	AutoSyncPrice      bool    `json:"auto_sync_price"`
}

// Create 创建连接
func (s *SiteConnectionService) Create(input CreateConnectionInput) (*models.SiteConnection, error) {
	protocol := strings.TrimSpace(input.Protocol)
	if protocol == "" {
		protocol = constants.ConnectionProtocolDujiaoNext
	}
	name := strings.TrimSpace(input.Name)
	baseURL := strings.TrimSpace(input.BaseURL)
	apiKey := strings.TrimSpace(input.ApiKey)
	apiSecret := strings.TrimSpace(input.ApiSecret)
	callbackURL := strings.TrimSpace(input.CallbackURL)
	if err := validateSiteConnection(protocol, name, baseURL, apiKey, apiSecret, callbackURL); err != nil {
		return nil, err
	}
	if err := s.validateWebhookApiKey(protocol, apiKey, 0); err != nil {
		return nil, err
	}

	encryptedSecret, err := crypto.Encrypt(s.encryptKey, apiSecret)
	if err != nil {
		return nil, err
	}
	retryMax := input.RetryMax
	if retryMax <= 0 {
		retryMax = 5
	}
	retryIntervals := strings.TrimSpace(input.RetryIntervals)
	if retryIntervals == "" {
		retryIntervals = "[30,60,300]"
	}

	roundingMode := strings.TrimSpace(input.PriceRoundingMode)
	if roundingMode == "" {
		roundingMode = "none"
	}

	conn := &models.SiteConnection{
		Name:               name,
		BaseURL:            normalizeConnectionURL(protocol, baseURL),
		ApiKey:             apiKey,
		ApiSecret:          encryptedSecret,
		Protocol:           protocol,
		CallbackURL:        callbackURL,
		Status:             constants.ConnectionStatusPending,
		RetryMax:           retryMax,
		RetryIntervals:     retryIntervals,
		ExchangeRate:       s.normalizeExchangeRate(input.ExchangeRate),
		PriceMarkupPercent: decimal.NewFromFloat(input.PriceMarkupPercent),
		PriceRoundingMode:  roundingMode,
		AutoSyncPrice:      input.AutoSyncPrice,
	}
	if err := s.connRepo.Create(conn); err != nil {
		return nil, err
	}
	return conn, nil
}

// UpdateConnectionInput 更新连接输入
type UpdateConnectionInput struct {
	Name               string   `json:"name"`
	BaseURL            string   `json:"base_url"`
	ApiKey             string   `json:"api_key"`
	ApiSecret          string   `json:"api_secret"` // 为空则不更新
	Protocol           string   `json:"protocol"`
	CallbackURL        string   `json:"callback_url"`
	RetryMax           int      `json:"retry_max"`
	RetryIntervals     string   `json:"retry_intervals"`
	ExchangeRate       *float64 `json:"exchange_rate"`
	PriceMarkupPercent *float64 `json:"price_markup_percent"` // 指针类型，区分 0 和未传
	PriceRoundingMode  *string  `json:"price_rounding_mode"`
	AutoSyncPrice      *bool    `json:"auto_sync_price"`
}

// Update 更新连接
func (s *SiteConnectionService) Update(id uint, input UpdateConnectionInput) (*models.SiteConnection, error) {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, ErrConnectionNotFound
	}

	// 记录定价配置旧值，用于判断本次保存是否需要重算已映射商品的本地售价。
	prevExchangeRate := conn.ExchangeRate
	prevMarkupPercent := conn.PriceMarkupPercent
	prevRoundingMode := conn.PriceRoundingMode

	if strings.TrimSpace(input.Name) != "" {
		conn.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.BaseURL) != "" {
		conn.BaseURL = strings.TrimSpace(input.BaseURL)
	}
	if strings.TrimSpace(input.ApiKey) != "" {
		conn.ApiKey = strings.TrimSpace(input.ApiKey)
	}
	plainSecret := ""
	if strings.TrimSpace(input.ApiSecret) != "" {
		plainSecret = strings.TrimSpace(input.ApiSecret)
		encrypted, err := crypto.Encrypt(s.encryptKey, plainSecret)
		if err != nil {
			return nil, err
		}
		conn.ApiSecret = encrypted
	} else if strings.TrimSpace(conn.ApiSecret) != "" {
		plainSecret, err = s.decryptSecret(conn)
		if err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(input.Protocol) != "" {
		conn.Protocol = strings.TrimSpace(input.Protocol)
	}
	if input.CallbackURL != "" {
		conn.CallbackURL = strings.TrimSpace(input.CallbackURL)
	}
	if input.RetryMax > 0 {
		conn.RetryMax = input.RetryMax
	}
	if strings.TrimSpace(input.RetryIntervals) != "" {
		conn.RetryIntervals = strings.TrimSpace(input.RetryIntervals)
	}
	if input.ExchangeRate != nil {
		conn.ExchangeRate = s.normalizeExchangeRate(*input.ExchangeRate)
	}
	if input.PriceMarkupPercent != nil {
		conn.PriceMarkupPercent = decimal.NewFromFloat(*input.PriceMarkupPercent)
	}
	if input.PriceRoundingMode != nil {
		mode := strings.TrimSpace(*input.PriceRoundingMode)
		if mode == "" {
			mode = "none"
		}
		conn.PriceRoundingMode = mode
	}
	if input.AutoSyncPrice != nil {
		conn.AutoSyncPrice = *input.AutoSyncPrice
	}
	conn.BaseURL = normalizeConnectionURL(conn.Protocol, conn.BaseURL)
	if err := validateSiteConnection(conn.Protocol, conn.Name, conn.BaseURL, conn.ApiKey, plainSecret, conn.CallbackURL); err != nil {
		return nil, err
	}
	if err := s.validateWebhookApiKey(conn.Protocol, conn.ApiKey, conn.ID); err != nil {
		return nil, err
	}

	if err := s.connRepo.Update(conn); err != nil {
		return nil, err
	}

	// 定价配置（汇率/加价/取整）发生实际变化时，自动重算该连接已映射商品的本地售价，
	// 避免「改了汇率但已有商品价格不联动」。重算为尽力而为：失败不影响连接保存本身，
	// 仅记录告警，用户仍可通过「重新应用加价」手动补救。
	priceConfigChanged := !conn.ExchangeRate.Equal(prevExchangeRate) ||
		!conn.PriceMarkupPercent.Equal(prevMarkupPercent) ||
		conn.PriceRoundingMode != prevRoundingMode
	if priceConfigChanged && conn.Protocol == constants.ConnectionProtocolDujiaoNext && s.markupReapplier != nil {
		if _, err := s.markupReapplier.ReapplyMarkup(conn.ID); err != nil {
			logger.Warnw("reapply_markup_after_connection_update_failed",
				"connection_id", conn.ID, "error", err)
		}
	}

	return conn, nil
}

// Delete 删除连接
func (s *SiteConnectionService) Delete(id uint) error {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return err
	}
	if conn == nil {
		return ErrConnectionNotFound
	}
	return s.connRepo.Delete(id)
}

// GetByID 获取连接
func (s *SiteConnectionService) GetByID(id uint) (*models.SiteConnection, error) {
	return s.connRepo.GetByID(id)
}

// List 列表查询
func (s *SiteConnectionService) List(filter repository.SiteConnectionListFilter) ([]models.SiteConnection, int64, error) {
	return s.connRepo.List(filter)
}

// SetStatus 设置连接状态
func (s *SiteConnectionService) SetStatus(id uint, status string) error {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return err
	}
	if conn == nil {
		return ErrConnectionNotFound
	}
	conn.Status = status
	return s.connRepo.Update(conn)
}

// Ping 测试连接
func (s *SiteConnectionService) Ping(id uint) (*upstream.PingResult, error) {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, ErrConnectionNotFound
	}

	// 解密 secret
	decrypted, err := s.decryptSecret(conn)
	if err != nil {
		return nil, err
	}

	tester, err := upstream.NewConnectionTester(&models.SiteConnection{
		BaseURL:   conn.BaseURL,
		ApiKey:    conn.ApiKey,
		ApiSecret: decrypted,
		Protocol:  conn.Protocol,
	}, s.uploadsDir)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, pingErr := tester.Ping(ctx)
	now := time.Now()
	conn.LastPingAt = &now
	conn.LastPingOK = pingErr == nil

	if pingErr == nil && conn.Status == constants.ConnectionStatusPending {
		conn.Status = constants.ConnectionStatusActive
	}

	// 更新连接状态（不管 ping 是否成功）
	_ = s.connRepo.Update(conn)

	if pingErr != nil {
		return nil, pingErr
	}
	return result, nil
}

// GetAdapter 获取连接的适配器（解密 secret 后构建）
func (s *SiteConnectionService) GetAdapter(conn *models.SiteConnection) (upstream.Adapter, error) {
	decrypted, err := s.decryptSecret(conn)
	if err != nil {
		return nil, err
	}

	return upstream.NewAdapter(&models.SiteConnection{
		BaseURL:   conn.BaseURL,
		ApiKey:    conn.ApiKey,
		ApiSecret: decrypted,
		Protocol:  conn.Protocol,
	}, s.uploadsDir)
}

func (s *SiteConnectionService) GetOrderSubmitter(conn *models.SiteConnection) (upstream.OrderSubmitter, error) {
	decrypted, err := s.decryptSecret(conn)
	if err != nil {
		return nil, err
	}
	return upstream.NewOrderSubmitter(&models.SiteConnection{
		BaseURL:     conn.BaseURL,
		ApiKey:      conn.ApiKey,
		ApiSecret:   decrypted,
		Protocol:    conn.Protocol,
		CallbackURL: conn.CallbackURL,
	}, s.uploadsDir)
}

func (s *SiteConnectionService) VerifyGenericWebhookToken(token string) (*models.SiteConnection, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrConnectionTokenInvalid
	}
	conn, err := s.connRepo.GetByApiKey(token)
	if err != nil {
		return nil, err
	}
	if conn == nil || conn.Protocol != constants.ConnectionProtocolGenericWebhook {
		return nil, ErrConnectionTokenInvalid
	}
	return conn, nil
}

func (s *SiteConnectionService) decryptSecret(conn *models.SiteConnection) (string, error) {
	return crypto.Decrypt(s.encryptKey, conn.ApiSecret)
}

// DecryptSecret 解密加密后的 api_secret（公开方法，用于回调签名验证）
func (s *SiteConnectionService) DecryptSecret(encrypted string) (string, error) {
	return crypto.Decrypt(s.encryptKey, encrypted)
}

func validateSiteConnection(protocol, name, baseURL, apiKey, apiSecret, callbackURL string) error {
	if strings.TrimSpace(name) == "" || !isHTTPURL(baseURL) {
		return ErrConnectionInvalid
	}
	switch protocol {
	case constants.ConnectionProtocolDujiaoNext:
		if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiSecret) == "" {
			return ErrConnectionInvalid
		}
	case constants.ConnectionProtocolGenericWebhook:
		if strings.TrimSpace(apiKey) == "" || !isHTTPURL(callbackURL) {
			return ErrConnectionInvalid
		}
	default:
		return ErrConnectionInvalid
	}
	return nil
}

// validateWebhookApiKey prevents ambiguous Bearer-token lookup without changing
// the legacy rule that Dujiao Next API keys are not globally unique.
func (s *SiteConnectionService) validateWebhookApiKey(protocol, apiKey string, connectionID uint) error {
	existing, err := s.connRepo.GetByApiKey(apiKey)
	if err != nil {
		return err
	}
	if existing == nil || existing.ID == connectionID {
		return nil
	}
	if protocol == constants.ConnectionProtocolGenericWebhook || existing.Protocol == constants.ConnectionProtocolGenericWebhook {
		return ErrConnectionInvalid
	}
	return nil
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func normalizeConnectionURL(protocol, raw string) string {
	raw = strings.TrimSpace(raw)
	if protocol == constants.ConnectionProtocolDujiaoNext {
		return strings.TrimRight(raw, "/")
	}
	return raw
}

// normalizeExchangeRate 规范化汇率值，<=0 时返回 1
func (s *SiteConnectionService) normalizeExchangeRate(rate float64) decimal.Decimal {
	if rate <= 0 {
		return decimal.NewFromInt(1)
	}
	return decimal.NewFromFloat(rate)
}
