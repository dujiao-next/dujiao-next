package application

import (
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

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

// noopSKUReader 在调用方没有提供 SKU 读取器时的空实现。
// 用它兜底而不是在各处判 nil，避免新增构造点时漏注入导致 SKU 级比例被静默忽略。
type noopSKUReader struct{}

func (noopSKUReader) ListByIDs([]uint) ([]productdomain.ProductSKU, error) {
	return nil, nil
}

func (noopSKUReader) ListByProduct(uint, bool) ([]productdomain.ProductSKU, error) {
	return nil, nil
}

// NewService 创建推广返利服务
func NewService(
	repo affiliatecontract.Store,
	userRepo usercontract.Store,
	orderRepo affiliatecontract.OrderReader,
	productRepo affiliatecontract.ProductReader,
	skuRepo affiliatecontract.SKUReader,
	settings affiliatecontract.SettingsReader,
) *Service {
	if skuRepo == nil {
		skuRepo = noopSKUReader{}
	}
	return &Service{
		repo:        repo,
		userRepo:    userRepo,
		orderRepo:   orderRepo,
		productRepo: productRepo,
		skuRepo:     skuRepo,
		settings:    settings,
	}
}
