package application

import (
	"strconv"
	"testing"

	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestCalculateMarkedUpPrice(t *testing.T) {
	tests := []struct {
		name     string
		upstream string
		markup   string
		rounding string
		expected string
	}{
		{"zero markup returns original", "10.00", "0", "none", "10.00"},
		{"50% markup", "10.00", "50", "none", "15.00"},
		{"100% markup doubles price", "10.00", "100", "none", "20.00"},
		{"200% markup triples price", "10.00", "200", "none", "30.00"},
		{"small price with markup", "0.01", "100", "none", "0.02"},
		{"fractional result rounds to 2 decimals", "10.00", "33", "none", "13.30"},
		{"negative markup floors at zero", "5.00", "-200", "none", "0"},
		{"ceil_int rounds up", "10.00", "23.4", "ceil_int", "13.00"},
		{"ceil_int exact integer stays", "10.00", "100", "ceil_int", "20"},
		{"ceil_tenth rounds up to 0.1", "10.00", "23.4", "ceil_tenth", "12.40"},
		{"ceil_tenth exact tenth stays", "10.00", "50", "ceil_tenth", "15.00"},
		{"zero price stays zero", "0.00", "100", "none", "0.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := decimal.RequireFromString(tt.upstream)
			markup := decimal.RequireFromString(tt.markup)
			expected := decimal.RequireFromString(tt.expected)

			result := CalculateMarkedUpPrice(upstream, markup, tt.rounding)
			if !result.Equal(expected) {
				t.Errorf("CalculateMarkedUpPrice(%s, %s%%, %s) = %s, want %s",
					tt.upstream, tt.markup, tt.rounding, result.String(), tt.expected)
			}
		})
	}
}

func TestCalculateLocalPrice(t *testing.T) {
	tests := []struct {
		name     string
		upstream string
		rate     string
		markup   string
		rounding string
		expected string
	}{
		{"USD to CNY with markup", "1.00", "7.2", "50", "none", "10.80"},
		{"rate 1 same as no conversion", "10.00", "1", "50", "none", "15.00"},
		{"rate 0 treated as 1", "10.00", "0", "50", "none", "15.00"},
		{"negative rate treated as 1", "10.00", "-1", "50", "none", "15.00"},
		{"rate with no markup", "1.00", "7.2", "0", "none", "7.20"},
		{"rate with ceil_int", "1.00", "7.2", "50", "ceil_int", "11.00"},
		{"rate with ceil_tenth", "1.00", "7.35", "0", "ceil_tenth", "7.35"},
		{"rate with ceil_tenth fractional", "1.00", "7.34", "1", "ceil_tenth", "7.50"},
		{"high precision rate", "100.00", "0.138462", "0", "none", "13.85"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := decimal.RequireFromString(tt.upstream)
			rate := decimal.RequireFromString(tt.rate)
			markup := decimal.RequireFromString(tt.markup)
			expected := decimal.RequireFromString(tt.expected)

			result := CalculateLocalPrice(upstream, rate, markup, tt.rounding)
			if !result.Equal(expected) {
				t.Errorf("CalculateLocalPrice(%s, rate=%s, %s%%, %s) = %s, want %s",
					tt.upstream, tt.rate, tt.markup, tt.rounding, result.String(), tt.expected)
			}
		})
	}
}

func TestResolveCategoryMarkup(t *testing.T) {
	def := decimal.RequireFromString("10")
	markups := jsonmap.JSON{
		"5": float64(50), // 父分类
		"7": "20",        // 非 float64（生产中不会出现）：视为未配置，向上继承
		"8": float64(0),  // 显式 0：该分类不加价，不能回落到默认
		"9": float64(-5), // 负数视为未配置
		"x": float64(99), // 非法键被忽略
	}
	parents := map[uint]uint{6: 5, 7: 5, 8: 5, 9: 5, 10: 6, 20: 21, 21: 20} // 20/21 成环

	tests := []struct {
		name       string
		markups    jsonmap.JSON
		categoryID uint
		parents    map[uint]uint
		want       string
	}{
		{"no config uses default", nil, 5, parents, "10"},
		{"uncategorized uses default", markups, 0, parents, "10"},
		{"own config wins", markups, 5, parents, "50"},
		{"non-numeric value ignored, inherits parent", markups, 7, parents, "50"},
		{"explicit zero is respected", markups, 8, parents, "0"},
		{"child inherits parent", markups, 6, parents, "50"},
		{"grandchild inherits ancestor", markups, 10, parents, "50"},
		{"negative falls through to parent", markups, 9, parents, "50"},
		{"unknown category uses default", markups, 99, parents, "10"},
		{"nil parents does not inherit", markups, 6, nil, "10"},
		{"parent cycle terminates with default", markups, 20, parents, "10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveCategoryMarkup(def, tt.markups, tt.categoryID, tt.parents)
			if !got.Equal(decimal.RequireFromString(tt.want)) {
				t.Errorf("ResolveCategoryMarkup(category=%d) = %s, want %s", tt.categoryID, got, tt.want)
			}
		})
	}
}

// markupReapplyMappings 只实现 reapplyMarkupPrices 用到的方法。
type markupReapplyMappings struct {
	mappingcontract.MappingRepository
	byConnection []mappingdomain.Mapping
}

func (m *markupReapplyMappings) ListActiveByConnection(connectionID uint) ([]mappingdomain.Mapping, error) {
	return m.byConnection, nil
}

type markupReapplySKUMappings struct {
	mappingcontract.SKUMappingRepository
	byProductMapping map[uint][]mappingdomain.SKUMapping
}

