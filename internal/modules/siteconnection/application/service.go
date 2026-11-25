package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	"golang.org/x/sync/singleflight"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/crypto"
	"github.com/dujiao-next/internal/logger"
	siteconnectioncontract "github.com/dujiao-next/internal/modules/siteconnection/contract"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/upstream"

	"github.com/shopspring/decimal"
)

// normalizeCategoryMarkups 清洗前端提交的分类加价：键必须是正整数的上游分类 ID，值必须是非负数，
// 其余条目直接丢弃。结果为空时返回 nil，避免库里留下空对象。
func normalizeCategoryMarkups(input map[string]float64) jsonmap.JSON {
	if len(input) == 0 {
		return nil
	}
	result := make(jsonmap.JSON, len(input))
	for key, percent := range input {
		id, err := strconv.ParseUint(strings.TrimSpace(key), 10, 64)
		if err != nil || id == 0 || percent < 0 {
			continue
		}
		result[strconv.FormatUint(id, 10)] = percent
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// categoryMarkupsEqual 比较两份分类加价是否相同（nil 与空视为相同）。
func categoryMarkupsEqual(a, b jsonmap.JSON) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

// MarkupReapplier 在连接定价配置（汇率/加价/取整）变更后，按新配置重算该连接已映射商品的本地售价。
// 由 Catalog Mapping 应用服务实现，通过 setter 注入以避免与本服务的循环依赖。
type MarkupReapplier interface {
	ReapplyMarkup(connectionID uint) (int, error)
	// ReapplyMarkupLocal 只用本地已知数据重算，不访问上游，供保存请求这种必须快速返回的路径使用。
	ReapplyMarkupLocal(connectionID uint) (int, error)
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

// Create 创建连接
func (s *Service) Create(input CreateInput) (*siteconnectiondomain.Connection, error) {
	if strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.BaseURL) == "" {
		return nil, siteconnectioncontract.ErrInvalid
	}
	if strings.TrimSpace(input.ApiKey) == "" || strings.TrimSpace(input.ApiSecret) == "" {
		return nil, siteconnectioncontract.ErrInvalid
	}

	protocol := strings.TrimSpace(input.Protocol)
	if protocol == "" {
		protocol = constants.ConnectionProtocolDujiaoNext
	}

	encryptedSecret, err := crypto.Encrypt(s.encryptKey, input.ApiSecret)
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

	conn := &siteconnectiondomain.Connection{
		Name:               strings.TrimSpace(input.Name),
		BaseURL:            strings.TrimRight(strings.TrimSpace(input.BaseURL), "/"),
		ApiKey:             strings.TrimSpace(input.ApiKey),
		ApiSecret:          encryptedSecret,
		Protocol:           protocol,
		CallbackURL:        strings.TrimSpace(input.CallbackURL),
		Status:             constants.ConnectionStatusPending,
		RetryMax:           retryMax,
		RetryIntervals:     retryIntervals,
		ExchangeRate:       s.normalizeExchangeRate(input.ExchangeRate),
		PriceMarkupPercent: decimal.NewFromFloat(input.PriceMarkupPercent),
		PriceRoundingMode:  roundingMode,
		CategoryMarkups:    normalizeCategoryMarkups(input.CategoryMarkups),
		AutoSyncPrice:      input.AutoSyncPrice,
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
	prevCategoryMarkups := conn.CategoryMarkups

	if strings.TrimSpace(input.Name) != "" {
		conn.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.BaseURL) != "" {
		conn.BaseURL = strings.TrimRight(strings.TrimSpace(input.BaseURL), "/")
	}
	if strings.TrimSpace(input.ApiKey) != "" {
		conn.ApiKey = strings.TrimSpace(input.ApiKey)
	}
	if strings.TrimSpace(input.ApiSecret) != "" {
		encrypted, err := crypto.Encrypt(s.encryptKey, input.ApiSecret)
		if err != nil {
			return nil, err
		}
		conn.ApiSecret = encrypted
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
	if input.CategoryMarkups != nil {
		conn.CategoryMarkups = normalizeCategoryMarkups(input.CategoryMarkups)
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
		conn.PriceRoundingMode != prevRoundingMode ||
		!categoryMarkupsEqual(conn.CategoryMarkups, prevCategoryMarkups)
	if priceConfigChanged && s.markupReapplier != nil {
		// 保存请求里只能用本地数据重算：完整重算要拉上游全部商品和分类树，
		// 最长可到 5 分钟，会把保存请求拖到反向代理超时、价格只改一半。
		// 需要补齐分类 ID 的场景留给「重新应用加价」按钮。
		if _, err := s.markupReapplier.ReapplyMarkupLocal(conn.ID); err != nil {
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

	// 解密 secret
	decrypted, err := s.decryptSecret(conn)
	if err != nil {
		return nil, err
	}

	adapter, err := upstream.NewAdapter(&siteconnectiondomain.Connection{
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
	decrypted, err := s.decryptSecret(conn)
	if err != nil {
		return nil, err
	}

	return upstream.NewAdapter(&siteconnectiondomain.Connection{
		BaseURL:   conn.BaseURL,
		ApiKey:    conn.ApiKey,
		ApiSecret: decrypted,
		Protocol:  conn.Protocol,
	}, s.uploadsDir)
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

const (
	balanceCheckTimeout  = 8 * time.Second
	balanceCheckCooldown = 20 * time.Second
	balanceStaleAfter    = 15 * time.Minute
)

var balanceNumber = regexp.MustCompile(`^-?[0-9]{1,24}(\.[0-9]{1,12})?$`)
var currencyCode = regexp.MustCompile(`^[A-Z]{2,12}$`)

type connectionProductStatsReader interface {
	ConnectionProductStats(ids []uint) ([]mappingcontract.ConnectionProductStats, error)
}

// Management reads account balances without changing connection state or purchasing behavior.
type Management struct {
	connections *Service
	balances    siteconnectioncontract.BalanceRepository
	products    connectionProductStatsReader
	checks      singleflight.Group
}

func NewManagement(connections *Service, balances siteconnectioncontract.BalanceRepository, products connectionProductStatsReader) *Management {
	return &Management{connections: connections, balances: balances, products: products}
}

func (m *Management) Overview() ([]UpstreamOverview, error) {
	connections, _, err := m.connections.List(siteconnectioncontract.ListFilter{})
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(connections))
	for _, conn := range connections {
		ids = append(ids, conn.ID)
	}
	snapshots, err := m.balances.ListBalanceSnapshots(ids)
	if err != nil {
		return nil, err
	}
	balances := make(map[uint]siteconnectiondomain.BalanceSnapshot, len(snapshots))
	for _, snapshot := range snapshots {
		balances[snapshot.ConnectionID] = snapshot
	}
	counts, err := m.products.ConnectionProductStats(ids)
	if err != nil {
		return nil, err
	}
	stats := make(map[uint]mappingcontract.ConnectionProductStats, len(counts))
	for _, count := range counts {
		stats[count.ConnectionID] = count
	}

	rows := make([]UpstreamOverview, 0, len(connections))
	now := time.Now()
	for _, conn := range connections {
		count := stats[conn.ID]
		rows = append(rows, UpstreamOverview{
			ID: conn.ID, Name: conn.Name, Protocol: conn.Protocol, Status: conn.Status,
			MappedProducts: count.Mapped, ListedProducts: count.Listed, LastSyncedAt: count.LastSyncedAt,
			Balance: balanceView(&conn, balances[conn.ID], now),
		})
	}
	return rows, nil
}

func (m *Management) Refresh(ctx context.Context, id uint) (BalanceView, error) {
	// The check is shared by concurrent visitors, even if one closes the page.
	ch := m.checks.DoChan(strconv.FormatUint(uint64(id), 10), func() (any, error) {
		return m.refresh(context.WithoutCancel(ctx), id)
	})
	select {
	case <-ctx.Done():
		return BalanceView{}, ctx.Err()
	case result := <-ch:
		if result.Err != nil {
			return BalanceView{}, result.Err
		}
		return result.Val.(BalanceView), nil
	}
}

func (m *Management) refresh(ctx context.Context, id uint) (BalanceView, error) {
	conn, snapshot, err := m.load(id)
	if err != nil {
		return BalanceView{}, err
	}
	now := time.Now()
	if conn.Status != constants.ConnectionStatusActive {
		return balanceView(conn, snapshot, now), nil
	}
	version := balanceAccountVersion(conn)
	if snapshot.AccountVersion != version {
		snapshot = siteconnectiondomain.BalanceSnapshot{ConnectionID: id, AccountVersion: version}
	}
	if snapshot.CheckedAt != nil && now.Sub(*snapshot.CheckedAt) < balanceCheckCooldown {
		return balanceView(conn, snapshot, now), nil
	}

	snapshot.CheckState = "error"
	if adapter, err := m.connections.GetAdapter(conn); err == nil {
		callCtx, cancel := context.WithTimeout(ctx, balanceCheckTimeout)
		result, pingErr := adapter.Ping(callCtx)
		cancel()
		if pingErr == nil {
			snapshot.CheckState = "unavailable"
			if result != nil && balanceNumber.MatchString(strings.TrimSpace(result.Balance)) {
				snapshot.Amount = strings.TrimSpace(result.Balance)
				snapshot.Currency = strings.ToUpper(strings.TrimSpace(result.Currency))
				if !currencyCode.MatchString(snapshot.Currency) {
					snapshot.Currency = ""
				}
				snapshot.CheckState = "ok"
				success := time.Now().UTC()
				snapshot.LastSuccessAt = &success
			}
		}
	}
	checkedAt := time.Now().UTC()
	snapshot.CheckedAt = &checkedAt
	if err := m.balances.SaveBalanceCheck(&snapshot); err != nil {
		return BalanceView{}, err
	}
	conn, snapshot, err = m.load(id)
	if err != nil {
		return BalanceView{}, err
	}
	return balanceView(conn, snapshot, time.Now()), nil
}

func (m *Management) SetThreshold(id uint, amount, currency string) (BalanceView, error) {
	conn, snapshot, err := m.load(id)
	if err != nil {
		return BalanceView{}, err
	}
	current := balanceView(conn, snapshot, time.Now())
	amount = strings.TrimSpace(amount)
	if current.Currency == "" || currency != current.Currency {
		return BalanceView{}, siteconnectioncontract.ErrInvalid
	}
	if amount != "" {
		if !balanceNumber.MatchString(amount) {
			return BalanceView{}, siteconnectioncontract.ErrInvalid
		}
		value, _ := decimal.NewFromString(amount)
		if value.IsNegative() {
			return BalanceView{}, siteconnectioncontract.ErrInvalid
		}
		amount = value.String()
	}
	if err := m.balances.SaveBalanceThreshold(id, amount, currency); err != nil {
		return BalanceView{}, err
	}
	snapshot.Threshold, snapshot.ThresholdCurrency = amount, currency
	return balanceView(conn, snapshot, time.Now()), nil
}

func (m *Management) load(id uint) (*siteconnectiondomain.Connection, siteconnectiondomain.BalanceSnapshot, error) {
	conn, err := m.connections.GetByID(id)
	if err != nil {
		return nil, siteconnectiondomain.BalanceSnapshot{}, err
	}
	if conn == nil {
		return nil, siteconnectiondomain.BalanceSnapshot{}, siteconnectioncontract.ErrNotFound
	}
	snapshots, err := m.balances.ListBalanceSnapshots([]uint{id})
	if len(snapshots) == 0 || err != nil {
		return conn, siteconnectiondomain.BalanceSnapshot{}, err
	}
	return conn, snapshots[0], nil
}

func balanceAccountVersion(conn *siteconnectiondomain.Connection) string {
	hash := sha256.Sum256([]byte(strings.Join([]string{conn.Protocol, conn.BaseURL, conn.ApiKey, conn.ApiSecret}, "\x00")))
	return hex.EncodeToString(hash[:])
}

func balanceView(conn *siteconnectiondomain.Connection, snapshot siteconnectiondomain.BalanceSnapshot, now time.Time) BalanceView {
	view := BalanceView{State: "unchecked", Risk: "unknown"}
	if snapshot.AccountVersion == balanceAccountVersion(conn) && snapshot.CheckedAt != nil {
		view.Amount, view.Currency = snapshot.Amount, snapshot.Currency
		view.CheckedAt, view.LastSuccessAt = snapshot.CheckedAt, snapshot.LastSuccessAt
		view.State = snapshot.CheckState
		if view.State == "ok" && now.Sub(*view.CheckedAt) >= balanceStaleAfter {
			view.State = "stale"
		}
	}
	if conn.Status != constants.ConnectionStatusActive {
		view.State = "disabled"
	}
	if snapshot.Threshold != "" && view.Currency == snapshot.ThresholdCurrency {
		view.Threshold = snapshot.Threshold
	} else {
		switch view.Currency {
		case "CNY":
			view.Threshold = "50"
		case "USD", "USDT":
			view.Threshold = "10"
		}
	}
	amount, err := decimal.NewFromString(view.Amount)
	if err != nil || view.State == "disabled" {
		return view
	}
	threshold, _ := decimal.NewFromString(view.Threshold)
	switch {
	case !amount.IsPositive():
		view.Risk = "empty"
	case amount.LessThanOrEqual(threshold):
		view.Risk = "low"
	default:
		view.Risk = "normal"
	}
	return view
}
