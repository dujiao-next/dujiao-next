package application

import (
	"context"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/crypto"
	"github.com/dujiao-next/internal/logger"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/upstream"

	"github.com/shopspring/decimal"
)

// MarkupReapplier 在连接定价配置（汇率/加价/取整）变更后，按新配置重算该连接已映射商品的本地售价。
// 由 Catalog Mapping 应用服务实现，通过 setter 注入以避免与本服务的循环依赖。
type MarkupReapplier interface {
	ReapplyMarkup(connectionID uint) (int, error)
}

// Service 对接连接服务。
type Service struct {
	connRepo        siteconnectioncontract.Repository
	encryptKey      []byte
	uploadsDir      string
	markupReapplier MarkupReapplier
}

// NewService 创建连接服务。
func NewService(connRepo siteconnectioncontract.Repository, appSecretKey, uploadsDir string) *Service {
	return &Service{
		connRepo:   connRepo,
		encryptKey: crypto.DeriveKey(appSecretKey),
		uploadsDir: uploadsDir,
	}
}

// SetMarkupReapplier 注入定价重算器（容器装配时调用）。
func (s *Service) SetMarkupReapplier(r MarkupReapplier) {
	s.markupReapplier = r
}

// NormalizeProdSellerDefaults applies the storefront's agreed ProdSeller
// pricing contract: cost × 5, cent ceiling, and automatic price sync. The
// operation is idempotent so restarts do not create additional changes.
func (s *Service) NormalizeProdSellerDefaults() error {
	if s == nil || s.connRepo == nil {
		return nil
	}
	connections, _, err := s.connRepo.List(siteconnectioncontract.ListFilter{})
	if err != nil {
		return err
	}
	for i := range connections {
		conn := &connections[i]
		if conn.Protocol != constants.ConnectionProtocolProdSeller {
			continue
		}
		desiredMarkup := decimal.NewFromInt(400)
		desiredRounding := "ceil_cent"
		if conn.PriceMarkupPercent.Equal(desiredMarkup) && strings.TrimSpace(conn.PriceRoundingMode) == desiredRounding && conn.AutoSyncPrice {
			continue
		}
		conn.PriceMarkupPercent = desiredMarkup
		conn.PriceRoundingMode = desiredRounding
		conn.AutoSyncPrice = true
		if err := s.connRepo.Update(conn); err != nil {
			return err
		}
		if s.markupReapplier != nil {
			if _, err := s.markupReapplier.ReapplyMarkup(conn.ID); err != nil {
				logger.Warnw("normalize_prodseller_markup_failed", "connection_id", conn.ID, "error", err)
			}
		}
		logger.Infow("normalize_prodseller_pricing_defaults_done", "connection_id", conn.ID)
	}
	return nil
}

