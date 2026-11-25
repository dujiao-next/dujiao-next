package procurement_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	"github.com/dujiao-next/internal/config"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	procurementapp "github.com/dujiao-next/internal/modules/procurement/application"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	procurementgormstore "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"
	procurementmapping "github.com/dujiao-next/internal/modules/procurement/infrastructure/mappingreader"
	procurementorder "github.com/dujiao-next/internal/modules/procurement/infrastructure/orderreader"
	procurementupstream "github.com/dujiao-next/internal/modules/procurement/infrastructure/upstreamgateway"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	siteconnectiongormstore "github.com/dujiao-next/internal/modules/siteconnection/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/upstream"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ListFilter = procurementcontract.ListFilter
type ProcurementOrder = procurementdomain.Order

var (
	ErrExists = procurementcontract.ErrExists
)

func newTestSiteConnectionService(db *gorm.DB, secretKey, uploadsDir string) *siteconnectionapp.Service {
	return siteconnectionapp.NewService(siteconnectiongormstore.New(db), secretKey, uploadsDir)
}

func setupProcurementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:procurement_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&orderdomain.OrderRefundRecord{},
		&fulfillmentdomain.Fulfillment{},
		&procurementdomain.Order{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	gormdb.DB = db
	return db
}

func createProcTestOrder(t *testing.T, db *gorm.DB, orderNo, status, fulfillmentType string) *orderdomain.Order {
	t.Helper()
	order := &orderdomain.Order{
		OrderNo:        orderNo,
		UserID:         1,
		Status:         status,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	item := &orderdomain.OrderItem{
		OrderID:         order.ID,
		ProductID:       1,
		SKUID:           1,
		Quantity:        1,
		FulfillmentType: fulfillmentType,
		TitleJSON:       jsonmap.JSON{"zh-CN": "Test Product"},
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(100)),
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(100)),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}
	var loaded orderdomain.Order
	if err := db.Preload("Items").First(&loaded, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	return &loaded
}

func createTestProcurementOrder(t *testing.T, db *gorm.DB, connID, localOrderID uint, localOrderNo, status string) *ProcurementOrder {
	t.Helper()
	order := &ProcurementOrder{
		ConnectionID:    connID,
		LocalOrderID:    localOrderID,
		LocalOrderNo:    localOrderNo,
		Status:          status,
		LocalSellAmount: money.FromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		TraceID:         "test-trace-id",
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create procurement order failed: %v", err)
	}
	return order
}

func newTestProcurementService(db *gorm.DB, connections *siteconnectionapp.Service) *procurementapp.Service {
	orders := ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")
	return procurementapp.NewService(procurementapp.Options{
		Repository:      procurementgormstore.New(db),
		Orders:          procurementorder.New(orders),
		ProductMappings: procurementmapping.NewProducts(mappinggormstore.NewMappingStore(db)),
		SKUMappings:     procurementmapping.NewSKUs(mappinggormstore.NewSKUMappingStore(db)),
		Connections:     procurementupstream.New(connections),
		OrderLifecycle:  procurementgormstore.NewLifecycle(db, nil, nil, config.EmailConfig{}),
	})
}

// 本包的巡检用例会向上游测试服务器发数百次请求，客户端节流会让整个包跑上几分钟。
// 这里统一关闭节流；限速逻辑本身由 internal/upstream 的用例覆盖。
func TestMain(m *testing.M) {
	restore := upstream.SetRequestPacing(0, 0)
	code := m.Run()
	restore()
	os.Exit(code)
}
