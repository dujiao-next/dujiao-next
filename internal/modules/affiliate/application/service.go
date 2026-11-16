package application

import (
	"time"

	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
)

const (
	affiliateCodeLength        = 8
	affiliateSplitTypePrefix   = "sp"
	affiliateAttributionWindow = 30 * 24 * time.Hour
	affiliateClickDedupeWindow = 10 * time.Minute
)

// Service 推广返利业务服务
type Service struct {
	repo        affiliatecontract.Store
	userRepo    usercontract.Store
	orderRepo   affiliatecontract.OrderReader
	productRepo affiliatecontract.ProductReader
	skuRepo     affiliatecontract.SKUReader
	settings    affiliatecontract.SettingsReader
}

// SetSKUReader 注入 SKU 读取依赖，用于解析 SKU 级返利比例。
// 未注入时按商品级/全站比例计算，已有调用方无需改动。
func (s *Service) SetSKUReader(reader affiliatecontract.SKUReader) {
	if s == nil {
		return
	}
	s.skuRepo = reader
}

// NewService 创建推广返利服务
func NewService(
	repo affiliatecontract.Store,
	userRepo usercontract.Store,
	orderRepo affiliatecontract.OrderReader,
	productRepo affiliatecontract.ProductReader,
	settings affiliatecontract.SettingsReader,
) *Service {
	return &Service{
		repo:        repo,
		userRepo:    userRepo,
		orderRepo:   orderRepo,
		productRepo: productRepo,
		settings:    settings,
	}
}
