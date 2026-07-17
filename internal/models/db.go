package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/glebarez/sqlite" // 纯 Go SQLite 驱动（基于 modernc.org/sqlite）
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

const (
	manualStockRemainingMigrationSettingKey         = "migration/manual_stock_remaining_v1"
	skuMigrationSettingKey                          = "migration/product_sku_v1"
	categoryParentMigrationSettingKey               = "migration/category_parent_v1"
	paymentProviderBepusdtRenameMigrationSettingKey = "migration/payment_provider_bepusdt_rename_v1"
	orderItemOriginalPriceMigrationKey              = "migration/order_item_original_price_v1"
	manualStockUnlimitedValue                       = -1
)

// DBPoolConfig 数据库连接池配置
type DBPoolConfig struct {
	MaxOpenConns           int
	MaxIdleConns           int
	ConnMaxLifetimeSeconds int
	ConnMaxIdleTimeSeconds int
}

// InitDB 初始化数据库连接
func InitDB(driver, dsn string, pool DBPoolConfig) error {
	var err error
	normalized := strings.ToLower(strings.TrimSpace(driver))
	var dialector gorm.Dialector
	switch normalized {
	case "", "sqlite":
		// glebarez/sqlite 是基于 modernc.org/sqlite 的纯 Go 驱动
		// 追加 PRAGMA 参数避免并发写入时 busy-spin 导致 CPU 飙升
		dialector = sqlite.Open(appendSQLitePragmas(dsn))
	case "postgres", "postgresql":
		dialector = postgres.Open(dsn)
	default:
		return fmt.Errorf("unsupported database driver: %s", driver)
	}
	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger:  logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return err
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	applyDBPool(sqlDB, pool)

	// SQLite 额外执行 PRAGMA 确保 WAL 模式生效
	if normalized == "" || normalized == "sqlite" {
		DB.Exec("PRAGMA journal_mode=WAL")
		DB.Exec("PRAGMA busy_timeout=5000")
		DB.Exec("PRAGMA synchronous=NORMAL")
	}
	return nil
}

// appendSQLitePragmas 在 SQLite DSN 中追加关键 PRAGMA 参数
func appendSQLitePragmas(dsn string) string {
	// glebarez/sqlite 支持 ?_pragma=key=value 形式
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep +
		"_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)"
}

func applyDBPool(sqlDB *sql.DB, pool DBPoolConfig) {
	if sqlDB == nil {
		return
	}
	if pool.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(pool.MaxOpenConns)
	}
	if pool.MaxIdleConns >= 0 {
		sqlDB.SetMaxIdleConns(pool.MaxIdleConns)
	}
	if pool.ConnMaxLifetimeSeconds > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(pool.ConnMaxLifetimeSeconds) * time.Second)
	}
	if pool.ConnMaxIdleTimeSeconds > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(pool.ConnMaxIdleTimeSeconds) * time.Second)
	}
}

// AutoMigrate 自动迁移所有数据库表
func AutoMigrate() error {
	if err := DB.AutoMigrate(
		&Admin{},
		&User{},
		&UserOAuthIdentity{},
		&AffiliateProfile{},
		&AffiliateClick{},
		&AffiliateCommission{},
		&AffiliateWithdrawRequest{},
		&ResellerProfile{},
		&ResellerDomain{},
		&ResellerSiteConfig{},
		&ResellerProductSetting{},
		&ResellerOrderSnapshot{},
		&ResellerLedgerEntry{},
		&ResellerWithdrawRequest{},
		&ResellerBalanceAccount{},
		&ResellerRelatedAccount{},
		&WalletAccount{},
		&WalletTransaction{},
		&WalletRechargeOrder{},
		&UserLoginLog{},
		&AuthzAuditLog{},
		&NotificationLog{},
		&AdminLoginLog{},
		&EmailVerifyCode{},
		&Order{},
		&OrderItem{},
		&OrderRefundRecord{},
		&CartItem{},
		&PaymentChannel{},
		&Payment{},
		&CardSecret{},
		&CardSecretBatch{},
		&GiftCard{},
		&GiftCardBatch{},
		&Fulfillment{},
		&Coupon{},
		&CouponUsage{},
		&Promotion{},
		&Category{},
		&Product{},
		&ProductSKU{},
		&Post{},
		&PostProduct{},
		&PostCategory{},
		&Banner{},
		&Setting{},
		&ApiCredential{},
		&SiteConnection{},
		&ProductMapping{},
		&SKUMapping{},
		&ProcurementOrder{},
		&DownstreamOrderRef{},
		&ReconciliationJob{},
		&ReconciliationItem{},
		&ChannelClient{},
		&TelegramBroadcast{},
		&MemberLevel{},
		&MemberLevelPrice{},
		&Media{},
	); err != nil {
		return err
	}

	if err := migrateCartSKUUniqueIndex(); err != nil {
		return err
	}

	if err := ensureProductSKUMigration(); err != nil {
		return err
	}
	if err := ensureManualStockRemainingMigration(); err != nil {
		return err
	}
	if err := ensureCategoryParentMigration(); err != nil {
		return err
	}
	if err := ensurePaymentProviderBepusdtRenameMigration(); err != nil {
		return err
	}
	if err := ensureOrderItemOriginalPriceMigration(); err != nil {
		return err
	}
	if err := ensureResellerIndexes(DB); err != nil {
		return err
	}

	// 移除历史遗留商品币种列，统一由站点配置提供币种。
	if DB.Migrator().HasColumn(&Product{}, "price_currency") {
		if err := DB.Migrator().DropColumn(&Product{}, "price_currency"); err != nil {
			return err
		}
	}
	return nil
}
