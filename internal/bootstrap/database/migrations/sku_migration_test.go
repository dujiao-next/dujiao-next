package migrations

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	cartdomain "github.com/dujiao-next/internal/modules/cart/domain"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupSKUMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := gormdb.DB
	t.Cleanup(func() {
		gormdb.DB = previousDB
	})
	dsn := fmt.Sprintf("file:sku_migration_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	gormdb.DB = db
	return db
}

func setupRegistryMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := gormdb.DB
	t.Cleanup(func() {
		gormdb.DB = previousDB
	})
	dsn := fmt.Sprintf("file:registry_migration_test_%d?mode=memory&cache=shared&_pragma=foreign_keys(1)", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	gormdb.DB = db
	return db
}

func modelIndexNames(t *testing.T, db *gorm.DB, model interface{}) []string {
	t.Helper()
	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(model); err != nil {
		t.Fatalf("parse model indexes: %v", err)
	}
	indexes := statement.Schema.ParseIndexes()
	names := make([]string, 0, len(indexes))
	for _, index := range indexes {
		names = append(names, index.Name)
	}
	if len(names) == 0 {
		t.Fatalf("model %T does not declare any indexes", model)
	}
	return names
}

func assertModelIndexesExist(t *testing.T, db *gorm.DB, model interface{}) {
	t.Helper()
	for _, name := range modelIndexNames(t, db, model) {
		if !db.Migrator().HasIndex(model, name) {
			t.Errorf("model %T index %s does not exist", model, name)
		}
	}
}

func dropModelIndexes(t *testing.T, db *gorm.DB, model interface{}) {
	t.Helper()
	for _, name := range modelIndexNames(t, db, model) {
		if !db.Migrator().HasIndex(model, name) {
			continue
		}
		if err := db.Migrator().DropIndex(model, name); err != nil {
			t.Fatalf("drop model %T index %s: %v", model, name, err)
		}
	}
}