// Create 创建连接
func (s *Service) Create(input CreateInput) (*siteconnectiondomain.Connection, error) {
	protocol := strings.TrimSpace(input.Protocol)
	if protocol == "" {
		protocol = constants.ConnectionProtocolDujiaoNext
	}
	baseURL := strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	if strings.TrimSpace(input.Name) == "" || (baseURL == "" && protocol != constants.ConnectionProtocolProdSeller) {
		return nil, siteconnectioncontract.ErrInvalid
	}

	secretRef := strings.TrimSpace(input.SecretRef)
	var encryptedSecret string
	if protocol == constants.ConnectionProtocolProdSeller {
		// ProdSeller 的真实 API Key 不能经过 ApiKey/ApiSecret 入库路径。
		if strings.TrimSpace(input.ApiKey) != "" || strings.TrimSpace(input.ApiSecret) != "" {
			return nil, siteconnectioncontract.ErrInvalid
		}
		if secretRef == "" {
			secretRef = constants.ConnectionSecretRefProdSeller
		}
		if baseURL == "" {
			baseURL = "https://prodseller.com/v1"
		}
	} else {
		if strings.TrimSpace(input.ApiKey) == "" || strings.TrimSpace(input.ApiSecret) == "" {
			return nil, siteconnectioncontract.ErrInvalid
		}
		var err error
		encryptedSecret, err = crypto.Encrypt(s.encryptKey, input.ApiSecret)
		if err != nil {
			return nil, err
		}
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
	markupPercent := decimal.NewFromFloat(input.PriceMarkupPercent)
	autoSyncPrice := input.AutoSyncPrice
	if protocol == constants.ConnectionProtocolProdSeller {
		if input.PriceMarkupPercent == 0 {
			// ProdSeller price is the USDT cost. The storefront default is cost × 5,
			// represented by a +400% markup in the generic pricing model.
			markupPercent = decimal.NewFromInt(400)
		}
		if strings.TrimSpace(input.PriceRoundingMode) == "" {
			roundingMode = "ceil_cent"
		}
		autoSyncPrice = true
	}

	conn := &siteconnectiondomain.Connection{
		Name:               strings.TrimSpace(input.Name),
		BaseURL:            baseURL,
		ApiKey:             strings.TrimSpace(input.ApiKey),
		ApiSecret:          encryptedSecret,
		SecretRef:          secretRef,
		Protocol:           protocol,
		CallbackURL:        strings.TrimSpace(input.CallbackURL),
		Status:             constants.ConnectionStatusPending,
		RetryMax:           retryMax,
		RetryIntervals:     retryIntervals,
		ExchangeRate:       s.normalizeExchangeRate(input.ExchangeRate),
		PriceMarkupPercent: markupPercent,
		PriceRoundingMode:  roundingMode,
		AutoSyncPrice:      autoSyncPrice,
	}

	if err := s.connRepo.Create(conn); err != nil {
		return nil, err
	}
	return conn, nil
}

// Update 更新连接
func (s *Service) Update(id uint, input UpdateInput) (*siteconnectiondomain.Connection, error) {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, siteconnectioncontract.ErrNotFound
	}

	// 记录定价配置旧值，用于判断本次保存是否需要重算已映射商品的本地售价。
	prevExchangeRate := conn.ExchangeRate
	prevMarkupPercent := conn.PriceMarkupPercent
	prevRoundingMode := conn.PriceRoundingMode

	targetProtocol := conn.Protocol
	if strings.TrimSpace(input.Protocol) != "" {
		targetProtocol = strings.TrimSpace(input.Protocol)
	}
	if targetProtocol == constants.ConnectionProtocolProdSeller &&
		(strings.TrimSpace(input.ApiKey) != "" || strings.TrimSpace(input.ApiSecret) != "") {
		return nil, siteconnectioncontract.ErrInvalid
	}

	if strings.TrimSpace(input.Name) != "" {
		conn.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.BaseURL) != "" {
		conn.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	}
	if strings.TrimSpace(input.ApiKey) != "" {
		conn.ApiKey = strings.TrimSpace(input.ApiKey)
	}
	if strings.TrimSpace(input.ApiSecret) != "" && targetProtocol != constants.ConnectionProtocolProdSeller {
		encrypted, err := crypto.Encrypt(s.encryptKey, input.ApiSecret)
		if err != nil {
			return nil, err
		}
		conn.ApiSecret = encrypted
	}
	if strings.TrimSpace(input.Protocol) != "" {
		conn.Protocol = targetProtocol
		if conn.Protocol == constants.ConnectionProtocolProdSeller {
			conn.ApiKey = ""
			conn.ApiSecret = ""
			if conn.SecretRef == "" {
				conn.SecretRef = constants.ConnectionSecretRefProdSeller
			}
		}
	}
	if strings.TrimSpace(input.SecretRef) != "" {
		conn.SecretRef = strings.TrimSpace(input.SecretRef)
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

	if err := s.connRepo.Update(conn); err != nil {
		return nil, err
	}

	// 定价配置（汇率/加价/取整）发生实际变化时，自动重算该连接已映射商品的本地售价，
	// 避免「改了汇率但已有商品价格不联动」。重算为尽力而为：失败不影响连接保存本身，
	// 仅记录告警，用户仍可通过「重新应用加价」手动补救。
	priceConfigChanged := !conn.ExchangeRate.Equal(prevExchangeRate) ||
		!conn.PriceMarkupPercent.Equal(prevMarkupPercent) ||
		conn.PriceRoundingMode != prevRoundingMode
	if priceConfigChanged && s.markupReapplier != nil {
		if _, err := s.markupReapplier.ReapplyMarkup(conn.ID); err != nil {
			logger.Warnw("reapply_markup_after_connection_update_failed",
				"connection_id", conn.ID, "error", err)
		}
	}

	return conn, nil
}

// Delete 删除连接
func (s *Service) Delete(id uint) error {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return err
	}
	if conn == nil {
		return siteconnectioncontract.ErrNotFound
	}
	return s.connRepo.Delete(id)
}