func (m *markupReapplySKUMappings) ListByProductMapping(productMappingID uint) ([]mappingdomain.SKUMapping, error) {
	return m.byProductMapping[productMappingID], nil
}

type markupReapplySKUs struct {
	mappingcontract.SKURepository
	items  map[uint]*productdomain.ProductSKU
	update []uint
}

func (m *markupReapplySKUs) GetByID(id uint) (*productdomain.ProductSKU, error) {
	return m.items[id], nil
}

func (m *markupReapplySKUs) Update(item *productdomain.ProductSKU) error {
	m.update = append(m.update, item.ID)
	m.items[item.ID] = item
	return nil
}

// ListByProduct 供 recalcProductPrice 重新计算商品基准价。
func (m *markupReapplySKUs) ListByProduct(productID uint, onlyActive bool) ([]productdomain.ProductSKU, error) {
	result := make([]productdomain.ProductSKU, 0, len(m.items))
	for _, sku := range m.items {
		if sku.ProductID == productID {
			result = append(result, *sku)
		}
	}
	return result, nil
}

type markupReapplyProducts struct {
	mappingcontract.ProductRepository
	items map[string]*productdomain.Product
}

func (m *markupReapplyProducts) GetByID(id string) (*productdomain.Product, error) {
	return m.items[id], nil
}

func (m *markupReapplyProducts) Update(item *productdomain.Product) error {
	m.items[strconv.FormatUint(uint64(item.ID), 10)] = item
	return nil
}

type markupReapplyConnections struct {
	mappingcontract.ConnectionProvider
	conn *siteconnectiondomain.Connection
}

func (m *markupReapplyConnections) GetByID(id uint) (*siteconnectiondomain.Connection, error) {
	return m.conn, nil
}

// TestReapplyMarkupLocalSkipsMappingsWithoutCategoryTree 覆盖本地重算的跳过规则：
// 分类直接配了加价的映射要重算；继承父分类的和分类 ID 还是 0 的必须跳过，
// 否则会被按默认加价写成错价，而父分类的配置其实更高。
func TestReapplyMarkupLocalSkipsMappingsWithoutCategoryTree(t *testing.T) {
	conn := &siteconnectiondomain.Connection{
		ID:                 7,
		ExchangeRate:       decimal.NewFromInt(1),
		PriceMarkupPercent: decimal.NewFromInt(10),
		PriceRoundingMode:  "none",
		CategoryMarkups: jsonmap.JSON{
			"500": float64(50), // 分类 500 直接配置 50%
			"600": float64(50), // 分类 601 的父分类
		},
	}

	// 三个映射：分类直接配置 / 继承父分类 / 分类 ID 为 0。
	direct := &markupReapplyMappings{byConnection: []mappingdomain.Mapping{
		{ID: 1, LocalProductID: 101, UpstreamCategoryID: 500},
		{ID: 2, LocalProductID: 102, UpstreamCategoryID: 601}, // 601 无配置，需继承 600
		{ID: 3, LocalProductID: 103, UpstreamCategoryID: 0},
	}}

	skus := &markupReapplySKUs{items: map[uint]*productdomain.ProductSKU{
		1: {ID: 1, ProductID: 101, PriceAmount: money.FromDecimal(decimal.NewFromInt(10))},
		2: {ID: 2, ProductID: 102, PriceAmount: money.FromDecimal(decimal.NewFromInt(10))},
		3: {ID: 3, ProductID: 103, PriceAmount: money.FromDecimal(decimal.NewFromInt(10))},
	}}
	skuMappings := &markupReapplySKUMappings{byProductMapping: map[uint][]mappingdomain.SKUMapping{
		1: {{ID: 1, ProductMappingID: 1, LocalSKUID: 1, UpstreamPrice: money.FromDecimal(decimal.NewFromInt(10))}},
		2: {{ID: 2, ProductMappingID: 2, LocalSKUID: 2, UpstreamPrice: money.FromDecimal(decimal.NewFromInt(10))}},
		3: {{ID: 3, ProductMappingID: 3, LocalSKUID: 3, UpstreamPrice: money.FromDecimal(decimal.NewFromInt(10))}},
	}}
	products := &markupReapplyProducts{items: map[string]*productdomain.Product{
		"101": {ID: 101},
		"102": {ID: 102},
		"103": {ID: 103},
	}}

	svc := &Service{
		mappings:    direct,
		skuMappings: skuMappings,
		skus:        skus,
		products:    products,
		connections: &markupReapplyConnections{conn: conn},
	}

	updated, err := svc.ReapplyMarkupLocal(7)
	if err != nil {
		t.Fatalf("reapply local failed: %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated = %d, want 1 (only the direct-configured category)", updated)
	}
	if len(skus.update) != 1 || skus.update[0] != 1 {
		t.Fatalf("updated skus = %v, want only sku 1", skus.update)
	}
	// 分类 500 配 50%：10 × 1 × 1.5 = 15
	if got := skus.items[1].PriceAmount.String(); got != "15.00" {
		t.Fatalf("direct category price = %s, want 15.00", got)
	}
	// 被跳过的两个 SKU 价格保持不变
	if got := skus.items[2].PriceAmount.String(); got != "10.00" {
		t.Fatalf("inherited-category sku price = %s, want unchanged 10.00", got)
	}
	if got := skus.items[3].PriceAmount.String(); got != "10.00" {
		t.Fatalf("unknown-category sku price = %s, want unchanged 10.00", got)
	}
}