func TestEnsureProductSKUMigrationBackfillLegacyData(t *testing.T) {
	db := setupSKUMigrationTestDB(t)

	if err := db.AutoMigrate(
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&orderdomain.OrderItem{},
		&cartdomain.Item{},
		&cardsecretdomain.Secret{},
		&cardsecretdomain.Batch{},
		&settingsstore.SettingRecord{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	product := &productdomain.Product{
		CategoryID:        1,
		Slug:              "sku-migration-legacy",
		TitleJSON:         jsonmap.JSON{"zh-CN": "历史商品"},
		PriceAmount:       money.FromDecimal(decimal.NewFromInt(128)),
		PurchaseType:      "member",
		FulfillmentType:   "manual",
		ManualStockTotal:  20,
		ManualStockLocked: 3,
		ManualStockSold:   5,
		IsActive:          true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	now := time.Now()
	orderItem := &orderdomain.OrderItem{
		OrderID:         1,
		ProductID:       product.ID,
		SKUID:           0,
		TitleJSON:       jsonmap.JSON{"zh-CN": "历史商品"},
		UnitPrice:       product.PriceAmount,
		Quantity:        1,
		TotalPrice:      product.PriceAmount,
		FulfillmentType: "manual",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(orderItem).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}

	cartItem := &cartdomain.Item{
		UserID:          1001,
		ProductID:       product.ID,
		SKUID:           0,
		Quantity:        2,
		FulfillmentType: "manual",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(cartItem).Error; err != nil {
		t.Fatalf("create cart item failed: %v", err)
	}

	batch := &cardsecretdomain.Batch{
		ProductID:  product.ID,
		SKUID:      0,
		BatchNo:    "SKU-MIGRATION-BATCH-001",
		Source:     "manual",
		TotalCount: 1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(batch).Error; err != nil {
		t.Fatalf("create card secret batch failed: %v", err)
	}

	secret := &cardsecretdomain.Secret{
		ProductID: product.ID,
		SKUID:     0,
		BatchID:   &batch.ID,
		Secret:    "CARD-001",
		Status:    cardsecretdomain.StatusAvailable,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(secret).Error; err != nil {
		t.Fatalf("create card secret failed: %v", err)
	}

	if err := ensureProductSKUMigration(); err != nil {
		t.Fatalf("ensure sku migration failed: %v", err)
	}

	var sku productdomain.ProductSKU
	if err := db.Where("product_id = ? AND sku_code = ?", product.ID, productdomain.DefaultSKUCode).First(&sku).Error; err != nil {
		t.Fatalf("query default sku failed: %v", err)
	}
	if !sku.PriceAmount.Decimal.Equal(product.PriceAmount.Decimal) {
		t.Fatalf("default sku price mismatch want %s got %s", product.PriceAmount.String(), sku.PriceAmount.String())
	}
	if sku.ManualStockTotal != product.ManualStockTotal || sku.ManualStockLocked != product.ManualStockLocked || sku.ManualStockSold != product.ManualStockSold {
		t.Fatalf("default sku stock snapshot mismatch")
	}

	var gotOrderItem orderdomain.OrderItem
	if err := db.First(&gotOrderItem, orderItem.ID).Error; err != nil {
		t.Fatalf("reload order item failed: %v", err)
	}
	if gotOrderItem.SKUID != sku.ID {
		t.Fatalf("order item sku_id want %d got %d", sku.ID, gotOrderItem.SKUID)
	}

	var gotCartItem cartdomain.Item
	if err := db.First(&gotCartItem, cartItem.ID).Error; err != nil {
		t.Fatalf("reload cart item failed: %v", err)
	}
	if gotCartItem.SKUID != sku.ID {
		t.Fatalf("cart item sku_id want %d got %d", sku.ID, gotCartItem.SKUID)
	}

	var gotBatch cardsecretdomain.Batch
	if err := db.First(&gotBatch, batch.ID).Error; err != nil {
		t.Fatalf("reload card secret batch failed: %v", err)
	}
	if gotBatch.SKUID != sku.ID {
		t.Fatalf("card secret batch sku_id want %d got %d", sku.ID, gotBatch.SKUID)
	}

	var gotSecret cardsecretdomain.Secret
	if err := db.First(&gotSecret, secret.ID).Error; err != nil {
		t.Fatalf("reload card secret failed: %v", err)
	}
	if gotSecret.SKUID != sku.ID {
		t.Fatalf("card secret sku_id want %d got %d", sku.ID, gotSecret.SKUID)
	}

	if err := ensureProductSKUMigration(); err != nil {
		t.Fatalf("ensure sku migration second run failed: %v", err)
	}

	var skuCount int64
	if err := db.Model(&productdomain.ProductSKU{}).Where("product_id = ?", product.ID).Count(&skuCount).Error; err != nil {
		t.Fatalf("count product sku failed: %v", err)
	}
	if skuCount != 1 {
		t.Fatalf("idempotent check failed: sku count want 1 got %d", skuCount)
	}
}

func TestMigrateCartSKUUniqueIndex(t *testing.T) {
	db := setupSKUMigrationTestDB(t)
	if err := db.AutoMigrate(&cartdomain.Item{}); err != nil {
		t.Fatalf("auto migrate cart item failed: %v", err)
	}

	// 构造历史唯一索引，验证迁移函数会移除该索引并保留新索引。
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_cart_user_product ON cart_items(user_id, product_id)").Error; err != nil {
		t.Fatalf("create legacy index failed: %v", err)
	}

	if err := migrateCartSKUUniqueIndex(); err != nil {
		t.Fatalf("migrate cart unique index failed: %v", err)
	}

	if db.Migrator().HasIndex(&cartdomain.Item{}, "idx_cart_user_product") {
		t.Fatalf("legacy unique index idx_cart_user_product should be dropped")
	}
	if !db.Migrator().HasIndex(&cartdomain.Item{}, "idx_cart_user_product_sku") {
		t.Fatalf("new unique index idx_cart_user_product_sku should exist")
	}
}

func TestEnsureUserOAuthIdentityUserProviderUniqueIndexFailsClosedOnLegacyDuplicates(t *testing.T) {
	db := setupSKUMigrationTestDB(t)
	if err := db.AutoMigrate(&externalidentitydomain.Identity{}); err != nil {
		t.Fatalf("auto migrate identity: %v", err)
	}
	now := time.Now()
	first := &externalidentitydomain.Identity{
		UserID: 42, Provider: "telegram", ProviderUserID: "legacy-effective",
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	second := &externalidentitydomain.Identity{
		UserID: 42, Provider: "telegram", ProviderUserID: "legacy-hidden",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(first).Error; err != nil {
		t.Fatalf("create first legacy identity: %v", err)
	}
	if err := db.Create(second).Error; err != nil {
		t.Fatalf("create duplicate legacy identity: %v", err)
	}

	err := ensureUserOAuthIdentityUserProviderUniqueIndex()
	if err == nil {
		t.Fatalf("expected explicit duplicate preflight failure")
	}
	for _, expected := range []string{
		userOAuthIdentityUserProviderUniqueIndex,
		"1 duplicate user/provider group",
		"user_id=42",
		`provider="telegram"`,
		"count=2",
		"SELECT id,user_id,provider,provider_user_id",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("migration error %q does not include %q", err, expected)
		}
	}
	if db.Migrator().HasIndex(
		&userOAuthIdentityUserProviderIndexSchema{},
		userOAuthIdentityUserProviderUniqueIndex,
	) {
		t.Fatalf("index %s must not exist after dirty preflight failure", userOAuthIdentityUserProviderUniqueIndex)
	}

	var identities []externalidentitydomain.Identity
	if err := db.Where("user_id = ? AND provider = ?", 42, "telegram").
		Order("id ASC").
		Find(&identities).Error; err != nil {
		t.Fatalf("load deduplicated identities: %v", err)
	}
	if len(identities) != 2 || identities[0].ID != first.ID || identities[1].ID != second.ID {
		t.Fatalf("dirty migration changed credentials: %+v", identities)
	}

	// After an operator resolves the duplicate deliberately, the same migration
	// creates the invariant and remains idempotent.
	if err := db.Delete(second).Error; err != nil {
		t.Fatalf("resolve duplicate explicitly: %v", err)
	}
	if err := ensureUserOAuthIdentityUserProviderUniqueIndex(); err != nil {
		t.Fatalf("ensure unique index after resolution: %v", err)
	}
	if err := ensureUserOAuthIdentityUserProviderUniqueIndex(); err != nil {
		t.Fatalf("idempotent ensure unique index: %v", err)
	}
	if !db.Migrator().HasIndex(
		&userOAuthIdentityUserProviderIndexSchema{},
		userOAuthIdentityUserProviderUniqueIndex,
	) {
		t.Fatalf("missing index %s", userOAuthIdentityUserProviderUniqueIndex)
	}

	duplicate := &externalidentitydomain.Identity{
		UserID: 42, Provider: "telegram", ProviderUserID: "must-fail",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(duplicate).Error; err == nil {
		t.Fatalf("expected database unique constraint for same user/provider")
	}
	otherUser := &externalidentitydomain.Identity{
		UserID: 43, Provider: "telegram", ProviderUserID: "other-user",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(otherUser).Error; err != nil {
		t.Fatalf("same provider on another user should remain valid: %v", err)
	}
}

type racingNamedIndexMigrator struct {
	hasIndexResults []bool
	hasIndexCalls   int
	createCalls     int
	createErr       error
}

func (m *racingNamedIndexMigrator) HasIndex(interface{}, string) bool {
	index := m.hasIndexCalls
	m.hasIndexCalls++
	if index >= len(m.hasIndexResults) {
		return false
	}
	return m.hasIndexResults[index]
}

func (m *racingNamedIndexMigrator) CreateIndex(interface{}, string) error {
	m.createCalls++
	return m.createErr
}

func TestCreateIndexConvergingOnExistingHandlesOnlyConcurrentWinner(t *testing.T) {
	ddlErr := errors.New("index already exists")
	migrator := &racingNamedIndexMigrator{
		hasIndexResults: []bool{false, true},
		createErr:       ddlErr,
	}
	if err := createIndexConvergingOnExisting(migrator, &struct{}{}, "idx_target"); err != nil {
		t.Fatalf("concurrent winner should converge: %v", err)
	}
	if migrator.createCalls != 1 || migrator.hasIndexCalls != 2 {
		t.Fatalf("calls = create:%d has:%d, want 1/2", migrator.createCalls, migrator.hasIndexCalls)
	}
}

func TestCreateIndexConvergingOnExistingPreservesDDLFailure(t *testing.T) {
	ddlErr := errors.New("duplicate rows prevent unique index")
	migrator := &racingNamedIndexMigrator{
		hasIndexResults: []bool{false, false},
		createErr:       ddlErr,
	}
	err := createIndexConvergingOnExisting(migrator, &struct{}{}, "idx_target")
	if !errors.Is(err, ddlErr) {
		t.Fatalf("error = %v, want original DDL error", err)
	}
}

func TestAutoMigrateOwnsResellerSchemaAndCrossModuleConstraints(t *testing.T) {
	db := setupRegistryMigrationTestDB(t)
	if err := AutoMigrate(); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	if err := AutoMigrate(); err != nil {
		t.Fatalf("idempotent auto migrate failed: %v", err)
	}
	for _, table := range []string{
		"reseller_profiles",
		"reseller_domains",
		"reseller_site_configs",
		"reseller_product_settings",
		"reseller_order_snapshots",
		"reseller_ledger_entries",
		"reseller_withdraw_requests",
		"reseller_balance_accounts",
		"reseller_related_accounts",
	} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("central AutoMigrate did not create reseller table %s", table)
		}
	}
	for _, index := range []struct {
		model interface{}
		name  string
	}{
		{model: &resellerdomain.Domain{}, name: "idx_reseller_domains_active_domain"},
		{model: &resellerdomain.SiteConfig{}, name: "idx_reseller_site_configs_active_reseller"},
		{model: &resellerdomain.ProductSetting{}, name: "idx_reseller_product_settings_active_scope"},
		{model: &resellerdomain.BalanceAccount{}, name: "idx_reseller_balance_accounts_active_currency"},
		{model: &resellerdomain.RelatedAccount{}, name: "idx_reseller_related_accounts_active_user"},
	} {
		if !db.Migrator().HasIndex(index.model, index.name) {
			t.Errorf("central AutoMigrate did not create reseller index %s", index.name)
		}
	}
	for _, constraint := range []string{
		cartProductForeignKeyConstraint,
		cartSKUForeignKeyConstraint,
	} {
		if !db.Migrator().HasConstraint(&cartItemConstraintSchema{}, constraint) {
			t.Errorf("central AutoMigrate did not create cart constraint %s", constraint)
		}
	}
	if !db.Migrator().HasConstraint(&procurementdomain.Order{}, procurementOrderForeignKeyConstraint) {
		t.Errorf("central AutoMigrate did not create procurement constraint %s", procurementOrderForeignKeyConstraint)
	}
	if db.Migrator().HasConstraint(&procurementdomain.Order{}, supersededProcurementOrderForeignKeyConstraint) {
		t.Errorf("central AutoMigrate retained superseded procurement constraint %s", supersededProcurementOrderForeignKeyConstraint)
	}
}

func TestBackfillPendingOrderRiskIPsHandlesLegacyNullsAndIPv6(t *testing.T) {
	db := setupSKUMigrationTestDB(t)
	if err := db.AutoMigrate(&orderdomain.Order{}); err != nil {
		t.Fatalf("auto migrate orders: %v", err)
	}

	pendingIPv4 := &orderdomain.Order{OrderNo: "risk-ipv4", Status: constants.OrderStatusPendingPayment, Currency: "USD", ClientIP: "1.2.3.4"}
	pendingIPv6 := &orderdomain.Order{OrderNo: "risk-ipv6", Status: constants.OrderStatusPendingPayment, Currency: "USD", ClientIP: "2001:db8:abcd:12::9"}
	paid := &orderdomain.Order{OrderNo: "risk-paid", Status: constants.OrderStatusPaid, Currency: "USD", ClientIP: "5.6.7.8"}
	for _, order := range []*orderdomain.Order{pendingIPv4, pendingIPv6, paid} {
		if err := db.Create(order).Error; err != nil {
			t.Fatalf("create order %s: %v", order.OrderNo, err)
		}
		if err := db.Exec("UPDATE orders SET risk_ip = NULL WHERE id = ?", order.ID).Error; err != nil {
			t.Fatalf("set legacy NULL risk_ip for %s: %v", order.OrderNo, err)
		}
	}

	if err := backfillPendingOrderRiskIPs(db); err != nil {
		t.Fatalf("backfill risk IPs: %v", err)
	}
	var gotIPv4, gotIPv6, gotPaid orderdomain.Order
	if err := db.First(&gotIPv4, pendingIPv4.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotIPv6, pendingIPv6.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&gotPaid, paid.ID).Error; err != nil {
		t.Fatal(err)
	}
	if gotIPv4.RiskIP != "1.2.3.4" || gotIPv6.RiskIP != "2001:db8:abcd:12::/64" {
		t.Fatalf("unexpected risk IP backfill: ipv4=%q ipv6=%q", gotIPv4.RiskIP, gotIPv6.RiskIP)
	}
	if gotPaid.RiskIP != "" {
		t.Fatalf("paid history must not be backfilled, got %q", gotPaid.RiskIP)
	}
}

func TestEnsureCartForeignKeyConstraintsPreservesRowsAndEnforcesReferences(t *testing.T) {
	db := setupRegistryMigrationTestDB(t)
	for _, statement := range []string{
		`CREATE TABLE products (id integer PRIMARY KEY)`,
		`CREATE TABLE product_skus (id integer PRIMARY KEY)`,
		`INSERT INTO products (id) VALUES (1)`,
		`INSERT INTO product_skus (id) VALUES (2)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare cart schema: %v", err)
		}
	}
	if err := db.AutoMigrate(&cartdomain.Item{}); err != nil {
		t.Fatalf("prepare indexed cart schema: %v", err)
	}
	if err := db.Exec(`INSERT INTO cart_items (id, user_id, product_id, sku_id, quantity) VALUES (3, 4, 1, 2, 1)`).Error; err != nil {
		t.Fatalf("prepare cart row: %v", err)
	}
	assertModelIndexesExist(t, db, &cartdomain.Item{})

	if err := ensureCartForeignKeyConstraints(); err != nil {
		t.Fatalf("ensure cart constraints: %v", err)
	}
	assertModelIndexesExist(t, db, &cartdomain.Item{})
	var rowCount int64
	if err := db.Table("cart_items").Where("id = ?", 3).Count(&rowCount).Error; err != nil {
		t.Fatalf("count preserved cart rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("cart constraint migration lost existing row: count=%d", rowCount)
	}
	if err := db.Exec(`INSERT INTO cart_items (id, user_id, product_id, sku_id, quantity) VALUES (5, 6, 999, 2, 1)`).Error; err == nil {
		t.Fatal("cart product foreign key did not reject missing product")
	}
	if err := db.Exec(`INSERT INTO cart_items (id, user_id, product_id, sku_id, quantity) VALUES (7, 8, 1, 999, 1)`).Error; err == nil {
		t.Fatal("cart SKU foreign key did not reject missing SKU")
	}

	dropModelIndexes(t, db, &cartdomain.Item{})
	if err := ensureCartForeignKeyConstraints(); err != nil {
		t.Fatalf("repair cart indexes with existing constraints: %v", err)
	}
	assertModelIndexesExist(t, db, &cartdomain.Item{})
	for _, constraint := range []string{cartProductForeignKeyConstraint, cartSKUForeignKeyConstraint} {
		if !db.Migrator().HasConstraint(&cartItemConstraintSchema{}, constraint) {
			t.Errorf("cart index repair removed constraint %s", constraint)
		}
	}
	if err := db.Exec(`INSERT INTO cart_items (id, user_id, product_id, sku_id, quantity) VALUES (9, 4, 1, 2, 1)`).Error; err == nil {
		t.Fatal("repaired cart unique index did not reject duplicate user/product/SKU")
	}
}

func TestEnsureCartForeignKeyConstraintsRejectsOrphansBeforeSchemaChanges(t *testing.T) {
	db := setupRegistryMigrationTestDB(t)
	for _, statement := range []string{
		`CREATE TABLE products (id integer PRIMARY KEY)`,
		`CREATE TABLE product_skus (id integer PRIMARY KEY)`,
		`CREATE TABLE cart_items (id integer PRIMARY KEY, user_id integer NOT NULL, product_id integer NOT NULL, sku_id integer NOT NULL, quantity integer NOT NULL)`,
		`INSERT INTO product_skus (id) VALUES (2)`,
		`INSERT INTO cart_items (id, user_id, product_id, sku_id, quantity) VALUES (3, 4, 999, 2, 1)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("prepare orphaned cart schema: %v", err)
		}
	}

	err := ensureCartForeignKeyConstraints()
	if err == nil || !strings.Contains(err.Error(), cartProductForeignKeyConstraint) {
		t.Fatalf("expected descriptive product reference error, got %v", err)
	}
	for _, constraint := range []string{cartProductForeignKeyConstraint, cartSKUForeignKeyConstraint} {
		if db.Migrator().HasConstraint(&cartItemConstraintSchema{}, constraint) {
			t.Fatalf("orphan preflight left partial constraint %s", constraint)
		}
	}
}

type supersededProcurementOrderSchema struct {
	ID                  uint `gorm:"primarykey"`
	LocalOrderID        uint
	LocalOrderReference *supersededProcurementOrderReference `gorm:"foreignKey:LocalOrderID;references:ID;constraint:fk_procurement_orders_local_order_reference"`
}

func (supersededProcurementOrderSchema) TableName() string { return "procurement_orders" }

type supersededProcurementOrderReference struct {
	ID uint `gorm:"primarykey"`
}

func (supersededProcurementOrderReference) TableName() string { return "orders" }

type procurementOrderIndexSchema struct {
	ID              uint       `gorm:"primarykey"`
	ConnectionID    uint       `gorm:"index"`
	LocalOrderID    uint       `gorm:"index"`
	LocalOrderNo    string     `gorm:"index"`
	UpstreamOrderNo string     `gorm:"index"`
	Status          string     `gorm:"index"`
	NextRetryAt     *time.Time `gorm:"index"`
	TraceID         string     `gorm:"index"`
	CreatedAt       time.Time  `gorm:"index"`
	UpdatedAt       time.Time  `gorm:"index"`
	DeletedAt       *time.Time `gorm:"index"`
}

func (procurementOrderIndexSchema) TableName() string { return "procurement_orders" }

func TestAutoMigrateReplacesSupersededProcurementConstraintName(t *testing.T) {
	db := setupRegistryMigrationTestDB(t)
	if err := db.AutoMigrate(&supersededProcurementOrderSchema{}); err != nil {
		t.Fatalf("create superseded procurement schema failed: %v", err)
	}
	if err := db.AutoMigrate(&procurementOrderIndexSchema{}); err != nil {
		t.Fatalf("create indexed superseded procurement schema failed: %v", err)
	}
	assertModelIndexesExist(t, db, &procurementdomain.Order{})
	if !db.Migrator().HasConstraint(&supersededProcurementOrderSchema{}, supersededProcurementOrderForeignKeyConstraint) {
		t.Fatalf("test setup did not create superseded procurement constraint")
	}

	if err := AutoMigrate(); err != nil {
		t.Fatalf("auto migrate superseded schema failed: %v", err)
	}
	if db.Migrator().HasConstraint(&procurementdomain.Order{}, supersededProcurementOrderForeignKeyConstraint) {
		t.Errorf("superseded procurement constraint %s still exists", supersededProcurementOrderForeignKeyConstraint)
	}
	if !db.Migrator().HasConstraint(&procurementdomain.Order{}, procurementOrderForeignKeyConstraint) {
		t.Errorf("canonical procurement constraint %s does not exist", procurementOrderForeignKeyConstraint)
	}
	assertModelIndexesExist(t, db, &procurementdomain.Order{})

	dropModelIndexes(t, db, &procurementdomain.Order{})
	if err := ensureProcurementOrderForeignKeyConstraint(); err != nil {
		t.Fatalf("repair procurement indexes with canonical constraint: %v", err)
	}
	assertModelIndexesExist(t, db, &procurementdomain.Order{})
	if !db.Migrator().HasConstraint(&procurementdomain.Order{}, procurementOrderForeignKeyConstraint) {
		t.Errorf("procurement index repair removed canonical constraint %s", procurementOrderForeignKeyConstraint)
	}
}

// 旧库升级：布尔开关 false → 比例 0（不参与），true → NULL（跟随全站）。
func TestEnsureProductAffiliateRateMigrationMapsLegacySwitch(t *testing.T) {
	db := setupSKUMigrationTestDB(t)
	if err := db.AutoMigrate(&productdomain.Product{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("migrate schema failed: %v", err)
	}
	// 模拟旧版本遗留的布尔列。
	if err := db.Exec("ALTER TABLE products ADD COLUMN is_affiliate_enabled numeric NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("add legacy column failed: %v", err)
	}
	seed := func(slug string, legacy bool) uint {
		product := productdomain.Product{Slug: slug, CategoryID: 1, TitleJSON: jsonmap.JSON{"zh-CN": slug}}
		if err := db.Select("Slug", "CategoryID", "TitleJSON").Create(&product).Error; err != nil {
			t.Fatalf("seed %s failed: %v", slug, err)
		}
		if err := db.Exec("UPDATE products SET is_affiliate_enabled = ? WHERE id = ?", legacy, product.ID).Error; err != nil {
			t.Fatalf("set legacy flag %s failed: %v", slug, err)
		}
		return product.ID
	}
	enabledID := seed("enabled", true)
	disabledID := seed("disabled", false)

	if err := ensureProductAffiliateRateMigration(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	var enabled, disabled productdomain.Product
	if err := db.First(&enabled, enabledID).Error; err != nil {
		t.Fatalf("load enabled failed: %v", err)
	}
	if err := db.First(&disabled, disabledID).Error; err != nil {
		t.Fatalf("load disabled failed: %v", err)
	}
	if enabled.AffiliateRatePercent != nil {
		t.Fatalf("legacy enabled product must inherit global rate (NULL), got %s", enabled.AffiliateRatePercent.String())
	}
	if disabled.AffiliateRatePercent == nil || !disabled.AffiliateRatePercent.IsZero() {
		t.Fatalf("legacy disabled product must be excluded (0), got %v", disabled.AffiliateRatePercent)
	}

	// 上线后验证：不应存在“旧开关为关却仍继承全站比例”的商品，否则会给被排除的商品发佣金。
	var leaked int64
	if err := db.Model(&productdomain.Product{}).
		Where("affiliate_rate_percent IS NULL AND is_affiliate_enabled = ?", false).
		Count(&leaked).Error; err != nil {
		t.Fatalf("leak check failed: %v", err)
	}
	if leaked != 0 {
		t.Fatalf("expected 0 excluded products leaking into global rate, got %d", leaked)
	}
}

// 迁移只能执行一次：之后新建的商品（旧列默认 false、新比例 NULL）不能被重跑的迁移误改成“不参与”。
func TestEnsureProductAffiliateRateMigrationRunsOnlyOnce(t *testing.T) {
	db := setupSKUMigrationTestDB(t)
	if err := db.AutoMigrate(&productdomain.Product{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("migrate schema failed: %v", err)
	}
	if err := db.Exec("ALTER TABLE products ADD COLUMN is_affiliate_enabled numeric NOT NULL DEFAULT 0").Error; err != nil {
		t.Fatalf("add legacy column failed: %v", err)
	}
	if err := ensureProductAffiliateRateMigration(); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	created := productdomain.Product{Slug: "created-after-upgrade", CategoryID: 1, TitleJSON: jsonmap.JSON{"zh-CN": "x"}}
	if err := db.Select("Slug", "CategoryID", "TitleJSON").Create(&created).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	if err := ensureProductAffiliateRateMigration(); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	var reloaded productdomain.Product
	if err := db.First(&reloaded, created.ID).Error; err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if reloaded.AffiliateRatePercent != nil {
		t.Fatalf("product created after upgrade must keep inheriting (NULL), got %s", reloaded.AffiliateRatePercent.String())
	}
}

// 全新安装没有旧列：迁移应直接成功并写入标记。
func TestEnsureProductAffiliateRateMigrationFreshInstall(t *testing.T) {
	db := setupSKUMigrationTestDB(t)
	if err := db.AutoMigrate(&productdomain.Product{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("migrate schema failed: %v", err)
	}
	if err := ensureProductAffiliateRateMigration(); err != nil {
		t.Fatalf("fresh install migration failed: %v", err)
	}
	var marker settingsstore.SettingRecord
	if err := db.First(&marker, "key = ?", productAffiliateRateMigrationKey).Error; err != nil {
		t.Fatalf("marker not written: %v", err)
	}
	if !migrationDone(marker.ValueJSON) {
		t.Fatalf("marker should be done, got %v", marker.ValueJSON)
	}
}