// GetByID 获取连接
func (s *Service) GetByID(id uint) (*siteconnectiondomain.Connection, error) {
	return s.connRepo.GetByID(id)
}

// List 列表查询
func (s *Service) List(filter siteconnectioncontract.ListFilter) ([]siteconnectiondomain.Connection, int64, error) {
	return s.connRepo.List(filter)
}

// SetStatus 设置连接状态
func (s *Service) SetStatus(id uint, status string) error {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return err
	}
	if conn == nil {
		return siteconnectioncontract.ErrNotFound
	}
	conn.Status = status
	return s.connRepo.Update(conn)
}

// Ping 测试连接
func (s *Service) Ping(id uint) (*PingResult, error) {
	conn, err := s.connRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, siteconnectioncontract.ErrNotFound
	}

	adapter, err := s.newAdapter(conn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	result, pingErr := adapter.Ping(ctx)
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
	if result == nil {
		return nil, nil
	}
	return &PingResult{
		SiteName:        result.SiteName,
		ProtocolVersion: result.ProtocolVersion,
		UserID:          result.UserID,
		Balance:         result.Balance,
		Currency:        result.Currency,
		MemberLevel:     result.MemberLevel,
	}, nil
}

// GetAdapter 获取连接的适配器（解密 secret 后构建）
func (s *Service) GetAdapter(conn *siteconnectiondomain.Connection) (upstream.Adapter, error) {
	return s.newAdapter(conn)
}

func (s *Service) newAdapter(conn *siteconnectiondomain.Connection) (upstream.Adapter, error) {
	if conn == nil {
		return nil, siteconnectioncontract.ErrNotFound
	}
	adapterConn := &siteconnectiondomain.Connection{
		BaseURL:   conn.BaseURL,
		ApiKey:    conn.ApiKey,
		Protocol:  conn.Protocol,
		SecretRef: conn.SecretRef,
	}
	if conn.Protocol != constants.ConnectionProtocolProdSeller {
		decrypted, err := s.decryptSecret(conn)
		if err != nil {
			return nil, err
		}
		adapterConn.ApiSecret = decrypted
	}
	return upstream.NewAdapter(adapterConn, s.uploadsDir)
}

func (s *Service) decryptSecret(conn *siteconnectiondomain.Connection) (string, error) {
	return crypto.Decrypt(s.encryptKey, conn.ApiSecret)
}

// DecryptSecret 解密加密后的 api_secret（公开方法，用于回调签名验证）
func (s *Service) DecryptSecret(encrypted string) (string, error) {
	return crypto.Decrypt(s.encryptKey, encrypted)
}

// normalizeExchangeRate 规范化汇率值，<=0 时返回 1
func (s *Service) normalizeExchangeRate(rate float64) decimal.Decimal {
	if rate <= 0 {
		return decimal.NewFromInt(1)
	}
	return decimal.NewFromFloat(rate)
}
